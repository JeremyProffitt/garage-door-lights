package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/google/uuid"

	"candle-lights/backend/shared"
)

var (
	devicesTable  = os.Getenv("DEVICES_TABLE")
	usersTable    = os.Getenv("USERS_TABLE")
	alexaSkillID  = os.Getenv("ALEXA_SKILL_ID")
)

func handler(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== Alexa Handler Called ===")
	log.Printf("Namespace: %s", request.Directive.Header.Namespace)
	log.Printf("Name: %s", request.Directive.Header.Name)
	log.Printf("MessageID: %s", request.Directive.Header.MessageID)

	namespace := request.Directive.Header.Namespace
	name := request.Directive.Header.Name

	switch namespace {
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
		if name == "ReportState" {
			return handleReportState(ctx, request)
		}
	case "Alexa.Authorization":
		if name == "AcceptGrant" {
			return handleAcceptGrant(ctx, request)
		}
	}

	log.Printf("Unsupported directive: %s/%s", namespace, name)
	return createErrorResponse(request, "INVALID_DIRECTIVE", "Unsupported directive")
}

// handleDiscovery returns all user's devices to Alexa
func handleDiscovery(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== handleDiscovery ===")

	// Extract bearer token from scope
	token := ""
	if payload, ok := request.Directive.Payload.(map[string]interface{}); ok {
		if scope, ok := payload["scope"].(map[string]interface{}); ok {
			token, _ = scope["token"].(string)
		}
	}

	if token == "" {
		log.Printf("No token in discovery request")
		return createErrorResponse(request, "INVALID_AUTHORIZATION_CREDENTIAL", "Missing authorization token")
	}

	// Validate token and get user
	userID, err := shared.ValidateAccessToken(ctx, token)
	if err != nil || userID == "" {
		log.Printf("Invalid token: %v", err)
		return createErrorResponse(request, "INVALID_AUTHORIZATION_CREDENTIAL", "Invalid authorization token")
	}

	log.Printf("Discovering devices for user: %s", userID)

	// Get user's devices from DynamoDB
	devices, err := getUserDevices(ctx, userID)
	if err != nil {
		log.Printf("Failed to get devices: %v", err)
		return createErrorResponse(request, "INTERNAL_ERROR", "Failed to retrieve devices")
	}

	// Build endpoints for each LED strip on each device
	endpoints := []shared.AlexaDiscoveryEndpoint{}

	for _, device := range devices {
		if !device.IsReady {
			log.Printf("Skipping device %s - not ready", device.Name)
			continue
		}

		// If device has no LED strips configured, skip it
		if len(device.LEDStrips) == 0 {
			log.Printf("Skipping device %s - no LED strips configured", device.Name)
			continue
		}

		// Create an endpoint for each LED strip
		for _, strip := range device.LEDStrips {
			endpointID := fmt.Sprintf("%s-strip-D%d", device.DeviceID, strip.Pin)
			friendlyName := fmt.Sprintf("%s Strip D%d", device.Name, strip.Pin)

			endpoint := shared.AlexaDiscoveryEndpoint{
				EndpointID:        endpointID,
				ManufacturerName:  "Garage Lights",
				FriendlyName:      friendlyName,
				Description:       fmt.Sprintf("LED strip on pin D%d with %d LEDs", strip.Pin, strip.LEDCount),
				DisplayCategories: []string{"LIGHT"},
				Cookie: shared.Cookie{
					"deviceId":   device.DeviceID,
					"particleId": device.ParticleID,
					"pin":        strconv.Itoa(strip.Pin),
					"ledCount":   strconv.Itoa(strip.LEDCount),
				},
				Capabilities: buildCapabilities(),
				AdditionalAttributes: &shared.AdditionalAttributes{
					Manufacturer:    "Garage Lights",
					Model:           "LED Strip Controller",
					FirmwareVersion: device.FirmwareVersion,
				},
			}

			endpoints = append(endpoints, endpoint)
			log.Printf("Added endpoint: %s (%s)", endpointID, friendlyName)
		}
	}

	log.Printf("Discovered %d endpoints", len(endpoints))

	response := shared.AlexaResponse{
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:      "Alexa.Discovery",
				Name:           "Discover.Response",
				PayloadVersion: "3",
				MessageID:      uuid.New().String(),
			},
			Payload: shared.DiscoveryPayload{
				Endpoints: endpoints,
			},
		},
	}

	return response, nil
}

// handlePowerControl handles TurnOn and TurnOff directives
func handlePowerControl(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== handlePowerControl: %s ===", request.Directive.Header.Name)

	// Validate token and get user
	userID, err := validateEndpointToken(ctx, request)
	if err != nil {
		return createErrorResponse(request, "INVALID_AUTHORIZATION_CREDENTIAL", err.Error())
	}

	// Parse endpoint
	deviceID, pin, err := parseEndpointID(request.Directive.Endpoint.EndpointID)
	if err != nil {
		return createErrorResponse(request, "NO_SUCH_ENDPOINT", err.Error())
	}

	// Get device and particle token
	device, particleToken, err := getDeviceAndToken(ctx, userID, deviceID)
	if err != nil {
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", err.Error())
	}

	// Determine power state
	powerState := "OFF"
	patternNum := 0
	if request.Directive.Header.Name == "TurnOn" {
		powerState = "ON"
		patternNum = 2 // Solid pattern when turning on
	}

	// Send command to device
	patternArg := fmt.Sprintf("%d,%d,50", pin, patternNum)
	if err := callParticleFunction(device.ParticleID, "setPattern", patternArg, particleToken); err != nil {
		log.Printf("Failed to set power: %v", err)
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", "Failed to control device")
	}

	// Save state
	state := &shared.AlexaDeviceState{
		EndpointID: request.Directive.Endpoint.EndpointID,
		UserID:     userID,
		DeviceID:   deviceID,
		Pin:        pin,
		PowerState: powerState,
	}
	shared.SaveAlexaDeviceState(ctx, state)

	// Build response
	return buildPowerResponse(request, powerState)
}

// handleBrightnessControl handles SetBrightness and AdjustBrightness
func handleBrightnessControl(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== handleBrightnessControl: %s ===", request.Directive.Header.Name)

	userID, err := validateEndpointToken(ctx, request)
	if err != nil {
		return createErrorResponse(request, "INVALID_AUTHORIZATION_CREDENTIAL", err.Error())
	}

	deviceID, pin, err := parseEndpointID(request.Directive.Endpoint.EndpointID)
	if err != nil {
		return createErrorResponse(request, "NO_SUCH_ENDPOINT", err.Error())
	}

	device, particleToken, err := getDeviceAndToken(ctx, userID, deviceID)
	if err != nil {
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", err.Error())
	}

	// Get current state for adjustment
	currentState, _ := shared.GetAlexaDeviceState(ctx, request.Directive.Endpoint.EndpointID)

	var brightness int

	if request.Directive.Header.Name == "SetBrightness" {
		payload, _ := json.Marshal(request.Directive.Payload)
		var setBrightness shared.SetBrightnessPayload
		json.Unmarshal(payload, &setBrightness)
		brightness = setBrightness.Brightness
	} else if request.Directive.Header.Name == "AdjustBrightness" {
		payload, _ := json.Marshal(request.Directive.Payload)
		var adjustBrightness shared.AdjustBrightnessPayload
		json.Unmarshal(payload, &adjustBrightness)

		currentBrightness := 100
		if currentState != nil {
			currentBrightness = currentState.Brightness
		}
		brightness = shared.ClampBrightness(currentBrightness + adjustBrightness.BrightnessDelta)
	}

	// Convert to firmware value (0-255)
	firmwareBrightness := shared.BrightnessPercentToFirmware(brightness)

	// Send command
	brightnessArg := fmt.Sprintf("%d,%d", pin, firmwareBrightness)
	if err := callParticleFunction(device.ParticleID, "setBright", brightnessArg, particleToken); err != nil {
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", "Failed to set brightness")
	}

	// Save state
	state := &shared.AlexaDeviceState{
		EndpointID: request.Directive.Endpoint.EndpointID,
		UserID:     userID,
		DeviceID:   deviceID,
		Pin:        pin,
		Brightness: brightness,
		PowerState: "ON",
	}
	if currentState != nil {
		state.ColorHue = currentState.ColorHue
		state.ColorSaturation = currentState.ColorSaturation
		state.PatternMode = currentState.PatternMode
	}
	shared.SaveAlexaDeviceState(ctx, state)

	return buildBrightnessResponse(request, brightness)
}

// handleColorControl handles SetColor directive
func handleColorControl(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== handleColorControl ===")

	userID, err := validateEndpointToken(ctx, request)
	if err != nil {
		return createErrorResponse(request, "INVALID_AUTHORIZATION_CREDENTIAL", err.Error())
	}

	deviceID, pin, err := parseEndpointID(request.Directive.Endpoint.EndpointID)
	if err != nil {
		return createErrorResponse(request, "NO_SUCH_ENDPOINT", err.Error())
	}

	device, particleToken, err := getDeviceAndToken(ctx, userID, deviceID)
	if err != nil {
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", err.Error())
	}

	// Parse color payload
	payload, _ := json.Marshal(request.Directive.Payload)
	var setColor shared.SetColorPayload
	json.Unmarshal(payload, &setColor)

	// Convert HSB to RGB
	rgb := shared.HSBToRGB(setColor.Color.Hue, setColor.Color.Saturation, setColor.Color.Brightness)
	log.Printf("Color conversion: HSB(%.1f, %.2f, %.2f) -> RGB(%d, %d, %d)",
		setColor.Color.Hue, setColor.Color.Saturation, setColor.Color.Brightness,
		rgb.R, rgb.G, rgb.B)

	// Send color command
	colorArg := fmt.Sprintf("%d,%d,%d,%d", pin, rgb.R, rgb.G, rgb.B)
	if err := callParticleFunction(device.ParticleID, "setColor", colorArg, particleToken); err != nil {
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", "Failed to set color")
	}

	// Ensure pattern is set to solid for color to show
	patternArg := fmt.Sprintf("%d,2,50", pin)
	callParticleFunction(device.ParticleID, "setPattern", patternArg, particleToken)

	// Save state
	state := &shared.AlexaDeviceState{
		EndpointID:      request.Directive.Endpoint.EndpointID,
		UserID:          userID,
		DeviceID:        deviceID,
		Pin:             pin,
		PowerState:      "ON",
		ColorHue:        setColor.Color.Hue,
		ColorSaturation: setColor.Color.Saturation,
		Brightness:      int(setColor.Color.Brightness * 100),
		PatternMode:     shared.AlexaModeSolid,
	}
	shared.SaveAlexaDeviceState(ctx, state)

	return buildColorResponse(request, setColor.Color)
}

// handleModeControl handles SetMode directive for patterns
func handleModeControl(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== handleModeControl ===")

	userID, err := validateEndpointToken(ctx, request)
	if err != nil {
		return createErrorResponse(request, "INVALID_AUTHORIZATION_CREDENTIAL", err.Error())
	}

	deviceID, pin, err := parseEndpointID(request.Directive.Endpoint.EndpointID)
	if err != nil {
		return createErrorResponse(request, "NO_SUCH_ENDPOINT", err.Error())
	}

	device, particleToken, err := getDeviceAndToken(ctx, userID, deviceID)
	if err != nil {
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", err.Error())
	}

	// Parse mode payload
	payload, _ := json.Marshal(request.Directive.Payload)
	var setMode shared.SetModePayload
	json.Unmarshal(payload, &setMode)

	log.Printf("Setting mode: %s", setMode.Mode)

	// Convert Alexa mode to firmware pattern number
	patternNum, ok := shared.AlexaModeToPattern[setMode.Mode]
	if !ok {
		log.Printf("Unknown mode: %s", setMode.Mode)
		return createErrorResponse(request, "VALUE_OUT_OF_RANGE", "Unknown mode")
	}

	// Send pattern command
	patternArg := fmt.Sprintf("%d,%d,50", pin, patternNum)
	if err := callParticleFunction(device.ParticleID, "setPattern", patternArg, particleToken); err != nil {
		return createErrorResponse(request, "ENDPOINT_UNREACHABLE", "Failed to set mode")
	}

	// Save state
	currentState, _ := shared.GetAlexaDeviceState(ctx, request.Directive.Endpoint.EndpointID)
	state := &shared.AlexaDeviceState{
		EndpointID:  request.Directive.Endpoint.EndpointID,
		UserID:      userID,
		DeviceID:    deviceID,
		Pin:         pin,
		PowerState:  "ON",
		PatternMode: setMode.Mode,
	}
	if currentState != nil {
		state.Brightness = currentState.Brightness
		state.ColorHue = currentState.ColorHue
		state.ColorSaturation = currentState.ColorSaturation
	}
	shared.SaveAlexaDeviceState(ctx, state)

	return buildModeResponse(request, setMode.Mode)
}

// handleReportState returns current state of an endpoint
func handleReportState(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== handleReportState ===")

	userID, err := validateEndpointToken(ctx, request)
	if err != nil {
		return createErrorResponse(request, "INVALID_AUTHORIZATION_CREDENTIAL", err.Error())
	}

	endpointID := request.Directive.Endpoint.EndpointID
	state, err := shared.GetAlexaDeviceState(ctx, endpointID)
	if err != nil {
		log.Printf("Failed to get state: %v", err)
	}

	// Default state if not found
	if state == nil {
		state = &shared.AlexaDeviceState{
			EndpointID: endpointID,
			UserID:     userID,
			PowerState: "OFF",
			Brightness: 100,
		}
	}

	return buildStateReportResponse(request, state)
}

// handleAcceptGrant handles OAuth grant acceptance
func handleAcceptGrant(ctx context.Context, request shared.AlexaRequest) (interface{}, error) {
	log.Printf("=== handleAcceptGrant ===")

	// AcceptGrant is used when proactive state updates are enabled
	// For now, just acknowledge it
	response := shared.AlexaResponse{
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:      "Alexa.Authorization",
				Name:           "AcceptGrant.Response",
				PayloadVersion: "3",
				MessageID:      uuid.New().String(),
			},
			Payload: map[string]interface{}{},
		},
	}

	return response, nil
}

// Helper functions

func buildCapabilities() []shared.AlexaCapability {
	return []shared.AlexaCapability{
		{
			Type:      "AlexaInterface",
			Interface: "Alexa",
			Version:   "3",
		},
		{
			Type:      "AlexaInterface",
			Interface: "Alexa.PowerController",
			Version:   "3",
			Properties: &shared.CapabilityProperties{
				Supported: []shared.SupportedProperty{
					{Name: "powerState"},
				},
				ProactivelyReported: false,
				Retrievable:         true,
			},
		},
		{
			Type:      "AlexaInterface",
			Interface: "Alexa.BrightnessController",
			Version:   "3",
			Properties: &shared.CapabilityProperties{
				Supported: []shared.SupportedProperty{
					{Name: "brightness"},
				},
				ProactivelyReported: false,
				Retrievable:         true,
			},
		},
		{
			Type:      "AlexaInterface",
			Interface: "Alexa.ColorController",
			Version:   "3",
			Properties: &shared.CapabilityProperties{
				Supported: []shared.SupportedProperty{
					{Name: "color"},
				},
				ProactivelyReported: false,
				Retrievable:         true,
			},
		},
		{
			Type:      "AlexaInterface",
			Interface: "Alexa.ModeController",
			Instance:  "LightStrip.Pattern",
			Version:   "3",
			Properties: &shared.CapabilityProperties{
				Supported: []shared.SupportedProperty{
					{Name: "mode"},
				},
				ProactivelyReported: false,
				Retrievable:         true,
			},
			CapabilityResources: &shared.CapabilityResources{
				FriendlyNames: []shared.FriendlyName{
					{Type: "text", Value: shared.FriendlyNameVal{Text: "pattern", Locale: "en-US"}},
					{Type: "text", Value: shared.FriendlyNameVal{Text: "effect", Locale: "en-US"}},
					{Type: "text", Value: shared.FriendlyNameVal{Text: "mode", Locale: "en-US"}},
				},
			},
			Configuration: &shared.ModeConfiguration{
				Ordered: false,
				SupportedModes: []shared.SupportedMode{
					{
						Value: shared.AlexaModeSolid,
						ModeResources: &shared.CapabilityResources{
							FriendlyNames: []shared.FriendlyName{
								{Type: "text", Value: shared.FriendlyNameVal{Text: "solid", Locale: "en-US"}},
								{Type: "text", Value: shared.FriendlyNameVal{Text: "static", Locale: "en-US"}},
							},
						},
					},
					{
						Value: shared.AlexaModeCandle,
						ModeResources: &shared.CapabilityResources{
							FriendlyNames: []shared.FriendlyName{
								{Type: "text", Value: shared.FriendlyNameVal{Text: "candle", Locale: "en-US"}},
								{Type: "text", Value: shared.FriendlyNameVal{Text: "flicker", Locale: "en-US"}},
							},
						},
					},
					{
						Value: shared.AlexaModePulse,
						ModeResources: &shared.CapabilityResources{
							FriendlyNames: []shared.FriendlyName{
								{Type: "text", Value: shared.FriendlyNameVal{Text: "pulse", Locale: "en-US"}},
								{Type: "text", Value: shared.FriendlyNameVal{Text: "breathing", Locale: "en-US"}},
							},
						},
					},
					{
						Value: shared.AlexaModeWave,
						ModeResources: &shared.CapabilityResources{
							FriendlyNames: []shared.FriendlyName{
								{Type: "text", Value: shared.FriendlyNameVal{Text: "wave", Locale: "en-US"}},
							},
						},
					},
					{
						Value: shared.AlexaModeRainbow,
						ModeResources: &shared.CapabilityResources{
							FriendlyNames: []shared.FriendlyName{
								{Type: "text", Value: shared.FriendlyNameVal{Text: "rainbow", Locale: "en-US"}},
								{Type: "text", Value: shared.FriendlyNameVal{Text: "colorful", Locale: "en-US"}},
							},
						},
					},
					{
						Value: shared.AlexaModeFire,
						ModeResources: &shared.CapabilityResources{
							FriendlyNames: []shared.FriendlyName{
								{Type: "text", Value: shared.FriendlyNameVal{Text: "fire", Locale: "en-US"}},
								{Type: "text", Value: shared.FriendlyNameVal{Text: "flame", Locale: "en-US"}},
							},
						},
					},
				},
			},
		},
	}
}

func validateEndpointToken(ctx context.Context, request shared.AlexaRequest) (string, error) {
	token := request.Directive.Endpoint.Scope.Token
	if token == "" {
		return "", fmt.Errorf("missing authorization token")
	}

	userID, err := shared.ValidateAccessToken(ctx, token)
	if err != nil || userID == "" {
		return "", fmt.Errorf("invalid authorization token")
	}

	return userID, nil
}

func parseEndpointID(endpointID string) (deviceID string, pin int, err error) {
	// Format: {deviceId}-strip-D{pin}
	parts := strings.Split(endpointID, "-strip-D")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid endpoint ID format: %s", endpointID)
	}

	deviceID = parts[0]
	pin, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid pin in endpoint ID: %s", endpointID)
	}

	return deviceID, pin, nil
}

func getUserDevices(ctx context.Context, userID string) ([]shared.Device, error) {
	indexName := "userId-index"
	var devices []shared.Device

	expressionValues := map[string]interface{}{
		":userId": userID,
	}

	av, _ := attributevalue.MarshalMap(expressionValues)
	if err := shared.Query(ctx, devicesTable, &indexName, "userId = :userId", av, &devices); err != nil {
		return nil, err
	}

	return devices, nil
}

func getDeviceAndToken(ctx context.Context, userID, deviceID string) (*shared.Device, string, error) {
	// Get device
	deviceKey, _ := attributevalue.MarshalMap(map[string]string{
		"deviceId": deviceID,
	})

	var device shared.Device
	if err := shared.GetItem(ctx, devicesTable, deviceKey, &device); err != nil {
		return nil, "", fmt.Errorf("failed to get device: %v", err)
	}

	if device.DeviceID == "" {
		return nil, "", fmt.Errorf("device not found")
	}

	// Verify ownership
	if device.UserID != userID {
		return nil, "", fmt.Errorf("access denied")
	}

	// Get user's Particle token
	userKey, _ := attributevalue.MarshalMap(map[string]string{
		"username": userID,
	})

	var user shared.User
	if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
		return nil, "", fmt.Errorf("failed to get user: %v", err)
	}

	if user.ParticleToken == "" {
		return nil, "", fmt.Errorf("Particle token not configured")
	}

	return &device, user.ParticleToken, nil
}

// Response builders

func buildPowerResponse(request shared.AlexaRequest, powerState string) (interface{}, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	return shared.AlexaResponse{
		Context: &shared.AlexaContext{
			Properties: []shared.AlexaProperty{
				{
					Namespace:                 "Alexa.PowerController",
					Name:                      "powerState",
					Value:                     powerState,
					TimeOfSample:              now,
					UncertaintyInMilliseconds: 500,
				},
			},
		},
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:        "Alexa",
				Name:             "Response",
				PayloadVersion:   "3",
				MessageID:        uuid.New().String(),
				CorrelationToken: request.Directive.Header.CorrelationToken,
			},
			Endpoint: shared.AlexaEndpoint{
				EndpointID: request.Directive.Endpoint.EndpointID,
			},
			Payload: map[string]interface{}{},
		},
	}, nil
}

func buildBrightnessResponse(request shared.AlexaRequest, brightness int) (interface{}, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	return shared.AlexaResponse{
		Context: &shared.AlexaContext{
			Properties: []shared.AlexaProperty{
				{
					Namespace:                 "Alexa.BrightnessController",
					Name:                      "brightness",
					Value:                     brightness,
					TimeOfSample:              now,
					UncertaintyInMilliseconds: 500,
				},
			},
		},
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:        "Alexa",
				Name:             "Response",
				PayloadVersion:   "3",
				MessageID:        uuid.New().String(),
				CorrelationToken: request.Directive.Header.CorrelationToken,
			},
			Endpoint: shared.AlexaEndpoint{
				EndpointID: request.Directive.Endpoint.EndpointID,
			},
			Payload: map[string]interface{}{},
		},
	}, nil
}

func buildColorResponse(request shared.AlexaRequest, color shared.HSBColor) (interface{}, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	return shared.AlexaResponse{
		Context: &shared.AlexaContext{
			Properties: []shared.AlexaProperty{
				{
					Namespace: "Alexa.ColorController",
					Name:      "color",
					Value: map[string]float64{
						"hue":        color.Hue,
						"saturation": color.Saturation,
						"brightness": color.Brightness,
					},
					TimeOfSample:              now,
					UncertaintyInMilliseconds: 500,
				},
			},
		},
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:        "Alexa",
				Name:             "Response",
				PayloadVersion:   "3",
				MessageID:        uuid.New().String(),
				CorrelationToken: request.Directive.Header.CorrelationToken,
			},
			Endpoint: shared.AlexaEndpoint{
				EndpointID: request.Directive.Endpoint.EndpointID,
			},
			Payload: map[string]interface{}{},
		},
	}, nil
}

func buildModeResponse(request shared.AlexaRequest, mode string) (interface{}, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	return shared.AlexaResponse{
		Context: &shared.AlexaContext{
			Properties: []shared.AlexaProperty{
				{
					Namespace:                 "Alexa.ModeController",
					Name:                      "mode",
					Value:                     mode,
					TimeOfSample:              now,
					UncertaintyInMilliseconds: 500,
				},
			},
		},
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:        "Alexa",
				Name:             "Response",
				PayloadVersion:   "3",
				MessageID:        uuid.New().String(),
				CorrelationToken: request.Directive.Header.CorrelationToken,
			},
			Endpoint: shared.AlexaEndpoint{
				EndpointID: request.Directive.Endpoint.EndpointID,
			},
			Payload: map[string]interface{}{},
		},
	}, nil
}

func buildStateReportResponse(request shared.AlexaRequest, state *shared.AlexaDeviceState) (interface{}, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	properties := []shared.AlexaProperty{
		{
			Namespace:                 "Alexa.PowerController",
			Name:                      "powerState",
			Value:                     state.PowerState,
			TimeOfSample:              now,
			UncertaintyInMilliseconds: 0,
		},
		{
			Namespace:                 "Alexa.BrightnessController",
			Name:                      "brightness",
			Value:                     state.Brightness,
			TimeOfSample:              now,
			UncertaintyInMilliseconds: 0,
		},
	}

	if state.ColorHue > 0 || state.ColorSaturation > 0 {
		properties = append(properties, shared.AlexaProperty{
			Namespace: "Alexa.ColorController",
			Name:      "color",
			Value: map[string]float64{
				"hue":        state.ColorHue,
				"saturation": state.ColorSaturation,
				"brightness": float64(state.Brightness) / 100,
			},
			TimeOfSample:              now,
			UncertaintyInMilliseconds: 0,
		})
	}

	if state.PatternMode != "" {
		properties = append(properties, shared.AlexaProperty{
			Namespace:                 "Alexa.ModeController",
			Name:                      "mode",
			Value:                     state.PatternMode,
			TimeOfSample:              now,
			UncertaintyInMilliseconds: 0,
		})
	}

	return shared.AlexaResponse{
		Context: &shared.AlexaContext{
			Properties: properties,
		},
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:        "Alexa",
				Name:             "StateReport",
				PayloadVersion:   "3",
				MessageID:        uuid.New().String(),
				CorrelationToken: request.Directive.Header.CorrelationToken,
			},
			Endpoint: shared.AlexaEndpoint{
				EndpointID: request.Directive.Endpoint.EndpointID,
			},
			Payload: map[string]interface{}{},
		},
	}, nil
}

func createErrorResponse(request shared.AlexaRequest, errorType, message string) (interface{}, error) {
	response := shared.AlexaResponse{
		Event: shared.AlexaEvent{
			Header: shared.AlexaHeader{
				Namespace:        "Alexa",
				Name:             "ErrorResponse",
				PayloadVersion:   "3",
				MessageID:        uuid.New().String(),
				CorrelationToken: request.Directive.Header.CorrelationToken,
			},
			Endpoint: shared.AlexaEndpoint{
				EndpointID: request.Directive.Endpoint.EndpointID,
			},
			Payload: shared.ErrorPayload{
				Type:    errorType,
				Message: message,
			},
		},
	}

	return response, nil
}

func main() {
	lambda.Start(handler)
}
