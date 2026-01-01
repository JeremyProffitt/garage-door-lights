package shared

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// LCL Bytecode Format Version 3 - Fixed Format (no opcodes)
// This is a simplified format that uses fixed byte positions instead of opcodes.
//
// HEADER (8 bytes):
//   [0-2]   Magic "LCL"
//   [3]     Version = 0x03
//   [4-5]   Total length (16-bit big-endian)
//   [6]     Checksum (XOR of bytes 8+)
//   [7]     Flags
//
// CORE PARAMETERS (8 bytes):
//   [8]     Effect type
//   [9]     Brightness (0-255)
//   [10]    Speed (0-255)
//   [11]    Param1 (effect-specific: density, rhythm, cooling, wave_count)
//   [12]    Param2 (effect-specific: sparking)
//   [13]    Param3 (reserved)
//   [14]    Direction (0=forward, 1=reverse)
//   [15]    Reserved
//
// PRIMARY COLOR (3 bytes):
//   [16-18] R, G, B
//
// PALETTE (1 + 3*N bytes):
//   [19]    Color count (0-8)
//   [20+]   Colors (R, G, B each)

const (
	LCLMagic          = "LCL"
	LCLVersion        = 0x03 // Version 3: Fixed format
	LCLHeaderSize     = 8
	LCLCoreParamsSize = 8
	LCLColorSize      = 3
	MaxPaletteColors  = 8
)

// Byte offsets in the fixed-format bytecode
const (
	OffsetMagic      = 0
	OffsetVersion    = 3
	OffsetLength     = 4
	OffsetChecksum   = 6
	OffsetFlags      = 7
	OffsetEffect     = 8
	OffsetBrightness = 9
	OffsetSpeed      = 10
	OffsetParam1     = 11
	OffsetParam2     = 12
	OffsetParam3     = 13
	OffsetDirection  = 14
	OffsetReserved   = 15
	OffsetColorR     = 16
	OffsetColorG     = 17
	OffsetColorB     = 18
	OffsetColorCount = 19
	OffsetPalette    = 20
)

// Effect Type IDs (must match firmware)
const (
	EffectSolid    byte = 0x01
	EffectPulse    byte = 0x02
	EffectSparkle  byte = 0x04
	EffectGradient byte = 0x05
	EffectFire     byte = 0x07
	EffectCandle   byte = 0x08
	EffectWave     byte = 0x09
	EffectRainbow  byte = 0x0A
)

// Effect type name to ID mapping
var effectTypes = map[string]byte{
	"solid":    EffectSolid,
	"pulse":    EffectPulse,
	"breathe":  EffectPulse, // alias
	"sparkle":  EffectSparkle,
	"gradient": EffectGradient,
	"fire":     EffectFire,
	"candle":   EffectCandle,
	"wave":     EffectWave,
	"chase":    EffectWave, // alias
	"rainbow":  EffectRainbow,
}

// PatternSpec is the simplified JSON input format for the LLM
type PatternSpec struct {
	Effect     string   `json:"effect"`               // Required: solid, pulse, sparkle, gradient, wave, fire, rainbow
	Colors     []string `json:"colors"`               // Required: array of hex colors like "#FF0000"
	Brightness int      `json:"brightness,omitempty"` // Optional: 0-255, default 200
	Speed      int      `json:"speed,omitempty"`      // Optional: 0-255, default 128
	Density    int      `json:"density,omitempty"`    // Optional: 0-255, for sparkle
	Cooling    int      `json:"cooling,omitempty"`    // Optional: 0-255, for fire
	Sparking   int      `json:"sparking,omitempty"`   // Optional: 0-255, for fire
	WaveCount  int      `json:"wave_count,omitempty"` // Optional: 1-10, for wave
	Direction  int      `json:"direction,omitempty"`  // Optional: 0=forward, 1=reverse
}

// CompileLCLv3 compiles a PatternSpec to fixed-format bytecode
func CompileLCLv3(spec *PatternSpec) ([]byte, error) {
	// Validate effect type
	effectID, ok := effectTypes[strings.ToLower(spec.Effect)]
	if !ok {
		return nil, fmt.Errorf("unknown effect type: %s", spec.Effect)
	}

	// Validate and parse colors
	if len(spec.Colors) == 0 {
		return nil, fmt.Errorf("at least one color is required")
	}
	if len(spec.Colors) > MaxPaletteColors {
		spec.Colors = spec.Colors[:MaxPaletteColors]
	}

	colors := make([][3]byte, 0, len(spec.Colors))
	for _, colorStr := range spec.Colors {
		r, g, b, err := parseHexColor(colorStr)
		if err != nil {
			return nil, fmt.Errorf("invalid color %s: %v", colorStr, err)
		}
		colors = append(colors, [3]byte{r, g, b})
	}

	// Apply defaults
	brightness := spec.Brightness
	if brightness <= 0 {
		brightness = 200
	}
	if brightness > 255 {
		brightness = 255
	}

	speed := spec.Speed
	if speed <= 0 {
		speed = 128
	}
	if speed > 255 {
		speed = 255
	}

	// Calculate effect-specific params
	param1, param2 := getEffectParams(effectID, spec)

	// Build bytecode
	paletteSize := 1 + len(colors)*3
	totalLen := LCLHeaderSize + LCLCoreParamsSize + LCLColorSize + paletteSize
	bytecode := make([]byte, totalLen)

	// Header
	copy(bytecode[OffsetMagic:], LCLMagic)
	bytecode[OffsetVersion] = LCLVersion
	bytecode[OffsetLength] = byte((totalLen - LCLHeaderSize) >> 8)
	bytecode[OffsetLength+1] = byte(totalLen - LCLHeaderSize)
	bytecode[OffsetFlags] = 0

	// Core parameters
	bytecode[OffsetEffect] = effectID
	bytecode[OffsetBrightness] = byte(brightness)
	bytecode[OffsetSpeed] = byte(speed)
	bytecode[OffsetParam1] = param1
	bytecode[OffsetParam2] = param2
	bytecode[OffsetParam3] = 0
	bytecode[OffsetDirection] = byte(spec.Direction & 0x01)
	bytecode[OffsetReserved] = 0

	// Primary color (first color in palette)
	bytecode[OffsetColorR] = colors[0][0]
	bytecode[OffsetColorG] = colors[0][1]
	bytecode[OffsetColorB] = colors[0][2]

	// Palette
	bytecode[OffsetColorCount] = byte(len(colors))
	for i, color := range colors {
		offset := OffsetPalette + i*3
		bytecode[offset] = color[0]
		bytecode[offset+1] = color[1]
		bytecode[offset+2] = color[2]
	}

	// Calculate checksum (XOR of all bytes after header)
	checksum := byte(0)
	for i := LCLHeaderSize; i < len(bytecode); i++ {
		checksum ^= bytecode[i]
	}
	bytecode[OffsetChecksum] = checksum

	return bytecode, nil
}

// getEffectParams returns param1 and param2 based on effect type
func getEffectParams(effectID byte, spec *PatternSpec) (byte, byte) {
	switch effectID {
	case EffectSparkle:
		density := spec.Density
		if density <= 0 {
			density = 128 // default medium density
		}
		return byte(density), 0

	case EffectPulse:
		// Param1 = rhythm (higher = slower pulse)
		rhythm := 128 - spec.Speed/2 // inverse of speed
		if rhythm < 10 {
			rhythm = 10
		}
		return byte(rhythm), 0

	case EffectFire, EffectCandle:
		cooling := spec.Cooling
		if cooling <= 0 {
			cooling = 55 // default cooling
		}
		sparking := spec.Sparking
		if sparking <= 0 {
			sparking = 120 // default sparking
		}
		return byte(cooling), byte(sparking)

	case EffectWave:
		waveCount := spec.WaveCount
		if waveCount <= 0 {
			waveCount = 3 // default wave count
		}
		if waveCount > 10 {
			waveCount = 10
		}
		return byte(waveCount), 0

	default:
		return 0, 0
	}
}

// parseHexColor parses a hex color string like "#FF0000" or "FF0000"
func parseHexColor(s string) (byte, byte, byte, error) {
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimSpace(s)

	if len(s) == 3 {
		// Short form: "F00" -> "FF0000"
		s = string(s[0]) + string(s[0]) + string(s[1]) + string(s[1]) + string(s[2]) + string(s[2])
	}

	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color length")
	}

	r, err := strconv.ParseUint(s[0:2], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	g, err := strconv.ParseUint(s[2:4], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	b, err := strconv.ParseUint(s[4:6], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	return byte(r), byte(g), byte(b), nil
}

// Named color map for convenience
var namedColors = map[string]string{
	"red":        "#FF0000",
	"green":      "#00FF00",
	"blue":       "#0000FF",
	"yellow":     "#FFFF00",
	"orange":     "#FFA500",
	"purple":     "#800080",
	"magenta":    "#FF00FF",
	"cyan":       "#00FFFF",
	"white":      "#FFFFFF",
	"black":      "#000000",
	"pink":       "#FFC0CB",
	"warm_white": "#FFF4E5",
	"cool_white": "#F4FFFA",
}

// resolveColor converts named colors to hex
func resolveColor(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if hex, ok := namedColors[s]; ok {
		return hex
	}
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	return s
}

// ParsePatternJSON parses JSON pattern specification
func ParsePatternJSON(jsonStr string) (*PatternSpec, error) {
	var spec PatternSpec
	if err := json.Unmarshal([]byte(jsonStr), &spec); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}

	// Resolve named colors to hex
	for i, c := range spec.Colors {
		spec.Colors[i] = resolveColor(c)
	}

	return &spec, nil
}

// ValidatePatternSpec validates a pattern specification
func ValidatePatternSpec(spec *PatternSpec) []string {
	var errors []string

	// Check effect type
	if _, ok := effectTypes[strings.ToLower(spec.Effect)]; !ok {
		errors = append(errors, fmt.Sprintf("invalid effect '%s'. Valid: solid, pulse, sparkle, gradient, wave, fire, rainbow", spec.Effect))
	}

	// Check colors
	if len(spec.Colors) == 0 {
		errors = append(errors, "at least one color is required")
	}
	for _, c := range spec.Colors {
		resolved := resolveColor(c)
		if _, _, _, err := parseHexColor(resolved); err != nil {
			errors = append(errors, fmt.Sprintf("invalid color '%s'", c))
		}
	}

	// Check numeric ranges
	if spec.Brightness < 0 || spec.Brightness > 255 {
		errors = append(errors, "brightness must be 0-255")
	}
	if spec.Speed < 0 || spec.Speed > 255 {
		errors = append(errors, "speed must be 0-255")
	}

	return errors
}

// =============================================================================
// LEGACY SUPPORT: Parse old YAML-like LCL format and convert to PatternSpec
// =============================================================================

// ParseLegacyLCL parses the old YAML-like format and converts to PatternSpec
func ParseLegacyLCL(lclText string) (*PatternSpec, error) {
	spec := &PatternSpec{
		Brightness: 200,
		Speed:      128,
	}

	lines := strings.Split(lclText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip section headers (behavior:, appearance:, timing:)
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
			continue
		}

		// Parse key: value pairs
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		switch key {
		case "effect":
			spec.Effect = value
		case "colors", "color":
			// Parse color list
			spec.Colors = parseColorList(value)
		case "brightness":
			spec.Brightness = parseBrightnessValue(value)
		case "speed":
			spec.Speed = parseSpeedValue(value)
		case "density":
			spec.Density = parseDensityValue(value)
		case "wave_count":
			spec.WaveCount = parseWaveCountValue(value)
		case "flame_height":
			spec.Cooling = parseFlameHeightValue(value)
		case "spark_frequency":
			spec.Sparking = parseSparkFrequencyValue(value)
		case "direction":
			if value == "reverse" || value == "backward" {
				spec.Direction = 1
			}
		}
	}

	if spec.Effect == "" {
		return nil, fmt.Errorf("effect type is required")
	}
	if len(spec.Colors) == 0 {
		spec.Colors = []string{"#FFFFFF"} // default white
	}

	return spec, nil
}

// parseColorList parses comma-separated color values
func parseColorList(s string) []string {
	s = strings.Trim(s, "[]")
	parts := strings.Split(s, ",")
	colors := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"'")
		if p != "" {
			colors = append(colors, resolveColor(p))
		}
	}
	return colors
}

// Semantic value parsers
func parseBrightnessValue(s string) int {
	switch strings.ToLower(s) {
	case "dim":
		return 64
	case "medium":
		return 128
	case "bright":
		return 200
	case "full":
		return 255
	default:
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
		return 200
	}
}

func parseSpeedValue(s string) int {
	switch strings.ToLower(s) {
	case "frozen":
		return 0
	case "glacial":
		return 20
	case "very_slow":
		return 40
	case "slow":
		return 70
	case "medium":
		return 128
	case "fast":
		return 180
	case "very_fast":
		return 220
	case "frantic":
		return 255
	default:
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
		return 128
	}
}

func parseDensityValue(s string) int {
	switch strings.ToLower(s) {
	case "sparse":
		return 30
	case "light":
		return 60
	case "medium":
		return 128
	case "dense":
		return 200
	case "packed":
		return 255
	default:
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
		return 128
	}
}

func parseWaveCountValue(s string) int {
	switch strings.ToLower(s) {
	case "one":
		return 1
	case "few":
		return 2
	case "several":
		return 4
	case "many":
		return 8
	default:
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
		return 3
	}
}

func parseFlameHeightValue(s string) int {
	// Cooling: higher = shorter flames
	switch strings.ToLower(s) {
	case "very_tall":
		return 30
	case "tall":
		return 45
	case "medium":
		return 55
	case "short":
		return 80
	case "very_short":
		return 120
	default:
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
		return 55
	}
}

func parseSparkFrequencyValue(s string) int {
	switch strings.ToLower(s) {
	case "rare":
		return 50
	case "occasional":
		return 80
	case "frequent":
		return 120
	case "high":
		return 170
	case "intense":
		return 220
	default:
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
		return 120
	}
}

// =============================================================================
// MAIN COMPILE FUNCTION - handles both JSON and legacy YAML formats
// =============================================================================

// CompileLCL is the main entry point - detects format and compiles to bytecode
func CompileLCL(input string) ([]byte, []string, error) {
	input = strings.TrimSpace(input)
	var spec *PatternSpec
	var err error
	var warnings []string

	// Try JSON first
	if strings.HasPrefix(input, "{") {
		spec, err = ParsePatternJSON(input)
		if err != nil {
			return nil, nil, fmt.Errorf("JSON parse error: %v", err)
		}
	} else {
		// Try legacy YAML-like format
		spec, err = ParseLegacyLCL(input)
		if err != nil {
			return nil, nil, fmt.Errorf("LCL parse error: %v", err)
		}
		warnings = append(warnings, "Using legacy LCL format - consider switching to JSON")
	}

	// Validate
	validationErrors := ValidatePatternSpec(spec)
	if len(validationErrors) > 0 {
		return nil, nil, fmt.Errorf("validation errors: %s", strings.Join(validationErrors, "; "))
	}

	// Compile to bytecode
	bytecode, err := CompileLCLv3(spec)
	if err != nil {
		return nil, nil, err
	}

	return bytecode, warnings, nil
}

// ValidateLCL validates LCL input without compiling
func ValidateLCL(input string) (bool, []string) {
	input = strings.TrimSpace(input)
	var spec *PatternSpec
	var err error

	if strings.HasPrefix(input, "{") {
		spec, err = ParsePatternJSON(input)
	} else {
		spec, err = ParseLegacyLCL(input)
	}

	if err != nil {
		return false, []string{err.Error()}
	}

	errors := ValidatePatternSpec(spec)
	return len(errors) == 0, errors
}
