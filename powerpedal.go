package main

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// power_pedal (PF=SA=0xA2, OD node 0xA2, PS address 0x92) — the pedal-assist /
// torque + cadence sensor sub-ECU. NXP LPC546xx, SA5 v1.5.0-main firmware.
//
// Grounding (firmware + main-module OD registry; see CANBUS.md):
//   - heartbeat              0x01111440  (device-encoded 0x440 = (0xA2&0x7F)<<5)
//   - OD telemetry publish   id = 0x14401040 | (a1 << 13)   on node 0xA2
//   - power_switch_control   a1 = 0x0A  ->  0x14415040  (the only named power_pedal
//     OD signal documented by the i.MX8 `vm` registry; polled,
//     1 byte). Verified: 0x14401040 | (0x0A<<13) == 0x14415040.
//
// NOTE ON torque / cadence (pedal_rpm): the specific OD signal a1 indices for
// torque and cadence are NOT documented and NOT present in the available bus
// captures (every dump was taken with the bike locked, i.e. no pedaling). The
// sub-ECU firmware stores its signal names off-image (>0x6f4f) and registers its
// tasks via xTaskCreate; the semantic name->signal map lives in the i.MX8 `ride`
// service, which is not in the analysis set. Rather than fabricate CAN IDs, the
// faker emits a protocol-correct OD frame for an a1 index YOU supply (see
// PowerPedalFakeSignal). Once the torque/cadence indices are pinned (from the
// `ride` ELF or a capture taken while pedaling) they can be added as named aliases.
const (
	PowerPedalHeartbeatID = 0x01111440 // power_pedal heartbeat (MCU alive)
	PowerPedalNode        = 0xA2       // OD node id (a0)
	PowerPedalSelfID      = 0x14403440 // power_pedal "self" OD identity (class 0x3)

	// PowerPedalSwitchA1 is the only named power_pedal OD signal in the i.MX8
	// `vm` registry: power_pedal_power_switch_control_init (polled, 1 byte).
	PowerPedalSwitchA1 = 0x0A
	PowerPedalSwitchID = 0x14415040 // == PowerPedalODPublishID(0x0A)
)

// PowerPedalODPublishID returns the on-wire 29-bit CAN ID for an OD telemetry
// signal published by power_pedal (node 0xA2) at signal index a1. Derived from
// the documented switch signal (a1=0x0A -> 0x14415040) and the battery OD
// telemetry IDs, all of which fit id = 0x10000000 | (a0<<21) | (a1<<13) | 0x1040
// with a0=0xA2, port a2=0x82 (the i.MX8 power port).
func PowerPedalODPublishID(a1 byte) uint32 {
	return 0x14401040 | (uint32(a1) << 13)
}

// IsPowerPedalHeartbeat reports whether the CAN ID is the power_pedal heartbeat.
func IsPowerPedalHeartbeat(canIDHex string) bool {
	return strings.EqualFold(canIDHex, "01111440")
}

// IsPowerPedalODFrame reports whether the CAN ID is an OD frame on node 0xA2
// (power_pedal as the source node) and returns the a1 signal index if so.
func IsPowerPedalODFrame(canIDHex string) (a1 byte, ok bool) {
	a0, sig, parsed := odFields(canIDHex)
	if !parsed || a0 != PowerPedalNode {
		return 0, false
	}
	return sig, true
}

// RunPowerPedalCheck attaches to the CAN device (or reads a piped capture) and
// reports power_pedal bench health.
//
// Observable signals (per the firmware + main-module OD registry):
//   - heartbeat 0x01111440 ............ the MCU is alive
//   - any OD frame on node 0xA2 ....... the device is publishing on the bus
//   - switch_control 0x14415040 (a1=0x0A) the documented polled switch signal
//
// power_pedal exposes no documented dedicated fault code, so the verdict is OK
// once it is alive and publishing.
func RunPowerPedalCheck(iface string, dur time.Duration) {
	var sawHeartbeat, sawOD, sawSwitch bool
	seenSignals := map[byte]bool{}

	fmt.Println("Power-pedal bench health check")
	readFrames(iface, dur, func(f *CANFrame) {
		if IsPowerPedalHeartbeat(f.ID) {
			if !sawHeartbeat {
				fmt.Println("  [alive]  power_pedal heartbeat (0x01111440)")
			}
			sawHeartbeat = true
			return
		}
		if a1, ok := IsPowerPedalODFrame(f.ID); ok {
			sawOD = true
			if a1 == PowerPedalSwitchA1 {
				sawSwitch = true
			}
			if !seenSignals[a1] {
				seenSignals[a1] = true
				fmt.Printf("  [signal] %s#%X\n", strings.ToUpper(f.ID), f.Data)
				if od := FormatODAddress(f.ID); od != "" {
					fmt.Printf("           %s\n", od)
				}
			}
		}
	})

	fmt.Println(strings.Repeat("-", 50))
	if !sawHeartbeat && !sawOD {
		fmt.Println("VERDICT: NO SIGNAL — no power_pedal frames seen.")
		fmt.Println("         Check power, CAN wiring and 120-ohm bus termination.")
		return
	}
	fmt.Printf("Alive (heartbeat): %v\n", sawHeartbeat)
	fmt.Printf("Publishing OD:     %v (%d distinct signal index/es)\n", sawOD, len(seenSignals))
	fmt.Printf("Switch signal:     %v\n", sawSwitch)
	switch {
	case sawHeartbeat && sawOD:
		fmt.Println("VERDICT: OK — power_pedal is alive and publishing OD telemetry.")
	case sawHeartbeat:
		fmt.Println("VERDICT: PARTIAL — heartbeat seen but no OD telemetry; the pedal may be idle")
		fmt.Println("         (no pedaling) or not finished init.")
	default:
		fmt.Println("VERDICT: PARTIAL — OD frames seen but no heartbeat; verify the device.")
	}
}

// PowerPedalFakeSignal prints a cansend command that injects (spoofs) an OD
// telemetry frame as if published by power_pedal (node 0xA2) at signal index a1
// with the given payload. This is the grounded, generic path for faking torque,
// cadence/rpm and any other power_pedal signal: the on-wire ID is computed from
// the verified OD-publish format, but the a1 index and payload are supplied by
// the caller (not fabricated). data may be nil/empty for a zero-length frame.
func PowerPedalFakeSignal(a1 byte, data []byte, iface, label string) {
	id := PowerPedalODPublishID(a1)
	frame := EshifterCANFrame{
		CANID: fmt.Sprintf("%08X", id),
		Data:  data,
		Desc:  label,
	}

	fmt.Printf("Power-pedal: fake OD signal a1=0x%02X (%s)\n", a1, label)
	fmt.Println(strings.Repeat("-", 50))
	if a1 == PowerPedalSwitchA1 {
		fmt.Println("# a1=0x0A is the documented power_switch_control_init signal.")
	} else {
		fmt.Println("# NOTE: this a1 index is caller-supplied. torque/cadence indices are not")
		fmt.Println("#       documented or captured — confirm the index before trusting the result.")
	}
	fmt.Printf("# %s\n", frame.Desc)
	fmt.Println(frame.FormatCansend(iface))
	fmt.Println()
	fmt.Printf("# Watch the bus for the echo / downstream reaction:\n")
	fmt.Printf("# candump %s,%08X:1FFFFFFF\n", iface, id)
}

// PowerPedalFakeSwitch is the named convenience for the one documented
// power_pedal signal: power_switch_control_init (a1=0x0A), a 1-byte value.
func PowerPedalFakeSwitch(value byte, iface string) {
	PowerPedalFakeSignal(PowerPedalSwitchA1, []byte{value},
		iface, fmt.Sprintf("power_switch_control_init = 0x%02X", value))
}

// ParseHexPayload parses a hex string ("01F4", "01 F4", "0x01F4") into bytes.
func ParseHexPayload(s string) ([]byte, error) {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "0x")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return nil, nil
	}
	return hex.DecodeString(s)
}
