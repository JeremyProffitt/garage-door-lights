package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"
)

const ClaudeAPIURL = "https://api.anthropic.com/v1/messages"
const ClaudeAPIVersion = "2023-06-01"

// ClaudeClient wraps the Anthropic Claude API
type ClaudeClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewClaudeClient creates a new Claude API client
func NewClaudeClient() *ClaudeClient {
	return &ClaudeClient{
		apiKey: os.Getenv("CLAUDE_API_KEY"),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ClaudeRequest represents a request to the Claude API
type ClaudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []ClaudeMessage `json:"messages"`
}

// ClaudeMessage represents a message in the Claude API
type ClaudeMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ClaudeResponse represents a response from the Claude API
type ClaudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ClaudeError represents an error from the Claude API
type ClaudeError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// SendMessage sends a message to Claude and returns the response
func (c *ClaudeClient) SendMessage(model, systemPrompt string, messages []ClaudeMessage) (*ClaudeResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("CLAUDE_API_KEY environment variable not set")
	}

	// Validate model
	if !ValidModels[model] {
		model = DefaultModel
	}

	request := ClaudeRequest{
		Model:     model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  messages,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", ClaudeAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", ClaudeAPIVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var claudeErr ClaudeError
		if err := json.Unmarshal(body, &claudeErr); err == nil {
			return nil, fmt.Errorf("Claude API error: %s - %s", claudeErr.Error.Type, claudeErr.Error.Message)
		}
		return nil, fmt.Errorf("Claude API error: status %d - %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &claudeResp, nil
}

// GetResponseText extracts the text content from a Claude response
func (c *ClaudeClient) GetResponseText(resp *ClaudeResponse) string {
	if len(resp.Content) > 0 && resp.Content[0].Type == "text" {
		return resp.Content[0].Text
	}
	return ""
}

// ExtractLCLFromResponse extracts LCL code blocks from the response text
func ExtractLCLFromResponse(text string) string {
	// Look for ```lcl ... ``` code blocks
	re := regexp.MustCompile("(?s)```lcl\\s*\\n(.+?)\\n```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}

	// Also try ```yaml ... ``` for intent layer
	re = regexp.MustCompile("(?s)```yaml\\s*\\n(effect:.+?)\\n```")
	matches = re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// ExtractDescriptionFromLCL extracts the description field from LCL text
func ExtractDescriptionFromLCL(lclText string) string {
	re := regexp.MustCompile(`(?m)^description:\s*["']?(.+?)["']?\s*$`)
	matches := re.FindStringSubmatch(lclText)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ConvertMessagesToClaudeFormat converts conversation messages to Claude API format
func ConvertMessagesToClaudeFormat(messages []Message) []ClaudeMessage {
	claudeMessages := make([]ClaudeMessage, 0, len(messages))
	for _, msg := range messages {
		claudeMessages = append(claudeMessages, ClaudeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return claudeMessages
}
