// Particle Argon WS2812B Candle Simulator
// Supports 8 RGB LEDs with cloud control and flash storage

#include "neopixel.h"

#define PIXEL_COUNT 8
#define PIXEL_PIN D2
#define PIXEL_TYPE WS2812B

Adafruit_NeoPixel strip(PIXEL_COUNT, PIXEL_PIN, PIXEL_TYPE);

// Flash storage addresses
#define EEPROM_PATTERN_ADDR 0
#define EEPROM_COLOR_ADDR 100
#define EEPROM_BRIGHTNESS_ADDR 200

// Pattern types
enum PatternType {
    CANDLE = 0,
    SOLID = 1,
    PULSE = 2,
    WAVE = 3,
    RAINBOW = 4,
    FIRE = 5
};

// Current state
struct LightConfig {
    PatternType pattern;
    uint8_t red;
    uint8_t green;
    uint8_t blue;
    uint8_t brightness;
    uint16_t speed;
} config;

// Candle effect variables
uint8_t heat[PIXEL_COUNT];
unsigned long lastUpdate = 0;
unsigned long patternDelay = 50;

void setup() {
    // Initialize serial for debugging
    Serial.begin(9600);

    // Initialize NeoPixel strip
    strip.begin();
    strip.setBrightness(255);
    strip.show();

    // Load configuration from flash
    loadConfiguration();

    // Register Particle cloud functions
    Particle.function("setPattern", setPattern);
    Particle.function("setColor", setColor);
    Particle.function("setBright", setBrightness);
    Particle.function("saveConfig", saveConfiguration);

    // Register Particle cloud variables
    Particle.variable("pattern", config.pattern);
    Particle.variable("brightness", config.brightness);

    Serial.println("Candle Lights initialized!");
}

void loop() {
    unsigned long currentMillis = millis();

    if (currentMillis - lastUpdate >= patternDelay) {
        lastUpdate = currentMillis;

        switch (config.pattern) {
            case CANDLE:
                candleEffect();
                break;
            case SOLID:
                solidColor();
                break;
            case PULSE:
                pulseEffect();
                break;
            case WAVE:
                waveEffect();
                break;
            case RAINBOW:
                rainbowEffect();
                break;
            case FIRE:
                fireEffect();
                break;
        }

        strip.show();
    }
}

// Cloud function to set pattern
// Format: "pattern:speed" e.g., "0:50" for CANDLE at 50ms
int setPattern(String command) {
    int colonIndex = command.indexOf(':');
    if (colonIndex > 0) {
        int pattern = command.substring(0, colonIndex).toInt();
        int speed = command.substring(colonIndex + 1).toInt();

        if (pattern >= 0 && pattern <= 5) {
            config.pattern = (PatternType)pattern;
            patternDelay = speed > 0 ? speed : 50;
            config.speed = patternDelay;
            Serial.printlnf("Pattern set to %d, speed %d", pattern, speed);
            return 1;
        }
    }
    return -1;
}

// Cloud function to set color
// Format: "R,G,B" e.g., "255,100,0"
int setColor(String command) {
    int firstComma = command.indexOf(',');
    int secondComma = command.indexOf(',', firstComma + 1);

    if (firstComma > 0 && secondComma > firstComma) {
        config.red = command.substring(0, firstComma).toInt();
        config.green = command.substring(firstComma + 1, secondComma).toInt();
        config.blue = command.substring(secondComma + 1).toInt();
        Serial.printlnf("Color set to R:%d G:%d B:%d", config.red, config.green, config.blue);
        return 1;
    }
    return -1;
}

// Cloud function to set brightness
// Format: "0-255"
int setBrightness(String command) {
    int brightness = command.toInt();
    if (brightness >= 0 && brightness <= 255) {
        config.brightness = brightness;
        strip.setBrightness(brightness);
        Serial.printlnf("Brightness set to %d", brightness);
        return 1;
    }
    return -1;
}

// Save current configuration to flash
int saveConfiguration(String command) {
    EEPROM.put(EEPROM_PATTERN_ADDR, config.pattern);
    EEPROM.put(EEPROM_COLOR_ADDR, config.red);
    EEPROM.put(EEPROM_COLOR_ADDR + 1, config.green);
    EEPROM.put(EEPROM_COLOR_ADDR + 2, config.blue);
    EEPROM.put(EEPROM_BRIGHTNESS_ADDR, config.brightness);
    EEPROM.put(EEPROM_BRIGHTNESS_ADDR + 1, config.speed);
    Serial.println("Configuration saved to flash");
    return 1;
}

// Load configuration from flash
void loadConfiguration() {
    EEPROM.get(EEPROM_PATTERN_ADDR, config.pattern);
    EEPROM.get(EEPROM_COLOR_ADDR, config.red);
    EEPROM.get(EEPROM_COLOR_ADDR + 1, config.green);
    EEPROM.get(EEPROM_COLOR_ADDR + 2, config.blue);
    EEPROM.get(EEPROM_BRIGHTNESS_ADDR, config.brightness);
    EEPROM.get(EEPROM_BRIGHTNESS_ADDR + 1, config.speed);

    // Validate loaded values
    if (config.pattern > 5) config.pattern = CANDLE;
    if (config.brightness == 0 || config.brightness > 255) config.brightness = 128;
    if (config.speed == 0) config.speed = 50;
    if (config.red == 0 && config.green == 0 && config.blue == 0) {
        config.red = 255;
        config.green = 100;
        config.blue = 0;
    }

    patternDelay = config.speed;
    strip.setBrightness(config.brightness);

    Serial.printlnf("Configuration loaded - Pattern:%d, RGB:(%d,%d,%d), Brightness:%d",
                    config.pattern, config.red, config.green, config.blue, config.brightness);
}

// Pattern Effects

void candleEffect() {
    // Realistic candle flicker simulation
    for (int i = 0; i < PIXEL_COUNT; i++) {
        // Random flicker
        int flicker = random(-30, 30);
        int r = constrain(config.red + flicker, 0, 255);
        int g = constrain(config.green + flicker/2, 0, 255);
        int b = constrain(config.blue, 0, 255);

        // Occasional stronger flickers
        if (random(100) < 5) {
            r = constrain(r - random(50), 0, 255);
            g = constrain(g - random(30), 0, 255);
        }

        strip.setPixelColor(i, strip.Color(r, g, b));
    }
}

void solidColor() {
    for (int i = 0; i < PIXEL_COUNT; i++) {
        strip.setPixelColor(i, strip.Color(config.red, config.green, config.blue));
    }
}

void pulseEffect() {
    static uint8_t pulseValue = 0;
    static int8_t pulseDirection = 1;

    pulseValue += pulseDirection * 5;
    if (pulseValue >= 255 || pulseValue <= 0) {
        pulseDirection = -pulseDirection;
    }

    uint8_t r = (config.red * pulseValue) / 255;
    uint8_t g = (config.green * pulseValue) / 255;
    uint8_t b = (config.blue * pulseValue) / 255;

    for (int i = 0; i < PIXEL_COUNT; i++) {
        strip.setPixelColor(i, strip.Color(r, g, b));
    }
}

void waveEffect() {
    static uint16_t wavePosition = 0;
    wavePosition++;

    for (int i = 0; i < PIXEL_COUNT; i++) {
        uint8_t brightness = (sin((i + wavePosition) * 0.5) + 1) * 127;
        uint8_t r = (config.red * brightness) / 255;
        uint8_t g = (config.green * brightness) / 255;
        uint8_t b = (config.blue * brightness) / 255;
        strip.setPixelColor(i, strip.Color(r, g, b));
    }
}

void rainbowEffect() {
    static uint16_t rainbowPosition = 0;
    rainbowPosition++;

    for (int i = 0; i < PIXEL_COUNT; i++) {
        strip.setPixelColor(i, Wheel(((i * 256 / PIXEL_COUNT) + rainbowPosition) & 255));
    }
}

void fireEffect() {
    // Fire simulation using heat array
    static uint8_t cooling = 50;
    static uint8_t sparking = 120;

    // Cool down every cell
    for (int i = 0; i < PIXEL_COUNT; i++) {
        heat[i] = qsub8(heat[i], random8(0, ((cooling * 10) / PIXEL_COUNT) + 2));
    }

    // Heat from each cell drifts up
    for (int k = PIXEL_COUNT - 1; k >= 2; k--) {
        heat[k] = (heat[k - 1] + heat[k - 2] + heat[k - 2]) / 3;
    }

    // Randomly ignite new sparks
    if (random8() < sparking) {
        int y = random8(3);
        heat[y] = qadd8(heat[y], random8(160, 255));
    }

    // Convert heat to LED colors
    for (int j = 0; j < PIXEL_COUNT; j++) {
        uint8_t colorindex = scale8(heat[j], 240);

        uint8_t r = 0, g = 0, b = 0;
        if (colorindex < 85) {
            r = colorindex * 3;
            g = 0;
            b = 0;
        } else if (colorindex < 170) {
            r = 255;
            g = (colorindex - 85) * 3;
            b = 0;
        } else {
            r = 255;
            g = 255;
            b = (colorindex - 170) * 3;
        }

        strip.setPixelColor(j, strip.Color(r, g, b));
    }
}

// Helper functions for effects
uint32_t Wheel(byte WheelPos) {
    if (WheelPos < 85) {
        return strip.Color(WheelPos * 3, 255 - WheelPos * 3, 0);
    } else if (WheelPos < 170) {
        WheelPos -= 85;
        return strip.Color(255 - WheelPos * 3, 0, WheelPos * 3);
    } else {
        WheelPos -= 170;
        return strip.Color(0, WheelPos * 3, 255 - WheelPos * 3);
    }
}

uint8_t qsub8(uint8_t i, uint8_t j) {
    int result = (int)i - (int)j;
    if (result < 0) result = 0;
    return result;
}

uint8_t qadd8(uint8_t i, uint8_t j) {
    unsigned int result = (unsigned int)i + (unsigned int)j;
    if (result > 255) result = 255;
    return result;
}

uint8_t scale8(uint8_t i, uint8_t scale) {
    return ((uint16_t)i * (uint16_t)scale) >> 8;
}

uint8_t random8(uint8_t min, uint8_t max) {
    return random(min, max);
}

uint8_t random8() {
    return random(256);
}
