package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"

    "candle-lights/backend/shared"
)

var (
    devicesTable  = os.Getenv("DEVICES_TABLE")
    patternsTable = os.Getenv("PATTERNS_TABLE")
    usersTable    = os.Getenv("USERS_TABLE")
)

const particleAPIBase = "https://api.particle.io/v1"

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Validate authentication
    username, err := shared.ValidateAuth(request)
    if err != nil || username == "" {
        return shared.CreateErrorResponse(401, "Unauthorized"), nil
    }

    path := request.Path
    method := request.HTTPMethod
    deviceID := request.PathParameters["deviceId"]

    switch {
    case path == "/api/particle/command" && method == "POST":
        return handleSendCommand(ctx, username, request)
    case path == "/api/particle/devices/refresh" && method == "POST":
        return handleRefreshDevices(ctx, username)
    case deviceID != "" && method == "GET":
        return handleGetDeviceInfo(ctx, username, deviceID)
    default:
        return shared.CreateErrorResponse(404, "Not found"), nil
    }
}

func handleSendCommand(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var cmdReq struct {
        DeviceID  string `json:"deviceId"`
        PatternID string `json:"patternId,omitempty"`
        Command   string `json:"command,omitempty"`
        Argument  string `json:"argument,omitempty"`
    }

    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &cmdReq); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    if cmdReq.DeviceID == "" {
        return shared.CreateErrorResponse(400, "deviceId is required"), nil
    }

    // Get device
    deviceKey, _ := attributevalue.MarshalMap(map[string]string{
        "deviceId": cmdReq.DeviceID,
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

    // Get user's Particle token
    userKey, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if user.ParticleToken == "" {
        return shared.CreateErrorResponse(400, "Particle token not configured"), nil
    }

    // If PatternID is provided, get pattern and send to device
    if cmdReq.PatternID != "" {
        patternKey, _ := attributevalue.MarshalMap(map[string]string{
            "patternId": cmdReq.PatternID,
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

        // Apply pattern to device
        if err := applyPatternToDevice(device.ParticleID, pattern, user.ParticleToken); err != nil {
            return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to apply pattern: %v", err)), nil
        }

        return shared.CreateSuccessResponse(200, map[string]string{
            "message": "Pattern applied successfully",
            "device":  device.Name,
            "pattern": pattern.Name,
        }), nil
    }

    // Otherwise, send custom command
    if cmdReq.Command == "" || cmdReq.Argument == "" {
        return shared.CreateErrorResponse(400, "command and argument are required"), nil
    }

    if err := callParticleFunction(device.ParticleID, cmdReq.Command, cmdReq.Argument, user.ParticleToken); err != nil {
        return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to send command: %v", err)), nil
    }

    return shared.CreateSuccessResponse(200, map[string]string{
        "message": "Command sent successfully",
    }), nil
}

func handleRefreshDevices(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
    // Get user's Particle token
    userKey, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if user.ParticleToken == "" {
        return shared.CreateErrorResponse(400, "Particle token not configured"), nil
    }

    // Get devices from Particle cloud
    devices, err := getParticleDevices(user.ParticleToken)
    if err != nil {
        return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to get devices from Particle: %v", err)), nil
    }

    return shared.CreateSuccessResponse(200, map[string]interface{}{
        "count":   len(devices),
        "devices": devices,
    }), nil
}

func handleGetDeviceInfo(ctx context.Context, username string, deviceID string) (events.APIGatewayProxyResponse, error) {
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

    // Get user's Particle token
    userKey, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if user.ParticleToken == "" {
        return shared.CreateErrorResponse(400, "Particle token not configured"), nil
    }

    // Get device info from Particle cloud
    info, err := getParticleDeviceInfo(device.ParticleID, user.ParticleToken)
    if err != nil {
        return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to get device info: %v", err)), nil
    }

    return shared.CreateSuccessResponse(200, info), nil
}

func applyPatternToDevice(particleID string, pattern shared.Pattern, token string) error {
    // Convert pattern type to firmware pattern number
    patternMap := map[string]string{
        shared.PatternCandle:  "0",
        shared.PatternSolid:   "1",
        shared.PatternPulse:   "2",
        shared.PatternWave:    "3",
        shared.PatternRainbow: "4",
        shared.PatternFire:    "5",
    }

    patternNum := patternMap[pattern.Type]

    // Send pattern command
    patternArg := fmt.Sprintf("%s:%d", patternNum, pattern.Speed)
    if err := callParticleFunction(particleID, "setPattern", patternArg, token); err != nil {
        return err
    }

    // Send color command
    colorArg := fmt.Sprintf("%d,%d,%d", pattern.Red, pattern.Green, pattern.Blue)
    if err := callParticleFunction(particleID, "setColor", colorArg, token); err != nil {
        return err
    }

    // Send brightness command
    brightnessArg := fmt.Sprintf("%d", pattern.Brightness)
    if err := callParticleFunction(particleID, "setBright", brightnessArg, token); err != nil {
        return err
    }

    // Save configuration to flash
    if err := callParticleFunction(particleID, "saveConfig", "1", token); err != nil {
        return err
    }

    return nil
}

func callParticleFunction(deviceID, functionName, argument, token string) error {
    url := fmt.Sprintf("%s/devices/%s/%s", particleAPIBase, deviceID, functionName)

    data := map[string]string{
        "arg": argument,
    }
    jsonData, _ := json.Marshal(data)

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("particle API error: %s", string(body))
    }

    return nil
}

func getParticleDevices(token string) ([]map[string]interface{}, error) {
    url := fmt.Sprintf("%s/devices", particleAPIBase)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("particle API error: %s", string(body))
    }

    var devices []map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
        return nil, err
    }

    return devices, nil
}

func getParticleDeviceInfo(deviceID, token string) (map[string]interface{}, error) {
    url := fmt.Sprintf("%s/devices/%s", particleAPIBase, deviceID)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("particle API error: %s", string(body))
    }

    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result, nil
}

func main() {
    lambda.Start(handler)
}
