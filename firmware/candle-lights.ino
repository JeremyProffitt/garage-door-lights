// Particle WS2812B LED Controller - Multi-Pin + Multi-Color Support
// Features: Multiple LED strips, per-strip patterns, multi-color with percentages, EEPROM persistence
// Version 2.2.0 - Full config readable via cloud variables

#include "Particle.h"
#include "neopixel.h"
#include <math.h>

// =============================================================================
// CONFIGURATION
// =============================================================================

#define MAX_STRIPS 4           // Maximum number of LED strips supported
#define MAX_LEDS_PER_STRIP 60  // Maximum LEDs per strip
#define MAX_COLORS_PER_STRIP 8 // Maximum colors per strip
#define PIXEL_TYPE WS2812B     // LED type for all strips

// Default values
#define DEFAULT_RED 255
#define DEFAULT_GREEN 100
#define DEFAULT_BLUE 0
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
    PATTERN_BYTECODE = 7  // LCL bytecode pattern
};

// Bytecode constants
#define MAX_BYTECODE_SIZE 256
#define BYTECODE_HEADER_SIZE 8

// Bytecode opcodes (must match lcl_compiler.go)
#define OP_PUSH_COLOR  0x05
#define OP_PALETTE     0x44
#define OP_SET_PATTERN 0x50
#define OP_SET_PARAM   0x51
#define OP_HALT        0x67

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

// Parameter IDs from bytecode (must match lcl_compiler.go)
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

// Color entry with percentage
struct ColorEntry {
    uint8_t r;
    uint8_t g;
    uint8_t b;
    uint8_t percent;  // Percentage of LEDs for this color (0-100)
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
    uint8_t pulseValue;                 // For pulse effect
    int8_t pulseDirection;              // Pulse direction
    uint8_t ledColorIndex[MAX_LEDS_PER_STRIP]; // Which color index each LED uses
    // Bytecode VM state
    uint8_t bytecode[MAX_BYTECODE_SIZE];
    uint16_t bytecodeLen;
    uint8_t bytecodeEffect;             // Parsed effect type from bytecode
    uint8_t bytecodeR, bytecodeG, bytecodeB;
    uint8_t bytecodeBrightness;
    uint8_t bytecodeSpeed;
    // Palette support
    PaletteColor palette[MAX_PALETTE_COLORS];
    uint8_t paletteCount;               // Number of colors in palette
    // Chase effect parameters
    uint8_t headSize;                   // Size of chase head (LEDs)
    uint8_t tailLength;                 // Length of chase tail (LEDs)
};

// =============================================================================
// GLOBALS
// =============================================================================

#define FIRMWARE_VERSION "2.2.0"

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

// Parse bytecode and extract parameters
void parseBytecode(int stripIdx) {
    StripRuntime& rt = stripRuntime[stripIdx];

    // Validate header
    if (rt.bytecodeLen < BYTECODE_HEADER_SIZE) return;
    if (rt.bytecode[0] != 'L' || rt.bytecode[1] != 'C' || rt.bytecode[2] != 'L') return;

    // Set defaults
    rt.bytecodeEffect = EFFECT_SOLID;
    rt.bytecodeR = 255;
    rt.bytecodeG = 147;
    rt.bytecodeB = 41;
    rt.bytecodeBrightness = 128;
    rt.bytecodeSpeed = 50;
    rt.paletteCount = 0;
    rt.headSize = 5;       // Default chase head size
    rt.tailLength = 10;    // Default chase tail length

    // Color stack for palette building
    PaletteColor colorStack[MAX_PALETTE_COLORS];
    uint8_t stackSize = 0;

    // Execute bytecode instructions
    uint16_t pc = BYTECODE_HEADER_SIZE;
    while (pc < rt.bytecodeLen) {
        uint8_t op = rt.bytecode[pc];

        switch (op) {
            case OP_PUSH_COLOR:
                // Push RGB color onto stack (3 bytes follow: R, G, B)
                if (pc + 3 < rt.bytecodeLen && stackSize < MAX_PALETTE_COLORS) {
                    colorStack[stackSize].r = rt.bytecode[pc + 1];
                    colorStack[stackSize].g = rt.bytecode[pc + 2];
                    colorStack[stackSize].b = rt.bytecode[pc + 3];
                    stackSize++;
                    pc += 4;
                } else {
                    pc++;
                }
                break;

            case OP_PALETTE:
                // Pop N colors from stack and set as palette (1 byte follows: count)
                if (pc + 1 < rt.bytecodeLen) {
                    uint8_t count = rt.bytecode[pc + 1];
                    if (count > stackSize) count = stackSize;
                    if (count > MAX_PALETTE_COLORS) count = MAX_PALETTE_COLORS;

                    // Copy colors from stack to palette
                    // Colors are pushed in order, take the last N
                    uint8_t startIdx = (stackSize >= count) ? stackSize - count : 0;
                    for (uint8_t i = 0; i < count; i++) {
                        rt.palette[i] = colorStack[startIdx + i];
                    }
                    rt.paletteCount = count;

                    // Also set primary color to first palette color
                    if (count > 0) {
                        rt.bytecodeR = rt.palette[0].r;
                        rt.bytecodeG = rt.palette[0].g;
                        rt.bytecodeB = rt.palette[0].b;
                    }

                    // Clear stack
                    stackSize = 0;
                    pc += 2;
                } else {
                    pc++;
                }
                break;

            case OP_SET_PATTERN:
                if (pc + 1 < rt.bytecodeLen) {
                    rt.bytecodeEffect = rt.bytecode[pc + 1];
                    pc += 2;
                } else {
                    pc++;
                }
                break;

            case OP_SET_PARAM:
                if (pc + 2 < rt.bytecodeLen) {
                    uint8_t paramId = rt.bytecode[pc + 1];
                    uint8_t value = rt.bytecode[pc + 2];
                    switch (paramId) {
                        case PARAM_RED: rt.bytecodeR = value; break;
                        case PARAM_GREEN: rt.bytecodeG = value; break;
                        case PARAM_BLUE: rt.bytecodeB = value; break;
                        case PARAM_BRIGHTNESS: rt.bytecodeBrightness = value; break;
                        case PARAM_SPEED: rt.bytecodeSpeed = value; break;
                        case PARAM_HEAD_SIZE: rt.headSize = value; break;
                        case PARAM_TAIL_LEN: rt.tailLength = value; break;
                    }
                    pc += 3;
                } else {
                    pc++;
                }
                break;

            case OP_HALT:
                return;

            default:
                pc++;
                break;
        }
    }
}

// Set bytecode for a strip: "pin,base64EncodedBytecode"
// Example: "6,TENMAQ..." (base64 encoded LCL bytecode)
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
    // Simple base64 decoder
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

    if (rt.bytecodeLen < BYTECODE_HEADER_SIZE) {
        Serial.println("Bytecode too short");
        return -1;
    }

    // Parse the bytecode to extract effect and parameters
    parseBytecode(stripIdx);

    // Set pattern to bytecode mode
    cfg.pattern = PATTERN_BYTECODE;

    // Apply brightness from bytecode
    if (rt.strip != nullptr) {
        rt.strip->setBrightness(rt.bytecodeBrightness);
    }

    Serial.printlnf("Strip D%d: bytecode loaded (%d bytes), effect=%d, RGB=(%d,%d,%d), palette=%d colors, head=%d, tail=%d",
                    pin, rt.bytecodeLen, rt.bytecodeEffect,
                    rt.bytecodeR, rt.bytecodeG, rt.bytecodeB, rt.paletteCount,
                    rt.headSize, rt.tailLength);

    // Log palette colors if present
    if (rt.paletteCount > 0) {
        Serial.print("  Palette: ");
        for (int i = 0; i < rt.paletteCount; i++) {
            Serial.printf("[%d,%d,%d] ", rt.palette[i].r, rt.palette[i].g, rt.palette[i].b);
        }
        Serial.println();
    }

    return rt.bytecodeLen;
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
    uint8_t r, g, b;

    switch (cfg.pattern) {
        case PATTERN_OFF:
            strip->clear();
            break;

        case PATTERN_CANDLE:
            for (int i = 0; i < count; i++) {
                // Blend between all configured colors for flame palette
                if (cfg.colorCount == 1) {
                    r = cfg.colors[0].r;
                    g = cfg.colors[0].g;
                    b = cfg.colors[0].b;
                } else {
                    // Random blend position through the color palette
                    int blendPos = random(0, 256);
                    int totalColors = cfg.colorCount;
                    int segment = 256 / totalColors;
                    int colorIdx = blendPos / segment;
                    if (colorIdx >= totalColors) colorIdx = totalColors - 1;
                    int nextIdx = (colorIdx + 1) % totalColors;
                    int blend = (blendPos % segment) * 256 / segment;

                    // Lerp between two adjacent colors
                    r = ((cfg.colors[colorIdx].r * (256 - blend)) + (cfg.colors[nextIdx].r * blend)) / 256;
                    g = ((cfg.colors[colorIdx].g * (256 - blend)) + (cfg.colors[nextIdx].g * blend)) / 256;
                    b = ((cfg.colors[colorIdx].b * (256 - blend)) + (cfg.colors[nextIdx].b * blend)) / 256;
                }

                // Apply brightness flicker
                int flickerPercent = random(70, 130);
                r = constrain((r * flickerPercent) / 100, 0, 255);
                g = constrain((g * flickerPercent) / 100, 0, 255);
                b = constrain((b * flickerPercent) / 100, 0, 255);

                // Occasional deeper flicker (5% chance)
                if (random(100) < 5) {
                    int dimPercent = random(50, 80);
                    r = (r * dimPercent) / 100;
                    g = (g * dimPercent) / 100;
                    b = (b * dimPercent) / 100;
                }
                strip->setPixelColor(i, strip->Color(r, g, b));
            }
            break;

        case PATTERN_SOLID:
            for (int i = 0; i < count; i++) {
                getColorForLed(idx, i, r, g, b);
                strip->setPixelColor(i, strip->Color(r, g, b));
            }
            break;

        case PATTERN_PULSE:
            rt.pulseValue += rt.pulseDirection * 5;
            if (rt.pulseValue >= 250 || rt.pulseValue <= 5) {
                rt.pulseDirection = -rt.pulseDirection;
            }
            for (int i = 0; i < count; i++) {
                getColorForLed(idx, i, r, g, b);
                r = (r * rt.pulseValue) / 255;
                g = (g * rt.pulseValue) / 255;
                b = (b * rt.pulseValue) / 255;
                strip->setPixelColor(i, strip->Color(r, g, b));
            }
            break;

        case PATTERN_WAVE:
            rt.animPosition++;
            for (int i = 0; i < count; i++) {
                getColorForLed(idx, i, r, g, b);
                uint8_t brightness = (sin((i + rt.animPosition) * 0.5) + 1) * 127;
                r = (r * brightness) / 255;
                g = (g * brightness) / 255;
                b = (b * brightness) / 255;
                strip->setPixelColor(i, strip->Color(r, g, b));
            }
            break;

        case PATTERN_RAINBOW:
            rt.animPosition++;
            for (int i = 0; i < count; i++) {
                uint8_t pos = ((i * 256 / count) + rt.animPosition) & 255;
                uint32_t color;
                if (pos < 85) {
                    color = strip->Color(pos * 3, 255 - pos * 3, 0);
                } else if (pos < 170) {
                    pos -= 85;
                    color = strip->Color(255 - pos * 3, 0, pos * 3);
                } else {
                    pos -= 170;
                    color = strip->Color(0, pos * 3, 255 - pos * 3);
                }
                strip->setPixelColor(i, color);
            }
            break;

        case PATTERN_FIRE:
            {
                uint8_t cooling = 50;
                uint8_t sparking = 120;

                for (int i = 0; i < count; i++) {
                    int cooldown = random(0, ((cooling * 10) / count) + 2);
                    rt.heat[i] = (rt.heat[i] > cooldown) ? rt.heat[i] - cooldown : 0;
                }

                for (int k = count - 1; k >= 2; k--) {
                    rt.heat[k] = (rt.heat[k - 1] + rt.heat[k - 2] + rt.heat[k - 2]) / 3;
                }

                if (random(255) < sparking) {
                    int y = random(0, 3);
                    if (y < count) {
                        rt.heat[y] = min(255, rt.heat[y] + random(160, 255));
                    }
                }

                for (int j = 0; j < count; j++) {
                    uint8_t t = (rt.heat[j] * 240) / 256;
                    if (t < 85) {
                        r = t * 3; g = 0; b = 0;
                    } else if (t < 170) {
                        r = 255; g = (t - 85) * 3; b = 0;
                    } else {
                        r = 255; g = 255; b = (t - 170) * 3;
                    }
                    strip->setPixelColor(j, strip->Color(r, g, b));
                }
            }
            break;

        case PATTERN_BYTECODE:
            // Execute pattern based on parsed bytecode effect type
            // Map bytecode effect to native patterns using parsed parameters
            switch (rt.bytecodeEffect) {
                case EFFECT_SOLID:
                    for (int i = 0; i < count; i++) {
                        strip->setPixelColor(i, strip->Color(rt.bytecodeR, rt.bytecodeG, rt.bytecodeB));
                    }
                    break;

                case EFFECT_PULSE:
                    rt.pulseValue += rt.pulseDirection * 5;
                    if (rt.pulseValue >= 250 || rt.pulseValue <= 5) {
                        rt.pulseDirection = -rt.pulseDirection;
                    }
                    for (int i = 0; i < count; i++) {
                        r = (rt.bytecodeR * rt.pulseValue) / 255;
                        g = (rt.bytecodeG * rt.pulseValue) / 255;
                        b = (rt.bytecodeB * rt.pulseValue) / 255;
                        strip->setPixelColor(i, strip->Color(r, g, b));
                    }
                    break;

                case EFFECT_WAVE:
                    // Wave effect - palette colors distributed across entire strip, scrolling
                    rt.animPosition++;
                    {
                        if (rt.paletteCount > 0) {
                            // Distribute palette colors across entire strip with smooth blending
                            for (int i = 0; i < count; i++) {
                                // Calculate position in the palette cycle (0.0 to paletteCount)
                                // Animation shifts the pattern over time
                                float pos = (float)(i + rt.animPosition) * rt.paletteCount / count;

                                // Wrap around
                                while (pos >= rt.paletteCount) pos -= rt.paletteCount;
                                while (pos < 0) pos += rt.paletteCount;

                                // Get the two colors to blend between
                                int colorIdx1 = (int)pos;
                                int colorIdx2 = (colorIdx1 + 1) % rt.paletteCount;
                                float blend = pos - colorIdx1;  // 0.0 to 1.0

                                // Linear interpolation between colors
                                r = rt.palette[colorIdx1].r + (rt.palette[colorIdx2].r - rt.palette[colorIdx1].r) * blend;
                                g = rt.palette[colorIdx1].g + (rt.palette[colorIdx2].g - rt.palette[colorIdx1].g) * blend;
                                b = rt.palette[colorIdx1].b + (rt.palette[colorIdx2].b - rt.palette[colorIdx1].b) * blend;

                                strip->setPixelColor(i, strip->Color(r, g, b));
                            }
                        } else {
                            // No palette - use primary color with brightness wave
                            for (int i = 0; i < count; i++) {
                                float phase = (float)(i + rt.animPosition) * 2.0 * 3.14159 / count;
                                float brightness = 0.3 + 0.7 * (sin(phase) * 0.5 + 0.5);
                                r = rt.bytecodeR * brightness;
                                g = rt.bytecodeG * brightness;
                                b = rt.bytecodeB * brightness;
                                strip->setPixelColor(i, strip->Color(r, g, b));
                            }
                        }
                    }
                    break;

                case EFFECT_RAINBOW:
                    rt.animPosition++;
                    for (int i = 0; i < count; i++) {
                        uint8_t pos = ((i * 256 / count) + rt.animPosition) & 255;
                        uint32_t color;
                        if (pos < 85) {
                            color = strip->Color(pos * 3, 255 - pos * 3, 0);
                        } else if (pos < 170) {
                            pos -= 85;
                            color = strip->Color(255 - pos * 3, 0, pos * 3);
                        } else {
                            pos -= 170;
                            color = strip->Color(0, pos * 3, 255 - pos * 3);
                        }
                        strip->setPixelColor(i, color);
                    }
                    break;

                case EFFECT_FIRE:
                case EFFECT_CANDLE:
                    {
                        uint8_t cooling = 50;
                        uint8_t sparking = 120;
                        for (int i = 0; i < count; i++) {
                            int cooldown = random(0, ((cooling * 10) / count) + 2);
                            rt.heat[i] = (rt.heat[i] > cooldown) ? rt.heat[i] - cooldown : 0;
                        }
                        for (int k = count - 1; k >= 2; k--) {
                            rt.heat[k] = (rt.heat[k - 1] + rt.heat[k - 2] + rt.heat[k - 2]) / 3;
                        }
                        if (random(255) < sparking) {
                            int y = random(0, 3);
                            if (y < count) {
                                rt.heat[y] = min(255, rt.heat[y] + random(160, 255));
                            }
                        }
                        for (int j = 0; j < count; j++) {
                            uint8_t t = rt.heat[j];

                            if (rt.paletteCount > 1) {
                                // Use palette colors - interpolate based on heat
                                float scaledPos = (t / 255.0f) * (rt.paletteCount - 1);
                                int idx1 = (int)scaledPos;
                                int idx2 = idx1 + 1;
                                if (idx2 >= rt.paletteCount) idx2 = rt.paletteCount - 1;
                                float blend = scaledPos - idx1;

                                // Lerp between two palette colors
                                r = rt.palette[idx1].r + (rt.palette[idx2].r - rt.palette[idx1].r) * blend;
                                g = rt.palette[idx1].g + (rt.palette[idx2].g - rt.palette[idx1].g) * blend;
                                b = rt.palette[idx1].b + (rt.palette[idx2].b - rt.palette[idx1].b) * blend;
                            } else {
                                // Default fire colors
                                t = (t * 240) / 256;
                                if (t < 85) {
                                    r = t * 3; g = 0; b = 0;
                                } else if (t < 170) {
                                    r = 255; g = (t - 85) * 3; b = 0;
                                } else {
                                    r = 255; g = 255; b = (t - 170) * 3;
                                }
                            }
                            strip->setPixelColor(j, strip->Color(r, g, b));
                        }
                    }
                    break;

                case EFFECT_GRADIENT:
                    // Gradient effect - static palette distribution across strip
                    {
                        if (rt.paletteCount > 0) {
                            // Distribute palette colors across entire strip with smooth blending
                            for (int i = 0; i < count; i++) {
                                // Calculate position in the palette (0.0 to paletteCount)
                                float pos = (float)i * rt.paletteCount / count;

                                // Wrap around (shouldn't be needed for gradient but safety)
                                while (pos >= rt.paletteCount) pos -= rt.paletteCount;

                                // Get the two colors to blend between
                                int colorIdx1 = (int)pos;
                                int colorIdx2 = (colorIdx1 + 1) % rt.paletteCount;
                                float blend = pos - colorIdx1;  // 0.0 to 1.0

                                // Linear interpolation between colors
                                r = rt.palette[colorIdx1].r + (rt.palette[colorIdx2].r - rt.palette[colorIdx1].r) * blend;
                                g = rt.palette[colorIdx1].g + (rt.palette[colorIdx2].g - rt.palette[colorIdx1].g) * blend;
                                b = rt.palette[colorIdx1].b + (rt.palette[colorIdx2].b - rt.palette[colorIdx1].b) * blend;

                                strip->setPixelColor(i, strip->Color(r, g, b));
                            }
                        } else {
                            // No palette - use primary color
                            for (int i = 0; i < count; i++) {
                                strip->setPixelColor(i, strip->Color(rt.bytecodeR, rt.bytecodeG, rt.bytecodeB));
                            }
                        }
                    }
                    break;

                default:
                    // Default to solid with bytecode color
                    for (int i = 0; i < count; i++) {
                        strip->setPixelColor(i, strip->Color(rt.bytecodeR, rt.bytecodeG, rt.bytecodeB));
                    }
                    break;
            }
            break;
    }

    strip->show();
}

// =============================================================================
// SETUP & LOOP
// =============================================================================

void setup() {
    Serial.begin(9600);
    delay(1000);

    Serial.printlnf("LED Controller v%s on %s", FIRMWARE_VERSION, DEVICE_PLATFORM_NAME);

    for (int i = 0; i < MAX_STRIPS; i++) {
        stripRuntime[i].strip = nullptr;
    }

    loadAllConfig();
    initAllStrips();
    updateAllInfo();

    snprintf(deviceInfo, sizeof(deviceInfo), "%s|%s|%d|%d|%d",
             FIRMWARE_VERSION, DEVICE_PLATFORM_NAME, MAX_STRIPS, MAX_LEDS_PER_STRIP, MAX_COLORS_PER_STRIP);

    // Initialize query result
    queryResult[0] = '\0';

    // Register cloud functions
    Particle.function("addStrip", addStrip);       // "pin,ledCount"
    Particle.function("removeStrip", removeStrip); // "pin"
    Particle.function("setPattern", setPattern);   // "pin,pattern,speed"
    Particle.function("setColor", setColor);       // "pin,R,G,B"
    Particle.function("setColors", setColors);     // "pin,R,G,B,%;R,G,B,%;..."
    Particle.function("setBright", setBrightness); // "pin,brightness"
    Particle.function("saveConfig", saveConfig);   // ""
    Particle.function("clearAll", clearAll);       // ""
    Particle.function("getConfig", getConfig);     // "" - refreshes variables
    Particle.function("getStrip", getStrip);       // "pin" - result in query variable
    Particle.function("getColors", getColors);     // "pin" - result in query variable
    Particle.function("setBytecode", setBytecode); // "pin,base64Bytecode" - LCL bytecode pattern

    // Register cloud variables
    Particle.variable("deviceInfo", deviceInfo);   // "version|platform|maxStrips|maxLeds|maxColors"
    Particle.variable("strips", stripInfo);        // "D6:8:1:128:50:2;D2:12:5:255:30:1"
    Particle.variable("colors", colorsInfo);       // "6=255,0,0,50;0,255,0,50|2=255,100,0,100"
    Particle.variable("numStrips", numStrips);     // Number of configured strips
    Particle.variable("query", queryResult);       // Result of getStrip/getColors calls

    Serial.printlnf("Ready with %d strips", numStrips);
}

void loop() {
    unsigned long now = millis();

    uint8_t minDelay = 255;
    for (int i = 0; i < numStrips; i++) {
        if (stripConfigs[i].speed < minDelay) {
            minDelay = stripConfigs[i].speed;
        }
    }
    if (minDelay == 0) minDelay = 50;

    if (now - lastUpdate >= minDelay) {
        lastUpdate = now;

        for (int i = 0; i < numStrips; i++) {
            runPattern(i);
        }
    }
}
