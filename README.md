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
## VanMoof SA5 CAN Bus Protocol

### Bus Configuration

| Parameter | Value |
|---|---|
| Speed | 1 Mbps |
| Controller | Bosch M_CAN (CAN FD capable, used as classic CAN) |
| M_CAN base | 0x4009D000 |
| Hardware filters | None (promiscuous mode) |
| Frame type | 29-bit Extended CAN ID |
#### DP=0: Special Purpose Messages

| Pattern | Name | Description |
|---|---|---|
| `0x00{PF}8887` | Light-pair sync | Every device sends one; b1=0x88(rearlight), b0=0x87(frontlight) |
| `0x00{PF}0182` | BLE cmd type 1 | Command to BLE module |
| `0x00{PF}0382` | BLE cmd type 3 | Command to BLE module |
| `0x00{PF}{cmd}{target_pfsa}` | Device command | DP=0 command to specific device by PF/SA value |

### CBOR Framing Protocol

The VanMoof CAN bus uses a framing mechanism to transmit multi-frame CBOR-encoded messages over 8-byte CAN frames.

#### Header Byte Structure

The first byte of each CAN frame is a header byte. The high nibble determines the frame type:

| Header Range | Type | Description |
|---|---|---|
| `0xAx` (0xA0-0xAF) | **START** | Begins a new CBOR message |
| `0x1x` (0x10-0x1F) | **CONTINUATION** | Continuation of current message |
| `0x8x` / `0x9x` | **DATA** | Raw data frame (not CBOR) |
| `0x0x` (0x00) | **STATUS/HEARTBEAT** | Keep-alive or status (IDs starting with `01111`, all-zero payload) |

#### Decoding Steps

1. **Detect** frame type by checking header byte high nibble
2. **Extract** payload: remove header byte (first byte), keep remaining 7 bytes
3. **Accumulate**: START frames initialize buffer; CONTINUATION frames append
4. **Decode** completed CBOR message when parser succeeds
5. **Display** the decoded structure recursively

### Bus Topology

```
Main CAN Bus (1 Mbps, no hardware filters)
 |
 |-- imx8_bridge (0x80)    Central gateway to i.MX8 SoC
 |-- ble (0x82)            Bluetooth Low Energy module
 |-- modem (0x83)          Cellular connectivity
 |-- motor_sensor (0x84)   Speed/cadence sensing
 |-- elock (0x85)          Electronic lock mechanism
 |-- user_ecu (0x86)       Main user controller (largest firmware)
 |-- frontlight (0x87)     Front light controller
 |-- rearlight (0x88)      Rear light controller
 |-- eshifter (0x91)       Electronic gear shifter
 |-- power_pedal (0x92)    Pedal assist / torque sensor
 |-- motor_control (0x93)  Motor controller (non-ARM MCU)
 +-- power_control (??)    Power management

Charger CAN Bus (separate segment)
 |-- charger (0x70)        Liteon charger controller
 +-- charger_target (0x8D) Battery BMS / power delivery unit
```

### Communication Matrix

Devices with full bus access (11 handlers): motor_sensor, elock, user_ecu, frontlight, rearlight, eshifter, power_pedal

Devices with limited bus access (7 handlers): imx8_bridge, power_control

The charger operates on a separate CAN bus segment communicating only with device 0x8D.
