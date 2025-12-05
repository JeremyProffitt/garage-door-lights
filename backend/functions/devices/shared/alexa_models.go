package shared

import "time"

// AlexaRequest represents an incoming Alexa Smart Home directive
type AlexaRequest struct {
	Directive AlexaDirective `json:"directive"`
}

// AlexaDirective contains the header, endpoint, and payload
type AlexaDirective struct {
	Header   AlexaHeader   `json:"header"`
	Endpoint AlexaEndpoint `json:"endpoint,omitempty"`
	Payload  interface{}   `json:"payload"`
}

// AlexaHeader contains directive metadata
type AlexaHeader struct {
	Namespace        string `json:"namespace"`
	Name             string `json:"name"`
	PayloadVersion   string `json:"payloadVersion"`
	MessageID        string `json:"messageId"`
	CorrelationToken string `json:"correlationToken,omitempty"`
}

// AlexaEndpoint identifies the target device
type AlexaEndpoint struct {
	Scope      AlexaScope `json:"scope,omitempty"`
	EndpointID string     `json:"endpointId"`
	Cookie     Cookie     `json:"cookie,omitempty"`
}

// AlexaScope contains the bearer token for authorization
type AlexaScope struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// Cookie contains endpoint metadata
type Cookie map[string]string

// AlexaResponse is the base response structure
type AlexaResponse struct {
	Context *AlexaContext `json:"context,omitempty"`
	Event   AlexaEvent    `json:"event"`
}

// AlexaContext contains property state reports
type AlexaContext struct {
	Properties []AlexaProperty `json:"properties,omitempty"`
}

// AlexaProperty represents a capability property state
type AlexaProperty struct {
	Namespace                 string      `json:"namespace"`
	Name                      string      `json:"name"`
	Value                     interface{} `json:"value"`
	TimeOfSample              string      `json:"timeOfSample"`
	UncertaintyInMilliseconds int         `json:"uncertaintyInMilliseconds"`
}

// AlexaEvent is the event portion of a response
type AlexaEvent struct {
	Header   AlexaHeader   `json:"header"`
	Endpoint AlexaEndpoint `json:"endpoint,omitempty"`
	Payload  interface{}   `json:"payload"`
}

// DiscoveryPayload is the payload for discovery responses
type DiscoveryPayload struct {
	Endpoints []AlexaDiscoveryEndpoint `json:"endpoints"`
}

// AlexaDiscoveryEndpoint describes a discoverable device
type AlexaDiscoveryEndpoint struct {
	EndpointID        string                   `json:"endpointId"`
	ManufacturerName  string                   `json:"manufacturerName"`
	FriendlyName      string                   `json:"friendlyName"`
	Description       string                   `json:"description"`
	DisplayCategories []string                 `json:"displayCategories"`
	Cookie            Cookie                   `json:"cookie,omitempty"`
	Capabilities      []AlexaCapability        `json:"capabilities"`
	AdditionalAttributes *AdditionalAttributes `json:"additionalAttributes,omitempty"`
}

// AdditionalAttributes provides extra device information
type AdditionalAttributes struct {
	Manufacturer     string `json:"manufacturer,omitempty"`
	Model            string `json:"model,omitempty"`
	SerialNumber     string `json:"serialNumber,omitempty"`
	FirmwareVersion  string `json:"firmwareVersion,omitempty"`
	SoftwareVersion  string `json:"softwareVersion,omitempty"`
	CustomIdentifier string `json:"customIdentifier,omitempty"`
}

// AlexaCapability describes a device capability
type AlexaCapability struct {
	Type                   string                  `json:"type"`
	Interface              string                  `json:"interface"`
	Instance               string                  `json:"instance,omitempty"`
	Version                string                  `json:"version"`
	Properties             *CapabilityProperties   `json:"properties,omitempty"`
	CapabilityResources    *CapabilityResources    `json:"capabilityResources,omitempty"`
	Configuration          *ModeConfiguration      `json:"configuration,omitempty"`
	Semantics              *Semantics              `json:"semantics,omitempty"`
}

// CapabilityProperties describes property support
type CapabilityProperties struct {
	Supported           []SupportedProperty `json:"supported,omitempty"`
	ProactivelyReported bool                `json:"proactivelyReported"`
	Retrievable         bool                `json:"retrievable"`
}

// SupportedProperty names a supported property
type SupportedProperty struct {
	Name string `json:"name"`
}

// CapabilityResources for mode controller
type CapabilityResources struct {
	FriendlyNames []FriendlyName `json:"friendlyNames"`
}

// FriendlyName for voice invocation
type FriendlyName struct {
	Type  string          `json:"@type"`
	Value FriendlyNameVal `json:"value"`
}

// FriendlyNameVal contains the actual name text
type FriendlyNameVal struct {
	Text   string `json:"text,omitempty"`
	Locale string `json:"locale,omitempty"`
	AssetID string `json:"assetId,omitempty"`
}

// ModeConfiguration for mode controller
type ModeConfiguration struct {
	Ordered        bool          `json:"ordered"`
	SupportedModes []SupportedMode `json:"supportedModes"`
}

// SupportedMode describes a supported mode
type SupportedMode struct {
	Value         string              `json:"value"`
	ModeResources *CapabilityResources `json:"modeResources"`
}

// Semantics for action mappings
type Semantics struct {
	ActionMappings []ActionMapping `json:"actionMappings,omitempty"`
	StateMappings  []StateMapping  `json:"stateMappings,omitempty"`
}

// ActionMapping maps actions to directives
type ActionMapping struct {
	Type      string   `json:"@type"`
	Actions   []string `json:"actions"`
	Directive *SemanticDirective `json:"directive,omitempty"`
}

// SemanticDirective for action mapping
type SemanticDirective struct {
	Name    string      `json:"name"`
	Payload interface{} `json:"payload,omitempty"`
}

// StateMapping maps states to values
type StateMapping struct {
	Type   string      `json:"@type"`
	States []string    `json:"states"`
	Value  interface{} `json:"value,omitempty"`
	Range  *StateRange `json:"range,omitempty"`
}

// StateRange for range state mappings
type StateRange struct {
	MinimumValue int `json:"minimumValue"`
	MaximumValue int `json:"maximumValue"`
}

// ErrorPayload for error responses
type ErrorPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// SetBrightnessPayload for brightness directives
type SetBrightnessPayload struct {
	Brightness int `json:"brightness"`
}

// AdjustBrightnessPayload for brightness adjustment
type AdjustBrightnessPayload struct {
	BrightnessDelta int `json:"brightnessDelta"`
}

// SetColorPayload for color directives
type SetColorPayload struct {
	Color HSBColor `json:"color"`
}

// HSBColor represents a color in HSB format
type HSBColor struct {
	Hue        float64 `json:"hue"`
	Saturation float64 `json:"saturation"`
	Brightness float64 `json:"brightness"`
}

// SetModePayload for mode controller directives
type SetModePayload struct {
	Mode string `json:"mode"`
}

// OAuth2 Models for Account Linking

// OAuthAuthCode represents an authorization code
type OAuthAuthCode struct {
	Code        string    `json:"code" dynamodbav:"code"`
	UserID      string    `json:"userId" dynamodbav:"userId"`
	ClientID    string    `json:"clientId" dynamodbav:"clientId"`
	RedirectURI string    `json:"redirectUri" dynamodbav:"redirectUri"`
	Scope       string    `json:"scope" dynamodbav:"scope"`
	ExpiresAt   int64     `json:"expiresAt" dynamodbav:"expiresAt"`
	CreatedAt   time.Time `json:"createdAt" dynamodbav:"createdAt"`
}

// OAuthToken represents an access token
type OAuthToken struct {
	TokenHash    string    `json:"tokenHash" dynamodbav:"tokenHash"`
	UserID       string    `json:"userId" dynamodbav:"userId"`
	RefreshToken string    `json:"refreshToken" dynamodbav:"refreshToken"`
	Scope        string    `json:"scope" dynamodbav:"scope"`
	ExpiresAt    int64     `json:"expiresAt" dynamodbav:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt" dynamodbav:"createdAt"`
}

// AlexaDeviceState tracks the state of each endpoint for Alexa
type AlexaDeviceState struct {
	EndpointID     string    `json:"endpointId" dynamodbav:"endpointId"`
	UserID         string    `json:"userId" dynamodbav:"userId"`
	DeviceID       string    `json:"deviceId" dynamodbav:"deviceId"`
	Pin            int       `json:"pin" dynamodbav:"pin"`
	PowerState     string    `json:"powerState" dynamodbav:"powerState"`         // "ON" or "OFF"
	Brightness     int       `json:"brightness" dynamodbav:"brightness"`         // 0-100
	ColorHue       float64   `json:"colorHue" dynamodbav:"colorHue"`             // 0-360
	ColorSaturation float64  `json:"colorSaturation" dynamodbav:"colorSaturation"` // 0-1
	PatternMode    string    `json:"patternMode" dynamodbav:"patternMode"`       // Pattern mode name
	LastUpdated    time.Time `json:"lastUpdated" dynamodbav:"lastUpdated"`
}

// TokenResponse is the OAuth token endpoint response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// TokenRequest is the OAuth token endpoint request
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Pattern mode constants for Alexa
const (
	AlexaModeSolid   = "LightEffect.Solid"
	AlexaModeCandle  = "LightEffect.Candle"
	AlexaModePulse   = "LightEffect.Pulse"
	AlexaModeWave    = "LightEffect.Wave"
	AlexaModeRainbow = "LightEffect.Rainbow"
	AlexaModeFire    = "LightEffect.Fire"
)

// AlexaModeToPattern maps Alexa mode values to firmware pattern numbers
var AlexaModeToPattern = map[string]int{
	AlexaModeSolid:   2,
	AlexaModeCandle:  1,
	AlexaModePulse:   3,
	AlexaModeWave:    4,
	AlexaModeRainbow: 5,
	AlexaModeFire:    6,
}

// PatternToAlexaMode maps firmware pattern numbers to Alexa mode values
var PatternToAlexaMode = map[int]string{
	0: "", // OFF - no mode
	1: AlexaModeCandle,
	2: AlexaModeSolid,
	3: AlexaModePulse,
	4: AlexaModeWave,
	5: AlexaModeRainbow,
	6: AlexaModeFire,
}
