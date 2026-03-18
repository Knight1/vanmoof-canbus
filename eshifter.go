package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Eshifter CAN IDs (on-wire encoded)
const (
	EshifterHeartbeatID = 0x01111840 // Eshifter heartbeat
	EshifterConfigID    = 0x10F11840 // Config + sensor data TO eshifter (0x840)
	EshifterTelemetryID = 0x10F11841 // Cadence + actuator feedback TO eshifter (0x841)
	EshifterGearCmdID   = 0x18411840 // Gear change commands TO eshifter
	EshifterStatusID    = 0x1840B110 // Status report FROM eshifter
	EshifterAckID       = 0x1840B100 // Empty ack FROM eshifter
)

// Eshifter config data types (byte 4 after 00 A9 FC 8C prefix)
const (
	EshifterTypeGearSet  = 0x44 // Set initial gear position
	EshifterTypeEnable   = 0x55 // Enable eshifter
	EshifterTypeConfig   = 0x66 // Configuration mode
	EshifterTypeSpeed    = 0x8D // Continuous speed data
	EshifterTypeSpeedAlt = 0x98 // Alternate speed data
	EshifterTypeClear    = 0xCA // Clear / reset
	EshifterTypeConfirm1 = 0xEE // Gear change confirmation part 1
	EshifterTypeConfirm2 = 0xEF // Gear change confirmation part 2
	EshifterTypeStatus   = 0x00 // Status / init
)

// Config data prefix
var eshifterConfigPrefix = []byte{0x00, 0xA9, 0xFC, 0x8C}

// EshifterFrameType identifies the kind of eshifter frame
type EshifterFrameType int

const (
	EshifterFrameUnknown EshifterFrameType = iota
	EshifterFrameHeartbeat
	EshifterFrameGearCmd
	EshifterFrameConfig
	EshifterFrameTelemetry
	EshifterFrameStatusReport
	EshifterFrameAck
)

// EshifterGearCmd represents a decoded gear change command
type EshifterGearCmd struct {
	TargetGear byte
	Mode       byte // 0x00=set, 0x01=force
}

// EshifterStatusReport represents a decoded eshifter status
type EshifterStatusReport struct {
	StatusFlags byte
	SubStatus   byte
	Param1      byte
	CurrentGear byte
	Flags2      byte
	Param2      byte
	Param3      byte
	Param4      byte
}

// EshifterConfigData represents a decoded config/sensor frame
type EshifterConfigData struct {
	DataType byte
	Data     [3]byte
}

// EshifterTelemetryData represents decoded feedback telemetry
type EshifterTelemetryData struct {
	Cadence  byte   // byte 2: cadence RPM during riding, target gear during shift
	IsError  bool   // byte 3 == 0xFF
	Position uint16 // bytes 6-7: wheel counter or actuator position (LE)
}

// IsEshifterFrame checks if a CAN ID belongs to the eshifter protocol
func IsEshifterFrame(canIDHex string) EshifterFrameType {
	canIDHex = strings.ToUpper(canIDHex)
	switch canIDHex {
	case "01111840":
		return EshifterFrameHeartbeat
	case "18411840":
		return EshifterFrameGearCmd
	case "10F11840":
		return EshifterFrameConfig
	case "10F11841":
		return EshifterFrameTelemetry
	case "1840B110":
		return EshifterFrameStatusReport
	case "1840B100":
		return EshifterFrameAck
	}
	return EshifterFrameUnknown
}

// DecodeEshifterGearCmd decodes a gear change command payload
func DecodeEshifterGearCmd(data []byte) *EshifterGearCmd {
	if len(data) < 5 {
		return nil
	}
	if data[0] != 0x01 || data[1] != 0x01 {
		return nil
	}
	return &EshifterGearCmd{
		Mode:       data[2],
		TargetGear: data[3],
	}
}

// DecodeEshifterStatus decodes a status report payload
func DecodeEshifterStatus(data []byte) *EshifterStatusReport {
	if len(data) < 8 {
		return nil
	}
	return &EshifterStatusReport{
		StatusFlags: data[0],
		SubStatus:   data[1],
		Param1:      data[2],
		CurrentGear: data[3],
		Flags2:      data[4],
		Param2:      data[5],
		Param3:      data[6],
		Param4:      data[7],
	}
}

// DecodeEshifterConfig decodes a config/sensor data payload
func DecodeEshifterConfig(data []byte) *EshifterConfigData {
	if len(data) < 8 {
		return nil
	}
	// Check for 00 A9 FC 8C prefix
	if data[0] != 0x00 || data[1] != 0xA9 || data[2] != 0xFC || data[3] != 0x8C {
		return nil
	}
	return &EshifterConfigData{
		DataType: data[4],
		Data:     [3]byte{data[5], data[6], data[7]},
	}
}

// DecodeEshifterTelemetry decodes feedback telemetry payload
func DecodeEshifterTelemetry(data []byte) *EshifterTelemetryData {
	if len(data) < 8 {
		return nil
	}
	return &EshifterTelemetryData{
		Cadence:  data[2],
		IsError:  data[3] == 0xFF,
		Position: binary.LittleEndian.Uint16(data[6:8]),
	}
}

// FormatEshifterFrame returns a human-readable description of an eshifter frame
func FormatEshifterFrame(canIDHex string, data []byte) string {
	ft := IsEshifterFrame(canIDHex)
	switch ft {
	case EshifterFrameGearCmd:
		cmd := DecodeEshifterGearCmd(data)
		if cmd == nil {
			return ""
		}
		mode := "set"
		if cmd.Mode == 0x01 {
			mode = "force"
		}
		return fmt.Sprintf("GEAR COMMAND: shift to gear %d (0x%02X) mode=%s",
			cmd.TargetGear, cmd.TargetGear, mode)

	case EshifterFrameStatusReport:
		st := DecodeEshifterStatus(data)
		if st == nil {
			return ""
		}
		return fmt.Sprintf("ESHIFTER STATUS: gear=%d flags=0x%02X,0x%02X params=%d,%d,%d,%d",
			st.CurrentGear, st.StatusFlags, st.Flags2,
			st.Param1, st.Param2, st.Param3, st.Param4)

	case EshifterFrameConfig:
		cfg := DecodeEshifterConfig(data)
		if cfg != nil {
			return formatConfigType(cfg)
		}
		// Non-prefixed config (init handshake)
		return "ESHIFTER INIT HANDSHAKE"

	case EshifterFrameTelemetry:
		tel := DecodeEshifterTelemetry(data)
		if tel == nil {
			return ""
		}
		if tel.IsError {
			return fmt.Sprintf("ESHIFTER TELEMETRY: ERROR pos=%d (0x%04X)",
				tel.Position, tel.Position)
		}
		if tel.Position == 0 && tel.Cadence == 0 {
			return "ESHIFTER TELEMETRY: idle"
		}
		return fmt.Sprintf("ESHIFTER TELEMETRY: cadence/gear=%d pos/counter=%d (0x%04X)",
			tel.Cadence, tel.Position, tel.Position)

	case EshifterFrameAck:
		return "ESHIFTER ACK"

	case EshifterFrameHeartbeat:
		return "ESHIFTER HEARTBEAT"
	}
	return ""
}

func formatConfigType(cfg *EshifterConfigData) string {
	switch cfg.DataType {
	case EshifterTypeGearSet:
		return fmt.Sprintf("ESHIFTER CONFIG: set gear=%d (0x%02X)", cfg.Data[1], cfg.Data[1])
	case EshifterTypeEnable:
		return "ESHIFTER CONFIG: enable"
	case EshifterTypeConfig:
		return "ESHIFTER CONFIG: configuration mode"
	case EshifterTypeClear:
		return "ESHIFTER CONFIG: clear/reset"
	case EshifterTypeSpeed:
		speed := uint16(cfg.Data[1]) | uint16(cfg.Data[2])<<8
		return fmt.Sprintf("ESHIFTER SENSOR: speed=%d (0x%04X)", speed, speed)
	case EshifterTypeSpeedAlt:
		speed := uint16(cfg.Data[1]) | uint16(cfg.Data[2])<<8
		return fmt.Sprintf("ESHIFTER SENSOR: alt_speed=%d (0x%04X)", speed, speed)
	case EshifterTypeConfirm1:
		return "ESHIFTER CONFIRM: gear change ack part 1"
	case EshifterTypeConfirm2:
		return "ESHIFTER CONFIRM: gear change ack part 2"
	case EshifterTypeStatus:
		return "ESHIFTER CONFIG: status/init"
	default:
		return fmt.Sprintf("ESHIFTER CONFIG: type=0x%02X data=%02X%02X%02X",
			cfg.DataType, cfg.Data[0], cfg.Data[1], cfg.Data[2])
	}
}

// BuildGearCommand creates a gear change CAN frame payload
func BuildGearCommand(gear byte, force bool) []byte {
	mode := byte(0x00)
	if force {
		mode = 0x01
	}
	return []byte{0x01, 0x01, mode, gear, 0x00}
}

// BuildEshifterInitSequence returns the CAN frames needed to initialize the eshifter
func BuildEshifterInitSequence(gear byte) []EshifterCANFrame {
	return []EshifterCANFrame{
		{CANID: "10F11840", Data: []byte{0x3D, 0x95, 0x16, 0x87, 0x18, 0x00, 0x35, 0x00}, Desc: "init handshake"},
		{CANID: "10F11840", Data: []byte{0x00, 0xA9, 0xFC, 0x8C, 0x55, 0x01, 0x01, 0x00}, Desc: "enable eshifter"},
		{CANID: "10F11840", Data: []byte{0x00, 0xA9, 0xFC, 0x8C, 0x44, 0x00, gear, 0x00}, Desc: fmt.Sprintf("set gear %d", gear)},
		{CANID: "10F11840", Data: []byte{0x00, 0xA9, 0xFC, 0x8C, 0x66, 0x01, 0x00, 0x00}, Desc: "configuration mode"},
	}
}

// EshifterCANFrame represents a CAN frame to be sent
type EshifterCANFrame struct {
	CANID string
	Data  []byte
	Desc  string
}

// FormatCansend formats a frame for use with the cansend utility
func (f *EshifterCANFrame) FormatCansend(iface string) string {
	hex := fmt.Sprintf("%X", f.Data)
	return fmt.Sprintf("cansend %s %s#%s", iface, f.CANID, hex)
}

// PrintGearCommands outputs cansend commands for a gear change
func PrintGearCommands(gear byte, force bool, iface string) {
	cmd := EshifterCANFrame{
		CANID: "18411840",
		Data:  BuildGearCommand(gear, force),
		Desc:  fmt.Sprintf("shift to gear %d", gear),
	}

	mode := "set"
	if force {
		mode = "force"
	}

	fmt.Printf("Eshifter: shift to gear %d (0x%02X) mode=%s\n", gear, gear, mode)
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("# %s\n", cmd.Desc)
	fmt.Println(cmd.FormatCansend(iface))
	fmt.Println()
	fmt.Println("# Monitor status after sending:")
	fmt.Printf("# candump %s,1840B110:1FFFFFFF  (status response)\n", iface)
	fmt.Printf("# candump %s,10F11841:1FFFFFFF  (actuator position)\n", iface)
}

// PrintEshifterInitCommands outputs cansend commands for initialization
func PrintEshifterInitCommands(gear byte, iface string) {
	frames := BuildEshifterInitSequence(gear)

	fmt.Printf("Eshifter: initialize with gear %d (0x%02X)\n", gear, gear)
	fmt.Println(strings.Repeat("-", 50))
	for i, f := range frames {
		fmt.Printf("# Step %d: %s\n", i+1, f.Desc)
		fmt.Println(f.FormatCansend(iface))
	}
	fmt.Println()
	fmt.Println("# After init, send gear command:")
	gearCmd := EshifterCANFrame{
		CANID: "18411840",
		Data:  BuildGearCommand(gear, false),
		Desc:  fmt.Sprintf("set gear %d", gear),
	}
	fmt.Printf("# %s\n", gearCmd.Desc)
	fmt.Println(gearCmd.FormatCansend(iface))
	fmt.Println()
	fmt.Println("# Monitor responses:")
	fmt.Printf("# candump %s,1840B110:1FFFFFFF  (status)\n", iface)
	fmt.Printf("# candump %s,10F11841:1FFFFFFF  (telemetry)\n", iface)
	fmt.Printf("# candump %s,10F11840:1FFFFFFF  (confirmations)\n", iface)
}
