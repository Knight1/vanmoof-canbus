package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

func main() {
	unaccountedOnly := flag.Bool("unaccounted-only", false, "only display frames that are not CBOR or heartbeat/keep-alive")
	hideUnaccounted := flag.Bool("hide-unaccounted", false, "hide unaccounted frames, show only decoded CBOR and heartbeat frames")
	hideAccounted := flag.Bool("hide-accounted", false, "hide accounted frames (CBOR and heartbeat), show only unaccounted frames")
	groupByID := flag.Bool("group-by-id", false, "group frames by CAN ID, then sort by timestamp within each group")
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
	var allFrames []*FrameInfo // For grouping mode

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
		} else if *hideAccounted {
			return "Unaccounted frames only (hiding accounted)"
		}
		if *groupByID {
			return "All frames (grouped by ID)"
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
		var timestampFloat float64
		if frame.Timestamp != "" {
			if ts, err := strconv.ParseFloat(frame.Timestamp, 64); err == nil {
				// CSV timestamps are in microseconds, convert to seconds
				if isCSV {
					timestampFloat = ts / 1_000_000
				} else {
					timestampFloat = ts
				}
				if timestampFloat < minTimestamp {
					minTimestamp = timestampFloat
				}
				if timestampFloat > maxTimestamp {
					maxTimestamp = timestampFloat
				}
				if !captureStarted {
					captureStarted = true
				}
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

		// Determine frame type
		var frameType string
		var isCBOR bool
		var isHeartbeat bool

		if isStartFrame {
			frameType = "START"
			isCBOR = true
		} else if isContinuation {
			frameType = "CONT"
			isCBOR = true
		} else {
			// Check if heartbeat
			isHeartbeat = checkIfHeartbeat(frame)
			if isHeartbeat {
				frameType = "HEARTBEAT"
			} else {
				frameType = "UNACCOUNTED"
			}
		}

		// Store frame info if grouping
		if *groupByID {
			allFrames = append(allFrames, &FrameInfo{
				Frame:          frame,
				TimestampFloat: timestampFloat,
				Header:         header,
				FrameType:      frameType,
				IsHeartbeat:    isHeartbeat,
				IsCBOR:         isCBOR,
				SequenceNum:    totalFramesProcessed,
			})
		}

		// Skip immediate display if grouping
		if *groupByID {
			// Still need to process CBOR for accurate counts
			if isStartFrame {
				cborBuffer = make([]byte, 0)
				cborBuffer = append(cborBuffer, payload...)
				lastCanID = frame.ID
				frameCount = 1
			} else if isContinuation {
				cborBuffer = append(cborBuffer, payload...)
				frameCount++
			}

			if isCBOR && len(cborBuffer) > 0 {
				bufReader := bytes.NewReader(cborBuffer)
				dec := cbor.NewDecoder(bufReader)
				var item interface{}
				if dec.Decode(&item) == nil {
					cborMessageCount++
					cborBuffer = cborBuffer[len(cborBuffer)-bufReader.Len():]
				}
			}

			if isHeartbeat {
				heartbeatCount++
			}
			continue
		}

		// --- VANMOOF FRAMING LOGIC ---
		if isStartFrame {
			if !*unaccountedOnly && !*hideAccounted {
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
			if !*unaccountedOnly && !*hideAccounted {
				printFrameHeader(frame, header, "CONT")
			}
			// Continuation of current message
			cborBuffer = append(cborBuffer, payload...)
			frameCount++
			fmt.Printf("   ➕ Frame %d appended, buffer now: %X (%d bytes)\n",
				frameCount, cborBuffer, len(cborBuffer))
		} else {
			// Not CBOR framing - analyze as raw data
			showVerbose := !*unaccountedOnly && !*hideUnaccounted && !*hideAccounted
			isHeartbeat := analyzeRawFrame(frame, showVerbose)
			if !isHeartbeat && (*unaccountedOnly || *hideAccounted) {
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

	// Display grouped output if requested
	if *groupByID && len(allFrames) > 0 {
		displayGroupedFrames(allFrames, *hideAccounted, *hideUnaccounted)
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
