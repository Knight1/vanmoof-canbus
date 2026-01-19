package main

import (
	"fmt"
	"strings"
)

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
