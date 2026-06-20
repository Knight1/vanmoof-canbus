//go:build linux

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"
)

// SocketCAN constants (not all exported by the stdlib syscall package).
const (
	afCAN      = 29         // AF_CAN
	canRaw     = 1          // CAN_RAW protocol
	canEFFFlag = 0x80000000 // extended (29-bit) frame format flag in can_id
	canEFFMask = 0x1FFFFFFF // 29-bit extended id mask
	canSFFMask = 0x000007FF // 11-bit standard id mask
)

// readFramesLive attaches a raw SocketCAN socket to the named interface and
// calls handle for every CAN frame received within the given duration. The bus
// must already be up at the correct bitrate, e.g.:
//
//	sudo ip link set can0 up type can bitrate 1000000
//
// It returns an error if the interface cannot be opened/bound.
func readFramesLive(iface string, duration time.Duration, handle func(*CANFrame)) error {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q not found: %w", iface, err)
	}

	fd, err := syscall.Socket(afCAN, syscall.SOCK_RAW, canRaw)
	if err != nil {
		return fmt.Errorf("socket(AF_CAN, SOCK_RAW, CAN_RAW): %w", err)
	}
	defer syscall.Close(fd)

	// struct sockaddr_can { family u16; _pad u16; ifindex i32; addr[8] } = 16 bytes
	var sa [16]byte
	binary.LittleEndian.PutUint16(sa[0:2], afCAN)
	binary.LittleEndian.PutUint32(sa[4:8], uint32(ifi.Index))
	if _, _, e := syscall.Syscall(syscall.SYS_BIND, uintptr(fd),
		uintptr(unsafe.Pointer(&sa[0])), uintptr(len(sa))); e != 0 {
		return fmt.Errorf("bind %s: %w", iface, e)
	}

	// 1s receive timeout so the deadline (and Ctrl-C) are honoured between frames
	tv := syscall.NsecToTimeval(time.Second.Nanoseconds())
	_ = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)

	deadline := time.Now().Add(duration)
	buf := make([]byte, 16) // one struct can_frame
	for time.Now().Before(deadline) {
		n, rerr := syscall.Read(fd, buf)
		if rerr != nil {
			if rerr == syscall.EAGAIN || rerr == syscall.EWOULDBLOCK || rerr == syscall.EINTR {
				continue // receive timeout tick — re-check the deadline
			}
			return fmt.Errorf("read %s: %w", iface, rerr)
		}
		if n < 8 {
			continue // not a full can_frame header
		}

		canID := binary.LittleEndian.Uint32(buf[0:4])
		dlc := int(buf[4])
		if dlc > 8 {
			dlc = 8
		}
		if 8+dlc > n {
			dlc = n - 8
		}

		f := &CANFrame{
			Data:   append([]byte(nil), buf[8:8+dlc]...),
			Length: dlc,
		}
		if canID&canEFFFlag != 0 {
			f.IsExtended = true
			f.ID = fmt.Sprintf("%08X", canID&canEFFMask)
		} else {
			f.ID = fmt.Sprintf("%03X", canID&canSFFMask)
		}
		handle(f)
	}
	return nil
}
