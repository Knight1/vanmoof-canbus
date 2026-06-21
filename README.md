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
| `--shift-gear N --confirm` | Shift, then watch the eshifter status until the gear changes |
| `--eshifter-init N` | Output `cansend` commands to initialize eshifter with gear N |
| `--frontlight N` | Output `cansend` command for frontlight brightness (0=off, 1-100=%) |
| `--frontlight N --confirm` | Set brightness, then watch the status until it reaches the target |
| `--rearlight N` | Output `cansend` command for rearlight brightness (0=off, 1-100=%) |
| `--rearlight N --confirm` | Set brightness, then watch the status until it reaches the target |
| `--fake-pedal-switch N` | Output `cansend` faking the power_pedal switch_control signal (a1=0x0A) with byte value N |
| `--fake-pedal-signal A1` | Output `cansend` faking any power_pedal OD signal at index A1 (pair with `--fake-pedal-data`) |
| `--fake-pedal-data HEX` | Hex payload for `--fake-pedal-signal` (e.g. `01F4`); empty = zero-length frame |
| `--elock-unlock` | Output `cansend` command to unlock the elock (kicklock) |
| `--elock-unlock-confirm` | Unlock, then watch the lock state for the result (UNLOCKED/STUCK) |
| `--iface NAME` | CAN interface name (default: `can0`) |

#### Live diagnostics (attach to the CAN device, or pipe a capture)

These commands **attach directly to the CAN interface** (Linux SocketCAN) for a few
seconds and report a verdict. If a capture is piped/redirected into stdin they parse
that instead, so they also work offline on a saved dump.

| Flag | Description |
|---|---|
| `--elock-check` | Report elock health: alive (heartbeat) / lock state / STUCK fault |
| `--eshifter-check` | Report eshifter health: alive / current gear / telemetry error |
| `--frontlight-check` | Report frontlight health: alive / brightness |
| `--rearlight-check` | Report rearlight health: alive / brightness |
| `--power-pedal-check` | Report power_pedal health: alive (heartbeat) / publishing OD telemetry |
| `--firmware` | Inventory every ECU present on the bus + its firmware version |
| `--duration N` | Seconds to listen on the bus for the `*-check`/`--firmware` commands (default: 6) |

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

# Fake the power_pedal switch_control signal (documented a1=0x0A)
./canbus --fake-pedal-switch 1

# Fake any power_pedal OD signal by index (e.g. publish a1=0x05 = 0x01F4)
./canbus --fake-pedal-signal 5 --fake-pedal-data 01F4

# Use a specific CAN interface
./canbus --shift-gear 11 --iface vcan0

# Shift to gear 11 and confirm it took effect (sends, then watches the status)
./canbus --shift-gear 11 --confirm

# Set the frontlight to 60% and confirm the reported brightness reaches it
./canbus --frontlight 60 --confirm

# Unlock the kicklock and watch the lock state for the result
./canbus --elock-unlock-confirm
```

### Live diagnostics

The `*-check` and `--firmware` commands attach to the CAN device directly (Linux
SocketCAN), or read a piped/saved capture. Bring the bus up first:
`sudo ip link set can0 up type can bitrate 1000000`.

```bash
# Health-check a device on the live bus (attaches to can0 for 6s)
./canbus --elock-check
./canbus --eshifter-check
./canbus --frontlight-check --iface can1
./canbus --rearlight-check --duration 10
./canbus --power-pedal-check

# Inventory every ECU on the bus and its firmware version
./canbus --firmware

# The same checks work offline on a saved capture
./canbus --elock-check    < dumps/startup_from_app.log
candump -L can0 | ./canbus --firmware
```

## VanMoof SA5 CAN Bus Protocol

### Bus Configuration

| Parameter | Value |
|---|---|
| Speed | 1 Mbps |
| Controller | Bosch M_CAN (CAN FD capable, used as classic CAN) |
Impl| Hardware filters | None (promiscuous mode) |
| Frame type | 29-bit Extended CAN ID |

### Device Table

| PS Address | Device | PF=SA | MCU | Notes |
|---|---|---|---|---|
| 0x80 | imx8_bridge | 0x8F | NXP LPC546xx | Central gateway to i.MX8 |
| 0x82 | ble | 0x82 | Nordic nRF52840 | Bluetooth Low Energy |
| 0x83 | modem | 0x83 | Nordic nRF52 | Cellular modem |
| 0x84 | motor_sensor | 0xA1 | NXP LPC546xx | Speed/cadence sensor |
| 0x85 | elock | 0xC1 | NXP LPC546xx | Electronic lock |
| 0x86 | user_ecu | 0xC0 | NXP LPC546xx | Main user controller |
| 0x87 | frontlight | 0xC4 | NXP LPC546xx | Front light controller |
| 0x88 | rearlight | 0xC3 | NXP LPC546xx | Rear light controller |
| 0x8D | charger_target | ??? | Unknown | Battery BMS or power delivery |
| 0x91 | eshifter | 0xC2 | NXP LPC546xx | Electronic gear shifter |
| 0x92 | power_pedal | 0xA2 | NXP LPC546xx | Pedal assist/torque sensor |
| 0x93 | motor_control | 0x93 | Unknown | Motor controller (non-ARM MCU) |
| ??? | power_control | 0xA3 | NXP LPC546xx | Power management (PS addr unknown) |
| ??? | charger | 0x70 | NXP LPC546xx | Liteon charger (separate CAN bus) |

### On-Wire CAN ID Encoding

CAN IDs on the wire use a device-encoded value: `device_encoded = (PFSA & 0x7F) << 5`

- **Heartbeat:** `CAN_ID = 0x01111000 + device_encoded`
- **Data frame:** `CAN_ID = (1 << 28) | (sender << 16) | (class << 12) | destination`

See [CANBUS.md](CANBUS.md) for the full CAN ID map and protocol details.

### CBOR Framing Protocol

The CAN bus uses a framing mechanism to transmit multi-frame CBOR-encoded messages over 8-byte CAN frames.

| Header Range | Type | Description |
|---|---|---|
| `0xAx` (0xA0-0xAF) | **START** | Begins a new CBOR message |
| `0x1x` (0x10-0x1F) | **CONTINUATION** | Continuation of current message |
| `0x0x` (0x00) | **HEARTBEAT** | Keep-alive (IDs starting with `01111`, all-zero payload) |

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

