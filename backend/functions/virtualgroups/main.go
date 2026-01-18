package main

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/google/uuid"

    "candle-lights/backend/shared"
)

var (
    virtualGroupsTable = os.Getenv("VIRTUAL_GROUPS_TABLE")
    devicesTable       = os.Getenv("DEVICES_TABLE")
    patternsTable      = os.Getenv("PATTERNS_TABLE")
    usersTable         = os.Getenv("USERS_TABLE")
)

const particleAPIBase = "https://api.particle.io/v1"

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Printf("=== VirtualGroups Handler Called ===")
    log.Printf("Path: %s", request.Path)
    log.Printf("Method: %s", request.HTTPMethod)
    log.Printf("PathParameters: %+v", request.PathParameters)

    // Validate authentication
    username, err := shared.ValidateAuth(ctx, request)
    if err != nil || username == "" {
        log.Printf("Authentication failed: err=%v, username=%s", err, username)
        return shared.CreateErrorResponse(401, "Unauthorized"), nil
    }

    log.Printf("Authenticated user: %s", username)

    path := request.Path
    method := request.HTTPMethod
    groupID := request.PathParameters["groupId"]

    switch {
    case path == "/api/virtual-groups" && method == "GET":
        log.Println("Routing to handleListGroups")
        return handleListGroups(ctx, username)
    case path == "/api/virtual-groups" && method == "POST":
        log.Println("Routing to handleCreateGroup")
        return handleCreateGroup(ctx, username, request)
    case groupID != "" && strings.HasSuffix(path, "/apply") && method == "POST":
        log.Printf("Routing to handleApplyPattern for groupId: %s", groupID)
        return handleApplyPattern(ctx, username, groupID, request)
    case groupID != "" && method == "GET":
        log.Printf("Routing to handleGetGroup for groupId: %s", groupID)
        return handleGetGroup(ctx, username, groupID)
    case groupID != "" && method == "PUT":
        log.Printf("Routing to handleUpdateGroup for groupId: %s", groupID)
        return handleUpdateGroup(ctx, username, groupID, request)
    case groupID != "" && method == "DELETE":
        log.Printf("Routing to handleDeleteGroup for groupId: %s", groupID)
        return handleDeleteGroup(ctx, username, groupID)
    default:
        log.Printf("No matching route for path: %s, method: %s", path, method)
        return shared.CreateErrorResponse(404, "Not found"), nil
    }
}

func handleListGroups(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
    indexName := "userId-index"
    keyCondition := "userId = :userId"
    expressionValues := map[string]types.AttributeValue{
        ":userId": &types.AttributeValueMemberS{Value: username},
    }

    var groups []shared.VirtualGroup
    if err := shared.Query(ctx, virtualGroupsTable, &indexName, keyCondition, expressionValues, &groups); err != nil {
        log.Printf("Failed to query virtual groups: %v", err)
        return shared.CreateErrorResponse(500, "Failed to retrieve virtual groups"), nil
    }

    return shared.CreateSuccessResponse(200, groups), nil
}

func handleCreateGroup(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var groupReq struct {
        Name    string                     `json:"name"`
        Members []shared.VirtualGroupMember `json:"members"`
    }

    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &groupReq); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    if groupReq.Name == "" {
        return shared.CreateErrorResponse(400, "Name is required"), nil
    }

    if len(groupReq.Members) == 0 {
        return shared.CreateErrorResponse(400, "At least one member is required"), nil
    }

    // Validate that all devices belong to the user
    for _, member := range groupReq.Members {
        deviceKey, _ := attributevalue.MarshalMap(map[string]string{
            "deviceId": member.DeviceID,
        })

        var device shared.Device
        if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
            log.Printf("Failed to get device %s: %v", member.DeviceID, err)
            return shared.CreateErrorResponse(500, "Database error"), nil
        }

        if device.DeviceID == "" {
            return shared.CreateErrorResponse(400, fmt.Sprintf("Device %s not found", member.DeviceID)), nil
        }

        if device.UserID != username {
            return shared.CreateErrorResponse(403, fmt.Sprintf("Access denied to device %s", member.DeviceID)), nil
        }
    }

    now := time.Now()
    group := shared.VirtualGroup{
        GroupID:   uuid.New().String(),
        UserID:    username,
        Name:      groupReq.Name,
        Members:   groupReq.Members,
        CreatedAt: now,
        UpdatedAt: now,
    }

    if err := shared.PutItem(ctx, virtualGroupsTable, group); err != nil {
        log.Printf("Failed to create virtual group: %v", err)
        return shared.CreateErrorResponse(500, "Failed to create virtual group"), nil
    }

    return shared.CreateSuccessResponse(201, group), nil
}

func handleGetGroup(ctx context.Context, username string, groupID string) (events.APIGatewayProxyResponse, error) {
    key, _ := attributevalue.MarshalMap(map[string]string{
        "groupId": groupID,
    })

    var group shared.VirtualGroup
    if err := shared.GetItem(ctx, virtualGroupsTable, key, &group); err != nil {
        log.Printf("Failed to get virtual group: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if group.GroupID == "" {
        return shared.CreateErrorResponse(404, "Virtual group not found"), nil
    }

    if group.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    return shared.CreateSuccessResponse(200, group), nil
}

func handleUpdateGroup(ctx context.Context, username string, groupID string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Get existing group
    key, _ := attributevalue.MarshalMap(map[string]string{
        "groupId": groupID,
    })

    var existingGroup shared.VirtualGroup
    if err := shared.GetItem(ctx, virtualGroupsTable, key, &existingGroup); err != nil {
        log.Printf("Failed to get virtual group: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if existingGroup.GroupID == "" {
        return shared.CreateErrorResponse(404, "Virtual group not found"), nil
    }

    if existingGroup.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Parse updates
    var updates struct {
        Name    string                      `json:"name,omitempty"`
        Members []shared.VirtualGroupMember `json:"members,omitempty"`
    }

    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &updates); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    // Update fields
    if updates.Name != "" {
        existingGroup.Name = updates.Name
    }

    if updates.Members != nil {
        if len(updates.Members) == 0 {
            return shared.CreateErrorResponse(400, "At least one member is required"), nil
        }

        // Validate new members
        for _, member := range updates.Members {
            deviceKey, _ := attributevalue.MarshalMap(map[string]string{
                "deviceId": member.DeviceID,
            })

            var device shared.Device
            if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
                log.Printf("Failed to get device %s: %v", member.DeviceID, err)
                return shared.CreateErrorResponse(500, "Database error"), nil
            }

            if device.DeviceID == "" {
                return shared.CreateErrorResponse(400, fmt.Sprintf("Device %s not found", member.DeviceID)), nil
            }

            if device.UserID != username {
                return shared.CreateErrorResponse(403, fmt.Sprintf("Access denied to device %s", member.DeviceID)), nil
            }
        }

        existingGroup.Members = updates.Members
    }

    existingGroup.UpdatedAt = time.Now()

    if err := shared.PutItem(ctx, virtualGroupsTable, existingGroup); err != nil {
        log.Printf("Failed to update virtual group: %v", err)
        return shared.CreateErrorResponse(500, "Failed to update virtual group"), nil
    }

    return shared.CreateSuccessResponse(200, existingGroup), nil
}

func handleDeleteGroup(ctx context.Context, username string, groupID string) (events.APIGatewayProxyResponse, error) {
    // Get group to verify ownership
    key, _ := attributevalue.MarshalMap(map[string]string{
        "groupId": groupID,
    })

    var group shared.VirtualGroup
    if err := shared.GetItem(ctx, virtualGroupsTable, key, &group); err != nil {
        log.Printf("Failed to get virtual group: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if group.GroupID == "" {
        return shared.CreateErrorResponse(404, "Virtual group not found"), nil
    }

    if group.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Delete group
    if err := shared.DeleteItem(ctx, virtualGroupsTable, key); err != nil {
        log.Printf("Failed to delete virtual group: %v", err)
        return shared.CreateErrorResponse(500, "Failed to delete virtual group"), nil
    }

    return shared.CreateSuccessResponse(200, map[string]string{
        "message": "Virtual group deleted successfully",
    }), nil
}

// MemberResult represents the result of applying a pattern to a single member
type MemberResult struct {
    DeviceID   string `json:"deviceId"`
    DeviceName string `json:"deviceName"`
    Pin        int    `json:"pin"`
    Success    bool   `json:"success"`
    Error      string `json:"error,omitempty"`
}

// ApplyResult represents the aggregated result of applying a pattern to all members
type ApplyResult struct {
    Success    bool           `json:"success"`
    Message    string         `json:"message"`
    PatternID  string         `json:"patternId"`
    Results    []MemberResult `json:"results"`
    Succeeded  int            `json:"succeeded"`
    Failed     int            `json:"failed"`
}

func handleApplyPattern(ctx context.Context, username string, groupID string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Printf("=== handleApplyPattern: Starting for user %s, groupId %s ===", username, groupID)

    // Parse request
    var applyReq struct {
        PatternID string `json:"patternId"`
    }

    body := shared.GetRequestBody(request)
    if err := json.Unmarshal([]byte(body), &applyReq); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    if applyReq.PatternID == "" {
        return shared.CreateErrorResponse(400, "patternId is required"), nil
    }

    // Get group
    groupKey, _ := attributevalue.MarshalMap(map[string]string{
        "groupId": groupID,
    })

    var group shared.VirtualGroup
    if err := shared.GetItem(ctx, virtualGroupsTable, groupKey, &group); err != nil {
        log.Printf("Failed to get virtual group: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if group.GroupID == "" {
        return shared.CreateErrorResponse(404, "Virtual group not found"), nil
    }

    if group.UserID != username {
        return shared.CreateErrorResponse(403, "Access denied"), nil
    }

    // Get pattern
    patternKey, _ := attributevalue.MarshalMap(map[string]string{
        "patternId": applyReq.PatternID,
    })

    var pattern shared.Pattern
    if err := shared.GetItem(ctx, patternsTable, patternKey, &pattern); err != nil {
        log.Printf("Failed to get pattern: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if pattern.PatternID == "" {
        return shared.CreateErrorResponse(404, "Pattern not found"), nil
    }

    if pattern.UserID != username {
        return shared.CreateErrorResponse(403, "Pattern access denied"), nil
    }

    // Get user's Particle token
    userKey, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
        log.Printf("Failed to get user: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if user.ParticleToken == "" {
        return shared.CreateErrorResponse(400, "Particle token not configured"), nil
    }

    // Apply pattern to each member
    results := make([]MemberResult, 0, len(group.Members))
    succeeded := 0
    failed := 0

    // Cache devices to avoid repeated lookups
    deviceCache := make(map[string]*shared.Device)

    for _, member := range group.Members {
        log.Printf("Processing member: deviceId=%s, pin=%d", member.DeviceID, member.Pin)

        // Get device (with caching)
        device, ok := deviceCache[member.DeviceID]
        if !ok {
            deviceKey, _ := attributevalue.MarshalMap(map[string]string{
                "deviceId": member.DeviceID,
            })

            var d shared.Device
            if err := shared.GetItem(ctx, devicesTable, deviceKey, &d); err != nil {
                log.Printf("Failed to get device %s: %v", member.DeviceID, err)
                results = append(results, MemberResult{
                    DeviceID: member.DeviceID,
                    Pin:      member.Pin,
                    Success:  false,
                    Error:    "Database error",
                })
                failed++
                continue
            }
            device = &d
            deviceCache[member.DeviceID] = device
        }

        if device.DeviceID == "" {
            results = append(results, MemberResult{
                DeviceID: member.DeviceID,
                Pin:      member.Pin,
                Success:  false,
                Error:    "Device not found",
            })
            failed++
            continue
        }

        if device.UserID != username {
            results = append(results, MemberResult{
                DeviceID:   device.DeviceID,
                DeviceName: device.Name,
                Pin:        member.Pin,
                Success:    false,
                Error:      "Access denied",
            })
            failed++
            continue
        }

        if !device.IsOnline {
            results = append(results, MemberResult{
                DeviceID:   device.DeviceID,
                DeviceName: device.Name,
                Pin:        member.Pin,
                Success:    false,
                Error:      "Device is offline",
            })
            failed++
            continue
        }

        // Find the strip for this pin to get LED count
        var ledCount int = 8 // default
        for _, strip := range device.LEDStrips {
            if strip.Pin == member.Pin {
                ledCount = strip.LEDCount
                break
            }
        }

        // Compile and send pattern
        err := compileAndSendPattern(device, member.Pin, pattern, ledCount, user.ParticleToken)
        if err != nil {
            log.Printf("Failed to apply pattern to device %s pin %d: %v", device.Name, member.Pin, err)
            results = append(results, MemberResult{
                DeviceID:   device.DeviceID,
                DeviceName: device.Name,
                Pin:        member.Pin,
                Success:    false,
                Error:      err.Error(),
            })
            failed++
            continue
        }

        // Update strip's patternId in device
        stripUpdated := false
        for i, strip := range device.LEDStrips {
            if strip.Pin == member.Pin {
                device.LEDStrips[i].PatternID = applyReq.PatternID
                stripUpdated = true
                break
            }
        }

        if stripUpdated {
            device.UpdatedAt = time.Now()
            if err := shared.PutItem(ctx, devicesTable, *device); err != nil {
                log.Printf("Warning: Failed to update device %s strip patternId: %v", device.DeviceID, err)
            }
        }

        results = append(results, MemberResult{
            DeviceID:   device.DeviceID,
            DeviceName: device.Name,
            Pin:        member.Pin,
            Success:    true,
        })
        succeeded++
    }

    // Update group's patternId
    group.PatternID = applyReq.PatternID
    group.UpdatedAt = time.Now()
    if err := shared.PutItem(ctx, virtualGroupsTable, group); err != nil {
        log.Printf("Warning: Failed to update group patternId: %v", err)
    }

    result := ApplyResult{
        Success:   failed == 0,
        PatternID: applyReq.PatternID,
        Results:   results,
        Succeeded: succeeded,
        Failed:    failed,
    }

    if failed == 0 {
        result.Message = fmt.Sprintf("Pattern applied successfully to all %d members", succeeded)
    } else if succeeded == 0 {
        result.Message = fmt.Sprintf("Pattern failed to apply to all %d members", failed)
    } else {
        result.Message = fmt.Sprintf("Pattern applied to %d members, failed on %d members", succeeded, failed)
    }

    return shared.CreateSuccessResponse(200, result), nil
}

func compileAndSendPattern(device *shared.Device, pin int, pattern shared.Pattern, ledCount int, token string) error {
    var bytecode []byte

    // If pattern has WLED JSON state, parse it, update LED count, and recompile
    if pattern.WLEDState != "" {
        log.Printf("[compileAndSendPattern] Using WLED state for pattern %s", pattern.Name)
        var wledJson map[string]interface{}
        if err := json.Unmarshal([]byte(pattern.WLEDState), &wledJson); err != nil {
            return fmt.Errorf("failed to parse WLED state: %v", err)
        }

        // Update all segment stop values to match device LED count
        if segs, ok := wledJson["seg"].([]interface{}); ok {
            for _, seg := range segs {
                if segMap, ok := seg.(map[string]interface{}); ok {
                    segMap["stop"] = ledCount
                }
            }
        }

        updatedWledState, _ := json.Marshal(wledJson)
        var err error
        bytecode, _, err = shared.CompileWLED(string(updatedWledState))
        if err != nil {
            return fmt.Errorf("failed to compile WLED: %v", err)
        }
    } else {
        // Build WLED JSON from pattern fields (legacy patterns)
        effectMap := map[string]int{
            "solid":   0,
            "pulse":   2,
            "wave":    67,
            "rainbow": 9,
            "fire":    66,
            "candle":  71,
        }

        effectId := effectMap[pattern.Type]
        if pattern.Metadata != nil {
            if eid, ok := pattern.Metadata["effectId"]; ok {
                fmt.Sscanf(eid, "%d", &effectId)
            }
        }

        speed := 128
        intensity := 128
        custom1 := 128

        if pattern.Metadata != nil {
            if s, ok := pattern.Metadata["speed"]; ok {
                fmt.Sscanf(s, "%d", &speed)
            }
            if i, ok := pattern.Metadata["intensity"]; ok {
                fmt.Sscanf(i, "%d", &intensity)
            }
            if c, ok := pattern.Metadata["custom1"]; ok {
                fmt.Sscanf(c, "%d", &custom1)
            }
        }

        // Build colors array
        var colors [][]int
        if len(pattern.Colors) > 0 {
            for _, c := range pattern.Colors {
                colors = append(colors, []int{clamp(c.R), clamp(c.G), clamp(c.B)})
            }
        } else {
            colors = [][]int{{clamp(pattern.Red), clamp(pattern.Green), clamp(pattern.Blue)}}
        }

        wledJson := map[string]interface{}{
            "on":  true,
            "bri": clamp(pattern.Brightness),
            "seg": []map[string]interface{}{
                {
                    "id":    0,
                    "start": 0,
                    "stop":  ledCount,
                    "fx":    effectId,
                    "sx":    clamp(speed),
                    "ix":    clamp(intensity),
                    "c1":    clamp(custom1),
                    "col":   colors,
                    "on":    true,
                },
            },
        }

        wledJsonBytes, _ := json.Marshal(wledJson)
        var err error
        bytecode, _, err = shared.CompileWLED(string(wledJsonBytes))
        if err != nil {
            return fmt.Errorf("failed to compile WLED: %v", err)
        }
    }

    // Send bytecode to device
    return sendBytecodeToDevice(device.ParticleID, pin, bytecode, token)
}

func clamp(val int) int {
    if val < 0 {
        return 0
    }
    if val > 255 {
        return 255
    }
    return val
}

func sendBytecodeToDevice(particleID string, pin int, bytecode []byte, token string) error {
    // Base64 encode the bytecode
    encoded := base64.StdEncoding.EncodeToString(bytecode)
    argument := fmt.Sprintf("%d,%s", pin, encoded)

    return callParticleFunction(particleID, "setBytecode", argument, token)
}

func callParticleFunction(deviceID, functionName, argument, token string) error {
    url := fmt.Sprintf("%s/devices/%s/%s", particleAPIBase, deviceID, functionName)

    log.Printf("Calling Particle function: %s on device %s", functionName, deviceID)

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

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("Particle API error (status %d): %s", resp.StatusCode, string(body))
    }

    return nil
}

func main() {
    lambda.Start(handler)
}
