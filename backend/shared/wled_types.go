package shared

// WLED JSON API Types
// These types map to the WLED JSON API format for LED control.
// See: https://kno.wledge.me/interfaces/json-api/

// WLEDState represents the top-level WLED state object
type WLEDState struct {
	On         bool          `json:"on"`                    // Master power state
	Brightness int           `json:"bri"`                   // Global brightness (0-255)
	Transition int           `json:"transition,omitempty"`  // Transition time in 100ms units
	Segments   []WLEDSegment `json:"seg"`                   // Segment configurations
}

// WLEDSegment represents a single LED segment with its own effect and colors
type WLEDSegment struct {
	ID        int     `json:"id,omitempty"`   // Segment ID (0-based)
	Start     int     `json:"start"`          // Start LED index (inclusive)
	Stop      int     `json:"stop"`           // Stop LED index (exclusive)
	EffectID  int     `json:"fx"`             // WLED effect ID
	Speed     int     `json:"sx,omitempty"`   // Effect speed (0-255)
	Intensity int     `json:"ix,omitempty"`   // Effect intensity (0-255)
	Custom1   int     `json:"c1,omitempty"`   // Custom parameter 1
	Custom2   int     `json:"c2,omitempty"`   // Custom parameter 2
	Custom3   int     `json:"c3,omitempty"`   // Custom parameter 3
	Colors    [][]int `json:"col"`            // Colors array: [[R,G,B], [R,G,B], [R,G,B]]
	PaletteID int     `json:"pal,omitempty"`  // WLED palette ID (0-71)
	Reverse   bool    `json:"rev,omitempty"`  // Reverse direction
	Mirror    bool    `json:"mi,omitempty"`   // Mirror effect
	On        bool    `json:"on"`             // Segment power state
}

// WLEDSegmentFlags packs segment boolean flags into a single byte
type WLEDSegmentFlags struct {
	Reverse bool
	Mirror  bool
	On      bool
}

// ToByte converts segment flags to a packed byte
func (f WLEDSegmentFlags) ToByte() byte {
	var b byte
	if f.Reverse {
		b |= 0x01
	}
	if f.Mirror {
		b |= 0x02
	}
	if f.On {
		b |= 0x04
	}
	return b
}

// FromByte unpacks a byte into segment flags
func (f *WLEDSegmentFlags) FromByte(b byte) {
	f.Reverse = (b & 0x01) != 0
	f.Mirror = (b & 0x02) != 0
	f.On = (b & 0x04) != 0
}

// WLEDBinaryHeader represents the 8-byte header of WLEDb format
type WLEDBinaryHeader struct {
	Magic   [4]byte // "WLED" (0x57 0x4C 0x45 0x44)
	Version byte    // Format version (0x01)
	Flags   byte    // bit0: power on, bit1-7: reserved
	Length  uint16  // Total length excluding header (big-endian)
}

// WLEDBinaryGlobalState represents the 4-byte global state section
type WLEDBinaryGlobalState struct {
	Brightness   byte   // Global brightness (0-255)
	Transition   uint16 // Transition time in 100ms units (big-endian)
	SegmentCount byte   // Number of segments (1-8)
}

// WLEDBinarySegment represents per-segment binary data (~24 bytes)
type WLEDBinarySegment struct {
	ID        byte      // Segment ID
	Start     uint16    // Start LED (big-endian)
	Stop      uint16    // Stop LED (big-endian)
	EffectID  byte      // WLED effect ID
	Speed     byte      // Effect speed (0-255)
	Intensity byte      // Effect intensity (0-255)
	Custom1   byte      // Effect param c1
	Custom2   byte      // Effect param c2
	Custom3   byte      // Effect param c3
	PaletteID byte      // WLED palette ID
	Flags     byte      // bit0: reverse, bit1: mirror, bit2: on
	Colors    [3][3]byte // Up to 3 RGB colors
	Checksum  byte      // XOR of segment bytes
}

// WLEDb binary format constants
const (
	WLEDBMagic          = "WLED"
	WLEDBVersion        = 0x01
	WLEDBHeaderSize     = 8
	WLEDBGlobalSize     = 4
	WLEDBSegmentMinSize = 24  // Minimum segment size (1 color)
	WLEDBSegmentMaxSize = 30  // Maximum segment size (3 colors)
	WLEDBMaxSegments    = 8
	WLEDBMaxColors      = 3
)

// WLEDb binary offsets
const (
	WLEDBOffsetMagic         = 0
	WLEDBOffsetVersion       = 4
	WLEDBOffsetFlags         = 5
	WLEDBOffsetLength        = 6
	WLEDBOffsetBrightness    = 8
	WLEDBOffsetTransition    = 9
	WLEDBOffsetSegmentCount  = 11
	WLEDBOffsetSegmentsStart = 12
)

// Per-segment offsets (relative to segment start)
const (
	WLEDBSegOffsetID        = 0
	WLEDBSegOffsetStart     = 1
	WLEDBSegOffsetStop      = 3
	WLEDBSegOffsetEffectID  = 5
	WLEDBSegOffsetSpeed     = 6
	WLEDBSegOffsetIntensity = 7
	WLEDBSegOffsetCustom1   = 8
	WLEDBSegOffsetCustom2   = 9
	WLEDBSegOffsetCustom3   = 10
	WLEDBSegOffsetPaletteID = 11
	WLEDBSegOffsetFlags     = 12
	WLEDBSegOffsetColorCnt  = 13
	WLEDBSegOffsetColor1    = 14
	WLEDBSegOffsetColor2    = 17
	WLEDBSegOffsetColor3    = 20
	WLEDBSegOffsetChecksum  = 23
)

// FormatVersion constants for pattern storage
const (
	FormatVersionLCL  = 1 // LCL v4 bytecode format
	FormatVersionWLED = 2 // WLED binary format
)
