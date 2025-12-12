package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	config Config
	client *http.Client
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(config Config) (*OllamaProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if config.Timeout == 0 {
		config.Timeout = 300 // 5 minutes default
	}
	// If no provider name is set, use a default
	if config.ProviderName == "" {
		config.ProviderName = "Ollama"
	}

	// For streaming responses, we don't want a global timeout
	// Only set connection timeout via Transport
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 120 * time.Second, // Increased for slower models
			// No IdleConnTimeout or overall timeout for streaming
		},
	}

	return &OllamaProvider{
		config: config,
		client: client,
	}, nil
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// StreamChat implements streaming chat
func (p *OllamaProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamResponse, error) {
	responseChan := make(chan StreamResponse)

	// Convert messages to Ollama format
	ollamaMessages := make([]ollamaMessage, 0, len(messages))
	for _, msg := range messages {
		ollamaMessages = append(ollamaMessages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	reqBody := ollamaChatRequest{
		Model:    p.config.Model,
		Messages: ollamaMessages,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		close(responseChan)
		return responseChan, fmt.Errorf("failed to marshal request: %w", err)
	}

	go func() {
		defer close(responseChan)

		req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/chat", bytes.NewBuffer(jsonData))
		if err != nil {
			responseChan <- StreamResponse{Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			responseChan <- StreamResponse{Error: fmt.Errorf("failed to send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			responseChan <- StreamResponse{Error: fmt.Errorf("ollama error: %s", string(body))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var chatResp ollamaChatResponse
			if err := json.Unmarshal(scanner.Bytes(), &chatResp); err != nil {
				responseChan <- StreamResponse{Error: fmt.Errorf("failed to parse response: %w", err)}
				return
			}

			if chatResp.Message.Content != "" {
				responseChan <- StreamResponse{Content: chatResp.Message.Content}
			}

			if chatResp.Done {
				responseChan <- StreamResponse{Done: true}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			responseChan <- StreamResponse{Error: fmt.Errorf("scanner error: %w", err)}
		}
	}()

	return responseChan, nil
}

// Chat implements non-streaming chat
func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	// Convert messages to Ollama format
	ollamaMessages := make([]ollamaMessage, 0, len(messages))
	for _, msg := range messages {
		ollamaMessages = append(ollamaMessages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	reqBody := ollamaChatRequest{
		Model:    p.config.Model,
		Messages: ollamaMessages,
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error: %s", string(body))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return p.config.ProviderName
}

// Models returns supported models (these are examples, actual models depend on what's installed)
func (p *OllamaProvider) Models() []string {
	// Return models from config if available
	if len(p.config.Models) > 0 {
		return p.config.Models
	}
	// Fallback to default models if not configured
	return []string{
		"llama2",
		"llama2:13b",
		"llama2:70b",
		"mistral",
		"mixtral",
		"codellama",
		"phi",
	}
}

// GenerateTitle generates a short title based on the conversation
func (p *OllamaProvider) GenerateTitle(ctx context.Context, messages []Message) (string, error) {
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
func (p *OllamaProvider) ValidateConfig() error {
	if p.config.BaseURL == "" {
		return errors.New("base URL is required")
	}
	return nil
}
