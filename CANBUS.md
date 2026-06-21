# VanMoof SA5 CAN Bus - Complete ID Map

## Bus Configuration

| Parameter | Value |
|---|---|
| Speed | 1 Mbps |
| Controller | Bosch M_CAN (CAN FD capable, used as classic CAN) |
| M_CAN base address | 0x4009D000 |
| Hardware filters | None (promiscuous mode) |
| Frame type | 29-bit Extended CAN ID |
| Max data length | 8 bytes per frame |
| Platform MCU | NXP LPC546xx (Cortex-M4) |

## Device Address Table

| PS Addr | Device | PF=SA | MCU | Notes |
|---|---|---|---|---|
| 0x80 | imx8_bridge | 0x8F | NXP LPC546xx | Central gateway to i.MX8 SoC |
| 0x82 | ble | 0x82 | Nordic nRF52840 | Bluetooth Low Energy module |
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
| ??? | power_control | 0xA3 | NXP LPC546xx | Power management |
| ??? | charger | 0x70 | NXP LPC546xx | Liteon charger (separate bus) |

## CAN ID Format (29-bit Extended)

The handler table uses a J1939-inspired format:

```
Bits 28:24 = DP (Data Page) — 0x01 = standard, 0x00 = special
Bits 23:16 = PF (PDU Format) — source device identifier
Bits 15:8  = PS (PDU Specific) — target device address
Bits 7:0   = SA (Source Address) — always equals PF

Pattern: 0x01{SRC}{DST}{SRC}
```

Each device has a unique PF=SA value. Every device registers handlers for each other device it communicates with, identified by the target's PS address.

**Example:** user_ecu (PF=SA=0xC0) handler for motor_sensor (PS=0x84):

```
CAN ID = 0x01C084C0
         01 = DP (standard messaging)
         C0 = PF (user_ecu's identifier)
         84 = PS (motor_sensor's address)
         C0 = SA (= PF, user_ecu)
```

## On-Wire CAN ID Encoding

CAN IDs on the wire use a different encoding. The key is the **device encoded value**:

```
device_encoded = (PFSA & 0x7F) << 5     (12-bit result)
```

| Device | PFSA | Encoded | High Nibble | Low Byte |
|---|---|---|---|---|
| imx8_bridge | 0x8F | 0x1E0 | 0x1 | 0xE0 |
| ble | 0x82 | 0x040 | 0x0 | 0x40 |
| modem | 0x83 | 0x060 | 0x0 | 0x60 |
| motor_sensor | 0xA1 | 0x420 | 0x4 | 0x20 |
| elock | 0xC1 | 0x820 | 0x8 | 0x20 |
| user_ecu | 0xC0 | 0x800 | 0x8 | 0x00 |
| frontlight | 0xC4 | 0x880 | 0x8 | 0x80 |
| rearlight | 0xC3 | 0x860 | 0x8 | 0x60 |
| eshifter | 0xC2 | 0x840 | 0x8 | 0x40 |
| power_pedal | 0xA2 | 0x440 | 0x4 | 0x40 |
| motor_control | 0x93 | 0x260 | 0x2 | 0x60 |
| power_control | 0xA3 | 0x460 | 0x4 | 0x60 |

### On-Wire Bit Layout

| Bits | Heartbeat | Data Frame |
|---|---|---|
| Bit 28 | 0 | 1 |
| Bits 27:16 | 0x111 (constant) | sender device_encoded (12 bits) |
| Bits 15:12 | 0x1 (constant) | message class marker (4 bits) |
| Bits 11:0 | source device_encoded | destination device_encoded (12 bits) |

**Heartbeat formula:** `CAN_ID = 0x01111000 + device_encoded`

**Data frame formula:** `CAN_ID = (1 << 28) | (sender_enc << 16) | (class << 12) | dest_enc`

### Message Class Markers (bits 15:12)

| Marker | Meaning |
|---|---|
| 0x3 | Primary self/status message |
| 0x5 | Secondary status/telemetry |
| 0x7 | Cross-device data transfer |
| 0x9 | Tertiary status/command |
| 0xB | Extended status |
| 0xD | System state command (broadcast) |

### Decoded Examples

| On-Wire ID | Decoded | Class |
|---|---|---|
| 0x011111E0 | heartbeat from imx8_bridge | - |
| 0x01111800 | heartbeat from user_ecu | - |
| 0x01111820 | heartbeat from elock | - |
| 0x10403040 | ble self | 0x3 |
| 0x104071E0 | ble -> imx8_bridge | 0x7 |
| 0x10407800 | ble -> user_ecu | 0x7 |
| 0x14203420 | motor_sensor self | 0x3 |
| 0x14403440 | power_pedal self | 0x3 |
| 0x18009800 | user_ecu self | 0x9 |
| 0x1800D110 | user_ecu -> broadcast (system state) | 0xD |
| 0x18209820 | elock self | 0x9 |

## DP=1 CAN IDs — Standard Device-to-Device Messaging

### imx8_bridge (PF=SA=0x8F)

| CAN ID | Target | Payload |
|---|---|---|
| 0x018F808F | self (bridge) | 7 bytes |
| 0x018F828F | ble (0x82) | 4 bytes |
| 0x018F838F | modem (0x83) | 8 bytes |
| 0x018F848F | motor_sensor (0x84) | 4 bytes |
| 0x018F858F | elock (0x85) | 1 byte |
| 0x018F868F | user_ecu (0x86) | variable |
| 0x018F938F | motor_control (0x93) | 1 byte |

### user_ecu (PF=SA=0xC0)

| CAN ID | Target | Payload |
|---|---|---|
| 0x01C080C0 | imx8_bridge (0x80) | |
| 0x01C082C0 | ble (0x82) | 4 bytes |
| 0x01C083C0 | modem (0x83) | 8 bytes |
| 0x01C084C0 | motor_sensor (0x84) | 4 bytes |
| 0x01C085C0 | elock (0x85) | 1 byte |
| 0x01C086C0 | self (user_ecu 0x86) | 0 bytes |
| 0x01C087C0 | frontlight (0x87) | 13 bytes |
| 0x01C088C0 | rearlight (0x88) | 13 bytes |
| 0x01C091C0 | eshifter (0x91) | 16 bytes |
| 0x01C092C0 | power_pedal (0x92) | 16 bytes |
| 0x01C093C0 | motor_control (0x93) | 1 byte |

### motor_sensor (PF=SA=0xA1)

| CAN ID | Target |
|---|---|
| 0x01A180A1 | imx8_bridge (0x80) |
| 0x01A182A1 | ble (0x82) |
| 0x01A183A1 | modem (0x83) |
| 0x01A184A1 | self (motor_sensor) |
| 0x01A185A1 | elock (0x85) |
| 0x01A186A1 | user_ecu (0x86) |
| 0x01A187A1 | frontlight (0x87) |
| 0x01A188A1 | rearlight (0x88) |
| 0x01A191A1 | eshifter (0x91) |
| 0x01A192A1 | power_pedal (0x92) |
| 0x01A193A1 | motor_control (0x93) |

### elock (PF=SA=0xC1)

| CAN ID | Target |
|---|---|
| 0x01C180C1 | imx8_bridge (0x80) |
| 0x01C182C1 | ble (0x82) |
| 0x01C183C1 | modem (0x83) |
| 0x01C184C1 | motor_sensor (0x84) |
| 0x01C185C1 | self (elock 0x85) |
| 0x01C186C1 | user_ecu (0x86) |
| 0x01C187C1 | frontlight (0x87) |
| 0x01C188C1 | rearlight (0x88) |
| 0x01C191C1 | eshifter (0x91) |
| 0x01C192C1 | power_pedal (0x92) |
| 0x01C193C1 | motor_control (0x93) |

### frontlight (PF=SA=0xC4)

| CAN ID | Target |
|---|---|
| 0x01C480C4 | imx8_bridge (0x80) |
| 0x01C482C4 | ble (0x82) |
| 0x01C483C4 | modem (0x83) |
| 0x01C484C4 | motor_sensor (0x84) |
| 0x01C485C4 | elock (0x85) |
| 0x01C486C4 | user_ecu (0x86) |
| 0x01C487C4 | self (frontlight 0x87) |
| 0x01C488C4 | rearlight (0x88) |
| 0x01C491C4 | eshifter (0x91) |
| 0x01C492C4 | power_pedal (0x92) |
| 0x01C493C4 | motor_control (0x93) |

### rearlight (PF=SA=0xC3)

| CAN ID | Target |
|---|---|
| 0x01C380C3 | imx8_bridge (0x80) |
| 0x01C382C3 | ble (0x82) |
| 0x01C383C3 | modem (0x83) |
| 0x01C384C3 | motor_sensor (0x84) |
| 0x01C385C3 | elock (0x85) |
| 0x01C386C3 | user_ecu (0x86) |
| 0x01C387C3 | frontlight (0x87) |
| 0x01C388C3 | self (rearlight 0x88) |
| 0x01C391C3 | eshifter (0x91) |
| 0x01C392C3 | power_pedal (0x92) |
| 0x01C393C3 | motor_control (0x93) |

### eshifter (PF=SA=0xC2)

| CAN ID | Target |
|---|---|
| 0x01C280C2 | imx8_bridge (0x80) |
| 0x01C282C2 | ble (0x82) |
| 0x01C283C2 | modem (0x83) |
| 0x01C284C2 | motor_sensor (0x84) |
| 0x01C285C2 | elock (0x85) |
| 0x01C286C2 | user_ecu (0x86) |
| 0x01C287C2 | frontlight (0x87) |
| 0x01C288C2 | rearlight (0x88) |
| 0x01C291C2 | self (eshifter 0x91) |
| 0x01C292C2 | power_pedal (0x92) |
| 0x01C293C2 | motor_control (0x93) |

### power_pedal (PF=SA=0xA2)

| CAN ID | Target |
|---|---|
| 0x01A280A2 | imx8_bridge (0x80) |
| 0x01A282A2 | ble (0x82) |
| 0x01A283A2 | modem (0x83) |
| 0x01A284A2 | motor_sensor (0x84) |
| 0x01A285A2 | elock (0x85) |
| 0x01A286A2 | user_ecu (0x86) |
| 0x01A287A2 | frontlight (0x87) |
| 0x01A288A2 | rearlight (0x88) |
| 0x01A291A2 | eshifter (0x91) |
| 0x01A292A2 | self (power_pedal 0x92) |
| 0x01A293A2 | motor_control (0x93) |

### power_control (PF=SA=0xA3) — Limited bus access

| CAN ID | Target | Notes |
|---|---|---|
| 0x01A380A3 | imx8_bridge (0x80) | |
| 0x01A382A3 | ble (0x82) | |
| 0x01A383A3 | modem (0x83) | |
| 0x01A384A3 | motor_sensor (0x84) | |
| 0x01A385A3 | elock (0x85) | |
| 0x01A386A3 | user_ecu (0x86) | |
| 0x01A393A3 | motor_control (0x93) | |
| 0x01A301A3 | ??? (PS=0x01) | Non-standard target |
| 0x01A302A3 | ??? (PS=0x02) | Non-standard target |
| 0x01A304A3 | ??? (PS=0x04) | Non-standard target |

### charger (PF=SA=0x70) — Separate bus segment

| CAN ID | Target | Notes |
|---|---|---|
| 0x01708D70 | charger_target (0x8D) | Battery BMS/PMU |

## DP=0 CAN IDs — Special Purpose Messages

Format: `0x00{PF}{CMD}{TARGET_PF_OR_ADDR}`

### Light Pair Sync

Every device sends one. Byte 1 = 0x88 (rearlight addr), byte 0 = 0x87 (frontlight addr).

| CAN ID | Source |
|---|---|
| 0x008F8887 | imx8_bridge |
| 0x00A18887 | motor_sensor |
| 0x00A28887 | power_pedal |
| 0x00A38887 | power_control |
| 0x00C08887 | user_ecu |
| 0x00C18887 | elock |
| 0x00C28887 | eshifter |
| 0x00C38887 | rearlight |
| 0x00C48887 | frontlight |

### BLE Command Messages (CMD=0x03 or 0x01, Target=0x82)

| CAN IDs | Source |
|---|---|
| 0x008F0382 / 0x008F0182 | imx8_bridge |
| 0x00A10382 / 0x00A10182 | motor_sensor |
| 0x00A20382 / 0x00A20182 | power_pedal |
| 0x00A30382 / 0x00A30182 | power_control |
| 0x00C00382 / 0x00C00182 | user_ecu |
| 0x00C10382 / 0x00C10182 | elock |

### elock Special Messages

| CAN ID | Description |
|---|---|
| 0x00C101A1 | CMD=0x01 to motor_sensor (PF=0xA1) |
| 0x00C10BA2 | CMD=0x0B to power_pedal (PF=0xA2) |

### eshifter Special Messages

| CAN ID | Description |
|---|---|
| 0x00C201A1 | CMD=0x01 to motor_sensor (PF=0xA1) |
| 0x00C201A2 | CMD=0x01 to power_pedal (PF=0xA2) |
| 0x00C205A4 | CMD=0x05 to ??? (PF=0xA4, unknown device) |

### power_control Special Messages

| CAN ID | Description |
|---|---|
| 0x00A301A4 | CMD=0x01 to ??? (PF=0xA4) |
| 0x00A302A4 | CMD=0x02 to ??? (PF=0xA4) |
| 0x00A303A4 | CMD=0x03 to ??? (PF=0xA4) |
| 0x00A306A4 | CMD=0x06 to ??? (PF=0xA4) |
| 0x00A307A4 | CMD=0x07 to ??? (PF=0xA4) |
| 0x00A310A7 | CMD=0x10 to ??? (PF=0xA7) |
| 0x00A311A7 | CMD=0x11 to ??? (PF=0xA7) |
| 0x00A312A7 | CMD=0x12 to ??? (PF=0xA7) |

> PF=0xA4 and PF=0xA7 may be battery/charger subsystems.

### user_ecu Special Messages

| CAN ID | Description |
|---|---|
| 0x00C0EB02 | CMD=0xEB to ??? (0x02) |
| 0x00C0EB04 | CMD=0xEB to ??? (0x04) |

## Bus Topology

```
Main CAN Bus (1 Mbps, no hardware filters)
 |
 |-- imx8_bridge (0x80)    Central gateway to i.MX8 SoC
 |-- ble (0x82)            Bluetooth Low Energy module
 |-- modem (0x83)          Cellular modem
 |-- motor_sensor (0x84)   Speed/cadence sensor
 |-- elock (0x85)          Electronic lock
 |-- user_ecu (0x86)       Main user controller
 |-- frontlight (0x87)     Front light controller
 |-- rearlight (0x88)      Rear light controller
 |-- eshifter (0x91)       Electronic gear shifter
 |-- power_pedal (0x92)    Pedal assist/torque sensor
 |-- motor_control (0x93)  Motor controller (non-ARM MCU)
 +-- power_control (??)    Power management (limited bus access)

Charger CAN Bus (separate segment)
 |-- charger (0x70)        Liteon charger controller
 +-- charger_target (0x8D) Battery BMS / power delivery unit
```

## CBOR Framing Protocol

The CAN bus uses a framing mechanism to transmit multi-frame CBOR-encoded messages over 8-byte CAN frames.

### Frame Types

The first byte (header) high nibble determines the frame type:

| Header Range | Type | Description |
|---|---|---|
| `0xAx` (0xA0-0xAF) | **START** | Begins a new CBOR message |
| `0x1x` (0x10-0x1F) | **CONTINUATION** | Continuation of current message |
| `0x0x` (0x00) | **HEARTBEAT** | Keep-alive (IDs 0x01111xxx, all-zero payload) |
| Other | **RAW DATA** | Single-frame: CBOR, raw binary, or ASCII |

### Multi-Frame Decoding

1. **START** frame (0xAx): strip header byte, init buffer with remaining 7 bytes
2. **CONT** frames (0x1x): strip header byte, append 7 bytes to buffer
3. Attempt CBOR decode on accumulated buffer after each frame
4. When decode succeeds, display the CBOR structure

### Single-Frame Messages

Most frames (~98%) are NOT multi-frame CBOR. The full 8-byte payload is interpreted as:
- **Raw CBOR** — if entire payload forms a valid CBOR item
- **Raw binary** — sensor values, status bytes, counters
- **ASCII text** — serial numbers, model identifiers

## Communication Matrix

**Full bus access** (11 handlers, talks to every main bus device):
motor_sensor, elock, user_ecu, frontlight, rearlight, eshifter, power_pedal

**Limited bus access** (7 handlers):
- imx8_bridge — self, ble, modem, motor_sensor, elock, user_ecu, motor_control
- power_control — bridge, ble, motor_sensor, elock, user_ecu, power_pedal, motor_control

**Separate bus:**
charger (0x70) <-> charger_target (0x8D) only

## Eshifter Protocol (Electronic Gear Shifter)

The eshifter controls an Enviolo CVT (Continuously Variable Transmission) hub via a stepper/servo actuator. It uses raw binary framing (not CBOR) for real-time control.

### Eshifter On-Wire CAN IDs

| On-Wire CAN ID | Direction | Description |
|---|---|---|
| `0x01111840` | Heartbeat | Eshifter keepalive (8 bytes, all zeros) |
| `0x10F11840` | To eshifter (0x840) | Configuration and sensor data |
| `0x10F11841` | To eshifter feedback (0x841) | Real-time cadence and actuator position |
| `0x18411840` | To eshifter (0x840) | Gear change commands |
| `0x1840B110` | From eshifter | Status report (class=0xB) |
| `0x1840B100` | From eshifter | Empty acknowledgment frame |

### Sub-Addressing

The eshifter uses two sub-addresses derived from its base encoded value (0x840):

| Address | Channel | Purpose |
|---|---|---|
| 0x840 | Main | Configuration, sensor data, gear commands |
| 0x841 | Feedback | Real-time cadence and actuator position telemetry |

This sub-addressing pattern also applies to other devices (e.g., power_control uses 0x460-0x463).

### Gear Change Command

**CAN ID:** `0x18411840` (sender=0x841, dest=0x840, class=0x1)

5-byte payload:

| Byte | Value | Description |
|---|---|---|
| 0 | `0x01` | Command identifier |
| 1 | `0x01` | Sub-command |
| 2 | `0x00` or `0x01` | Mode: 0x00=set, 0x01=force/re-confirm |
| 3 | `0x0A`-`0x11` | Target gear position |
| 4 | `0x00` | Reserved |

Observed gear positions: `0x0A` (10), `0x0B` (11), `0x11` (17).

### Configuration Data

**CAN ID:** `0x10F11840` (sender=0x0F1, dest=0x840, class=0x1)

8-byte payload with constant prefix `00 A9 FC 8C`:

| Type (byte 4) | Bytes 5-7 | Description |
|---|---|---|
| `0x44` | `00 GG 00` | Set initial gear position (GG = gear) |
| `0x55` | `01 01 00` | Enable eshifter |
| `0x66` | `01 00 00` | Configuration mode |
| `0xCA` | `00 00 00` | Clear / reset |
| `0x8D` | `00 SL SH` | Continuous speed data (16-bit LE, ~1500-1900) |
| `0x98` | `00 SL SH` | Alternate speed data (rare) |
| `0xEE` | `00 01 00` | Gear change confirmation part 1 |
| `0xEF` | `00 01 00` | Gear change confirmation part 2 |
| `0x00` | `01 01 00` | Status / init |

The `0xEE`/`0xEF` pair always appears together immediately after a gear change command.

The `0x8D` type is the most frequent, providing continuous speed/sensor data used for auto-shifting decisions.

### Eshifter Status Response

**CAN ID:** `0x1840B110` (sender=0x840, dest=0x110, class=0xB)

8-byte payload:

| Byte | Example | Description |
|---|---|---|
| 0 | `0xDC` | Status flags |
| 1 | `0x05` | Sub-status |
| 2 | `0x28` (40) | Parameter (possibly temperature) |
| 3 | **`0x0A`** | **Current gear position** |
| 4 | `0x60` | Status flags |
| 5 | `0x09` | Parameter |
| 6 | `0x14` (20) | Parameter |
| 7 | `0x05` | Parameter |

Byte 3 contains the current gear position and matches the last gear command sent.

### Real-Time Feedback Telemetry

**CAN ID:** `0x10F11841` (sender=0x0F1, dest=0x841, class=0x1)

8-byte payload:

| Byte | During Riding | During Gear Change |
|---|---|---|
| 0 | `0x00` | `0x00` |
| 1 | `0x00` | `0x00` |
| 2 | Cadence RPM (~60-118) | Echoes target gear, then small values |
| 3 | `0x00` | `0x00` (or `0xFF` on error) |
| 4 | `0x00` | `0x00` |
| 5 | `0x00` | `0x00` |
| 6 | Wheel counter low byte | Actuator position low byte |
| 7 | Wheel counter high byte | Actuator position high byte |

During riding, bytes 6-7 form a steadily incrementing 16-bit LE wheel revolution counter.

During a gear change, bytes 6-7 show the CVT actuator position moving smoothly from one position to another (~27 frames for a full shift).

### Actuator Position Range

The CVT hub actuator has approximately **140 discrete positions**:

| Position | Gear Ratio |
|---|---|
| ~201 (0xC9) | Highest (hardest) gear |
| ~61 (0x3D) | Lowest (easiest) gear |

### Initialization Sequence

On startup, the following frames are sent to bring the eshifter online:

| Step | CAN ID | Payload | Description |
|---|---|---|---|
| 1 | `0x10F11840` | `3D 95 16 87 18 00 35 00` | Init handshake |
| 2 | `0x10F11840` | `00 A9 FC 8C 55 01 01 00` | Enable eshifter |
| 3 | `0x10F11840` | `00 A9 FC 8C 44 00 GG 00` | Set initial gear (GG = gear) |
| 4 | `0x10F11840` | `00 A9 FC 8C 66 01 00 00` | Configuration mode |

After initialization, the eshifter responds with its status on `0x1840B110` and an empty ack on `0x1840B100`.

### Gear Change Sequence

The complete gear change flow:

```
1. COMMAND    0x18411840  01 01 MM GG 00          Set gear to GG (mode MM)
2. CONFIRM 1  0x10F11840  00 A9 FC 8C EE 00 01 00  Acknowledgment part 1
3. CONFIRM 2  0x10F11840  00 A9 FC 8C EF 00 01 00  Acknowledgment part 2
4. FEEDBACK   0x10F11841  00 00 GG 00 00 00 00 00  Echo target gear
5. FEEDBACK   0x10F11841  00 00 01 00 00 00 00 00  Ack
6. FEEDBACK   0x10F11841  00 00 XX 00 00 00 PL PH  Actuator moving (PL:PH = position)
   ... (repeated ~27 times as actuator reaches target) ...
7. STATUS     0x1840B110  DC 05 28 GG 60 09 14 05  Gear position updated
```

If the gear is already at the requested position, feedback frame may return `FF FF` in bytes 2-3 with `94 80` in bytes 6-7 (error/already-at-position indicator).

### Sender 0x0F1

The sender address 0x0F1 does not follow the standard `(PFSA & 0x7F) << 5` device encoding formula. It appears to be a system-level sensor bus address that provides data to multiple actuator devices:

- To eshifter: 0x840 (config), 0x841 (telemetry)
- To frontlight: 0x880 (init), 0x881-0x883 (config channels)
- To rearlight: 0x860 (init), 0x861-0x863 (config channels)
- To power_control: 0x460 (CBOR), 0x461-0x463 (telemetry channels)

### Gear Shift Mode Trigger

The value `0x3FD` serves as a gear shift mode indicator in CAN messages between devices. When detected by the user_ecu, it activates the gear change flow.

## Light Protocol (Frontlight & Rearlight)

The frontlight (0xC4 / PS 0x87 / encoded 0x880) and rearlight (0xC3 / PS 0x88 / encoded 0x860) use identical protocols. Both use raw binary framing (not CBOR) and share the same sub-addressing pattern as the eshifter.

### Light On-Wire CAN IDs

**Frontlight (encoded 0x880):**

| On-Wire CAN ID | Direction | Description |
|---|---|---|
| `0x01111880` | From frontlight | Heartbeat (8 bytes, all zeros) |
| `0x18827880` | To frontlight | Light commands (on/off/brightness) |
| `0x10F11880` | To frontlight | Init handshake |
| `0x10F11881` | To frontlight | Init config 1 |
| `0x10F11882` | To frontlight | Init config 2 |
| `0x10F11883` | To frontlight | Init config 3 (terminator) |
| `0x18805110` | From frontlight | Brightness status report |
| `0x18805100` | From frontlight | Status acknowledgment (empty) |
| `0x1880F110` | From frontlight | Feedback status |
| `0x1880F100` | From frontlight | Feedback acknowledgment (empty) |
| `0x18823110` | From frontlight | Detailed status report |

**Rearlight (encoded 0x860):**

| On-Wire CAN ID | Direction | Description |
|---|---|---|
| `0x01111860` | From rearlight | Heartbeat (8 bytes, all zeros) |
| `0x18627860` | To rearlight | Light commands (on/off/brightness) |
| `0x10F11860` | To rearlight | Init handshake |
| `0x10F11861` | To rearlight | Init config 1 |
| `0x10F11862` | To rearlight | Init config 2 |
| `0x10F11863` | To rearlight | Init config 3 (terminator) |
| `0x18605110` | From rearlight | Brightness status report |
| `0x18605100` | From rearlight | Status acknowledgment (empty) |
| `0x1860F110` | From rearlight | Feedback status |
| `0x1860F100` | From rearlight | Feedback acknowledgment (empty) |
| `0x18623110` | From rearlight | Detailed status report |

### Sub-Addressing

Each light uses 4 sub-addresses derived from its base encoded value:

| Sub-Address | Channel | Purpose |
|---|---|---|
| +0 (0x880/0x860) | Base | Commands and init handshake |
| +1 (0x881/0x861) | Config 1 | Initialization config data |
| +2 (0x882/0x862) | Config 2 | Extended config, command sender address |
| +3 (0x883/0x863) | Config 3 | Initialization terminator |

### Light Command

**CAN ID:** `0x18827880` (frontlight) / `0x18627860` (rearlight)

2-byte payload:

| Byte | Value | Description |
|---|---|---|
| 0 | `0x00`-`0x64` | Brightness level (0-100%) |
| 1 | `0x01` | Mode: on/solid |

Special off command: `FF FF` (both bytes 0xFF).

Observed brightness values: 0x3C (60%).

### Brightness Status Report

**CAN ID:** `0x18805110` (frontlight) / `0x18605110` (rearlight)

1-byte payload: current brightness percentage (0x0A to 0x64, i.e. 10-100 in steps of 10).

Each status report is followed by an empty acknowledgment frame on `0x18805100` / `0x18605100`.

### Feedback Status

**CAN ID:** `0x1880F110` (frontlight) / `0x1860F110` (rearlight)

1-byte payload: status byte (observed value: 0xC8 = 200).

### Detailed Status Report

**CAN ID:** `0x18823110` (frontlight) / `0x18623110` (rearlight)

7-byte payload:

| Byte | Frontlight | Rearlight | Description |
|---|---|---|---|
| 0 | `0x01` | `0x01` | Status flag |
| 1 | `0x00` | `0x00` | Reserved |
| 2 | `0x16` (22) | `0x16` (22) | Parameter |
| 3 | `0x3C` (60) | `0x3C` (60) | Current brightness level |
| 4 | `0x01` | `0x01` | Mode (on) |
| 5 | `0x4F` (79) | `0x1E` (30) | Device-specific parameter |
| 6 | `0x00` | `0x00` | Reserved |

### Initialization Sequence

On startup, the system initializes each light with a 4-frame sequence via sender 0x0F1:

| Step | Sub-Addr | Payload | Description |
|---|---|---|---|
| 1 | +0 | `3D 95 16 87 18 00 35 00` | Init handshake |
| 2 | +1 | `00 00 03 00 00 00 05 01` | Configuration data |
| 3 | +2 | `00 00 00 60 00 00 00 00` | Extended configuration |
| 4 | +3 | `00 00 00 00 00 00` | Terminator (6 bytes) |

Both frontlight and rearlight receive identical initialization data.

### Light Control Flow

```
1. COMMAND   0x18827880  3C 01              Set brightness to 60%, on
2. STATUS    0x18805110  3C                 Brightness confirmation
3. ACK       0x18805100  (empty)            Acknowledgment
4. FEEDBACK  0x1880F110  C8                 Status feedback
5. ACK       0x1880F100  (empty)            Feedback acknowledgment
6. DETAIL    0x18823110  01 00 16 3C 01 4F 00  Detailed status report

To turn off:
1. COMMAND   0x18827880  FF FF              Turn off light
```

## Battery Management System (BMS)

The SA5 battery uses a DynaPack SX3 BMS (VM13-147 series) with Panasonic cells in a 10S configuration. The S3/S4 used Modbus RTU over UART for BMS communication; the S5/S6 uses CAN bus instead. The register layout and fault protection logic remain identical across generations.

### Battery Specifications

| Parameter | Value |
|---|---|
| Cell Chemistry | Panasonic (Li-ion) |
| Cell Configuration | 10S (10 cells in series) |
| Cell Voltage Range | 2500 mV (low) — 4300 mV (high) |
| Pack Voltage Range | 25000 mV (low) — 43000 mV (high) |
| Cell Imbalance Warning | 20 mV (max-min) |
| BMS Firmware Format | `GRN_` header (Renesas MCU, not ARM Cortex-M) |
| SPIN Bootloader | Used for BMS firmware updates |

### BMS CAN IDs

The BMS uses 29-bit extended CAN IDs. These are **not** part of the standard bike CAN bus device-to-device protocol — they use a separate addressing scheme for direct BMS communication.

#### Core Communication

| CAN ID | Name | Direction | Description |
|---|---|---|---|
| `0x10801000` | Command/Heartbeat | Bidirectional | Main register read/write and heartbeat |
| `0x1077E400` | Write | To BMS | Write register data |
| `0x1077E800` | Write Variant | To BMS | Alternative write command |

#### Status Control (version-dependent)

| CAN ID | Firmware Version | Description |
|---|---|---|
| `0x00000030` | v001 | SetStatus (11-bit standard ID) |
| `0x149D9060` | v004 | SetStatus (29-bit extended) |
| `0x1489BB70` | v015, v016, v019, v020 | SetStatus (29-bit extended) |

#### Charger & USB-PD Control (v004+)

| CAN ID | Name | Description |
|---|---|---|
| `0x149DE9A0` | Set Charger State | Charger mode control |
| `0x149E4160` | Set USB-PD State | USB Power Delivery control |

#### SPIN Bootloader

| CAN ID | Name | Description |
|---|---|---|
| `0x10823800` | BL Update 1 | Bootloader update frame |
| `0x10823A00` | BL Update 2 | Bootloader data/ACK |
| `0x10823C00` | BL Update 3 | Bootloader version |
| `0x1077F200` | BL Data | Firmware data chunks |
| `0x1077F400` | BL ACK | Bootloader acknowledgment |
| `0x1077F600` | BL Version | Bootloader version query |

#### AP (Application) Firmware Update

| CAN ID | Name | Direction |
|---|---|---|
| `0x14901460` | Version Reply | From BMS |
| `0x14901470` | Version Request | To BMS |
| `0x14903460` | Flash Size Reply | From BMS |
| `0x14903470` | Flash Size Request | To BMS |
| `0x14905470` | Erase | To BMS |
| `0x14907470` | Write | To BMS |
| `0x14909460` | CRC Reply | From BMS |
| `0x14909470` | CRC Request | To BMS |
| `0x1490B460` | Update Reply | From BMS |
| `0x1490B470` | Update Command | To BMS |
| `0x1490D470` | Reboot | To BMS |

### SetStatus Command

The SetStatus command controls the BMS operating mode. It is sent to the version-specific SetStatus CAN ID with a 2-byte payload `[status_code, flags]`.

#### Status Codes (byte 0)

| Code | Name | Description |
|---|---|---|
| `0x01` | Normal | Normal operating mode |
| `0x02` | Charge | Enter charging mode |
| `0x03` | Sleep/Shipping | Enter lowest power state (ship mode) |
| `0x04` | Charge Off | Disable charging (v001) |
| `0x05` | Charge MOSFET Off | Normal mode with charge MOSFET disabled (v004+) |
| `0x08` | Standby | Standby mode (v015+) |
| `0x0C` | Set Charge On | Enable charging |
| `0x0D` | Charge Open | Charger mode / charge MOSFET open |
| `0x40` | Request BMS Info | Request BMS to broadcast all register data |
| `0x80` | Cell Balancing | Request cell balancing |

#### Protection Unlock Flags (byte 1)

| Code | Name | Description |
|---|---|---|
| `0x01` | Unlock COCP | Unlock over current protection |
| `0x02` | Unlock Voltage | Unlock voltage protection (OVP/UVP) |
| `0x04` | Unlock Temperature | Unlock temperature protection |

### Charger State Command (v004+)

Sent to CAN ID `0x149DE9A0`. Controls the charger subsystem.

| Byte 0 | Description | Additional Data |
|---|---|---|
| `0x01` | Standby Mode (v004) | — |
| `0x02` | Standby Mode (v010) / Charger Mode (v004) | Bytes 1-2: voltage (LE uint16 mV), Bytes 3-4: current (LE uint16 mA) |
| `0x08` | Request BMS Info | — |

### USB-PD State Command (v004+)

Sent to CAN ID `0x149E4160`. Controls USB Power Delivery.

| Byte 0 | Name | Description |
|---|---|---|
| `0x01` | Disable Source | Disable USB-PD source mode |
| `0x02` | Allow Source | Enable USB-PD source mode (power bank output) |
| `0x04` | Disable Sink | Disable USB-PD sink mode |
| `0x08` | Allow Sink | Enable USB-PD sink mode (charging input) |

### BMS Passive Registers

The BMS broadcasts register data in response to a Request BMS Info command (`0x40`). The register layout matches the S3 BMS register map. Data is little-endian.

#### Live Status Registers (0x02 — 0x2C)

| Addr | Name | Type | Formula | Unit |
|---|---|---|---|---|
| 0x02 | Fault Status | H (16-bit flags) | — | hex |
| 0x03 | Battery Temperature | U16 | (x-2731)/10 | °C |
| 0x04 | Battery Voltage | U16 | — | mV |
| 0x05 | RSOC | U16 | — | % |
| 0x06 | Current | I16 | x×10 | mA |
| 0x07 | Charging Status | H (flags) | — | hex |
| 0x08 | Discharging Status | H (flags) | — | hex |
| 0x09 | Test Mode | H | — | hex |
| 0x0A | Hardware Version | H | — | hex |
| 0x0B | Software Version | H | — | hex |
| 0x0C-0x12 | ESN | ASCII | — | 14 chars (7 regs) |
| 0x13-0x14 | Manufacture Date | DATE | — | [0x00, YY, MM, DD] |
| 0x15 | Normal Capacity | U16 | — | mAh |
| 0x16 | Full Charge Capacity | U16 | — | mAh |
| 0x17 | Remaining Capacity | U16 | — | mAh |
| 0x18 | Absolute SOC | U16 | — | % |
| 0x19 | Cycle Count | U16 | — | count |
| 0x1A | CHG MOS Control | H (flags) | — | hex |
| 0x1B | Cell 1 Voltage | U16 | — | mV |
| 0x1C | Cell 2 Voltage | U16 | — | mV |
| 0x1D | Cell 3 Voltage | U16 | — | mV |
| 0x1E | Cell 4 Voltage | U16 | — | mV |
| 0x1F | Cell 5 Voltage | U16 | — | mV |
| 0x20 | Cell 6 Voltage | U16 | — | mV |
| 0x21 | Cell 7 Voltage | U16 | — | mV |
| 0x22 | Cell 8 Voltage | U16 | — | mV |
| 0x23 | Cell 9 Voltage | U16 | — | mV |
| 0x24 | Cell 10 Voltage | U16 | — | mV |
| 0x25 | Battery Temp Sensor 1 | U16 | (x-2731)/10 | °C |
| 0x26 | Battery Temp Sensor 2 | U16 | (x-2731)/10 | °C |
| 0x27 | DSG-MOS Temperature | U16 | (x-2731)/10 | °C |
| 0x28 | Warning Status | H (flags) | — | hex |
| 0x29 | Max Cell Voltage | U16 | — | mV |
| 0x2A | Min Cell Voltage | U16 | — | mV |
| 0x2B | Cell Balance | U16 | — | — |
| 0x2C | Bootloader Version | H | — | hex |

#### DataFlash Registers (0x30 — 0x5B)

Stored fault records and protection trigger counts. Snapshot of state at last fault event.

| Addr | Name | Type | Formula | Unit |
|---|---|---|---|---|
| 0x30 | Fault Status Record | H (flags) | — | hex |
| 0x31 | Battery Temp1 Record | U16 | (x-2731)/10 | °C |
| 0x32 | Battery Temp2 Record | U16 | (x-2731)/10 | °C |
| 0x33 | MOS Temp Record | U16 | (x-2731)/10 | °C |
| 0x34 | Battery Voltage Record | U16 | — | mV |
| 0x35 | Current Record | I16 | x×10 | mA |
| 0x36 | Full Charge Capacity Record | U16 | — | mAh |
| 0x37 | Remaining Capacity Record | U16 | — | mAh |
| 0x38 | RSOC Record | U16 | — | % |
| 0x39 | Absolute SOC Record | U16 | — | % |
| 0x3A | Cycle Count Record | U16 | — | count |
| 0x3B-0x44 | Cell 1-10 Voltage Record | U16 | — | mV |
| 0x45 | Max Battery Voltage Record | U16 | — | mV |
| 0x46 | Min Battery Voltage Record | U16 | — | mV |
| 0x47 | DOTP Trigger Count | U16 | — | count |
| 0x48 | DUTP Trigger Count | U16 | — | count |
| 0x49 | COTP Trigger Count | U16 | — | count |
| 0x4A | CUTP Trigger Count | U16 | — | count |
| 0x4B | DOCP1 Trigger Count | U16 | — | count |
| 0x4C | DOCP2 Trigger Count | U16 | — | count |
| 0x4D | COCP1 Trigger Count | U16 | — | count |
| 0x4E | COCP2 Trigger Count | U16 | — | count |
| 0x4F | OVP1 Trigger Count | U16 | — | count |
| 0x50 | OVP2 Trigger Count | U16 | — | count |
| 0x51 | UVP1 Trigger Count | U16 | — | count |
| 0x52 | UVP2 Trigger Count | U16 | — | count |
| 0x53 | PDOCP Trigger Count | U16 | — | count |
| 0x54 | PDSCP Trigger Count | U16 | — | count |
| 0x55 | MOTP Trigger Count | U16 | — | count |
| 0x56 | SCP Trigger Count | U16 | — | count |
| 0x57 | Max Charge Current Record | I16 | x×10 | mA |
| 0x58 | Max Discharge Current Record | I16 | x×10 | mA |
| 0x59 | Max Cell Temperature Record | I16 | (x-2731)/10 | °C |
| 0x5A | Min Cell Temperature Record | I16 | (x-2731)/10 | °C |
| 0x5B | Max MOS Temperature Record | I16 | (x-2731)/10 | °C |

### Fault Status Flags (Register 0x02) — 16-bit

| Bit | Flag | Full Name |
|---|---|---|
| 0 | DOTP | Discharge Over Temperature Protection |
| 1 | DUTP | Discharge Under Temperature Protection |
| 2 | COTP | Charging Over Temperature Protection |
| 3 | CUTP | Charging Under Temperature Protection |
| 4 | DOCP1 | Discharge Over Current Protection Level 1 |
| 5 | DOCP2 | Discharge Over Current Protection Level 2 |
| 6 | COCP1 | Charging Over Current Protection Level 1 |
| 7 | COCP2 | Charging Over Current Protection Level 2 |
| 8 | OVP1 | Over Voltage Protection Level 1 (cell) |
| 9 | OVP2 | Over Voltage Protection Level 2 (cell) |
| 10 | UVP1 | Under Voltage Protection Level 1 (cell) |
| 11 | UVP2 | Under Voltage Protection Level 2 (cell) |
| 12 | PDOCP | Peak Discharge Over Current Protection |
| 13 | PDSCP | Peak Discharge Short Circuit Protection |
| 14 | MOTP | MOSFET Over Temperature Protection |
| 15 | SCP | Short Circuit Protection |

### Warning Status Flags (Register 0x28) — 16-bit

| Bit | Flag | Full Name |
|---|---|---|
| 0 | DOTPW | Discharge Over Temperature Warning |
| 1 | DUTPW | Discharge Under Temperature Warning |
| 2 | COTPW | Charging Over Temperature Warning |
| 3 | CUTPW | Charging Under Temperature Warning |
| 4 | DOCPW | Discharge Over Current Warning |
| 5 | — | Reserved |
| 6 | COCPW | Charging Over Current Warning |
| 7 | — | Reserved |
| 8 | OVP1W | Over Voltage Warning Level 1 |
| 9 | — | Reserved |
| 10 | UVP1W | Under Voltage Warning Level 1 |
| 11 | SOC | State of Charge Low Warning |
| 12 | PDOCPW | Peak Discharge Over Current Warning |
| 13 | — | Reserved |
| 14 | MOTPW | MOSFET Over Temperature Warning |
| 15 | — | Reserved |

### Charging Status Flags (Register 0x07) — 16-bit

| Bit | Flag | Description |
|---|---|---|
| 0 | CHG | Charging active |
| 1 | Fault | Charging fault detected |
| 2 | CHG_IN | Charger input detected |
| 3-15 | — | Reserved |

### Discharging Status Flags (Register 0x08) — 16-bit

| Bit | Flag | Description |
|---|---|---|
| 0 | DSG | Discharging active |
| 1-15 | — | Reserved |

### Conversion Formulas

| Parameter | Formula | Example |
|---|---|---|
| Temperature | (raw - 2731) / 10 | 3000 → 26.9°C |
| Current | raw × 10 | 100 → 1000 mA |
| Voltage | raw (direct) | 36500 → 36500 mV |
| SOC / Capacity | raw (direct) | 85 → 85% |
| Date | byte[1]=year, byte[2]=month, byte[3]=day | — |

Data types: `U` = unsigned, `I` = signed (two's complement), `H` = hex (big-endian display), `S` = ASCII string, `D` = date, `B` = BCD.

### SPIN Bootloader Protocol

The BMS uses a SPIN bootloader for firmware updates. The bootloader commands are defined in the BMS configuration.

#### Bootloader Commands

| Command | Description |
|---|---|
| HEARTBEAT | Bootloader keepalive |
| VERSIONS | Query bootloader and application versions |
| ENTER_BOOTLOADER | Enter bootloader mode |
| EXIT_BOOTLOADER | Exit bootloader, jump to application |
| START_UPLOAD | Begin firmware upload |
| REQUEST_CHUNK | BMS requests next data chunk |
| SEND_CHUNK | Send firmware data chunk (6 bytes per CAN frame) |
| BMS_SYSTEM_SHUT_DOWN | System shutdown command |

#### Bootloader Heartbeat Status

| Value | State |
|---|---|
| 0 | Waiting |
| 1 | Erasing flash |
| 2 | Uploading firmware |
| 3 | Validating CRC |
| 4 | Ready to jump to application |
| 21 (0x15) | Error |

#### AP Firmware Update Flow

```
1. Send VERSION request to 0x14901470
2. Receive version reply from 0x14901460 (BL ver + AP ver, 6 bytes)
3. Send FLASH_SIZE request to 0x14903470
4. Receive flash size reply from 0x14903460
5. Send ERASE command to 0x14905470
6. Send WRITE commands to 0x14907470 (firmware data in chunks)
7. Send CRC request to 0x14909470
8. Receive CRC reply from 0x14909460
9. Send UPDATE command to 0x1490B470
10. Receive UPDATE reply from 0x1490B460
11. Send REBOOT to 0x1490D470
```

### BMS Internal Commands

The BMS accepts extended internal commands for calibration, diagnostics, and configuration. These require a preamble handshake before each command.

**Preamble:** Send 8-byte ASCII `"DynaPack"` to the command CAN ID, then send the command after a 5ms delay to the same CAN ID.

#### Internal Command Codes

| Code | Name | Byte 1 | Description |
|---|---|---|---|
| 0x00 | Set RTC | ss,mm,HH,dd,MM,yy,0x00 | Set real-time clock |
| 0x0E | Unlock PF | 0x01 | Unlock permanent failure lockout |
| 0x14 | Discharge Control | 0/1 | 0=off, 1=on — control discharge MOSFET |
| 0x15 | Charge Control | 0/1 | 0=off, 1=on — control charge MOSFET |
| 0x18 | Notice Control | 0/1 | 0=enable broadcasts, 1=disable |
| 0x1B | Set SN Part 1 | 7 ASCII chars | First 7 characters of 13-char serial number |
| 0x1C | Set SN Part 2 | 6 ASCII chars | Last 6 characters of serial number |
| 0x1D | Set HW Ver Part 1 | 7 ASCII chars | First 7 characters of HW version |
| 0x1E | Set HW Ver Part 2 | 7 ASCII chars | Characters 8-14 of HW version |
| 0x1F | Set HW Ver Part 3 | 2 ASCII chars | Characters 15-16 of HW version |
| 0x22 | Clear Error Log | 0x00 | Clear all stored fault records |
| 0x23 | Coulomb Counter Check | 0x00 | Trigger coulomb counter verification |
| 0x24 | External Watchdog | 0/1 | 0=enable, 1=disable (v019+) |
| 0x25 | DCDC Voltage Offset | LE int16 | Set DC-DC converter voltage offset (v010+) |
| 0x27 | DCDC Current Offset | LE int16 | Set DC-DC converter current offset (v010+) |

### BMS Feature Control

Additional BMS features are controlled via CAN ID `0x000002FF` with a single command byte.

| Byte 0 | Description |
|---|---|
| 0x02 | Disable PDSCP (Peak Discharge Short Circuit Protection) |
| 0x03 | Enable PDSCP |
| 0x04 | Disable Cell OffLine detection |
| 0x05 | Enable Cell OffLine detection |
| 0x06 | Disable Cell Balancing |
| 0x07 | Enable Cell Balancing |

### BMS Version History

| Version | Key Changes |
|---|---|
| v001 | Basic status control (Normal, Charge, Sleep, Charge On/Off) |
| v004 | Added Charger State and USB-PD State commands, PowerBank support |
| v010 | Added StandBy/Charger/Normal modes for PowerBank, DCDC offsets |
| v015 | Standby mode, Charge Voltage Offset, Clear Error Log, CoulombCounter |
| v016 | Same as v015 with minor protocol changes |
| v019 | Added External Watchdog control |
| v020 | Same as v019, latest version |

### power_control Device

The `power_control` device (PF=SA=0xA3) manages battery power distribution on the main CAN bus. It has limited bus access (7 handlers) and communicates with unknown subsystems via non-standard PS addresses:

| CAN ID | Target | Notes |
|---|---|---|
| `0x01A301A3` | PS=0x01 | Unknown subsystem |
| `0x01A302A3` | PS=0x02 | Unknown subsystem |
| `0x01A304A3` | PS=0x04 | Unknown subsystem |

The power_control firmware (`power_control.20240129.145222.1.5.0.main.v1.5.0-main.bin`) runs FreeRTOS with tasks including `power_control_task`, `PwrQ`, and `CanTX`.

Special purpose CAN IDs from power_control to unknown devices:

| CAN ID | Description |
|---|---|
| `0x00A301A4` — `0x00A307A4` | Commands to PF=0xA4 (possibly battery/charger subsystem) |
| `0x00A310A7` — `0x00A312A7` | Commands to PF=0xA7 (possibly battery monitoring) |

### Charger Bus

The charger operates on a **separate CAN bus segment**, not the main bike bus.

| Device | PF=SA | CAN ID | Notes |
|---|---|---|---|
| charger (Liteon) | 0x70 | `0x01708D70` | Sends to charger_target |
| charger_target | 0x8D | — | Battery BMS / power delivery unit |

The charger CAN bus is physically isolated from the main bus. The `charger_target` (0x8D) is the BMS or a power delivery unit that the Liteon charger communicates with directly.

> **Note:** The BMS-specific CAN IDs (0x10801000, 0x1077E400, 0x1489BB70, etc.) are used for direct BMS communication over this separate charger bus, not the main bike CAN bus. They do not appear in main bus captures.

### Battery Telemetry on Main Bus

Battery status is relayed to the main CAN bus via the user_ecu device. The user_ecu reads BMS register data and repackages it into a compact 8-byte frame broadcast on the main bus. These frames are visible in standard captures.

**CAN ID: `0x18009800`** — user_ecu battery telemetry (class 0x9)

8-byte payload with alternating data/padding bytes:

| Byte | Description | Unit | Example Values |
|---|---|---|---|
| 0 | State of Charge (SoC) | % (0-100) | 0x00 (init), 0x58 (88%), 0x59 (89%), 0x5D (93%) |
| 1 | Padding | — | Always 0x00 |
| 2 | Current draw | raw (x10 = mA) | 0x0B (110mA), 0x0C (120mA) |
| 3 | Padding | — | Always 0x00 |
| 4 | Battery temperature | Fahrenheit | 0x45 (69F=20.6C), 0x46 (70F=21.1C) |
| 5 | Padding | — | Always 0x00 |
| 6-7 | Pack voltage (LE uint16) | /10 = volts | 0x0179 (37.7V), 0x01B5 (43.7V) |

**Field details:**

- **SoC** (byte 0): Relative state of charge percentage from BMS register 0x05. First frame after power-on may be 0x00 before BMS reports. Observed range 55-93% across captures.
- **Current** (byte 2): Battery current from BMS register 0x06. Raw value multiplied by 10 gives milliamps. Quiescent draw when locked is 110-120mA (0x0B-0x0C).
- **Temperature** (byte 4): Battery temperature in degrees Fahrenheit. Observed values 0x45-0x46 (69-70 decimal) = 20.6-21.1C, consistent with room temperature ambient. The BMS stores temperature in decikelvin (register 0x03); the firmware converts to Fahrenheit before broadcast. Changes slowly between captures, consistent with the thermal mass of the pack.
- **Pack voltage** (bytes 6-7): Total pack voltage as LE uint16, divide by 10 for volts. For a 10S Li-ion pack, valid range is 25.0V (discharged) to 43.0V (fully charged). Observed range in captures: 27.6V to 43.7V.

**CAN ID: `0x18009801`** — Continuation frame (2 bytes: `01 00`)

These frames are broadcast continuously (~every few seconds) even when the bike is locked.

### Battery BLE Protocol

The bike exposes battery data over Bluetooth Low Energy. The app reads these characteristics to display battery status.

#### BLE Battery Characteristic UUIDs

**SA5/S6 (XS4 model):**

| UUID | Name | Data |
|---|---|---|
| `278D5541-4692-039F-3445-A23FC55333D0` | Motor Battery Level | Integer 0-100 (%) |
| `278D5542-4692-039F-3445-A23FC55333D0` | Motor Battery State | Charging state enum |
| `278D5543-4692-039F-3445-A23FC55333D0` | Module Battery Level | Integer 0-100 (%) |
| `278D5544-4692-039F-3445-A23FC55333D0` | Module Battery State | Charging state enum |
| `278D5550-4692-039F-3445-A23FC55333D0` | Battery Firmware Version | String |

**S3/Pro (older models):**

| UUID | Name |
|---|---|
| `6ACC5541-E631-4069-944D-B8CA7598AD50` | Motor Battery Level |
| `6ACC5542-E631-4069-944D-B8CA7598AD50` | Motor Battery State |
| `6ACC5543-E631-4069-944D-B8CA7598AD50` | Module Battery Level |
| `6ACC5544-E631-4069-944D-B8CA7598AD50` | Module Battery State |
| `6ACC5550-E631-4069-944D-B8CA7598AD50` | Battery Firmware Version |

#### Battery Charging State Enum

| Value | State | Description |
|---|---|---|
| 0x00 | DISCHARGING | Battery is discharging (normal use) |
| 0x01 | CHARGING | Charger connected, actively charging |
| 0x02 | FULL | Battery fully charged |
| 0x03 | ERROR | Battery fault / charging error |
| 0x04 | UNKNOWN | State cannot be determined |

#### BLE Battery Data Types

| Data | Type | Range |
|---|---|---|
| Battery Level | Integer | 0-100 (percentage) |
| Battery Health | Integer | 0-100 (percentage) |
| Charge Cycles | Integer | 0+ (count) |
| Charging State | Byte | 0x00-0x04 (enum above) |

#### BLE Message Topic Codes (Battery-Related)

| Code | Hex | Description |
|---|---|---|
| 353 | 0x0161 | Battery state update |
| 354 | 0x0162 | Charging state |
| 355 | 0x0163 | Power level |
| 356 | 0x0164 | Battery health |
| 362 | 0x016A | Extended battery info |

#### BLE Service UUIDs

**SA5/S6:**

| UUID | Service |
|---|---|
| `278D5500-4692-039F-3445-A23FC55333D0` | Security Service |
| `278D5540-4692-039F-3445-A23FC55333D0` | Vehicle Info Service (contains battery) |
| `278D5560-4692-039F-3445-A23FC55333D0` | Vehicle State Service |

## Elock Protocol (Electronic Lock)

The elock (0xC1 / PS 0x85 / encoded 0x820) controls the bike's electronic barrel lock. It uses raw binary framing (not CBOR) for status broadcast and communicates with the BLE module for lock/unlock operations.

### Elock On-Wire CAN IDs

| On-Wire CAN ID | Direction | Description |
|---|---|---|
| `0x01111820` | From elock | Heartbeat (8 bytes, all zeros, ~8.3s interval) |
| `0x18209820` | From elock | Lock state broadcast (class 0x9, self-addressed, 1-byte payload) |
| `0x18203110` | From elock | Data report (class 0x3, to dest 0x110, variable payload) |
| `0x18203100` | From elock | Data acknowledgment (class 0x3, to dest 0x100, empty) |

### Elock Special Purpose CAN IDs (DP=0)

| CAN ID | Description |
|---|---|
| `0x00C18887` | Light pair sync (byte 0 = 0x87 frontlight, byte 1 = 0x88 rearlight) |
| `0x00C10382` | BLE command (CMD=0x03 to BLE module 0x82) |
| `0x00C10182` | BLE command (CMD=0x01 to BLE module 0x82) |
| `0x00C101A1` | CMD=0x01 to motor_sensor (motion detection for alarm) |
| `0x00C10BA2` | CMD=0x0B to power_pedal (disable pedal assist when locked) |

### Lock State Byte

**CAN ID:** `0x18209820` (elock self, class 0x9)

1-byte payload broadcast continuously (~every 2-4 seconds). Maps to the S6 `ElectrifiedS6ELockState` enum (BLE topic 362 / 0x016A):

| Value | State | Description |
|---|---|---|
| `0x00` | UNKNOWN | Initial/undetermined state |
| `0x01` | LOCKED | Lock barrel is engaged |
| `0x02` | UNLOCKED | Lock barrel is disengaged |
| `0x03` | STUCK | Lock barrel failed to extend/retract (app shows "kick lock is stuck" notification) |

Note: This is the **e-lock state** (physical kick lock mechanism), distinct from the **physical lock state** (topic 352: UNLOCKED=0, LOCKED=1, UNLOCKING=2).

Observed in captures:
- All `bikelocked*.log` captures show consistent `0x01` (LOCKED)
- `startup_from_app.log` shows transition from `0x01` to `0x03` (STUCK — lock attempted to open but mechanism jammed)

### Data Report Frame

A data report is a short **multi-frame stream**: a start frame (ID ending
`…10`), zero or more continuation frames (`…11`), and a terminating empty
acknowledgment (`…00`). The destination `0x110` is a broadcast/status address
(same pattern as eshifter's `0x1840B110`).

| Role | CAN ID | Notes |
|---|---|---|
| Report (start) | `0x18203110` / `0x18023110` | up to 8 bytes |
| Continuation | `0x18203111` / `0x18023111` | ≤7 bytes, appended in order |
| Acknowledgment | `0x18203100` / `0x18023100` | empty, ends the stream |

Two ID spellings occur in the captures (`0x1820_31xx` and `0x1802_31xx`); both
are decoded. Observed payloads are plaintext (e.g. `8877665544332211`, and small
status/counter bytes during the unlock exchange) — see *Elock Security on the CAN
Bus*. The decoder reassembles start + continuations into a single payload; field
semantics are not interpreted (undocumented).

### Lock/Unlock Flow

The normal unlock flow goes through BLE: the app authenticates with the BLE module, which signals the user_ecu to broadcast the unlock command on CAN. However, the CAN command itself is a simple unencrypted broadcast that can be sent directly on the bus without BLE authentication.

**CAN Unlock Command:**

The unlock is triggered by a system state broadcast on the CAN bus:

| CAN ID | Payload | Description |
|---|---|---|
| `0x1800D110` | `0x01` | System state broadcast: unlock (user_ecu -> broadcast, class 0xD) |
| `0x1800D100` | empty | Acknowledgment frame |

This CAN ID uses class marker 0xD (system state command) with destination 0x110 (broadcast address). The elock listens for this broadcast in promiscuous mode and triggers the unlock mechanism when it receives payload `0x01`.

This message only appears in `startup_from_app.log` (app-initiated startup with unlock attempt). It does not appear in any `bikelocked*.log` captures (idle locked state).

**BLE Defence Service UUIDs (SA5/S6):**

| UUID | Name | Description |
|---|---|---|
| `278D5520-4692-039F-3445-A23FC55333D0` | Defence Service | Lock control service |
| `278D5521-4692-039F-3445-A23FC55333D0` | Lock State | Current lock state (read/notify) |
| `278D5522-4692-039F-3445-A23FC55333D0` | Unlock Request | Write to request unlock |
| `278D5523-4692-039F-3445-A23FC55333D0` | Alarm State | Alarm triggered state |
| `278D5524-4692-039F-3445-A23FC55333D0` | Alarm Mode | Alarm sensitivity mode |

**BLE Security Service UUIDs (authentication required before unlock):**

| UUID | Name | Description |
|---|---|---|
| `278D5500-4692-039F-3445-A23FC55333D0` | Security Service | Authentication service |
| `278D5501-4692-039F-3445-A23FC55333D0` | Challenge Code | Challenge for authentication |
| `278D5502-4692-039F-3445-A23FC55333D0` | Key Index | Encryption key selection |
| `278D5503-4692-039F-3445-A23FC55333D0` | Backup Code | Backup unlock code |

**S6 Topic-Based Lock Control (via BLE pub/sub protocol):**

The S6 uses a topic-based messaging protocol (`ElectrifiedS6Message`) over a single BLE characteristic (`df286101-1440-48a1-88f1-247d7f7d274d`). Lock-related topics:

| Topic ID | Hex | Description |
|---|---|---|
| 352 | 0x0160 | Physical lock state (UNLOCKED=0, LOCKED=1) |
| 353 | 0x0161 | Lock command (SET with empty payload = lock) |
| 355 | 0x0163 | Unlock request notification |
| 362 | 0x016A | E-lock (kick lock) state (UNKNOWN/LOCKED/UNLOCKED/STUCK) |

Message format: `[fragment_byte][type|qos|retain][topic_id:2][payload_len:2][payload...][crc16:2]`

Message types: ACK=0, PUBLISH=1, SUBSCRIBE=2, UNSUBSCRIBE=3, GET=4, SET=5, OWN=6, SYNC=7

**Unlock sequence (observed in `startup_from_app.log`):**

```
1. App connects via BLE and authenticates (challenge-response on Security Service)
2. App sends SET message to topic 24 with module state ON (unlock)
3. BLE module signals user_ecu; user_ecu broadcasts 0x1800D110#01 (class 0xD system state)
4. Elock receives broadcast and attempts unlock; immediately sends data reports on 0x18023110
5. Elock status transitions on 0x18209820:
   - 0x01 (LOCKED) -> 0x02 (UNLOCKED) on success
   - 0x01 (LOCKED) -> 0x03 (STUCK) if mechanism jams
6. Elock sends CMD to motor_sensor (0x00C101A1) and power_pedal (0x00C10BA2)
```

**Observed timeline from `startup_from_app.log` (lines 300-314):**

```
0x1800D110#01           <- user_ecu broadcasts system state (unlock trigger)
0x18023110#0000000...01 <- elock data report (immediate response)
0x1800D100#             <- ack
0x18023111#00005C030800 <- elock data continuation
0x18023110#0000000...01 <- elock data report
0x18023111#000063030100 <- elock data continuation
0x18023100#             <- elock ack
...
0x18209820#03           <- elock status: STUCK (unlock attempted, mechanism jammed)
```

### Elock Security on the CAN Bus

**The elock unlock is not cryptographically protected on the CAN bus.** All
observed elock traffic is plaintext, and the unlock is a plain unauthenticated
broadcast:

- **Unlock = `0x1800D110#01`** (class 0xD system-state broadcast). The elock acts
  on this directly; there is no challenge/response, signature or encryption on
  the CAN side. Anyone on the bus can send it (the BLE authentication only gates
  the app → user_ecu step; it does not protect the resulting CAN command).
- **Lock-state broadcasts** (`0x18209820`) and the **data-report stream**
  (`0x18023110`/`…3111`/`…3100`) are plaintext — the observed payloads are small
  counters/status bytes and a fixed `8877665544332211` placeholder, not
  ciphertext. No encrypted frame has been seen in any capture.
- The lock holds its own key/secret material **internally**; it is never carried
  on the CAN bus and is **not required to unlock**. Consequently a replacement
  elock works **without any bus-side re-keying** — there is nothing to update on
  the Module or the bus when the lock is swapped.

> Earlier notes in this file described an on-bus "AES layer with 4 key slots" and
> a CAN decrypt path. That is not borne out by the captures — no elock CAN frame
> is encrypted, and the unlock path carries no key. The earlier per-function
> addresses were misattributed and have been removed.

> A "Key Index" exists on the **BLE** Defence/Security service, not on CAN. BLE
> key/auth behaviour is out of scope here — see `BLE.md`.

### Elock Heartbeat

**CAN ID:** `0x01111820`

8-byte payload, all zeros. Broadcast at ~8.3 second intervals. The heartbeat continues regardless of lock state.

### Timing

| Frame | Interval |
|---|---|
| Heartbeat (`0x01111820`) | ~8.3 seconds |
| Status (`0x18209820`) | ~2-4 seconds (appears twice per heartbeat cycle) |

## Object Dictionary (OD) Addressing

Battery, charger and power-control traffic uses an Object-Dictionary view of the
same 29-bit extended ID. The ID decomposes into four packed fields:

```
can_id = (a0 << 21) | (a1 << 13) | (a2 << 5) | (a3 & 0x1F)   // 8 + 8 + 8 + 5 = 29 bits

a0 = (id >> 21) & 0xFF      // node / device id        bits 28..21
a1 = (id >> 13) & 0xFF      // sub-system / signal idx  bits 20..13
a2 = (id >>  5) & 0xFF      // port / message class     bits 12..5
a3 =  id        & 0x1F      // sub-id / register        bits  4..0
```

`a0` is the node octet (it matches the device PF=SA for the physical ECUs).
A signal is matched on `(a0, a1)` — the node plus the signal index. The `0x10`
bit of `a3` is the **request/response direction flag** (set = read-request that
triggers a publish; clear = incoming data). Multi-byte OD payload fields are
**big-endian** on the wire.

Worked example: `0x14807040` → `a0=0xA4 (battery), a1=0x03 (charging),
a2=0x82, a3=0x00`.

## Device / OD Node ID Table

The firmware maps device-name tokens to a single OD/CAN node byte. The physical
CAN ECUs match the PF=SA values in the device table above; the remaining codes
are i.MX8-side software service endpoints in the same address space.

| Node | Device / service | | Node | Device / service |
|---|---|---|---|---|
| 0x00 | core_services | | 0xA1 | motor_sensor |
| 0x08 | heartbeat | | 0xA2 | power_pedal |
| 0x82 | power | | 0xA3 | power_control |
| 0x84 | ride | | 0xA4 | **battery_primary** |
| 0x87 | logging | | 0xA5 | **battery_secondary** |
| 0x88 | ux | | 0xA7 | **charger** |
| 0x8A | update | | 0xC0 | user_ecu |
| 0x8B | mqtt_od_bridge | | 0xC1 | elock |
| 0x8C | mqtt_ftp | | 0xC2 | eshifter |
| 0x8D | motor_control | | 0xC3 | rearlight |
| 0x8F | imx8_bridge | | 0xC4 | frontlight |
| 0x90 | ble | | 0xE0 | phone |
| 0x91 | modem | | 0xE2 | backoffice |
| 0xFB | ping | | 0xFC | dummy |
| 0xFD | battery_test | | 0xFE | raspberry |
| 0xFF | test | | | |

Tokens also accept a dash-form spelling (e.g. `MOTOR-CONTROL` as well as
`MOTOR_CONTROL`). `TEST` resolves to 0xFF.

> Note: the OD node bytes for `ble` (0x90), `modem` (0x91) and `motor_control`
> (0x8D) are the i.MX8 OD routing codes and differ from the on-wire CAN PF=SA
> values used in the device table at the top of this document.

### OD address structure (on-wire telemetry IDs)

An OD telemetry CAN ID packs four octet fields:

    id  =  a0<<21 | a1<<13 | a2<<5 | class
    a0    = node      (bits 28:21) — the publishing node
    a1    = signal    (bits 20:13) — the signal / object index
    a2    = peer/port (bits 12:5)  — peer node / port (0x82 power, 0x08 broadcast, self, …)
    class = sub-type  (bits 4:0)   — message class / sub-type

Verified against the documented signals (battery `0x14811040` → a0=0xA4 a1=0x08
a2=0x82; power_pedal switch `0x14415040` → a0=0xA2 a1=0x0A a2=0x82). The decoder
annotates every recognised OD frame (both in the main dump and the `*-check`
commands) with this breakdown, e.g.

    OD a0=0x87(logging) a1=0x88 a2=0xC2(eshifter) class=0x02   # eshifter log record -> logging node
    OD a0=0xA2(power_pedal) a1=0x0A a2=0x82(power) class=0x10   # power_switch_control_init

## Battery Primary (Node 0xA4) — OD Telemetry

The battery pack publishes a contiguous run of OD signals on the main bus
(`a0 = 0xA4`, `a2 = 0x82`, `a3 = 0`). Signals are keyed by the `a1` index.

| Signal | a1 | DLC | CAN ID | Decoded fields |
|---|---|---|---|---|
| charging | 0x03 | 5 | `0x14807040` | charge_current = u16[0:2] (mA); charge_voltage = u16[2:4] (mV) |
| cell | 0x04 | 5 | `0x14809040` | per-cell series (no-op decoder) |
| capacity | 0x05 | 7 | `0x1480B040` | soc = u16[0:2]; soc_app = remap(u8[0]) |
| warning | 0x06 | 6 | `0x1480D040` | ~20 alarm booleans (bit-field over bytes 0..4) |
| status | 0x07 | 3 | `0x1480F040` | ~40 status booleans; charging flag, state nibble |
| voltage | 0x08 | 8 | `0x14811040` | voltage = u16[0:2] (mV) |
| temperature | 0x09 | 26 | `0x14813040` | temps{cell1,cell2,chg MOS,dsg MOS} = u8[0..3]; discharge_current = u16[4:6]; max_current = u16[6:8]; power = (I/1000)·(V/1000) |
| health | 0x0A | 8 | `0x14815040` | health = u16[4:6]; cycles = u16[6:8] |

The `temperature` signal's logical record is 26 bytes (multi-frame reassembled);
a single captured frame carries the leading 8 bytes (cell temps + currents).

## Battery / Charger Commands (Node 0xA3 → 0xA4)

The power-control board (node 0xA3) relays a 1-byte opcode in `frame[0]` to the
battery. Command channel CAN ID `0x14603040` (`a1 = 0x01`).

| Opcode | Command | | Opcode | Command |
|---|---|---|---|---|
| 0x00 | battery OFF | | 0x06 | battery RESET |
| 0x01 | battery ON | | 0x08 | shipping mode |
| 0x05 | IdentifyCharger | | 0x09 | clear fault flags |

## Charger Node (0xA7)

The charger lives on its own OD node `0xA7` (`a1 = 0x11`) on the main bus.

| Frame | CAN ID | Data | Meaning |
|---|---|---|---|
| state_init | `0x14E23040` | — | OD init/handshake on the charger node |
| clear test #1 | `0x14E23214` | `A5 5A 00` | clear factory test & burn-in mode (sent first) |
| clear test #2 | `0x14E23210` | `A5 5A 00` | same routine (sent second); only `a3` differs |

The two `0x14E232xx` frames are raw (sent via `cansend`, not the OD path).
`A5 5A` is the fleet's unlock/command magic.

## Power-Control OD Signals

| OD signal | a0 | a1 | CAN ID | kind |
|---|---|---|---|---|
| power_state | 0x82 | 0x01 | `0x10403041` | multi-frame |
| power_control control/state | 0xA3 | 0x02 | `0x14605040` | multi-frame |
| power_control measurements | 0xA3 | 0x04 | `0x14609040` | multi-frame |
| power_pedal switch control init | 0xA2 | 0x0A | `0x14415040` | polled |
| battery/charger command | 0xA3 | 0x01 | `0x14603040` | request |
| rear-carrier query | 0xC3 | 0x80 | `0x1870F040` | — |

The whole battery/power bus uses `a2 = 0x82`.

## Power-Pedal (Node 0xA2) — OD Telemetry & Bench Tooling

power_pedal is the pedal-assist / torque + cadence sensor (PF=SA=0xA2, PS=0x92).
It is alive when its heartbeat `0x01111440` is on the bus, and it publishes OD
signals on node `0xA2`.

**OD telemetry publish format** (verified): a power_pedal OD signal at index `a1`
is published on

    CAN ID = 0x14401040 | (a1 << 13)        (a0 = 0xA2, a2 = 0x82, port form 0x1040)

This matches the one named power_pedal OD signal in the i.MX8 `vm` registry,
`power_pedal_power_switch_control_init` (`a1 = 0x0A` → `0x14415040`), and the same
`a0<<21 | a1<<13 | 0x1040` shape used by the battery OD telemetry above.

| Signal | a1 | CAN ID | Notes |
|---|---|---|---|
| switch_control_init | 0x0A | `0x14415040` | documented (vm registry); polled, 1 byte |
| self / identity | — | `0x14403440` | class-0x3 identity (different low-bits form) |
| (idle telemetry) | 0x03 | `0x14407440` | class-0x3 frame seen on a locked bus, 1 byte `00` |

**Torque / cadence (pedal_rpm): not pinned.** The specific OD signal indices for
torque and cadence are not documented and not present in the captures (all dumps
were taken locked = no pedaling, so power_pedal emits only its heartbeat + the
class-0x3 identity/idle frames above). The sub-ECU firmware keeps its signal name
strings off-image and creates its tasks via `xTaskCreate`; the semantic
name→signal map lives in the i.MX8 `ride` service (`ride/info/pedal_rpm` exists as
an MQTT topic). Pinning them needs the `ride` binary or a capture taken while
pedaling — until then they are injected via the generic index path, not fabricated.

**Bench tooling** (the tool prints `cansend`/`candump`; it does not write the bus):

| Command | Effect |
|---|---|
| `--power-pedal-check` | alive (heartbeat) + publishing-OD verdict over a live attach or piped dump |
| `--fake-pedal-switch N` | `cansend` for the documented switch signal (`a1=0x0A`, byte N) |
| `--fake-pedal-signal A1 --fake-pedal-data HEX` | `cansend` for any power_pedal OD signal index (torque/cadence/other once the index is known) |

## Peripheral OTA over CAN (page-CRC flash)

Battery, charger and CAN motor targets are flashed with a page-CRC-over-CAN
protocol. The image is split into fixed pages; each page is written and verified
with a CRC32 (up to 3 retries per page), then the full image is CRC-checked.

**Object-dictionary flash ops** (lightweight CAN flash, e.g. `motor_control`):
`flash` / `ErasePage` / `write` / `reboot`. The flash FSM walks the states
SIZE_INFO → PAGE_CRC → IMAGE_CRC → UPDATE_REQ → TERMINAL. The size-info reply
byteswaps page size/count (big-endian wire) and bounds-checks the geometry.

**Supplier dispatch** — the battery flashing client is chosen from the firmware
package *file name*, not from a sensed chemistry byte:

| Name contains | Client | Node | Notes |
|---|---|---|---|
| `_panasonic` | Panasonic | battery 0xA4 | version-gated timing |
| `_dynapack` | Dynapack | battery 0xA4 | different protocol/timing |
| `charger` | Liteon | charger 0xA7 | validates "charger type matched with max current" |

A `battery_primary` package with neither supplier suffix is rejected.

**Per-vendor flash timing** (page timeout / step / retry / full-image timeout):

| Client | Page | Step | Retry | Full image |
|---|---|---|---|---|
| Panasonic legacy (≤ v1.3.0.x) | 300 ms | 30 ms | 30 ms | 900 s |
| Panasonic new (> v1.3.0.255) | 100 ms | 10 ms | 10 ms | 360 s |
| Dynapack (mode 0) | 50 ms | 1 ms | — | — |
| Liteon (charger) | 20 ms | 10 ms | 10 ms | — |

The Panasonic version gate is `version > 0x010300FF` (v1.3.0.255); newer BMS
firmware flashes ~3× faster. The lightweight CAN motor-flash client uses
15 s / 2 s / 180 s retry/op/total timing.

This page-CRC-over-CAN flash is the same mechanism behind the BMS AP firmware
update CAN IDs (`0x14901460`…`0x1490D470`) documented above.

**TI motor controller (TMS320F40049C)** uses a separate path: GPIO reset
(gpio12/125/126) → auto-baud → SCI-kernel byte-stream with echo verify → DFU
application image with a running checksum and footer magic `0x1BE4` / `0xE41B`
(status `0x1000`).

---

*OD telemetry, battery/charger commands and the peripheral OTA flash protocol
documented for nodes 0xA4 (battery) and 0xA7 (charger).*
