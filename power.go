package main

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// Battery primary (node 0xA4) Object-Dictionary telemetry CAN IDs.
//
// The battery pack publishes a contiguous run of OD signals on the main bus.
// Each signal is keyed by its a1 (signal index) octet; the node octet a0=0xA4,
// the port a2=0x82 and a3=0 are constant. Multi-byte fields are big-endian on
// the wire (the transport byteswaps to host order).
const (
	BatteryChargingID    = 0x14807040 // a1=0x03: charge voltage/current
	BatteryCellID        = 0x14809040 // a1=0x04: per-cell series (no-op decoder)
	BatteryCapacityID    = 0x1480B040 // a1=0x05: SoC
	BatteryWarningID     = 0x1480D040 // a1=0x06: alarm bit-field
	BatteryStatusID      = 0x1480F040 // a1=0x07: status bit-field
	BatteryVoltageID     = 0x14811040 // a1=0x08: pack voltage (mV)
	BatteryTemperatureID = 0x14813040 // a1=0x09: temperatures + currents
	BatteryHealthID      = 0x14815040 // a1=0x0A: health + cycle count
)

// Battery/charger command + charger node CAN IDs.
const (
	// BatteryCommandID carries the 1-byte battery/charger command opcode in
	// frame[0]. The power-control board (node 0xA3) relays it to the battery.
	BatteryCommandID = 0x14603040 // node 0xA3 command channel (a1=0x01)

	// Charger (node 0xA7) factory test / burn-in clear frames. Payload A5 5A 00.
	ChargerStateInitID  = 0x14E23040 // OD init/handshake on the charger node
	ChargerClearTest1ID = 0x14E23214 // clear test & burn-in mode (sent first)
	ChargerClearTest2ID = 0x14E23210 // clear test & burn-in mode (sent second)
)

// Power-control OD signals (node 0xA3 / main power node 0x82).
const (
	PowerStateID           = 0x10403041 // power_state (main power node 0x82)
	PowerControlControlID  = 0x14605040 // power_control control/state
	PowerControlMeasureID  = 0x14609040 // power_control measurements
	PowerPedalSwitchInitID = 0x14415040 // power_pedal switch control init (polled)
	RearCarrierQueryID     = 0x1870F040 // rear-carrier switch_control query
)

// Battery/charger command opcodes (frame[0] on BatteryCommandID).
const (
	BatteryCmdOff             = 0x00 // battery OFF
	BatteryCmdOn              = 0x01 // battery ON
	BatteryCmdIdentifyCharger = 0x05 // IdentifyCharger
	BatteryCmdReset           = 0x06 // battery RESET
	BatteryCmdShipping        = 0x08 // shipping mode
	BatteryCmdClearFaults     = 0x09 // clear fault flags
)

// ODNodes maps an Object-Dictionary node id (the a0 octet) to its device /
// service name. The physical CAN ECUs match the PF=SA values in the device
// table; the rest are i.MX8-side software service endpoints sharing the same
// OD address space. Sourced from the firmware's device-name → node-code map.
var ODNodes = map[byte]string{
	0x00: "core_services",
	0x08: "heartbeat",
	0x82: "power",
	0x84: "ride",
	0x87: "logging",
	0x88: "ux",
	0x8A: "update",
	0x8B: "mqtt_od_bridge",
	0x8C: "mqtt_ftp",
	0x8D: "motor_control",
	0x8F: "imx8_bridge",
	0x90: "ble",
	0x91: "modem",
	0xA1: "motor_sensor",
	0xA2: "power_pedal",
	0xA3: "power_control",
	0xA4: "battery_primary",
	0xA5: "battery_secondary",
	0xA7: "charger",
	0xC0: "user_ecu",
	0xC1: "elock",
	0xC2: "eshifter",
	0xC3: "rearlight",
	0xC4: "frontlight",
	0xE0: "phone",
	0xE2: "backoffice",
	0xFB: "ping",
	0xFC: "dummy",
	0xFD: "battery_test",
	0xFE: "raspberry",
	0xFF: "test",
}

// odFields extracts the Object-Dictionary fields from a 29-bit CAN ID:
// a0 = node (bits 28:21), a1 = signal index (bits 20:13).
func odFields(canIDHex string) (a0, a1 byte, ok bool) {
	id, err := strconv.ParseUint(canIDHex, 16, 32)
	if err != nil {
		return 0, 0, false
	}
	return byte((id >> 21) & 0xFF), byte((id >> 13) & 0xFF), true
}

// ODNodeName returns the OD device/service name for a node id, or "".
func ODNodeName(node byte) string { return ODNodes[node] }

// PowerFrameType identifies a battery/charger/power-control OD frame.
type PowerFrameType int

const (
	PowerFrameUnknown PowerFrameType = iota
	PowerFrameBatteryCharging
	PowerFrameBatteryCell
	PowerFrameBatteryCapacity
	PowerFrameBatteryWarning
	PowerFrameBatteryStatus
	PowerFrameBatteryVoltage
	PowerFrameBatteryTemperature
	PowerFrameBatteryHealth
	PowerFrameBatteryCommand
	PowerFrameChargerStateInit
	PowerFrameChargerClearTest
	PowerFramePowerState
	PowerFramePowerControlControl
	PowerFramePowerControlMeasure
	PowerFramePowerPedalSwitchInit
	PowerFrameRearCarrierQuery
)

// IsPowerFrame maps a CAN ID (hex string, no 0x) to its OD frame type.
func IsPowerFrame(canIDHex string) PowerFrameType {
	switch strings.ToUpper(canIDHex) {
	case "14807040":
		return PowerFrameBatteryCharging
	case "14809040":
		return PowerFrameBatteryCell
	case "1480B040":
		return PowerFrameBatteryCapacity
	case "1480D040":
		return PowerFrameBatteryWarning
	case "1480F040":
		return PowerFrameBatteryStatus
	case "14811040":
		return PowerFrameBatteryVoltage
	case "14813040":
		return PowerFrameBatteryTemperature
	case "14815040":
		return PowerFrameBatteryHealth
	case "14603040":
		return PowerFrameBatteryCommand
	case "14E23040":
		return PowerFrameChargerStateInit
	case "14E23210", "14E23214":
		return PowerFrameChargerClearTest
	case "10403041":
		return PowerFramePowerState
	case "14605040":
		return PowerFramePowerControlControl
	case "14609040":
		return PowerFramePowerControlMeasure
	case "14415040":
		return PowerFramePowerPedalSwitchInit
	case "1870F040":
		return PowerFrameRearCarrierQuery
	}
	return PowerFrameUnknown
}

// batteryCmdName returns the human-readable name for a battery/charger opcode.
func batteryCmdName(op byte) string {
	switch op {
	case BatteryCmdOff:
		return "BatteryOFF"
	case BatteryCmdOn:
		return "BatteryON"
	case BatteryCmdIdentifyCharger:
		return "IdentifyCharger"
	case BatteryCmdReset:
		return "BatteryRESET"
	case BatteryCmdShipping:
		return "ShippingMode"
	case BatteryCmdClearFaults:
		return "ClearFaultFlags"
	default:
		return fmt.Sprintf("0x%02X", op)
	}
}

// FormatPowerFrame returns a human-readable description of a battery/charger/
// power-control OD frame, or "" if the CAN ID is not recognized.
func FormatPowerFrame(canIDHex string, data []byte) string {
	switch IsPowerFrame(canIDHex) {
	case PowerFrameBatteryCharging:
		if len(data) >= 4 {
			current := binary.BigEndian.Uint16(data[0:2])
			voltage := binary.BigEndian.Uint16(data[2:4])
			return fmt.Sprintf("BATTERY CHARGING: charge_current=%dmA charge_voltage=%dmV", current, voltage)
		}
		return "BATTERY CHARGING"

	case PowerFrameBatteryCell:
		return "BATTERY CELL"

	case PowerFrameBatteryCapacity:
		if len(data) >= 2 {
			soc := binary.BigEndian.Uint16(data[0:2])
			return fmt.Sprintf("BATTERY CAPACITY: soc=%d soc_app=%d", soc, data[0])
		}
		return "BATTERY CAPACITY"

	case PowerFrameBatteryWarning:
		return fmt.Sprintf("BATTERY WARNING: flags=%X", data)

	case PowerFrameBatteryStatus:
		return fmt.Sprintf("BATTERY STATUS: flags=%X", data)

	case PowerFrameBatteryVoltage:
		if len(data) >= 2 {
			mv := binary.BigEndian.Uint16(data[0:2])
			return fmt.Sprintf("BATTERY VOLTAGE: %dmV", mv)
		}
		return "BATTERY VOLTAGE"

	case PowerFrameBatteryTemperature:
		// Logical record is multi-frame (26 bytes); decode the leading 8 bytes:
		// cell temps + currents.
		if len(data) >= 8 {
			dischg := binary.BigEndian.Uint16(data[4:6])
			maxCur := binary.BigEndian.Uint16(data[6:8])
			return fmt.Sprintf("BATTERY TEMP: cell1=%d cell2=%d chgMOS=%d dsgMOS=%d discharge_current=%dmA max_current=%dmA",
				data[0], data[1], data[2], data[3], dischg, maxCur)
		}
		return "BATTERY TEMP"

	case PowerFrameBatteryHealth:
		if len(data) >= 8 {
			health := binary.BigEndian.Uint16(data[4:6])
			cycles := binary.BigEndian.Uint16(data[6:8])
			return fmt.Sprintf("BATTERY HEALTH: health=%d cycles=%d", health, cycles)
		}
		return "BATTERY HEALTH"

	case PowerFrameBatteryCommand:
		if len(data) >= 1 {
			return fmt.Sprintf("BATTERY CMD: %s", batteryCmdName(data[0]))
		}
		return "BATTERY CMD"

	case PowerFrameChargerStateInit:
		return "CHARGER STATE INIT"

	case PowerFrameChargerClearTest:
		return "CHARGER CLEAR TEST & BURN-IN"

	case PowerFramePowerState:
		return fmt.Sprintf("POWER STATE: %X", data)

	case PowerFramePowerControlControl:
		return fmt.Sprintf("POWER CONTROL: %X", data)

	case PowerFramePowerControlMeasure:
		return fmt.Sprintf("POWER MEASUREMENTS: %X", data)

	case PowerFramePowerPedalSwitchInit:
		return "POWER PEDAL SWITCH INIT"

	case PowerFrameRearCarrierQuery:
		return "REAR CARRIER QUERY"
	}

	// Fallback: name otherwise-undocumented OD signals on the battery/charger/
	// power-control nodes (a0 in 0xA3..0xA7) by their node + signal index.
	if a0, a1, ok := odFields(canIDHex); ok && a0 >= 0xA3 && a0 <= 0xA7 {
		if name := ODNodeName(a0); name != "" {
			return fmt.Sprintf("OD %s signal a1=0x%02X", name, a1)
		}
	}
	return ""
}
