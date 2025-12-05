# Particle LED Controller Firmware

WS2812B LED controller firmware for Particle IoT devices. This firmware enables cloud-controlled LED light patterns for decorative lighting applications such as candle simulations, ambient lighting, and visual effects.

## Supported Devices

| Device | Platform | Status |
|--------|----------|--------|
| Argon | Gen 3 (nRF52840) | Supported |
| Boron | Gen 3 (nRF52840) | Supported |
| Photon | Gen 2 (STM32F205) | Supported |
| Photon 2 / P2 | Gen 3 (RTL8721DM) | Supported |
| Electron | Gen 2 (STM32F205) | Supported |

## Hardware Requirements

- **LEDs**: WS2812B (NeoPixel) compatible addressable RGB LEDs
- **Default Configuration**: 8 LEDs on pin D2
- **Power**: 5V power supply (recommended 60mA per LED at full brightness)
- **Data Line**: 3.3V logic level (most WS2812B accept 3.3V; use level shifter if needed)

### Wiring Diagram

```
Particle Device          WS2812B Strip
     D2 -----------------> DIN (Data In)
    GND -----------------> GND
    5V* -----------------> 5V (+VCC)

* For more than 8 LEDs, use external 5V power supply
```

## Configuration

The firmware can be configured at compile time by modifying the following defines in `candle-lights.ino`:

```cpp
#define PIXEL_COUNT 8      // Number of LEDs in the strip
#define PIXEL_PIN D2       // Data pin connected to LEDs
#define PIXEL_TYPE WS2812B // LED type (WS2812B, WS2811, SK6812, etc.)
```

### LED Types Supported

- `WS2812B` - Most common (default)
- `WS2812`
- `WS2811`
- `SK6812`
- `SK6812RGBW` - RGBW strips

## LED Patterns

The firmware supports 6 built-in patterns:

| Pattern | ID | Description |
|---------|-----|-------------|
| Candle | 0 | Realistic flickering candle flame simulation |
| Solid | 1 | Static solid color |
| Pulse | 2 | Smooth breathing/pulsing effect |
| Wave | 3 | Color wave traveling across the strip |
| Rainbow | 4 | Full spectrum rainbow cycling |
| Fire | 5 | Dynamic fire/flame simulation |

## Cloud Functions

The firmware exposes the following Particle Cloud functions:

### `setPattern(command)`
Set the active pattern and animation speed.

**Format**: `"pattern_id:speed_ms"`

**Example**: `particle call my-device setPattern "0:50"` (Candle at 50ms)

### `setColor(command)`
Set the RGB color for patterns that use a base color.

**Format**: `"R,G,B"` (values 0-255)

**Example**: `particle call my-device setColor "255,100,0"` (Orange)

### `setBright(command)`
Set the overall brightness.

**Format**: `"0-255"`

**Example**: `particle call my-device setBright "128"` (50% brightness)

### `saveConfig(command)`
Save current configuration to flash memory (persists across reboots).

**Example**: `particle call my-device saveConfig ""`

## Cloud Variables

| Variable | Type | Description |
|----------|------|-------------|
| `pattern` | int | Current pattern ID (0-5) |
| `brightness` | int | Current brightness (0-255) |

## Flash Storage

Configuration is stored in EEPROM and persists across power cycles:

| Address | Data |
|---------|------|
| 0 | Pattern type |
| 100-102 | RGB color values |
| 200-201 | Brightness and speed |

## Building and Flashing

### Prerequisites

- [Particle CLI](https://docs.particle.io/getting-started/developer-tools/cli/) installed
- Particle account and device claimed

### Using Particle CLI

```bash
# Login to Particle
particle login

# Compile for your device type
particle compile argon candle-lights.ino --saveTo firmware.bin
particle compile photon candle-lights.ino --saveTo firmware.bin
particle compile boron candle-lights.ino --saveTo firmware.bin
particle compile p2 candle-lights.ino --saveTo firmware.bin

# Flash OTA (device must be online)
particle flash <device-name> firmware.bin

# Flash via USB (device in DFU mode)
particle flash --usb firmware.bin
```

### Using Flash Scripts

Bash (Linux/macOS):
```bash
./flash.sh [device-name]
```

Windows (CMD):
```cmd
flash.bat [device-name]
```

Default device name is `garage-door-lights` if not specified.

## Integration with Web Application

This firmware integrates with the garage-door-lights web application which provides:

- Device management and monitoring
- Pattern creation with multi-color support
- Real-time LED simulation preview
- Scheduled pattern changes
- Device grouping and control

### API Integration

The web application sends commands via the Particle Cloud API:

```javascript
// Set pattern via API
Particle.callFunction('setPattern', `${patternId}:${speed}`);
Particle.callFunction('setColor', `${red},${green},${blue}`);
Particle.callFunction('setBright', `${brightness}`);
```

## Troubleshooting

### LEDs not lighting up

1. Check power connections (5V and GND)
2. Verify data pin matches `PIXEL_PIN` define
3. Ensure data line direction (DIN not DOUT)
4. Check LED count matches `PIXEL_COUNT`

### Incorrect colors

1. Verify `PIXEL_TYPE` matches your LED strip
2. Some strips use GRB ordering - check manufacturer specs
3. Try reducing brightness if colors appear washed out

### Cloud functions not responding

1. Ensure device is online: `particle list`
2. Check device is claimed to your account
3. Verify function names are correct (case-sensitive)

### Configuration not persisting

1. Call `saveConfig` after making changes
2. EEPROM has limited write cycles (~100,000)
3. Avoid calling saveConfig in a loop

## Default Settings

On first boot or after EEPROM clear:

- Pattern: Candle (0)
- Color: Orange (255, 100, 0)
- Brightness: 128 (50%)
- Speed: 50ms

## License

MIT License - See project root for details.

## Resources

- [Particle Documentation](https://docs.particle.io/)
- [NeoPixel Library](https://github.com/particle-iot/particle-neopixel)
- [WS2812B Datasheet](https://cdn-shop.adafruit.com/datasheets/WS2812B.pdf)
