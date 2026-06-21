package main

import (
	"fmt"
	"strings"
	"time"
)

// VanMoof SA5 v1.5.0-main firmware release. The NXP LPC546xx sub-ECUs share an
// identical VMFW build header across their device images (confirmed during the
// firmware analysis); the battery/charger/motor report their own versions on
// the bus.
const (
	S5FirmwareRelease = "1.5.0-main"
	S5FirmwareBuild   = "Jan 29 2024 14:50:32"
)

// lpc546xxSubECU lists the device names that run the shared SA5 v1.5.0-main
// LPC546xx firmware (so their version is known without querying).
var lpc546xxSubECU = map[string]bool{
	"imx8_bridge": true, "elock": true, "eshifter": true, "frontlight": true,
	"rearlight": true, "user_ecu": true, "motor_sensor": true, "power_pedal": true,
	"power_control": true,
}

// RunFirmwareScan attaches to the CAN device (or reads a piped capture) and
// builds a firmware/device inventory: which ECUs are present (by heartbeat) and
// their firmware. The LPC546xx sub-ECUs share the v1.5.0-main release; the
// battery answers an active firmware-version query (0x14901470 -> 0x14901460).
func RunFirmwareScan(iface string, dur time.Duration) {
	present := map[string]bool{}
	var batteryReply string

	fmt.Println("Firmware / device inventory scan")
	readFrames(iface, dur, func(f *CANFrame) {
		info := AnalyzeWireCANID(f.ID)
		if info.IsHeartbeat && info.Source != nil && !present[info.Source.Name] {
			present[info.Source.Name] = true
			fmt.Printf("  [present] %s heartbeat\n", info.Source.Name)
		}
		// battery AP firmware-version reply (0x14901460)
		if strings.EqualFold(f.ID, "14901460") {
			if d := FormatBatteryFrame(f.ID, f.Data); d != "" {
				batteryReply = d
			}
		}
	})

	fmt.Println(strings.Repeat("-", 64))
	fmt.Printf("%-16s  %-8s  %s\n", "DEVICE", "PRESENT", "FIRMWARE")
	for _, d := range Devices {
		p := "no"
		if present[d.Name] {
			p = "yes"
		}
		fw := "(query device)"
		if lpc546xxSubECU[d.Name] {
			fw = S5FirmwareRelease
		}
		fmt.Printf("%-16s  %-8s  %s\n", d.Name, p, fw)
	}

	fmt.Printf("\nLPC546xx sub-ECUs share the SA5 %s build (%s) per their VMFW image headers.\n",
		S5FirmwareRelease, S5FirmwareBuild)
	if batteryReply != "" {
		fmt.Printf("Battery firmware (from reply): %s\n", batteryReply)
	} else {
		fmt.Println("To actively request the battery AP firmware version:")
		fmt.Printf("  cansend %s 14901470#       (reply decoded on 0x14901460)\n", iface)
	}
}
