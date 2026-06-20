//go:build !linux

package main

import (
	"fmt"
	"time"
)

// readFramesLive is only supported on Linux (SocketCAN). On other platforms it
// reports an error; pipe a candump/CSV capture into the check instead.
func readFramesLive(iface string, duration time.Duration, handle func(*CANFrame)) error {
	_ = iface
	_ = duration
	_ = handle
	return fmt.Errorf("live CAN attach requires Linux SocketCAN; pipe a candump/CSV capture instead")
}
