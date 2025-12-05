package shared

import (
	"math"
)

// RGB represents an RGB color with values 0-255
type RGB struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

// HSBToRGB converts HSB (Hue, Saturation, Brightness) to RGB
// Alexa sends: hue (0-360), saturation (0-1), brightness (0-1)
// Returns: R, G, B (0-255)
func HSBToRGB(hue, saturation, brightness float64) RGB {
	// Normalize hue to 0-360 range
	hue = math.Mod(hue, 360)
	if hue < 0 {
		hue += 360
	}

	// Handle edge cases
	if saturation <= 0 {
		// No saturation = grayscale
		v := uint8(brightness * 255)
		return RGB{R: v, G: v, B: v}
	}

	if brightness <= 0 {
		return RGB{R: 0, G: 0, B: 0}
	}

	// HSB to RGB conversion
	c := brightness * saturation
	x := c * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	m := brightness - c

	var r1, g1, b1 float64

	switch {
	case hue < 60:
		r1, g1, b1 = c, x, 0
	case hue < 120:
		r1, g1, b1 = x, c, 0
	case hue < 180:
		r1, g1, b1 = 0, c, x
	case hue < 240:
		r1, g1, b1 = 0, x, c
	case hue < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}

	return RGB{
		R: uint8(math.Round((r1 + m) * 255)),
		G: uint8(math.Round((g1 + m) * 255)),
		B: uint8(math.Round((b1 + m) * 255)),
	}
}

// RGBToHSB converts RGB (0-255) to HSB
// Returns: hue (0-360), saturation (0-1), brightness (0-1)
func RGBToHSB(r, g, b uint8) (hue, saturation, brightness float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	delta := max - min

	// Brightness
	brightness = max

	// Saturation
	if max > 0 {
		saturation = delta / max
	} else {
		saturation = 0
	}

	// Hue
	if delta == 0 {
		hue = 0
	} else {
		switch max {
		case rf:
			hue = 60 * math.Mod((gf-bf)/delta, 6)
		case gf:
			hue = 60 * ((bf-rf)/delta + 2)
		case bf:
			hue = 60 * ((rf-gf)/delta + 4)
		}
	}

	if hue < 0 {
		hue += 360
	}

	return hue, saturation, brightness
}

// NamedColors maps common color names to RGB values
var NamedColors = map[string]RGB{
	"red":        {R: 255, G: 0, B: 0},
	"orange":     {R: 255, G: 165, B: 0},
	"yellow":     {R: 255, G: 255, B: 0},
	"green":      {R: 0, G: 255, B: 0},
	"cyan":       {R: 0, G: 255, B: 255},
	"blue":       {R: 0, G: 0, B: 255},
	"purple":     {R: 128, G: 0, B: 128},
	"pink":       {R: 255, G: 192, B: 203},
	"magenta":    {R: 255, G: 0, B: 255},
	"white":      {R: 255, G: 255, B: 255},
	"warm_white": {R: 255, G: 244, B: 229},
	"soft_white": {R: 255, G: 250, B: 240},
	"daylight":   {R: 255, G: 255, B: 255},
}

// BrightnessPercentToFirmware converts Alexa brightness (0-100) to firmware (0-255)
func BrightnessPercentToFirmware(percent int) int {
	if percent <= 0 {
		return 0
	}
	if percent >= 100 {
		return 255
	}
	return int(math.Round(float64(percent) * 255 / 100))
}

// BrightnessFirmwareToPercent converts firmware brightness (0-255) to Alexa (0-100)
func BrightnessFirmwareToPercent(value int) int {
	if value <= 0 {
		return 0
	}
	if value >= 255 {
		return 100
	}
	return int(math.Round(float64(value) * 100 / 255))
}

// ClampBrightness ensures brightness is within valid range
func ClampBrightness(brightness int) int {
	if brightness < 0 {
		return 0
	}
	if brightness > 100 {
		return 100
	}
	return brightness
}

// ApplyBrightnessToRGB scales RGB values by brightness factor
func ApplyBrightnessToRGB(color RGB, brightnessPercent int) RGB {
	factor := float64(brightnessPercent) / 100
	return RGB{
		R: uint8(math.Round(float64(color.R) * factor)),
		G: uint8(math.Round(float64(color.G) * factor)),
		B: uint8(math.Round(float64(color.B) * factor)),
	}
}
