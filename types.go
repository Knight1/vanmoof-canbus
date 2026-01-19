package main

// CANFrame represents a parsed CAN bus frame
type CANFrame struct {
	Timestamp  string
	ID         string
	IsExtended bool
	Direction  string
	Bus        int
	Length     int
	Data       []byte
}

// FrameInfo stores frame with metadata for grouping
type FrameInfo struct {
	Frame          *CANFrame
	TimestampFloat float64
	Header         byte
	FrameType      string
	IsHeartbeat    bool
	IsCBOR         bool
	SequenceNum    int // For maintaining order when timestamps are identical
}
