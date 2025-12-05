package shared

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	alexaTokensTable = os.Getenv("ALEXA_TOKENS_TABLE")
	alexaCodesTable  = os.Getenv("ALEXA_CODES_TABLE")
	alexaStateTable  = os.Getenv("ALEXA_STATE_TABLE")
)

// GenerateAuthCode creates a new OAuth authorization code
func GenerateAuthCode(ctx context.Context, userID, clientID, redirectURI, scope string) (*OAuthAuthCode, error) {
	code, err := generateSecureToken(32)
	if err != nil {
		log.Printf("[ALEXA_DB] Failed to generate auth code: %v", err)
		return nil, err
	}

	authCode := &OAuthAuthCode{
		Code:        code,
		UserID:      userID,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ExpiresAt:   time.Now().Add(10 * time.Minute).Unix(), // 10 minute expiration
		CreatedAt:   time.Now(),
	}

	if err := PutItem(ctx, alexaCodesTable, authCode); err != nil {
		log.Printf("[ALEXA_DB] Failed to save auth code: %v", err)
		return nil, err
	}

	log.Printf("[ALEXA_DB] Created auth code for user %s", userID)
	return authCode, nil
}

// GetAuthCode retrieves and validates an authorization code
func GetAuthCode(ctx context.Context, code string) (*OAuthAuthCode, error) {
	key, err := attributevalue.MarshalMap(map[string]string{
		"code": code,
	})
	if err != nil {
		return nil, err
	}

	var authCode OAuthAuthCode
	if err := GetItem(ctx, alexaCodesTable, key, &authCode); err != nil {
		log.Printf("[ALEXA_DB] Failed to get auth code: %v", err)
		return nil, err
	}

	if authCode.Code == "" {
		log.Printf("[ALEXA_DB] Auth code not found")
		return nil, nil
	}

	// Check expiration
	if time.Now().Unix() > authCode.ExpiresAt {
		log.Printf("[ALEXA_DB] Auth code expired")
		return nil, nil
	}

	return &authCode, nil
}

// DeleteAuthCode removes an authorization code after use
func DeleteAuthCode(ctx context.Context, code string) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"code": code,
	})
	if err != nil {
		return err
	}

	return DeleteItem(ctx, alexaCodesTable, key)
}

// CreateAccessToken generates a new access token for a user
func CreateAccessToken(ctx context.Context, userID, scope string) (*OAuthToken, string, error) {
	// Generate access token
	accessToken, err := generateSecureToken(32)
	if err != nil {
		return nil, "", err
	}

	// Generate refresh token
	refreshToken, err := generateSecureToken(32)
	if err != nil {
		return nil, "", err
	}

	// Hash the access token for storage
	tokenHash := hashToken(accessToken)

	token := &OAuthToken{
		TokenHash:    tokenHash,
		UserID:       userID,
		RefreshToken: refreshToken,
		Scope:        scope,
		ExpiresAt:    time.Now().Add(1 * time.Hour).Unix(), // 1 hour expiration
		CreatedAt:    time.Now(),
	}

	if err := PutItem(ctx, alexaTokensTable, token); err != nil {
		log.Printf("[ALEXA_DB] Failed to save access token: %v", err)
		return nil, "", err
	}

	log.Printf("[ALEXA_DB] Created access token for user %s", userID)
	return token, accessToken, nil
}

// ValidateAccessToken checks if an access token is valid and returns the user ID
func ValidateAccessToken(ctx context.Context, accessToken string) (string, error) {
	tokenHash := hashToken(accessToken)

	key, err := attributevalue.MarshalMap(map[string]string{
		"tokenHash": tokenHash,
	})
	if err != nil {
		return "", err
	}

	var token OAuthToken
	if err := GetItem(ctx, alexaTokensTable, key, &token); err != nil {
		log.Printf("[ALEXA_DB] Failed to get access token: %v", err)
		return "", err
	}

	if token.TokenHash == "" {
		log.Printf("[ALEXA_DB] Access token not found")
		return "", nil
	}

	// Check expiration
	if time.Now().Unix() > token.ExpiresAt {
		log.Printf("[ALEXA_DB] Access token expired")
		return "", nil
	}

	return token.UserID, nil
}

// RefreshAccessToken creates a new access token using a refresh token
func RefreshAccessToken(ctx context.Context, refreshToken string) (*OAuthToken, string, error) {
	// Find token by refresh token (need to scan or use GSI)
	// For simplicity, we'll scan the table
	var tokens []OAuthToken
	if err := Scan(ctx, alexaTokensTable, &tokens); err != nil {
		return nil, "", err
	}

	var existingToken *OAuthToken
	for _, t := range tokens {
		if t.RefreshToken == refreshToken {
			existingToken = &t
			break
		}
	}

	if existingToken == nil {
		log.Printf("[ALEXA_DB] Refresh token not found")
		return nil, "", nil
	}

	// Delete old token
	oldKey, _ := attributevalue.MarshalMap(map[string]string{
		"tokenHash": existingToken.TokenHash,
	})
	DeleteItem(ctx, alexaTokensTable, oldKey)

	// Create new token
	return CreateAccessToken(ctx, existingToken.UserID, existingToken.Scope)
}

// SaveAlexaDeviceState saves the state of an Alexa endpoint
func SaveAlexaDeviceState(ctx context.Context, state *AlexaDeviceState) error {
	state.LastUpdated = time.Now()
	return PutItem(ctx, alexaStateTable, state)
}

// GetAlexaDeviceState retrieves the state of an Alexa endpoint
func GetAlexaDeviceState(ctx context.Context, endpointID string) (*AlexaDeviceState, error) {
	key, err := attributevalue.MarshalMap(map[string]string{
		"endpointId": endpointID,
	})
	if err != nil {
		return nil, err
	}

	var state AlexaDeviceState
	if err := GetItem(ctx, alexaStateTable, key, &state); err != nil {
		return nil, err
	}

	if state.EndpointID == "" {
		return nil, nil
	}

	return &state, nil
}

// GetUserAlexaDeviceStates retrieves all Alexa device states for a user
func GetUserAlexaDeviceStates(ctx context.Context, userID string) ([]AlexaDeviceState, error) {
	indexName := "userId-index"
	var states []AlexaDeviceState

	expressionValues := map[string]types.AttributeValue{
		":userId": &types.AttributeValueMemberS{Value: userID},
	}

	if err := Query(ctx, alexaStateTable, &indexName, "userId = :userId", expressionValues, &states); err != nil {
		return nil, err
	}

	return states, nil
}

// DeleteAlexaDeviceState removes an Alexa device state
func DeleteAlexaDeviceState(ctx context.Context, endpointID string) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"endpointId": endpointID,
	})
	if err != nil {
		return err
	}

	return DeleteItem(ctx, alexaStateTable, key)
}

// Helper functions

func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
