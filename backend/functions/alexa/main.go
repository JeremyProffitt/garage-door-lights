package main

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

    "candle-lights/backend/shared"
)

var (
    devicesTable  = os.Getenv("DEVICES_TABLE")
    patternsTable = os.Getenv("PATTERNS_TABLE")
    usersTable    = os.Getenv("USERS_TABLE")
)

// Alexa Smart Home Skill structures
type AlexaRequest struct {
    Directive Directive `json:"directive"`
}

type Directive struct {
    Header   Header    `json:"header"`
    Endpoint *Endpoint `json:"endpoint,omitempty"`
    Payload  Payload   `json:"payload"`
}

type Header struct {
    Namespace        string `json:"namespace"`
    Name             string `json:"name"`
    PayloadVersion   string `json:"payloadVersion"`
    MessageID        string `json:"messageId"`
    CorrelationToken string `json:"correlationToken,omitempty"`
}

type Endpoint struct {
    EndpointID string `json:"endpointId"`
    Scope      Scope  `json:"scope"`
    Cookie     Cookie `json:"cookie,omitempty"`
}

type Scope struct {
    Type  string `json:"type"`
    Token string `json:"token"`
}

type Cookie struct {
    Username string `json:"username,omitempty"`
}

type Payload map[string]interface{}

type AlexaResponse struct {
    Event Event `json:"event"`
}

type Event struct {
    Header   Header    `json:"header"`
    Endpoint *Endpoint `json:"endpoint,omitempty"`
    Payload  Payload   `json:"payload"`
}

func handler(ctx context.Context, request AlexaRequest) (AlexaResponse, error) {
    namespace := request.Directive.Header.Namespace
    name := request.Directive.Header.Name

    switch namespace {
    case "Alexa.Discovery":
        return handleDiscovery(ctx, request)
    case "Alexa.PowerController":
        return handlePowerControl(ctx, request)
    case "Alexa.ColorController":
        return handleColorControl(ctx, request)
    case "Alexa.BrightnessController":
        return handleBrightnessControl(ctx, request)
    case "Alexa":
        if name == "ReportState" {
            return handleReportState(ctx, request)
        }
    }

    return createErrorResponse(request, "INVALID_DIRECTIVE", "Unsupported directive"), nil
}

func handleDiscovery(ctx context.Context, request AlexaRequest) (AlexaResponse, error) {
    // Get username from token (simplified - in production, validate OAuth token)
    username := getUsernameFromToken(request.Directive.Payload)

    // Get user's devices
    indexName := "userId-index"
    keyCondition := "userId = :userId"
    expressionValues := map[string]types.AttributeValue{
        ":userId": &types.AttributeValueMemberS{Value: username},
    }

    var devices []shared.Device
    shared.Query(ctx, devicesTable, &indexName, keyCondition, expressionValues, &devices)

    // Build discovery response
    endpoints := make([]interface{}, 0)
    for _, device := range devices {
        endpoints = append(endpoints, map[string]interface{}{
            "endpointId":        device.DeviceID,
            "manufacturerName":  "Particle",
            "friendlyName":      device.Name,
            "description":       "WS2812B Candle Lights",
            "displayCategories": []string{"LIGHT"},
            "capabilities": []map[string]interface{}{
                {
                    "type":      "AlexaInterface",
                    "interface": "Alexa.PowerController",
                    "version":   "3",
                    "properties": map[string]interface{}{
                        "supported": []map[string]string{
                            {"name": "powerState"},
                        },
                        "proactivelyReported": false,
                        "retrievable":         true,
                    },
                },
                {
                    "type":      "AlexaInterface",
                    "interface": "Alexa.BrightnessController",
                    "version":   "3",
                    "properties": map[string]interface{}{
                        "supported": []map[string]string{
                            {"name": "brightness"},
                        },
                        "proactivelyReported": false,
                        "retrievable":         true,
                    },
                },
                {
                    "type":      "AlexaInterface",
                    "interface": "Alexa.ColorController",
                    "version":   "3",
                    "properties": map[string]interface{}{
                        "supported": []map[string]string{
                            {"name": "color"},
                        },
                        "proactivelyReported": false,
                        "retrievable":         true,
                    },
                },
                {
                    "type":      "AlexaInterface",
                    "interface": "Alexa",
                    "version":   "3",
                },
            },
            "cookie": map[string]string{
                "username": username,
            },
        })
    }

    return AlexaResponse{
        Event: Event{
            Header: Header{
                Namespace:      "Alexa.Discovery",
                Name:           "Discover.Response",
                PayloadVersion: "3",
                MessageID:      request.Directive.Header.MessageID,
            },
            Payload: Payload{
                "endpoints": endpoints,
            },
        },
    }, nil
}

func handlePowerControl(ctx context.Context, request AlexaRequest) (AlexaResponse, error) {
    deviceID := request.Directive.Endpoint.EndpointID
    username := request.Directive.Endpoint.Cookie.Username
    powerState := request.Directive.Header.Name // TurnOn or TurnOff

    // Get device
    deviceKey, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": deviceID,
    })

    var device shared.Device
    if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
        return createErrorResponse(request, "ENDPOINT_UNREACHABLE", "Device not found"), nil
    }

    // Get user
    userKey, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
        return createErrorResponse(request, "INTERNAL_ERROR", "User not found"), nil
    }

    // Send command to device
    var command, argument string
    if powerState == "TurnOn" {
        // Turn on with last pattern or default candle pattern
        command = "setPattern"
        argument = "0:50" // Candle pattern
        if device.AssignedPattern != "" {
            // Get pattern and apply it
            patternKey, _ := attributevalue.MarshalMap(map[string]string{
                "patternId": device.AssignedPattern,
            })
            var pattern shared.Pattern
            if err := shared.GetItem(ctx, patternsTable, patternKey, &pattern); err == nil {
                applyPatternToDevice(device.ParticleID, pattern, user.ParticleToken)
            }
        } else {
            callParticleFunction(device.ParticleID, command, argument, user.ParticleToken)
        }
    } else {
        // Turn off by setting brightness to 0
        command = "setBright"
        argument = "0"
        callParticleFunction(device.ParticleID, command, argument, user.ParticleToken)
    }

    return createSuccessResponse(request, "powerState", powerState == "TurnOn" ? "ON" : "OFF"), nil
}

func handleColorControl(ctx context.Context, request AlexaRequest) (AlexaResponse, error) {
    deviceID := request.Directive.Endpoint.EndpointID
    username := request.Directive.Endpoint.Cookie.Username

    // Extract color from payload
    color := request.Directive.Payload["color"].(map[string]interface{})
    hue := color["hue"].(float64)
    saturation := color["saturation"].(float64)
    brightness := color["brightness"].(float64)

    // Convert HSB to RGB
    r, g, b := hsbToRgb(hue, saturation, brightness)

    // Get device and user
    deviceKey, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": deviceID,
    })

    var device shared.Device
    shared.GetItem(ctx, devicesTable, deviceKey, &device)

    userKey, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    shared.GetItem(ctx, usersTable, userKey, &user)

    // Send color command
    colorArg := fmt.Sprintf("%d,%d,%d", r, g, b)
    callParticleFunction(device.ParticleID, "setColor", colorArg, user.ParticleToken)

    return createSuccessResponse(request, "color", map[string]float64{
        "hue":        hue,
        "saturation": saturation,
        "brightness": brightness,
    }), nil
}

func handleBrightnessControl(ctx context.Context, request AlexaRequest) (AlexaResponse, error) {
    deviceID := request.Directive.Endpoint.EndpointID
    username := request.Directive.Endpoint.Cookie.Username

    brightness := int(request.Directive.Payload["brightness"].(float64))

    // Get device and user
    deviceKey, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": deviceID,
    })

    var device shared.Device
    shared.GetItem(ctx, devicesTable, deviceKey, &device)

    userKey, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    shared.GetItem(ctx, usersTable, userKey, &user)

    // Convert 0-100 to 0-255
    brightnessValue := (brightness * 255) / 100

    // Send brightness command
    brightnessArg := fmt.Sprintf("%d", brightnessValue)
    callParticleFunction(device.ParticleID, "setBright", brightnessArg, user.ParticleToken)

    return createSuccessResponse(request, "brightness", brightness), nil
}

func handleReportState(ctx context.Context, request AlexaRequest) (AlexaResponse, error) {
    // Return current state of the device
    return createSuccessResponse(request, "powerState", "ON"), nil
}

func createSuccessResponse(request AlexaRequest, propertyName string, value interface{}) AlexaResponse {
    return AlexaResponse{
        Event: Event{
            Header: Header{
                Namespace:        "Alexa",
                Name:             "Response",
                PayloadVersion:   "3",
                MessageID:        request.Directive.Header.MessageID,
                CorrelationToken: request.Directive.Header.CorrelationToken,
            },
            Endpoint: request.Directive.Endpoint,
            Payload:  Payload{},
        },
    }
}

func createErrorResponse(request AlexaRequest, errorType, message string) AlexaResponse {
    return AlexaResponse{
        Event: Event{
            Header: Header{
                Namespace:      "Alexa",
                Name:           "ErrorResponse",
                PayloadVersion: "3",
                MessageID:      request.Directive.Header.MessageID,
            },
            Endpoint: request.Directive.Endpoint,
            Payload: Payload{
                "type":    errorType,
                "message": message,
            },
        },
    }
}

func getUsernameFromToken(payload Payload) string {
    // In production, validate OAuth token and extract username
    // For now, return a placeholder
    if scope, ok := payload["scope"].(map[string]interface{}); ok {
        if token, ok := scope["token"].(string); ok {
            // Parse token to get username
            parts := strings.Split(token, ":")
            if len(parts) > 0 {
                return parts[0]
            }
        }
    }
    return "default"
}

func hsbToRgb(hue, saturation, brightness float64) (int, int, int) {
    h := hue / 360.0
    s := saturation
    v := brightness

    var r, g, b float64

    if s == 0 {
        r, g, b = v, v, v
    } else {
        h = h * 6
        if h == 6 {
            h = 0
        }
        i := int(h)
        f := h - float64(i)
        p := v * (1 - s)
        q := v * (1 - s*f)
        t := v * (1 - s*(1-f))

        switch i {
        case 0:
            r, g, b = v, t, p
        case 1:
            r, g, b = q, v, p
        case 2:
            r, g, b = p, v, t
        case 3:
            r, g, b = p, q, v
        case 4:
            r, g, b = t, p, v
        default:
            r, g, b = v, p, q
        }
    }

    return int(r * 255), int(g * 255), int(b * 255)
}

// Reuse functions from particle handler
func applyPatternToDevice(particleID string, pattern shared.Pattern, token string) error {
    // Implementation from particle handler
    patternMap := map[string]string{
        shared.PatternCandle:  "0",
        shared.PatternSolid:   "1",
        shared.PatternPulse:   "2",
        shared.PatternWave:    "3",
        shared.PatternRainbow: "4",
        shared.PatternFire:    "5",
    }

    patternNum := patternMap[pattern.Type]
    patternArg := fmt.Sprintf("%s:%d", patternNum, pattern.Speed)
    callParticleFunction(particleID, "setPattern", patternArg, token)

    colorArg := fmt.Sprintf("%d,%d,%d", pattern.Red, pattern.Green, pattern.Blue)
    callParticleFunction(particleID, "setColor", colorArg, token)

    brightnessArg := fmt.Sprintf("%d", pattern.Brightness)
    callParticleFunction(particleID, "setBright", brightnessArg, token)

    return nil
}

func callParticleFunction(deviceID, functionName, argument, token string) error {
    // Implementation from particle handler
    // Note: This is a simplified version - actual implementation should use HTTP client
    return nil
}

func main() {
    lambda.Start(handler)
}
