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

// GeminiProvider implements the Provider interface for Google Gemini
type GeminiProvider struct {
	apiKey  string
	baseURL string
	config  Config
	client  *http.Client
}

// GeminiContent represents content in Gemini's format
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

// GeminiPart represents a part of content
type GeminiPart struct {
	Text       string           `json:"text,omitempty"`
	InlineData *GeminiInlineData `json:"inlineData,omitempty"`
}

// GeminiInlineData represents inline data (e.g., images)
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64 encoded
}

// GeminiRequest represents a request to Gemini API
type GeminiRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings   []GeminiSafetySetting   `json:"safetySettings,omitempty"`
}

// GeminiGenerationConfig represents generation configuration
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

// GeminiSafetySetting represents safety settings
type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiResponse represents a response from Gemini API
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []GeminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	PromptFeedback *struct {
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"promptFeedback,omitempty"`
}

// NewGeminiProvider creates a new Gemini provider
func NewGeminiProvider(config Config) (*GeminiProvider, error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	// Set defaults
	if config.MaxTokens == 0 {
		config.MaxTokens = 8192
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.Model == "" {
		config.Model = "gemini-1.5-flash"
	}
	if config.ProviderName == "" {
		config.ProviderName = "Gemini"
	}

	return &GeminiProvider{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		config:  config,
		client:  &http.Client{},
	}, nil
}

// StreamChat implements streaming chat
func (p *GeminiProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamResponse, error) {
	responseChan := make(chan StreamResponse)

	// Convert messages to Gemini format
	geminiContents := p.convertMessages(messages)

	req := GeminiRequest{
		Contents: geminiContents,
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     p.config.Temperature,
			MaxOutputTokens: p.config.MaxTokens,
		},
		SafetySettings: p.getDefaultSafetySettings(),
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
func (p *GeminiProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	// Convert messages to Gemini format
	geminiContents := p.convertMessages(messages)

	req := GeminiRequest{
		Contents: geminiContents,
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     p.config.Temperature,
			MaxOutputTokens: p.config.MaxTokens,
		},
		SafetySettings: p.getDefaultSafetySettings(),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL with API key
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.config.Model, p.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", errors.New("no candidates in response")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("no content in response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// Name returns the provider name
func (p *GeminiProvider) Name() string {
	return p.config.ProviderName
}

// Models returns supported models
func (p *GeminiProvider) Models() []string {
	// Return models from config if available
	if len(p.config.Models) > 0 {
		return p.config.Models
	}
	// Fallback to default Gemini models
	return []string{
		"gemini-1.5-flash",
		"gemini-1.5-pro",
		"gemini-1.0-pro",
		"gemini-2.0-flash-exp",
	}
}

// GenerateTitle generates a short title based on the conversation
func (p *GeminiProvider) GenerateTitle(ctx context.Context, messages []Message) (string, error) {
	// Build a prompt to generate a title
	titlePrompt := []Message{
		{
			Role:    "user",
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
func (p *GeminiProvider) ValidateConfig() error {
	if p.apiKey == "" {
		return errors.New("API key is required")
	}
	return nil
}

// convertMessages converts our Message format to Gemini's format
// Gemini uses a different message structure with contents and parts
func (p *GeminiProvider) convertMessages(messages []Message) []GeminiContent {
	var geminiContents []GeminiContent
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

	// If there's a system prompt, prepend it to the first user message
	firstUserMsgIndex := -1
	for i, msg := range messages {
		if msg.Role == "user" {
			firstUserMsgIndex = i
			break
		}
	}

	// Convert user and assistant messages
	for i, msg := range messages {
		if msg.Role == "system" {
			continue // Already handled
		}

		content := msg.Content
		// Prepend system prompt to first user message
		if i == firstUserMsgIndex && systemPrompt != "" {
			content = systemPrompt + "\n\n" + content
		}

		// Map roles: Gemini uses "user" and "model" instead of "assistant"
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		// Build parts array
		parts := []GeminiPart{{Text: content}}
		
		// Add image attachments
		for _, att := range msg.Attachments {
			if att.Type == "image" {
				b64 := base64.StdEncoding.EncodeToString(att.Data)
				parts = append(parts, GeminiPart{
					InlineData: &GeminiInlineData{
						MimeType: att.MimeType,
						Data:     b64,
					},
				})
			}
		}
		
		geminiContents = append(geminiContents, GeminiContent{
			Parts: parts,
			Role:  role,
		})
	}

	return geminiContents
}

// getDefaultSafetySettings returns default safety settings
func (p *GeminiProvider) getDefaultSafetySettings() []GeminiSafetySetting {
	categories := []string{
		"HARM_CATEGORY_HARASSMENT",
		"HARM_CATEGORY_HATE_SPEECH",
		"HARM_CATEGORY_SEXUALLY_EXPLICIT",
		"HARM_CATEGORY_DANGEROUS_CONTENT",
	}

	settings := make([]GeminiSafetySetting, len(categories))
	for i, category := range categories {
		settings[i] = GeminiSafetySetting{
			Category:  category,
			Threshold: "BLOCK_MEDIUM_AND_ABOVE",
		}
	}

	return settings
}

// streamRequest handles the streaming request to Gemini API
func (p *GeminiProvider) streamRequest(ctx context.Context, req GeminiRequest, responseChan chan<- StreamResponse) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL with API key for streaming
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", p.baseURL, p.config.Model, p.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

		// Skip empty data
		if data == "" || data == "[DONE]" {
			continue
		}

		var geminiResp GeminiResponse
		if err := json.Unmarshal([]byte(data), &geminiResp); err != nil {
			// Skip malformed events
			continue
		}

		// Extract text from response
		if len(geminiResp.Candidates) > 0 {
			candidate := geminiResp.Candidates[0]
			if len(candidate.Content.Parts) > 0 {
				text := candidate.Content.Parts[0].Text
				if text != "" {
					responseChan <- StreamResponse{Content: text}
				}
			}

			// Check if this is the final chunk
			if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
				// Handle non-normal finish reasons
				if candidate.FinishReason == "SAFETY" {
					responseChan <- StreamResponse{Error: errors.New("response blocked by safety filters")}
					return nil
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	responseChan <- StreamResponse{Done: true}
	return nil
}
