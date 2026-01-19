package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

// CANFrame represents a parsed CAN bus frame
type CANFrame struct {
	Timestamp  string
	ID         string
	IsExtended bool
	Direction  string
	Bus        int
	Length     int
	Data       []byte
}

// parseCSVLine parses a CSV line in the format:
// Time Stamp,ID,Extended,Dir,Bus,LEN,D1,D2,D3,D4,D5,D6,D7,D8
func parseCSVLine(fields []string) (*CANFrame, error) {
	if len(fields) < 14 {
		return nil, fmt.Errorf("not enough fields: got %d, need 14", len(fields))
	}

	frame := &CANFrame{
		Timestamp:  fields[0],
		ID:         fields[1],
		IsExtended: strings.ToLower(fields[2]) == "true",
		Direction:  fields[3],
	}

	// Parse Bus
	bus, err := strconv.Atoi(fields[4])
	if err == nil {
		frame.Bus = bus
	}

	// Parse Length
	length, err := strconv.Atoi(fields[5])
	if err != nil {
		return nil, fmt.Errorf("invalid length: %v", err)
	}
	frame.Length = length

	// Parse Data bytes (D1-D8)
	frame.Data = make([]byte, 0, 8)
	for i := 0; i < 8 && i < length; i++ {
		hexStr := strings.TrimSpace(fields[6+i])
		if hexStr == "" || hexStr == "00" && i >= length {
			continue
		}
		b, err := strconv.ParseUint(hexStr, 16, 8)
		if err != nil {
			continue
		}
		frame.Data = append(frame.Data, byte(b))
	}

	return frame, nil
}

// parseCandumpLine extracts CAN ID and payload from candump format
// Format: (timestamp) interface ID#PAYLOAD
func parseCandumpLine(line string) (*CANFrame, error) {
	// Find the '#' separator
	idxHash := strings.Index(line, "#")
	if idxHash == -1 {
		return nil, fmt.Errorf("no # separator found")
	}

	// Extract ID part (everything before #)
	idPart := line[:idxHash]
	idPart = strings.TrimSpace(idPart)

	// Remove timestamps like (1234.567890)
	if idx := strings.LastIndex(idPart, ")"); idx != -1 {
		idPart = idPart[idx+1:]
	}
	idPart = strings.TrimSpace(idPart)

	// Remove interface name (vcan0, can0, etc.)
	if idx := strings.LastIndex(idPart, " "); idx != -1 {
		idPart = idPart[idx+1:]
	}

	canID := strings.TrimSpace(idPart)

	// Extract and decode payload (everything after #)
	payloadHex := line[idxHash+1:]
	payloadHex = strings.ReplaceAll(payloadHex, " ", "")

	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return nil, err
	}

	return &CANFrame{
		ID:         canID,
		IsExtended: len(canID) > 3,
		Data:       payload,
		Length:     len(payload),
	}, nil
}

func main() {
	// Buffer to accumulate CBOR data from multiple CAN frames
	var cborBuffer []byte
	var lastCanID string
	var frameCount int

	// Main Loop: Read Stdin
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("VanMoof CAN Bus Decoder")
	fmt.Println("Supports: CSV format (SavvyCAN) and candump format")
	fmt.Println("Protocol: Ax = Start Frame, 1x = Continuation")
	fmt.Println("---------------------------------------------------")

	isCSV := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var frame *CANFrame
		var err error

		// Detect format on first data line
		if lineNum == 1 {
			// Check if this looks like a CSV header
			if strings.Contains(line, "Time Stamp") || strings.Contains(line, "ID,Extended") {
				isCSV = true
				fmt.Println("📄 Detected CSV format")
				continue // Skip header
			} else if strings.Contains(line, "#") {
				isCSV = false
				fmt.Println("📄 Detected candump format")
			}
		}

		// Parse based on format
		if isCSV || strings.Contains(line, ",") && !strings.Contains(line, "#") {
			isCSV = true
			// Parse as CSV - split by comma
			fields := strings.Split(line, ",")
			frame, err = parseCSVLine(fields)
			if err != nil {
				continue
			}
		} else {
			// Parse as candump
			frame, err = parseCandumpLine(line)
			if err != nil {
				continue
			}
		}

		if frame == nil || len(frame.Data) == 0 {
			continue
		}

		// Extract header byte and payload
		header := frame.Data[0]
		payload := frame.Data[1:]

		// VanMoof Protocol Analysis:
		// Header byte high nibble indicates frame type:
		// - 0x8x/0x9x = Possibly data frames (seen in your CSV)
		// - 0xAx (e.g., A2) = Start of new CBOR message
		// - 0x1x (e.g., 11) = Continuation frame
		// - 0x0x = Could be status/heartbeat
		isStartFrame := (header & 0xF0) == 0xA0
		isContinuation := (header & 0xF0) == 0x10

		// Display frame info
		idType := "Std"
		if frame.IsExtended {
			idType = "Ext"
		}

		frameType := "DATA"
		if isStartFrame {
			frameType = "START"
		} else if isContinuation {
			frameType = "CONT"
		} else if header == 0x00 {
			frameType = "ZERO"
		}

		fmt.Printf("📍 ID:0x%s(%s) Hdr:%02X [%s] Data[%d]: %X\n",
			frame.ID, idType, header, frameType, len(frame.Data), frame.Data)

		// --- VANMOOF FRAMING LOGIC ---
		if isStartFrame {
			// New message starting - reset buffer
			if len(cborBuffer) > 0 {
				fmt.Printf("   ⚠️ Discarding incomplete buffer (%d bytes): %X\n",
					len(cborBuffer), cborBuffer)
			}
			cborBuffer = make([]byte, 0)
			cborBuffer = append(cborBuffer, payload...)
			lastCanID = frame.ID
			frameCount = 1
			fmt.Printf("   🆕 New message started, buffer: %X\n", cborBuffer)
		} else if isContinuation {
			// Continuation of current message
			cborBuffer = append(cborBuffer, payload...)
			frameCount++
			fmt.Printf("   ➕ Frame %d appended, buffer now: %X (%d bytes)\n",
				frameCount, cborBuffer, len(cborBuffer))
		} else {
			// Not CBOR framing - analyze as raw data
			analyzeRawFrame(frame)
			continue
		}

		// Try to decode CBOR from the accumulated buffer
		if len(cborBuffer) > 0 {
			bufReader := bytes.NewReader(cborBuffer)
			dec := cbor.NewDecoder(bufReader)

			var item interface{}
			err := dec.Decode(&item)

			if err == nil {
				// Successfully decoded!
				bytesConsumed := len(cborBuffer) - bufReader.Len()

				fmt.Println("\n===================================================")
				fmt.Printf("✅ COMPLETE CBOR MESSAGE (CAN ID: 0x%s, %d frames, %d bytes)\n",
					lastCanID, frameCount, bytesConsumed)
				fmt.Printf("Raw CBOR: %X\n", cborBuffer[:bytesConsumed])
				fmt.Println("---------------------------------------------------")

				// Decode and display the structure
				decodeAndPrint(item, 0)

				fmt.Println("===================================================")

				// Remove consumed bytes
				cborBuffer = cborBuffer[bytesConsumed:]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

// analyzeRawFrame analyzes non-CBOR frames for patterns
func analyzeRawFrame(frame *CANFrame) {
	// Analyze specific CAN IDs for known patterns
	switch {
	case frame.ID == "14609460":
		// This ID appears frequently in your data
		if len(frame.Data) >= 4 {
			fmt.Printf("   📊 Telemetry? Bytes[1:4]: %02X %02X %02X %02X\n",
				frame.Data[0], frame.Data[1], frame.Data[2], frame.Data[3])
		}
	case strings.HasPrefix(frame.ID, "01111"):
		// IDs starting with 01111 seem to be heartbeats (all zeros)
		allZero := true
		for _, b := range frame.Data {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			fmt.Printf("   💓 Heartbeat/Keep-alive (all zeros)\n")
		}
	case frame.ID == "18209820":
		if len(frame.Data) >= 1 {
			fmt.Printf("   🔢 Status byte: %02X\n", frame.Data[0])
		}
	}
}

// decodeAndPrint recursively prints CBOR structures with indentation
func decodeAndPrint(item interface{}, indent int) {
	prefix := strings.Repeat("  ", indent)

	switch v := item.(type) {
	case []uint8:
		fmt.Printf("%sType: Byte String (%d bytes)\n", prefix, len(v))
		fmt.Printf("%sHex: %X\n", prefix, v)
		// ASCII interpretation
		ascii := make([]byte, len(v))
		for i, b := range v {
			if b >= 32 && b < 127 {
				ascii[i] = b
			} else {
				ascii[i] = '.'
			}
		}
		fmt.Printf("%sASCII: %s\n", prefix, string(ascii))

		// VanMoof specific: Check if this could be a nonce/IV (9 bytes)
		if len(v) == 9 {
			fmt.Printf("%s💡 Possible Nonce/IV (9 bytes)\n", prefix)
		}

	case string:
		fmt.Printf("%sType: Text String (%d chars)\n", prefix, len(v))
		fmt.Printf("%sValue: %q\n", prefix, v)
		// Check for binary data disguised as text
		hasBinary := false
		for _, r := range v {
			if r < 32 || r > 126 {
				hasBinary = true
				break
			}
		}
		if hasBinary {
			fmt.Printf("%s⚠️ Contains non-printable bytes (possibly encrypted data)\n", prefix)
			fmt.Printf("%sRaw Hex: %X\n", prefix, []byte(v))
		}

	case []interface{}:
		fmt.Printf("%sType: Array (length %d)\n", prefix, len(v))
		for i, elem := range v {
			fmt.Printf("%s  [%d]:\n", prefix, i)
			decodeAndPrint(elem, indent+2)
		}

	case map[interface{}]interface{}:
		fmt.Printf("%sType: Map (%d entries)\n", prefix, len(v))
		for k, val := range v {
			fmt.Printf("%s  Key: %v\n", prefix, k)
			fmt.Printf("%s  Value:\n", prefix)
			decodeAndPrint(val, indent+2)
		}

	case uint64:
		fmt.Printf("%sType: Unsigned Int\n", prefix)
		fmt.Printf("%sValue: %d (0x%X)\n", prefix, v, v)

	case int64:
		fmt.Printf("%sType: Signed Int\n", prefix)
		fmt.Printf("%sValue: %d\n", prefix, v)

	case bool:
		fmt.Printf("%sType: Boolean\n", prefix)
		fmt.Printf("%sValue: %v\n", prefix, v)

	case nil:
		fmt.Printf("%sType: Null\n", prefix)

	default:
		fmt.Printf("%sType: %T\n", prefix, v)
		fmt.Printf("%sValue: %v\n", prefix, v)
	}
}
