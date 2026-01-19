package main

import (
	"fmt"
	"sort"
	"strings"
)

// UnaccountedFrame represents a unique unaccounted frame pattern
type UnaccountedFrame struct {
	ID          string
	Header      byte
	DataHex     string
	Occurrences map[string]int // filename -> count
}

// CompareUnaccountedFrames compares unaccounted frames across multiple files
func CompareUnaccountedFrames(fileFrames map[string][]*FrameInfo) {
	// Collect all unique unaccounted frame patterns
	framePatterns := make(map[string]*UnaccountedFrame)

	for filename, frames := range fileFrames {
		for _, f := range frames {
			if f.IsCBOR || f.IsHeartbeat {
				continue
			}

			// Create unique key from ID + header + data
			dataHex := fmt.Sprintf("%X", f.Frame.Data)
			key := fmt.Sprintf("%s:%02X:%s", f.Frame.ID, f.Header, dataHex)

			if _, exists := framePatterns[key]; !exists {
				framePatterns[key] = &UnaccountedFrame{
					ID:          f.Frame.ID,
					Header:      f.Header,
					DataHex:     dataHex,
					Occurrences: make(map[string]int),
				}
			}
			framePatterns[key].Occurrences[filename]++
		}
	}

	// Get sorted list of filenames for consistent output
	var filenames []string
	for fn := range fileFrames {
		filenames = append(filenames, fn)
	}
	sort.Strings(filenames)

	// Display comparison results
	fmt.Println("\n===================================================")
	fmt.Println("📊 UNACCOUNTED FRAMES COMPARISON")
	fmt.Println("===================================================")

	// Show file summary
	fmt.Println("Files analyzed:")
	for i, fn := range filenames {
		totalFrames := len(fileFrames[fn])
		unaccountedCount := 0
		for _, f := range fileFrames[fn] {
			if !f.IsCBOR && !f.IsHeartbeat {
				unaccountedCount++
			}
		}
		fmt.Printf("  [%d] %s (%d unaccounted / %d total frames)\n", i+1, fn, unaccountedCount, totalFrames)
	}

	// Categorize frames
	commonToAll := make([]*UnaccountedFrame, 0)
	uniqueFrames := make(map[string][]*UnaccountedFrame)
	partialFrames := make([]*UnaccountedFrame, 0)

	for _, pattern := range framePatterns {
		if len(pattern.Occurrences) == len(filenames) {
			commonToAll = append(commonToAll, pattern)
		} else if len(pattern.Occurrences) == 1 {
			for fn := range pattern.Occurrences {
				uniqueFrames[fn] = append(uniqueFrames[fn], pattern)
			}
		} else {
			partialFrames = append(partialFrames, pattern)
		}
	}

	// Display common frames
	if len(commonToAll) > 0 {
		fmt.Printf("\n🔗 Frames Common to ALL Files (%d patterns):\n", len(commonToAll))
		fmt.Println(strings.Repeat("-", 70))
		sortFramesByID(commonToAll)
		for _, p := range commonToAll {
			counts := make([]string, len(filenames))
			for i, fn := range filenames {
				counts[i] = fmt.Sprintf("%d", p.Occurrences[fn])
			}
			fmt.Printf("  ID:0x%s Hdr:%02X Data:%s\n", p.ID, p.Header, p.DataHex)
			fmt.Printf("    Occurrences: [%s]\n", strings.Join(counts, ", "))
		}
	}

	// Display unique frames per file
	for _, fn := range filenames {
		frames := uniqueFrames[fn]
		if len(frames) > 0 {
			fmt.Printf("\n🔸 Frames UNIQUE to %s (%d patterns):\n", fn, len(frames))
			fmt.Println(strings.Repeat("-", 70))
			sortFramesByID(frames)
			for _, p := range frames {
				fmt.Printf("  ID:0x%s Hdr:%02X Data:%s (count: %d)\n",
					p.ID, p.Header, p.DataHex, p.Occurrences[fn])
			}
		}
	}

	// Display partial matches
	if len(partialFrames) > 0 {
		fmt.Printf("\n🔀 Frames in SOME Files (%d patterns):\n", len(partialFrames))
		fmt.Println(strings.Repeat("-", 70))
		sortFramesByID(partialFrames)
		for _, p := range partialFrames {
			presentIn := make([]string, 0)
			for i, fn := range filenames {
				if count, ok := p.Occurrences[fn]; ok {
					presentIn = append(presentIn, fmt.Sprintf("[%d]:%d", i+1, count))
				}
			}
			fmt.Printf("  ID:0x%s Hdr:%02X Data:%s\n", p.ID, p.Header, p.DataHex)
			fmt.Printf("    Present in: %s\n", strings.Join(presentIn, ", "))
		}
	}

	// Summary statistics
	fmt.Println("\n===================================================")
	fmt.Printf("📈 Summary:\n")
	fmt.Printf("   Total unique patterns: %d\n", len(framePatterns))
	fmt.Printf("   Common to all files: %d\n", len(commonToAll))
	fmt.Printf("   Unique to one file: %d\n", countUniqueFrames(uniqueFrames))
	fmt.Printf("   In some files: %d\n", len(partialFrames))
	fmt.Println("===================================================")
}

func sortFramesByID(frames []*UnaccountedFrame) {
	sort.Slice(frames, func(i, j int) bool {
		if frames[i].ID == frames[j].ID {
			if frames[i].Header == frames[j].Header {
				return frames[i].DataHex < frames[j].DataHex
			}
			return frames[i].Header < frames[j].Header
		}
		return frames[i].ID < frames[j].ID
	})
}

func countUniqueFrames(uniqueFrames map[string][]*UnaccountedFrame) int {
	total := 0
	for _, frames := range uniqueFrames {
		total += len(frames)
	}
	return total
}
