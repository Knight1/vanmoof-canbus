package main

import (
	"fmt"
	"strings"
)

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

// checkIfHeartbeat checks if a frame is a heartbeat/keep-alive frame
func checkIfHeartbeat(frame *CANFrame) bool {
	if !strings.HasPrefix(frame.ID, "01111") {
		return false
	}
	// Check if all bytes are zero
	for _, b := range frame.Data {
		if b != 0 {
			return false
		}
	}
	return true
}
