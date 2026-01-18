package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// CompileWLEDToBinary converts a WLEDState to compact WLEDb binary format
func CompileWLEDToBinary(state *WLEDState) ([]byte, error) {
	if state == nil {
		return nil, errors.New("state is nil")
	}

	// Validate segment count
	if len(state.Segments) == 0 {
		return nil, errors.New("at least one segment is required")
	}
	if len(state.Segments) > WLEDBMaxSegments {
		return nil, fmt.Errorf("too many segments: %d (max %d)", len(state.Segments), WLEDBMaxSegments)
	}

	// Calculate total size
	// Header (8) + Global (4) + Segments (variable based on color count)
	totalSize := WLEDBHeaderSize + WLEDBGlobalSize
	for _, seg := range state.Segments {
		colorCount := len(seg.Colors)
		if colorCount == 0 {
			colorCount = 1 // Minimum 1 color
		}
		if colorCount > WLEDBMaxColors {
			colorCount = WLEDBMaxColors
		}
		// Segment base (14 bytes) + colors (3 bytes each) + checksum (1 byte)
		segSize := 14 + (colorCount * 3) + 1
		totalSize += segSize
	}

	bytecode := make([]byte, totalSize)

	// Write header
	copy(bytecode[WLEDBOffsetMagic:], WLEDBMagic)
	bytecode[WLEDBOffsetVersion] = WLEDBVersion

	// Flags: bit0 = power on
	flags := byte(0)
	if state.On {
		flags |= 0x01
	}
	bytecode[WLEDBOffsetFlags] = flags

	// Length (excluding header, big-endian)
	payloadLen := totalSize - WLEDBHeaderSize
	bytecode[WLEDBOffsetLength] = byte(payloadLen >> 8)
	bytecode[WLEDBOffsetLength+1] = byte(payloadLen)

	// Write global state
	brightness := state.Brightness
	if brightness <= 0 {
		brightness = 200
	}
	if brightness > 255 {
		brightness = 255
	}
	bytecode[WLEDBOffsetBrightness] = byte(brightness)

	// Transition (big-endian)
	bytecode[WLEDBOffsetTransition] = byte(state.Transition >> 8)
	bytecode[WLEDBOffsetTransition+1] = byte(state.Transition)

	// Segment count
	bytecode[WLEDBOffsetSegmentCount] = byte(len(state.Segments))

	// Write segments
	offset := WLEDBOffsetSegmentsStart
	for i, seg := range state.Segments {
		segStart := offset

		// Segment ID
		segID := seg.ID
		if segID == 0 && i > 0 {
			segID = i
		}
		bytecode[offset+WLEDBSegOffsetID] = byte(segID)

		// Start/Stop (big-endian)
		bytecode[offset+WLEDBSegOffsetStart] = byte(seg.Start >> 8)
		bytecode[offset+WLEDBSegOffsetStart+1] = byte(seg.Start)
		bytecode[offset+WLEDBSegOffsetStop] = byte(seg.Stop >> 8)
		bytecode[offset+WLEDBSegOffsetStop+1] = byte(seg.Stop)

		// Effect parameters
		bytecode[offset+WLEDBSegOffsetEffectID] = byte(seg.EffectID)
		bytecode[offset+WLEDBSegOffsetSpeed] = byte(clampByte(seg.Speed))
		bytecode[offset+WLEDBSegOffsetIntensity] = byte(clampByte(seg.Intensity))
		bytecode[offset+WLEDBSegOffsetCustom1] = byte(clampByte(seg.Custom1))
		bytecode[offset+WLEDBSegOffsetCustom2] = byte(clampByte(seg.Custom2))
		bytecode[offset+WLEDBSegOffsetCustom3] = byte(clampByte(seg.Custom3))
		bytecode[offset+WLEDBSegOffsetPaletteID] = byte(seg.PaletteID)

		// Flags
		segFlags := WLEDSegmentFlags{
			Reverse: seg.Reverse,
			Mirror:  seg.Mirror,
			On:      seg.On,
		}
		bytecode[offset+WLEDBSegOffsetFlags] = segFlags.ToByte()

		// Colors
		colorCount := len(seg.Colors)
		if colorCount == 0 {
			colorCount = 1
			// Default white if no colors
			seg.Colors = [][]int{{255, 255, 255}}
		}
		if colorCount > WLEDBMaxColors {
			colorCount = WLEDBMaxColors
		}
		bytecode[offset+WLEDBSegOffsetColorCnt] = byte(colorCount)

		// Write colors
		colorOffset := offset + WLEDBSegOffsetColor1
		for c := 0; c < colorCount; c++ {
			if c < len(seg.Colors) && len(seg.Colors[c]) >= 3 {
				bytecode[colorOffset] = byte(clampByte(seg.Colors[c][0]))
				bytecode[colorOffset+1] = byte(clampByte(seg.Colors[c][1]))
				bytecode[colorOffset+2] = byte(clampByte(seg.Colors[c][2]))
			}
			colorOffset += 3
		}

		// Calculate checksum (XOR of segment bytes)
		checksumOffset := offset + WLEDBSegOffsetColor1 + (colorCount * 3)
		checksum := byte(0)
		for j := segStart; j < checksumOffset; j++ {
			checksum ^= bytecode[j]
		}
		bytecode[checksumOffset] = checksum

		// Move to next segment
		offset = checksumOffset + 1
	}

	return bytecode, nil
}

// ParseBinaryToWLED converts WLEDb binary format back to WLEDState
func ParseBinaryToWLED(binary []byte) (*WLEDState, error) {
	if len(binary) < WLEDBHeaderSize+WLEDBGlobalSize {
		return nil, errors.New("binary too short")
	}

	// Verify magic
	if string(binary[WLEDBOffsetMagic:WLEDBOffsetMagic+4]) != WLEDBMagic {
		return nil, errors.New("invalid magic bytes")
	}

	// Check version
	if binary[WLEDBOffsetVersion] != WLEDBVersion {
		return nil, fmt.Errorf("unsupported version: %d", binary[WLEDBOffsetVersion])
	}

	state := &WLEDState{}

	// Parse flags
	state.On = (binary[WLEDBOffsetFlags] & 0x01) != 0

	// Parse global state
	state.Brightness = int(binary[WLEDBOffsetBrightness])
	state.Transition = int(binary[WLEDBOffsetTransition])<<8 | int(binary[WLEDBOffsetTransition+1])

	segmentCount := int(binary[WLEDBOffsetSegmentCount])
	if segmentCount > WLEDBMaxSegments {
		return nil, fmt.Errorf("too many segments: %d", segmentCount)
	}

	// Parse segments
	offset := WLEDBOffsetSegmentsStart
	state.Segments = make([]WLEDSegment, 0, segmentCount)

	for i := 0; i < segmentCount; i++ {
		if offset+WLEDBSegOffsetColorCnt >= len(binary) {
			return nil, errors.New("binary truncated in segment header")
		}

		seg := WLEDSegment{
			ID:        int(binary[offset+WLEDBSegOffsetID]),
			Start:     int(binary[offset+WLEDBSegOffsetStart])<<8 | int(binary[offset+WLEDBSegOffsetStart+1]),
			Stop:      int(binary[offset+WLEDBSegOffsetStop])<<8 | int(binary[offset+WLEDBSegOffsetStop+1]),
			EffectID:  int(binary[offset+WLEDBSegOffsetEffectID]),
			Speed:     int(binary[offset+WLEDBSegOffsetSpeed]),
			Intensity: int(binary[offset+WLEDBSegOffsetIntensity]),
			Custom1:   int(binary[offset+WLEDBSegOffsetCustom1]),
			Custom2:   int(binary[offset+WLEDBSegOffsetCustom2]),
			Custom3:   int(binary[offset+WLEDBSegOffsetCustom3]),
			PaletteID: int(binary[offset+WLEDBSegOffsetPaletteID]),
		}

		// Parse flags
		var flags WLEDSegmentFlags
		flags.FromByte(binary[offset+WLEDBSegOffsetFlags])
		seg.Reverse = flags.Reverse
		seg.Mirror = flags.Mirror
		seg.On = flags.On

		// Parse colors
		colorCount := int(binary[offset+WLEDBSegOffsetColorCnt])
		if colorCount > WLEDBMaxColors {
			colorCount = WLEDBMaxColors
		}

		colorOffset := offset + WLEDBSegOffsetColor1
		if colorOffset+(colorCount*3) > len(binary) {
			return nil, errors.New("binary truncated in colors")
		}

		seg.Colors = make([][]int, colorCount)
		for c := 0; c < colorCount; c++ {
			seg.Colors[c] = []int{
				int(binary[colorOffset]),
				int(binary[colorOffset+1]),
				int(binary[colorOffset+2]),
			}
			colorOffset += 3
		}

		state.Segments = append(state.Segments, seg)

		// Move past checksum to next segment
		offset = colorOffset + 1
	}

	return state, nil
}

// ValidateWLEDState validates a WLEDState and returns errors
func ValidateWLEDState(state *WLEDState) (bool, []string) {
	var errors []string

	if state == nil {
		return false, []string{"state is nil"}
	}

	// Validate brightness
	if state.Brightness < 0 || state.Brightness > 255 {
		errors = append(errors, fmt.Sprintf("brightness %d out of range (0-255)", state.Brightness))
	}

	// Validate segments
	if len(state.Segments) == 0 {
		errors = append(errors, "at least one segment is required")
	}

	if len(state.Segments) > WLEDBMaxSegments {
		errors = append(errors, fmt.Sprintf("too many segments: %d (max %d)", len(state.Segments), WLEDBMaxSegments))
	}

	for i, seg := range state.Segments {
		prefix := fmt.Sprintf("segment[%d]", i)

		// Validate LED range
		if seg.Start < 0 {
			errors = append(errors, fmt.Sprintf("%s: start %d cannot be negative", prefix, seg.Start))
		}
		if seg.Stop < 0 {
			errors = append(errors, fmt.Sprintf("%s: stop %d cannot be negative", prefix, seg.Stop))
		}
		if seg.Stop <= seg.Start {
			errors = append(errors, fmt.Sprintf("%s: stop %d must be greater than start %d", prefix, seg.Stop, seg.Start))
		}

		// Validate effect
		if !IsEffectSupported(seg.EffectID) {
			errors = append(errors, fmt.Sprintf("%s: unsupported effect ID %d", prefix, seg.EffectID))
		}

		// Validate parameters
		if seg.Speed < 0 || seg.Speed > 255 {
			errors = append(errors, fmt.Sprintf("%s: speed %d out of range (0-255)", prefix, seg.Speed))
		}
		if seg.Intensity < 0 || seg.Intensity > 255 {
			errors = append(errors, fmt.Sprintf("%s: intensity %d out of range (0-255)", prefix, seg.Intensity))
		}

		// Validate colors
		if len(seg.Colors) == 0 {
			errors = append(errors, fmt.Sprintf("%s: at least one color is required", prefix))
		}
		for j, color := range seg.Colors {
			if len(color) != 3 {
				errors = append(errors, fmt.Sprintf("%s: color[%d] must have 3 components (RGB)", prefix, j))
				continue
			}
			for k, v := range color {
				if v < 0 || v > 255 {
					errors = append(errors, fmt.Sprintf("%s: color[%d][%d] value %d out of range (0-255)", prefix, j, k, v))
				}
			}
		}
	}

	return len(errors) == 0, errors
}

// ParseWLEDJSON parses a WLED JSON string into WLEDState
func ParseWLEDJSON(jsonStr string) (*WLEDState, error) {
	var state WLEDState
	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}

	// Apply defaults for segments that are "on" but don't have it set
	for i := range state.Segments {
		if !state.Segments[i].On && state.On {
			state.Segments[i].On = true
		}
	}

	return &state, nil
}

// WLEDStateToJSON converts WLEDState to JSON string
func WLEDStateToJSON(state *WLEDState) (string, error) {
	bytes, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ExtractWLEDFromResponse extracts WLED JSON from LLM response text
// Looks for JSON in code blocks (```json ... ```) or plain JSON objects
func ExtractWLEDFromResponse(response string) string {
	// First try to find JSON in a code block
	jsonBlockPattern := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?({.*?})\\s*\\n?```")
	matches := jsonBlockPattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find a bare JSON object with "seg" key (WLED-specific)
	bareJSONPattern := regexp.MustCompile(`(?s)(\{[^{}]*"seg"\s*:\s*\[[^\]]*\][^{}]*\})`)
	matches = bareJSONPattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find any JSON object that looks like WLED state
	jsonObjPattern := regexp.MustCompile(`(?s)(\{[^{}]*"on"\s*:[^{}]*"bri"\s*:[^{}]*\})`)
	matches = jsonObjPattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// ExtractPatternName extracts pattern name from LLM response text
// Looks for "**Pattern:**" followed by the name
func ExtractPatternName(response string) string {
	// Look for **Pattern:** Name format
	patternNameRegex := regexp.MustCompile(`\*\*Pattern:\*\*\s*(.+?)(?:\n|$)`)
	matches := patternNameRegex.FindStringSubmatch(response)
	if len(matches) > 1 {
		name := strings.TrimSpace(matches[1])
		// Remove any trailing asterisks or markdown formatting
		name = strings.TrimRight(name, "*")
		name = strings.TrimSpace(name)
		return name
	}

	// Fallback: look for "Pattern:" without bold
	simplePatternRegex := regexp.MustCompile(`(?i)Pattern:\s*(.+?)(?:\n|$)`)
	matches = simplePatternRegex.FindStringSubmatch(response)
	if len(matches) > 1 {
		name := strings.TrimSpace(matches[1])
		return name
	}

	return ""
}

// IsWLEDBinary checks if the given bytes are in WLEDb format
func IsWLEDBinary(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return string(data[0:4]) == WLEDBMagic
}

// IsLCLBinary checks if the given bytes are in LCL format
func IsLCLBinary(data []byte) bool {
	if len(data) < 3 {
		return false
	}
	return string(data[0:3]) == LCLMagic
}

// DetectBinaryFormat returns the format type of binary data
func DetectBinaryFormat(data []byte) int {
	if IsWLEDBinary(data) {
		return FormatVersionWLED
	}
	if IsLCLBinary(data) {
		return FormatVersionLCL
	}
	return 0
}

// CompileWLED is the main entry point - takes JSON string, returns binary
func CompileWLED(jsonStr string) ([]byte, []string, error) {
	state, err := ParseWLEDJSON(jsonStr)
	if err != nil {
		return nil, nil, err
	}

	valid, errors := ValidateWLEDState(state)
	if !valid {
		return nil, errors, fmt.Errorf("validation failed: %v", errors)
	}

	binary, err := CompileWLEDToBinary(state)
	if err != nil {
		return nil, nil, err
	}

	return binary, nil, nil
}

// Helper function to clamp values to byte range
func clampByte(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// CreateDefaultWLEDState creates a default WLEDState with one solid white segment
func CreateDefaultWLEDState(ledCount int) *WLEDState {
	return &WLEDState{
		On:         true,
		Brightness: 200,
		Transition: 0,
		Segments: []WLEDSegment{
			{
				ID:       0,
				Start:    0,
				Stop:     ledCount,
				EffectID: WLEDFXSolid,
				Speed:    128,
				Intensity: 128,
				Colors: [][]int{
					{255, 255, 255},
				},
				On: true,
			},
		},
	}
}

// ConvertLCLToWLED attempts to convert an LCL PatternSpec to WLEDState
// This is used for migration of existing patterns
func ConvertLCLToWLED(spec *PatternSpec, ledCount int) (*WLEDState, error) {
	if spec == nil {
		return nil, errors.New("spec is nil")
	}

	// Map LCL effect to WLED effect
	wledFX, ok := LCLToWLEDEffectMap[strings.ToLower(spec.Effect)]
	if !ok {
		wledFX = WLEDFXSolid // Default to solid
	}

	// Build colors array
	colors := make([][]int, 0, 3)
	for i, colorStr := range spec.Colors {
		if i >= 3 {
			break
		}
		r, g, b, err := parseHexColor(colorStr)
		if err != nil {
			colors = append(colors, []int{255, 255, 255}) // Default white
		} else {
			colors = append(colors, []int{int(r), int(g), int(b)})
		}
	}
	if len(colors) == 0 {
		colors = append(colors, []int{255, 255, 255})
	}

	// Add background color if specified
	if spec.BackgroundColor != "" && len(colors) < 3 {
		r, g, b, err := parseHexColor(spec.BackgroundColor)
		if err == nil {
			// Background typically goes in color slot 2 or 3
			if len(colors) == 1 {
				colors = append(colors, []int{int(r), int(g), int(b)})
			}
		}
	}

	// Map LCL parameters to WLED parameters
	speed := spec.Speed
	if speed == 0 {
		speed = 128
	}

	intensity := 128 // Default
	custom1 := 0

	switch wledFX {
	case WLEDFXBreathe:
		// Pulse/Breathe: intensity = min brightness
		intensity = 0
	case WLEDFXSparkle:
		// Sparkle: intensity = density
		intensity = spec.Density
		if intensity == 0 {
			intensity = 128
		}
	case WLEDFXScanner:
		// Scanner: intensity = eye width, custom1 = tail
		intensity = spec.EyeSize * 25 // Scale 1-10 to 0-255
		if intensity == 0 {
			intensity = 50
		}
		custom1 = spec.TailLength * 16 // Scale 0-16 to 0-255
		if custom1 == 0 {
			custom1 = 64
		}
	case WLEDFXFire2012:
		// Fire: intensity = cooling, custom1 = sparking
		intensity = spec.Cooling
		if intensity == 0 {
			intensity = 55
		}
		custom1 = spec.Sparking
		if custom1 == 0 {
			custom1 = 120
		}
	case WLEDFXColorwaves:
		// Wave: intensity controls spread
		intensity = spec.WaveCount * 25
		if intensity == 0 {
			intensity = 75
		}
	}

	seg := WLEDSegment{
		ID:        0,
		Start:     0,
		Stop:      ledCount,
		EffectID:  wledFX,
		Speed:     speed,
		Intensity: intensity,
		Custom1:   custom1,
		Colors:    colors,
		On:        true,
	}

	// Handle direction
	if spec.Direction == 1 {
		seg.Reverse = true
	}

	return &WLEDState{
		On:         true,
		Brightness: spec.Brightness,
		Transition: 0,
		Segments:   []WLEDSegment{seg},
	}, nil
}
