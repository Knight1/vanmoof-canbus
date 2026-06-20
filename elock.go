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

// Elock data-report multi-frame stream.
//
// A data report is emitted as a start frame (ID ending ...10), zero or more
// continuation frames (...11), and a terminating empty ack (...00). Two ID
// spellings appear on the wire — the 0x182031xx family (matching
// ElockDataReportID / ElockDataAckID above) and the 0x180231xx family seen
// during the unlock exchange — both are recognised so the decoder works on
// either capture.
const (
	ElockDataReportContID  = 0x18203111 // continuation paired with ElockDataReportID
	ElockDataReport2ID     = 0x18023110 // data report start (alt spelling)
	ElockDataReport2ContID = 0x18023111 // data report continuation (alt spelling)
	ElockDataAck2ID        = 0x18023100 // data report ack (alt spelling)
)

// Elock special purpose CAN IDs (DP=0)
const (
	ElockLightSyncID   = 0x00C18887 // Light pair sync
	ElockBLECmd03ID    = 0x00C10382 // BLE command (CMD=0x03)
	ElockBLECmd01ID    = 0x00C10182 // BLE command (CMD=0x01)
	ElockMotorSensorID = 0x00C101A1 // CMD=0x01 to motor_sensor
	ElockPowerPedalID  = 0x00C10BA2 // CMD=0x0B to power_pedal
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
	ElockFrameDataReportCont
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
	case "18203110", "18023110":
		return ElockFrameDataReport
	case "18203111", "18023111":
		return ElockFrameDataReportCont
	case "18203100", "18023100":
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
			return "ELOCK DATA REPORT (start)"
		}
		return fmt.Sprintf("ELOCK DATA REPORT (start, %d bytes): %X", len(data), data)

	case ElockFrameDataReportCont:
		if len(data) == 0 {
			return "ELOCK DATA REPORT (cont)"
		}
		return fmt.Sprintf("ELOCK DATA REPORT (cont, %d bytes): %X", len(data), data)

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

// PrintElockUnlockConfirm outputs a runnable send-then-monitor unlock sequence:
// it sends the unlock trigger, then watches the lock-state broadcast and reports
// the outcome (UNLOCKED/STUCK) or times out. The unlock frame is identical to
// PrintElockUnlockCommand; only the monitor wrapper is added.
func PrintElockUnlockConfirm(iface string) {
	frame := EshifterCANFrame{
		CANID: "1800D110",
		Data:  BuildElockUnlockCommand(),
		Desc:  "unlock elock (system state broadcast)",
	}

	fmt.Println("Elock: unlock + confirm")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("# 1) send the unlock trigger")
	fmt.Println(frame.FormatCansend(iface))
	fmt.Println()
	fmt.Println("# 2) watch the lock state (timeout 5s) and report the result")
	fmt.Printf("timeout 5 candump -L %s,18209820:1FFFFFFF | while read -r line; do\n", iface)
	fmt.Println("  case \"${line##*#}\" in")
	fmt.Println("    02) echo \"RESULT: UNLOCKED\";              break ;;")
	fmt.Println("    03) echo \"RESULT: STUCK (mechanism jammed)\"; break ;;")
	fmt.Println("  esac")
	fmt.Println("done")
	fmt.Println()
	fmt.Println("# Note: unlock is angle/debounce confirmed by the lock mechanism —")
	fmt.Println("# a unit with no mechanical load may report STUCK (03).")
}

// ElockDataReport is a reassembled elock data-report multi-frame message.
type ElockDataReport struct {
	Payload []byte // start frame + all continuation frames, concatenated in order
	Frames  int    // number of CAN frames consumed (start + continuations)
}

// ElockDataReportAssembler reassembles the elock data-report stream: a start
// frame (ID ...10) followed by continuation frames (...11), terminated by an ack
// frame (...00) or by the next start frame. Field semantics of the payload are
// not interpreted — only the on-wire framing is reassembled.
type ElockDataReportAssembler struct {
	active  bool
	payload []byte
	frames  int
}

// Feed pushes one CAN frame into the assembler. It returns a completed report
// (and true) when the stream terminates — on the ack frame, or when a new start
// frame arrives while a report is already in progress. Otherwise it returns
// (nil, false). Non-data-report frames are ignored.
func (a *ElockDataReportAssembler) Feed(canIDHex string, data []byte) (*ElockDataReport, bool) {
	switch IsElockFrame(canIDHex) {
	case ElockFrameDataReport:
		var done *ElockDataReport
		if a.active {
			done = a.flush() // a new report starts before the previous one was acked
		}
		a.active = true
		a.payload = append([]byte(nil), data...)
		a.frames = 1
		return done, done != nil

	case ElockFrameDataReportCont:
		if !a.active {
			return nil, false
		}
		a.payload = append(a.payload, data...)
		a.frames++
		return nil, false

	case ElockFrameDataAck:
		if !a.active {
			return nil, false
		}
		return a.flush(), true

	default:
		return nil, false
	}
}

// flush returns the in-progress report and resets the assembler.
func (a *ElockDataReportAssembler) flush() *ElockDataReport {
	r := &ElockDataReport{Payload: a.payload, Frames: a.frames}
	a.active = false
	a.payload = nil
	a.frames = 0
	return r
}

// FormatElockDataReport renders a reassembled data report.
func FormatElockDataReport(r *ElockDataReport) string {
	if r == nil || len(r.Payload) == 0 {
		return "ELOCK DATA REPORT: (empty)"
	}
	return fmt.Sprintf("ELOCK DATA REPORT: %d bytes over %d frame(s): %X",
		len(r.Payload), r.Frames, r.Payload)
}
