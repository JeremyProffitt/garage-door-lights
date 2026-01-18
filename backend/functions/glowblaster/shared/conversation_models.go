package shared

import "time"

// Conversation represents a Glow Blaster chat session
type Conversation struct {
	ConversationID  string    `json:"conversationId" dynamodbav:"conversationId"`
	UserID          string    `json:"userId" dynamodbav:"userId"`
	Title           string    `json:"title" dynamodbav:"title"`
	Messages        []Message `json:"messages" dynamodbav:"messages"`
	CurrentLCL      string    `json:"currentLcl,omitempty" dynamodbav:"currentLcl,omitempty"`           // Legacy LCL YAML
	CurrentBytecode []byte    `json:"currentBytecode,omitempty" dynamodbav:"currentBytecode,omitempty"` // Legacy LCL bytecode
	// WLED fields (new format)
	CurrentWLED    string `json:"currentWled,omitempty" dynamodbav:"currentWled,omitempty"`       // Current WLED JSON state
	CurrentWLEDBin []byte `json:"currentWledBin,omitempty" dynamodbav:"currentWledBin,omitempty"` // Current WLED binary
	Model          string `json:"model" dynamodbav:"model"`                                       // claude-sonnet-4, claude-3-5-sonnet, claude-3-5-haiku
	TotalTokens    int    `json:"totalTokens" dynamodbav:"totalTokens"`
	PatternID      string `json:"patternId,omitempty" dynamodbav:"patternId,omitempty"` // Associated saved pattern
	CreatedAt      time.Time `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt" dynamodbav:"updatedAt"`
	ExpiresAt      int64     `json:"expiresAt,omitempty" dynamodbav:"expiresAt,omitempty"` // TTL (1 year)
}

// Message represents a single chat message
type Message struct {
	Role      string    `json:"role" dynamodbav:"role"` // "user" or "assistant"
	Content   string    `json:"content" dynamodbav:"content"`
	TokensIn  int       `json:"tokensIn,omitempty" dynamodbav:"tokensIn,omitempty"`
	TokensOut int       `json:"tokensOut,omitempty" dynamodbav:"tokensOut,omitempty"`
	Timestamp time.Time `json:"timestamp" dynamodbav:"timestamp"`
}

// ChatRequest represents a request to send a message
type ChatRequest struct {
	Message string `json:"message"`
	Model   string `json:"model,omitempty"` // Optional: override conversation model
}

// ChatResponse represents the response from a chat message
type ChatResponse struct {
	Message     string         `json:"message"`               // AI response text
	LCL         string         `json:"lcl,omitempty"`         // Updated LCL if pattern changed (legacy)
	Bytecode    []byte         `json:"bytecode,omitempty"`    // Compiled bytecode for preview (legacy LCL or WLED)
	WLED        string         `json:"wled,omitempty"`        // WLED JSON state
	WLEDBinary  []byte         `json:"wledBinary,omitempty"`  // WLED binary for device
	TokensUsed  int            `json:"tokensUsed"`            // Tokens used in this request
	TotalTokens int            `json:"totalTokens"`           // Total tokens in conversation
	Suggestions []string       `json:"suggestions,omitempty"` // Follow-up suggestions
	Debug       *ChatDebugInfo `json:"debug,omitempty"`       // Debug info (prompt, messages)
}

// ChatDebugInfo contains debug information about the chat request
type ChatDebugInfo struct {
	SystemPrompt string          `json:"systemPrompt"`
	Messages     []ClaudeMessage `json:"messages"`
}

// CompileRequest represents a request to compile LCL
type CompileRequest struct {
	LCL string `json:"lcl"` // LCL specification or intent YAML
}

// CompileResponse represents the result of LCL compilation
type CompileResponse struct {
	Success  bool     `json:"success"`
	Bytecode []byte   `json:"bytecode,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// CreateConversationRequest represents a request to create a new conversation
type CreateConversationRequest struct {
	Title string `json:"title,omitempty"`
	Model string `json:"model,omitempty"` // Default: claude-sonnet-4
}

// SavePatternRequest represents a request to save a pattern from conversation
type SavePatternRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	LCL            string `json:"lcl,omitempty"`
}

// CompactRequest represents a request to compact a conversation
type CompactRequest struct {
	KeepRecent int `json:"keepRecent,omitempty"` // Number of recent messages to keep (default: 4)
}

// Available Claude models for Glow Blaster
const (
	ModelClaude37Sonnet = "claude-3-7-sonnet-20250219"
	ModelClaude35Sonnet = "claude-3-5-sonnet-20241022"
	ModelClaude35Haiku  = "claude-3-5-haiku-20241022"
	DefaultModel        = ModelClaude37Sonnet
)

// IsValidModel checks if the model ID is valid (basic format check or known model)
func IsValidModel(model string) bool {
	// Allow any model ID that looks like an Anthropic model (starts with claude-)
	// This allows dynamic models to be used even if not hardcoded here.
	if len(model) > 7 && model[:7] == "claude-" {
		return true
	}
	return false
}

// GetModelDisplayName returns a human-readable name for the model
func GetModelDisplayName(model string) string {
	// Dynamic formatting: "claude-3-7-sonnet-20250219" -> "Claude 3.7 Sonnet"
	// This is a rough heuristic for display if not found in map (if we had one)
	if len(model) > 7 && model[:7] == "claude-" {
		// Capitalize first letter, replace hyphens with spaces?
		// Simple static mapping for known ones is fine for UI, but backend mostly passes it through.
		return model // Return ID as fallback
	}
	return model
}

// OneYearInSeconds is the TTL duration for conversations (365 days)
const OneYearInSeconds = 365 * 24 * 60 * 60
