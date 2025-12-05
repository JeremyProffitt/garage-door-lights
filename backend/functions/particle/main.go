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
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/google/uuid"

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
	case deviceID != "" && method == "GET" && strings.HasSuffix(path, "/variables"):
		log.Printf("Routing to handleGetDeviceVariables for deviceID: %s", deviceID)
		return handleGetDeviceVariables(ctx, username, deviceID)
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
		if err := applyPatternToDevice(device, pattern, user.ParticleToken); err != nil {
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

	if cmdReq.Command == "" {
		log.Println("Command missing")
		return shared.CreateErrorResponse(400, "command is required"), nil
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
	log.Printf("=== handleRefreshDevices V2 (Fixed Double-Marshal Bug): Starting for user %s ===", username)

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

		// Check device readiness if online
		var isReady bool
		var firmwareVersion, platform string
		if connected {
			isReady, firmwareVersion, platform = checkDeviceReadiness(particleID, user.ParticleToken)
		}

		// Check if device already exists for this user
		log.Printf("Checking if device %s already exists for user %s", particleID, username)
		existingDevice, err := findDeviceByParticleID(ctx, username, particleID)
		if err != nil {
			log.Printf("Error checking for existing device: %v", err)
			continue
		}

		now := time.Now()

		if existingDevice != nil {
			// Update existing device
			log.Printf("Updating existing device: %s", existingDevice.DeviceID)
			existingDevice.Name = name
			existingDevice.IsOnline = connected
			existingDevice.IsReady = isReady
			existingDevice.FirmwareVersion = firmwareVersion
			existingDevice.Platform = platform
			if connected {
				existingDevice.LastSeen = now
			}
			existingDevice.UpdatedAt = now

			// Dereference pointer to pass value, not pointer
			deviceValue := *existingDevice
			log.Printf("About to PutItem - deviceValue type: %T, deviceId: %s, isReady: %v", deviceValue, deviceValue.DeviceID, deviceValue.IsReady)
			if err := shared.PutItem(ctx, devicesTable, deviceValue); err != nil {
				log.Printf("Failed to update device: %v", err)
				continue
			}
			log.Printf("Successfully updated device: %s", existingDevice.DeviceID)
		} else {
			// Create new device
			deviceID := uuid.New().String()
			log.Printf("Creating new device with ID: %s", deviceID)

			device := shared.Device{
				DeviceID:        deviceID,
				UserID:          username,
				Name:            name,
				ParticleID:      particleID,
				IsOnline:        connected,
				IsReady:         isReady,
				FirmwareVersion: firmwareVersion,
				Platform:        platform,
				LastSeen:        now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}

			log.Printf("About to PutItem - device type: %T, deviceId: %s, isReady: %v", device, device.DeviceID, device.IsReady)
			if err := shared.PutItem(ctx, devicesTable, device); err != nil {
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

func handleGetDeviceVariables(ctx context.Context, username string, deviceID string) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleGetDeviceVariables: Starting for user %s, deviceID %s ===", username, deviceID)

	// Get device
	deviceKey, _ := attributevalue.MarshalMap(map[string]string{
		"deviceId": deviceID,
	})

	var device shared.Device
	if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
		log.Printf("Database error fetching device: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if device.DeviceID == "" {
		return shared.CreateErrorResponse(404, "Device not found"), nil
	}

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

	// Read all firmware variables
	result := map[string]interface{}{
		"deviceId":   deviceID,
		"particleId": device.ParticleID,
		"name":       device.Name,
	}

	// Read deviceInfo variable: "version|platform|maxStrips|maxLeds"
	if deviceInfo, err := getParticleVariable(device.ParticleID, "deviceInfo", user.ParticleToken); err == nil {
		result["deviceInfo"] = deviceInfo
		parts := strings.Split(deviceInfo, "|")
		if len(parts) >= 4 {
			result["firmwareVersion"] = parts[0]
			result["platform"] = parts[1]
			if maxStrips, err := strconv.Atoi(parts[2]); err == nil {
				result["maxStrips"] = maxStrips
			}
			if maxLeds, err := strconv.Atoi(parts[3]); err == nil {
				result["maxLedsPerStrip"] = maxLeds
			}
		}
	} else {
		log.Printf("Failed to read deviceInfo: %v", err)
	}

	// Read numStrips variable
	if numStrips, err := getParticleVariable(device.ParticleID, "numStrips", user.ParticleToken); err == nil {
		if n, err := strconv.Atoi(numStrips); err == nil {
			result["numStrips"] = n
		}
	} else {
		log.Printf("Failed to read numStrips: %v", err)
	}

	// Read strips variable: "D6:8:1;D2:12:2" (pin:ledCount:pattern)
	if stripsStr, err := getParticleVariable(device.ParticleID, "strips", user.ParticleToken); err == nil {
		result["stripsRaw"] = stripsStr
		var strips []map[string]interface{}
		if stripsStr != "" {
			stripParts := strings.Split(stripsStr, ";")
			for _, sp := range stripParts {
				parts := strings.Split(sp, ":")
				if len(parts) >= 3 {
					strip := map[string]interface{}{}
					// Parse pin (e.g., "D6" -> 6)
					pinStr := parts[0]
					if strings.HasPrefix(pinStr, "D") {
						pinStr = pinStr[1:]
					}
					if pin, err := strconv.Atoi(pinStr); err == nil {
						strip["pin"] = pin
					}
					if ledCount, err := strconv.Atoi(parts[1]); err == nil {
						strip["ledCount"] = ledCount
					}
					if pattern, err := strconv.Atoi(parts[2]); err == nil {
						strip["pattern"] = pattern
					}
					strips = append(strips, strip)
				}
			}
		}
		result["strips"] = strips
	} else {
		log.Printf("Failed to read strips: %v", err)
		result["strips"] = []map[string]interface{}{}
	}

	log.Printf("Device variables retrieved successfully")
	return shared.CreateSuccessResponse(200, result), nil
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

func applyPatternToDevice(device shared.Device, pattern shared.Pattern, token string) error {
	log.Printf("=== applyPatternToDevice: device=%s, pattern=%s ===", device.Name, pattern.Name)

	// Convert pattern type to firmware pattern number
	patternMap := map[string]int{
		shared.PatternCandle:  1,
		shared.PatternSolid:   2,
		shared.PatternPulse:   3,
		shared.PatternWave:    4,
		shared.PatternRainbow: 5,
		shared.PatternFire:    6,
	}

	patternNum := patternMap[pattern.Type]
	log.Printf("Pattern type %s mapped to number %d", pattern.Type, patternNum)

	// If device has configured LED strips, apply pattern to each strip
	if len(device.LEDStrips) > 0 {
		log.Printf("Device has %d configured LED strips", len(device.LEDStrips))

		for _, strip := range device.LEDStrips {
			pin := strip.Pin
			log.Printf("Applying pattern to strip on pin D%d", pin)

			// Send pattern command: "pin,pattern,speed"
			patternArg := fmt.Sprintf("%d,%d,%d", pin, patternNum, pattern.Speed)
			log.Printf("Sending setPattern command with arg: %s", patternArg)
			if err := callParticleFunction(device.ParticleID, "setPattern", patternArg, token); err != nil {
				log.Printf("setPattern failed for pin D%d: %v", pin, err)
				return err
			}

			// Send color command: "pin,R,G,B"
			colorArg := fmt.Sprintf("%d,%d,%d,%d", pin, pattern.Red, pattern.Green, pattern.Blue)
			log.Printf("Sending setColor command with arg: %s", colorArg)
			if err := callParticleFunction(device.ParticleID, "setColor", colorArg, token); err != nil {
				log.Printf("setColor failed for pin D%d: %v", pin, err)
				return err
			}

			// Send brightness command: "pin,brightness"
			brightnessArg := fmt.Sprintf("%d,%d", pin, pattern.Brightness)
			log.Printf("Sending setBright command with arg: %s", brightnessArg)
			if err := callParticleFunction(device.ParticleID, "setBright", brightnessArg, token); err != nil {
				log.Printf("setBright failed for pin D%d: %v", pin, err)
				return err
			}
		}
	} else {
		// Fallback for devices without configured strips - apply to default pin 6
		log.Printf("No LED strips configured, using default pin D6")
		pin := 6

		// Send pattern command
		patternArg := fmt.Sprintf("%d,%d,%d", pin, patternNum, pattern.Speed)
		if err := callParticleFunction(device.ParticleID, "setPattern", patternArg, token); err != nil {
			log.Printf("setPattern failed: %v", err)
			return err
		}

		// Send color command
		colorArg := fmt.Sprintf("%d,%d,%d,%d", pin, pattern.Red, pattern.Green, pattern.Blue)
		if err := callParticleFunction(device.ParticleID, "setColor", colorArg, token); err != nil {
			log.Printf("setColor failed: %v", err)
			return err
		}

		// Send brightness command
		brightnessArg := fmt.Sprintf("%d,%d", pin, pattern.Brightness)
		if err := callParticleFunction(device.ParticleID, "setBright", brightnessArg, token); err != nil {
			log.Printf("setBright failed: %v", err)
			return err
		}
	}

	// Save configuration to flash
	log.Println("Sending saveConfig command")
	if err := callParticleFunction(device.ParticleID, "saveConfig", "1", token); err != nil {
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

// getParticleVariable gets a specific variable from a Particle device
func getParticleVariable(deviceID, variableName, token string) (string, error) {
	url := fmt.Sprintf("%s/devices/%s/%s", particleAPIBase, deviceID, variableName)

	log.Printf("Getting variable %s from device %s", variableName, deviceID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get variable: status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Variable value is in "result" field
	if val, ok := result["result"]; ok {
		switch v := val.(type) {
		case string:
			return v, nil
		case float64:
			return fmt.Sprintf("%.0f", v), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	}

	return "", fmt.Errorf("no result in response")
}

// checkDeviceReadiness checks if a device has valid firmware by reading deviceInfo variable
func checkDeviceReadiness(particleID, token string) (isReady bool, firmwareVersion, platform string) {
	deviceInfo, err := getParticleVariable(particleID, "deviceInfo", token)
	if err != nil {
		log.Printf("Device %s: could not read deviceInfo variable: %v", particleID, err)
		return false, "", ""
	}

	// deviceInfo format: "version|platform|maxStrips|maxLeds"
	parts := strings.Split(deviceInfo, "|")
	if len(parts) >= 2 {
		log.Printf("Device %s: firmware=%s, platform=%s", particleID, parts[0], parts[1])
		return true, parts[0], parts[1]
	}

	return false, "", ""
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
