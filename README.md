# CAN Bus Parser

A Go application for parsing and decoding CAN bus frames from VanMoof SA5 and later bikes.  
This tool reads CAN frame data in both CSV and candump formats, reassembles multi-frame CBOR messages, and decodes the CBOR-encoded payloads according to the VanMoof protocol specification.

## Prerequisites

- Go 1.24 or higher

## Installation

```bash
go build -v -ldflags="-w -s" -mod=vendor ./...
```

## Usage

Parse a CSV file containing CAN frame data:

```bash
./canbus < input.log
```

## VanMoof Protocol

The VanMoof CAN bus protocol uses a framing mechanism to transmit multi-frame CBOR-encoded messages. Understanding the header byte is critical for proper message reassembly.

### Header Byte Structure

The first byte of each CAN frame is a header byte that indicates the frame type. The high nibble (upper 4 bits) determines the frame type:

| Header Range | Type | Description |
|---|---|---|
| `0xAx` (0xA0-0xAF) | **START** | Begins a new CBOR message |
| `0x1x` (0x10-0x1F) | **CONTINUATION** | Continuation of current message |
| `0x8x` (0x80-0x8F) | **DATA** | Raw data frame (not part of CBOR) |
| `0x9x` (0x90-0x9F) | **DATA** | Raw data frame (not part of CBOR) |
| `0x0x` (0x00) | **STATUS/HEARTBEAT** | Keep-alive or status information |

### Decoding Steps

1. **Detect Frame Type**: Check header byte high nibble
2. **Extract Payload**: Remove header byte (first byte), keep remaining 7 bytes
3. **Accumulate**: For START frames, initialize buffer; for CONTINUATION frames, append to buffer
4. **Decode CBOR**: Once a complete message is buffered, decode using CBOR decoder
5. **Pretty Print**: Recursively display the decoded structure