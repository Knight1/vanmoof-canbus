package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

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
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing %s: %v", filePath, err)
		}
	}()

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

		// Skip standard 11-bit CAN IDs
		if !frame.IsExtended {
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
