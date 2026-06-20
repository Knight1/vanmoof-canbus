package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// stdinIsPiped reports whether stdin is a pipe or redirected file (not a tty).
func stdinIsPiped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// readFrames drives frames into handle from the best available source: if a
// capture is piped/redirected into stdin it is parsed; otherwise the tool
// attaches live to the CAN interface for the given duration.
func readFrames(iface string, dur time.Duration, handle func(*CANFrame)) {
	if stdinIsPiped() {
		fmt.Println("Source: piped capture (stdin)")
		scanFrames(handle)
		return
	}
	fmt.Printf("Source: live attach to %s for %s  (Ctrl-C to stop early)\n", iface, dur)
	if err := readFramesLive(iface, dur, handle); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		fmt.Printf("Tip: bring the bus up first:  sudo ip link set %s up type can bitrate 1000000\n", iface)
		fmt.Println("     or pipe a capture instead:  candump -L " + iface + " | canbus <check>")
	}
}

// scanFrames reads a CAN capture (candump or SavvyCAN CSV) from stdin and calls
// handle for every parsed frame. Format is auto-detected on the first data line,
// matching the main decoder loop. Use with a live capture, e.g.:
//
//	timeout 6 candump -L can0 | canbus -elock-check
//
// or a saved dump:  canbus -elock-check < dumps/bikelocked.log
func scanFrames(handle func(*CANFrame)) {
	scanner := bufio.NewScanner(os.Stdin)
	isCSV := false
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		lineNum++
		if lineNum == 1 {
			if strings.Contains(line, "Time Stamp") || strings.Contains(line, "ID,Extended") {
				isCSV = true
				continue // skip CSV header
			} else if strings.Contains(line, "#") {
				isCSV = false
			}
		}

		var frame *CANFrame
		var err error
		if isCSV || (strings.Contains(line, ",") && !strings.Contains(line, "#")) {
			isCSV = true
			frame, err = parseCSVLine(strings.Split(line, ","))
		} else {
			frame, err = parseCandumpLine(line)
		}
		if err != nil || frame == nil {
			continue
		}
		handle(frame)
	}
}

// RunElockCheck reads a CAN capture from stdin and reports elock bench health.
//
// Observable health signals (per the elock firmware + bus captures):
//   - heartbeat 0x01111820 ............ the MCU is alive
//   - lock state 0x18209820 (1 byte) .. UNKNOWN/LOCKED/UNLOCKED/STUCK
//   - STUCK (0x03) .................... fault: the lock barrel failed to move
//     (the lock state machine reports this when the actuator cannot complete)
//   - data report 0x18203110/0x18023110 the lock is publishing telemetry
func RunElockCheck(iface string, dur time.Duration) {
	var sawHeartbeat, sawStatus, sawReport, haveState, stuck bool
	var lastState byte
	printedState := -1 // last lock-state byte printed (dedupe repeats)

	fmt.Println("Elock bench health check")
	readFrames(iface, dur, func(f *CANFrame) {
		switch IsElockFrame(f.ID) {
		case ElockFrameHeartbeat:
			if !sawHeartbeat {
				fmt.Println("  [alive]  elock heartbeat (0x01111820)")
			}
			sawHeartbeat = true
		case ElockFrameStatus:
			st := DecodeElockStatus(f.Data)
			if st != nil {
				sawStatus = true
				haveState = true
				lastState = st.State
				if st.State == LockStateStuck {
					stuck = true
				}
				if int(st.State) != printedState { // print only on change
					printedState = int(st.State)
					fmt.Printf("  [status] %s\n", FormatElockFrame(f.ID, f.Data))
				}
			}
		case ElockFrameDataReport, ElockFrameDataReportCont:
			sawReport = true
		}
	})

	fmt.Println(strings.Repeat("-", 50))
	if !sawHeartbeat && !sawStatus && !sawReport {
		fmt.Println("VERDICT: NO SIGNAL — no elock frames seen.")
		fmt.Println("         Check power, CAN wiring and 120-ohm bus termination.")
		return
	}
	fmt.Printf("Alive (heartbeat): %v\n", sawHeartbeat)
	if haveState {
		fmt.Printf("Lock state:        %s (0x%02X)\n", lockStateName(lastState), lastState)
	} else {
		fmt.Println("Lock state:        (no status frame seen)")
	}
	switch {
	case stuck:
		fmt.Println("VERDICT: FAULT — lock reported STUCK (barrel failed to move).")
		fmt.Println("         Expected on a bench unit with no mechanical load; otherwise a jam/motor fault.")
	case haveState:
		fmt.Println("VERDICT: OK — elock is alive and reporting lock state.")
	default:
		fmt.Println("VERDICT: PARTIAL — frames seen but no lock-state status; verify the device finished init.")
	}
}

// RunEshifterCheck reads a CAN capture from stdin and reports eshifter bench health.
//
// Observable health signals (per the eshifter firmware + bus captures):
//   - heartbeat 0x01111840 ............ the MCU is alive
//   - status report 0x1840B110 ........ current gear (byte 3) + status flags
//   - telemetry 0x10F11841 ............ byte 3 == 0xFF signals an actuator error
func RunEshifterCheck(iface string, dur time.Duration) {
	var sawHeartbeat, sawStatus, haveGear, telErr bool
	var lastGear byte
	printedGear := -1 // last gear printed (dedupe repeats)
	printedTelErr := false

	fmt.Println("Eshifter bench health check")
	readFrames(iface, dur, func(f *CANFrame) {
		switch IsEshifterFrame(f.ID) {
		case EshifterFrameHeartbeat:
			if !sawHeartbeat {
				fmt.Println("  [alive]  eshifter heartbeat (0x01111840)")
			}
			sawHeartbeat = true
		case EshifterFrameStatusReport:
			st := DecodeEshifterStatus(f.Data)
			if st != nil {
				sawStatus = true
				haveGear = true
				lastGear = st.CurrentGear
				if int(st.CurrentGear) != printedGear { // print only on change
					printedGear = int(st.CurrentGear)
					fmt.Printf("  [status] %s\n", FormatEshifterFrame(f.ID, f.Data))
				}
			}
		case EshifterFrameTelemetry:
			tel := DecodeEshifterTelemetry(f.Data)
			if tel != nil && tel.IsError {
				telErr = true
				if !printedTelErr {
					printedTelErr = true
					fmt.Printf("  [FAULT]  %s\n", FormatEshifterFrame(f.ID, f.Data))
				}
			}
		}
	})

	fmt.Println(strings.Repeat("-", 50))
	if !sawHeartbeat && !sawStatus && !telErr {
		fmt.Println("VERDICT: NO SIGNAL — no eshifter frames seen.")
		fmt.Println("         Check power, CAN wiring and 120-ohm bus termination.")
		return
	}
	fmt.Printf("Alive (heartbeat): %v\n", sawHeartbeat)
	if haveGear {
		fmt.Printf("Current gear:      %d\n", lastGear)
	} else {
		fmt.Println("Current gear:      (no status frame seen)")
	}
	switch {
	case telErr:
		fmt.Println("VERDICT: FAULT — eshifter reported a telemetry/actuator error (0xFF).")
	case haveGear:
		fmt.Println("VERDICT: OK — eshifter is alive and reporting gear position.")
	default:
		fmt.Println("VERDICT: PARTIAL — frames seen but no gear status; the eshifter may be uninitialized (see -eshifter-init).")
	}
}

// RunFrontlightCheck / RunRearlightCheck attach to the CAN device (or read a
// piped capture) and report front/rear LED-light health. Front and rear lights
// run the same firmware family; the only difference is the CAN node.
func RunFrontlightCheck(iface string, dur time.Duration) {
	runLightCheck(iface, dur, LightTargetFront, "Frontlight")
}
func RunRearlightCheck(iface string, dur time.Duration) {
	runLightCheck(iface, dur, LightTargetRear, "Rearlight")
}

// runLightCheck reports the bench health of a front/rear LED light.
//
// Observable health signals (per the frontlight firmware + bus captures):
//   - heartbeat (0x01111880 front / 0x01111860 rear) .... the MCU is alive
//   - brightness status (0x188051x0 / 0x186051x0) ....... current brightness 0-100%
//   - detailed status (0x188231x0 / 0x186231x0) ......... brightness + mode + dev param
//   - feedback (0x1880F1x0 / 0x1860F1x0) ................. raw status byte
//
// The light protocol carries no documented dedicated fault code, so the verdict
// is OK once the device is alive and reporting brightness.
func runLightCheck(iface string, dur time.Duration, target LightTarget, label string) {
	var sawHeartbeat, haveBrightness, sawDetail bool
	var lastBrightness byte
	printedBrightness := -1

	fmt.Printf("%s bench health check\n", label)
	readFrames(iface, dur, func(f *CANFrame) {
		ft, tgt := IsLightFrame(f.ID)
		if tgt != target {
			return
		}
		switch ft {
		case LightFrameHeartbeat:
			if !sawHeartbeat {
				fmt.Printf("  [alive]  %s heartbeat\n", label)
			}
			sawHeartbeat = true
		case LightFrameStatus:
			if st := DecodeLightStatus(f.Data); st != nil {
				haveBrightness = true
				lastBrightness = st.Brightness
				if int(st.Brightness) != printedBrightness { // print on change
					printedBrightness = int(st.Brightness)
					fmt.Printf("  [status] %s\n", FormatLightFrame(f.ID, f.Data))
				}
			}
		case LightFrameDetail:
			if det := DecodeLightDetail(f.Data); det != nil {
				haveBrightness = true
				lastBrightness = det.Brightness
				if !sawDetail { // print the first detail frame
					sawDetail = true
					fmt.Printf("  [detail] %s\n", FormatLightFrame(f.ID, f.Data))
				}
			}
		}
	})

	fmt.Println(strings.Repeat("-", 50))
	if !sawHeartbeat && !haveBrightness {
		fmt.Printf("VERDICT: NO SIGNAL — no %s frames seen.\n", label)
		fmt.Println("         Check power, CAN wiring and 120-ohm bus termination.")
		return
	}
	fmt.Printf("Alive (heartbeat): %v\n", sawHeartbeat)
	if haveBrightness {
		fmt.Printf("Brightness:        %d%%\n", lastBrightness)
	} else {
		fmt.Println("Brightness:        (no status frame seen)")
	}
	if haveBrightness {
		fmt.Printf("VERDICT: OK — %s is alive and reporting brightness.\n", label)
	} else {
		fmt.Printf("VERDICT: PARTIAL — %s heartbeat seen but no brightness status.\n", label)
	}
}
