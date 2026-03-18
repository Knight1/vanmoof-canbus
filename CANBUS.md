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

