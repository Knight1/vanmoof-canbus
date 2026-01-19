package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// printFrameHeader prints a formatted frame header with metadata
func printFrameHeader(frame *CANFrame, header byte, frameType string) {
	idType := "Std"
	if frame.IsExtended {
		idType = "Ext"
	}
	fmt.Printf("📍 ID:0x%s(%s) Hdr:%02X [%s] Data[%d]: %X\n",
		frame.ID, idType, header, frameType, len(frame.Data), frame.Data)
}

// displayGroupedFrames displays frames grouped by CAN ID and sorted by timestamp
func displayGroupedFrames(frames []*FrameInfo, hideAccounted, hideUnaccounted bool) {
	// Group frames by CAN ID
	grouped := make(map[string][]*FrameInfo)
	for _, f := range frames {
		grouped[f.Frame.ID] = append(grouped[f.Frame.ID], f)
	}

	// Get sorted list of IDs
	var ids []string
	for id := range grouped {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	fmt.Println("\n===================================================")
	fmt.Println("📋 FRAMES GROUPED BY CAN ID")
	fmt.Println("===================================================")

	for _, id := range ids {
		frameList := grouped[id]

		// Sort by timestamp within each group
		sort.Slice(frameList, func(i, j int) bool {
			if frameList[i].TimestampFloat == frameList[j].TimestampFloat {
				// If timestamps are equal, sort by sequence number to maintain order
				return frameList[i].SequenceNum < frameList[j].SequenceNum
			}
			return frameList[i].TimestampFloat < frameList[j].TimestampFloat
		})

		// Filter frames based on flags
		var filteredFrames []*FrameInfo
		for _, f := range frameList {
			if hideAccounted && (f.IsCBOR || f.IsHeartbeat) {
				continue
			}
			if hideUnaccounted && !f.IsCBOR && !f.IsHeartbeat {
				continue
			}
			filteredFrames = append(filteredFrames, f)
		}

		if len(filteredFrames) == 0 {
			continue
		}

		fmt.Printf("\n🔖 CAN ID: 0x%s (%d frames)\n", id, len(filteredFrames))
		fmt.Println(strings.Repeat("-", 60))

		for _, f := range filteredFrames {
			tsStr := strconv.FormatFloat(f.TimestampFloat, 'f', 6, 64)
			fmt.Printf("  [%s #%d] ", tsStr, f.SequenceNum)
			printFrameHeader(f.Frame, f.Header, f.FrameType)
		}
	}

	fmt.Println("\n===================================================")
}
