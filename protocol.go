package main

import (
	"fmt"
	"strconv"
	"strings"
)

// PrintDeviceTable prints the device table in a formatted layout.
func PrintDeviceTable() {
	fmt.Println("VanMoof SA5 CAN Bus Device Table")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-4s  %-4s  %-6s  %-16s  %s\n", "PS", "PFSA", "Wire", "Name", "MCU")
	fmt.Println(strings.Repeat("-", 80))
	for _, d := range Devices {
		wire := fmt.Sprintf("0x%03X", deviceEncoded(d.PFSA))
		fmt.Printf("0x%02X  0x%02X  %s  %-16s  %s\n", d.Address, d.PFSA, wire, d.Name, d.MCU)
	}
	wire := fmt.Sprintf("0x%03X", deviceEncoded(PowerControlDevice.PFSA))
	fmt.Printf("????  0x%02X  %s  %-16s  %s\n", PowerControlDevice.PFSA, wire, PowerControlDevice.Name, PowerControlDevice.MCU)
	wire = fmt.Sprintf("0x%03X", deviceEncoded(ChargerDevice.PFSA))
	fmt.Printf("????  0x%02X  %s  %-16s  %s  (separate bus)\n", ChargerDevice.PFSA, wire, ChargerDevice.Name, ChargerDevice.MCU)
	wire = fmt.Sprintf("0x%03X", deviceEncoded(BatteryPrimaryDevice.PFSA))
	fmt.Printf("????  0x%02X  %s  %-16s  %s\n", BatteryPrimaryDevice.PFSA, wire, BatteryPrimaryDevice.Name, BatteryPrimaryDevice.MCU)
	fmt.Println(strings.Repeat("-", 80))
}

// LookupCANID looks up a CAN ID hex string and returns its wire-level analysis.
func LookupCANID(idHex string) *WireCANIDInfo {
	return AnalyzeWireCANID(idHex)
}

// Device represents a CAN bus device on the VanMoof SA5.
type Device struct {
	Address uint8  // PS address CAN IDs (e.g., 0x80)
	PFSA    uint8  // PF=SA identifier unique to each LPC546xx device (e.g., 0x8F)
	Name    string // Human-readable name
	MCU     string // Microcontroller type
}

// MessageClass categorizes CAN messages by their DataPage value.
type MessageClass int

const (
	MessageClassStandard MessageClass = iota // DP=1: standard device-to-device
	MessageClassSpecial                      // DP=0: special purpose (light sync, BLE cmd, etc.)
	MessageClassUnknown
)

// DecodedCANID holds the decoded fields of a 29-bit extended CAN ID.
type DecodedCANID struct {
	RawID        uint32
	DataPage     uint8
	PF           uint8 // PDU Format = source device identifier
	PS           uint8 // PDU Specific = target device address
	SA           uint8 // Source Address = always equals PF
	SourceDevice *Device
	TargetDevice *Device
	Class        MessageClass
	Description  string
}

// Devices is the complete device table for SA5
var Devices = []Device{
	{Address: 0x80, PFSA: 0x8F, Name: "imx8_bridge", MCU: "NXP LPC546xx"},
	{Address: 0x82, PFSA: 0x82, Name: "ble", MCU: "Nordic nRF52840"},
	{Address: 0x83, PFSA: 0x83, Name: "modem", MCU: "Nordic nRF52"},
	{Address: 0x84, PFSA: 0xA1, Name: "motor_sensor", MCU: "NXP LPC546xx"},
	{Address: 0x85, PFSA: 0xC1, Name: "elock", MCU: "NXP LPC546xx"},
	{Address: 0x86, PFSA: 0xC0, Name: "user_ecu", MCU: "NXP LPC546xx"},
	{Address: 0x87, PFSA: 0xC4, Name: "frontlight", MCU: "NXP LPC546xx"},
	{Address: 0x88, PFSA: 0xC3, Name: "rearlight", MCU: "NXP LPC546xx"},
	{Address: 0x8D, PFSA: 0x00, Name: "charger_target", MCU: "Unknown"},
	{Address: 0x91, PFSA: 0xC2, Name: "eshifter", MCU: "NXP LPC546xx"},
	{Address: 0x92, PFSA: 0xA2, Name: "power_pedal", MCU: "NXP LPC546xx"},
	{Address: 0x93, PFSA: 0x93, Name: "motor_control", MCU: "Unknown"},
}

// PowerControlDevice is listed separately because its PS address is unknown.
var PowerControlDevice = Device{Address: 0x00, PFSA: 0xA3, Name: "power_control", MCU: "NXP LPC546xx"}

// ChargerDevice uses a separate CAN bus segment.
var ChargerDevice = Device{Address: 0x00, PFSA: 0x70, Name: "charger", MCU: "NXP LPC546xx"}

// BatteryPrimaryDevice is the battery pack OD node (a0=0xA4) on the main bus.
// It publishes the battery telemetry signal run (charging..health).
var BatteryPrimaryDevice = Device{Address: 0x00, PFSA: 0xA4, Name: "battery_primary", MCU: "BMS (Panasonic/Dynapack)"}

// deviceByAddress returns the device with the given PS address, or nil.
func deviceByAddress(addr uint8) *Device {
	for i := range Devices {
		if Devices[i].Address == addr {
			return &Devices[i]
		}
	}
	return nil
}

// deviceByPFSA returns the device with the given PF=SA identifier, or nil.
func deviceByPFSA(pfsa uint8) *Device {
	for i := range Devices {
		if Devices[i].PFSA == pfsa {
			return &Devices[i]
		}
	}
	if PowerControlDevice.PFSA == pfsa {
		return &PowerControlDevice
	}
	if ChargerDevice.PFSA == pfsa {
		return &ChargerDevice
	}
	if BatteryPrimaryDevice.PFSA == pfsa {
		return &BatteryPrimaryDevice
	}
	return nil
}

// CANID represents a CAN ID.
// Format: 0x{DP}{PF}{PS}{SA} where PF=SA=source identifier, PS=target address.
type CANID struct {
	ID          uint32
	Source      *Device
	Target      *Device
	PayloadSize int
	Description string
}

// AllCANIDs is the complete list of CAN IDs.
// DP=1 entries are standard device-to-device messaging handlers.
var AllCANIDs = []CANID{
	// imx8_bridge (PF=SA=0x8F)
	{ID: 0x018F808F, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x80), PayloadSize: 7, Description: "bridge self-handler"},
	{ID: 0x018F828F, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x82), PayloadSize: 4, Description: "bridge<->BLE"},
	{ID: 0x018F838F, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x83), PayloadSize: 8, Description: "bridge<->modem"},
	{ID: 0x018F848F, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x84), PayloadSize: 4, Description: "bridge<->motor_sensor"},
	{ID: 0x018F858F, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x85), PayloadSize: 1, Description: "bridge<->elock"},
	{ID: 0x018F868F, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x86), PayloadSize: 0, Description: "bridge<->user_ecu"},
	{ID: 0x018F938F, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x93), PayloadSize: 1, Description: "bridge<->motor_control"},

	// imx8_bridge DP=0 special messages
	{ID: 0x008F8887, Source: deviceByPFSA(0x8F), Target: nil, PayloadSize: 30, Description: "bridge light-pair sync"},
	{ID: 0x008F0382, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x82), PayloadSize: 8, Description: "bridge->BLE cmd type 3"},
	{ID: 0x008F0182, Source: deviceByPFSA(0x8F), Target: deviceByAddress(0x82), PayloadSize: 1, Description: "bridge->BLE cmd type 1"},
}

// DecodeCANID decodes a 29-bit CAN ID.
func DecodeCANID(raw uint32) *DecodedCANID {
	d := &DecodedCANID{
		RawID:    raw,
		DataPage: uint8((raw >> 24) & 0x1F),
		PF:       uint8((raw >> 16) & 0xFF),
		PS:       uint8((raw >> 8) & 0xFF),
		SA:       uint8(raw & 0xFF),
	}

	d.SourceDevice = deviceByPFSA(d.PF)
	d.TargetDevice = deviceByAddress(d.PS)

	switch d.DataPage {
	case 1:
		d.Class = MessageClassStandard
	case 0:
		d.Class = MessageClassSpecial
	default:
		d.Class = MessageClassUnknown
	}

	d.Description = d.describe()
	return d
}

func (d *DecodedCANID) describe() string {
	src := "unknown"
	if d.SourceDevice != nil {
		src = d.SourceDevice.Name
	}

	dst := fmt.Sprintf("0x%02X", d.PS)
	if d.TargetDevice != nil {
		dst = d.TargetDevice.Name
	}

	switch d.Class {
	case MessageClassStandard:
		if d.SourceDevice != nil && d.TargetDevice != nil && d.SourceDevice.Address == d.PS {
			return fmt.Sprintf("%s self-handler", src)
		}
		return fmt.Sprintf("%s -> %s", src, dst)
	case MessageClassSpecial:
		if d.PS == 0x88 && d.SA == 0x87 {
			return fmt.Sprintf("%s light-pair sync", src)
		}
		if d.SA == 0x82 {
			return fmt.Sprintf("%s -> BLE cmd 0x%02X", src, d.PS)
		}
		return fmt.Sprintf("%s special -> %s (cmd=0x%02X)", src, dst, d.PS)
	default:
		return fmt.Sprintf("%s -> %s (DP=%d)", src, dst, d.DataPage)
	}
}

// String returns a human-readable representation.
func (d *DecodedCANID) String() string {
	return fmt.Sprintf("0x%08X [DP=%d PF=0x%02X PS=0x%02X SA=0x%02X] %s",
		d.RawID, d.DataPage, d.PF, d.PS, d.SA, d.Description)
}

// ParseCANIDString parses a hex string like "018F808F" into a CAN ID.
func ParseCANIDString(s string) (*DecodedCANID, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	val, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid CAN ID hex string %q: %w", s, err)
	}
	return DecodeCANID(uint32(val)), nil
}

// ConstructCANID builds a CAN ID from components.
func ConstructCANID(dp uint8, sourcePFSA uint8, targetAddr uint8) uint32 {
	return (uint32(dp) << 24) | (uint32(sourcePFSA) << 16) | (uint32(targetAddr) << 8) | uint32(sourcePFSA)
}

// deviceEncoded computes the on-wire encoded value for a device's PFSA.
// Formula: (PFSA & 0x7F) << 5, producing a 12-bit value.
func deviceEncoded(pfsa uint8) uint16 {
	return uint16(pfsa&0x7F) << 5
}

// deviceByEncoded returns the device matching the 12-bit on-wire encoded value.
func deviceByEncoded(enc uint16) *Device {
	if enc%0x20 != 0 {
		return nil // not a valid device-encoded value
	}
	for i := range Devices {
		if Devices[i].PFSA != 0 && deviceEncoded(Devices[i].PFSA) == enc {
			return &Devices[i]
		}
	}
	if deviceEncoded(PowerControlDevice.PFSA) == enc {
		return &PowerControlDevice
	}
	if deviceEncoded(ChargerDevice.PFSA) == enc {
		return &ChargerDevice
	}
	if deviceEncoded(BatteryPrimaryDevice.PFSA) == enc {
		return &BatteryPrimaryDevice
	}
	return nil
}

// WireCANIDInfo holds the decoded on-wire CAN ID fields.
//
// On-wire 29-bit CAN ID structure:
//
//	Bit 28:    0 = heartbeat, 1 = data frame
//	Heartbeat: bits 27:16 = 0x111, bits 15:12 = 0x1, bits 11:0 = device_encoded
//	Data:      bits 27:16 = sender_encoded, bits 15:12 = msg_class, bits 11:0 = dest_encoded
//
// device_encoded = (PFSA & 0x7F) << 5
type WireCANIDInfo struct {
	RawHex      string
	RawValue    uint32
	IsExtended  bool
	IsHeartbeat bool
	Source      *Device // sender (data frames) or heartbeat source
	Destination *Device // receiver (data frames only)
	MsgClass    uint8   // bits 15:12 message class marker
	Annotations []string
}

// AnalyzeWireCANID decodes an on-wire CAN ID into source/destination devices.
func AnalyzeWireCANID(idHex string) *WireCANIDInfo {
	val, err := strconv.ParseUint(idHex, 16, 32)
	if err != nil {
		return &WireCANIDInfo{RawHex: idHex}
	}

	info := &WireCANIDInfo{
		RawHex:     idHex,
		RawValue:   uint32(val),
		IsExtended: len(idHex) > 3,
	}

	if len(idHex) != 8 {
		return info
	}

	bit28 := (val >> 28) & 1

	if bit28 == 0 {
		// Heartbeat: bits 27:16 = 0x111, bits 15:12 = 0x1, bits 11:0 = device
		if (val>>12)&0xFFFF == 0x1111 {
			info.IsHeartbeat = true
			devEnc := uint16(val & 0xFFF)
			dev := deviceByEncoded(devEnc)
			info.Source = dev
			if dev != nil {
				info.Annotations = append(info.Annotations,
					fmt.Sprintf("heartbeat from %s", dev.Name))
			} else {
				info.Annotations = append(info.Annotations,
					fmt.Sprintf("heartbeat (device 0x%03X)", devEnc))
			}
		}
		return info
	}

	// Data frame: bits 27:16 = sender, bits 15:12 = class, bits 11:0 = destination
	senderEnc := uint16((val >> 16) & 0xFFF)
	info.MsgClass = uint8((val >> 12) & 0xF)
	destEnc := uint16(val & 0xFFF)

	info.Source = deviceByEncoded(senderEnc)
	info.Destination = deviceByEncoded(destEnc)

	srcName := fmt.Sprintf("0x%03X", senderEnc)
	if info.Source != nil {
		srcName = info.Source.Name
	}
	dstName := fmt.Sprintf("0x%03X", destEnc)
	if info.Destination != nil {
		dstName = info.Destination.Name
	}

	if senderEnc == destEnc {
		info.Annotations = append(info.Annotations,
			fmt.Sprintf("%s (class=0x%X)", srcName, info.MsgClass))
	} else {
		info.Annotations = append(info.Annotations,
			fmt.Sprintf("%s -> %s (class=0x%X)", srcName, dstName, info.MsgClass))
	}

	return info
}

// PrintProtocolSummary prints the CAN ID format and key findings.
func PrintProtocolSummary() {
	fmt.Println("VanMoof SA5 CAN Bus Protocol Summary")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Println()
	fmt.Println("CAN ID Format (29-bit Extended):")
	fmt.Println("  Bits 28:24 = DP (Data Page): 0x01=standard, 0x00=special")
	fmt.Println("  Bits 23:16 = PF (PDU Format): source device identifier")
	fmt.Println("  Bits 15:8  = PS (PDU Specific): target device address")
	fmt.Println("  Bits 7:0   = SA (Source Address): always equals PF")
	fmt.Println("  Pattern: 0x01{SRC}{DST}{SRC}")
	fmt.Println()
	fmt.Println("On-Wire CAN ID Encoding:")
	fmt.Println("  device_encoded = (PFSA & 0x7F) << 5  (12-bit value)")
	fmt.Println()
	fmt.Println("  Heartbeat: bit28=0, bits27:16=0x111, bits15:12=0x1, bits11:0=device")
	fmt.Println("  Data:      bit28=1, bits27:16=sender, bits15:12=class, bits11:0=dest")
	fmt.Println()
	fmt.Println("  Heartbeat formula: CAN_ID = 0x01111000 + device_encoded")
	fmt.Println("  Data formula:      CAN_ID = (1<<28) | (src<<16) | (class<<12) | dst")
	fmt.Println()
	fmt.Println("Bus Configuration:")
	fmt.Println("  Speed: 1 Mbps")
	fmt.Println("  Hardware filters: none (promiscuous mode)")
	fmt.Println("  Controller: Bosch M_CAN (CAN FD capable, used as classic CAN)")
	fmt.Println("  Base address: 0x4009D000")
	fmt.Println()
	fmt.Println("Bus Topology:")
	fmt.Println("  Main bus: 12 devices (bridge, BLE, modem, motor_sensor, elock,")
	fmt.Println("            user_ecu, frontlight, rearlight, eshifter, power_pedal,")
	fmt.Println("            motor_control, power_control)")
	fmt.Println("  Charger bus: charger(0x70) <-> battery/PMU(0x8D)")
	fmt.Println(strings.Repeat("=", 72))
}

// PrintAllCANIDs prints every CAN ID grouped by device.
func PrintAllCANIDs() {
	fmt.Println("All CAN IDs (grouped by source device):")
	fmt.Println(strings.Repeat("=", 72))

	// Build CAN IDs for each device dynamically
	allDevicePFSA := []uint8{0x8F, 0xA1, 0xC1, 0xC0, 0xC4, 0xC3, 0xC2, 0xA2, 0xA3, 0x70}
	targetAddrs := []uint8{0x80, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x91, 0x92, 0x93}

	for _, pfsa := range allDevicePFSA {
		src := deviceByPFSA(pfsa)
		if src == nil {
			continue
		}
		fmt.Printf("\n%s (PF=SA=0x%02X):\n", src.Name, pfsa)

		for _, addr := range targetAddrs {
			tgt := deviceByAddress(addr)
			canID := ConstructCANID(1, pfsa, addr)
			tgtName := fmt.Sprintf("0x%02X", addr)
			selfMark := ""
			if tgt != nil {
				tgtName = tgt.Name
				if tgt.Address == src.Address {
					selfMark = " (SELF)"
				}
			}
			fmt.Printf("  0x%08X -> %-16s%s\n", canID, tgtName, selfMark)
		}

		// DP=0 special: light sync
		fmt.Printf("  0x00%02X8887 -> light-pair sync\n", pfsa)

		// DP=0 special: BLE commands
		fmt.Printf("  0x00%02X0382 -> BLE cmd type 3\n", pfsa)
		fmt.Printf("  0x00%02X0182 -> BLE cmd type 1\n", pfsa)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 72))
}
