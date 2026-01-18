package shared

import "strings"

// WLED Effect IDs
// These map to the WLED firmware effect IDs.
// See: https://kno.wledge.me/features/effects/
const (
	WLEDFXSolid           = 0
	WLEDFXBlink           = 1
	WLEDFXBreathe         = 2
	WLEDFXWipe            = 3
	WLEDFXWipeRandom      = 4
	WLEDFXRandomColors    = 5
	WLEDFXSweep           = 6
	WLEDFXDynamic         = 7
	WLEDFXColorloop       = 8
	WLEDFXRainbow         = 9
	WLEDFXScan            = 10
	WLEDFXScanDual        = 11
	WLEDFXFade            = 12
	WLEDFXTheater         = 13
	WLEDFXTheaterRainbow  = 14
	WLEDFXRunning         = 15
	WLEDFXSawTooth        = 16
	WLEDFXTwinkle         = 17
	WLEDFXDissolve        = 18
	WLEDFXDissolveRnd     = 19
	WLEDFXSparkle         = 20
	WLEDFXSparkleFlash    = 21
	WLEDFXSparklePlus     = 22
	WLEDFXStrobe          = 23
	WLEDFXStrobeRainbow   = 24
	WLEDFXStrobeMega      = 25
	WLEDFXBlinkRainbow    = 26
	WLEDFXChase           = 27
	WLEDFXChaseRandom     = 28
	WLEDFXChaseRainbow    = 29
	WLEDFXChaseFlash      = 30
	WLEDFXChaseFlashRnd   = 31
	WLEDFXRainbowRunner   = 32
	WLEDFXColorful        = 33
	WLEDFXTrafficLight    = 34
	WLEDFXSweepRandom     = 35
	WLEDFXChase2          = 36
	WLEDFXAurora          = 37
	WLEDFXStream          = 38
	WLEDFXScanner         = 39
	WLEDFXLightning       = 40
	WLEDFXICU             = 41
	WLEDFXMultiComet      = 42
	WLEDFXScannerDual     = 43
	WLEDFXStreamTwo       = 44
	WLEDFXOscillate       = 45
	WLEDFXPride           = 46
	WLEDFXJuggle          = 47
	WLEDFXPalette         = 48
	WLEDFXFire2012        = 49
	WLEDFXColorwaves      = 50
	WLEDFXBPM             = 51
	WLEDFXFillNoise       = 52
	WLEDFXNoise1          = 53
	WLEDFXNoise2          = 54
	WLEDFXNoise3          = 55
	WLEDFXNoise4          = 56
	WLEDFXColortwinkle    = 57
	WLEDFXLake            = 58
	WLEDFXMeteor          = 59
	WLEDFXMeteorSmooth    = 60
	WLEDFXRailway         = 61
	WLEDFXRipple          = 62
	WLEDFXTwinkleFox      = 63
	WLEDFXTwinkleCat      = 64
	WLEDFXHalloweenEyes   = 65
	WLEDFXSolidPattern    = 66
	WLEDFXSolidPatternTri = 67
	WLEDFXSpots           = 68
	WLEDFXSpotsFade       = 69
	WLEDFXGlitter         = 70
	WLEDFXCandle          = 71
	WLEDFXFireworks       = 72
	WLEDFXFireworksStarb  = 73
	WLEDFXFireworks1D     = 74
	WLEDFXBouncing        = 75
	WLEDFXSinelon         = 76
	WLEDFXSinelonDual     = 77
	WLEDFXSinelonRainbow  = 78
	WLEDFXPopcorn         = 79
	WLEDFXDrip            = 80
	WLEDFXPlasma          = 81
	WLEDFXPercent         = 82
	WLEDFXRipplePeak      = 83
	WLEDFXFlowStripe      = 84
	WLEDFXWaves           = 85
)

// EffectMetadata contains metadata about a WLED effect
type EffectMetadata struct {
	ID          int      // WLED effect ID
	Name        string   // Display name
	Description string   // Effect description
	HasSpeed    bool     // Uses sx parameter
	HasIntensity bool    // Uses ix parameter
	HasCustom1  bool     // Uses c1 parameter
	HasCustom2  bool     // Uses c2 parameter
	HasCustom3  bool     // Uses c3 parameter
	UsesPalette bool     // Can use palette colors
	MinColors   int      // Minimum required colors
	MaxColors   int      // Maximum useful colors
	SpeedDesc   string   // Description of what speed controls
	IntensDesc  string   // Description of what intensity controls
	Custom1Desc string   // Description of c1
	Custom2Desc string   // Description of c2
	Custom3Desc string   // Description of c3
}

// SupportedEffects contains metadata for effects we support in the firmware
var SupportedEffects = map[int]EffectMetadata{
	WLEDFXSolid: {
		ID:          WLEDFXSolid,
		Name:        "Solid",
		Description: "Static solid color",
		MinColors:   1,
		MaxColors:   1,
	},
	WLEDFXBreathe: {
		ID:          WLEDFXBreathe,
		Name:        "Breathe",
		Description: "Smooth pulsing brightness",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Breath rate",
		IntensDesc:  "Minimum brightness",
	},
	WLEDFXWipe: {
		ID:          WLEDFXWipe,
		Name:        "Wipe",
		Description: "Color wipe across strip",
		HasSpeed:    true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Wipe speed",
	},
	WLEDFXScan: {
		ID:          WLEDFXScan,
		Name:        "Scan",
		Description: "Single dot bouncing back and forth",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Scan speed",
		IntensDesc:  "Dot width",
	},
	WLEDFXSparkle: {
		ID:          WLEDFXSparkle,
		Name:        "Sparkle",
		Description: "Random twinkling pixels",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Sparkle rate",
		IntensDesc:  "Sparkle density",
	},
	WLEDFXScanner: {
		ID:          WLEDFXScanner,
		Name:        "Scanner",
		Description: "Knight Rider style scanner with trail",
		HasSpeed:    true,
		HasIntensity: true,
		HasCustom1:  true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Scan speed",
		IntensDesc:  "Eye width",
		Custom1Desc: "Trail length",
	},
	WLEDFXRainbow: {
		ID:          WLEDFXRainbow,
		Name:        "Rainbow",
		Description: "Moving rainbow gradient",
		HasSpeed:    true,
		UsesPalette: false,
		MinColors:   0,
		MaxColors:   0,
		SpeedDesc:   "Cycle speed",
	},
	WLEDFXColorwaves: {
		ID:          WLEDFXColorwaves,
		Name:        "Colorwaves",
		Description: "Smooth flowing color waves",
		HasSpeed:    true,
		HasIntensity: true,
		UsesPalette: true,
		MinColors:   2,
		MaxColors:   3,
		SpeedDesc:   "Wave speed",
		IntensDesc:  "Wave spread",
	},
	WLEDFXFire2012: {
		ID:          WLEDFXFire2012,
		Name:        "Fire 2012",
		Description: "Realistic fire simulation",
		HasSpeed:    true,
		HasIntensity: true,
		HasCustom1:  true,
		UsesPalette: true,
		MinColors:   3,
		MaxColors:   3,
		SpeedDesc:   "Flame speed",
		IntensDesc:  "Cooling (flame height)",
		Custom1Desc: "Sparking (new flames)",
	},
	WLEDFXCandle: {
		ID:          WLEDFXCandle,
		Name:        "Candle",
		Description: "Flickering candle flame",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Flicker speed",
		IntensDesc:  "Flicker intensity",
	},
	WLEDFXMeteor: {
		ID:          WLEDFXMeteor,
		Name:        "Meteor",
		Description: "Shooting meteor with trail",
		HasSpeed:    true,
		HasIntensity: true,
		HasCustom1:  true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Meteor speed",
		IntensDesc:  "Trail length",
		Custom1Desc: "Decay rate",
	},
	WLEDFXRipple: {
		ID:          WLEDFXRipple,
		Name:        "Ripple",
		Description: "Expanding ripple effect",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   3,
		SpeedDesc:   "Ripple speed",
		IntensDesc:  "Max ripples",
	},
	WLEDFXTwinkle: {
		ID:          WLEDFXTwinkle,
		Name:        "Twinkle",
		Description: "Random fading twinkles",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   3,
		SpeedDesc:   "Twinkle rate",
		IntensDesc:  "Density",
	},
	WLEDFXChase: {
		ID:          WLEDFXChase,
		Name:        "Chase",
		Description: "Theater chase pattern",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   2,
		SpeedDesc:   "Chase speed",
		IntensDesc:  "Gap size",
	},
	WLEDFXFireworks: {
		ID:          WLEDFXFireworks,
		Name:        "Fireworks",
		Description: "Multi-burst fireworks",
		HasSpeed:    true,
		HasIntensity: true,
		MinColors:   1,
		MaxColors:   3,
		SpeedDesc:   "Launch rate",
		IntensDesc:  "Burst intensity",
	},
	WLEDFXPalette: {
		ID:          WLEDFXPalette,
		Name:        "Palette",
		Description: "Smooth palette cycling",
		HasSpeed:    true,
		HasIntensity: true,
		UsesPalette: true,
		MinColors:   2,
		MaxColors:   3,
		SpeedDesc:   "Cycle speed",
		IntensDesc:  "Spread",
	},
	WLEDFXPride: {
		ID:          WLEDFXPride,
		Name:        "Gradient",
		Description: "Static color gradient",
		HasSpeed:    true,
		UsesPalette: true,
		MinColors:   2,
		MaxColors:   3,
		SpeedDesc:   "Animation speed (0 for static)",
	},
}

// LCLToWLEDEffectMap maps LCL effect names to WLED effect IDs
var LCLToWLEDEffectMap = map[string]int{
	"solid":    WLEDFXSolid,
	"pulse":    WLEDFXBreathe,
	"breathe":  WLEDFXBreathe,
	"sparkle":  WLEDFXSparkle,
	"gradient": WLEDFXPride,
	"fire":     WLEDFXFire2012,
	"candle":   WLEDFXCandle,
	"wave":     WLEDFXColorwaves,
	"chase":    WLEDFXChase,
	"rainbow":  WLEDFXRainbow,
	"scanner":  WLEDFXScanner,
	"wipe":     WLEDFXWipe,
}

// EffectNameMap maps effect names (case-insensitive) to WLED effect IDs
var EffectNameMap = map[string]int{
	"solid":       WLEDFXSolid,
	"breathe":     WLEDFXBreathe,
	"pulse":       WLEDFXBreathe,
	"wipe":        WLEDFXWipe,
	"scan":        WLEDFXScan,
	"sparkle":     WLEDFXSparkle,
	"scanner":     WLEDFXScanner,
	"rainbow":     WLEDFXRainbow,
	"colorwaves":  WLEDFXColorwaves,
	"wave":        WLEDFXColorwaves,
	"fire":        WLEDFXFire2012,
	"fire2012":    WLEDFXFire2012,
	"candle":      WLEDFXCandle,
	"meteor":      WLEDFXMeteor,
	"ripple":      WLEDFXRipple,
	"twinkle":     WLEDFXTwinkle,
	"chase":       WLEDFXChase,
	"fireworks":   WLEDFXFireworks,
	"palette":     WLEDFXPalette,
	"gradient":    WLEDFXPride,
}

// GetEffectByName returns the WLED effect ID for a given name
func GetEffectByName(name string) (int, bool) {
	id, ok := EffectNameMap[strings.ToLower(name)]
	return id, ok
}

// GetEffectMetadata returns metadata for a WLED effect ID
func GetEffectMetadata(id int) (EffectMetadata, bool) {
	meta, ok := SupportedEffects[id]
	return meta, ok
}

// GetEffectName returns the display name for a WLED effect ID
func GetEffectName(id int) string {
	if meta, ok := SupportedEffects[id]; ok {
		return meta.Name
	}
	return "Unknown"
}

// IsEffectSupported returns true if the effect ID is supported by our firmware
func IsEffectSupported(id int) bool {
	_, ok := SupportedEffects[id]
	return ok
}

// GetSupportedEffectIDs returns a list of all supported effect IDs
func GetSupportedEffectIDs() []int {
	ids := make([]int, 0, len(SupportedEffects))
	for id := range SupportedEffects {
		ids = append(ids, id)
	}
	return ids
}

// WLED Palette IDs (commonly used)
const (
	WLEDPaletteDefault      = 0
	WLEDPaletteRandom       = 1
	WLEDPaletteGradient     = 2
	WLEDPaletteFire         = 35
	WLEDPaletteParty        = 6
	WLEDPaletteCloud        = 7
	WLEDPaletteLava         = 8
	WLEDPaletteOcean        = 9
	WLEDPaletteForest       = 10
	WLEDPaletteRainbow      = 11
	WLEDPaletteRainbowStripe = 12
	WLEDPaletteSunset       = 33
)
