package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Battery CAN IDs — main bus telemetry from user_ecu
const (
	BatteryTelemetryID     = 0x18009800 // user_ecu battery status (class 0x9)
	BatteryTelemetryContID = 0x18009801 // Continuation frame
)

// BMS CAN IDs — charger bus (separate physical bus)
const (
	BMSCommandID      = 0x10801000 // Command/Heartbeat (bidirectional)
	BMSWriteID        = 0x1077E400 // Write register data
	BMSWriteVariantID = 0x1077E800 // Write variant

	BMSSetStatusV001ID = 0x00000030 // SetStatus v001 (11-bit standard)
	BMSSetStatusV004ID = 0x149D9060 // SetStatus v004
	BMSSetStatusV015ID = 0x1489BB70 // SetStatus v015+

	BMSChargerStateID = 0x149DE9A0 // Set Charger State (v004+)
	BMSUSBPDStateID   = 0x149E4160 // Set USB-PD State (v004+)

	BMSBLUpdate1ID = 0x10823800 // SPIN bootloader frame 1
	BMSBLUpdate2ID = 0x10823A00 // SPIN bootloader frame 2
	BMSBLUpdate3ID = 0x10823C00 // SPIN bootloader frame 3
	BMSBLDataID    = 0x1077F200 // Bootloader firmware data
	BMSBLAckID     = 0x1077F400 // Bootloader ACK
	BMSBLVersionID = 0x1077F600 // Bootloader version

	BMSAPVersionReplyID   = 0x14901460 // AP firmware version reply
	BMSAPVersionRequestID = 0x14901470 // AP firmware version request
	BMSAPFlashSizeReplyID = 0x14903460 // Flash size reply
	BMSAPFlashSizeReqID   = 0x14903470 // Flash size request
	BMSAPEraseID          = 0x14905470 // Erase
	BMSAPWriteID          = 0x14907470 // Write firmware
	BMSAPCRCReplyID       = 0x14909460 // CRC reply
	BMSAPCRCRequestID     = 0x14909470 // CRC request
	BMSAPUpdateReplyID    = 0x1490B460 // Update reply
	BMSAPUpdateCmdID      = 0x1490B470 // Update command
	BMSAPRebootID         = 0x1490D470 // Reboot

	BMSFeatureControlID = 0x000002FF // Feature control
)

// BMS status codes
const (
	BMSStatusNormal       = 0x01
	BMSStatusCharge       = 0x02
	BMSStatusSleep        = 0x03
	BMSStatusChargeOff    = 0x04
	BMSStatusChargeMOSOff = 0x05
	BMSStatusStandby      = 0x08
	BMSStatusChargeOn     = 0x0C
	BMSStatusChargeOpen   = 0x0D
	BMSStatusRequestInfo  = 0x40
	BMSStatusCellBalance  = 0x80
)

// BatteryFrameType identifies the kind of battery/BMS frame
type BatteryFrameType int

const (
	BatteryFrameUnknown BatteryFrameType = iota
	BatteryFrameTelemetry
	BatteryFrameTelemetryCont
	BatteryFrameBMSCommand
	BatteryFrameBMSWrite
	BatteryFrameBMSWriteVariant
	BatteryFrameBMSSetStatus
	BatteryFrameBMSChargerState
	BatteryFrameBMSUSBPDState
	BatteryFrameBMSBootloader
	BatteryFrameBMSFWUpdate
	BatteryFrameBMSFeatureCtrl
)

// BatteryTelemetryData represents decoded main bus battery telemetry
//
// CAN ID 0x18009800 (user_ecu self, class 0x9) sends an 8-byte frame:
//
//	Byte 0: SoC (State of Charge, 0-100 percentage from BMS register 0x05)
//	Byte 1: 0x00 (padding)
//	Byte 2: Current draw (from BMS register 0x06, raw value; multiply by 10 for mA)
//	Byte 3: 0x00 (padding)
//	Byte 4: Temperature (battery temperature in degrees Fahrenheit; observed 69-70 at ~20-21C ambient)
//	Byte 5: 0x00 (padding)
//	Bytes 6-7: Pack voltage (LE uint16, value/10 = volts, from BMS register 0x04 scaled)
//
// Followed by continuation frame 0x18009801 (2 bytes: 01 00).
type BatteryTelemetryData struct {
	SoC         byte   // byte 0: state of charge percentage (0-100)
	Current     byte   // byte 2: current draw (BMS raw, *10 = mA)
	Temperature byte   // byte 4: battery temperature (Fahrenheit)
	Voltage     uint16 // bytes 6-7: pack voltage (LE uint16, /10 = volts)
}

// BMSSetStatusCmd represents a decoded SetStatus command
type BMSSetStatusCmd struct {
	StatusCode byte
	Flags      byte
}

// IsBatteryFrame checks if a CAN ID belongs to the battery/BMS protocol
func IsBatteryFrame(canIDHex string) BatteryFrameType {
	canIDHex = strings.ToUpper(canIDHex)
	switch canIDHex {
	// Main bus telemetry
	case "18009800":
		return BatteryFrameTelemetry
	case "18009801":
		return BatteryFrameTelemetryCont
	// BMS core
	case "10801000":
		return BatteryFrameBMSCommand
	case "1077E400":
		return BatteryFrameBMSWrite
	case "1077E800":
		return BatteryFrameBMSWriteVariant
	// SetStatus (version-dependent)
	case "00000030", "030", "30", "149D9060", "1489BB70":
		return BatteryFrameBMSSetStatus
	// Charger / USB-PD
	case "149DE9A0":
		return BatteryFrameBMSChargerState
	case "149E4160":
		return BatteryFrameBMSUSBPDState
	// SPIN Bootloader
	case "10823800", "10823A00", "10823C00", "1077F200", "1077F400", "1077F600":
		return BatteryFrameBMSBootloader
	// AP firmware update
	case "14901460", "14901470", "14903460", "14903470",
		"14905470", "14907470", "14909460", "14909470",
		"1490B460", "1490B470", "1490D470":
		return BatteryFrameBMSFWUpdate
	// Feature control
	case "000002FF", "02FF", "2FF":
		return BatteryFrameBMSFeatureCtrl
	}
	return BatteryFrameUnknown
}

// DecodeBatteryTelemetry decodes the 8-byte main bus battery telemetry payload
func DecodeBatteryTelemetry(data []byte) *BatteryTelemetryData {
	if len(data) < 8 {
		return nil
	}
	return &BatteryTelemetryData{
		SoC:         data[0],
		Current:     data[2],
		Temperature: data[4],
		Voltage:     binary.LittleEndian.Uint16(data[6:8]),
	}
}

// DecodeBMSSetStatus decodes a SetStatus command payload
func DecodeBMSSetStatus(data []byte) *BMSSetStatusCmd {
	if len(data) < 1 {
		return nil
	}
	cmd := &BMSSetStatusCmd{StatusCode: data[0]}
	if len(data) >= 2 {
		cmd.Flags = data[1]
	}
	return cmd
}

// FormatBatteryFrame returns a human-readable description of a battery/BMS frame
func FormatBatteryFrame(canIDHex string, data []byte) string {
	ft := IsBatteryFrame(canIDHex)
	switch ft {
	case BatteryFrameTelemetry:
		tel := DecodeBatteryTelemetry(data)
		if tel == nil {
			return "BATTERY TELEMETRY"
		}
		voltageV := float64(tel.Voltage) / 10.0
		currentMA := int(tel.Current) * 10
		tempC := (float64(tel.Temperature) - 32.0) * 5.0 / 9.0
		return fmt.Sprintf("BATTERY TELEMETRY: soc=%d%% current=%dmA temp=%.1fC voltage=%.1fV",
			tel.SoC, currentMA, tempC, voltageV)

	case BatteryFrameTelemetryCont:
		return "BATTERY TELEMETRY CONT"

	case BatteryFrameBMSCommand:
		if len(data) == 0 || isAllZero(data) {
			return "BMS HEARTBEAT"
		}
		return fmt.Sprintf("BMS COMMAND: data=%X", data)

	case BatteryFrameBMSWrite:
		return fmt.Sprintf("BMS WRITE: data=%X", data)

	case BatteryFrameBMSWriteVariant:
		return fmt.Sprintf("BMS WRITE VARIANT: data=%X", data)

	case BatteryFrameBMSSetStatus:
		cmd := DecodeBMSSetStatus(data)
		if cmd == nil {
			return "BMS SET STATUS"
		}
		statusName := bmsStatusName(cmd.StatusCode)
		if cmd.Flags != 0 {
			flags := formatProtectionFlags(cmd.Flags)
			return fmt.Sprintf("BMS SET STATUS: %s (0x%02X) flags=%s", statusName, cmd.StatusCode, flags)
		}
		return fmt.Sprintf("BMS SET STATUS: %s (0x%02X)", statusName, cmd.StatusCode)

	case BatteryFrameBMSChargerState:
		return formatChargerState(data)

	case BatteryFrameBMSUSBPDState:
		return formatUSBPDState(data)

	case BatteryFrameBMSBootloader:
		return formatBootloaderFrame(canIDHex, data)

	case BatteryFrameBMSFWUpdate:
		return formatFWUpdateFrame(canIDHex, data)

	case BatteryFrameBMSFeatureCtrl:
		return formatFeatureControl(data)
	}
	return ""
}

func bmsStatusName(code byte) string {
	switch code {
	case BMSStatusNormal:
		return "Normal"
	case BMSStatusCharge:
		return "Charge"
	case BMSStatusSleep:
		return "Sleep/Shipping"
	case BMSStatusChargeOff:
		return "ChargeOff"
	case BMSStatusChargeMOSOff:
		return "ChargeMOS Off"
	case BMSStatusStandby:
		return "Standby"
	case BMSStatusChargeOn:
		return "ChargeOn"
	case BMSStatusChargeOpen:
		return "ChargeOpen"
	case BMSStatusRequestInfo:
		return "RequestBMSInfo"
	case BMSStatusCellBalance:
		return "CellBalancing"
	default:
		return "Unknown"
	}
}

func formatProtectionFlags(flags byte) string {
	var parts []string
	if flags&0x01 != 0 {
		parts = append(parts, "UnlockCOCP")
	}
	if flags&0x02 != 0 {
		parts = append(parts, "UnlockVoltage")
	}
	if flags&0x04 != 0 {
		parts = append(parts, "UnlockTemp")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("0x%02X", flags)
	}
	return strings.Join(parts, "|")
}

func formatChargerState(data []byte) string {
	if len(data) < 1 {
		return "BMS CHARGER STATE"
	}
	switch data[0] {
	case 0x01:
		return "BMS CHARGER STATE: Standby"
	case 0x02:
		if len(data) >= 5 {
			voltage := binary.LittleEndian.Uint16(data[1:3])
			current := binary.LittleEndian.Uint16(data[3:5])
			return fmt.Sprintf("BMS CHARGER STATE: Charge voltage=%dmV current=%dmA", voltage, current)
		}
		return "BMS CHARGER STATE: Charge"
	case 0x08:
		return "BMS CHARGER STATE: RequestInfo"
	default:
		return fmt.Sprintf("BMS CHARGER STATE: 0x%02X", data[0])
	}
}

func formatUSBPDState(data []byte) string {
	if len(data) < 1 {
		return "BMS USB-PD STATE"
	}
	var parts []string
	if data[0]&0x01 != 0 {
		parts = append(parts, "DisableSource")
	}
	if data[0]&0x02 != 0 {
		parts = append(parts, "AllowSource")
	}
	if data[0]&0x04 != 0 {
		parts = append(parts, "DisableSink")
	}
	if data[0]&0x08 != 0 {
		parts = append(parts, "AllowSink")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("BMS USB-PD STATE: 0x%02X", data[0])
	}
	return "BMS USB-PD STATE: " + strings.Join(parts, "|")
}

func formatBootloaderFrame(canIDHex string, data []byte) string {
	canIDHex = strings.ToUpper(canIDHex)
	switch canIDHex {
	case "10823800":
		return fmt.Sprintf("BMS BOOTLOADER UPDATE1: data=%X", data)
	case "10823A00":
		return fmt.Sprintf("BMS BOOTLOADER UPDATE2: data=%X", data)
	case "10823C00":
		return fmt.Sprintf("BMS BOOTLOADER UPDATE3: data=%X", data)
	case "1077F200":
		return fmt.Sprintf("BMS BOOTLOADER DATA: %d bytes", len(data))
	case "1077F400":
		return "BMS BOOTLOADER ACK"
	case "1077F600":
		return fmt.Sprintf("BMS BOOTLOADER VERSION: data=%X", data)
	}
	return "BMS BOOTLOADER"
}

func formatFWUpdateFrame(canIDHex string, data []byte) string {
	canIDHex = strings.ToUpper(canIDHex)
	switch canIDHex {
	case "14901460":
		return fmt.Sprintf("BMS FW VERSION REPLY: data=%X", data)
	case "14901470":
		return "BMS FW VERSION REQUEST"
	case "14903460":
		return fmt.Sprintf("BMS FW FLASH SIZE REPLY: data=%X", data)
	case "14903470":
		return "BMS FW FLASH SIZE REQUEST"
	case "14905470":
		return "BMS FW ERASE"
	case "14907470":
		return fmt.Sprintf("BMS FW WRITE: %d bytes", len(data))
	case "14909460":
		return fmt.Sprintf("BMS FW CRC REPLY: data=%X", data)
	case "14909470":
		return "BMS FW CRC REQUEST"
	case "1490B460":
		return fmt.Sprintf("BMS FW UPDATE REPLY: data=%X", data)
	case "1490B470":
		return "BMS FW UPDATE COMMAND"
	case "1490D470":
		return "BMS FW REBOOT"
	}
	return "BMS FW UPDATE"
}

func formatFeatureControl(data []byte) string {
	if len(data) < 1 {
		return "BMS FEATURE CONTROL"
	}
	switch data[0] {
	case 0x02:
		return "BMS FEATURE: Disable PDSCP"
	case 0x03:
		return "BMS FEATURE: Enable PDSCP"
	case 0x04:
		return "BMS FEATURE: Disable CellOffLine"
	case 0x05:
		return "BMS FEATURE: Enable CellOffLine"
	case 0x06:
		return "BMS FEATURE: Disable CellBalancing"
	case 0x07:
		return "BMS FEATURE: Enable CellBalancing"
	default:
		return fmt.Sprintf("BMS FEATURE: 0x%02X", data[0])
	}
}

func isAllZero(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

// DecodeFaultStatus decodes the 16-bit fault status flags (register 0x02)
func DecodeFaultStatus(flags uint16) []string {
	names := [16]string{
		"DOTP", "DUTP", "COTP", "CUTP",
		"DOCP1", "DOCP2", "COCP1", "COCP2",
		"OVP1", "OVP2", "UVP1", "UVP2",
		"PDOCP", "PDSCP", "MOTP", "SCP",
	}
	var active []string
	for i, name := range names {
		if flags&(1<<i) != 0 {
			active = append(active, name)
		}
	}
	return active
}

// DecodeWarningStatus decodes the 16-bit warning status flags (register 0x28)
func DecodeWarningStatus(flags uint16) []string {
	names := map[int]string{
		0:  "DOTPW",
		1:  "DUTPW",
		2:  "COTPW",
		3:  "CUTPW",
		4:  "DOCPW",
		6:  "COCPW",
		8:  "OVP1W",
		10: "UVP1W",
		11: "SOC",
		12: "PDOCPW",
		14: "MOTPW",
	}
	var active []string
	for bit := 0; bit < 16; bit++ {
		if flags&(1<<bit) != 0 {
			if name, ok := names[bit]; ok {
				active = append(active, name)
			}
		}
	}
	return active
}

// FormatFaultStatus returns a human-readable string for fault status flags
func FormatFaultStatus(flags uint16) string {
	if flags == 0 {
		return "OK"
	}
	return strings.Join(DecodeFaultStatus(flags), "|")
}

// FormatWarningStatus returns a human-readable string for warning status flags
func FormatWarningStatus(flags uint16) string {
	if flags == 0 {
		return "OK"
	}
	return strings.Join(DecodeWarningStatus(flags), "|")
}

// BMSTemperature converts a raw BMS temperature value to degrees Celsius
func BMSTemperature(raw uint16) float64 {
	return float64(int(raw)-2731) / 10.0
}

// BMSCurrent converts a raw BMS current value to milliamps
func BMSCurrent(raw int16) int {
	return int(raw) * 10
}
