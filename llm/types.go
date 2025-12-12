package llm

import (
	"context"
	"strings"
)

// Message represents a chat message
type Message struct {
	Role        string       `json:"role"` // "user" or "assistant" or "system"
	Content     string       `json:"content"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment represents a file or image attachment
type Attachment struct {
	Type     string `json:"type"`      // "image", "file"
	MimeType string `json:"mime_type"` // "image/png", "text/plain", etc.
	Data     []byte `json:"data"`      // raw data or base64 encoded
	Filename string `json:"filename"`
}

// StreamResponse represents a chunk of streaming response
type StreamResponse struct {
	Content string
	Done    bool
	Error   error
}

// Provider interface defines the common interface for all LLM providers
type Provider interface {
	// StreamChat sends messages and returns a channel for streaming responses
	StreamChat(ctx context.Context, messages []Message) (<-chan StreamResponse, error)

	// Chat sends messages and returns the complete response (non-streaming)
	Chat(ctx context.Context, messages []Message) (string, error)

	// GenerateTitle generates a short title based on the conversation messages
	GenerateTitle(ctx context.Context, messages []Message) (string, error)

	// Name returns the provider name
	Name() string

	// Models returns the list of supported models
	Models() []string

	// ValidateConfig validates the provider configuration
	ValidateConfig() error
}

// Config represents provider configuration
type Config struct {
	ProviderName string   // Display name for the provider
	APIKey       string
	BaseURL      string
	Model        string
	Models       []string // Available models list
	Timeout      int      // seconds
	MaxTokens    int
	Temperature  float64
}

// cleanTitle cleans up a generated title by removing quotes and extra whitespace
func cleanTitle(title string) string {
	// Trim whitespace
	title = strings.TrimSpace(title)
	
	// Remove surrounding quotes (single or double)
	title = strings.Trim(title, "\"'")
	
	// Trim again after removing quotes
	title = strings.TrimSpace(title)
	
	// Limit length to reasonable size
	if len(title) > 100 {
		title = title[:100] + "..."
	}
	
	// If title is empty, return a default
	if title == "" {
		title = "New Chat"
	}
	
	return title
}
