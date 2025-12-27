package shared

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var sessionsTable = os.Getenv("SESSIONS_TABLE")

// Session represents a user session
type Session struct {
	SessionID string    `json:"sessionId" dynamodbav:"sessionId"`
	Username  string    `json:"username" dynamodbav:"username"`
	CreatedAt time.Time `json:"createdAt" dynamodbav:"createdAt"`
	ExpiresAt int64     `json:"expiresAt" dynamodbav:"expiresAt"` // Unix timestamp for TTL
	UserAgent string    `json:"userAgent,omitempty" dynamodbav:"userAgent,omitempty"`
	IPAddress string    `json:"ipAddress,omitempty" dynamodbav:"ipAddress,omitempty"`
}

// CreateSession creates a new session for a user
func CreateSession(ctx context.Context, username, userAgent, ipAddress string) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		log.Printf("CreateSession: Failed to generate session ID: %v", err)
		return nil, err
	}

	session := &Session{
		SessionID: sessionID,
		Username:  username,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), // 24 hour expiration
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}

	log.Printf("CreateSession: Creating session for user %s, sessionID: %s (first 10 chars)", username, safeDisplay(sessionID, 10))

	if err := PutItem(ctx, sessionsTable, session); err != nil {
		log.Printf("CreateSession: Failed to save session: %v", err)
		return nil, err
	}

	log.Printf("CreateSession: Session created successfully for user %s", username)
	return session, nil
}

// GetSession retrieves a session by ID
func GetSession(ctx context.Context, sessionID string) (*Session, error) {
	log.Printf("GetSession: Looking up session: %s (first 10 chars)", safeDisplay(sessionID, 10))

	key, err := attributevalue.MarshalMap(map[string]string{
		"sessionId": sessionID,
	})
	if err != nil {
		log.Printf("GetSession: Failed to marshal key: %v", err)
		return nil, err
	}

	var session Session
	if err := GetItem(ctx, sessionsTable, key, &session); err != nil {
		log.Printf("GetSession: Failed to get session: %v", err)
		return nil, err
	}

	if session.SessionID == "" {
		log.Printf("GetSession: Session not found")
		return nil, nil
	}

	// Check if session is expired
	if time.Now().Unix() > session.ExpiresAt {
		log.Printf("GetSession: Session expired at %v", time.Unix(session.ExpiresAt, 0))
		return nil, nil
	}

	log.Printf("GetSession: Session found for user %s", session.Username)
	return &session, nil
}

// DeleteSession deletes a session
func DeleteSession(ctx context.Context, sessionID string) error {
	log.Printf("DeleteSession: Deleting session: %s (first 10 chars)", safeDisplay(sessionID, 10))

	key, err := attributevalue.MarshalMap(map[string]string{
		"sessionId": sessionID,
	})
	if err != nil {
		log.Printf("DeleteSession: Failed to marshal key: %v", err)
		return err
	}

	client, err := InitDynamoDB()
	if err != nil {
		log.Printf("DeleteSession: Failed to init DynamoDB: %v", err)
		return err
	}

	_, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &sessionsTable,
		Key:       key,
	})

	if err != nil {
		log.Printf("DeleteSession: Failed to delete session: %v", err)
		return err
	}

	log.Printf("DeleteSession: Session deleted successfully")
	return nil
}

// DeleteUserSessions deletes all sessions for a user
func DeleteUserSessions(ctx context.Context, username string) error {
	log.Printf("DeleteUserSessions: Deleting all sessions for user %s", username)

	client, err := InitDynamoDB()
	if err != nil {
		log.Printf("DeleteUserSessions: Failed to init DynamoDB: %v", err)
		return err
	}

	// Query sessions by username using the GSI
	queryInput := &dynamodb.QueryInput{
		TableName:              &sessionsTable,
		IndexName:              stringPtr("username-index"),
		KeyConditionExpression: stringPtr("username = :username"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":username": &types.AttributeValueMemberS{Value: username},
		},
	}

	result, err := client.Query(ctx, queryInput)
	if err != nil {
		log.Printf("DeleteUserSessions: Failed to query sessions: %v", err)
		return err
	}

	// Delete each session
	for _, item := range result.Items {
		var session Session
		if err := attributevalue.UnmarshalMap(item, &session); err != nil {
			log.Printf("DeleteUserSessions: Failed to unmarshal session: %v", err)
			continue
		}

		if err := DeleteSession(ctx, session.SessionID); err != nil {
			log.Printf("DeleteUserSessions: Failed to delete session %s: %v", session.SessionID, err)
		}
	}

	log.Printf("DeleteUserSessions: Deleted %d sessions for user %s", len(result.Items), username)
	return nil
}

// generateSessionID generates a secure random session ID
func generateSessionID() (string, error) {
	log.Println("[SESSION] Generating new session ID")
	b := make([]byte, 32) // 32 bytes = 256 bits
	if _, err := rand.Read(b); err != nil {
		log.Printf("[SESSION] ERROR: Failed to generate random bytes: %v", err)
		return "", err
	}
	sessionID := base64.URLEncoding.EncodeToString(b)
	log.Printf("[SESSION] Generated session ID: %s (first 10 chars)", safeDisplay(sessionID, 10))
	return sessionID, nil
}

// safeDisplay returns a safe-to-log portion of a string
func safeDisplay(s string, length int) string {
	if len(s) < length {
		return s
	}
	return s[:length]
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}
