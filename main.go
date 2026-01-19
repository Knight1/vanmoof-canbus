package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

const Version = "0.1.0"

func main() {
	version := flag.Bool("version", false, "show version information")
	unaccountedOnly := flag.Bool("unaccounted-only", false, "only display frames that are not CBOR or heartbeat/keep-alive")
	hideUnaccounted := flag.Bool("hide-unaccounted", false, "hide unaccounted frames, show only decoded CBOR and heartbeat frames")
	hideAccounted := flag.Bool("hide-accounted", false, "hide accounted frames (CBOR and heartbeat), show only unaccounted frames")
	groupByID := flag.Bool("group-by-id", false, "group frames by CAN ID, then sort by timestamp within each group")
	compareMode := flag.Bool("compare", false, "compare unaccounted frames across multiple files (provide file paths as arguments)")
	flag.Parse()

	if *version {
		fmt.Println("vanmoof-canbus version", Version)
		fmt.Printf("OS: %s, Arch: %s, Go: %s, CPUs: %d, Compiler: %s\n", runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumCPU(), runtime.Compiler)
		return
	}

	// Compare mode: process multiple files
	if *compareMode {
		files := flag.Args()
		if len(files) < 2 {
			fmt.Println("Error: Compare mode requires at least 2 file paths as arguments")
			fmt.Println("Usage: canbus -compare file1.csv file2.csv [file3.csv file4.csv ...]")
			os.Exit(1)
		}
		compareFiles(files)
		return
	}

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

// compareFiles processes multiple files and compares their unaccounted frames
func compareFiles(filePaths []string) {
	fileFrames := make(map[string][]*FrameInfo)

	for _, filePath := range filePaths {
		fmt.Printf("Processing %s...\n", filePath)
		frames := processFile(filePath)
		fileFrames[filePath] = frames
	}

	CompareUnaccountedFrames(fileFrames)
}

// processFile reads a file and returns all frame info
func processFile(filePath string) []*FrameInfo {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening %s: %v", filePath, err)
		return nil
	}
	defer file.Close()

	var allFrames []*FrameInfo
	var isCSV bool
	lineNum := 0
	sequenceNum := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if strings.TrimSpace(line) == "" {
			continue
		}

		var frame *CANFrame
		var parseErr error

		// Detect format on first data line
		if lineNum == 1 {
			if strings.Contains(line, "Time Stamp") || strings.Contains(line, "ID,Extended") {
				isCSV = true
				continue
			} else if strings.Contains(line, "#") {
				isCSV = false
			}
		}

		// Parse based on format
		if isCSV || strings.Contains(line, ",") && !strings.Contains(line, "#") {
			isCSV = true
			fields := strings.Split(line, ",")
			frame, parseErr = parseCSVLine(fields)
		} else {
			frame, parseErr = parseCandumpLine(line)
		}

		if parseErr != nil || frame == nil || len(frame.Data) == 0 {
			continue
		}

		sequenceNum++

		// Parse timestamp
		var timestampFloat float64
		if frame.Timestamp != "" {
			if ts, err := strconv.ParseFloat(frame.Timestamp, 64); err == nil {
				if isCSV {
					timestampFloat = ts / 1_000_000
				} else {
					timestampFloat = ts
				}
			}
		}

		// Extract header and determine frame type
		header := frame.Data[0]
		isStartFrame := (header & 0xF0) == 0xA0
		isContinuation := (header & 0xF0) == 0x10

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
			isHeartbeat = checkIfHeartbeat(frame)
			if isHeartbeat {
				frameType = "HEARTBEAT"
			} else {
				frameType = "UNACCOUNTED"
			}
		}

		allFrames = append(allFrames, &FrameInfo{
			Frame:          frame,
			TimestampFloat: timestampFloat,
			Header:         header,
			FrameType:      frameType,
			IsHeartbeat:    isHeartbeat,
			IsCBOR:         isCBOR,
			SequenceNum:    sequenceNum,
		})
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading %s: %v", filePath, err)
	}

	return allFrames
}
