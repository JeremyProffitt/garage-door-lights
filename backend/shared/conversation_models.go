package shared

import "time"

// Conversation represents a Glow Blaster chat session
type Conversation struct {
	ConversationID string    `json:"conversationId" dynamodbav:"conversationId"`
	UserID         string    `json:"userId" dynamodbav:"userId"`
	Title          string    `json:"title" dynamodbav:"title"`
	Messages       []Message `json:"messages" dynamodbav:"messages"`
	CurrentLCL     string    `json:"currentLcl,omitempty" dynamodbav:"currentLcl,omitempty"`
	CurrentBytecode []byte   `json:"currentBytecode,omitempty" dynamodbav:"currentBytecode,omitempty"`
	Model          string    `json:"model" dynamodbav:"model"` // claude-sonnet-4, claude-3-5-sonnet, claude-3-5-haiku
	TotalTokens    int       `json:"totalTokens" dynamodbav:"totalTokens"`
	PatternID      string    `json:"patternId,omitempty" dynamodbav:"patternId,omitempty"` // Associated saved pattern
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
	Message     string   `json:"message"`               // AI response text
	LCL         string   `json:"lcl,omitempty"`         // Updated LCL if pattern changed
	Bytecode    []byte   `json:"bytecode,omitempty"`    // Compiled bytecode for preview
	TokensUsed  int      `json:"tokensUsed"`            // Tokens used in this request
	TotalTokens int      `json:"totalTokens"`           // Total tokens in conversation
	Suggestions []string `json:"suggestions,omitempty"` // Follow-up suggestions
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
	ModelClaudeOpus45   = "claude-opus-4-5-20251101"
	ModelClaudeSonnet4  = "claude-sonnet-4-20250514"
	ModelClaude35Sonnet = "claude-3-5-sonnet-20241022"
	ModelClaude35Haiku  = "claude-3-5-haiku-20241022"
	DefaultModel        = ModelClaudeSonnet4
)

// ValidModels is the list of valid model identifiers
var ValidModels = map[string]bool{
	ModelClaudeOpus45:   true,
	ModelClaudeSonnet4:  true,
	ModelClaude35Sonnet: true,
	ModelClaude35Haiku:  true,
}

// GetModelDisplayName returns a human-readable name for the model
func GetModelDisplayName(model string) string {
	switch model {
	case ModelClaudeOpus45:
		return "Claude Opus 4.5"
	case ModelClaudeSonnet4:
		return "Claude Sonnet 4"
	case ModelClaude35Sonnet:
		return "Claude 3.5 Sonnet"
	case ModelClaude35Haiku:
		return "Claude 3.5 Haiku"
	default:
		return model
	}
}

// OneYearInSeconds is the TTL duration for conversations (365 days)
const OneYearInSeconds = 365 * 24 * 60 * 60
