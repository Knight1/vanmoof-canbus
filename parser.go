package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

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

// parseTimestamp extracts the numeric timestamp from a line
func parseTimestamp(line string, isCSV bool) (float64, error) {
	if isCSV {
		// CSV format: first field is timestamp
		fields := strings.Split(line, ",")
		if len(fields) > 0 {
			return strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
		}
	} else {
		// candump format: (timestamp) ...
		start := strings.Index(line, "(")
		end := strings.Index(line, ")")
		if start != -1 && end != -1 && start < end {
			tsStr := line[start+1 : end]
			return strconv.ParseFloat(tsStr, 64)
		}
	}
	return 0, fmt.Errorf("could not parse timestamp")
}

// formatDuration formats milliseconds to a human-readable string
func formatDuration(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.2f ms", ms)
	}

	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.2f sec", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		secs := int(seconds) % 60
		return fmt.Sprintf("%d min %d sec", int(minutes), secs)
	}

	hours := minutes / 60
	mins := int(minutes) % 60
	return fmt.Sprintf("%d hour %d min", int(hours), mins)
}
