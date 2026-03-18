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

Parse a CAN dump file:

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
| `--shift-gear N` | Output `cansend` command to shift to gear N |
| `--shift-gear N --force` | Force gear shift (re-confirm mode) |
| `--eshifter-init N` | Output `cansend` commands to initialize eshifter with gear N |
| `--frontlight N` | Output `cansend` command for frontlight brightness (0=off, 1-100=%) |
| `--rearlight N` | Output `cansend` command for rearlight brightness (0=off, 1-100=%) |
| `--iface NAME` | CAN interface name for cansend commands (default: `can0`) |

### Examples

```bash
# Decode a capture
./canbus < dumps/bikelocked.log

# Show only unrecognized frames
./canbus --unaccounted-only < dumps/startup_from_app.log

# Print the device address table
./canbus --devices

# Print the full protocol reference
./canbus --protocol

# List all CAN IDs
./canbus --canids

# Decode a specific CAN ID
./canbus --decode-id 018F808F

# Compare unaccounted frames across captures
./canbus --compare dumps/bikelocked.log dumps/startup_from_app.log

# Shift to gear 11
./canbus --shift-gear 11

# Force shift to gear 17
./canbus --shift-gear 17 --force

# Initialize eshifter with gear 10
./canbus --eshifter-init 10

# Turn on frontlight at 60% brightness
./canbus --frontlight 60

# Turn off frontlight
./canbus --frontlight 0

# Turn on rearlight at full brightness
./canbus --rearlight 100

# Use a specific CAN interface
./canbus --shift-gear 11 --iface vcan0
```

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
 |-- user_ecu (0x86)       Main user controller
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

## Capturing CAN Bus Data

### Hardware

You need a CAN bus adapter that supports **1 Mbps** and **29-bit extended IDs**. The **MCP2518FD** (CAN FD SPI controller) is recommended — commonly available on breakout boards like the "CANBed FD", CANable, Adafruit MCP2518FD. A simple MCP2515 will also work since the bus uses classic CAN framing but you need a specific Quarz (at least 20MHz) on the PCB. Most AliExpress PCB uses way less and with them you do not get 1Mbits on the bus.

**Wiring to the bike:**

Connect CAN_H and CAN_L from the adapter to the bike's CAN bus. A convenient tap point is a hidden connector between on the right side of the bike near the pedal under a cover.  
The rumor is that this was/is for a powerbank. But I use it to connect to the bike without removing anything.    
A termination resistor is needed when you connect only one device directly to the adapter like the Battery.

### Linux / Raspberry Pi

The MCP2518FD works via SPI with the `mcp251xfd` kernel driver. On a Raspberry Pi, add to `/boot/config.txt`:

```
dtoverlay=mcp251xfd,spi0-0,oscillator=40000000,interrupt=25
```

Then bring up the interface and capture:

```bash
# install can tools
sudo apt install can-utils

# Set up the CAN interface at 1 Mbps
sudo ip link set can0 up type can bitrate 1000000

# Capture to file using candump (from can-utils)
candump -L can0 > capture.log

# Or capture with timestamps
candump -ta can0 > capture.log
```

