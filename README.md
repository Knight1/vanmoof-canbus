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

### Flags

| Flag | Description |
|---|---|
| `--version` | Show version information |
| `--devices` | Print the SA5 CAN bus device table and exit |
| `--protocol` | Print the CAN bus protocol summary and exit |
| `--canids` | Print all CAN IDs and exit |
| `--decode-id HEX` | Decode a CAN ID (e.g. `018F808F`) |
| `--group-by-id` | Group frames by CAN ID, sorted by timestamp |
| `--unaccounted-only` | Show only frames that are not CBOR or heartbeat |
| `--hide-unaccounted` | Hide unaccounted frames |
| `--hide-accounted` | Hide CBOR and heartbeat frames |
| `--compare FILE...` | Compare unaccounted frames across multiple files |

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