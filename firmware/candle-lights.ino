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
    PATTERN_FIRE = 6
};

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

// Runtime strip data
struct StripRuntime {
    Adafruit_NeoPixel* strip;
    uint8_t heat[MAX_LEDS_PER_STRIP];  // For fire effect
    uint16_t animPosition;              // Animation position counter
    uint8_t pulseValue;                 // For pulse effect
    int8_t pulseDirection;              // Pulse direction
    uint8_t ledColorIndex[MAX_LEDS_PER_STRIP]; // Which color index each LED uses
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
                getColorForLed(idx, i, r, g, b);

                // Apply brightness flicker proportionally to all channels
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
