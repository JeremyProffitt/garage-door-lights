package shared

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// LCL Bytecode Format Version 4 - Expanded Fixed Format
// This is an expanded format that uses fixed byte positions instead of opcodes.
//
// HEADER (8 bytes):
//   [0-2]   Magic "LCL"
//   [3]     Version = 0x04
//   [4-5]   Total length (16-bit big-endian)
//   [6]     Checksum (XOR of bytes 8+)
//   [7]     Flags
//
// CORE PARAMETERS (16 bytes):
//   [8]     Effect type
//   [9]     Brightness (0-255)
//   [10]    Speed (0-255)
//   [11]    Param1 (density, rhythm, cooling, wave_count)
//   [12]    Param2 (sparking, width, eye_size)
//   [13]    Param3 (tail_length, fade_rate)
//   [14]    Param4 (direction, style)
//   [15]    ColorMode (0=Palette, 1=Dual, 2=Texture)
//   [16-23] Reserved / Secondary Color indices? -> Let's keep reserved for now
//
// COLORS (Starts at 24):
//   [24-26] Primary Color (R, G, B)
//   [27-29] Secondary Color (R, G, B)
//   [30]    Palette Count (0-8)
//   [31+]   Palette Colors (R, G, B each)

const (
	LCLMagic          = "LCL"
	LCLVersion        = 0x04 // Version 4: Expanded Fixed format
	LCLHeaderSize     = 8
	LCLCoreParamsSize = 16
	LCLColorBlockSize = 7 // Primary(3) + Secondary(3) + Count(1)
	MaxPaletteColors  = 8
)

// Byte offsets in the fixed-format bytecode
const (
	OffsetMagic           = 0
	OffsetVersion         = 3
	OffsetLength          = 4
	OffsetChecksum        = 6
	OffsetFlags           = 7
	OffsetEffect          = 8
	OffsetBrightness      = 9
	OffsetSpeed           = 10
	OffsetParam1          = 11
	OffsetParam2          = 12
	OffsetParam3          = 13
	OffsetParam4          = 14
	OffsetColorMode       = 15
	OffsetReservedStart   = 16
	OffsetPrimaryColor    = 24
	OffsetSecondaryColor  = 27
	OffsetColorCount      = 30
	OffsetPalette         = 31
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
	EffectScanner  byte = 0x0B // New Scanner Effect (Larson Scanner)
	EffectWipe     byte = 0x0C // New Wipe Effect
	// EffectChase    byte = 0x09 // REMOVED: Duplicate of Wave
	// EffectBreathe  byte = 0x02 // REMOVED: Duplicate of Pulse
)

// Effect type name to ID mapping
var effectTypes = map[string]byte{
	"solid":    EffectSolid,
	"pulse":    EffectPulse,
	"breathe":  EffectPulse, // Map string "breathe" to Pulse ID
	"sparkle":  EffectSparkle,
	"gradient": EffectGradient,
	"fire":     EffectFire,
	"candle":   EffectCandle,
	"wave":     EffectWave,
	"chase":    EffectWave,    // Map string "chase" to Wave ID
	"scanner":  EffectScanner, // Knight Rider!
	"wipe":     EffectWipe,
	"rainbow":  EffectRainbow,
}

// PatternSpec is the Specification Layer (Intermediate Representation)
// This maps directly to the compiled bytecode parameters
type PatternSpec struct {
	Effect          string   `json:"effect"`               // Required
	Colors          []string `json:"colors"`               // Required: array of hex colors
	BackgroundColor string   `json:"background_color"`     // Optional: secondary color
	Brightness      int      `json:"brightness,omitempty"` // 0-255
	Speed           int      `json:"speed,omitempty"`      // 0-255
	
	// Param1
	Density    int      `json:"density,omitempty"`    // 0-255 (Sparkle)
	Cooling    int      `json:"cooling,omitempty"`    // 0-255 (Fire: Flame Height)
	WaveCount  int      `json:"wave_count,omitempty"` // 1-10 (Wave)
	Rhythm     int      `json:"rhythm,omitempty"`     // 0-255 (Pulse)

	// Param2
	Sparking   int      `json:"sparking,omitempty"`   // 0-255 (Fire: Spark Frequency)
	EyeSize    int      `json:"eye_size,omitempty"`   // 1-10 (Scanner)

	// Param3
	TailLength int      `json:"tail_length,omitempty"` // 0-20 (Scanner/Chase)
	
	// Param4
	Direction  int      `json:"direction,omitempty"`  // 0=forward, 1=reverse
	Style      int      `json:"style,omitempty"`      // 0=smooth, 1=bounce...
}

// CompileLCLv4 compiles a PatternSpec to fixed-format bytecode
func CompileLCLv4(spec *PatternSpec) ([]byte, error) {
	// Validate effect type
	effectID, ok := effectTypes[strings.ToLower(spec.Effect)]
	if !ok {
		return nil, fmt.Errorf("unknown effect type: %s", spec.Effect)
	}

	// Default colors if missing
	if len(spec.Colors) == 0 {
		spec.Colors = []string{"#FFFFFF"}
	}
	if len(spec.Colors) > MaxPaletteColors {
		spec.Colors = spec.Colors[:MaxPaletteColors]
	}

	// Parse primary colors
	colors := make([][3]byte, 0, len(spec.Colors))
	for _, colorStr := range spec.Colors {
		r, g, b, err := parseHexColor(colorStr)
		if err != nil {
			colors = append(colors, [3]byte{255, 255, 255})
		} else {
			colors = append(colors, [3]byte{r, g, b})
		}
	}

	// Parse secondary/background color
	secondaryColor := [3]byte{0, 0, 0}
	if spec.BackgroundColor != "" {
		r, g, b, err := parseHexColor(spec.BackgroundColor)
		if err == nil {
			secondaryColor = [3]byte{r, g, b}
		}
	} else if len(colors) > 1 {
		// If no background specified but palette has 2+ colors, use 2nd as secondary
		// secondaryColor = colors[1] // Actually, let's keep secondary explicit or default black
	}

	// Apply defaults
	brightness := spec.Brightness
	if brightness <= 0 { brightness = 200 }
	if brightness > 255 { brightness = 255 }

	speed := spec.Speed
	if speed <= 0 { speed = 128 }
	if speed > 255 { speed = 255 }

	// Calculate effect-specific params
	param1, param2, param3, param4 := getEffectParamsV4(effectID, spec)

	// Build bytecode
	paletteSize := 0
	if len(colors) > 0 {
		paletteSize = len(colors) * 3
	}
	
	// Total length: Header(8) + Core(16) + Primary(3) + Secondary(3) + Count(1) + Palette(N*3)
	totalLen := LCLHeaderSize + LCLCoreParamsSize + LCLColorBlockSize + paletteSize
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
	bytecode[OffsetParam3] = param3
	bytecode[OffsetParam4] = param4
	bytecode[OffsetColorMode] = 0 // Default palette mode for now
	
	// Direction override if Param4 didn't handle it
	// Actually direction is mapped to Param4 usually, or handled here?
	// Let's OR it in if Param4 is used for style, or use Param4 purely for direction/style combo
	// For simplicity, Param4 IS direction/style
	
	// Primary Color
	if len(colors) > 0 {
		bytecode[OffsetPrimaryColor] = colors[0][0]
		bytecode[OffsetPrimaryColor+1] = colors[0][1]
		bytecode[OffsetPrimaryColor+2] = colors[0][2]
	}

	// Secondary Color
	bytecode[OffsetSecondaryColor] = secondaryColor[0]
	bytecode[OffsetSecondaryColor+1] = secondaryColor[1]
	bytecode[OffsetSecondaryColor+2] = secondaryColor[2]

	// Palette
	bytecode[OffsetColorCount] = byte(len(colors))
	for i, color := range colors {
		offset := OffsetPalette + i*3
		bytecode[offset] = color[0]
		bytecode[offset+1] = color[1]
		bytecode[offset+2] = color[2]
	}

	// Calculate checksum
	checksum := byte(0)
	for i := LCLHeaderSize; i < len(bytecode); i++ {
		checksum ^= bytecode[i]
	}
	bytecode[OffsetChecksum] = checksum

	return bytecode, nil
}

// getEffectParamsV4 returns param1, param2, param3, param4 based on effect type
func getEffectParamsV4(effectID byte, spec *PatternSpec) (byte, byte, byte, byte) {
	p1, p2, p3, p4 := byte(0), byte(0), byte(0), byte(0)

	// Common Direction mapping for P4
	p4 = byte(spec.Direction & 0x01) // Default

	switch effectID {
	case EffectSparkle:
		if spec.Density <= 0 { spec.Density = 128 }
		p1 = byte(spec.Density)

	case EffectPulse: // Removed EffectBreathe case
		rhythm := 255 - spec.Speed
		if spec.Rhythm > 0 { rhythm = spec.Rhythm } // Allow override
		if rhythm < 10 { rhythm = 10 }
		p1 = byte(rhythm)

	case EffectFire, EffectCandle:
		if spec.Cooling <= 0 { spec.Cooling = 55 }
		if spec.Sparking <= 0 { spec.Sparking = 120 }
		p1 = byte(spec.Cooling)
		p2 = byte(spec.Sparking)

	case EffectWave: // Removed EffectChase case
		if spec.WaveCount <= 0 { spec.WaveCount = 3 }
		if spec.WaveCount > 10 { spec.WaveCount = 10 }
		p1 = byte(spec.WaveCount)
		// Chase could use P2/P3 for head/tail
		p2 = byte(spec.EyeSize)
		p3 = byte(spec.TailLength)
	
	case EffectScanner: // Knight Rider
		// P1: Reserved? Maybe speed modifier?
		// P2: Eye Size (Width)
		// P3: Tail Length (Fade)
		// P4: Direction/Bounce
		if spec.EyeSize <= 0 { spec.EyeSize = 2 }
		if spec.TailLength <= 0 { spec.TailLength = 4 }
		p2 = byte(spec.EyeSize)
		p3 = byte(spec.TailLength)
		// Direction handled by default p4
	}

	return p1, p2, p3, p4
}

// parseHexColor parses a hex color string
func parseHexColor(s string) (byte, byte, byte, error) {
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimSpace(s)

	if len(s) == 3 {
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

// =============================================================================
// INTENT LAYER PARSER (YAML -> PatternSpec)
// =============================================================================

// ParseIntentYAML parses the LCL v2 YAML format and maps it to a PatternSpec
func ParseIntentYAML(yamlStr string) (*PatternSpec, error) {
	spec := &PatternSpec{
		Brightness: 200,
		Speed:      128,
		Direction:  0,
	}

	lines := strings.Split(yamlStr, "\n")
	currentSection := ""

	for _, line := range lines {
		// Remove comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}
		
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check for section headers (no indentation, ends with :)
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, " ") {
			currentSection = strings.TrimSuffix(trimmed, ":")
			continue
		}

		// Parse Key-Value pairs
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "'\"")

		// Route based on section
		switch currentSection {
		case "", "root":
			if key == "effect" {
				spec.Effect = value
			}
		case "behavior":
			mapBehavior(key, value, spec)
		case "appearance":
			mapAppearance(key, value, spec)
		case "timing":
			mapTiming(key, value, spec)
		case "spatial":
			mapSpatial(key, value, spec)
		}
	}

	if spec.Effect == "" {
		return nil, fmt.Errorf("effect is required in LCL")
	}

	return spec, nil
}

// Semantic Mappings

func mapBehavior(key, value string, spec *PatternSpec) {
	switch key {
	// Fire
	case "flame_height":
		switch value {
		case "very_short", "tiny": spec.Cooling = 120
		case "short", "small": spec.Cooling = 80
		case "medium": spec.Cooling = 55
		case "tall", "large", "high": spec.Cooling = 40
		case "very_tall", "huge": spec.Cooling = 20
		default: spec.Cooling = 55
		}
	case "spark_frequency":
		switch value {
		case "rare", "few": spec.Sparking = 50
		case "occasional", "some": spec.Sparking = 80
		case "frequent": spec.Sparking = 120
		case "high", "many": spec.Sparking = 170
		case "intense", "lots": spec.Sparking = 220
		default: spec.Sparking = 120
		}
	
	// Sparkle
	case "density":
		switch value {
		case "sparse": spec.Density = 30
		case "light": spec.Density = 60
		case "medium": spec.Density = 128
		case "dense": spec.Density = 200
		case "packed": spec.Density = 255
		default: spec.Density = 128
		}

	// Wave
	case "wave_count":
		switch value {
		case "one": spec.WaveCount = 1
		case "few": spec.WaveCount = 2
		case "several": spec.WaveCount = 4
		case "many": spec.WaveCount = 8
		default: spec.WaveCount = 3
		}

	// Breathe
	case "rhythm":
		switch value {
		case "calm": spec.Speed = 70
		case "relaxed": spec.Speed = 100
		case "steady": spec.Speed = 128
		case "energetic": spec.Speed = 180
		case "frantic": spec.Speed = 220
		default: spec.Speed = 128
		}
	
	// Scanner / Chase (New in v4)
	case "eye_size", "head_size":
		switch value {
		case "tiny": spec.EyeSize = 1
		case "small": spec.EyeSize = 2
		case "medium": spec.EyeSize = 3
		case "large": spec.EyeSize = 5
		case "huge": spec.EyeSize = 8
		default: 
			if v, err := strconv.Atoi(value); err == nil {
				spec.EyeSize = v
			} else {
				spec.EyeSize = 3
			}
		}

	case "tail_length":
		switch value {
		case "none": spec.TailLength = 0
		case "short": spec.TailLength = 2
		case "medium": spec.TailLength = 4
		case "long": spec.TailLength = 8
		case "ghost": spec.TailLength = 16
		default:
			if v, err := strconv.Atoi(value); err == nil {
				spec.TailLength = v
			} else {
				spec.TailLength = 4
			}
		}
	}
}

func mapAppearance(key, value string, spec *PatternSpec) {
	switch key {
	case "color", "colors":
		spec.Colors = []string{resolveColor(value)}
	case "color_scheme":
		spec.Colors = resolveColorScheme(value)
	case "brightness":
		switch value {
		case "dim", "low": spec.Brightness = 64
		case "medium": spec.Brightness = 128
		case "bright", "high": spec.Brightness = 200
		case "full", "max": spec.Brightness = 255
		default: 
			if v, err := strconv.Atoi(value); err == nil {
				spec.Brightness = v
			}
		}
	case "background", "background_color":
		spec.BackgroundColor = resolveColor(value)
	}
}

func mapTiming(key, value string, spec *PatternSpec) {
	if key == "speed" {
		switch value {
		case "frozen": spec.Speed = 0
		case "glacial": spec.Speed = 20
		case "very_slow": spec.Speed = 40
		case "slow": spec.Speed = 70
		case "medium": spec.Speed = 128
		case "fast": spec.Speed = 180
		case "very_fast": spec.Speed = 220
		case "frantic": spec.Speed = 255
		default:
			if v, err := strconv.Atoi(value); err == nil {
				spec.Speed = v
			}
		}
	}
}

func mapSpatial(key, value string, spec *PatternSpec) {
	if key == "direction" {
		switch value {
		case "forward", "up", "right", "clockwise", "outward":
			spec.Direction = 0
		case "backward", "reverse", "down", "left", "counterclockwise", "inward":
			spec.Direction = 1
		}
	}
}

// Helper: Resolve Named Color Schemes
func resolveColorScheme(scheme string) []string {
	switch scheme {
	case "rainbow":
		return []string{"#FF0000", "#FFA500", "#FFFF00", "#00FF00", "#00FFFF", "#0000FF", "#800080"}
	case "sunset":
		return []string{"#FFA500", "#FFC0CB", "#800080", "#00008B"}
	case "ocean":
		return []string{"#00008B", "#0000FF", "#00FFFF", "#008080"}
	case "forest":
		return []string{"#006400", "#008000", "#32CD32", "#FFFF00"}
	case "fire", "classic_fire":
		return []string{"#000000", "#FF0000", "#FFA500", "#FFFF00", "#FFFFFF"}
	case "ice":
		return []string{"#FFFFFF", "#00FFFF", "#0000FF", "#00008B"}
	case "party":
		return []string{"#FF00FF", "#00FFFF", "#FFFF00", "#FF00FF"}
	case "warm_orange":
		return []string{"#8B4500", "#D2691E", "#FFA500", "#FFD700"}
	case "blue_gas":
		return []string{"#000000", "#00008B", "#0000FF", "#00FFFF", "#FFFFFF"}
	case "knight_rider", "scanner_red":
		return []string{"#FF0000"} // Basic Red
	default:
		return []string{"#FFFFFF"}
	}
}

// Helper: Resolve Color Name
func resolveColor(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	// Basic map
	colors := map[string]string{
		"red": "#FF0000", "green": "#00FF00", "blue": "#0000FF",
		"yellow": "#FFFF00", "orange": "#FFA500", "purple": "#800080",
		"cyan": "#00FFFF", "magenta": "#FF00FF", "pink": "#FFC0CB",
		"white": "#FFFFFF", "black": "#000000", "warm_white": "#FFF4E5",
		"cool_white": "#F4FFFA", "gold": "#FFD700", "teal": "#008080",
		"crimson": "#DC143C", "coral": "#FF7F50", "navy": "#000080",
	}
	if hex, ok := colors[s]; ok {
		return hex
	}
	if !strings.HasPrefix(s, "#") {
		// Assume hex without hash if valid length
		if len(s) == 3 || len(s) == 6 {
			return "#" + s
		}
	}
	return s
}

// =============================================================================
// MAIN ENTRY POINT
// =============================================================================

// CompileLCL compiles LCL text (YAML or JSON) to bytecode
func CompileLCL(input string) ([]byte, []string, error) {
	input = strings.TrimSpace(input)
	var spec *PatternSpec
	var err error
	var warnings []string

	// Detect format
	if strings.HasPrefix(input, "{") {
		// JSON (Legacy or Specification Layer)
		if err := json.Unmarshal([]byte(input), &spec); err != nil {
			return nil, nil, fmt.Errorf("JSON parse error: %v", err)
		}
	} else {
		// YAML (Intent Layer)
		spec, err = ParseIntentYAML(input)
		if err != nil {
			return nil, nil, fmt.Errorf("LCL parse error: %v", err)
		}
	}

	// Validate
	if spec.Effect == "" {
		return nil, nil, fmt.Errorf("effect is required")
	}

	// Compile - USE V4
	bytecode, err := CompileLCLv4(spec)
	if err != nil {
		return nil, nil, err
	}

	return bytecode, warnings, nil
}

// ValidateLCL validates without compiling
func ValidateLCL(input string) (bool, []string) {
	_, _, err := CompileLCL(input)
	if err != nil {
		return false, []string{err.Error()}
	}
	return true, nil
}

// ExtractDescriptionFromLCL extracts description (name) from LCL
func ExtractDescriptionFromLCL(lclText string) string {
	lines := strings.Split(lclText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			}
		}
	}
	return ""
}