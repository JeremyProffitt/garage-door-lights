package main

import (
	"context"
	"encoding/json"
	"log"
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

var conversationsTable = os.Getenv("CONVERSATIONS_TABLE")
var patternsTable = os.Getenv("PATTERNS_TABLE")

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== GlowBlaster Handler Called ===")
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
	conversationID := request.PathParameters["conversationId"]

	switch {
	// Conversation endpoints
	case path == "/api/glowblaster/conversations" && method == "GET":
		return handleListConversations(ctx, username)
	case path == "/api/glowblaster/conversations" && method == "POST":
		return handleCreateConversation(ctx, username, request)
	case strings.HasSuffix(path, "/chat") && method == "POST":
		return handleChat(ctx, username, conversationID, request)
	case strings.HasSuffix(path, "/compact") && method == "POST":
		return handleCompact(ctx, username, conversationID, request)
	case conversationID != "" && method == "GET" && !strings.Contains(path, "/chat"):
		return handleGetConversation(ctx, username, conversationID)
	case conversationID != "" && method == "DELETE":
		return handleDeleteConversation(ctx, username, conversationID)

	// Compile endpoint
	case path == "/api/glowblaster/compile" && method == "POST":
		return handleCompile(ctx, request)

	// Pattern endpoints
	case path == "/api/glowblaster/patterns" && method == "GET":
		return handleListGlowBlasterPatterns(ctx, username)
	case path == "/api/glowblaster/patterns" && method == "POST":
		return handleSavePattern(ctx, username, request)

	default:
		log.Printf("No matching route for path: %s, method: %s", path, method)
		return shared.CreateErrorResponse(404, "Not found"), nil
	}
}

func handleListConversations(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
	indexName := "userId-index"
	keyCondition := "userId = :userId"
	expressionValues := map[string]types.AttributeValue{
		":userId": &types.AttributeValueMemberS{Value: username},
	}

	var conversations []shared.Conversation
	if err := shared.Query(ctx, conversationsTable, &indexName, keyCondition, expressionValues, &conversations); err != nil {
		log.Printf("Failed to query conversations: %v", err)
		return shared.CreateErrorResponse(500, "Failed to retrieve conversations"), nil
	}

	// Return without full message history for list view
	summaries := make([]map[string]interface{}, len(conversations))
	for i, conv := range conversations {
		summaries[i] = map[string]interface{}{
			"conversationId": conv.ConversationID,
			"title":          conv.Title,
			"model":          conv.Model,
			"totalTokens":    conv.TotalTokens,
			"messageCount":   len(conv.Messages),
			"hasPattern":     conv.CurrentLCL != "",
			"createdAt":      conv.CreatedAt,
			"updatedAt":      conv.UpdatedAt,
		}
	}

	return shared.CreateSuccessResponse(200, summaries), nil
}

func handleCreateConversation(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req shared.CreateConversationRequest
	body := shared.GetRequestBody(request)
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		// Allow empty body for new conversation
		req = shared.CreateConversationRequest{}
	}

	// Set defaults
	if req.Title == "" {
		req.Title = "New Pattern"
	}
	if req.Model == "" || !shared.ValidModels[req.Model] {
		req.Model = shared.DefaultModel
	}

	now := time.Now()
	conversation := shared.Conversation{
		ConversationID: uuid.New().String(),
		UserID:         username,
		Title:          req.Title,
		Model:          req.Model,
		Messages:       []shared.Message{},
		TotalTokens:    0,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      now.Unix() + shared.OneYearInSeconds,
	}

	if err := shared.PutItem(ctx, conversationsTable, conversation); err != nil {
		log.Printf("Failed to create conversation: %v", err)
		return shared.CreateErrorResponse(500, "Failed to create conversation"), nil
	}

	return shared.CreateSuccessResponse(201, conversation), nil
}

func handleGetConversation(ctx context.Context, username, conversationID string) (events.APIGatewayProxyResponse, error) {
	key, _ := attributevalue.MarshalMap(map[string]string{
		"conversationId": conversationID,
	})

	var conversation shared.Conversation
	if err := shared.GetItem(ctx, conversationsTable, key, &conversation); err != nil {
		log.Printf("Failed to get conversation: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if conversation.ConversationID == "" {
		return shared.CreateErrorResponse(404, "Conversation not found"), nil
	}

	if conversation.UserID != username {
		return shared.CreateErrorResponse(403, "Access denied"), nil
	}

	return shared.CreateSuccessResponse(200, conversation), nil
}

func handleDeleteConversation(ctx context.Context, username, conversationID string) (events.APIGatewayProxyResponse, error) {
	key, _ := attributevalue.MarshalMap(map[string]string{
		"conversationId": conversationID,
	})

	var conversation shared.Conversation
	if err := shared.GetItem(ctx, conversationsTable, key, &conversation); err != nil {
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if conversation.ConversationID == "" {
		return shared.CreateErrorResponse(404, "Conversation not found"), nil
	}

	if conversation.UserID != username {
		return shared.CreateErrorResponse(403, "Access denied"), nil
	}

	if err := shared.DeleteItem(ctx, conversationsTable, key); err != nil {
		return shared.CreateErrorResponse(500, "Failed to delete conversation"), nil
	}

	return shared.CreateSuccessResponse(200, map[string]string{
		"message": "Conversation deleted successfully",
	}), nil
}

func handleChat(ctx context.Context, username, conversationID string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get conversation
	key, _ := attributevalue.MarshalMap(map[string]string{
		"conversationId": conversationID,
	})

	var conversation shared.Conversation
	if err := shared.GetItem(ctx, conversationsTable, key, &conversation); err != nil {
		log.Printf("Failed to get conversation: %v", err)
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if conversation.ConversationID == "" {
		return shared.CreateErrorResponse(404, "Conversation not found"), nil
	}

	if conversation.UserID != username {
		return shared.CreateErrorResponse(403, "Access denied"), nil
	}

	// Parse request
	var req shared.ChatRequest
	body := shared.GetRequestBody(request)
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return shared.CreateErrorResponse(400, "Invalid request body"), nil
	}

	if req.Message == "" {
		return shared.CreateErrorResponse(400, "Message is required"), nil
	}

	// Determine model to use
	model := conversation.Model
	if req.Model != "" && shared.ValidModels[req.Model] {
		model = req.Model
		conversation.Model = model
	}

	// Add user message
	userMessage := shared.Message{
		Role:      "user",
		Content:   req.Message,
		Timestamp: time.Now(),
	}
	conversation.Messages = append(conversation.Messages, userMessage)

	// Build Claude messages
	claudeMessages := shared.ConvertMessagesToClaudeFormat(conversation.Messages)

	// Call Claude API
	client := shared.NewClaudeClient()
	claudeResp, err := client.SendMessage(model, shared.GlowBlasterSystemPrompt, claudeMessages)
	if err != nil {
		log.Printf("Claude API error: %v", err)
		return shared.CreateErrorResponse(500, "AI service error: "+err.Error()), nil
	}

	// Extract response
	responseText := client.GetResponseText(claudeResp)
	tokensUsed := claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens

	// Add assistant message
	assistantMessage := shared.Message{
		Role:      "assistant",
		Content:   responseText,
		TokensIn:  claudeResp.Usage.InputTokens,
		TokensOut: claudeResp.Usage.OutputTokens,
		Timestamp: time.Now(),
	}
	conversation.Messages = append(conversation.Messages, assistantMessage)
	conversation.TotalTokens += tokensUsed

	// Extract LCL from response
	lcl := shared.ExtractLCLFromResponse(responseText)
	var bytecode []byte
	if lcl != "" {
		conversation.CurrentLCL = lcl
		// Compile to bytecode
		compiled, warnings, compileErr := shared.CompileLCL(lcl)
		if compileErr != nil {
			log.Printf("LCL compile warning: %v", compileErr)
		} else {
			bytecode = compiled
			conversation.CurrentBytecode = bytecode
		}
		if len(warnings) > 0 {
			log.Printf("LCL compile warnings: %v", warnings)
		}
	}

	// Update title if this is the first message
	if len(conversation.Messages) == 2 && conversation.Title == "New Pattern" {
		// Extract a title from the user's first message
		title := req.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conversation.Title = title
	}

	// Save conversation
	conversation.UpdatedAt = time.Now()
	conversation.ExpiresAt = time.Now().Unix() + shared.OneYearInSeconds

	if err := shared.PutItem(ctx, conversationsTable, conversation); err != nil {
		log.Printf("Failed to save conversation: %v", err)
		return shared.CreateErrorResponse(500, "Failed to save conversation"), nil
	}

	// Build response
	response := shared.ChatResponse{
		Message:     responseText,
		LCL:         lcl,
		Bytecode:    bytecode,
		TokensUsed:  tokensUsed,
		TotalTokens: conversation.TotalTokens,
	}

	return shared.CreateSuccessResponse(200, response), nil
}

func handleCompact(ctx context.Context, username, conversationID string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	key, _ := attributevalue.MarshalMap(map[string]string{
		"conversationId": conversationID,
	})

	var conversation shared.Conversation
	if err := shared.GetItem(ctx, conversationsTable, key, &conversation); err != nil {
		return shared.CreateErrorResponse(500, "Database error"), nil
	}

	if conversation.ConversationID == "" {
		return shared.CreateErrorResponse(404, "Conversation not found"), nil
	}

	if conversation.UserID != username {
		return shared.CreateErrorResponse(403, "Access denied"), nil
	}

	// Parse request
	var req shared.CompactRequest
	body := shared.GetRequestBody(request)
	json.Unmarshal([]byte(body), &req)

	keepRecent := 4
	if req.KeepRecent > 0 {
		keepRecent = req.KeepRecent
	}

	if len(conversation.Messages) <= keepRecent {
		return shared.CreateSuccessResponse(200, map[string]string{
			"message": "Conversation is already compact",
		}), nil
	}

	// Create summary of old messages
	oldMessages := conversation.Messages[:len(conversation.Messages)-keepRecent]
	summary := "Previous conversation summary:\n"
	for _, msg := range oldMessages {
		if msg.Role == "user" {
			summary += "- User asked about: " + truncate(msg.Content, 100) + "\n"
		}
	}

	// Keep current LCL context
	if conversation.CurrentLCL != "" {
		summary += "\nCurrent pattern LCL:\n```lcl\n" + conversation.CurrentLCL + "\n```\n"
	}

	// Create compacted conversation
	compactedMessages := []shared.Message{
		{
			Role:      "user",
			Content:   summary,
			Timestamp: time.Now(),
		},
		{
			Role:      "assistant",
			Content:   "Understood! I have the context from our previous conversation. How would you like to continue working on the pattern?",
			Timestamp: time.Now(),
		},
	}
	compactedMessages = append(compactedMessages, conversation.Messages[len(conversation.Messages)-keepRecent:]...)

	conversation.Messages = compactedMessages
	conversation.UpdatedAt = time.Now()

	if err := shared.PutItem(ctx, conversationsTable, conversation); err != nil {
		return shared.CreateErrorResponse(500, "Failed to compact conversation"), nil
	}

	return shared.CreateSuccessResponse(200, map[string]interface{}{
		"message":      "Conversation compacted successfully",
		"messageCount": len(conversation.Messages),
	}), nil
}

func handleCompile(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req shared.CompileRequest
	body := shared.GetRequestBody(request)
	log.Printf("[Compile] Received body length: %d", len(body))

	if err := json.Unmarshal([]byte(body), &req); err != nil {
		log.Printf("[Compile] JSON unmarshal error: %v", err)
		return shared.CreateErrorResponse(400, "Invalid request body"), nil
	}

	if req.LCL == "" {
		log.Printf("[Compile] LCL is empty")
		return shared.CreateErrorResponse(400, "LCL is required"), nil
	}

	log.Printf("[Compile] Compiling LCL (first 200 chars): %s", truncate(req.LCL, 200))

	bytecode, warnings, err := shared.CompileLCL(req.LCL)
	if err != nil {
		log.Printf("[Compile] Compilation error: %v", err)
		return shared.CreateSuccessResponse(200, shared.CompileResponse{
			Success: false,
			Errors:  []string{err.Error()},
		}), nil
	}

	log.Printf("[Compile] Success! Bytecode length: %d, Warnings: %v", len(bytecode), warnings)

	return shared.CreateSuccessResponse(200, shared.CompileResponse{
		Success:  true,
		Bytecode: bytecode,
		Warnings: warnings,
	}), nil
}

func handleListGlowBlasterPatterns(ctx context.Context, username string) (events.APIGatewayProxyResponse, error) {
	indexName := "userId-index"
	keyCondition := "userId = :userId"
	expressionValues := map[string]types.AttributeValue{
		":userId": &types.AttributeValueMemberS{Value: username},
	}

	var allPatterns []shared.Pattern
	if err := shared.Query(ctx, patternsTable, &indexName, keyCondition, expressionValues, &allPatterns); err != nil {
		return shared.CreateErrorResponse(500, "Failed to retrieve patterns"), nil
	}

	// Filter to only glowblaster patterns
	var patterns []shared.Pattern
	for _, p := range allPatterns {
		if p.Category == shared.CategoryGlowBlaster || p.Type == shared.PatternGlowBlaster {
			patterns = append(patterns, p)
		}
	}

	return shared.CreateSuccessResponse(200, patterns), nil
}

func handleSavePattern(ctx context.Context, username string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var req shared.SavePatternRequest
	body := shared.GetRequestBody(request)
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return shared.CreateErrorResponse(400, "Invalid request body"), nil
	}

	if req.Name == "" {
		return shared.CreateErrorResponse(400, "Name is required"), nil
	}

	// If conversation ID provided, get LCL from conversation
	lcl := req.LCL
	if req.ConversationID != "" && lcl == "" {
		key, _ := attributevalue.MarshalMap(map[string]string{
			"conversationId": req.ConversationID,
		})

		var conversation shared.Conversation
		if err := shared.GetItem(ctx, conversationsTable, key, &conversation); err == nil {
			if conversation.UserID == username && conversation.CurrentLCL != "" {
				lcl = conversation.CurrentLCL
			}
		}
	}

	if lcl == "" {
		return shared.CreateErrorResponse(400, "No LCL pattern to save"), nil
	}

	// Compile to bytecode
	bytecode, _, compileErr := shared.CompileLCL(lcl)
	if compileErr != nil {
		return shared.CreateErrorResponse(400, "Failed to compile pattern: "+compileErr.Error()), nil
	}

	now := time.Now()
	pattern := shared.Pattern{
		PatternID:   uuid.New().String(),
		UserID:      username,
		Name:        req.Name,
		Description: req.Description,
		Type:        shared.PatternGlowBlaster,
		Category:    shared.CategoryGlowBlaster,
		LCLSpec:     lcl,
		Bytecode:    bytecode,
		IntentLayer: lcl,
		Brightness:  204, // bright default
		Speed:       50,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := shared.PutItem(ctx, patternsTable, pattern); err != nil {
		return shared.CreateErrorResponse(500, "Failed to save pattern"), nil
	}

	return shared.CreateSuccessResponse(201, pattern), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func main() {
	lambda.Start(handler)
}
