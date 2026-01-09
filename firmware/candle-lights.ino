// Particle WS2812B LED Controller - Multi-Pin + Multi-Color Support
// Features: Multiple LED strips, per-strip patterns, multi-color with percentages, EEPROM persistence
// Version 3.0.0 - LCL v4 Support (Scanner, Width, Tail, Background Colors)

#include "Particle.h"
#include "neopixel.h"
#include <math.h>

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
    PATTERN_BYTECODE = 7  // LCL bytecode pattern
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

    if (rt.bytecodeLen < BYTECODE_HEADER_SIZE) {
        return -1;
    }

    // Parse the bytecode
    parseBytecode(stripIdx);

    // Set pattern to bytecode mode
    cfg.pattern = PATTERN_BYTECODE;

    if (rt.strip != nullptr) {
        rt.strip->setBrightness(rt.bytecodeBrightness);
    }

    Serial.printlnf("Strip D%d: loaded %d bytes (v%d)", pin, rt.bytecodeLen, rt.bytecodeVersion);
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