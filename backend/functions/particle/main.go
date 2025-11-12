package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

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
	log.Printf("=== Particle Handler Called ===")
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
	deviceID := request.PathParameters["deviceId"]

	switch {
	case path == "/api/particle/command" && method == "POST":
		log.Println("Routing to handleSendCommand")
		return handleSendCommand(ctx, username, request)
	case path == "/api/particle/devices/refresh" && method == "POST":
		log.Println("Routing to handleRefreshDevices")
		return handleRefreshDevices(ctx, username)
	case path == "/api/particle/validate-token" && method == "POST":
		log.Println("Routing to handleValidateToken")
		return handleValidateToken(ctx, username, request)
	case path == "/api/particle/oauth/initiate" && method == "POST":
		log.Println("Routing to handleOAuthInitiate")
		return handleOAuthInitiate(ctx, username)
	case deviceID != "" && method == "GET":
		log.Printf("Routing to handleGetDeviceInfo for deviceID: %s", deviceID)
		return handleGetDeviceInfo(ctx, username, deviceID)
	default:
		log.Printf("No matching route for path: %s, method: %s", path, method)
		return shared.CreateErrorResponse(404, "Not found"), nil
	}
}

func handleSendCommand(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleSendCommand: Starting for user %s ===", username)

	var cmdReq struct {
		DeviceID  string `json:"deviceId"`
		PatternID string `json:"patternId,omitempty"`
		Command   string `json:"command,omitempty"`
		Argument  string `json:"argument,omitempty"`
	}

	body := shared.GetRequestBody(request)
	log.Printf("Request body: %s", body)

	if err := json.Unmarshal([]byte(body), &cmdReq); err != nil {
		log.Printf("Failed to unmarshal request: %v", err)
		return shared.CreateErrorResponse(400, "Invalid request body"), nil
	}

	log.Printf("Parsed command request: deviceId=%s, patternId=%s, command=%s",
		cmdReq.DeviceID, cmdReq.PatternID, cmdReq.Command)

	if cmdReq.DeviceID == "" {
		log.Println("deviceId is missing")
		return shared.CreateErrorResponse(400, "deviceId is required"), nil
	}

	// Get device
	log.Printf("Fetching device from DynamoDB: %s", cmdReq.DeviceID)
	deviceKey, _ := attributevalue.MarshalMap(map[string]string{
		"deviceId": cmdReq.DeviceID,
	})

	var device shared.Device
	if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
		log.Printf("Database error fetching device: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if device.DeviceID == "" {
		log.Printf("Device not found: %s", cmdReq.DeviceID)
		return shared.CreateErrorResponse(404, "Device not found"), nil
	}

	log.Printf("Found device: %s (particleId=%s)", device.Name, device.ParticleID)

	// Verify ownership
	if device.UserID != username {
		log.Printf("Access denied: device belongs to %s, not %s", device.UserID, username)
		return shared.CreateErrorResponse(403, "Access denied"), nil
	}

	// Get user's Particle token
	log.Printf("Fetching user from DynamoDB: %s", username)
	userKey, _ := attributevalue.MarshalMap(map[string]string{
		"username": username,
	})

	var user shared.User
	if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
		log.Printf("Database error fetching user: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if user.ParticleToken == "" {
		log.Printf("User %s has no Particle token configured", username)
		return shared.CreateErrorResponse(400, "Particle token not configured"), nil
	}

	log.Printf("User has Particle token configured (length: %d chars)", len(user.ParticleToken))

	// If PatternID is provided, get pattern and send to device
	if cmdReq.PatternID != "" {
		log.Printf("Pattern ID provided: %s", cmdReq.PatternID)

		patternKey, _ := attributevalue.MarshalMap(map[string]string{
			"patternId": cmdReq.PatternID,
		})

		var pattern shared.Pattern
		if err := shared.GetItem(ctx, patternsTable, patternKey, &pattern); err != nil {
			log.Printf("Database error fetching pattern: %v", err)
			return shared.CreateErrorResponse(500, "Database error"), nil
		}

		if pattern.PatternID == "" {
			log.Printf("Pattern not found: %s", cmdReq.PatternID)
			return shared.CreateErrorResponse(404, "Pattern not found"), nil
		}

		log.Printf("Found pattern: %s (type=%s, r=%d, g=%d, b=%d, brightness=%d, speed=%d)",
			pattern.Name, pattern.Type, pattern.Red, pattern.Green, pattern.Blue, pattern.Brightness, pattern.Speed)

		if pattern.UserID != username {
			log.Printf("Pattern access denied: pattern belongs to %s, not %s", pattern.UserID, username)
			return shared.CreateErrorResponse(403, "Pattern access denied"), nil
		}

		// Apply pattern to device
		log.Printf("Applying pattern to device...")
		if err := applyPatternToDevice(device.ParticleID, pattern, user.ParticleToken); err != nil {
			log.Printf("Failed to apply pattern: %v", err)
			return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to apply pattern: %v", err)), nil
		}

		log.Printf("Successfully applied pattern %s to device %s", pattern.Name, device.Name)
		return shared.CreateSuccessResponse(200, map[string]string{
			"message": "Pattern applied successfully",
			"device":  device.Name,
			"pattern": pattern.Name,
		}), nil
	}

	// Otherwise, send custom command
	log.Printf("No pattern ID, sending custom command: %s with argument: %s", cmdReq.Command, cmdReq.Argument)

	if cmdReq.Command == "" || cmdReq.Argument == "" {
		log.Println("Command or argument missing")
		return shared.CreateErrorResponse(400, "command and argument are required"), nil
	}

	if err := callParticleFunction(device.ParticleID, cmdReq.Command, cmdReq.Argument, user.ParticleToken); err != nil {
		log.Printf("Failed to send command: %v", err)
		return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to send command: %v", err)), nil
	}

	log.Printf("Successfully sent command %s to device %s", cmdReq.Command, device.Name)
	return shared.CreateSuccessResponse(200, map[string]string{
		"message": "Command sent successfully",
	}), nil
}

func handleRefreshDevices(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleRefreshDevices: Starting for user %s ===", username)

	// Get user's Particle token
	log.Printf("Fetching user from DynamoDB: %s", username)
	userKey, _ := attributevalue.MarshalMap(map[string]string{
		"username": username,
	})

	var user shared.User
	if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
		log.Printf("Database error fetching user: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if user.ParticleToken == "" {
		log.Printf("User %s has no Particle token configured", username)
		return shared.CreateErrorResponse(400, "Particle token not configured"), nil
	}

	log.Printf("User has Particle token configured (length: %d chars)", len(user.ParticleToken))
	log.Printf("Particle token (first 10 chars): %s...", safeTokenDisplay(user.ParticleToken))

	// Get devices from Particle cloud
	log.Println("Calling Particle API to get devices...")
	particleDevices, err := getParticleDevices(user.ParticleToken)
	if err != nil {
		log.Printf("Failed to get devices from Particle: %v", err)
		return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to get devices from Particle: %v", err)), nil
	}

	log.Printf("Successfully retrieved %d devices from Particle", len(particleDevices))
	for i, dev := range particleDevices {
		log.Printf("  Device %d: id=%v, name=%v, connected=%v",
			i+1, dev["id"], dev["name"], dev["connected"])
	}

	// Save devices to DynamoDB
	savedCount := 0
	for _, particleDev := range particleDevices {
		particleID, ok := particleDev["id"].(string)
		if !ok || particleID == "" {
			log.Printf("Skipping device with invalid ID: %v", particleDev)
			continue
		}

		name, _ := particleDev["name"].(string)
		if name == "" {
			name = particleID // Use Particle ID as fallback name
		}

		connected, _ := particleDev["connected"].(bool)

		// Check if device already exists for this user
		log.Printf("Checking if device %s already exists for user %s", particleID, username)
		existingDevice, err := findDeviceByParticleID(ctx, username, particleID)
		if err != nil {
			log.Printf("Error checking for existing device: %v", err)
			continue
		}

		now := shared.GetCurrentTime()

		if existingDevice != nil {
			// Update existing device
			log.Printf("Updating existing device: %s", existingDevice.DeviceID)
			existingDevice.Name = name
			existingDevice.IsOnline = connected
			if connected {
				existingDevice.LastSeen = now
			}
			existingDevice.UpdatedAt = now

			deviceMap, err := attributevalue.MarshalMap(existingDevice)
			if err != nil {
				log.Printf("Failed to marshal device: %v", err)
				continue
			}

			if err := shared.PutItem(ctx, devicesTable, deviceMap); err != nil {
				log.Printf("Failed to update device: %v", err)
				continue
			}
			log.Printf("Successfully updated device: %s", existingDevice.DeviceID)
		} else {
			// Create new device
			deviceID := shared.GenerateID()
			log.Printf("Creating new device with ID: %s", deviceID)

			device := shared.Device{
				DeviceID:   deviceID,
				UserID:     username,
				Name:       name,
				ParticleID: particleID,
				IsOnline:   connected,
				LastSeen:   now,
				CreatedAt:  now,
				UpdatedAt:  now,
			}

			deviceMap, err := attributevalue.MarshalMap(device)
			if err != nil {
				log.Printf("Failed to marshal new device: %v", err)
				continue
			}

			if err := shared.PutItem(ctx, devicesTable, deviceMap); err != nil {
				log.Printf("Failed to save new device: %v", err)
				continue
			}
			log.Printf("Successfully created device: %s", deviceID)
		}
		savedCount++
	}

	log.Printf("Saved %d devices to database", savedCount)

	return shared.CreateSuccessResponse(200, map[string]interface{}{
		"count":   savedCount,
		"devices": particleDevices,
	}), nil
}

func handleGetDeviceInfo(ctx context.Context, username string, deviceID string) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleGetDeviceInfo: Starting for user %s, deviceID %s ===", username, deviceID)

	// Get device
	log.Printf("Fetching device from DynamoDB: %s", deviceID)
	deviceKey, _ := attributevalue.MarshalMap(map[string]string{
		"deviceId": deviceID,
	})

	var device shared.Device
	if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
		log.Printf("Database error fetching device: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if device.DeviceID == "" {
		log.Printf("Device not found: %s", deviceID)
		return shared.CreateErrorResponse(404, "Device not found"), nil
	}

	log.Printf("Found device: %s (particleId=%s)", device.Name, device.ParticleID)

	// Verify ownership
	if device.UserID != username {
		log.Printf("Access denied: device belongs to %s, not %s", device.UserID, username)
		return shared.CreateErrorResponse(403, "Access denied"), nil
	}

	// Get user's Particle token
	log.Printf("Fetching user from DynamoDB: %s", username)
	userKey, _ := attributevalue.MarshalMap(map[string]string{
		"username": username,
	})

	var user shared.User
	if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
		log.Printf("Database error fetching user: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if user.ParticleToken == "" {
		log.Printf("User %s has no Particle token configured", username)
		return shared.CreateErrorResponse(400, "Particle token not configured"), nil
	}

	log.Printf("User has Particle token configured (length: %d chars)", len(user.ParticleToken))

	// Get device info from Particle cloud
	log.Printf("Calling Particle API to get device info for: %s", device.ParticleID)
	info, err := getParticleDeviceInfo(device.ParticleID, user.ParticleToken)
	if err != nil {
		log.Printf("Failed to get device info: %v", err)
		return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to get device info: %v", err)), nil
	}

	log.Printf("Successfully retrieved device info from Particle")
	return shared.CreateSuccessResponse(200, info), nil
}

func applyPatternToDevice(particleID string, pattern shared.Pattern, token string) error {
	log.Printf("=== applyPatternToDevice: particleID=%s, pattern=%s ===", particleID, pattern.Name)

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
	log.Printf("Pattern type %s mapped to number %s", pattern.Type, patternNum)

	// Send pattern command
	patternArg := fmt.Sprintf("%s:%d", patternNum, pattern.Speed)
	log.Printf("Sending setPattern command with arg: %s", patternArg)
	if err := callParticleFunction(particleID, "setPattern", patternArg, token); err != nil {
		log.Printf("setPattern failed: %v", err)
		return err
	}

	// Send color command
	colorArg := fmt.Sprintf("%d,%d,%d", pattern.Red, pattern.Green, pattern.Blue)
	log.Printf("Sending setColor command with arg: %s", colorArg)
	if err := callParticleFunction(particleID, "setColor", colorArg, token); err != nil {
		log.Printf("setColor failed: %v", err)
		return err
	}

	// Send brightness command
	brightnessArg := fmt.Sprintf("%d", pattern.Brightness)
	log.Printf("Sending setBright command with arg: %s", brightnessArg)
	if err := callParticleFunction(particleID, "setBright", brightnessArg, token); err != nil {
		log.Printf("setBright failed: %v", err)
		return err
	}

	// Save configuration to flash
	log.Println("Sending saveConfig command")
	if err := callParticleFunction(particleID, "saveConfig", "1", token); err != nil {
		log.Printf("saveConfig failed: %v", err)
		return err
	}

	log.Println("Pattern applied successfully")
	return nil
}

func callParticleFunction(deviceID, functionName, argument, token string) error {
	url := fmt.Sprintf("%s/devices/%s/%s", particleAPIBase, deviceID, functionName)

	log.Printf("=== callParticleFunction ===")
	log.Printf("URL: %s", url)
	log.Printf("Function: %s", functionName)
	log.Printf("Argument: %s", argument)
	log.Printf("Token (first 10 chars): %s...", safeTokenDisplay(token))

	data := map[string]string{
		"arg": argument,
	}
	jsonData, _ := json.Marshal(data)
	log.Printf("Request body: %s", string(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	log.Printf("Request headers: Content-Type=application/json, Authorization=Bearer %s...", safeTokenDisplay(token))

	client := &http.Client{}
	log.Println("Sending HTTP request to Particle API...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	log.Printf("Response status: %d %s", resp.StatusCode, resp.Status)

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Particle API error (status %d): %s", resp.StatusCode, string(body))
		log.Printf("ERROR: %s", errMsg)
		return fmt.Errorf(errMsg)
	}

	log.Println("Particle function call successful")
	return nil
}

func getParticleDevices(token string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/devices", particleAPIBase)

	log.Printf("=== getParticleDevices ===")
	log.Printf("URL: %s", url)
	log.Printf("Token (first 10 chars): %s...", safeTokenDisplay(token))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	log.Printf("Request headers: Authorization=Bearer %s...", safeTokenDisplay(token))

	client := &http.Client{}
	log.Println("Sending HTTP request to Particle API...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Response status: %d %s", resp.StatusCode, resp.Status)

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Particle API error (status %d): %s", resp.StatusCode, string(body))
		log.Printf("ERROR: %s", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	var devices []map[string]interface{}
	if err := json.Unmarshal(body, &devices); err != nil {
		log.Printf("Failed to parse response JSON: %v", err)
		return nil, err
	}

	log.Printf("Successfully parsed %d devices from response", len(devices))
	return devices, nil
}

func getParticleDeviceInfo(deviceID, token string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/devices/%s", particleAPIBase, deviceID)

	log.Printf("=== getParticleDeviceInfo ===")
	log.Printf("URL: %s", url)
	log.Printf("Device ID: %s", deviceID)
	log.Printf("Token (first 10 chars): %s...", safeTokenDisplay(token))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	log.Printf("Request headers: Authorization=Bearer %s...", safeTokenDisplay(token))

	client := &http.Client{}
	log.Println("Sending HTTP request to Particle API...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Response status: %d %s", resp.StatusCode, resp.Status)

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Particle API error (status %d): %s", resp.StatusCode, string(body))
		log.Printf("ERROR: %s", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Failed to parse response JSON: %v", err)
		return nil, err
	}

	log.Println("Successfully parsed device info from response")
	return result, nil
}

// safeTokenDisplay returns the first N characters of a token for logging
func safeTokenDisplay(token string) string {
	if len(token) <= 10 {
		return strings.Repeat("*", len(token))
	}
	return token[:10]
}

func handleValidateToken(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleValidateToken: Starting for user %s ===", username)

	var req struct {
		ParticleToken string `json:"particleToken"`
	}

	body := shared.GetRequestBody(request)
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		log.Printf("Failed to parse request body: %v", err)
		return shared.CreateErrorResponse(400, "Invalid request body"), nil
	}

	if req.ParticleToken == "" {
		log.Println("No token provided in request")
		return shared.CreateErrorResponse(400, "Particle token is required"), nil
	}

	log.Printf("Validating token (first 10 chars): %s...", safeTokenDisplay(req.ParticleToken))

	// Try to get devices from Particle API to validate the token
	devices, err := getParticleDevices(req.ParticleToken)
	if err != nil {
		log.Printf("Token validation failed: %v", err)
		return shared.CreateErrorResponse(401, "Invalid Particle token"), nil
	}

	log.Printf("Token validation successful! Found %d devices", len(devices))

	return shared.CreateSuccessResponse(200, map[string]interface{}{
		"message": fmt.Sprintf("Token is valid! Found %d device(s)", len(devices)),
		"devices": len(devices),
	}), nil
}

func handleOAuthInitiate(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleOAuthInitiate: Starting for user %s ===", username)

	// Particle OAuth configuration
	clientID := os.Getenv("PARTICLE_CLIENT_ID")
	redirectURI := os.Getenv("PARTICLE_REDIRECT_URI")

	if clientID == "" || redirectURI == "" {
		log.Println("Particle OAuth not configured - missing CLIENT_ID or REDIRECT_URI")
		return shared.CreateErrorResponse(500, "Particle OAuth is not configured. Please use the manual token entry method."), nil
	}

	// Build OAuth authorization URL
	authURL := fmt.Sprintf(
		"https://login.particle.io/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=token&scope=*",
		clientID,
		redirectURI,
	)

	log.Printf("Generated OAuth URL (client_id=%s)", safeTokenDisplay(clientID))

	return shared.CreateSuccessResponse(200, map[string]string{
		"authUrl": authURL,
	}), nil
}

// findDeviceByParticleID searches for an existing device by Particle ID for a given user
func findDeviceByParticleID(ctx context.Context, username, particleID string) (*shared.Device, error) {
	log.Printf("=== findDeviceByParticleID: username=%s, particleID=%s ===", username, particleID)

	// Scan the devices table for a device with this Particle ID and user ID
	var devices []shared.Device
	if err := shared.Scan(ctx, devicesTable, &devices); err != nil {
		log.Printf("Failed to scan devices table: %v", err)
		return nil, err
	}

	for _, device := range devices {
		if device.UserID == username && device.ParticleID == particleID {
			log.Printf("Found existing device: deviceID=%s, particleID=%s", device.DeviceID, device.ParticleID)
			return &device, nil
		}
	}

	log.Printf("No existing device found for particleID=%s", particleID)
	return nil, nil
}

func main() {
	lambda.Start(handler)
}
