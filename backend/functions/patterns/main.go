package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "time"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/google/uuid"

    "candle-lights/backend/shared"
)

var patternsTable = os.Getenv("PATTERNS_TABLE")

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Printf("=== Patterns Handler Called ===")
    log.Printf("Path: %s", request.Path)
    log.Printf("Method: %s", request.HTTPMethod)

    // Validate authentication
    username, err := shared.ValidateAuth(ctx, request)
    if err != nil || username == "" {
        log.Printf("Authentication failed: err=%v, username=%s", err, username)
        return shared.CreateErrorResponse(401, "Unauthorized"), nil
    }

    log.Printf("Authenticated user: %s", username)

    path := request.Path
    method := request.HTTPMethod
    patternID := request.PathParameters["patternId"]

    switch {
    case path == "/api/effects" && method == "GET":
        log.Println("Routing to handleListEffects")
        return handleListEffects()
    case path == "/api/patterns" && method == "GET":
        log.Println("Routing to handleListPatterns")
        return handleListPatterns(ctx, username)
    case path == "/api/patterns" && method == "POST":
        log.Println("Routing to handleCreatePattern")
        return handleCreatePattern(ctx, username, request)
    case patternID != "" && method == "GET":
        log.Printf("Routing to handleGetPattern for patternID: %s", patternID)
        return handleGetPattern(ctx, username, patternID)
    case patternID != "" && method == "PUT":
        log.Printf("Routing to handleUpdatePattern for patternID: %s", patternID)
        return handleUpdatePattern(ctx, username, patternID, request)
    case patternID != "" && method == "DELETE":
        log.Printf("Routing to handleDeletePattern for patternID: %s", patternID)
        return handleDeletePattern(ctx, username, patternID)
    default:
        log.Printf("No matching route for path: %s, method: %s", path, method)
        return shared.CreateErrorResponse(404, "Not found"), nil
    }
}

// EffectResponse represents an effect for the API
type EffectResponse struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    HasSpeed    bool   `json:"hasSpeed"`
    HasIntensity bool  `json:"hasIntensity"`
    HasCustom1  bool   `json:"hasCustom1,omitempty"`
    HasCustom2  bool   `json:"hasCustom2,omitempty"`
    HasCustom3  bool   `json:"hasCustom3,omitempty"`
    SpeedDesc   string `json:"speedDesc,omitempty"`
    IntensDesc  string `json:"intensDesc,omitempty"`
    Custom1Desc string `json:"custom1Desc,omitempty"`
    Custom2Desc string `json:"custom2Desc,omitempty"`
    Custom3Desc string `json:"custom3Desc,omitempty"`
    MinColors   int    `json:"minColors"`
    MaxColors   int    `json:"maxColors"`
}

func handleListEffects() (events.APIGatewayProxyResponse, error) {
    effects := []EffectResponse{
        // Basic Effects
        {ID: 0, Name: "Solid", Description: "Static solid color", MinColors: 1, MaxColors: 1},
        {ID: 1, Name: "Blink", Description: "Simple on/off blinking", HasSpeed: true, HasIntensity: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Blink rate", IntensDesc: "Duty cycle"},
        {ID: 2, Name: "Breathe", Description: "Smooth pulsing brightness", HasSpeed: true, HasIntensity: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Breath rate", IntensDesc: "Min brightness"},
        {ID: 3, Name: "Wipe", Description: "Color wipe across strip", HasSpeed: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Wipe speed"},

        // Rainbow & Color Cycling
        {ID: 9, Name: "Rainbow", Description: "Moving rainbow gradient", HasSpeed: true, HasIntensity: true, MinColors: 0, MaxColors: 0, SpeedDesc: "Cycle speed", IntensDesc: "Size"},
        {ID: 12, Name: "Fade", Description: "Smooth color fading", HasSpeed: true, HasIntensity: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Fade speed", IntensDesc: "Width"},
        {ID: 13, Name: "Theater Chase", Description: "Classic theater chase lights", HasSpeed: true, HasIntensity: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Chase speed", IntensDesc: "Gap size"},

        // Twinkle & Sparkle
        {ID: 17, Name: "Twinkle", Description: "Random fading twinkles", HasSpeed: true, HasIntensity: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Twinkle rate", IntensDesc: "Density"},
        {ID: 20, Name: "Sparkle", Description: "Random twinkling pixels", HasSpeed: true, HasIntensity: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Sparkle rate", IntensDesc: "Sparkle count"},

        // Chase & Movement
        {ID: 28, Name: "Chase", Description: "Color chase pattern", HasSpeed: true, HasIntensity: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Chase speed", IntensDesc: "Gap size"},
        {ID: 39, Name: "Scanner", Description: "Knight Rider style scanner", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Scan speed", IntensDesc: "Eye width", Custom1Desc: "Fade rate"},
        {ID: 40, Name: "Larson Scanner", Description: "Cylon eye scanner with smooth fading", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Scan speed", IntensDesc: "Eye width", Custom1Desc: "Fade"},
        {ID: 41, Name: "Comet", Description: "Shooting comet with tail", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Comet speed", IntensDesc: "Tail length", Custom1Desc: "Fade rate"},
        {ID: 42, Name: "Fireworks", Description: "Exploding fireworks bursts", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Launch rate", IntensDesc: "Burst size", Custom1Desc: "Fade"},

        // Gradients & Waves
        {ID: 46, Name: "Gradient", Description: "Static color gradient", HasSpeed: true, HasIntensity: true, MinColors: 2, MaxColors: 3, SpeedDesc: "Animation", IntensDesc: "Spread"},
        {ID: 49, Name: "Fire 2012", Description: "Realistic fire simulation", HasSpeed: true, HasIntensity: true, HasCustom1: true, HasCustom2: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Flame speed", IntensDesc: "Cooling", Custom1Desc: "Sparking", Custom2Desc: "Boost"},
        {ID: 50, Name: "Colorwaves", Description: "Smooth flowing color waves", HasSpeed: true, HasIntensity: true, MinColors: 2, MaxColors: 3, SpeedDesc: "Wave speed", IntensDesc: "Wave width"},

        // Meteor & Trail Effects
        {ID: 59, Name: "Meteor", Description: "Shooting meteor with trail", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Meteor speed", IntensDesc: "Trail length", Custom1Desc: "Decay rate"},

        // Candle & Fire
        {ID: 71, Name: "Candle", Description: "Flickering candle flame", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Flicker speed", IntensDesc: "Flicker amount", Custom1Desc: "Multi-color blend"},

        // Ripple & Wave Effects
        {ID: 79, Name: "Ripple", Description: "Expanding ripple effect", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Ripple speed", IntensDesc: "Max ripples", Custom1Desc: "Width"},

        // Burst & Explosion Effects
        {ID: 89, Name: "Starburst", Description: "Exploding starburst pattern", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Burst rate", IntensDesc: "Star count", Custom1Desc: "Fade"},

        // Physics-based Effects
        {ID: 91, Name: "Bouncing Balls", Description: "Physics-based bouncing balls", HasSpeed: true, HasIntensity: true, HasCustom1: true, HasCustom2: true, MinColors: 1, MaxColors: 3, SpeedDesc: "Gravity", IntensDesc: "Ball count", Custom1Desc: "Fade", Custom2Desc: "Trail"},
        {ID: 92, Name: "Sinelon", Description: "Sine wave oscillating dot", HasSpeed: true, HasIntensity: true, HasCustom1: true, MinColors: 1, MaxColors: 2, SpeedDesc: "Speed", IntensDesc: "Fade rate", Custom1Desc: "Width"},
    }
    return shared.CreateSuccessResponse(200, effects), nil
}

func handleListPatterns(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
    indexName := "userId-index"
    keyCondition := "userId = :userId"
    expressionValues := map[string]types.AttributeValue{
        ":userId": &types.AttributeValueMemberS{Value: username},
    }

    var patterns []shared.Pattern
    if err := shared.Query(ctx, patternsTable, &indexName, keyCondition, expressionValues, &patterns); err != nil {
        return shared.CreateErrorResponse(500, "Failed to retrieve patterns"), nil
    }

    // Debug: Log pattern details including WLED state
    for _, p := range patterns {
        hasWLEDState := p.WLEDState != ""
        hasWLEDBinary := len(p.WLEDBinary) > 0
        hasBytecode := len(p.Bytecode) > 0
        log.Printf("[ListPatterns] Pattern: %s, Type: %s, FormatVersion: %d, HasWLEDState: %v (len=%d), HasWLEDBinary: %v, HasBytecode: %v",
            p.Name, p.Type, p.FormatVersion, hasWLEDState, len(p.WLEDState), hasWLEDBinary, hasBytecode)
    }

    return shared.CreateSuccessResponse(200, patterns), nil
}

func handleCreatePattern(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var pattern shared.Pattern
    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &pattern); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    // Validate pattern
    if pattern.Name == "" || pattern.Type == "" {
        return shared.CreateErrorResponse(400, "Name and type are required"), nil
    }

    // Validate pattern type
    validTypes := map[string]bool{
        shared.PatternCandle:  true,
        shared.PatternSolid:   true,
        shared.PatternPulse:   true,
        shared.PatternWave:    true,
        shared.PatternRainbow: true,
        shared.PatternFire:    true,
    }

    if !validTypes[pattern.Type] {
        return shared.CreateErrorResponse(400, "Invalid pattern type"), nil
    }

    // Validate RGB values (for backward compatibility)
    if pattern.Red < 0 || pattern.Red > 255 ||
        pattern.Green < 0 || pattern.Green > 255 ||
        pattern.Blue < 0 || pattern.Blue > 255 {
        return shared.CreateErrorResponse(400, "RGB values must be between 0 and 255"), nil
    }

    // Validate colors array if provided
    for _, color := range pattern.Colors {
        if color.R < 0 || color.R > 255 ||
            color.G < 0 || color.G > 255 ||
            color.B < 0 || color.B > 255 {
            return shared.CreateErrorResponse(400, "Color RGB values must be between 0 and 255"), nil
        }
        if color.Percentage < 0 || color.Percentage > 100 {
            return shared.CreateErrorResponse(400, "Color percentage must be between 0 and 100"), nil
        }
    }

    // Set defaults
    if pattern.Brightness == 0 {
        pattern.Brightness = 128
    }
    if pattern.Speed == 0 {
        pattern.Speed = 50
    }

    // If WLED state provided, set format version (compilation done client-side via /api/glowblaster/compile)
    if pattern.WLEDState != "" {
        pattern.FormatVersion = 2 // FormatVersionWLED
        log.Printf("Saving pattern with WLED state (length: %d)", len(pattern.WLEDState))
    }

    // Create pattern
    pattern.PatternID = uuid.New().String()
    pattern.UserID = username
    pattern.CreatedAt = time.Now()
    pattern.UpdatedAt = time.Now()

    if err := shared.PutItem(ctx, patternsTable, pattern); err != nil {
        return shared.CreateErrorResponse(500, "Failed to create pattern"), nil
    }

    return shared.CreateSuccessResponse(201, pattern), nil
}

func handleGetPattern(ctx context.Context, username string, patternID string) (events.APIGatewayProxyResponse, error) {
    key, _ := attributevalue.MarshalMap(map[string]string{
        "patternId": patternID,
    })

    var pattern shared.Pattern
    if err := shared.GetItem(ctx, patternsTable, key, &pattern); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if pattern.PatternID == "" {
        return shared.CreateErrorResponse(404, "Pattern not found"), nil
    }

    // Verify ownership
    if pattern.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    return shared.CreateSuccessResponse(200, pattern), nil
}

func handleUpdatePattern(ctx context.Context, username string, patternID string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Get existing pattern
    key, _ := attributevalue.MarshalMap(map[string]string{
        "patternId": patternID,
    })

    var existingPattern shared.Pattern
    if err := shared.GetItem(ctx, patternsTable, key, &existingPattern); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if existingPattern.PatternID == "" {
        return shared.CreateErrorResponse(404, "Pattern not found"), nil
    }

    // Verify ownership
    if existingPattern.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Parse updates
    var updates shared.Pattern
    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &updates); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    // Update fields
    if updates.Name != "" {
        existingPattern.Name = updates.Name
    }
    if updates.Description != "" {
        existingPattern.Description = updates.Description
    }
    if updates.Type != "" {
        existingPattern.Type = updates.Type
    }
    if updates.Red >= 0 && updates.Red <= 255 {
        existingPattern.Red = updates.Red
    }
    if updates.Green >= 0 && updates.Green <= 255 {
        existingPattern.Green = updates.Green
    }
    if updates.Blue >= 0 && updates.Blue <= 255 {
        existingPattern.Blue = updates.Blue
    }
    // Update colors array if provided
    if len(updates.Colors) > 0 {
        // Validate colors
        for _, color := range updates.Colors {
            if color.R < 0 || color.R > 255 ||
                color.G < 0 || color.G > 255 ||
                color.B < 0 || color.B > 255 {
                return shared.CreateErrorResponse(400, "Color RGB values must be between 0 and 255"), nil
            }
        }
        existingPattern.Colors = updates.Colors
    }
    if updates.Brightness > 0 {
        existingPattern.Brightness = updates.Brightness
    }
    if updates.Speed > 0 {
        existingPattern.Speed = updates.Speed
    }
    if updates.Metadata != nil {
        existingPattern.Metadata = updates.Metadata
    }

    // Update WLED state if provided (compilation done client-side via /api/glowblaster/compile)
    if updates.WLEDState != "" {
        existingPattern.WLEDState = updates.WLEDState
        existingPattern.FormatVersion = 2 // FormatVersionWLED
        log.Printf("Updating pattern with WLED state (length: %d)", len(updates.WLEDState))
    }

    existingPattern.UpdatedAt = time.Now()

    if err := shared.PutItem(ctx, patternsTable, existingPattern); err != nil {
        return shared.CreateErrorResponse(500, "Failed to update pattern"), nil
    }

    return shared.CreateSuccessResponse(200, existingPattern), nil
}

func handleDeletePattern(ctx context.Context, username string, patternID string) (events.APIGatewayProxyResponse, error) {
    // Get pattern to verify ownership
    key, _ := attributevalue.MarshalMap(map[string]string{
        "patternId": patternID,
    })

    var pattern shared.Pattern
    if err := shared.GetItem(ctx, patternsTable, key, &pattern); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if pattern.PatternID == "" {
        return shared.CreateErrorResponse(404, "Pattern not found"), nil
    }

    // Verify ownership
    if pattern.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Delete pattern
    if err := shared.DeleteItem(ctx, patternsTable, key); err != nil {
        return shared.CreateErrorResponse(500, "Failed to delete pattern"), nil
    }

    return shared.CreateSuccessResponse(200, map[string]string{
        "message": "Pattern deleted successfully",
    }), nil
}

func main() {
    lambda.Start(handler)
}
