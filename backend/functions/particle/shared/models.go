package shared

import "time"

// User represents a user in the system
type User struct {
    Username      string    `json:"username" dynamodbav:"username"`
    PasswordHash  string    `json:"-" dynamodbav:"passwordHash"`
    ParticleToken string    `json:"-" dynamodbav:"particleToken,omitempty"`
    CreatedAt     time.Time `json:"createdAt" dynamodbav:"createdAt"`
    UpdatedAt     time.Time `json:"updatedAt" dynamodbav:"updatedAt"`
}

// PatternColor represents a single color with percentage for multi-color patterns
type PatternColor struct {
    R          int `json:"r" dynamodbav:"r"`
    G          int `json:"g" dynamodbav:"g"`
    B          int `json:"b" dynamodbav:"b"`
    Percentage int `json:"percentage" dynamodbav:"percentage"`
}

// Pattern represents a light pattern/scheme
type Pattern struct {
    PatternID   string            `json:"patternId" dynamodbav:"patternId"`
    UserID      string            `json:"userId" dynamodbav:"userId"`
    Name        string            `json:"name" dynamodbav:"name"`
    Description string            `json:"description" dynamodbav:"description"`
    Type        string            `json:"type" dynamodbav:"type"` // candle, solid, pulse, wave, rainbow, fire
    Red         int               `json:"red" dynamodbav:"red"`
    Green       int               `json:"green" dynamodbav:"green"`
    Blue        int               `json:"blue" dynamodbav:"blue"`
    Colors      []PatternColor    `json:"colors,omitempty" dynamodbav:"colors,omitempty"`
    Brightness  int               `json:"brightness" dynamodbav:"brightness"`
    Speed       int               `json:"speed" dynamodbav:"speed"`
    Metadata    map[string]string `json:"metadata,omitempty" dynamodbav:"metadata"`
    CreatedAt   time.Time         `json:"createdAt" dynamodbav:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt" dynamodbav:"updatedAt"`
}

// LEDStrip represents configuration for a single LED strip on a device pin
type LEDStrip struct {
    Pin      int `json:"pin" dynamodbav:"pin"`           // Pin number (0-7 for D0-D7)
    LEDCount int `json:"ledCount" dynamodbav:"ledCount"` // Number of LEDs on this strip
}

// Device represents a Particle Argon device
type Device struct {
    DeviceID        string     `json:"deviceId" dynamodbav:"deviceId"`
    UserID          string     `json:"userId" dynamodbav:"userId"`
    Name            string     `json:"name" dynamodbav:"name"`
    ParticleID      string     `json:"particleId" dynamodbav:"particleId"`
    AssignedPattern string     `json:"assignedPattern,omitempty" dynamodbav:"assignedPattern"`
    LEDStrips       []LEDStrip `json:"ledStrips,omitempty" dynamodbav:"ledStrips,omitempty"`
    IsOnline        bool       `json:"isOnline" dynamodbav:"isOnline"`
    IsReady         bool       `json:"isReady" dynamodbav:"isReady"`
    FirmwareVersion string     `json:"firmwareVersion,omitempty" dynamodbav:"firmwareVersion"`
    Platform        string     `json:"platform,omitempty" dynamodbav:"platform"`
    IsHidden        bool       `json:"isHidden" dynamodbav:"isHidden"`
    LastSeen        time.Time  `json:"lastSeen" dynamodbav:"lastSeen"`
    CreatedAt       time.Time  `json:"createdAt" dynamodbav:"createdAt"`
    UpdatedAt       time.Time  `json:"updatedAt" dynamodbav:"updatedAt"`
}

// APIResponse is a standard API response
type APIResponse struct {
    Success bool        `json:"success"`
    Message string      `json:"message,omitempty"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
}

// LoginRequest represents a login request
type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
    Token    string `json:"token"`
    Username string `json:"username"`
}

// PatternType constants
const (
    PatternCandle  = "candle"
    PatternSolid   = "solid"
    PatternPulse   = "pulse"
    PatternWave    = "wave"
    PatternRainbow = "rainbow"
    PatternFire    = "fire"
)

// ParticleCommandRequest represents a command to send to Particle device
type ParticleCommandRequest struct {
    DeviceID string `json:"deviceId"`
    Function string `json:"function"`
    Argument string `json:"argument"`
}
