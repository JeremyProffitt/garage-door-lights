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

var devicesTable = os.Getenv("DEVICES_TABLE")
var patternsTable = os.Getenv("PATTERNS_TABLE")

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Printf("=== Devices Handler Called ===")
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
    deviceID := request.PathParameters["deviceId"]

    switch {
    case path == "/api/devices" && method == "GET":
        log.Println("Routing to handleListDevices")
        return handleListDevices(ctx, username)
    case path == "/api/devices" && method == "POST":
        log.Println("Routing to handleRegisterDevice")
        return handleRegisterDevice(ctx, username, request)
    case deviceID != "" && method == "GET":
        log.Printf("Routing to handleGetDevice for deviceID: %s", deviceID)
        return handleGetDevice(ctx, username, deviceID)
    case deviceID != "" && method == "PUT":
        log.Printf("Routing to handleUpdateDevice for deviceID: %s", deviceID)
        return handleUpdateDevice(ctx, username, deviceID, request)
    case deviceID != "" && method == "DELETE":
        log.Printf("Routing to handleDeleteDevice for deviceID: %s", deviceID)
        return handleDeleteDevice(ctx, username, deviceID)
    case deviceID != "" && path == "/api/devices/"+deviceID+"/pattern" && method == "PUT":
        log.Printf("Routing to handleAssignPattern for deviceID: %s", deviceID)
        return handleAssignPattern(ctx, username, deviceID, request)
    default:
        log.Printf("No matching route for path: %s, method: %s", path, method)
        return shared.CreateErrorResponse(404, "Not found"), nil
    }
}

func handleListDevices(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
    indexName := "userId-index"
    keyCondition := "userId = :userId"
    expressionValues := map[string]types.AttributeValue{
        ":userId": &types.AttributeValueMemberS{Value: username},
    }

    var devices []shared.Device
    if err := shared.Query(ctx, devicesTable, &indexName, keyCondition, expressionValues, &devices); err != nil {
        return shared.CreateErrorResponse(500, "Failed to retrieve devices"), nil
    }

    return shared.CreateSuccessResponse(200, devices), nil
}

func handleRegisterDevice(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var deviceReq struct {
        Name       string `json:"name"`
        ParticleID string `json:"particleId"`
    }

    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &deviceReq); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    if deviceReq.Name == "" || deviceReq.ParticleID == "" {
        return shared.CreateErrorResponse(400, "Name and particleId are required"), nil
    }

    // Create device
    device := shared.Device{
        DeviceID:   uuid.New().String(),
        UserID:     username,
        Name:       deviceReq.Name,
        ParticleID: deviceReq.ParticleID,
        IsOnline:   false,
        LastSeen:   time.Now(),
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }

    if err := shared.PutItem(ctx, devicesTable, device); err != nil {
        return shared.CreateErrorResponse(500, "Failed to register device"), nil
    }

    return shared.CreateSuccessResponse(201, device), nil
}

func handleGetDevice(ctx context.Context, username string, deviceID string) (events.APIGatewayProxyResponse, error) {
    key, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": deviceID,
    })

    var device shared.Device
    if err := shared.GetItem(ctx, devicesTable, key, &device); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if device.DeviceID == "" {
        return shared.CreateErrorResponse(404, "Device not found"), nil
    }

    // Verify ownership
    if device.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    return shared.CreateSuccessResponse(200, device), nil
}

func handleUpdateDevice(ctx context.Context, username string, deviceID string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Get existing device
    key, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": deviceID,
    })

    var existingDevice shared.Device
    if err := shared.GetItem(ctx, devicesTable, key, &existingDevice); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if existingDevice.DeviceID == "" {
        return shared.CreateErrorResponse(404, "Device not found"), nil
    }

    // Verify ownership
    if existingDevice.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Parse updates
    var updates struct {
        Name     string `json:"name,omitempty"`
        IsOnline *bool  `json:"isOnline,omitempty"`
    }

    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &updates); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    // Update fields
    if updates.Name != "" {
        existingDevice.Name = updates.Name
    }
    if updates.IsOnline != nil {
        existingDevice.IsOnline = *updates.IsOnline
        if *updates.IsOnline {
            existingDevice.LastSeen = time.Now()
        }
    }

    existingDevice.UpdatedAt = time.Now()

    if err := shared.PutItem(ctx, devicesTable, existingDevice); err != nil {
        return shared.CreateErrorResponse(500, "Failed to update device"), nil
    }

    return shared.CreateSuccessResponse(200, existingDevice), nil
}

func handleDeleteDevice(ctx context.Context, username string, deviceID string) (events.APIGatewayProxyResponse, error) {
    // Get device to verify ownership
    key, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": deviceID,
    })

    var device shared.Device
    if err := shared.GetItem(ctx, devicesTable, key, &device); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if device.DeviceID == "" {
        return shared.CreateErrorResponse(404, "Device not found"), nil
    }

    // Verify ownership
    if device.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Delete device
    if err := shared.DeleteItem(ctx, devicesTable, key); err != nil {
        return shared.CreateErrorResponse(500, "Failed to delete device"), nil
    }

    return shared.CreateSuccessResponse(200, map[string]string{
        "message": "Device deleted successfully",
    }), nil
}

func handleAssignPattern(ctx context.Context, username string, deviceID string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Get device
    deviceKey, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": deviceID,
    })

    var device shared.Device
    if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if device.DeviceID == "" {
        return shared.CreateErrorResponse(404, "Device not found"), nil
    }

    // Verify ownership
    if device.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Parse request
    var assignReq struct {
        PatternID string `json:"patternId"`
    }

    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &assignReq); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    if assignReq.PatternID == "" {
        return shared.CreateErrorResponse(400, "patternId is required"), nil
    }

    // Verify pattern exists and belongs to user
    patternKey, _ := attributevalue.MarshalMap(map[string]string{
        "patternId": assignReq.PatternID,
    })

    var pattern shared.Pattern
    if err := shared.GetItem(ctx, patternsTable, patternKey, &pattern); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if pattern.PatternID == "" {
        return shared.CreateErrorResponse(404, "Pattern not found"), nil
    }

    if pattern.UserID != username {
        return shared.CreateErrorResponse(403, "Pattern access denied"), nil
    }

    // Assign pattern to device
    device.AssignedPattern = assignReq.PatternID
    device.UpdatedAt = time.Now()

    if err := shared.PutItem(ctx, devicesTable, device); err != nil {
        return shared.CreateErrorResponse(500, "Failed to assign pattern"), nil
    }

    return shared.CreateSuccessResponse(200, device), nil
}

func main() {
    lambda.Start(handler)
}
