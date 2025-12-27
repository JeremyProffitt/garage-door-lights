package shared

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LCL Bytecode Header
const (
	LCLMagic      = "LCL"
	LCLVersion    = 0x02
	LCLHeaderSize = 8
)

// Effect Type IDs (must match firmware candle-lights.ino)
const (
	EffectSolid    byte = 0x01
	EffectPulse    byte = 0x02 // breathing/pulse effect
	EffectSparkle  byte = 0x04
	EffectGradient byte = 0x05
	EffectFire     byte = 0x07
	EffectCandle   byte = 0x08
	EffectWave     byte = 0x09
	EffectRainbow  byte = 0x0A
)

// Opcodes
const (
	OpNOP         byte = 0x00
	OpPushU8      byte = 0x01
	OpPushU16     byte = 0x02
	OpPushColor   byte = 0x05
	OpPop         byte = 0x06
	OpDup         byte = 0x07
	OpSwap        byte = 0x08
	OpAdd         byte = 0x10
	OpSub         byte = 0x11
	OpMul         byte = 0x12
	OpDiv         byte = 0x13
	OpMod         byte = 0x14
	OpSin         byte = 0x30
	OpCos         byte = 0x31
	OpClamp       byte = 0x37
	OpLerp        byte = 0x38
	OpRGB         byte = 0x40
	OpHSV         byte = 0x41
	OpBlend       byte = 0x42
	OpBrightness  byte = 0x43
	OpPalette     byte = 0x44
	OpSetPattern  byte = 0x50
	OpSetParam    byte = 0x51
	OpBeginFrame  byte = 0x53
	OpEndFrame    byte = 0x54
	OpSetLED      byte = 0x55
	OpFill        byte = 0x56
	OpForEachLED  byte = 0x65
	OpNextLED     byte = 0x66
	OpHalt        byte = 0x67
	OpLoadVar     byte = 0x70
	OpStoreVar    byte = 0x71
	OpGetTime     byte = 0x72
	OpGetFrame    byte = 0x73
	OpGetLEDCount byte = 0x74
	OpGetLEDIndex byte = 0x75
)

// Parameter IDs for OpSetParam
const (
	ParamCooling   byte = 0x01
	ParamSparking  byte = 0x02
	ParamSpeed     byte = 0x03
	ParamBright    byte = 0x04
	ParamDirection byte = 0x05
	ParamWaveCount byte = 0x06
	ParamHeadSize  byte = 0x07
	ParamTailLen   byte = 0x08
	ParamDensity   byte = 0x09
	ParamRhythm    byte = 0x0A
)

// Semantic value mappings
var speedValues = map[string]int{
	"frozen":    0,
	"glacial":   1,
	"very_slow": 2,
	"slow":      3,
	"medium":    4,
	"fast":      5,
	"very_fast": 6,
	"frantic":   7,
}

var brightnessValues = map[string]int{
	"dim":    51,  // 20%
	"medium": 128, // 50%
	"bright": 204, // 80%
	"full":   255, // 100%
}

var flameHeightValues = map[string]int{
	"very_short": 200,
	"short":      150,
	"medium":     100,
	"tall":       55,
	"very_tall":  20,
}

var sparkFrequencyValues = map[string]int{
	"rare":       30,
	"occasional": 60,
	"frequent":   120,
	"high":       180,
	"intense":    230,
}

var waveCountValues = map[string]int{
	"one":     1,
	"few":     2,
	"several": 4,
	"many":    8,
}

var densityValues = map[string]int{
	"sparse": 5,
	"light":  13,
	"medium": 26,
	"dense":  51,
	"packed": 102,
}

var effectTypes = map[string]byte{
	"solid":    EffectSolid,
	"pulse":    EffectPulse,
	"breathe":  EffectPulse, // alias
	"sparkle":  EffectSparkle,
	"gradient": EffectGradient,
	"fire":     EffectFire,
	"candle":   EffectCandle,
	"wave":     EffectWave,
	"rainbow":  EffectRainbow,
}

// Color schemes as RGB palettes
var colorSchemes = map[string][][3]byte{
	"classic_fire": {{0, 0, 0}, {51, 17, 0}, {255, 68, 0}, {255, 170, 0}, {255, 255, 255}},
	"warm_orange":  {{0, 0, 0}, {51, 34, 0}, {255, 136, 0}, {255, 204, 0}, {255, 255, 255}},
	"blue_gas":     {{0, 0, 0}, {0, 0, 51}, {0, 68, 255}, {0, 255, 255}, {255, 255, 255}},
	"green_toxic":  {{0, 0, 0}, {0, 51, 0}, {0, 255, 0}, {170, 255, 0}, {255, 255, 255}},
	"purple_mystical": {{0, 0, 0}, {51, 0, 51}, {255, 0, 255}, {255, 102, 255}, {255, 255, 255}},
	"rainbow":      {{255, 0, 0}, {255, 127, 0}, {255, 255, 0}, {0, 255, 0}, {0, 0, 255}, {139, 0, 255}},
	"ocean":        {{0, 0, 51}, {0, 68, 170}, {0, 170, 255}, {0, 255, 255}, {255, 255, 255}},
	"sunset":       {{255, 68, 0}, {255, 136, 68}, {255, 68, 170}, {68, 0, 102}, {0, 0, 51}},
	"forest":       {{0, 34, 0}, {0, 102, 0}, {68, 170, 0}, {170, 255, 0}, {255, 255, 102}},
}

// LCLCompiler compiles LCL intent layer to bytecode
type LCLCompiler struct {
	bytecode []byte
	warnings []string
	errors   []string
}

// NewLCLCompiler creates a new LCL compiler
func NewLCLCompiler() *LCLCompiler {
	return &LCLCompiler{
		bytecode: make([]byte, 0, 256),
		warnings: make([]string, 0),
		errors:   make([]string, 0),
	}
}

// ParsedLCL represents a parsed LCL pattern
type ParsedLCL struct {
	Effect     string
	Name       string
	Behavior   map[string]string
	Appearance map[string]string
	Timing     map[string]string
	Spatial    map[string]string
}

// Compile compiles LCL text to bytecode
func (c *LCLCompiler) Compile(lclText string) ([]byte, []string, error) {
	c.bytecode = make([]byte, 0, 256)
	c.warnings = make([]string, 0)
	c.errors = make([]string, 0)

	// Parse LCL text
	parsed, err := c.parseLCL(lclText)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Validate effect type
	effectID, ok := effectTypes[parsed.Effect]
	if !ok {
		return nil, nil, fmt.Errorf("unknown effect type: %s", parsed.Effect)
	}

	// Generate bytecode
	c.generateBytecode(effectID, parsed)

	// Add header
	finalBytecode := c.addHeader(c.bytecode)

	return finalBytecode, c.warnings, nil
}

// Validate validates LCL without compiling
func (c *LCLCompiler) Validate(lclText string) (bool, []string) {
	c.warnings = make([]string, 0)
	c.errors = make([]string, 0)

	parsed, err := c.parseLCL(lclText)
	if err != nil {
		c.errors = append(c.errors, err.Error())
		return false, c.errors
	}

	// Check effect type
	if _, ok := effectTypes[parsed.Effect]; !ok {
		c.errors = append(c.errors, fmt.Sprintf("unknown effect: %s", parsed.Effect))
	}

	// Validate parameters based on effect type
	c.validateParameters(parsed)

	return len(c.errors) == 0, append(c.errors, c.warnings...)
}

func (c *LCLCompiler) parseLCL(text string) (*ParsedLCL, error) {
	parsed := &ParsedLCL{
		Behavior:   make(map[string]string),
		Appearance: make(map[string]string),
		Timing:     make(map[string]string),
		Spatial:    make(map[string]string),
	}

	lines := strings.Split(text, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for section headers
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
			section := strings.TrimSuffix(line, ":")
			switch section {
			case "behavior", "appearance", "timing", "spatial":
				currentSection = section
			default:
				currentSection = ""
			}
			continue
		}

		// Parse key: value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		// Handle top-level keys
		if currentSection == "" {
			switch key {
			case "effect":
				parsed.Effect = value
			case "name":
				parsed.Name = value
			}
		} else {
			// Handle section keys
			switch currentSection {
			case "behavior":
				parsed.Behavior[key] = value
			case "appearance":
				parsed.Appearance[key] = value
			case "timing":
				parsed.Timing[key] = value
			case "spatial":
				parsed.Spatial[key] = value
			}
		}
	}

	if parsed.Effect == "" {
		return nil, fmt.Errorf("missing required field: effect")
	}

	return parsed, nil
}

func (c *LCLCompiler) validateParameters(parsed *ParsedLCL) {
	switch parsed.Effect {
	case "fire":
		if v, ok := parsed.Behavior["flame_height"]; ok {
			if _, valid := flameHeightValues[v]; !valid {
				c.errors = append(c.errors, fmt.Sprintf("invalid flame_height: %s", v))
			}
		}
		if v, ok := parsed.Behavior["spark_frequency"]; ok {
			if _, valid := sparkFrequencyValues[v]; !valid {
				c.errors = append(c.errors, fmt.Sprintf("invalid spark_frequency: %s", v))
			}
		}
	case "wave":
		if v, ok := parsed.Behavior["wave_count"]; ok {
			if _, valid := waveCountValues[v]; !valid {
				c.errors = append(c.errors, fmt.Sprintf("invalid wave_count: %s", v))
			}
		}
	case "sparkle":
		if v, ok := parsed.Behavior["density"]; ok {
			if _, valid := densityValues[v]; !valid {
				c.errors = append(c.errors, fmt.Sprintf("invalid density: %s", v))
			}
		}
	}

	// Check brightness
	if v, ok := parsed.Appearance["brightness"]; ok {
		if _, valid := brightnessValues[v]; !valid {
			c.warnings = append(c.warnings, fmt.Sprintf("unknown brightness '%s', using 'medium'", v))
		}
	}

	// Check speed
	if v, ok := parsed.Timing["speed"]; ok {
		if _, valid := speedValues[v]; !valid {
			c.warnings = append(c.warnings, fmt.Sprintf("unknown speed '%s', using 'medium'", v))
		}
	}
}

func (c *LCLCompiler) generateBytecode(effectID byte, parsed *ParsedLCL) {
	// Set pattern type
	c.emit(OpSetPattern)
	c.emit(effectID)

	// Generate effect-specific bytecode
	switch parsed.Effect {
	case "fire":
		c.generateFireBytecode(parsed)
	case "wave":
		c.generateWaveBytecode(parsed)
	case "solid":
		c.generateSolidBytecode(parsed)
	case "rainbow":
		c.generateRainbowBytecode(parsed)
	case "sparkle":
		c.generateSparkleBytecode(parsed)
	case "breathe":
		c.generateBreatheBytecode(parsed)
	case "chase":
		c.generateChaseBytecode(parsed)
	default:
		c.generateDefaultBytecode(parsed)
	}

	// Set common parameters
	c.generateCommonParameters(parsed)

	// End with HALT
	c.emit(OpHalt)
}

func (c *LCLCompiler) generateFireBytecode(parsed *ParsedLCL) {
	// Set cooling (flame_height)
	cooling := 100 // default: medium
	if v, ok := parsed.Behavior["flame_height"]; ok {
		if val, exists := flameHeightValues[v]; exists {
			cooling = val
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(cooling))
	c.emit(OpSetParam)
	c.emit(ParamCooling)

	// Set sparking (spark_frequency)
	sparking := 120 // default: frequent
	if v, ok := parsed.Behavior["spark_frequency"]; ok {
		if val, exists := sparkFrequencyValues[v]; exists {
			sparking = val
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(sparking))
	c.emit(OpSetParam)
	c.emit(ParamSparking)

	// Set color palette from color_scheme
	c.generatePalette(parsed)
}

func (c *LCLCompiler) generateWaveBytecode(parsed *ParsedLCL) {
	// Set wave count
	waveCount := 2 // default: few
	if v, ok := parsed.Behavior["wave_count"]; ok {
		if val, exists := waveCountValues[v]; exists {
			waveCount = val
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(waveCount))
	c.emit(OpSetParam)
	c.emit(ParamWaveCount)

	c.generatePalette(parsed)
}

func (c *LCLCompiler) generateSolidBytecode(parsed *ParsedLCL) {
	// Parse color
	r, g, b := c.parseColor(parsed.Appearance["color"])
	c.emit(OpPushColor)
	c.emit(r)
	c.emit(g)
	c.emit(b)
}

func (c *LCLCompiler) generateRainbowBytecode(parsed *ParsedLCL) {
	// Rainbow is self-contained, just set speed
}

func (c *LCLCompiler) generateSparkleBytecode(parsed *ParsedLCL) {
	// Set density
	density := 26 // default: medium
	if v, ok := parsed.Behavior["density"]; ok {
		if val, exists := densityValues[v]; exists {
			density = val
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(density))
	c.emit(OpSetParam)
	c.emit(ParamDensity)
}

func (c *LCLCompiler) generateBreatheBytecode(parsed *ParsedLCL) {
	// Set rhythm
	rhythm := 4 // default: medium
	if v, ok := parsed.Behavior["rhythm"]; ok {
		switch v {
		case "calm":
			rhythm = 8
		case "relaxed":
			rhythm = 6
		case "steady":
			rhythm = 4
		case "energetic":
			rhythm = 2
		case "frantic":
			rhythm = 1
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(rhythm))
	c.emit(OpSetParam)
	c.emit(ParamRhythm)

	// Parse color
	r, g, b := c.parseColor(parsed.Appearance["color"])
	c.emit(OpPushColor)
	c.emit(r)
	c.emit(g)
	c.emit(b)
}

func (c *LCLCompiler) generateChaseBytecode(parsed *ParsedLCL) {
	// Set head size
	headSize := 3 // default: small
	if v, ok := parsed.Behavior["head_size"]; ok {
		switch v {
		case "tiny":
			headSize = 1
		case "small":
			headSize = 3
		case "medium":
			headSize = 5
		case "large":
			headSize = 10
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(headSize))
	c.emit(OpSetParam)
	c.emit(ParamHeadSize)

	// Set tail length
	tailLen := 10 // default: medium
	if v, ok := parsed.Behavior["tail_length"]; ok {
		switch v {
		case "none":
			tailLen = 0
		case "short":
			tailLen = 5
		case "medium":
			tailLen = 10
		case "long":
			tailLen = 20
		case "very_long":
			tailLen = 40
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(tailLen))
	c.emit(OpSetParam)
	c.emit(ParamTailLen)
}

func (c *LCLCompiler) generateDefaultBytecode(parsed *ParsedLCL) {
	// For unknown effects, just generate palette
	c.generatePalette(parsed)
}

func (c *LCLCompiler) generateCommonParameters(parsed *ParsedLCL) {
	// Set brightness
	brightness := 204 // default: bright
	if v, ok := parsed.Appearance["brightness"]; ok {
		if val, exists := brightnessValues[v]; exists {
			brightness = val
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(brightness))
	c.emit(OpSetParam)
	c.emit(ParamBright)

	// Set speed
	speed := 4 // default: medium
	if v, ok := parsed.Timing["speed"]; ok {
		if val, exists := speedValues[v]; exists {
			speed = val
		}
	}
	c.emit(OpPushU8)
	c.emit(byte(speed))
	c.emit(OpSetParam)
	c.emit(ParamSpeed)
}

func (c *LCLCompiler) generatePalette(parsed *ParsedLCL) {
	scheme := "classic_fire"
	if v, ok := parsed.Appearance["color_scheme"]; ok {
		scheme = v
	}

	if palette, ok := colorSchemes[scheme]; ok {
		// Emit palette colors
		for _, color := range palette {
			c.emit(OpPushColor)
			c.emit(color[0])
			c.emit(color[1])
			c.emit(color[2])
		}
		c.emit(OpPalette)
		c.emit(byte(len(palette)))
	}
}

func (c *LCLCompiler) parseColor(colorStr string) (byte, byte, byte) {
	if colorStr == "" {
		return 255, 255, 255 // default white
	}

	// Handle hex colors
	if strings.HasPrefix(colorStr, "#") {
		hex := strings.TrimPrefix(colorStr, "#")
		if len(hex) == 3 {
			// Short form #RGB
			hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
		}
		if len(hex) == 6 {
			r, _ := strconv.ParseUint(hex[0:2], 16, 8)
			g, _ := strconv.ParseUint(hex[2:4], 16, 8)
			b, _ := strconv.ParseUint(hex[4:6], 16, 8)
			return byte(r), byte(g), byte(b)
		}
	}

	// Handle named colors
	namedColors := map[string][3]byte{
		"red":        {255, 0, 0},
		"orange":     {255, 165, 0},
		"yellow":     {255, 255, 0},
		"green":      {0, 255, 0},
		"cyan":       {0, 255, 255},
		"blue":       {0, 0, 255},
		"purple":     {128, 0, 128},
		"magenta":    {255, 0, 255},
		"pink":       {255, 192, 203},
		"white":      {255, 255, 255},
		"warm_white": {255, 244, 229},
		"cool_white": {244, 255, 250},
		"black":      {0, 0, 0},
	}

	if color, ok := namedColors[strings.ToLower(colorStr)]; ok {
		return color[0], color[1], color[2]
	}

	// Handle rgb(...) format
	re := regexp.MustCompile(`rgb\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)`)
	if matches := re.FindStringSubmatch(colorStr); len(matches) == 4 {
		r, _ := strconv.Atoi(matches[1])
		g, _ := strconv.Atoi(matches[2])
		b, _ := strconv.Atoi(matches[3])
		return byte(r), byte(g), byte(b)
	}

	return 255, 255, 255 // default white
}

func (c *LCLCompiler) emit(b byte) {
	c.bytecode = append(c.bytecode, b)
}

func (c *LCLCompiler) addHeader(code []byte) []byte {
	// Calculate checksum (simple XOR)
	checksum := byte(0)
	for _, b := range code {
		checksum ^= b
	}

	// Create header
	header := make([]byte, LCLHeaderSize)
	copy(header[0:3], []byte(LCLMagic)) // "LCL"
	header[3] = LCLVersion              // Version
	binary.LittleEndian.PutUint16(header[4:6], uint16(len(code)))
	header[6] = checksum // Checksum
	header[7] = 0x00     // Flags

	return append(header, code...)
}

// CompileLCL is a convenience function to compile LCL text
func CompileLCL(lclText string) ([]byte, []string, error) {
	compiler := NewLCLCompiler()
	return compiler.Compile(lclText)
}

// ValidateLCL is a convenience function to validate LCL text
func ValidateLCL(lclText string) (bool, []string) {
	compiler := NewLCLCompiler()
	return compiler.Validate(lclText)
}
