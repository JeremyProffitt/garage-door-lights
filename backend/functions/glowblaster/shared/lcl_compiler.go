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

// ... (PatternSpec struct and CompileLCLv4 function remain unchanged)

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

// ... (parseHexColor, ParseIntentYAML, and mapping functions remain unchanged)

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

// REMOVED: ExtractDescriptionFromLCL (duplicate)
