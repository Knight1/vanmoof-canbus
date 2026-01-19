package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
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

func main() {
	unaccountedOnly := flag.Bool("unaccounted-only", false, "only display frames that are not CBOR or heartbeat/keep-alive")
	hideUnaccounted := flag.Bool("hide-unaccounted", false, "hide unaccounted frames, show only decoded CBOR and heartbeat frames")
	flag.Parse()

	// Buffer to accumulate CBOR data from multiple CAN frames
	var cborBuffer []byte
	var lastCanID string
	var frameCount int
	var minTimestamp float64 = float64(^uint64(0) >> 1) // Max float
	var maxTimestamp float64 = 0
	var isCSV bool
	var captureStarted bool
	var cborMessageCount int
	var heartbeatCount int
	var totalFramesProcessed int

	// Main Loop: Read Stdin
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("VanMoof CAN Bus Decoder")
	fmt.Println("Supports: CSV format (SavvyCAN) and candump format")
	fmt.Println("Protocol: Ax = Start Frame, 1x = Continuation")
	fmt.Printf("Mode: %s\n", func() string {
		if *unaccountedOnly {
			return "Unaccounted frames only"
		} else if *hideUnaccounted {
			return "Decoded frames only (hiding unaccounted)"
		}
		return "All frames"
	}())
	fmt.Println("---------------------------------------------------")

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

		totalFramesProcessed++

		// Track capture timestamps
		if ts, err := parseTimestamp(line, isCSV); err == nil {
			if ts < minTimestamp {
				minTimestamp = ts
			}
			if ts > maxTimestamp {
				maxTimestamp = ts
			}
			if !captureStarted {
				captureStarted = true
			}
		}

		// Extract header byte and payload
		header := frame.Data[0]
		payload := frame.Data[1:]

		// VanMoof Protocol Analysis:
		// Header byte high nibble indicates frame type:
		// - 0x8x/0x9x = Possibly data frames
		// - 0xAx (e.g., A2) = Start of new CBOR message
		// - 0x1x (e.g., 11) = Continuation frame
		// - 0x0x = Could be status/heartbeat
		isStartFrame := (header & 0xF0) == 0xA0
		isContinuation := (header & 0xF0) == 0x10

		// --- VANMOOF FRAMING LOGIC ---
		if isStartFrame {
			if !*unaccountedOnly {
				printFrameHeader(frame, header, "START")
			}
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
			if !*unaccountedOnly {
				printFrameHeader(frame, header, "CONT")
			}
			// Continuation of current message
			cborBuffer = append(cborBuffer, payload...)
			frameCount++
			fmt.Printf("   ➕ Frame %d appended, buffer now: %X (%d bytes)\n",
				frameCount, cborBuffer, len(cborBuffer))
		} else {
			// Not CBOR framing - analyze as raw data
			showVerbose := !*unaccountedOnly && !*hideUnaccounted
			isHeartbeat := analyzeRawFrame(frame, showVerbose)
			if !isHeartbeat && *unaccountedOnly {
				printFrameHeader(frame, header, "UNACCOUNTED")
			}
			if isHeartbeat {
				heartbeatCount++
			}
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
				cborMessageCount++

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

	// Display capture summary
	if captureStarted && maxTimestamp > minTimestamp {
		durationSeconds := maxTimestamp - minTimestamp
		readableDuration := formatDuration(durationSeconds)
		fmt.Println("\n===================================================")
		fmt.Printf("📊 Capture Summary\n")
		fmt.Printf("   Duration: %s (%.3f sec)\n", readableDuration, durationSeconds)
		fmt.Printf("   From: %.6f to %.6f seconds\n", minTimestamp, maxTimestamp)
		fmt.Printf("   CBOR Messages Found: %d\n", cborMessageCount)
		fmt.Printf("   Heartbeat/Keep-Alive Frames: %d\n", heartbeatCount)
		unaccountedFrames := totalFramesProcessed - cborMessageCount - heartbeatCount
		if unaccountedFrames < 0 {
			unaccountedFrames = 0
		}
		fmt.Printf("   Unaccounted Frames: %d\n", unaccountedFrames)
		fmt.Printf("   Total Frames Processed: %d\n", totalFramesProcessed)
		fmt.Println("===================================================")
	}
}

// analyzeRawFrame analyzes non-CBOR frames for patterns and returns true if heartbeat detected
func analyzeRawFrame(frame *CANFrame, verbose bool) bool {
	isHeartbeat := false

	// Analyze specific CAN IDs for known patterns
	switch {
	case frame.ID == "14609460":
		// This ID appears frequently
		if verbose && len(frame.Data) >= 4 {
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
			if verbose {
				fmt.Printf("   💓 Heartbeat/Keep-alive (all zeros)\n")
			}
			isHeartbeat = true
		}
	case frame.ID == "18209820":
		if verbose && len(frame.Data) >= 1 {
			fmt.Printf("   🔢 Status byte: %02X\n", frame.Data[0])
		}
	}

	return isHeartbeat
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

func printFrameHeader(frame *CANFrame, header byte, frameType string) {
	idType := "Std"
	if frame.IsExtended {
		idType = "Ext"
	}
	fmt.Printf("📍 ID:0x%s(%s) Hdr:%02X [%s] Data[%d]: %X\n",
		frame.ID, idType, header, frameType, len(frame.Data), frame.Data)
}
