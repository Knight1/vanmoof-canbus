package main

import (
	"fmt"
	"strings"
)

// Light CAN IDs (on-wire encoded)
const (
	// Frontlight (encoded 0x880)
	FrontlightHeartbeatID   = 0x01111880
	FrontlightCmdID         = 0x18827880 // Commands TO frontlight
	FrontlightInitID        = 0x10F11880 // Init handshake
	FrontlightStatusID      = 0x18805110 // Brightness status FROM frontlight
	FrontlightStatusAckID   = 0x18805100 // Status ack FROM frontlight
	FrontlightFeedbackID    = 0x1880F110 // Feedback FROM frontlight
	FrontlightFeedbackAckID = 0x1880F100 // Feedback ack FROM frontlight
	FrontlightDetailID      = 0x18823110 // Detailed status FROM frontlight

	// Rearlight (encoded 0x860)
	RearlightHeartbeatID   = 0x01111860
	RearlightCmdID         = 0x18627860 // Commands TO rearlight
	RearlightInitID        = 0x10F11860 // Init handshake
	RearlightStatusID      = 0x18605110 // Brightness status FROM rearlight
	RearlightStatusAckID   = 0x18605100 // Status ack FROM rearlight
	RearlightFeedbackID    = 0x1860F110 // Feedback FROM rearlight
	RearlightFeedbackAckID = 0x1860F100 // Feedback ack FROM rearlight
	RearlightDetailID      = 0x18623110 // Detailed status FROM rearlight
)

// LightFrameType identifies the kind of light frame
type LightFrameType int

const (
	LightFrameUnknown LightFrameType = iota
	LightFrameHeartbeat
	LightFrameCommand
	LightFrameInit
	LightFrameStatus
	LightFrameStatusAck
	LightFrameFeedback
	LightFrameFeedbackAck
	LightFrameDetail
)

// LightTarget identifies which light a frame belongs to
type LightTarget int

const (
	LightTargetNone LightTarget = iota
	LightTargetFront
	LightTargetRear
)

// LightCommand represents a decoded light command
type LightCommand struct {
	Brightness byte // 0-100 (percentage)
	Mode       byte // 0x01=on, 0xFF=off
}

// LightStatus represents a decoded brightness status
type LightStatus struct {
	Brightness byte // 0-100
}

// LightFeedback represents a decoded feedback status
type LightFeedback struct {
	Status byte
}

// LightDetail represents a decoded detailed status report
type LightDetail struct {
	StatusFlag byte
	Reserved   byte
	Param      byte
	Brightness byte
	Mode       byte
	DevParam   byte
	Reserved2  byte
}

// IsLightFrame checks if a CAN ID belongs to the light protocol
func IsLightFrame(canIDHex string) (LightFrameType, LightTarget) {
	canIDHex = strings.ToUpper(canIDHex)
	switch canIDHex {
	// Frontlight
	case "01111880":
		return LightFrameHeartbeat, LightTargetFront
	case "18827880":
		return LightFrameCommand, LightTargetFront
	case "10F11880":
		return LightFrameInit, LightTargetFront
	case "18805110":
		return LightFrameStatus, LightTargetFront
	case "18805100":
		return LightFrameStatusAck, LightTargetFront
	case "1880F110":
		return LightFrameFeedback, LightTargetFront
	case "1880F100":
		return LightFrameFeedbackAck, LightTargetFront
	case "18823110":
		return LightFrameDetail, LightTargetFront
	// Rearlight
	case "01111860":
		return LightFrameHeartbeat, LightTargetRear
	case "18627860":
		return LightFrameCommand, LightTargetRear
	case "10F11860":
		return LightFrameInit, LightTargetRear
	case "18605110":
		return LightFrameStatus, LightTargetRear
	case "18605100":
		return LightFrameStatusAck, LightTargetRear
	case "1860F110":
		return LightFrameFeedback, LightTargetRear
	case "1860F100":
		return LightFrameFeedbackAck, LightTargetRear
	case "18623110":
		return LightFrameDetail, LightTargetRear
	}
	return LightFrameUnknown, LightTargetNone
}

// DecodeLightCommand decodes a light command payload
func DecodeLightCommand(data []byte) *LightCommand {
	if len(data) < 2 {
		return nil
	}
	return &LightCommand{
		Brightness: data[0],
		Mode:       data[1],
	}
}

// DecodeLightStatus decodes a brightness status payload
func DecodeLightStatus(data []byte) *LightStatus {
	if len(data) < 1 {
		return nil
	}
	return &LightStatus{Brightness: data[0]}
}

// DecodeLightFeedback decodes a feedback status payload
func DecodeLightFeedback(data []byte) *LightFeedback {
	if len(data) < 1 {
		return nil
	}
	return &LightFeedback{Status: data[0]}
}

// DecodeLightDetail decodes a detailed status report payload
func DecodeLightDetail(data []byte) *LightDetail {
	if len(data) < 7 {
		return nil
	}
	return &LightDetail{
		StatusFlag: data[0],
		Reserved:   data[1],
		Param:      data[2],
		Brightness: data[3],
		Mode:       data[4],
		DevParam:   data[5],
		Reserved2:  data[6],
	}
}

// FormatLightFrame returns a human-readable description of a light frame
func FormatLightFrame(canIDHex string, data []byte) string {
	ft, target := IsLightFrame(canIDHex)
	if ft == LightFrameUnknown {
		return ""
	}

	name := "FRONTLIGHT"
	if target == LightTargetRear {
		name = "REARLIGHT"
	}

	switch ft {
	case LightFrameHeartbeat:
		return fmt.Sprintf("%s HEARTBEAT", name)

	case LightFrameCommand:
		cmd := DecodeLightCommand(data)
		if cmd == nil {
			return ""
		}
		if cmd.Brightness == 0xFF && cmd.Mode == 0xFF {
			return fmt.Sprintf("%s COMMAND: off", name)
		}
		return fmt.Sprintf("%s COMMAND: brightness=%d%% mode=0x%02X", name, cmd.Brightness, cmd.Mode)

	case LightFrameInit:
		return fmt.Sprintf("%s INIT HANDSHAKE", name)

	case LightFrameStatus:
		st := DecodeLightStatus(data)
		if st == nil {
			return ""
		}
		return fmt.Sprintf("%s STATUS: brightness=%d%%", name, st.Brightness)

	case LightFrameStatusAck:
		return fmt.Sprintf("%s STATUS ACK", name)

	case LightFrameFeedback:
		fb := DecodeLightFeedback(data)
		if fb == nil {
			return ""
		}
		return fmt.Sprintf("%s FEEDBACK: status=0x%02X", name, fb.Status)

	case LightFrameFeedbackAck:
		return fmt.Sprintf("%s FEEDBACK ACK", name)

	case LightFrameDetail:
		det := DecodeLightDetail(data)
		if det == nil {
			return ""
		}
		return fmt.Sprintf("%s DETAIL: brightness=%d%% mode=0x%02X param=%d",
			name, det.Brightness, det.Mode, det.DevParam)
	}
	return ""
}

// BuildLightCommand creates a light command CAN frame payload
func BuildLightCommand(brightness byte, on bool) []byte {
	if !on {
		return []byte{0xFF, 0xFF}
	}
	return []byte{brightness, 0x01}
}

// PrintFrontlightCommands outputs cansend commands for frontlight control
func PrintFrontlightCommands(brightness byte, iface string) {
	var frame EshifterCANFrame
	if brightness == 0 {
		frame = EshifterCANFrame{
			CANID: "18827880",
			Data:  BuildLightCommand(0, false),
			Desc:  "turn off frontlight",
		}
	} else {
		frame = EshifterCANFrame{
			CANID: "18827880",
			Data:  BuildLightCommand(brightness, true),
			Desc:  fmt.Sprintf("set frontlight brightness to %d%%", brightness),
		}
	}

	if brightness == 0 {
		fmt.Println("Frontlight: off")
	} else {
		fmt.Printf("Frontlight: brightness %d%%\n", brightness)
	}
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("# %s\n", frame.Desc)
	fmt.Println(frame.FormatCansend(iface))
	fmt.Println()
	fmt.Println("# Monitor status after sending:")
	fmt.Printf("# candump %s,18805110:1FFFFFFF  (brightness status)\n", iface)
	fmt.Printf("# candump %s,1880F110:1FFFFFFF  (feedback)\n", iface)
	fmt.Printf("# candump %s,18823110:1FFFFFFF  (detailed status)\n", iface)
}

// PrintRearlightCommands outputs cansend commands for rearlight control
func PrintRearlightCommands(brightness byte, iface string) {
	var frame EshifterCANFrame
	if brightness == 0 {
		frame = EshifterCANFrame{
			CANID: "18627860",
			Data:  BuildLightCommand(0, false),
			Desc:  "turn off rearlight",
		}
	} else {
		frame = EshifterCANFrame{
			CANID: "18627860",
			Data:  BuildLightCommand(brightness, true),
			Desc:  fmt.Sprintf("set rearlight brightness to %d%%", brightness),
		}
	}

	if brightness == 0 {
		fmt.Println("Rearlight: off")
	} else {
		fmt.Printf("Rearlight: brightness %d%%\n", brightness)
	}
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("# %s\n", frame.Desc)
	fmt.Println(frame.FormatCansend(iface))
	fmt.Println()
	fmt.Println("# Monitor status after sending:")
	fmt.Printf("# candump %s,18605110:1FFFFFFF  (brightness status)\n", iface)
	fmt.Printf("# candump %s,1860F110:1FFFFFFF  (feedback)\n", iface)
	fmt.Printf("# candump %s,18623110:1FFFFFFF  (detailed status)\n", iface)
}
