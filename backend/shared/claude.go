package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	if !IsValidModel(model) {
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

// ExtractLCLFromResponse extracts YAML/LCL from the response text
// Tries code blocks first, then falls back to raw YAML-like text
func ExtractLCLFromResponse(text string) string {
	// First try ```yaml or ```lcl code blocks
	re := regexp.MustCompile(`(?s)` + "```(?:yaml|lcl)" + `\s*\n(.+?)\n` + "```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Fallback: look for raw YAML with "effect:" and "behavior:" or "appearance:"
	re = regexp.MustCompile(`(?s)effect:\s*[a-z]+.*?(?:behavior|appearance):`)
	if re.MatchString(text) {
		// Try to capture the block assuming it starts with effect: and usually ends with empty line or EOF
		// This is a rough heuristic
		start := strings.Index(text, "effect:")
		if start != -1 {
			// Find end - look for double newline or end of string
			rest := text[start:]
			// We take the whole rest of the string or up to a clear delimiter
			// But since we want to be safe, let's just assume the block ends at the next double newline
			// if it's far enough away
			end := strings.Index(rest, "\n\n")
			if end != -1 && end > 20 {
				return strings.TrimSpace(rest[:end])
			}
			return strings.TrimSpace(rest)
		}
	}

	return ""
}

// ClaudeModel represents a model returned by the API

type ClaudeModel struct {

	ID          string `json:"id"`

	DisplayName string `json:"display_name"`

	Created     int64  `json:"created_at"`

}



// ClaudeModelListResponse represents the response from the models endpoint

type ClaudeModelListResponse struct {

	Data []ClaudeModel `json:"data"`

}



// FetchLatestModels fetches available models and returns the latest ID for each family (opus, sonnet, haiku)

func (c *ClaudeClient) FetchLatestModels() (map[string]string, error) {

	if c.apiKey == "" {

		return nil, fmt.Errorf("CLAUDE_API_KEY environment variable not set")

	}



	req, err := http.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)

	if err != nil {

		return nil, fmt.Errorf("failed to create request: %w", err)

	}



	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", ClaudeAPIVersion)



	resp, err := c.httpClient.Do(req)
	if err != nil {

		return nil, fmt.Errorf("failed to fetch models: %w", err)

	}
	defer resp.Body.Close()



	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return nil, fmt.Errorf("failed to read response body: %w", err)

	}



	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("failed to list models: status %d - %s", resp.StatusCode, string(body))

	}



	var listResp ClaudeModelListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {

		return nil, fmt.Errorf("failed to decode models response: %w", err)

	}



	// Logic to find latest models

	latestModels := make(map[string]string)
	
families := []string{"opus", "sonnet", "haiku"}
	

	// Helper to calculate score based on user algorithm

	getScore := func(id string) float64 {
		// "claude-3-5-sonnet-20241022"
		

		// 1. Remove anything but numbers and dashes

		reg := regexp.MustCompile("[^0-9-]")

		clean := reg.ReplaceAllString(id, "")
		

		// 2. Remove leading/trailing dashes

		clean = strings.Trim(clean, "-")
		

		// 3. Replace dashes with periods

		// "3-5--20241022" -> "3.5..20241022"

		clean = strings.ReplaceAll(clean, "-", ".")
		

		// 4. Convert to float

		// Ensure only one decimal point for valid float parsing? 

		// Or assume the user wants "3.5" to be the float. 

		// "3.5.2024..."

		// We'll normalize multiple dots to ensure it parses. 

		// Keep first dot, remove others.

		parts := strings.Split(clean, ".")
	if len(parts) == 0 { return 0 }
		

		floatStr := parts[0]
	if len(parts) > 1 {
			// Join the rest without dots? Or keep decimal precision?

			// "3.5" vs "3.7". 

			// If we have "3.5.2024", this is 3.52024

			floatStr += "." + strings.Join(parts[1:], "")

		}
		

		f, _ := strconv.ParseFloat(floatStr, 64)

		return f

	}



	for _, family := range families {

		var bestID string

		var bestScore float64 = -1.0



		for _, m := range listResp.Data {

			if strings.Contains(m.ID, family) {

				score := getScore(m.ID)

				if score > bestScore {

					bestScore = score

					bestID = m.ID

				}

			}

		}

		if bestID != "" {

			latestModels[family] = bestID

		}

	}



	return latestModels, nil

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