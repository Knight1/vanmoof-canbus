package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

// tryRawCBORDecode attempts to decode the full frame data as a single CBOR item.
// Returns the decoded value and true only if the entire payload forms valid CBOR.
func tryRawCBORDecode(data []byte) (interface{}, bool) {
	if len(data) == 0 {
		return nil, false
	}
	reader := bytes.NewReader(data)
	dec := cbor.NewDecoder(reader)
	var item interface{}
	if err := dec.Decode(&item); err != nil {
		return nil, false
	}
	// Only count as valid if entire payload was consumed (no trailing bytes)
	if reader.Len() > 0 {
		return nil, false
	}
	return item, true
}

// formatCBORInline returns a compact single-line representation of a decoded CBOR value.
func formatCBORInline(item interface{}) string {
	switch v := item.(type) {
	case uint64:
		return fmt.Sprintf("%d (0x%X)", v, v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return "null"
	case string:
		return fmt.Sprintf("%q", v)
	case []uint8:
		if len(v) <= 16 {
			return fmt.Sprintf("bytes(%d): %X", len(v), v)
		}
		return fmt.Sprintf("bytes(%d): %X...", len(v), v[:16])
	case []interface{}:
		if len(v) <= 4 {
			parts := make([]string, len(v))
			for i, elem := range v {
				parts[i] = formatCBORInline(elem)
			}
			return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
		}
		return fmt.Sprintf("array(%d items)", len(v))
	case map[interface{}]interface{}:
		if len(v) <= 2 {
			parts := make([]string, 0, len(v))
			for k, val := range v {
				parts = append(parts, fmt.Sprintf("%v: %s", k, formatCBORInline(val)))
			}
			return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
		}
		return fmt.Sprintf("map(%d entries)", len(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// asciiInterpretation returns a printable ASCII representation of data bytes.
func asciiInterpretation(data []byte) string {
	ascii := make([]byte, len(data))
	for i, b := range data {
		if b >= 32 && b < 127 {
			ascii[i] = b
		} else {
			ascii[i] = '.'
		}
	}
	return string(ascii)
}

// hasPrintableASCII returns true if data contains at least minChars printable ASCII characters.
func hasPrintableASCII(data []byte, minChars int) bool {
	count := 0
	for _, b := range data {
		if b >= 32 && b < 127 {
			count++
			if count >= minChars {
				return true
			}
		}
	}
	return false
}

// analyzeRawFrame analyzes non-CBOR-framed frames and returns true if heartbeat.
// For non-heartbeat frames, attempts CBOR decode of the full frame data and shows
// ASCII interpretation when meaningful.
func analyzeRawFrame(frame *CANFrame, verbose bool) bool {
	if checkIfHeartbeat(frame) {
		if verbose {
			info := AnalyzeWireCANID(frame.ID)
			srcName := "unknown"
			if info.Source != nil {
				srcName = info.Source.Name
			}
			fmt.Printf("   Heartbeat from %s (all zeros)\n", srcName)
		}
		return true
	}

	if !verbose {
		return false
	}

	// Protocol annotation
	info := AnalyzeWireCANID(frame.ID)
	if len(info.Annotations) > 0 {
		fmt.Printf("   Protocol: %s\n", strings.Join(info.Annotations, ", "))
	}

	// Try CBOR decode of full frame data
	if item, ok := tryRawCBORDecode(frame.Data); ok {
		fmt.Printf("   CBOR: %s\n", formatCBORInline(item))
	}

	// ASCII interpretation (only show if enough printable chars)
	if hasPrintableASCII(frame.Data, 3) {
		fmt.Printf("   ASCII: %s\n", asciiInterpretation(frame.Data))
	}

	return false
}

// checkIfHeartbeat checks if a frame is a heartbeat/keep-alive frame
func checkIfHeartbeat(frame *CANFrame) bool {
	info := AnalyzeWireCANID(frame.ID)
	if !info.IsHeartbeat {
		return false
	}
	for _, b := range frame.Data {
		if b != 0 {
			return false
		}
	}
	return true
}
