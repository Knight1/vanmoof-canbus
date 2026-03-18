package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// printFrameHeader prints a formatted frame header with metadata.
// For UNACCOUNTED frames, appends inline CBOR decode if the payload is valid CBOR.
func printFrameHeader(frame *CANFrame, header byte, frameType string) {
	idType := "Std"
	if frame.IsExtended {
		idType = "Ext"
	}

	// Add protocol annotations if available
	info := AnalyzeWireCANID(frame.ID)
	annotation := ""
	if len(info.Annotations) > 0 {
		annotation = " {" + strings.Join(info.Annotations, ", ") + "}"
	}

	// For non-framed messages, try protocol decoders first, then CBOR
	extra := ""
	if frameType == "UNACCOUNTED" {
		if desc := FormatEshifterFrame(frame.ID, frame.Data); desc != "" {
			extra = " -> " + desc
		} else if desc := FormatLightFrame(frame.ID, frame.Data); desc != "" {
			extra = " -> " + desc
		} else if item, ok := tryRawCBORDecode(frame.Data); ok {
			extra = " -> CBOR: " + formatCBORInline(item)
		}
	}

	fmt.Printf("ID:0x%s(%s) Hdr:%02X [%s] Data[%d]: %X%s%s\n",
		frame.ID, idType, header, frameType, len(frame.Data), frame.Data, annotation, extra)
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
	fmt.Println("FRAMES GROUPED BY CAN ID")
	fmt.Println("===================================================")

	for _, id := range ids {
		frameList := grouped[id]

		// Sort by timestamp within each group
		sort.Slice(frameList, func(i, j int) bool {
			if frameList[i].TimestampFloat == frameList[j].TimestampFloat {
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

		// Add protocol annotation to group header
		info := AnalyzeWireCANID(id)
		annotation := ""
		if len(info.Annotations) > 0 {
			annotation = " [" + strings.Join(info.Annotations, ", ") + "]"
		}

		fmt.Printf("\nCAN ID: 0x%s (%d frames)%s\n", id, len(filteredFrames), annotation)
		fmt.Println(strings.Repeat("-", 60))

		for _, f := range filteredFrames {
			tsStr := strconv.FormatFloat(f.TimestampFloat, 'f', 6, 64)
			fmt.Printf("  [%s #%d] ", tsStr, f.SequenceNum)
			printFrameHeader(f.Frame, f.Header, f.FrameType)
			// Show ASCII for unaccounted frames with printable content
			if f.FrameType == "UNACCOUNTED" && hasPrintableASCII(f.Frame.Data, 3) {
				fmt.Printf("      ASCII: %s\n", asciiInterpretation(f.Frame.Data))
			}
		}
	}

	fmt.Println("\n===================================================")
}
