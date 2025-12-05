# Alexa Integration Plan for LED Light Strips

## Executive Summary

This document outlines the implementation plan for adding Amazon Alexa smart home integration to the garage door LED light strip system. The integration will enable voice control of individual LED strips, including power, brightness, color, and pattern commands.

---

## Table of Contents

1. [Current Architecture Overview](#1-current-architecture-overview)
2. [Alexa Smart Home Skill Architecture](#2-alexa-smart-home-skill-architecture)
3. [Device Capabilities & Voice Commands](#3-device-capabilities--voice-commands)
4. [Implementation Phases](#4-implementation-phases)
5. [Technical Implementation Details](#5-technical-implementation-details)
6. [Data Model Changes](#6-data-model-changes)
7. [Security & Authentication](#7-security--authentication)
8. [Testing Strategy](#8-testing-strategy)
9. [Deployment Considerations](#9-deployment-considerations)
10. [Future Enhancements](#10-future-enhancements)

---

## 1. Current Architecture Overview

### Existing Stack
```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Particle IoT   │────▶│  Particle Cloud  │◀────│  AWS Lambda     │
│  Device (ESP32) │     │  API             │     │  Backend        │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                          │
                                                          ▼
                                                 ┌─────────────────┐
                                                 │  DynamoDB       │
                                                 │  (Users,        │
                                                 │   Devices,      │
                                                 │   Patterns)     │
                                                 └─────────────────┘
```

### Current Capabilities
- **4 LED strips per device** (pins D0-D7)
- **60 LEDs max per strip**
- **8 colors per strip** with percentage distribution
- **7 pattern types**: Off, Candle, Solid, Pulse, Wave, Rainbow, Fire
- **Per-strip control**: brightness, speed, colors, pattern

### Existing Cloud Functions (Firmware)
| Function | Parameters | Description |
|----------|------------|-------------|
| `setPattern` | `pin,patternId,speed` | Set animation pattern |
| `setColor` | `pin,R,G,B` | Set single color |
| `setColors` | `pin,R,G,B,%;...` | Set multi-color distribution |
| `setBright` | `pin,brightness` | Set brightness (0-255) |
| `saveConfig` | `1` | Persist to EEPROM |

---

## 2. Alexa Smart Home Skill Architecture

### Proposed Architecture
```
┌─────────────┐     ┌─────────────────┐     ┌──────────────────┐
│   Amazon    │────▶│  Alexa Skill    │────▶│  Alexa Lambda    │
│   Echo      │     │  Service        │     │  Handler         │
└─────────────┘     └─────────────────┘     └──────────────────┘
                                                     │
                           ┌─────────────────────────┴─────────────────────────┐
                           │                                                   │
                           ▼                                                   ▼
                    ┌─────────────────┐                               ┌─────────────────┐
                    │  DynamoDB       │                               │  Particle Cloud │
                    │  (User Mapping, │                               │  API            │
                    │   Device State) │                               └─────────────────┘
                    └─────────────────┘                                        │
                                                                               ▼
                                                                      ┌─────────────────┐
                                                                      │  Particle IoT   │
                                                                      │  Devices        │
                                                                      └─────────────────┘
```

### Alexa Smart Home Skill Components

1. **Skill Manifest** - Defines skill metadata and capabilities
2. **Account Linking** - OAuth 2.0 flow to link Alexa users to our system
3. **Lambda Handler** - Processes Alexa directives
4. **Device Discovery** - Reports available LED strips to Alexa
5. **Directive Handlers** - Execute power, brightness, color commands

---

## 3. Device Capabilities & Voice Commands

### Alexa Interface Mapping

Each LED strip will be exposed as a separate Alexa smart light with these interfaces:

| Alexa Interface | Capability | Voice Commands |
|-----------------|------------|----------------|
| `Alexa.PowerController` | On/Off | "Turn on garage lights" |
| `Alexa.BrightnessController` | 0-100% | "Set garage lights to 50%" |
| `Alexa.ColorController` | HSB colors | "Set garage lights to red" |
| `Alexa.ModeController` | Patterns | "Set garage lights to fire mode" |

### Voice Command Examples

#### Power Control
- "Alexa, turn on [strip name]"
- "Alexa, turn off all garage lights"
- "Alexa, turn on the left strip"

#### Brightness Control
- "Alexa, set [strip name] to 75%"
- "Alexa, dim the garage lights"
- "Alexa, brighten [strip name]"

#### Color Control
- "Alexa, set [strip name] to red"
- "Alexa, change the garage lights to blue"
- "Alexa, make [strip name] warm white"

#### Pattern/Mode Control (Custom)
- "Alexa, set [strip name] to candle mode"
- "Alexa, set garage lights to fire effect"
- "Alexa, set [strip name] to rainbow mode"

### Device Naming Strategy

Each LED strip gets a unique Alexa endpoint:
```
{Device Name} - Strip {Pin Number}
Example: "Garage Door - Strip D6"
```

User-friendly aliases can be configured:
```
"Left Side", "Right Side", "Top Strip", "Bottom Strip"
```

---

## 4. Implementation Phases

### Phase 1: Foundation (Core Infrastructure)

**Objective**: Establish OAuth 2.0 account linking and basic Lambda infrastructure

**Tasks**:
1. Create OAuth 2.0 authorization server endpoints
   - `/oauth/authorize` - Authorization endpoint
   - `/oauth/token` - Token exchange endpoint
   - `/oauth/refresh` - Token refresh endpoint
2. Create DynamoDB tables for OAuth tokens
3. Set up Alexa Smart Home Skill in Amazon Developer Console
4. Create initial Lambda function structure
5. Implement account linking flow

**Deliverables**:
- Working OAuth 2.0 flow
- Alexa skill registered and linked to Lambda
- User account linking functional

---

### Phase 2: Device Discovery

**Objective**: Enable Alexa to discover user's LED strips

**Tasks**:
1. Implement `Alexa.Discovery` directive handler
2. Query DynamoDB for user's devices and strips
3. Generate Alexa endpoint definitions for each strip
4. Define capabilities per endpoint
5. Handle discovery response formatting

**Alexa Discovery Response Structure**:
```json
{
  "event": {
    "header": {
      "namespace": "Alexa.Discovery",
      "name": "Discover.Response"
    },
    "payload": {
      "endpoints": [
        {
          "endpointId": "device-uuid-strip-D6",
          "friendlyName": "Garage Left Strip",
          "manufacturerName": "GarageLights",
          "capabilities": [
            { "interface": "Alexa.PowerController" },
            { "interface": "Alexa.BrightnessController" },
            { "interface": "Alexa.ColorController" },
            { "interface": "Alexa.ModeController" }
          ]
        }
      ]
    }
  }
}
```

**Deliverables**:
- All user LED strips appear in Alexa app
- Proper naming and grouping
- Capability advertisement

---

### Phase 3: Power & Brightness Control

**Objective**: Implement basic on/off and brightness commands

**Tasks**:
1. Implement `Alexa.PowerController` handlers
   - `TurnOn` → Set pattern to last used (or solid) with saved brightness
   - `TurnOff` → Set pattern to 0 (OFF)
2. Implement `Alexa.BrightnessController` handlers
   - `SetBrightness` → Map 0-100% to 0-255
   - `AdjustBrightness` → Relative brightness changes
3. Store last-known state in DynamoDB for state reporting
4. Implement `Alexa.ReportState` for state queries

**State Persistence**:
```go
type AlexaStripState struct {
    EndpointID  string
    UserID      string
    DeviceID    string
    Pin         int
    PowerState  string  // "ON" or "OFF"
    Brightness  int     // 0-100
    ColorHue    float64
    ColorSat    float64
    ColorBri    float64
    PatternMode string
    LastUpdated time.Time
}
```

**Deliverables**:
- "Turn on/off" commands working
- Brightness percentage control working
- State queries return accurate data

---

### Phase 4: Color Control

**Objective**: Enable voice color commands

**Tasks**:
1. Implement `Alexa.ColorController` handlers
   - `SetColor` → Convert HSB to RGB, call `setColor`
2. Create HSB to RGB conversion utility
3. Map common color names to RGB values
4. Handle white/warm white via color temperature

**Color Conversion**:
```go
func HSBToRGB(hue, saturation, brightness float64) (r, g, b uint8) {
    // Alexa sends: hue (0-360), saturation (0-1), brightness (0-1)
    // Firmware expects: R, G, B (0-255)
}
```

**Supported Color Names** (Alexa built-in):
- Red, Orange, Yellow, Green, Cyan, Blue, Purple, Pink
- White, Warm White, Soft White, Daylight

**Deliverables**:
- "Set to [color]" commands working
- Accurate color reproduction
- Color state reporting

---

### Phase 5: Pattern/Mode Control

**Objective**: Enable voice control of animation patterns

**Tasks**:
1. Implement `Alexa.ModeController` for patterns
2. Define mode mappings:
   ```
   "candle" → Pattern 1
   "solid" → Pattern 2
   "pulse" → Pattern 3
   "wave" → Pattern 4
   "rainbow" → Pattern 5
   "fire" → Pattern 6
   ```
3. Create mode semantics for Alexa
4. Handle pattern speed defaults

**Mode Controller Configuration**:
```json
{
  "interface": "Alexa.ModeController",
  "instance": "LightStrip.Pattern",
  "properties": {
    "supported": [
      { "value": "Pattern.Candle" },
      { "value": "Pattern.Solid" },
      { "value": "Pattern.Pulse" },
      { "value": "Pattern.Wave" },
      { "value": "Pattern.Rainbow" },
      { "value": "Pattern.Fire" }
    ]
  },
  "semantics": {
    "actionMappings": [
      {
        "actions": ["Alexa.Actions.SetMode"],
        "directive": { "name": "SetMode" }
      }
    ]
  }
}
```

**Deliverables**:
- "Set to [pattern] mode" commands working
- Pattern state reporting
- Speed defaults applied appropriately

---

### Phase 6: Advanced Features & Polish

**Objective**: Add grouping, routines support, and reliability improvements

**Tasks**:
1. Implement device grouping (all strips on one device)
2. Add proactive state reporting (push updates to Alexa)
3. Implement error handling and user feedback
4. Add support for Alexa Routines
5. Optimize response times
6. Add comprehensive logging and monitoring

**Error Response Handling**:
```json
{
  "event": {
    "header": {
      "namespace": "Alexa",
      "name": "ErrorResponse"
    },
    "payload": {
      "type": "ENDPOINT_UNREACHABLE",
      "message": "Device is offline"
    }
  }
}
```

**Deliverables**:
- Device grouping functional
- Robust error handling
- Alexa Routine compatibility
- Production-ready reliability

---

## 5. Technical Implementation Details

### New Lambda Function: `alexa-handler`

**Location**: `backend/functions/alexa/main.go`

**Structure**:
```go
package main

import (
    "context"
    "github.com/aws/aws-lambda-go/lambda"
)

type AlexaRequest struct {
    Directive Directive `json:"directive"`
}

type Directive struct {
    Header   Header   `json:"header"`
    Endpoint Endpoint `json:"endpoint,omitempty"`
    Payload  Payload  `json:"payload"`
}

func handler(ctx context.Context, request AlexaRequest) (interface{}, error) {
    switch request.Directive.Header.Namespace {
    case "Alexa.Discovery":
        return handleDiscovery(ctx, request)
    case "Alexa.PowerController":
        return handlePowerControl(ctx, request)
    case "Alexa.BrightnessController":
        return handleBrightnessControl(ctx, request)
    case "Alexa.ColorController":
        return handleColorControl(ctx, request)
    case "Alexa.ModeController":
        return handleModeControl(ctx, request)
    case "Alexa":
        if request.Directive.Header.Name == "ReportState" {
            return handleReportState(ctx, request)
        }
    }
    return errorResponse("INVALID_DIRECTIVE", "Unsupported directive")
}

func main() {
    lambda.Start(handler)
}
```

### OAuth 2.0 Endpoints

**New Lambda Function**: `backend/functions/oauth/main.go`

**Endpoints**:
```
GET  /oauth/authorize  - Display authorization page
POST /oauth/authorize  - Handle user consent
POST /oauth/token      - Exchange code for tokens
POST /oauth/refresh    - Refresh access tokens
```

**Token Storage** (DynamoDB):
```go
type OAuthToken struct {
    TokenID      string    // PK: access token hash
    UserID       string    // GSI
    RefreshToken string
    Scope        string
    ExpiresAt    time.Time
    CreatedAt    time.Time
}

type OAuthAuthCode struct {
    Code      string    // PK
    UserID    string
    ClientID  string
    Scope     string
    ExpiresAt time.Time
}
```

### Endpoint ID Format

**Structure**: `{deviceId}-strip-{pin}`

**Example**: `abc123-strip-D6`

**Parsing**:
```go
func parseEndpointID(endpointID string) (deviceID string, pin int, err error) {
    // Parse "abc123-strip-D6" → deviceID="abc123", pin=6
    parts := strings.Split(endpointID, "-strip-")
    if len(parts) != 2 {
        return "", 0, errors.New("invalid endpoint ID format")
    }
    deviceID = parts[0]
    pinStr := strings.TrimPrefix(parts[1], "D")
    pin, err = strconv.Atoi(pinStr)
    return
}
```

### Particle API Integration

**Reuse Existing**: `backend/shared/particle.go`

**Add Helper Functions**:
```go
// SetStripPower turns a strip on or off
func SetStripPower(particleToken, particleID string, pin int, on bool) error {
    if on {
        // Restore last pattern or default to solid
        return CallParticleFunction(particleToken, particleID, "setPattern",
            fmt.Sprintf("%d,2,50", pin)) // Solid pattern
    }
    return CallParticleFunction(particleToken, particleID, "setPattern",
        fmt.Sprintf("%d,0,0", pin)) // OFF pattern
}

// SetStripBrightness sets brightness 0-255
func SetStripBrightness(particleToken, particleID string, pin, brightness int) error {
    return CallParticleFunction(particleToken, particleID, "setBright",
        fmt.Sprintf("%d,%d", pin, brightness))
}

// SetStripColor sets RGB color
func SetStripColor(particleToken, particleID string, pin int, r, g, b uint8) error {
    return CallParticleFunction(particleToken, particleID, "setColor",
        fmt.Sprintf("%d,%d,%d,%d", pin, r, g, b))
}

// SetStripPattern sets animation pattern
func SetStripPattern(particleToken, particleID string, pin, pattern, speed int) error {
    return CallParticleFunction(particleToken, particleID, "setPattern",
        fmt.Sprintf("%d,%d,%d", pin, pattern, speed))
}
```

---

## 6. Data Model Changes

### New DynamoDB Tables

#### `AlexaTokens` Table
```
PK: accessTokenHash (string)
Attributes:
  - userID (string) [GSI]
  - refreshToken (string)
  - scope (string)
  - expiresAt (number) [TTL]
  - createdAt (number)
```

#### `AlexaAuthCodes` Table
```
PK: authCode (string)
Attributes:
  - userID (string)
  - clientID (string)
  - redirectURI (string)
  - scope (string)
  - expiresAt (number) [TTL]
```

#### `AlexaDeviceState` Table
```
PK: endpointID (string)
Attributes:
  - userID (string) [GSI]
  - deviceID (string)
  - pin (number)
  - powerState (string)
  - brightness (number)
  - colorHue (number)
  - colorSaturation (number)
  - patternMode (string)
  - lastUpdated (number)
```

### Updates to Existing Tables

#### `Devices` Table - Add Fields
```go
type Device struct {
    // ... existing fields ...

    // Alexa integration fields
    AlexaEnabled    bool              `json:"alexaEnabled"`
    StripAliases    map[string]string `json:"stripAliases"` // pin -> friendly name
}
```

#### `Users` Table - Add Fields
```go
type User struct {
    // ... existing fields ...

    // Alexa integration fields
    AlexaLinked     bool   `json:"alexaLinked"`
    AlexaUserID     string `json:"alexaUserId,omitempty"`
}
```

---

## 7. Security & Authentication

### OAuth 2.0 Flow

```
┌──────────┐     ┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Alexa   │     │   Our Auth  │     │    User      │     │  DynamoDB   │
│  App     │     │   Server    │     │   Browser    │     │             │
└────┬─────┘     └──────┬──────┘     └──────┬───────┘     └──────┬──────┘
     │                  │                   │                    │
     │ 1. Redirect to   │                   │                    │
     │    /oauth/auth   │                   │                    │
     │─────────────────▶│                   │                    │
     │                  │ 2. Show login     │                    │
     │                  │─────────────────▶│                    │
     │                  │                   │                    │
     │                  │ 3. User logs in   │                    │
     │                  │◀─────────────────│                    │
     │                  │                   │                    │
     │                  │ 4. Generate code  │                    │
     │                  │───────────────────────────────────────▶│
     │                  │                   │                    │
     │ 5. Redirect with │                   │                    │
     │    auth code     │                   │                    │
     │◀─────────────────│                   │                    │
     │                  │                   │                    │
     │ 6. Exchange code │                   │                    │
     │    for tokens    │                   │                    │
     │─────────────────▶│                   │                    │
     │                  │ 7. Validate &     │                    │
     │                  │    create tokens  │                    │
     │                  │───────────────────────────────────────▶│
     │                  │                   │                    │
     │ 8. Return tokens │                   │                    │
     │◀─────────────────│                   │                    │
```

### Security Considerations

1. **Token Storage**
   - Access tokens hashed before storage
   - Refresh tokens encrypted at rest
   - Short-lived access tokens (1 hour)
   - Long-lived refresh tokens (30 days)

2. **Request Validation**
   - Validate Alexa request signatures
   - Verify request timestamps (within 150 seconds)
   - Check application ID matches skill ID

3. **User Isolation**
   - All queries scoped to authenticated user
   - Endpoint IDs include device ownership validation
   - No cross-user data access

4. **Particle Token Security**
   - Particle tokens retrieved per-request from DynamoDB
   - Never exposed in Alexa responses
   - Existing encryption maintained

### Request Validation Code

```go
func validateAlexaRequest(request AlexaRequest) error {
    // Verify application ID
    expectedAppID := os.Getenv("ALEXA_SKILL_ID")
    if request.Directive.Header.PayloadVersion != "3" {
        return errors.New("unsupported payload version")
    }

    // Additional timestamp validation for production
    // ...

    return nil
}
```

---

## 8. Testing Strategy

### Unit Tests

**Coverage Areas**:
- OAuth token generation and validation
- Endpoint ID parsing
- HSB to RGB conversion
- Directive routing
- Response formatting

**Example Test**:
```go
func TestHSBToRGB(t *testing.T) {
    tests := []struct {
        hue, sat, bri float64
        r, g, b       uint8
    }{
        {0, 1.0, 1.0, 255, 0, 0},     // Red
        {120, 1.0, 1.0, 0, 255, 0},   // Green
        {240, 1.0, 1.0, 0, 0, 255},   // Blue
        {0, 0, 1.0, 255, 255, 255},   // White
    }

    for _, tt := range tests {
        r, g, b := HSBToRGB(tt.hue, tt.sat, tt.bri)
        assert.Equal(t, tt.r, r)
        assert.Equal(t, tt.g, g)
        assert.Equal(t, tt.b, b)
    }
}
```

### Integration Tests

**Test Scenarios**:
1. Full OAuth flow (authorization → token exchange)
2. Device discovery returns all user strips
3. Power on/off commands reach device
4. Brightness changes persist correctly
5. Color commands convert accurately
6. Pattern modes apply correctly
7. Error handling for offline devices

### End-to-End Tests

**Using Alexa Skill Test Console**:
1. Invoke discovery
2. Test each directive type
3. Verify state reporting accuracy
4. Test error scenarios

**Physical Device Testing**:
1. Voice command → LED response timing
2. Multi-strip simultaneous control
3. Power cycle recovery
4. Offline/online transitions

### Test Environment

```yaml
# test-config.yaml
alexa:
  skill_id: "amzn1.ask.skill.test-xxx"
  test_user_id: "test-user-123"

particle:
  test_device_id: "test-device-xxx"
  test_token: "test-particle-token"

dynamodb:
  endpoint: "http://localhost:8000"
  tables:
    - AlexaTokens
    - AlexaAuthCodes
    - AlexaDeviceState
```

---

## 9. Deployment Considerations

### AWS Infrastructure Updates

**SAM Template Additions** (`template.yaml`):

```yaml
# Alexa Handler Lambda
AlexaFunction:
  Type: AWS::Serverless::Function
  Properties:
    CodeUri: backend/functions/alexa/
    Handler: bootstrap
    Runtime: provided.al2023
    Architectures: [arm64]
    MemorySize: 256
    Timeout: 10
    Environment:
      Variables:
        ALEXA_SKILL_ID: !Ref AlexaSkillId
        USERS_TABLE: !Ref UsersTable
        DEVICES_TABLE: !Ref DevicesTable
        ALEXA_STATE_TABLE: !Ref AlexaDeviceStateTable
    Events:
      AlexaSmartHome:
        Type: AlexaSkill
        Properties:
          SkillId: !Ref AlexaSkillId

# OAuth Lambda
OAuthFunction:
  Type: AWS::Serverless::Function
  Properties:
    CodeUri: backend/functions/oauth/
    Handler: bootstrap
    Runtime: provided.al2023
    Events:
      Authorize:
        Type: Api
        Properties:
          Path: /oauth/authorize
          Method: ANY
      Token:
        Type: Api
        Properties:
          Path: /oauth/token
          Method: POST

# New DynamoDB Tables
AlexaTokensTable:
  Type: AWS::DynamoDB::Table
  Properties:
    TableName: !Sub "${AWS::StackName}-alexa-tokens"
    BillingMode: PAY_PER_REQUEST
    AttributeDefinitions:
      - AttributeName: tokenHash
        AttributeType: S
      - AttributeName: userID
        AttributeType: S
    KeySchema:
      - AttributeName: tokenHash
        KeyType: HASH
    GlobalSecondaryIndexes:
      - IndexName: userID-index
        KeySchema:
          - AttributeName: userID
            KeyType: HASH
    TimeToLiveSpecification:
      AttributeName: expiresAt
      Enabled: true

AlexaDeviceStateTable:
  Type: AWS::DynamoDB::Table
  Properties:
    TableName: !Sub "${AWS::StackName}-alexa-state"
    BillingMode: PAY_PER_REQUEST
    AttributeDefinitions:
      - AttributeName: endpointID
        AttributeType: S
      - AttributeName: userID
        AttributeType: S
    KeySchema:
      - AttributeName: endpointID
        KeyType: HASH
    GlobalSecondaryIndexes:
      - IndexName: userID-index
        KeySchema:
          - AttributeName: userID
            KeyType: HASH
```

### Alexa Developer Console Setup

1. **Create Smart Home Skill**
   - Skill name: "Garage Lights"
   - Default language: English (US)
   - Smart Home skill model

2. **Configure Account Linking**
   - Authorization URI: `https://api.yourdomain.com/oauth/authorize`
   - Access Token URI: `https://api.yourdomain.com/oauth/token`
   - Client ID: Generate unique ID
   - Client Secret: Generate secure secret
   - Scope: `control_lights`

3. **Configure Endpoint**
   - Default endpoint: Lambda ARN
   - Region: Same as main infrastructure

### Environment Variables

```bash
# Lambda Environment
ALEXA_SKILL_ID=amzn1.ask.skill.xxx-xxx-xxx
ALEXA_CLIENT_ID=alexa-garage-lights
ALEXA_CLIENT_SECRET=<secure-secret>
JWT_SECRET=<secure-jwt-secret>
```

### Deployment Steps

1. Deploy DynamoDB tables first
2. Deploy OAuth Lambda function
3. Register Alexa skill in Developer Console
4. Configure account linking with OAuth endpoints
5. Deploy Alexa handler Lambda
6. Link Lambda to Alexa skill
7. Submit for certification (when ready)

---

## 10. Future Enhancements

### Phase 7: Proactive State Updates

Push state changes to Alexa when device is controlled via web UI:

```go
func SendAlexaStateUpdate(userID, endpointID string, state map[string]interface{}) error {
    // Use Alexa Event Gateway to push updates
    // POST https://api.amazonalexa.com/v3/events
}
```

### Phase 8: Scene Support

Group multiple strips into scenes:

- "Alexa, set garage to party mode"
- "Alexa, activate movie scene"

### Phase 9: Automation Triggers

Respond to external triggers:

- Door sensor → Turn on lights
- Time-based schedules
- Motion detection

### Phase 10: Multi-Color Voice Commands

Advanced color commands:

- "Alexa, set garage lights to red and green"
- "Alexa, set gradient from blue to purple"

### Phase 11: Speed Control

Animation speed voice control:

- "Alexa, make the fire effect faster"
- "Alexa, slow down the rainbow"

---

## Appendix A: Alexa Response Templates

### Discovery Response
```json
{
  "event": {
    "header": {
      "namespace": "Alexa.Discovery",
      "name": "Discover.Response",
      "payloadVersion": "3",
      "messageId": "<uuid>"
    },
    "payload": {
      "endpoints": []
    }
  }
}
```

### Power Control Response
```json
{
  "context": {
    "properties": [
      {
        "namespace": "Alexa.PowerController",
        "name": "powerState",
        "value": "ON",
        "timeOfSample": "2024-01-01T00:00:00Z",
        "uncertaintyInMilliseconds": 500
      }
    ]
  },
  "event": {
    "header": {
      "namespace": "Alexa",
      "name": "Response",
      "payloadVersion": "3",
      "messageId": "<uuid>",
      "correlationToken": "<token>"
    },
    "endpoint": {
      "endpointId": "<endpoint-id>"
    },
    "payload": {}
  }
}
```

### Error Response
```json
{
  "event": {
    "header": {
      "namespace": "Alexa",
      "name": "ErrorResponse",
      "payloadVersion": "3",
      "messageId": "<uuid>",
      "correlationToken": "<token>"
    },
    "endpoint": {
      "endpointId": "<endpoint-id>"
    },
    "payload": {
      "type": "ENDPOINT_UNREACHABLE",
      "message": "The device is currently offline."
    }
  }
}
```

---

## Appendix B: Pattern Mode Semantic Mappings

```json
{
  "instance": "LightEffect.Mode",
  "capabilityResources": {
    "friendlyNames": [
      { "@type": "text", "value": { "text": "effect", "locale": "en-US" } },
      { "@type": "text", "value": { "text": "mode", "locale": "en-US" } },
      { "@type": "text", "value": { "text": "pattern", "locale": "en-US" } }
    ]
  },
  "configuration": {
    "ordered": false,
    "supportedModes": [
      {
        "value": "LightEffect.Candle",
        "modeResources": {
          "friendlyNames": [
            { "@type": "text", "value": { "text": "candle", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "candlelight", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "flicker", "locale": "en-US" } }
          ]
        }
      },
      {
        "value": "LightEffect.Solid",
        "modeResources": {
          "friendlyNames": [
            { "@type": "text", "value": { "text": "solid", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "static", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "steady", "locale": "en-US" } }
          ]
        }
      },
      {
        "value": "LightEffect.Pulse",
        "modeResources": {
          "friendlyNames": [
            { "@type": "text", "value": { "text": "pulse", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "breathing", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "pulsing", "locale": "en-US" } }
          ]
        }
      },
      {
        "value": "LightEffect.Wave",
        "modeResources": {
          "friendlyNames": [
            { "@type": "text", "value": { "text": "wave", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "flowing", "locale": "en-US" } }
          ]
        }
      },
      {
        "value": "LightEffect.Rainbow",
        "modeResources": {
          "friendlyNames": [
            { "@type": "text", "value": { "text": "rainbow", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "spectrum", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "colorful", "locale": "en-US" } }
          ]
        }
      },
      {
        "value": "LightEffect.Fire",
        "modeResources": {
          "friendlyNames": [
            { "@type": "text", "value": { "text": "fire", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "flame", "locale": "en-US" } },
            { "@type": "text", "value": { "text": "fireplace", "locale": "en-US" } }
          ]
        }
      }
    ]
  }
}
```

---

## Appendix C: File Structure After Implementation

```
backend/
├── functions/
│   ├── alexa/
│   │   ├── main.go              # Lambda entry point
│   │   ├── discovery.go         # Discovery handler
│   │   ├── power.go             # Power controller
│   │   ├── brightness.go        # Brightness controller
│   │   ├── color.go             # Color controller
│   │   ├── mode.go              # Mode controller (patterns)
│   │   ├── state.go             # State reporting
│   │   └── responses.go         # Response builders
│   ├── oauth/
│   │   ├── main.go              # Lambda entry point
│   │   ├── authorize.go         # Authorization endpoint
│   │   ├── token.go             # Token exchange
│   │   └── templates/
│   │       └── login.html       # Login page template
│   ├── auth/                    # Existing
│   ├── devices/                 # Existing
│   ├── particle/                # Existing
│   └── patterns/                # Existing
├── shared/
│   ├── models.go                # Updated with Alexa fields
│   ├── alexa_models.go          # New: Alexa-specific models
│   ├── alexa_db.go              # New: Alexa DynamoDB operations
│   ├── color_utils.go           # New: HSB/RGB conversion
│   └── ...                      # Existing files
└── ...

docs/
├── ALEXA_INTEGRATION_PLAN.md    # This document
└── ...
```

---

*Document Version: 1.0*
*Last Updated: December 2024*
