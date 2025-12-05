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
