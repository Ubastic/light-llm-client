package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ClaudeProvider implements the Provider interface for Anthropic Claude
type ClaudeProvider struct {
	apiKey  string
	baseURL string
	config  Config
	client  *http.Client
}

// ClaudeMessage represents a message in Claude's format
type ClaudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ClaudeContentBlock
}

// ClaudeContentBlock represents a content block in Claude's multimodal format
type ClaudeContentBlock struct {
	Type   string                `json:"type"` // "text" or "image"
	Text   string                `json:"text,omitempty"`
	Source *ClaudeImageSource    `json:"source,omitempty"`
}

// ClaudeImageSource represents an image source in Claude's format
type ClaudeImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/jpeg", "image/png", etc.
	Data      string `json:"data"`       // base64 encoded image data
}

// ClaudeRequest represents a request to Claude API
type ClaudeRequest struct {
	Model       string          `json:"model"`
	Messages    []ClaudeMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
	System      string          `json:"system,omitempty"`
}

// ClaudeResponse represents a response from Claude API
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

// ClaudeStreamEvent represents a streaming event from Claude API
type ClaudeStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	Message      *ClaudeResponse `json:"message,omitempty"`
	ContentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block,omitempty"`
}

// NewClaudeProvider creates a new Claude provider
func NewClaudeProvider(config Config) (*ClaudeProvider, error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	// Set defaults
	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.Model == "" {
		config.Model = "claude-3-5-sonnet-20241022"
	}
	if config.ProviderName == "" {
		config.ProviderName = "Claude"
	}

	return &ClaudeProvider{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		config:  config,
		client:  &http.Client{},
	}, nil
}

// StreamChat implements streaming chat
func (p *ClaudeProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamResponse, error) {
	responseChan := make(chan StreamResponse)

	// Convert messages to Claude format and extract system message
	claudeMessages, systemPrompt := p.convertMessages(messages)

	req := ClaudeRequest{
		Model:       p.config.Model,
		Messages:    claudeMessages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      true,
		System:      systemPrompt,
	}

	go func() {
		defer close(responseChan)

		if err := p.streamRequest(ctx, req, responseChan); err != nil {
			responseChan <- StreamResponse{Error: err}
		}
	}()

	return responseChan, nil
}

// Chat implements non-streaming chat
func (p *ClaudeProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	// Convert messages to Claude format and extract system message
	claudeMessages, systemPrompt := p.convertMessages(messages)

	req := ClaudeRequest{
		Model:       p.config.Model,
		Messages:    claudeMessages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      false,
		System:      systemPrompt,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", errors.New("no content in response")
	}

	return claudeResp.Content[0].Text, nil
}

// Name returns the provider name
func (p *ClaudeProvider) Name() string {
	return p.config.ProviderName
}

// Models returns supported models
func (p *ClaudeProvider) Models() []string {
	// Return models from config if available
	if len(p.config.Models) > 0 {
		return p.config.Models
	}
	// Fallback to default Claude models
	return []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}
}

// GenerateTitle generates a short title based on the conversation
func (p *ClaudeProvider) GenerateTitle(ctx context.Context, messages []Message) (string, error) {
	// Build a prompt to generate a title
	titlePrompt := []Message{
		{
			Role:    "system",
			Content: "You are a helpful assistant that generates short, concise titles for conversations. Generate a title in the same language as the conversation (Chinese or English). The title should be 3-8 words, descriptive, and capture the main topic. Only output the title, nothing else.",
		},
	}

	// Add the first few messages for context (limit to avoid token issues)
	maxMessages := 4
	for i, msg := range messages {
		if i >= maxMessages {
			break
		}
		titlePrompt = append(titlePrompt, msg)
	}

	titlePrompt = append(titlePrompt, Message{
		Role:    "user",
		Content: "Based on the above conversation, generate a short title (3-8 words):",
	})

	// Use Chat method to get the title
	title, err := p.Chat(ctx, titlePrompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	// Clean up the title (remove quotes, trim whitespace)
	title = cleanTitle(title)

	return title, nil
}

// ValidateConfig validates the configuration
func (p *ClaudeProvider) ValidateConfig() error {
	if p.apiKey == "" {
		return errors.New("API key is required")
	}
	return nil
}

// convertMessages converts our Message format to Claude's format
// Claude requires alternating user/assistant messages and extracts system messages separately
func (p *ClaudeProvider) convertMessages(messages []Message) ([]ClaudeMessage, string) {
	var claudeMessages []ClaudeMessage
	var systemPrompt string

	// Extract system message if present
	for _, msg := range messages {
		if msg.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += msg.Content
		}
	}

	// Convert user and assistant messages
	for _, msg := range messages {
		if msg.Role == "system" {
			continue // Already handled
		}
		
		// Check if message has attachments
		if len(msg.Attachments) == 0 {
			// Simple text message
			claudeMessages = append(claudeMessages, ClaudeMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		} else {
			// Multimodal message with content blocks
			contentBlocks := []ClaudeContentBlock{
				{
					Type: "text",
					Text: msg.Content,
				},
			}
			
			// Add image attachments
			for _, att := range msg.Attachments {
				if att.Type == "image" {
					b64 := base64.StdEncoding.EncodeToString(att.Data)
					contentBlocks = append(contentBlocks, ClaudeContentBlock{
						Type: "image",
						Source: &ClaudeImageSource{
							Type:      "base64",
							MediaType: att.MimeType,
							Data:      b64,
						},
					})
				}
			}
			
			claudeMessages = append(claudeMessages, ClaudeMessage{
				Role:    msg.Role,
				Content: contentBlocks,
			})
		}
	}

	return claudeMessages, systemPrompt
}

// setHeaders sets the required headers for Claude API requests
func (p *ClaudeProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
}

// streamRequest handles the streaming request to Claude API
func (p *ClaudeProvider) streamRequest(ctx context.Context, req ClaudeRequest, responseChan chan<- StreamResponse) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {...}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Skip [DONE] message
		if data == "[DONE]" {
			responseChan <- StreamResponse{Done: true}
			return nil
		}

		var event ClaudeStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			// Skip malformed events
			continue
		}

		// Handle different event types
		switch event.Type {
		case "content_block_delta":
			if event.Delta.Text != "" {
				responseChan <- StreamResponse{Content: event.Delta.Text}
			}
		case "message_stop":
			responseChan <- StreamResponse{Done: true}
			return nil
		case "error":
			return fmt.Errorf("stream error: %s", data)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	responseChan <- StreamResponse{Done: true}
	return nil
}
