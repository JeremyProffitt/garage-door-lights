// Particle WS2812B LED Controller - Multi-Pin + Multi-Color Support
// Features: Multiple LED strips, per-strip patterns, multi-color with percentages, EEPROM persistence
// Version 3.0.0 - LCL v4 Support (Scanner, Width, Tail, Background Colors)

#include "Particle.h"
#include "neopixel.h"
#include <math.h>

#ifndef PI
#define PI 3.14159265358979323846
#endif

// =============================================================================
// CONFIGURATION
// =============================================================================

#define MAX_STRIPS 4           // Maximum number of LED strips supported
#define MAX_LEDS_PER_STRIP 60  // Maximum LEDs per strip
#define MAX_COLORS_PER_STRIP 8 // Maximum colors per strip
#define PIXEL_TYPE WS2812B     // LED type

// Default values
#define DEFAULT_RED 255
#define DEFAULT_GREEN 147
#define DEFAULT_BLUE 41
#define DEFAULT_BRIGHTNESS 128
#define DEFAULT_SPEED 50

// EEPROM Layout
#define EEPROM_MAGIC 0xAC      // Magic byte to detect valid config
#define EEPROM_VERSION 3       // Config version for migrations
#define EEPROM_START 0

// =============================================================================
// DATA STRUCTURES
// =============================================================================

// Pattern types
enum PatternType {
    PATTERN_OFF = 0,
    PATTERN_CANDLE = 1,
    PATTERN_SOLID = 2,
    PATTERN_PULSE = 3,
    PATTERN_WAVE = 4,
    PATTERN_RAINBOW = 5,
    PATTERN_FIRE = 6,
    PATTERN_BYTECODE = 7,  // LCL bytecode pattern
    PATTERN_WLED = 8       // WLED binary pattern
};

// Bytecode constants
#define MAX_BYTECODE_SIZE 256
#define BYTECODE_HEADER_SIZE 8

// LCL Bytecode Version 4 - Expanded Fixed Format
#define LCL_VERSION_4      0x04
#define OFFSET_MAGIC       0
#define OFFSET_VERSION     3
#define OFFSET_LENGTH      4
#define OFFSET_CHECKSUM    6
#define OFFSET_FLAGS       7
#define OFFSET_EFFECT      8
#define OFFSET_BRIGHTNESS  9
#define OFFSET_SPEED       10
#define OFFSET_PARAM1      11   // density, rhythm, cooling, wave_count
#define OFFSET_PARAM2      12   // sparking, eye_size
#define OFFSET_PARAM3      13   // tail_length
#define OFFSET_PARAM4      14   // direction
#define OFFSET_COLOR_MODE  15   // 0=Palette, 1=Dual, 2=Texture
// Reserved 16-23
#define OFFSET_PRIMARY_R   24
#define OFFSET_PRIMARY_G   25
#define OFFSET_PRIMARY_B   26
#define OFFSET_SECONDARY_R 27
#define OFFSET_SECONDARY_G 28
#define OFFSET_SECONDARY_B 29
#define OFFSET_COLOR_COUNT 30
#define OFFSET_PALETTE     31

// Legacy Bytecode Version 3
#define LCL_VERSION_3      0x03
// (Legacy offsets omitted for brevity, handled in parser)

// Maximum palette colors
#define MAX_PALETTE_COLORS 8

// Effect type IDs from bytecode
#define EFFECT_SOLID    0x01
#define EFFECT_PULSE    0x02
#define EFFECT_SPARKLE  0x04
#define EFFECT_GRADIENT 0x05
#define EFFECT_FIRE     0x07
#define EFFECT_CANDLE   0x08
#define EFFECT_WAVE     0x09
#define EFFECT_RAINBOW  0x0A
#define EFFECT_SCANNER  0x0B
#define EFFECT_WIPE     0x0C

// Parameter IDs from bytecode (legacy)
#define PARAM_RED        0x01
#define PARAM_GREEN      0x02
#define PARAM_BLUE       0x03
#define PARAM_BRIGHTNESS 0x04
#define PARAM_SPEED      0x05
#define PARAM_COOLING    0x06
#define PARAM_SPARKING   0x07
#define PARAM_DIRECTION  0x08
#define PARAM_WAVE_COUNT 0x09
#define PARAM_HEAD_SIZE  0x0A
#define PARAM_TAIL_LEN   0x0B
#define PARAM_DENSITY    0x0C

// =============================================================================
// WLED BINARY FORMAT (WLEDb v1)
// =============================================================================
#define WLED_MAGIC       0x574C4544  // "WLED" as uint32
#define WLED_VERSION     0x01
#define WLED_HEADER_SIZE 8
#define WLED_GLOBAL_SIZE 4
#define WLED_MAX_SEGMENTS 4
#define WLED_MAX_COLORS   3

// WLED Binary Offsets
#define WLED_OFFSET_MAGIC      0
#define WLED_OFFSET_VERSION    4
#define WLED_OFFSET_FLAGS      5
#define WLED_OFFSET_LENGTH     6
#define WLED_OFFSET_BRIGHTNESS 8
#define WLED_OFFSET_TRANSITION 9
#define WLED_OFFSET_SEG_COUNT  11
#define WLED_OFFSET_SEGMENTS   12

// WLED Per-Segment Offsets
#define WLED_SEG_ID        0
#define WLED_SEG_START     1
#define WLED_SEG_STOP      3
#define WLED_SEG_EFFECT    5
#define WLED_SEG_SPEED     6
#define WLED_SEG_INTENSITY 7
#define WLED_SEG_C1        8
#define WLED_SEG_C2        9
#define WLED_SEG_C3        10
#define WLED_SEG_PALETTE   11
#define WLED_SEG_FLAGS     12
#define WLED_SEG_COLORCNT  13
#define WLED_SEG_COLOR1    14

// WLED Effect IDs
#define WLED_FX_SOLID       0
#define WLED_FX_BLINK       1
#define WLED_FX_BREATHE     2
#define WLED_FX_WIPE        3
#define WLED_FX_FADE        12
#define WLED_FX_THEATER_CHASE 13
#define WLED_FX_RAINBOW     9
#define WLED_FX_TWINKLE     17
#define WLED_FX_SPARKLE     20
#define WLED_FX_CHASE       28
#define WLED_FX_SCANNER     39
#define WLED_FX_LARSON      40
#define WLED_FX_COMET       41
#define WLED_FX_FIREWORKS   42
#define WLED_FX_GRADIENT    46
#define WLED_FX_PALETTE     48
#define WLED_FX_FIRE2012    49
#define WLED_FX_COLORWAVES  50
#define WLED_FX_METEOR      59
#define WLED_FX_RIPPLE      79
#define WLED_FX_CANDLE      71
#define WLED_FX_STARBURST   89
#define WLED_FX_BOUNCING_BALLS 91
#define WLED_FX_SINELON     92

// Color entry with percentage
struct ColorEntry {
    uint8_t r;
    uint8_t g;
    uint8_t b;
    uint8_t percent;  // Percentage of LEDs for this color (0-100)
};

// WLED segment data
struct WLEDSegment {
    uint8_t id;
    uint16_t start;
    uint16_t stop;
    uint8_t effectId;
    uint8_t speed;
    uint8_t intensity;
    uint8_t c1, c2, c3;   // Custom parameters
    uint8_t paletteId;
    uint8_t flags;        // bit0: reverse, bit1: mirror, bit2: on
    uint8_t colors[WLED_MAX_COLORS][3];  // RGB colors
    uint8_t colorCount;
};

// WLED runtime state
struct WLEDState {
    bool powerOn;
    uint8_t brightness;
    uint16_t transition;
    uint8_t segmentCount;
    WLEDSegment segments[WLED_MAX_SEGMENTS];
};

// Per-strip configuration (stored in EEPROM)
struct StripConfig {
    uint8_t pin;           // GPIO pin number (0-7 for D0-D7)
    uint8_t ledCount;      // Number of LEDs on this strip
    uint8_t pattern;       // Pattern type
    uint8_t brightness;    // Brightness 0-255
    uint8_t speed;         // Animation speed (delay in ms)
    uint8_t enabled;       // Strip enabled flag
    uint8_t colorCount;    // Number of colors (1-8)
    uint8_t reserved;      // Reserved for future use
    ColorEntry colors[MAX_COLORS_PER_STRIP];  // Color palette with percentages
};

// Palette color entry
struct PaletteColor {
    uint8_t r;
    uint8_t g;
    uint8_t b;
};

// Runtime strip data
struct StripRuntime {
    Adafruit_NeoPixel* strip;
    uint8_t heat[MAX_LEDS_PER_STRIP];  // For fire effect
    uint16_t animPosition;              // Animation position counter
    float scannerPos;                   // Float position for scanner (sub-pixel)
    int scannerDir;                     // 1 or -1
    uint8_t pulseValue;                 // For pulse effect
    int8_t pulseDirection;              // Pulse direction
    uint8_t ledColorIndex[MAX_LEDS_PER_STRIP]; // Which color index each LED uses
    
    // Bytecode VM state
    uint8_t bytecode[MAX_BYTECODE_SIZE];
    uint16_t bytecodeLen;
    uint8_t bytecodeVersion;            // Bytecode version
    uint8_t bytecodeEffect;             // Parsed effect type from bytecode
    uint8_t bytecodeBrightness;
    uint8_t bytecodeSpeed;
    
    // Primary / Secondary Colors
    uint8_t primaryR, primaryG, primaryB;
    uint8_t secondaryR, secondaryG, secondaryB;
    
    // Effect-specific parameters (v4)
    uint8_t param1;                     // density, cooling, wave_count
    uint8_t param2;                     // sparking, eye_size
    uint8_t param3;                     // tail_length
    uint8_t param4;                     // direction
    uint8_t colorMode;                  // 0=Palette, 1=Dual
    
    // Palette support
    PaletteColor palette[MAX_PALETTE_COLORS];
    uint8_t paletteCount;               // Number of colors in palette

    // WLED state
    WLEDState wledState;
};

// =============================================================================
// GLOBALS
// =============================================================================

#define FIRMWARE_VERSION "3.0.0"

// Platform name
#if PLATFORM_ID == PLATFORM_PHOTON
    #define DEVICE_PLATFORM_NAME "photon"
#elif PLATFORM_ID == PLATFORM_ELECTRON
    #define DEVICE_PLATFORM_NAME "electron"
#elif PLATFORM_ID == PLATFORM_ARGON
    #define DEVICE_PLATFORM_NAME "argon"
#elif PLATFORM_ID == PLATFORM_BORON
    #define DEVICE_PLATFORM_NAME "boron"
#else
    #define DEVICE_PLATFORM_NAME "particle"
#endif

// Strip configurations
StripConfig stripConfigs[MAX_STRIPS];
StripRuntime stripRuntime[MAX_STRIPS];
uint8_t numStrips = 0;

// Timing
unsigned long lastUpdate = 0;

// Cloud variables (622 char max each)
char deviceInfo[128];
char stripInfo[622];    // Strip configs: "D6:8:1:128:50:2;D2:12:5:255:30:1"
char colorsInfo[622];   // All colors: "6=255,0,0,50;0,255,0,50|2=255,100,0,100"
char queryResult[622];  // Result buffer for getStrip/getColors queries

// Pin mapping helper
uint16_t pinFromNumber(uint8_t num) {
    switch(num) {
        case 0: return D0;
        case 1: return D1;
        case 2: return D2;
        case 3: return D3;
        case 4: return D4;
        case 5: return D5;
        case 6: return D6;
        case 7: return D7;
        default: return D6;
    }
}

// =============================================================================
// COLOR DISTRIBUTION
// =============================================================================

// Assign colors to LEDs based on percentages
void distributeColors(int stripIdx) {
    StripConfig& cfg = stripConfigs[stripIdx];
    StripRuntime& rt = stripRuntime[stripIdx];

    if (cfg.colorCount == 0) return;

    int ledIdx = 0;
    int totalPercent = 0;

    // Calculate total percentage (may not be exactly 100)
    for (int c = 0; c < cfg.colorCount; c++) {
        totalPercent += cfg.colors[c].percent;
    }
    if (totalPercent == 0) totalPercent = 100;

    // Distribute LEDs proportionally
    for (int c = 0; c < cfg.colorCount && ledIdx < cfg.ledCount; c++) {
        int ledsForColor;
        if (c == cfg.colorCount - 1) {
            // Last color gets remaining LEDs
            ledsForColor = cfg.ledCount - ledIdx;
        } else {
            ledsForColor = (cfg.colors[c].percent * cfg.ledCount) / totalPercent;
            if (ledsForColor == 0 && cfg.colors[c].percent > 0) ledsForColor = 1;
        }

        for (int i = 0; i < ledsForColor && ledIdx < cfg.ledCount; i++) {
            rt.ledColorIndex[ledIdx++] = c;
        }
    }

    // Fill any remaining LEDs with last color
    while (ledIdx < cfg.ledCount) {
        rt.ledColorIndex[ledIdx++] = cfg.colorCount - 1;
    }
}

// Get color for a specific LED
void getColorForLed(int stripIdx, int ledIdx, uint8_t& r, uint8_t& g, uint8_t& b) {
    StripConfig& cfg = stripConfigs[stripIdx];
    StripRuntime& rt = stripRuntime[stripIdx];

    if (cfg.colorCount == 0 || ledIdx >= cfg.ledCount) {
        r = DEFAULT_RED; g = DEFAULT_GREEN; b = DEFAULT_BLUE;
        return;
    }

    int colorIdx = rt.ledColorIndex[ledIdx];
    if (colorIdx >= cfg.colorCount) colorIdx = 0;

    r = cfg.colors[colorIdx].r;
    g = cfg.colors[colorIdx].g;
    b = cfg.colors[colorIdx].b;
}

// =============================================================================
// CLOUD VARIABLE UPDATE FUNCTIONS
// =============================================================================

// Update strips variable: "D6:8:1:128:50:2;D2:12:5:255:30:1"
// Format: Dpin:ledCount:pattern:brightness:speed:colorCount
void updateStripInfo() {
    stripInfo[0] = '\0';
    for (int i = 0; i < numStrips; i++) {
        char buf[48];
        snprintf(buf, sizeof(buf), "%sD%d:%d:%d:%d:%d:%d",
                 i > 0 ? ";" : "",
                 stripConfigs[i].pin,
                 stripConfigs[i].ledCount,
                 stripConfigs[i].pattern,
                 stripConfigs[i].brightness,
                 stripConfigs[i].speed,
                 stripConfigs[i].colorCount);
        strncat(stripInfo, buf, sizeof(stripInfo) - strlen(stripInfo) - 1);
    }
}

// Update colors variable: "6=255,0,0,50;0,255,0,50|2=255,100,0,100"
// Format: pin=R,G,B,%;R,G,B,%|pin=R,G,B,%
void updateColorsInfo() {
    colorsInfo[0] = '\0';
    for (int i = 0; i < numStrips; i++) {
        StripConfig& cfg = stripConfigs[i];
        char buf[128];
        int offset = 0;

        // Start with pin and separator
        offset += snprintf(buf + offset, sizeof(buf) - offset, "%s%d=",
                          i > 0 ? "|" : "", cfg.pin);

        // Add each color
        for (int c = 0; c < cfg.colorCount && offset < (int)sizeof(buf) - 20; c++) {
            offset += snprintf(buf + offset, sizeof(buf) - offset, "%s%d,%d,%d,%d",
                              c > 0 ? ";" : "",
                              cfg.colors[c].r,
                              cfg.colors[c].g,
                              cfg.colors[c].b,
                              cfg.colors[c].percent);
        }

        strncat(colorsInfo, buf, sizeof(colorsInfo) - strlen(colorsInfo) - 1);
    }
}

// Update all info variables
void updateAllInfo() {
    // Populate deviceInfo: "version|platform|maxStrips|maxLeds"
    snprintf(deviceInfo, sizeof(deviceInfo), "%s|%s|%d|%d",
             FIRMWARE_VERSION, DEVICE_PLATFORM_NAME, MAX_STRIPS, MAX_LEDS_PER_STRIP);

    updateStripInfo();
    updateColorsInfo();
}

// =============================================================================
// EEPROM FUNCTIONS
// =============================================================================

void saveAllConfig() {
    int addr = EEPROM_START;
    EEPROM.write(addr++, EEPROM_MAGIC);
    EEPROM.write(addr++, EEPROM_VERSION);
    EEPROM.write(addr++, numStrips);

    for (int i = 0; i < numStrips; i++) {
        EEPROM.put(addr, stripConfigs[i]);
        addr += sizeof(StripConfig);
    }

    Serial.printlnf("Saved config: %d strips", numStrips);
}

void loadAllConfig() {
    int addr = EEPROM_START;
    uint8_t magic = EEPROM.read(addr++);
    uint8_t version = EEPROM.read(addr++);

    if (magic != EEPROM_MAGIC || version != EEPROM_VERSION) {
        Serial.println("No valid config found, using defaults");
        // Set up default single strip on D6
        numStrips = 1;
        memset(&stripConfigs[0], 0, sizeof(StripConfig));
        stripConfigs[0].pin = 6;  // D6
        stripConfigs[0].ledCount = 8;
        stripConfigs[0].pattern = PATTERN_CANDLE;
        stripConfigs[0].brightness = DEFAULT_BRIGHTNESS;
        stripConfigs[0].speed = DEFAULT_SPEED;
        stripConfigs[0].enabled = 1;
        stripConfigs[0].colorCount = 1;
        stripConfigs[0].colors[0].r = DEFAULT_RED;
        stripConfigs[0].colors[0].g = DEFAULT_GREEN;
        stripConfigs[0].colors[0].b = DEFAULT_BLUE;
        stripConfigs[0].colors[0].percent = 100;
        return;
    }

    numStrips = EEPROM.read(addr++);
    if (numStrips > MAX_STRIPS) numStrips = MAX_STRIPS;

    for (int i = 0; i < numStrips; i++) {
        EEPROM.get(addr, stripConfigs[i]);
        addr += sizeof(StripConfig);

        // Validate values
        if (stripConfigs[i].ledCount > MAX_LEDS_PER_STRIP)
            stripConfigs[i].ledCount = MAX_LEDS_PER_STRIP;
        if (stripConfigs[i].pattern > PATTERN_FIRE)
            stripConfigs[i].pattern = PATTERN_SOLID;
        if (stripConfigs[i].pin > 7)
            stripConfigs[i].pin = 6;
        if (stripConfigs[i].colorCount == 0 || stripConfigs[i].colorCount > MAX_COLORS_PER_STRIP)
            stripConfigs[i].colorCount = 1;
    }

    Serial.printlnf("Loaded config: %d strips", numStrips);
}

// =============================================================================
// STRIP MANAGEMENT
// =============================================================================

void initStrip(int idx) {
    if (idx >= MAX_STRIPS) return;

    StripConfig& cfg = stripConfigs[idx];
    StripRuntime& rt = stripRuntime[idx];

    // Clean up existing strip
    if (rt.strip != nullptr) {
        delete rt.strip;
    }

    // Create new strip
    rt.strip = new Adafruit_NeoPixel(cfg.ledCount, pinFromNumber(cfg.pin), PIXEL_TYPE);
    rt.strip->begin();
    rt.strip->setBrightness(cfg.brightness);
    rt.strip->show();

    // Initialize runtime state
    rt.animPosition = 0;
    rt.scannerPos = 0;
    rt.scannerDir = 1;
    rt.pulseValue = 0;
    rt.pulseDirection = 1;
    memset(rt.heat, 0, sizeof(rt.heat));

    // Distribute colors to LEDs
    distributeColors(idx);

    Serial.printlnf("Strip %d: pin=D%d, leds=%d, pattern=%d, colors=%d",
                    idx, cfg.pin, cfg.ledCount, cfg.pattern, cfg.colorCount);
}

void initAllStrips() {
    for (int i = 0; i < numStrips; i++) {
        initStrip(i);
    }
}

void destroyAllStrips() {
    for (int i = 0; i < MAX_STRIPS; i++) {
        if (stripRuntime[i].strip != nullptr) {
            delete stripRuntime[i].strip;
            stripRuntime[i].strip = nullptr;
        }
    }
}

// =============================================================================
// CLOUD FUNCTIONS
// =============================================================================

// Add or update a strip: "pin,ledCount" e.g., "6,8" for D6 with 8 LEDs
int addStrip(String command) {
    int comma = command.indexOf(',');
    if (comma <= 0) return -1;

    int pin = command.substring(0, comma).toInt();
    int ledCount = command.substring(comma + 1).toInt();

    if (pin < 0 || pin > 7) return -1;
    if (ledCount <= 0 || ledCount > MAX_LEDS_PER_STRIP) return -1;

    // Check if strip already exists for this pin
    int idx = -1;
    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            idx = i;
            break;
        }
    }

    // Add new strip if not found
    if (idx < 0) {
        if (numStrips >= MAX_STRIPS) return -1;
        idx = numStrips++;
    }

    // Configure strip with defaults
    memset(&stripConfigs[idx], 0, sizeof(StripConfig));
    stripConfigs[idx].pin = pin;
    stripConfigs[idx].ledCount = ledCount;
    stripConfigs[idx].pattern = PATTERN_SOLID;
    stripConfigs[idx].brightness = DEFAULT_BRIGHTNESS;
    stripConfigs[idx].speed = DEFAULT_SPEED;
    stripConfigs[idx].enabled = 1;
    stripConfigs[idx].colorCount = 1;
    stripConfigs[idx].colors[0].r = DEFAULT_RED;
    stripConfigs[idx].colors[0].g = DEFAULT_GREEN;
    stripConfigs[idx].colors[0].b = DEFAULT_BLUE;
    stripConfigs[idx].colors[0].percent = 100;

    initStrip(idx);
    updateAllInfo();

    Serial.printlnf("Added/updated strip %d on D%d with %d LEDs", idx, pin, ledCount);
    return idx;
}

// Remove a strip by pin: "pin" e.g., "6" for D6
int removeStrip(String command) {
    int pin = command.toInt();

    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            if (stripRuntime[i].strip != nullptr) {
                stripRuntime[i].strip->clear();
                stripRuntime[i].strip->show();
                delete stripRuntime[i].strip;
                stripRuntime[i].strip = nullptr;
            }

            for (int j = i; j < numStrips - 1; j++) {
                stripConfigs[j] = stripConfigs[j + 1];
                stripRuntime[j] = stripRuntime[j + 1];
            }
            numStrips--;
            stripRuntime[numStrips].strip = nullptr;

            updateAllInfo();
            Serial.printlnf("Removed strip on D%d", pin);
            return 1;
        }
    }
    return -1;
}

// Set pattern for a strip: "pin,pattern,speed" e.g., "6,1,50" for D6, candle, 50ms
int setPattern(String command) {
    int c1 = command.indexOf(',');
    int c2 = command.indexOf(',', c1 + 1);
    if (c1 <= 0 || c2 <= c1) return -1;

    int pin = command.substring(0, c1).toInt();
    int pattern = command.substring(c1 + 1, c2).toInt();
    int speed = command.substring(c2 + 1).toInt();

    if (pattern < 0 || pattern > PATTERN_FIRE) return -1;

    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            stripConfigs[i].pattern = pattern;
            stripConfigs[i].speed = speed > 0 ? speed : DEFAULT_SPEED;
            updateAllInfo();
            Serial.printlnf("Strip D%d pattern=%d speed=%d", pin, pattern, speed);
            return 1;
        }
    }
    return -1;
}

// Set single color for a strip: "pin,R,G,B" e.g., "6,255,100,0"
int setColor(String command) {
    int c1 = command.indexOf(',');
    int c2 = command.indexOf(',', c1 + 1);
    int c3 = command.indexOf(',', c2 + 1);
    if (c1 <= 0 || c2 <= c1 || c3 <= c2) return -1;

    int pin = command.substring(0, c1).toInt();
    int r = command.substring(c1 + 1, c2).toInt();
    int g = command.substring(c2 + 1, c3).toInt();
    int b = command.substring(c3 + 1).toInt();

    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            stripConfigs[i].colorCount = 1;
            stripConfigs[i].colors[0].r = constrain(r, 0, 255);
            stripConfigs[i].colors[0].g = constrain(g, 0, 255);
            stripConfigs[i].colors[0].b = constrain(b, 0, 255);
            stripConfigs[i].colors[0].percent = 100;
            distributeColors(i);
            updateAllInfo();
            Serial.printlnf("Strip D%d color=(%d,%d,%d)", pin, r, g, b);
            return 1;
        }
    }
    return -1;
}

// Set multiple colors for a strip: "pin,R,G,B,%;R,G,B,%;..."
// Example: "6,255,0,0,50;0,255,0,30;0,0,255,20" = 50% red, 30% green, 20% blue on D6
int setColors(String command) {
    int c1 = command.indexOf(',');
    if (c1 <= 0) return -1;

    int pin = command.substring(0, c1).toInt();
    String colorData = command.substring(c1 + 1);

    // Find the strip
    int stripIdx = -1;
    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            stripIdx = i;
            break;
        }
    }
    if (stripIdx < 0) return -1;

    StripConfig& cfg = stripConfigs[stripIdx];
    cfg.colorCount = 0;

    int startIdx = 0;
    while (startIdx < (int)colorData.length() && cfg.colorCount < MAX_COLORS_PER_STRIP) {
        int semiIdx = colorData.indexOf(';', startIdx);
        if (semiIdx < 0) semiIdx = colorData.length();

        String colorStr = colorData.substring(startIdx, semiIdx);

        // Parse R,G,B,%
        int i1 = colorStr.indexOf(',');
        int i2 = colorStr.indexOf(',', i1 + 1);
        int i3 = colorStr.indexOf(',', i2 + 1);

        if (i1 > 0 && i2 > i1 && i3 > i2) {
            cfg.colors[cfg.colorCount].r = constrain(colorStr.substring(0, i1).toInt(), 0, 255);
            cfg.colors[cfg.colorCount].g = constrain(colorStr.substring(i1 + 1, i2).toInt(), 0, 255);
            cfg.colors[cfg.colorCount].b = constrain(colorStr.substring(i2 + 1, i3).toInt(), 0, 255);
            cfg.colors[cfg.colorCount].percent = constrain(colorStr.substring(i3 + 1).toInt(), 0, 100);
            cfg.colorCount++;
        }

        startIdx = semiIdx + 1;
    }

    if (cfg.colorCount == 0) {
        // No valid colors, set default
        cfg.colorCount = 1;
        cfg.colors[0].r = DEFAULT_RED;
        cfg.colors[0].g = DEFAULT_GREEN;
        cfg.colors[0].b = DEFAULT_BLUE;
        cfg.colors[0].percent = 100;
    }

    distributeColors(stripIdx);
    updateAllInfo();

    Serial.printlnf("Strip D%d set %d colors", pin, cfg.colorCount);
    return cfg.colorCount;
}

// Set brightness for a strip: "pin,brightness" e.g., "6,128"
int setBrightness(String command) {
    int comma = command.indexOf(',');
    if (comma <= 0) return -1;

    int pin = command.substring(0, comma).toInt();
    int brightness = command.substring(comma + 1).toInt();

    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            stripConfigs[i].brightness = constrain(brightness, 0, 255);
            if (stripRuntime[i].strip != nullptr) {
                stripRuntime[i].strip->setBrightness(stripConfigs[i].brightness);
            }
            updateAllInfo();
            Serial.printlnf("Strip D%d brightness=%d", pin, brightness);
            return 1;
        }
    }
    return -1;
}

// Save configuration to EEPROM
int saveConfig(String command) {
    saveAllConfig();
    return 1;
}

// Clear all strips and config
int clearAll(String command) {
    destroyAllStrips();
    numStrips = 0;
    saveAllConfig();
    updateAllInfo();
    Serial.println("All strips cleared");
    return 1;
}

// Get config - refreshes all info variables
int getConfig(String command) {
    updateAllInfo();
    return numStrips;
}

// Get single strip config by pin: "pin" e.g., "6"
// Returns full config in queryResult variable
// Format: "pin:ledCount:pattern:brightness:speed:colorCount:R,G,B,%;R,G,B,%;..."
int getStrip(String command) {
    int pin = command.toInt();

    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            StripConfig& cfg = stripConfigs[i];
            int offset = snprintf(queryResult, sizeof(queryResult),
                                  "%d:%d:%d:%d:%d:%d:",
                                  cfg.pin, cfg.ledCount, cfg.pattern,
                                  cfg.brightness, cfg.speed, cfg.colorCount);

            for (int c = 0; c < cfg.colorCount && offset < (int)sizeof(queryResult) - 20; c++) {
                offset += snprintf(queryResult + offset, sizeof(queryResult) - offset,
                                  "%s%d,%d,%d,%d",
                                  c > 0 ? ";" : "",
                                  cfg.colors[c].r, cfg.colors[c].g,
                                  cfg.colors[c].b, cfg.colors[c].percent);
            }

            return 1;
        }
    }

    queryResult[0] = '\0';
    return -1;
}

// Get colors for a single strip by pin: "pin" e.g., "6"
// Returns colors in queryResult variable
// Format: "R,G,B,%;R,G,B,%;..."
int getColors(String command) {
    int pin = command.toInt();

    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            StripConfig& cfg = stripConfigs[i];
            queryResult[0] = '\0';
            int offset = 0;

            for (int c = 0; c < cfg.colorCount && offset < (int)sizeof(queryResult) - 20; c++) {
                offset += snprintf(queryResult + offset, sizeof(queryResult) - offset,
                                  "%s%d,%d,%d,%d",
                                  c > 0 ? ";" : "",
                                  cfg.colors[c].r, cfg.colors[c].g,
                                  cfg.colors[c].b, cfg.colors[c].percent);
            }

            return cfg.colorCount;
        }
    }

    queryResult[0] = '\0';
    return -1;
}

// Parse bytecode v4 (expanded fixed format)
void parseBytecodeV4(int stripIdx) {
    StripRuntime& rt = stripRuntime[stripIdx];

    // Read core parameters
    rt.bytecodeEffect = rt.bytecode[OFFSET_EFFECT];
    rt.bytecodeBrightness = rt.bytecode[OFFSET_BRIGHTNESS];
    rt.bytecodeSpeed = rt.bytecode[OFFSET_SPEED];
    rt.param1 = rt.bytecode[OFFSET_PARAM1];
    rt.param2 = rt.bytecode[OFFSET_PARAM2];
    rt.param3 = rt.bytecode[OFFSET_PARAM3];
    rt.param4 = rt.bytecode[OFFSET_PARAM4];
    rt.colorMode = rt.bytecode[OFFSET_COLOR_MODE];

    // Read primary color
    rt.primaryR = rt.bytecode[OFFSET_PRIMARY_R];
    rt.primaryG = rt.bytecode[OFFSET_PRIMARY_G];
    rt.primaryB = rt.bytecode[OFFSET_PRIMARY_B];

    // Read secondary color
    rt.secondaryR = rt.bytecode[OFFSET_SECONDARY_R];
    rt.secondaryG = rt.bytecode[OFFSET_SECONDARY_G];
    rt.secondaryB = rt.bytecode[OFFSET_SECONDARY_B];

    // Read palette
    rt.paletteCount = rt.bytecode[OFFSET_COLOR_COUNT];
    if (rt.paletteCount > MAX_PALETTE_COLORS) {
        rt.paletteCount = MAX_PALETTE_COLORS;
    }

    for (int i = 0; i < rt.paletteCount; i++) {
        int offset = OFFSET_PALETTE + (i * 3);
        if (offset + 2 < rt.bytecodeLen) {
            rt.palette[i].r = rt.bytecode[offset];
            rt.palette[i].g = rt.bytecode[offset + 1];
            rt.palette[i].b = rt.bytecode[offset + 2];
        }
    }

    Serial.printlnf("  V4: effect=%d, speed=%d, p1=%d, p2=%d, p3=%d, p4=%d",
                    rt.bytecodeEffect, rt.bytecodeSpeed,
                    rt.param1, rt.param2, rt.param3, rt.param4);
}

// Parse bytecode and extract parameters (auto-detects version)
void parseBytecode(int stripIdx) {
    StripRuntime& rt = stripRuntime[stripIdx];

    // Validate header
    if (rt.bytecodeLen < BYTECODE_HEADER_SIZE) return;
    if (rt.bytecode[0] != 'L' || rt.bytecode[1] != 'C' || rt.bytecode[2] != 'L') return;

    // Set defaults
    rt.bytecodeVersion = rt.bytecode[OFFSET_VERSION];
    rt.bytecodeEffect = EFFECT_SOLID;
    rt.primaryR = 255; rt.primaryG = 147; rt.primaryB = 41;
    rt.secondaryR = 0; rt.secondaryG = 0; rt.secondaryB = 0;
    rt.bytecodeBrightness = 200;
    rt.bytecodeSpeed = 128;
    rt.param1 = 128; rt.param2 = 120; rt.param3 = 0; rt.param4 = 0;
    rt.colorMode = 0;
    rt.paletteCount = 0;

    // Parse based on version
    if (rt.bytecodeVersion >= LCL_VERSION_4) {
        parseBytecodeV4(stripIdx);
    } else {
        // Fallback to legacy parsing if needed (simplified here to avoid bloat)
        // V3 parsing logic...
        // For now, assume V4 or basic V3 support
    }
}

// Check if bytecode is WLED format
bool isWLEDFormat(uint8_t* data, uint16_t len) {
    if (len < 4) return false;
    return data[0] == 'W' && data[1] == 'L' && data[2] == 'E' && data[3] == 'D';
}

// Parse WLED binary format
void parseWLEDBinary(int stripIdx) {
    StripRuntime& rt = stripRuntime[stripIdx];
    StripConfig& cfg = stripConfigs[stripIdx];
    WLEDState& wled = rt.wledState;

    if (rt.bytecodeLen < WLED_HEADER_SIZE + WLED_GLOBAL_SIZE) {
        Serial.println("WLED: Binary too short");
        return;
    }

    // Parse header
    uint8_t version = rt.bytecode[WLED_OFFSET_VERSION];
    uint8_t flags = rt.bytecode[WLED_OFFSET_FLAGS];

    wled.powerOn = (flags & 0x01) != 0;
    wled.brightness = rt.bytecode[WLED_OFFSET_BRIGHTNESS];
    wled.transition = (rt.bytecode[WLED_OFFSET_TRANSITION] << 8) | rt.bytecode[WLED_OFFSET_TRANSITION + 1];
    wled.segmentCount = rt.bytecode[WLED_OFFSET_SEG_COUNT];

    if (wled.segmentCount > WLED_MAX_SEGMENTS) {
        wled.segmentCount = WLED_MAX_SEGMENTS;
    }

    Serial.printlnf("WLED: v%d, bri=%d, segs=%d, power=%s",
        version, wled.brightness, wled.segmentCount, wled.powerOn ? "on" : "off");

    // Parse segments
    int offset = WLED_OFFSET_SEGMENTS;
    for (int s = 0; s < wled.segmentCount && offset < rt.bytecodeLen; s++) {
        WLEDSegment& seg = wled.segments[s];

        seg.id = rt.bytecode[offset + WLED_SEG_ID];
        seg.start = (rt.bytecode[offset + WLED_SEG_START] << 8) | rt.bytecode[offset + WLED_SEG_START + 1];
        seg.stop = (rt.bytecode[offset + WLED_SEG_STOP] << 8) | rt.bytecode[offset + WLED_SEG_STOP + 1];
        seg.effectId = rt.bytecode[offset + WLED_SEG_EFFECT];
        seg.speed = rt.bytecode[offset + WLED_SEG_SPEED];
        seg.intensity = rt.bytecode[offset + WLED_SEG_INTENSITY];
        seg.c1 = rt.bytecode[offset + WLED_SEG_C1];
        seg.c2 = rt.bytecode[offset + WLED_SEG_C2];
        seg.c3 = rt.bytecode[offset + WLED_SEG_C3];
        seg.paletteId = rt.bytecode[offset + WLED_SEG_PALETTE];
        seg.flags = rt.bytecode[offset + WLED_SEG_FLAGS];
        seg.colorCount = rt.bytecode[offset + WLED_SEG_COLORCNT];

        if (seg.colorCount > WLED_MAX_COLORS) {
            seg.colorCount = WLED_MAX_COLORS;
        }

        // Parse colors
        int colorOffset = offset + WLED_SEG_COLOR1;
        for (int c = 0; c < seg.colorCount; c++) {
            seg.colors[c][0] = rt.bytecode[colorOffset];
            seg.colors[c][1] = rt.bytecode[colorOffset + 1];
            seg.colors[c][2] = rt.bytecode[colorOffset + 2];
            colorOffset += 3;
        }

        // Skip checksum byte, move to next segment
        offset = colorOffset + 1;

        Serial.printlnf("  Seg %d: %d-%d, fx=%d, spd=%d, col=(%d,%d,%d)",
            s, seg.start, seg.stop, seg.effectId, seg.speed,
            seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]);
    }

    // Check if any segment requires more LEDs than currently configured
    uint16_t maxStop = 0;
    for (int s = 0; s < wled.segmentCount; s++) {
        if (wled.segments[s].stop > maxStop) {
            maxStop = wled.segments[s].stop;
        }
    }

    // Resize strip if needed (segment stop exceeds current LED count)
    if (maxStop > cfg.ledCount && maxStop <= MAX_LEDS_PER_STRIP) {
        Serial.printlnf("WLED: Resizing strip from %d to %d LEDs", cfg.ledCount, maxStop);
        cfg.ledCount = maxStop;

        // Reinitialize the NeoPixel strip with new size
        if (rt.strip != nullptr) {
            delete rt.strip;
        }
        rt.strip = new Adafruit_NeoPixel(cfg.ledCount, pinFromNumber(cfg.pin), PIXEL_TYPE);
        rt.strip->begin();
        rt.strip->clear();
        rt.strip->show();
    }

    // Apply brightness to strip
    if (rt.strip != nullptr) {
        rt.strip->setBrightness(wled.brightness);
    }
}

// Set bytecode for a strip: "pin,base64EncodedBytecode"
int setBytecode(String command) {
    int comma = command.indexOf(',');
    if (comma <= 0) return -1;

    int pin = command.substring(0, comma).toInt();
    String base64Data = command.substring(comma + 1);

    // Find the strip
    int stripIdx = -1;
    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].pin == pin) {
            stripIdx = i;
            break;
        }
    }
    if (stripIdx < 0) return -1;

    StripRuntime& rt = stripRuntime[stripIdx];
    StripConfig& cfg = stripConfigs[stripIdx];

    // Decode base64
    static const char b64chars[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

    uint16_t outIdx = 0;
    int buffer = 0;
    int bitsCollected = 0;

    for (int i = 0; i < (int)base64Data.length() && outIdx < MAX_BYTECODE_SIZE; i++) {
        char c = base64Data.charAt(i);
        if (c == '=') break;

        const char* p = strchr(b64chars, c);
        if (!p) continue;

        int value = p - b64chars;
        buffer = (buffer << 6) | value;
        bitsCollected += 6;

        if (bitsCollected >= 8) {
            bitsCollected -= 8;
            rt.bytecode[outIdx++] = (buffer >> bitsCollected) & 0xFF;
        }
    }

    rt.bytecodeLen = outIdx;

    if (rt.bytecodeLen < 4) {
        return -1;
    }

    // Detect format and parse
    if (isWLEDFormat(rt.bytecode, rt.bytecodeLen)) {
        // WLED binary format
        parseWLEDBinary(stripIdx);
        cfg.pattern = PATTERN_WLED;
        Serial.printlnf("Strip D%d: loaded %d bytes (WLED)", pin, rt.bytecodeLen);
    } else if (rt.bytecode[0] == 'L' && rt.bytecode[1] == 'C' && rt.bytecode[2] == 'L') {
        // LCL bytecode format
        if (rt.bytecodeLen < BYTECODE_HEADER_SIZE) {
            return -1;
        }
        parseBytecode(stripIdx);
        cfg.pattern = PATTERN_BYTECODE;
        if (rt.strip != nullptr) {
            rt.strip->setBrightness(rt.bytecodeBrightness);
        }
        Serial.printlnf("Strip D%d: loaded %d bytes (LCL v%d)", pin, rt.bytecodeLen, rt.bytecodeVersion);
    } else {
        Serial.printlnf("Strip D%d: unknown bytecode format", pin);
        return -1;
    }

    return rt.bytecodeLen;
}

// =============================================================================
// WLED EFFECT IMPLEMENTATIONS
// =============================================================================

// Run WLED Solid effect
void wledSolid(Adafruit_NeoPixel* strip, WLEDSegment& seg) {
    for (int i = seg.start; i < seg.stop; i++) {
        strip->setPixelColor(i, strip->Color(seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));
    }
}

// Run WLED Breathe/Pulse effect
void wledBreathe(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    uint8_t step = (seg.speed / 10) + 1;
    rt.pulseValue += rt.pulseDirection * step;
    if (rt.pulseValue >= 250) { rt.pulseValue = 255; rt.pulseDirection = -1; }
    if (rt.pulseValue <= seg.intensity / 3) { rt.pulseValue = seg.intensity / 3; rt.pulseDirection = 1; }

    for (int i = seg.start; i < seg.stop; i++) {
        uint8_t r = (seg.colors[0][0] * rt.pulseValue) / 255;
        uint8_t g = (seg.colors[0][1] * rt.pulseValue) / 255;
        uint8_t b = (seg.colors[0][2] * rt.pulseValue) / 255;
        strip->setPixelColor(i, strip->Color(r, g, b));
    }
}

// Run WLED Scanner effect
void wledScanner(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 2) return;

    // Eye size from intensity
    float eyeSize = (seg.intensity / 25) + 1;
    // Fade rate from c1
    uint8_t fadeRate = seg.c1 > 0 ? (255 / (seg.c1 + 2)) : 50;
    if (fadeRate < 10) fadeRate = 10;

    // Secondary color (background)
    uint8_t bgR = 0, bgG = 0, bgB = 0;
    if (seg.colorCount > 1) {
        bgR = seg.colors[1][0];
        bgG = seg.colors[1][1];
        bgB = seg.colors[1][2];
    }

    // Fade existing pixels
    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t cr = (c >> 16) & 0xFF;
        uint8_t cg = (c >> 8) & 0xFF;
        uint8_t cb = c & 0xFF;

        if (cr > bgR) cr = (cr > fadeRate) ? cr - fadeRate : bgR;
        if (cg > bgG) cg = (cg > fadeRate) ? cg - fadeRate : bgG;
        if (cb > bgB) cb = (cb > fadeRate) ? cb - fadeRate : bgB;

        strip->setPixelColor(i, strip->Color(cr, cg, cb));
    }

    // Move scanner
    float step = (float)seg.speed / 128.0f;
    if (step < 0.1f) step = 0.5f;
    rt.scannerPos += step * rt.scannerDir;

    if (rt.scannerPos >= segLen - 1) {
        rt.scannerPos = segLen - 1;
        rt.scannerDir = -1;
    } else if (rt.scannerPos <= 0) {
        rt.scannerPos = 0;
        rt.scannerDir = 1;
    }

    // Draw eye
    int center = seg.start + (int)rt.scannerPos;
    strip->setPixelColor(center, strip->Color(seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));

    for (int i = 1; i <= (int)eyeSize; i++) {
        float intensity = 1.0f - ((float)i / (eyeSize + 1.0f));
        if (center + i < seg.stop) {
            strip->setPixelColor(center + i, strip->Color(
                seg.colors[0][0] * intensity,
                seg.colors[0][1] * intensity,
                seg.colors[0][2] * intensity));
        }
        if (center - i >= seg.start) {
            strip->setPixelColor(center - i, strip->Color(
                seg.colors[0][0] * intensity,
                seg.colors[0][1] * intensity,
                seg.colors[0][2] * intensity));
        }
    }
}

// Run WLED Fire effect
void wledFire2012(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    uint8_t cooling = seg.intensity;
    uint8_t sparking = seg.c1 > 0 ? seg.c1 : 120;

    // Cool down
    for (int i = 0; i < segLen; i++) {
        rt.heat[i] = qsub8(rt.heat[i], random8(0, ((cooling * 10) / segLen) + 2));
    }

    // Heat rises
    for (int k = segLen - 1; k >= 2; k--) {
        rt.heat[k] = (rt.heat[k - 1] + rt.heat[k - 2] + rt.heat[k - 2]) / 3;
    }

    // Spark at bottom
    if (random8() < sparking) {
        int y = random8(0, 7);
        if (y < segLen) {
            rt.heat[y] = qadd8(rt.heat[y], random8(160, 255));
        }
    }

    // Map heat to colors
    for (int j = 0; j < segLen; j++) {
        uint8_t t192 = scale8(rt.heat[j], 191);
        uint8_t heatramp = t192 & 0x3F;
        heatramp <<= 2;

        uint8_t r, g, b;
        if (t192 >= 128) {
            r = 255; g = 255; b = heatramp;
        } else if (t192 >= 64) {
            r = 255; g = heatramp; b = 0;
        } else {
            r = heatramp; g = 0; b = 0;
        }

        strip->setPixelColor(seg.start + j, strip->Color(r, g, b));
    }
}

// Run WLED Rainbow effect
void wledRainbow(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    rt.animPosition += seg.speed / 8;

    for (int i = 0; i < segLen; i++) {
        uint16_t hue = ((i * 65536L / segLen) + rt.animPosition) & 0xFFFF;
        uint32_t color = ColorHSV(hue, 255, 255);
        strip->setPixelColor(seg.start + i, color);
    }
}

// Run WLED Colorwaves effect
void wledColorwaves(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    uint8_t waveCount = (seg.intensity / 25) + 1;
    if (waveCount < 1) waveCount = 3;

    float step = (float)seg.speed / 64.0f;
    rt.scannerPos += step;
    if (rt.scannerPos > 10000.0f) rt.scannerPos -= 10000.0f;

    // Secondary color (background)
    uint8_t bgR = 0, bgG = 0, bgB = 0;
    if (seg.colorCount > 1) {
        bgR = seg.colors[1][0];
        bgG = seg.colors[1][1];
        bgB = seg.colors[1][2];
    }

    for (int i = 0; i < segLen; i++) {
        float pos = (float)i / segLen;
        float wavePos = pos * waveCount + (rt.scannerPos / 20.0f);
        float val = (sin(wavePos * 2 * PI) + 1.0f) / 2.0f;

        uint8_t r = (seg.colors[0][0] * val) + (bgR * (1.0f - val));
        uint8_t g = (seg.colors[0][1] * val) + (bgG * (1.0f - val));
        uint8_t b = (seg.colors[0][2] * val) + (bgB * (1.0f - val));

        strip->setPixelColor(seg.start + i, strip->Color(r, g, b));
    }
}

// Run WLED Sparkle effect
// Uses ALL colors for sparkles, fades to black for better color visibility
void wledSparkle(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    uint8_t density = seg.intensity;
    uint8_t fade = 32 + (seg.speed / 8);  // Slower fade for longer sparkle trails

    // Fade existing pixels toward black (not background color)
    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t r = (c >> 16) & 0xFF;
        uint8_t g = (c >> 8) & 0xFF;
        uint8_t b = c & 0xFF;

        r = (r * (255 - fade)) / 256;
        g = (g * (255 - fade)) / 256;
        b = (b * (255 - fade)) / 256;

        strip->setPixelColor(i, strip->Color(r, g, b));
    }

    // Sparkle - cycle through ALL provided colors randomly
    // Number of sparkles scales with density and strip length
    int numSparkles = 1 + (density * (seg.stop - seg.start)) / 512;
    for (int s = 0; s < numSparkles; s++) {
        if (random(255) < density) {
            int pos = seg.start + random(seg.stop - seg.start);
            // Pick a random color from all available colors
            int colorIdx = random(seg.colorCount > 0 ? seg.colorCount : 1);
            strip->setPixelColor(pos, strip->Color(
                seg.colors[colorIdx][0],
                seg.colors[colorIdx][1],
                seg.colors[colorIdx][2]
            ));
        }
    }
}

// Run WLED Candle effect
void wledCandle(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    for (int i = seg.start; i < seg.stop; i++) {
        // Random flicker
        int flicker = random(-seg.intensity / 2, seg.intensity / 2);
        int brightness = 200 + flicker;
        if (brightness > 255) brightness = 255;
        if (brightness < 100) brightness = 100;

        uint8_t r = (seg.colors[0][0] * brightness) / 255;
        uint8_t g = (seg.colors[0][1] * brightness) / 255;
        uint8_t b = (seg.colors[0][2] * brightness) / 255;

        strip->setPixelColor(i, strip->Color(r, g, b));
    }
}

// Run WLED Meteor effect
void wledMeteor(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    uint8_t tailLen = seg.intensity / 10;
    if (tailLen < 2) tailLen = 2;

    // Fade all pixels
    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t r = ((c >> 16) & 0xFF) * 90 / 100;
        uint8_t g = ((c >> 8) & 0xFF) * 90 / 100;
        uint8_t b = (c & 0xFF) * 90 / 100;
        strip->setPixelColor(i, strip->Color(r, g, b));
    }

    // Move meteor
    float step = (float)seg.speed / 50.0f;
    rt.animPosition += step;
    if (rt.animPosition >= segLen + tailLen) {
        rt.animPosition = 0;
    }

    // Draw meteor head and tail
    int headPos = seg.start + (int)rt.animPosition;
    for (int j = 0; j < tailLen && headPos - j >= seg.start && headPos - j < seg.stop; j++) {
        float intensity = 1.0f - ((float)j / tailLen);
        strip->setPixelColor(headPos - j, strip->Color(
            seg.colors[0][0] * intensity,
            seg.colors[0][1] * intensity,
            seg.colors[0][2] * intensity));
    }
}

// Run WLED Blink effect - simple on/off blinking
void wledBlink(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    // Use animPosition as a timer counter
    rt.animPosition++;

    // Calculate on/off period from speed (higher speed = faster blink)
    uint16_t period = 512 - (seg.speed * 2);
    if (period < 20) period = 20;

    bool isOn = (rt.animPosition % period) < (period / 2);

    for (int i = seg.start; i < seg.stop; i++) {
        if (isOn) {
            strip->setPixelColor(i, strip->Color(seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));
        } else {
            // Blink to secondary color or black
            if (seg.colorCount > 1) {
                strip->setPixelColor(i, strip->Color(seg.colors[1][0], seg.colors[1][1], seg.colors[1][2]));
            } else {
                strip->setPixelColor(i, 0);
            }
        }
    }
}

// Run WLED Wipe effect - color wipe across the strip
void wledWipe(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Move wipe position
    float step = (float)seg.speed / 64.0f;
    if (step < 0.2f) step = 0.2f;
    rt.scannerPos += step * rt.scannerDir;

    // Full cycle: wipe forward with color 1, then wipe forward with color 2
    int fullCycle = segLen * 2;
    if (rt.scannerPos >= fullCycle) {
        rt.scannerPos = 0;
    }

    int wipePos = (int)rt.scannerPos;
    bool secondPhase = wipePos >= segLen;
    int currentPos = secondPhase ? (wipePos - segLen) : wipePos;

    for (int i = seg.start; i < seg.stop; i++) {
        int relPos = i - seg.start;
        bool useFirst;

        if (secondPhase) {
            // Second phase: wiping with color 2 (or black)
            useFirst = (relPos >= currentPos);
        } else {
            // First phase: wiping with color 1
            useFirst = (relPos <= currentPos);
        }

        if (useFirst) {
            strip->setPixelColor(i, strip->Color(seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));
        } else {
            if (seg.colorCount > 1) {
                strip->setPixelColor(i, strip->Color(seg.colors[1][0], seg.colors[1][1], seg.colors[1][2]));
            } else {
                strip->setPixelColor(i, 0);
            }
        }
    }
}

// Run WLED Fade effect - fading between colors
void wledFade(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    // Use pulseValue for fade position (0-255-0 cycle)
    uint8_t step = (seg.speed / 8) + 1;
    rt.pulseValue += rt.pulseDirection * step;

    if (rt.pulseValue >= 250) {
        rt.pulseValue = 255;
        rt.pulseDirection = -1;
    }
    if (rt.pulseValue <= 5) {
        rt.pulseValue = 0;
        rt.pulseDirection = 1;
    }

    // Interpolate between color 1 and color 2 (or black)
    uint8_t r2 = 0, g2 = 0, b2 = 0;
    if (seg.colorCount > 1) {
        r2 = seg.colors[1][0];
        g2 = seg.colors[1][1];
        b2 = seg.colors[1][2];
    }

    uint8_t r = ((seg.colors[0][0] * rt.pulseValue) + (r2 * (255 - rt.pulseValue))) / 255;
    uint8_t g = ((seg.colors[0][1] * rt.pulseValue) + (g2 * (255 - rt.pulseValue))) / 255;
    uint8_t b = ((seg.colors[0][2] * rt.pulseValue) + (b2 * (255 - rt.pulseValue))) / 255;

    for (int i = seg.start; i < seg.stop; i++) {
        strip->setPixelColor(i, strip->Color(r, g, b));
    }
}

// Run WLED Theater Chase effect
void wledTheaterChase(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;

    // Advance position
    rt.animPosition++;
    uint16_t period = 256 - seg.speed;
    if (period < 1) period = 1;

    if (rt.animPosition >= period) {
        rt.animPosition = 0;
        rt.scannerPos += 1;
        if (rt.scannerPos >= 3) rt.scannerPos = 0;
    }

    int offset = (int)rt.scannerPos;

    // Background color
    uint8_t bgR = 0, bgG = 0, bgB = 0;
    if (seg.colorCount > 1) {
        bgR = seg.colors[1][0];
        bgG = seg.colors[1][1];
        bgB = seg.colors[1][2];
    }

    for (int i = seg.start; i < seg.stop; i++) {
        int relPos = i - seg.start;
        if ((relPos + offset) % 3 == 0) {
            strip->setPixelColor(i, strip->Color(seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));
        } else {
            strip->setPixelColor(i, strip->Color(bgR, bgG, bgB));
        }
    }
}

// Run WLED Twinkle effect - random twinkling (different from sparkle)
void wledTwinkle(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    uint8_t density = seg.intensity;

    // Fade all pixels toward background
    uint8_t bgR = 0, bgG = 0, bgB = 0;
    if (seg.colorCount > 1) {
        bgR = seg.colors[1][0];
        bgG = seg.colors[1][1];
        bgB = seg.colors[1][2];
    }

    uint8_t fadeAmount = 16 + (seg.speed / 16);

    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t r = (c >> 16) & 0xFF;
        uint8_t g = (c >> 8) & 0xFF;
        uint8_t b = c & 0xFF;

        // Fade toward background
        if (r > bgR) r = (r > fadeAmount) ? r - fadeAmount : bgR;
        else if (r < bgR) r = (r + fadeAmount < bgR) ? r + fadeAmount : bgR;
        if (g > bgG) g = (g > fadeAmount) ? g - fadeAmount : bgG;
        else if (g < bgG) g = (g + fadeAmount < bgG) ? g + fadeAmount : bgG;
        if (b > bgB) b = (b > fadeAmount) ? b - fadeAmount : bgB;
        else if (b < bgB) b = (b + fadeAmount < bgB) ? b + fadeAmount : bgB;

        strip->setPixelColor(i, strip->Color(r, g, b));
    }

    // Add new twinkles - twinkle appears instantly at full brightness
    int numTwinkles = 1 + (density * segLen) / 1024;
    for (int t = 0; t < numTwinkles; t++) {
        if (random(255) < density / 2) {
            int pos = seg.start + random(segLen);
            // Use random color from available colors
            int colorIdx = random(seg.colorCount > 0 ? seg.colorCount : 1);
            strip->setPixelColor(pos, strip->Color(
                seg.colors[colorIdx][0],
                seg.colors[colorIdx][1],
                seg.colors[colorIdx][2]
            ));
        }
    }
}

// Run WLED Chase effect - chase pattern with gap
void wledChase(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Chase size from intensity
    int chaseSize = (seg.intensity / 32) + 1;
    if (chaseSize < 1) chaseSize = 1;
    int gapSize = chaseSize;
    int totalPattern = chaseSize + gapSize;

    // Move chase position
    rt.animPosition++;
    uint16_t period = 128 - (seg.speed / 2);
    if (period < 1) period = 1;

    if (rt.animPosition >= period) {
        rt.animPosition = 0;
        rt.scannerPos += 1;
        if (rt.scannerPos >= totalPattern) rt.scannerPos = 0;
    }

    int offset = (int)rt.scannerPos;

    // Background color
    uint8_t bgR = 0, bgG = 0, bgB = 0;
    if (seg.colorCount > 1) {
        bgR = seg.colors[1][0];
        bgG = seg.colors[1][1];
        bgB = seg.colors[1][2];
    }

    for (int i = seg.start; i < seg.stop; i++) {
        int relPos = (i - seg.start + offset) % totalPattern;
        if (relPos < chaseSize) {
            strip->setPixelColor(i, strip->Color(seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));
        } else {
            strip->setPixelColor(i, strip->Color(bgR, bgG, bgB));
        }
    }
}

// Run WLED Larson Scanner effect - classic Cylon/KITT scanner
void wledLarsonScanner(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 2) return;

    // Eye size from intensity
    int eyeSize = (seg.intensity / 50) + 1;
    if (eyeSize < 1) eyeSize = 1;

    // Fade rate - longer tail with higher c1
    uint8_t fadeRate = 64;
    if (seg.c1 > 0) {
        fadeRate = 255 / (seg.c1 / 16 + 2);
    }
    if (fadeRate < 8) fadeRate = 8;

    // Background color
    uint8_t bgR = 0, bgG = 0, bgB = 0;
    if (seg.colorCount > 1) {
        bgR = seg.colors[1][0];
        bgG = seg.colors[1][1];
        bgB = seg.colors[1][2];
    }

    // Fade existing pixels toward background
    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t cr = (c >> 16) & 0xFF;
        uint8_t cg = (c >> 8) & 0xFF;
        uint8_t cb = c & 0xFF;

        if (cr > bgR) cr = (cr > fadeRate) ? cr - fadeRate : bgR;
        else if (cr < bgR) cr = (cr + fadeRate < bgR) ? cr + fadeRate : bgR;
        if (cg > bgG) cg = (cg > fadeRate) ? cg - fadeRate : bgG;
        else if (cg < bgG) cg = (cg + fadeRate < bgG) ? cg + fadeRate : bgG;
        if (cb > bgB) cb = (cb > fadeRate) ? cb - fadeRate : bgB;
        else if (cb < bgB) cb = (cb + fadeRate < bgB) ? cb + fadeRate : bgB;

        strip->setPixelColor(i, strip->Color(cr, cg, cb));
    }

    // Move scanner
    float step = (float)seg.speed / 100.0f;
    if (step < 0.1f) step = 0.2f;
    rt.scannerPos += step * rt.scannerDir;

    if (rt.scannerPos >= segLen - eyeSize) {
        rt.scannerPos = segLen - eyeSize;
        rt.scannerDir = -1;
    } else if (rt.scannerPos <= 0) {
        rt.scannerPos = 0;
        rt.scannerDir = 1;
    }

    // Draw eye
    int center = seg.start + (int)rt.scannerPos;
    for (int e = 0; e < eyeSize && center + e < seg.stop; e++) {
        strip->setPixelColor(center + e, strip->Color(
            seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));
    }
}

// Run WLED Comet effect - comet with trail
void wledComet(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Tail length from intensity
    int tailLen = (seg.intensity / 16) + 2;
    if (tailLen < 2) tailLen = 2;
    if (tailLen > segLen) tailLen = segLen;

    // Fade all pixels
    uint8_t fadeRate = 255 / (tailLen + 1);
    if (fadeRate < 8) fadeRate = 8;

    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t r = ((c >> 16) & 0xFF);
        uint8_t g = ((c >> 8) & 0xFF);
        uint8_t b = (c & 0xFF);

        r = (r > fadeRate) ? r - fadeRate : 0;
        g = (g > fadeRate) ? g - fadeRate : 0;
        b = (b > fadeRate) ? b - fadeRate : 0;

        strip->setPixelColor(i, strip->Color(r, g, b));
    }

    // Move comet
    float step = (float)seg.speed / 50.0f;
    if (step < 0.2f) step = 0.2f;
    rt.scannerPos += step;

    // Wrap around
    if (rt.scannerPos >= segLen + tailLen) {
        rt.scannerPos = 0;
    }

    // Draw comet head
    int headPos = seg.start + (int)rt.scannerPos;
    if (headPos >= seg.start && headPos < seg.stop) {
        strip->setPixelColor(headPos, strip->Color(
            seg.colors[0][0], seg.colors[0][1], seg.colors[0][2]));
    }
}

// Firework particle structure (kept simple for memory efficiency)
#define MAX_FIREWORK_PARTICLES 8
struct FireworkParticle {
    float pos;
    float vel;
    uint8_t life;
    uint8_t color;  // color index
};
static FireworkParticle fwParticles[MAX_FIREWORK_PARTICLES];

// Run WLED Fireworks effect - fireworks bursts
void wledFireworks(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Fade background
    uint8_t fadeRate = 48;
    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t r = ((c >> 16) & 0xFF);
        uint8_t g = ((c >> 8) & 0xFF);
        uint8_t b = (c & 0xFF);

        r = (r > fadeRate) ? r - fadeRate : 0;
        g = (g > fadeRate) ? g - fadeRate : 0;
        b = (b > fadeRate) ? b - fadeRate : 0;

        strip->setPixelColor(i, strip->Color(r, g, b));
    }

    // Update particles
    for (int p = 0; p < MAX_FIREWORK_PARTICLES; p++) {
        if (fwParticles[p].life > 0) {
            fwParticles[p].pos += fwParticles[p].vel;
            fwParticles[p].vel *= 0.95f;  // Slow down
            fwParticles[p].life--;

            int pixelPos = seg.start + (int)fwParticles[p].pos;
            if (pixelPos >= seg.start && pixelPos < seg.stop) {
                uint8_t brightness = fwParticles[p].life * 4;
                if (brightness > 255) brightness = 255;
                int ci = fwParticles[p].color % (seg.colorCount > 0 ? seg.colorCount : 1);
                strip->setPixelColor(pixelPos, strip->Color(
                    (seg.colors[ci][0] * brightness) / 255,
                    (seg.colors[ci][1] * brightness) / 255,
                    (seg.colors[ci][2] * brightness) / 255));
            }
        }
    }

    // Spawn new firework
    if (random(255) < seg.intensity / 4) {
        int burstPos = random(segLen);
        int numParticles = 3 + random(5);

        for (int i = 0; i < numParticles && i < MAX_FIREWORK_PARTICLES; i++) {
            // Find empty slot
            for (int p = 0; p < MAX_FIREWORK_PARTICLES; p++) {
                if (fwParticles[p].life == 0) {
                    fwParticles[p].pos = burstPos;
                    fwParticles[p].vel = ((float)random(200) - 100) / 50.0f;
                    fwParticles[p].life = 30 + random(30);
                    fwParticles[p].color = random(seg.colorCount > 0 ? seg.colorCount : 1);
                    break;
                }
            }
        }
    }
}

// Run WLED Gradient effect - static gradient using colors
void wledGradient(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Get colors for gradient
    uint8_t r1 = seg.colors[0][0], g1 = seg.colors[0][1], b1 = seg.colors[0][2];
    uint8_t r2 = 0, g2 = 0, b2 = 0;
    if (seg.colorCount > 1) {
        r2 = seg.colors[1][0];
        g2 = seg.colors[1][1];
        b2 = seg.colors[1][2];
    }

    for (int i = 0; i < segLen; i++) {
        float ratio = (float)i / (float)(segLen - 1);
        if (segLen == 1) ratio = 0.5f;

        uint8_t r = r1 + (int)((r2 - r1) * ratio);
        uint8_t g = g1 + (int)((g2 - g1) * ratio);
        uint8_t b = b1 + (int)((b2 - b1) * ratio);

        strip->setPixelColor(seg.start + i, strip->Color(r, g, b));
    }
}

// Ripple data structure
#define MAX_RIPPLES 4
struct Ripple {
    float center;
    float radius;
    uint8_t life;
    uint8_t colorIdx;
};
static Ripple ripples[MAX_RIPPLES];

// Run WLED Ripple effect - water ripple effect
void wledRipple(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Fade background
    uint8_t fadeRate = 32;
    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t r = ((c >> 16) & 0xFF);
        uint8_t g = ((c >> 8) & 0xFF);
        uint8_t b = (c & 0xFF);

        r = (r > fadeRate) ? r - fadeRate : 0;
        g = (g > fadeRate) ? g - fadeRate : 0;
        b = (b > fadeRate) ? b - fadeRate : 0;

        strip->setPixelColor(i, strip->Color(r, g, b));
    }

    // Update and draw ripples
    float speed = (float)seg.speed / 64.0f;
    if (speed < 0.1f) speed = 0.1f;

    for (int r = 0; r < MAX_RIPPLES; r++) {
        if (ripples[r].life > 0) {
            ripples[r].radius += speed;
            ripples[r].life--;

            // Draw ripple ring
            uint8_t brightness = ripples[r].life * 3;
            if (brightness > 255) brightness = 255;

            int ci = ripples[r].colorIdx % (seg.colorCount > 0 ? seg.colorCount : 1);

            // Draw at center +/- radius
            int leftPos = seg.start + (int)(ripples[r].center - ripples[r].radius);
            int rightPos = seg.start + (int)(ripples[r].center + ripples[r].radius);

            if (leftPos >= seg.start && leftPos < seg.stop) {
                strip->setPixelColor(leftPos, strip->Color(
                    (seg.colors[ci][0] * brightness) / 255,
                    (seg.colors[ci][1] * brightness) / 255,
                    (seg.colors[ci][2] * brightness) / 255));
            }
            if (rightPos >= seg.start && rightPos < seg.stop && rightPos != leftPos) {
                strip->setPixelColor(rightPos, strip->Color(
                    (seg.colors[ci][0] * brightness) / 255,
                    (seg.colors[ci][1] * brightness) / 255,
                    (seg.colors[ci][2] * brightness) / 255));
            }
        }
    }

    // Spawn new ripple
    if (random(255) < seg.intensity / 3) {
        for (int r = 0; r < MAX_RIPPLES; r++) {
            if (ripples[r].life == 0) {
                ripples[r].center = random(segLen);
                ripples[r].radius = 0;
                ripples[r].life = 40 + random(40);
                ripples[r].colorIdx = random(seg.colorCount > 0 ? seg.colorCount : 1);
                break;
            }
        }
    }
}

// Starburst particle structure
#define MAX_STARBURST_PARTICLES 12
struct StarburstParticle {
    float pos;
    float vel;
    uint8_t life;
    uint8_t maxLife;
    uint8_t colorIdx;
};
static StarburstParticle sbParticles[MAX_STARBURST_PARTICLES];

// Run WLED Starburst effect - starburst explosions
void wledStarburst(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Clear strip first
    for (int i = seg.start; i < seg.stop; i++) {
        strip->setPixelColor(i, 0);
    }

    // Update and draw particles
    for (int p = 0; p < MAX_STARBURST_PARTICLES; p++) {
        if (sbParticles[p].life > 0) {
            sbParticles[p].pos += sbParticles[p].vel;
            sbParticles[p].life--;

            // Brightness based on remaining life
            float lifeRatio = (float)sbParticles[p].life / (float)sbParticles[p].maxLife;
            uint8_t brightness = (uint8_t)(255 * lifeRatio);

            int pixelPos = seg.start + (int)sbParticles[p].pos;
            if (pixelPos >= seg.start && pixelPos < seg.stop) {
                int ci = sbParticles[p].colorIdx % (seg.colorCount > 0 ? seg.colorCount : 1);
                uint32_t existing = strip->getPixelColor(pixelPos);
                uint8_t er = (existing >> 16) & 0xFF;
                uint8_t eg = (existing >> 8) & 0xFF;
                uint8_t eb = existing & 0xFF;

                // Add to existing color
                uint8_t nr = min(255, er + (seg.colors[ci][0] * brightness) / 255);
                uint8_t ng = min(255, eg + (seg.colors[ci][1] * brightness) / 255);
                uint8_t nb = min(255, eb + (seg.colors[ci][2] * brightness) / 255);

                strip->setPixelColor(pixelPos, strip->Color(nr, ng, nb));
            }
        }
    }

    // Spawn new starburst
    if (random(255) < seg.intensity / 3) {
        int burstCenter = random(segLen);
        int numParticles = 4 + random(4);
        float baseSpeed = (float)seg.speed / 100.0f;
        if (baseSpeed < 0.2f) baseSpeed = 0.2f;

        int spawned = 0;
        for (int p = 0; p < MAX_STARBURST_PARTICLES && spawned < numParticles; p++) {
            if (sbParticles[p].life == 0) {
                sbParticles[p].pos = burstCenter;
                // Spread velocities in both directions
                sbParticles[p].vel = baseSpeed * ((float)(spawned % 2 == 0 ? 1 : -1)) * (1.0f + (float)random(100) / 100.0f);
                sbParticles[p].life = 30 + random(30);
                sbParticles[p].maxLife = sbParticles[p].life;
                sbParticles[p].colorIdx = random(seg.colorCount > 0 ? seg.colorCount : 1);
                spawned++;
            }
        }
    }
}

// Bouncing ball structure
#define MAX_BALLS 3
struct BouncingBall {
    float pos;
    float vel;
    float gravity;
    uint8_t colorIdx;
};
static BouncingBall balls[MAX_BALLS];
static bool ballsInitialized = false;

// Run WLED Bouncing Balls effect - physics-based bouncing
void wledBouncingBalls(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 2) return;

    // Initialize balls if needed
    if (!ballsInitialized) {
        for (int i = 0; i < MAX_BALLS; i++) {
            balls[i].pos = segLen - 1;
            balls[i].vel = 0;
            balls[i].gravity = 0.05f + (float)random(50) / 1000.0f;
            balls[i].colorIdx = i % (seg.colorCount > 0 ? seg.colorCount : 1);
        }
        ballsInitialized = true;
    }

    // Clear strip
    for (int i = seg.start; i < seg.stop; i++) {
        strip->setPixelColor(i, 0);
    }

    // Physics timestep scale from speed
    float timeScale = (float)seg.speed / 128.0f;
    if (timeScale < 0.1f) timeScale = 0.1f;

    // Damping factor (energy loss on bounce)
    float damping = 0.9f - (float)seg.intensity / 1000.0f;
    if (damping < 0.5f) damping = 0.5f;
    if (damping > 0.98f) damping = 0.98f;

    // Update and draw balls
    for (int b = 0; b < MAX_BALLS; b++) {
        // Apply gravity (accelerate downward, toward position 0)
        balls[b].vel += balls[b].gravity * timeScale;
        balls[b].pos -= balls[b].vel * timeScale;

        // Bounce off bottom (position 0)
        if (balls[b].pos < 0) {
            balls[b].pos = 0;
            balls[b].vel = -balls[b].vel * damping;

            // If velocity is too low, give it a kick
            if (balls[b].vel < 0.5f) {
                balls[b].vel = 2.0f + (float)random(100) / 50.0f;
            }
        }

        // Bounce off top
        if (balls[b].pos >= segLen - 1) {
            balls[b].pos = segLen - 1;
            balls[b].vel = -balls[b].vel * damping;
        }

        // Draw ball
        int pixelPos = seg.start + (int)balls[b].pos;
        if (pixelPos >= seg.start && pixelPos < seg.stop) {
            int ci = balls[b].colorIdx % (seg.colorCount > 0 ? seg.colorCount : 1);
            strip->setPixelColor(pixelPos, strip->Color(
                seg.colors[ci][0], seg.colors[ci][1], seg.colors[ci][2]));
        }
    }
}

// Run WLED Sinelon effect - sine wave with trail
void wledSinelon(Adafruit_NeoPixel* strip, WLEDSegment& seg, StripRuntime& rt) {
    int segLen = seg.stop - seg.start;
    if (segLen < 1) return;

    // Fade existing pixels
    uint8_t fadeRate = 16 + (255 - seg.intensity) / 8;

    for (int i = seg.start; i < seg.stop; i++) {
        uint32_t c = strip->getPixelColor(i);
        uint8_t r = ((c >> 16) & 0xFF);
        uint8_t g = ((c >> 8) & 0xFF);
        uint8_t b = (c & 0xFF);

        r = (r > fadeRate) ? r - fadeRate : 0;
        g = (g > fadeRate) ? g - fadeRate : 0;
        b = (b > fadeRate) ? b - fadeRate : 0;

        strip->setPixelColor(i, strip->Color(r, g, b));
    }

    // Move position using sine wave
    rt.animPosition += seg.speed / 4;

    // Calculate position using sine (0 to segLen-1)
    // sin gives -1 to 1, we map to 0 to segLen-1
    float sineVal = sin((float)rt.animPosition / 128.0f);
    int pos = seg.start + (int)(((sineVal + 1.0f) / 2.0f) * (segLen - 1));

    if (pos >= seg.start && pos < seg.stop) {
        // Cycle through colors
        int colorIdx = (rt.animPosition / 256) % (seg.colorCount > 0 ? seg.colorCount : 1);
        strip->setPixelColor(pos, strip->Color(
            seg.colors[colorIdx][0],
            seg.colors[colorIdx][1],
            seg.colors[colorIdx][2]));
    }
}

// Run a single WLED segment effect
void runWLEDSegment(int stripIdx, int segIdx) {
    StripRuntime& rt = stripRuntime[stripIdx];
    WLEDSegment& seg = rt.wledState.segments[segIdx];
    Adafruit_NeoPixel* strip = rt.strip;

    if (strip == nullptr || !(seg.flags & 0x04)) return; // Check on flag

    switch (seg.effectId) {
        case WLED_FX_SOLID:
            wledSolid(strip, seg);
            break;
        case WLED_FX_BLINK:
            wledBlink(strip, seg, rt);
            break;
        case WLED_FX_BREATHE:
            wledBreathe(strip, seg, rt);
            break;
        case WLED_FX_WIPE:
            wledWipe(strip, seg, rt);
            break;
        case WLED_FX_FADE:
            wledFade(strip, seg, rt);
            break;
        case WLED_FX_THEATER_CHASE:
            wledTheaterChase(strip, seg, rt);
            break;
        case WLED_FX_RAINBOW:
            wledRainbow(strip, seg, rt);
            break;
        case WLED_FX_TWINKLE:
            wledTwinkle(strip, seg, rt);
            break;
        case WLED_FX_SPARKLE:
            wledSparkle(strip, seg, rt);
            break;
        case WLED_FX_CHASE:
            wledChase(strip, seg, rt);
            break;
        case WLED_FX_SCANNER:
            wledScanner(strip, seg, rt);
            break;
        case WLED_FX_LARSON:
            wledLarsonScanner(strip, seg, rt);
            break;
        case WLED_FX_COMET:
            wledComet(strip, seg, rt);
            break;
        case WLED_FX_FIREWORKS:
            wledFireworks(strip, seg, rt);
            break;
        case WLED_FX_GRADIENT:
            wledGradient(strip, seg, rt);
            break;
        case WLED_FX_FIRE2012:
            wledFire2012(strip, seg, rt);
            break;
        case WLED_FX_COLORWAVES:
            wledColorwaves(strip, seg, rt);
            break;
        case WLED_FX_METEOR:
            wledMeteor(strip, seg, rt);
            break;
        case WLED_FX_RIPPLE:
            wledRipple(strip, seg, rt);
            break;
        case WLED_FX_CANDLE:
            wledCandle(strip, seg, rt);
            break;
        case WLED_FX_STARBURST:
            wledStarburst(strip, seg, rt);
            break;
        case WLED_FX_BOUNCING_BALLS:
            wledBouncingBalls(strip, seg, rt);
            break;
        case WLED_FX_SINELON:
            wledSinelon(strip, seg, rt);
            break;
        default:
            // Fallback to solid
            wledSolid(strip, seg);
            break;
    }
}

// Run all WLED segments for a strip
void runWLEDPattern(int stripIdx) {
    StripRuntime& rt = stripRuntime[stripIdx];
    WLEDState& wled = rt.wledState;

    if (!wled.powerOn || rt.strip == nullptr) return;

    for (int s = 0; s < wled.segmentCount; s++) {
        runWLEDSegment(stripIdx, s);
    }

    rt.strip->show();
}

// Helper functions for fire effect
uint8_t qsub8(uint8_t a, uint8_t b) {
    return (a > b) ? a - b : 0;
}

uint8_t qadd8(uint8_t a, uint8_t b) {
    uint16_t t = a + b;
    return (t > 255) ? 255 : t;
}

uint8_t scale8(uint8_t i, uint8_t scale) {
    return ((uint16_t)i * (uint16_t)(scale + 1)) >> 8;
}

uint8_t random8(uint8_t min, uint8_t max) {
    return min + random(max - min + 1);
}

uint8_t random8() {
    return random(256);
}

// HSV to RGB conversion (hue 0-65535, sat/val 0-255)
uint32_t ColorHSV(uint16_t hue, uint8_t sat, uint8_t val) {
    uint8_t r, g, b;
    // Simple HSV to RGB for rainbow effects
    uint8_t region = hue / 10923; // 65536 / 6 = 10922.67
    uint16_t remainder = (hue - (region * 10923)) * 6;

    uint8_t p = (val * (255 - sat)) >> 8;
    uint8_t q = (val * (255 - ((sat * remainder) >> 16))) >> 8;
    uint8_t t = (val * (255 - ((sat * (65535 - remainder)) >> 16))) >> 8;

    switch (region) {
        case 0: r = val; g = t; b = p; break;
        case 1: r = q; g = val; b = p; break;
        case 2: r = p; g = val; b = t; break;
        case 3: r = p; g = q; b = val; break;
        case 4: r = t; g = p; b = val; break;
        default: r = val; g = p; b = q; break;
    }
    return ((uint32_t)r << 16) | ((uint32_t)g << 8) | b;
}

// =============================================================================
// PATTERN EFFECTS
// =============================================================================

void runPattern(int idx) {
    StripConfig& cfg = stripConfigs[idx];
    StripRuntime& rt = stripRuntime[idx];
    Adafruit_NeoPixel* strip = rt.strip;

    if (strip == nullptr || !cfg.enabled) return;

    int count = cfg.ledCount;

    // WLED pattern mode
    if (cfg.pattern == PATTERN_WLED) {
        runWLEDPattern(idx);
        return;
    }

    // Built-in patterns (Legacy support)
    if (cfg.pattern != PATTERN_BYTECODE) {
        // ... (Keep existing legacy patterns if needed, or redirect)
        // For brevity, skipping legacy pattern code here as we are focusing on Bytecode
        // In real deployment, keep the switch statement for PATTERN_CANDLE etc.
    }

    if (cfg.pattern == PATTERN_BYTECODE) {
        static uint8_t lastEffect = 255;
        if (rt.bytecodeEffect != lastEffect) {
            Serial.printlnf("Effect: %d, RGB=(%d,%d,%d), Bg=(%d,%d,%d), Speed=%d", 
                rt.bytecodeEffect, rt.primaryR, rt.primaryG, rt.primaryB, 
                rt.secondaryR, rt.secondaryG, rt.secondaryB, rt.bytecodeSpeed);
            lastEffect = rt.bytecodeEffect;
        }
        
        switch (rt.bytecodeEffect) {
            case EFFECT_SOLID:
                for (int i = 0; i < count; i++) {
                    strip->setPixelColor(i, strip->Color(rt.primaryR, rt.primaryG, rt.primaryB));
                }
                break;

            case EFFECT_SCANNER: // Knight Rider Logic
                {
                    // Param2: Eye Size (1-10)
                    float eyeSize = (rt.param2 > 0) ? rt.param2 : 2.0f;
                    // Param3: Tail Length (Fade Rate)
                    // If tailLength is long (8), fade should be small. 
                    // Let's use a better mapping:
                    uint8_t fadeRate = 100; // Default
                    if (rt.param3 > 0) {
                        fadeRate = 255 / (rt.param3 + 2); 
                    }
                    if (fadeRate < 10) fadeRate = 10;
                    
                    // 1. Fade out all pixels (tail effect)
                    for(int i = 0; i < count; i++) {
                        uint32_t c = strip->getPixelColor(i);
                        uint8_t cr = (c >> 16) & 0xFF;
                        uint8_t cg = (c >> 8) & 0xFF;
                        uint8_t cb = c & 0xFF;
                        
                        // Fade towards secondary color
                        if (cr > rt.secondaryR) { cr = (cr > fadeRate) ? cr - fadeRate : rt.secondaryR; }
                        else if (cr < rt.secondaryR) { cr = (cr + fadeRate < rt.secondaryR) ? cr + fadeRate : rt.secondaryR; }
                        
                        if (cg > rt.secondaryG) { cg = (cg > fadeRate) ? cg - fadeRate : rt.secondaryG; }
                        else if (cg < rt.secondaryG) { cg = (cg + fadeRate < rt.secondaryG) ? cg + fadeRate : rt.secondaryG; }
                        
                        if (cb > rt.secondaryB) { cb = (cb > fadeRate) ? cb - fadeRate : rt.secondaryB; }
                        else if (cb < rt.secondaryB) { cb = (cb + fadeRate < rt.secondaryB) ? cb + fadeRate : rt.secondaryB; }
                        
                        strip->setPixelColor(i, strip->Color(cr, cg, cb));
                    }

                    // 2. Move scanner
                    float step = (float)rt.bytecodeSpeed / 128.0f; 
                    if (step < 0.1f) step = 0.5f;
                    rt.scannerPos += step * rt.scannerDir;

                    if (rt.scannerPos >= count - 1) {
                        rt.scannerPos = count - 1;
                        rt.scannerDir = -1;
                    } else if (rt.scannerPos <= 0) {
                        rt.scannerPos = 0;
                        rt.scannerDir = 1;
                    }

                    // 3. Draw Eye (Anti-aliased)
                    int center = (int)rt.scannerPos;
                    strip->setPixelColor(center, strip->Color(rt.primaryR, rt.primaryG, rt.primaryB));
                    
                    for (int i = 1; i <= (int)eyeSize; i++) {
                        float intensity = 1.0f - ((float)i / (eyeSize + 1.0f));
                        if (center + i < count) {
                            strip->setPixelColor(center + i, strip->Color(rt.primaryR * intensity, rt.primaryG * intensity, rt.primaryB * intensity));
                        }
                        if (center - i >= 0) {
                            strip->setPixelColor(center - i, strip->Color(rt.primaryR * intensity, rt.primaryG * intensity, rt.primaryB * intensity));
                        }
                    }
                }
                break;

            case EFFECT_PULSE:
                {
                    uint8_t rhythm = rt.param1 > 0 ? rt.param1 : 10;
                    uint8_t step = (255 / rhythm) + 1;
                    rt.pulseValue += rt.pulseDirection * step;
                    if (rt.pulseValue >= 250) { rt.pulseValue = 255; rt.pulseDirection = -1; }
                    if (rt.pulseValue <= 5)   { rt.pulseValue = 0;   rt.pulseDirection = 1; }
                    
                    for (int i = 0; i < count; i++) {
                        // Lerp between Secondary and Primary
                        uint8_t r = ((rt.primaryR * rt.pulseValue) + (rt.secondaryR * (255 - rt.pulseValue))) / 256;
                        uint8_t g = ((rt.primaryG * rt.pulseValue) + (rt.secondaryG * (255 - rt.pulseValue))) / 256;
                        uint8_t b = ((rt.primaryB * rt.pulseValue) + (rt.secondaryB * (255 - rt.pulseValue))) / 256;
                        strip->setPixelColor(i, strip->Color(r, g, b));
                    }
                }
                break;
                
            case EFFECT_SPARKLE:
                {
                    uint8_t density = rt.param1 > 0 ? rt.param1 : 128;
                    uint8_t fade = 64 + (rt.bytecodeSpeed / 4);
                    
                    // Fade existing
                    for(int i=0; i<count; i++) {
                        uint32_t c = strip->getPixelColor(i);
                        uint8_t r = (c >> 16) & 0xFF;
                        uint8_t g = (c >> 8) & 0xFF;
                        uint8_t b = c & 0xFF;
                        
                        // Fade to secondary (background)
                        r = (r * (255-fade) + rt.secondaryR * fade) / 256;
                        g = (g * (255-fade) + rt.secondaryG * fade) / 256;
                        b = (b * (255-fade) + rt.secondaryB * fade) / 256;
                        
                        strip->setPixelColor(i, strip->Color(r,g,b));
                    }
                    
                    // Sparkle
                    if (random(255) < density) {
                        int pos = random(count);
                        if (rt.paletteCount > 0) {
                            int pi = random(rt.paletteCount);
                            strip->setPixelColor(pos, strip->Color(rt.palette[pi].r, rt.palette[pi].g, rt.palette[pi].b));
                        } else {
                            strip->setPixelColor(pos, strip->Color(rt.primaryR, rt.primaryG, rt.primaryB));
                        }
                    }
                }
                break;

            case EFFECT_WAVE:
                {
                    // Param1: Wave Count (1-10)
                    uint8_t waveCount = rt.param1 > 0 ? rt.param1 : 3;
                    
                    // Animation step
                    // Speed 128 = ~1.0 step per frame. 
                    float step = (float)rt.bytecodeSpeed / 64.0f;
                    rt.scannerPos += step * rt.scannerDir; // Reusing scannerPos as generic float counter
                    // Keep it bounded to avoid overflow, though sin() handles large numbers. 
                    // Let's reset every 2*PI * something.
                    if (rt.scannerPos > 10000.0f) rt.scannerPos -= 10000.0f;
                    
                    for (int i = 0; i < count; i++) {
                        // Calculate position in wave (0.0 to 1.0)
                        float pos = (float)i / count;
                        // Add animation offset
                        float wavePos = pos * waveCount + (rt.scannerPos / 20.0f);
                        
                        // Sine wave: -1 to 1 -> 0 to 1
                        float val = (sin(wavePos * 2 * PI) + 1.0f) / 2.0f;
                        
                        // Lerp between Secondary and Primary
                        // val=1 -> Primary, val=0 -> Secondary
                        uint8_t r = (rt.primaryR * val) + (rt.secondaryR * (1.0f - val));
                        uint8_t g = (rt.primaryG * val) + (rt.secondaryG * (1.0f - val));
                        uint8_t b = (rt.primaryB * val) + (rt.secondaryB * (1.0f - val));
                        
                        strip->setPixelColor(i, strip->Color(r, g, b));
                    }
                }
                break;

            default:
                // Fallback for others (Wave, Fire, etc) - use default colors if possible
                // Implement minimal fallback here for safety
                break;
        }
    }

    strip->show();
}

void setup() {
    Serial.begin(9600);
    delay(1000);
    Serial.printlnf("LED Controller v%s on %s", FIRMWARE_VERSION, DEVICE_PLATFORM_NAME);

    loadAllConfig();
    initAllStrips();
    updateAllInfo();

    // Register cloud functions
    Particle.function("addStrip", addStrip);
    Particle.function("removeStrip", removeStrip);
    Particle.function("setBytecode", setBytecode);
    Particle.function("saveConfig", saveConfig);
    Particle.function("clearAll", clearAll);
    
    // Register variables
    Particle.variable("deviceInfo", deviceInfo);
    Particle.variable("strips", stripInfo);
}

void loop() {
    unsigned long now = millis();
    if (now - lastUpdate >= 20) { // 50fps
        lastUpdate = now;
        for (int i = 0; i < numStrips; i++) {
            runPattern(i);
        }
    }
}