package main

import (
	"fmt"
	"strings"
)

// Elock CAN IDs (on-wire encoded, device_encoded = 0x820)
const (
	ElockHeartbeatID  = 0x01111820 // Elock heartbeat (8 bytes, all zeros)
	ElockStatusID     = 0x18209820 // Lock state broadcast (class 0x9, self-addressed, 1 byte)
	ElockDataReportID = 0x18203110 // Data report (class 0x3, to dest 0x110)
	ElockDataAckID    = 0x18203100 // Data ack (class 0x3, to dest 0x100, empty)
)

// System state CAN IDs (user_ecu broadcast, class 0xD)
// These trigger lock/unlock on the elock via broadcast to all devices.
const (
	SystemStateID    = 0x1800D110 // user_ecu -> broadcast (class 0xD, dest 0x110), 1-byte payload
	SystemStateAckID = 0x1800D100 // user_ecu -> ack (class 0xD, dest 0x100), empty
)

// System state payload values (for 0x1800D110)
const (
	SystemStateUnlock = 0x01 // Triggers elock unlock attempt
)

// Elock special purpose CAN IDs (DP=0)
const (
	ElockLightSyncID    = 0x00C18887 // Light pair sync
	ElockBLECmd03ID     = 0x00C10382 // BLE command (CMD=0x03)
	ElockBLECmd01ID     = 0x00C10182 // BLE command (CMD=0x01)
	ElockMotorSensorID  = 0x00C101A1 // CMD=0x01 to motor_sensor
	ElockPowerPedalID   = 0x00C10BA2 // CMD=0x0B to power_pedal
)

// Lock state values (from CAN status byte on 0x18209820)
// Maps to S6 ElectrifiedS6ELockState enum (BLE topic 362 / 0x016A)
const (
	LockStateUnknown  = 0x00
	LockStateLocked   = 0x01
	LockStateUnlocked = 0x02
	LockStateStuck    = 0x03
)

// ElockFrameType identifies the kind of elock frame
type ElockFrameType int

const (
	ElockFrameUnknown ElockFrameType = iota
	ElockFrameHeartbeat
	ElockFrameStatus
	ElockFrameDataReport
	ElockFrameDataAck
	ElockFrameLightSync
	ElockFrameBLECmd
	ElockFrameMotorSensor
	ElockFramePowerPedal
	ElockFrameSystemState
	ElockFrameSystemStateAck
)

// ElockStatus represents a decoded lock state
type ElockStatus struct {
	State byte
}

// IsElockFrame checks if a CAN ID belongs to the elock protocol
func IsElockFrame(canIDHex string) ElockFrameType {
	canIDHex = strings.ToUpper(canIDHex)
	switch canIDHex {
	case "01111820":
		return ElockFrameHeartbeat
	case "18209820":
		return ElockFrameStatus
	case "18203110":
		return ElockFrameDataReport
	case "18203100":
		return ElockFrameDataAck
	case "00C18887":
		return ElockFrameLightSync
	case "00C10382", "00C10182":
		return ElockFrameBLECmd
	case "00C101A1":
		return ElockFrameMotorSensor
	case "00C10BA2":
		return ElockFramePowerPedal
	case "1800D110":
		return ElockFrameSystemState
	case "1800D100":
		return ElockFrameSystemStateAck
	}
	return ElockFrameUnknown
}

// DecodeElockStatus decodes the 1-byte lock state payload
func DecodeElockStatus(data []byte) *ElockStatus {
	if len(data) < 1 {
		return nil
	}
	return &ElockStatus{State: data[0]}
}

// lockStateName returns the human-readable name for a lock state byte
func lockStateName(state byte) string {
	switch state {
	case LockStateUnknown:
		return "UNKNOWN"
	case LockStateLocked:
		return "LOCKED"
	case LockStateUnlocked:
		return "UNLOCKED"
	case LockStateStuck:
		return "STUCK"
	default:
		return fmt.Sprintf("STATE_0x%02X", state)
	}
}

// FormatElockFrame returns a human-readable description of an elock frame
func FormatElockFrame(canIDHex string, data []byte) string {
	ft := IsElockFrame(canIDHex)
	switch ft {
	case ElockFrameHeartbeat:
		return "ELOCK HEARTBEAT"

	case ElockFrameStatus:
		st := DecodeElockStatus(data)
		if st == nil {
			return "ELOCK STATUS"
		}
		return fmt.Sprintf("ELOCK STATUS: %s (0x%02X)", lockStateName(st.State), st.State)

	case ElockFrameDataReport:
		if len(data) == 0 {
			return "ELOCK DATA REPORT"
		}
		return fmt.Sprintf("ELOCK DATA REPORT: data=%X", data)

	case ElockFrameDataAck:
		return "ELOCK DATA ACK"

	case ElockFrameLightSync:
		return "ELOCK LIGHT SYNC"

	case ElockFrameBLECmd:
		if len(data) == 0 {
			return "ELOCK BLE CMD"
		}
		return fmt.Sprintf("ELOCK BLE CMD: data=%X", data)

	case ElockFrameMotorSensor:
		if len(data) == 0 {
			return "ELOCK -> MOTOR_SENSOR CMD"
		}
		return fmt.Sprintf("ELOCK -> MOTOR_SENSOR CMD: data=%X", data)

	case ElockFramePowerPedal:
		if len(data) == 0 {
			return "ELOCK -> POWER_PEDAL CMD"
		}
		return fmt.Sprintf("ELOCK -> POWER_PEDAL CMD: data=%X", data)

	case ElockFrameSystemState:
		if len(data) == 0 {
			return "SYSTEM STATE CMD"
		}
		if data[0] == SystemStateUnlock {
			return "SYSTEM STATE CMD: UNLOCK (0x01)"
		}
		return fmt.Sprintf("SYSTEM STATE CMD: 0x%02X", data[0])

	case ElockFrameSystemStateAck:
		return "SYSTEM STATE ACK"
	}
	return ""
}

// BuildElockUnlockCommand creates the 1-byte payload that triggers elock unlock.
// Sent on CAN ID 0x1800D110 (user_ecu class 0xD broadcast).
// The elock listens on broadcast address 0x110 and actuates the lock motor.
func BuildElockUnlockCommand() []byte {
	return []byte{SystemStateUnlock}
}

// PrintElockUnlockCommand outputs cansend commands for unlocking the elock
func PrintElockUnlockCommand(iface string) {
	frame := EshifterCANFrame{
		CANID: "1800D110",
		Data:  BuildElockUnlockCommand(),
		Desc:  "unlock elock (system state broadcast)",
	}

	fmt.Println("Elock: unlock")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("# %s\n", frame.Desc)
	fmt.Println(frame.FormatCansend(iface))
	fmt.Println()
	fmt.Println("# Monitor lock state after sending:")
	fmt.Printf("# candump %s,18209820:1FFFFFFF  (lock state: 01=LOCKED 02=UNLOCKED 03=STUCK)\n", iface)
	fmt.Printf("# candump %s,1800D100:1FFFFFFF  (system state ack)\n", iface)
}
