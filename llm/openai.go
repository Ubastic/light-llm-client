package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client *openai.Client
	config Config
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config Config) (*OpenAIProvider, error) {
	// Allow empty API key - validation happens at runtime
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}

	client := openai.NewClientWithConfig(clientConfig)

	// Set defaults only if not provided
	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	// If no provider name is set, use a default
	if config.ProviderName == "" {
		config.ProviderName = "OpenAI Compatible"
	}

	return &OpenAIProvider{
		client: client,
		config: config,
	}, nil
}

// StreamChat implements streaming chat
func (p *OpenAIProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamResponse, error) {
	responseChan := make(chan StreamResponse)

	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		openaiMsg := p.convertMessage(msg)
		openaiMessages = append(openaiMessages, openaiMsg)
	}

	req := openai.ChatCompletionRequest{
		Model:       p.config.Model,
		Messages:    openaiMessages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: float32(p.config.Temperature),
		Stream:      true,
	}

	go func() {
		defer close(responseChan)

		stream, err := p.client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			responseChan <- StreamResponse{Error: fmt.Errorf("failed to create stream: %w", err)}
			return
		}
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				responseChan <- StreamResponse{Done: true}
				return
			}
			if err != nil {
				responseChan <- StreamResponse{Error: fmt.Errorf("stream error: %w", err)}
				return
			}

			if len(response.Choices) > 0 {
				content := response.Choices[0].Delta.Content
				if content != "" {
					responseChan <- StreamResponse{Content: content}
				}
			}
		}
	}()

	return responseChan, nil
}

// convertMessage converts our Message type to OpenAI format, handling attachments
func (p *OpenAIProvider) convertMessage(msg Message) openai.ChatCompletionMessage {
	// If no attachments, return simple text message
	if len(msg.Attachments) == 0 {
		return openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build multimodal message with attachments
	multiContent := []openai.ChatMessagePart{
		{
			Type: openai.ChatMessagePartTypeText,
			Text: msg.Content,
		},
	}

	// Add image attachments
	for _, att := range msg.Attachments {
		if att.Type == "image" {
			// Convert to base64 data URL
			b64 := base64.StdEncoding.EncodeToString(att.Data)
			dataURL := fmt.Sprintf("data:%s;base64,%s", att.MimeType, b64)
			
			multiContent = append(multiContent, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    dataURL,
					Detail: openai.ImageURLDetailAuto,
				},
			})
		}
	}

	return openai.ChatCompletionMessage{
		Role:         msg.Role,
		MultiContent: multiContent,
	}
}

// Chat implements non-streaming chat
func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		openaiMsg := p.convertMessage(msg)
		openaiMessages = append(openaiMessages, openaiMsg)
	}

	req := openai.ChatCompletionRequest{
		Model:       p.config.Model,
		Messages:    openaiMessages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: float32(p.config.Temperature),
	}

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return p.config.ProviderName
}

// Models returns supported models
func (p *OpenAIProvider) Models() []string {
	// Return models from config if available
	if len(p.config.Models) > 0 {
		return p.config.Models
	}
	// Fallback to default models if not configured
	return []string{
		openai.GPT4TurboPreview,
		openai.GPT4,
		openai.GPT3Dot5Turbo,
		"gpt-4-turbo",
		"gpt-4-0125-preview",
	}
}

// GenerateTitle generates a short title based on the conversation
func (p *OpenAIProvider) GenerateTitle(ctx context.Context, messages []Message) (string, error) {
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
func (p *OpenAIProvider) ValidateConfig() error {
	if p.config.APIKey == "" {
		return errors.New("API key is required")
	}
	return nil
}
