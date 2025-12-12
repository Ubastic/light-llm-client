package utils

import (
	"encoding/json"
	"fmt"
	"light-llm-client/db"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportFormat represents the export format
type ExportFormat string

const (
	FormatJSON     ExportFormat = "json"
	FormatMarkdown ExportFormat = "markdown"
)

// ConversationExport represents a conversation export structure
type ConversationExport struct {
	ID        int64              `json:"id"`
	Title     string             `json:"title"`
	Category  string             `json:"category"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	Messages  []MessageExport    `json:"messages"`
	Metadata  map[string]string  `json:"metadata,omitempty"`
}

// MessageExport represents a message export structure
type MessageExport struct {
	ID          int64     `json:"id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Attachments string    `json:"attachments,omitempty"`
	TokensUsed  int       `json:"tokens_used,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExportConversationToJSON exports a single conversation to JSON format
func ExportConversationToJSON(database *db.DB, conversationID int64, filepath string) error {
	// Get conversation
	conv, err := database.GetConversation(conversationID)
	if err != nil {
		return fmt.Errorf("failed to get conversation: %w", err)
	}

	// Get messages
	messages, err := database.ListMessages(conversationID)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	// Build export structure
	export := ConversationExport{
		ID:        conv.ID,
		Title:     conv.Title,
		Category:  conv.Category,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
		Messages:  make([]MessageExport, 0, len(messages)),
		Metadata: map[string]string{
			"export_version": "1.0",
			"export_date":    time.Now().Format(time.RFC3339),
			"app_name":       "Light LLM Client",
		},
	}

	for _, msg := range messages {
		export.Messages = append(export.Messages, MessageExport{
			ID:          msg.ID,
			Role:        msg.Role,
			Content:     msg.Content,
			Provider:    msg.Provider,
			Model:       msg.Model,
			Attachments: msg.Attachments,
			TokensUsed:  msg.TokensUsed,
			CreatedAt:   msg.CreatedAt,
		})
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExportConversationToMarkdown exports a single conversation to Markdown format
func ExportConversationToMarkdown(database *db.DB, conversationID int64, filepath string) error {
	// Get conversation
	conv, err := database.GetConversation(conversationID)
	if err != nil {
		return fmt.Errorf("failed to get conversation: %w", err)
	}

	// Get messages
	messages, err := database.ListMessages(conversationID)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	// Build Markdown content
	var sb strings.Builder
	
	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", conv.Title))
	if conv.Category != "" {
		sb.WriteString(fmt.Sprintf("**分类**: %s\n\n", conv.Category))
	}
	sb.WriteString(fmt.Sprintf("**创建时间**: %s\n", conv.CreatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**更新时间**: %s\n\n", conv.UpdatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString("---\n\n")

	// Messages
	for i, msg := range messages {
		// Role header
		roleIcon := "👤"
		roleName := "用户"
		if msg.Role == "assistant" {
			roleIcon = "🤖"
			roleName = "助手"
		} else if msg.Role == "system" {
			roleIcon = "⚙️"
			roleName = "系统"
		}

		sb.WriteString(fmt.Sprintf("## %s %s\n\n", roleIcon, roleName))
		
		// Metadata
		if msg.Provider != "" || msg.Model != "" {
			sb.WriteString(fmt.Sprintf("*%s - %s*\n\n", msg.Provider, msg.Model))
		}
		
		// Content
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
		
		// Separator (except for last message)
		if i < len(messages)-1 {
			sb.WriteString("---\n\n")
		}
	}

	// Footer
	sb.WriteString("\n---\n\n")
	sb.WriteString(fmt.Sprintf("*导出时间: %s*\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("*导出工具: Light LLM Client*\n")

	// Write to file
	if err := os.WriteFile(filepath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExportAllConversations exports all conversations to a single JSON file
func ExportAllConversations(database *db.DB, filepath string) error {
	// Get all conversations
	conversations, err := database.ListConversations(10000, 0) // Large limit to get all
	if err != nil {
		return fmt.Errorf("failed to list conversations: %w", err)
	}

	exports := make([]ConversationExport, 0, len(conversations))

	for _, conv := range conversations {
		// Get messages for this conversation
		messages, err := database.ListMessages(conv.ID)
		if err != nil {
			return fmt.Errorf("failed to get messages for conversation %d: %w", conv.ID, err)
		}

		export := ConversationExport{
			ID:        conv.ID,
			Title:     conv.Title,
			Category:  conv.Category,
			CreatedAt: conv.CreatedAt,
			UpdatedAt: conv.UpdatedAt,
			Messages:  make([]MessageExport, 0, len(messages)),
		}

		for _, msg := range messages {
			export.Messages = append(export.Messages, MessageExport{
				ID:          msg.ID,
				Role:        msg.Role,
				Content:     msg.Content,
				Provider:    msg.Provider,
				Model:       msg.Model,
				Attachments: msg.Attachments,
				TokensUsed:  msg.TokensUsed,
				CreatedAt:   msg.CreatedAt,
			})
		}

		exports = append(exports, export)
	}

	// Create wrapper with metadata
	wrapper := map[string]interface{}{
		"metadata": map[string]string{
			"export_version": "1.0",
			"export_date":    time.Now().Format(time.RFC3339),
			"app_name":       "Light LLM Client",
			"total_count":    fmt.Sprintf("%d", len(exports)),
		},
		"conversations": exports,
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ImportConversation imports a conversation from a JSON file
func ImportConversation(database *db.DB, filepath string) (*db.Conversation, error) {
	// Read file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal JSON
	var export ConversationExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Validate
	if export.Title == "" {
		return nil, fmt.Errorf("invalid export: missing title")
	}
	if len(export.Messages) == 0 {
		return nil, fmt.Errorf("invalid export: no messages")
	}

	// Create new conversation (don't use the original ID)
	conv, err := database.CreateConversation(export.Title, export.Category)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// Import messages
	for _, msgExport := range export.Messages {
		_, err := database.CreateMessage(
			conv.ID,
			msgExport.Role,
			msgExport.Content,
			msgExport.Provider,
			msgExport.Model,
			msgExport.Attachments,
			msgExport.TokensUsed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create message: %w", err)
		}
	}

	return conv, nil
}

// ImportAllConversations imports multiple conversations from a JSON file
func ImportAllConversations(database *db.DB, filepath string) (int, error) {
	// Read file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal JSON
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Get conversations array
	conversationsData, ok := wrapper["conversations"]
	if !ok {
		return 0, fmt.Errorf("invalid export: missing conversations array")
	}

	// Re-marshal and unmarshal to get proper structure
	conversationsJSON, err := json.Marshal(conversationsData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal conversations: %w", err)
	}

	var exports []ConversationExport
	if err := json.Unmarshal(conversationsJSON, &exports); err != nil {
		return 0, fmt.Errorf("failed to unmarshal conversations: %w", err)
	}

	// Import each conversation
	count := 0
	for _, export := range exports {
		// Validate
		if export.Title == "" || len(export.Messages) == 0 {
			continue
		}

		// Create conversation
		conv, err := database.CreateConversation(export.Title, export.Category)
		if err != nil {
			return count, fmt.Errorf("failed to create conversation: %w", err)
		}

		// Import messages
		for _, msgExport := range export.Messages {
			_, err := database.CreateMessage(
				conv.ID,
				msgExport.Role,
				msgExport.Content,
				msgExport.Provider,
				msgExport.Model,
				msgExport.Attachments,
				msgExport.TokensUsed,
			)
			if err != nil {
				return count, fmt.Errorf("failed to create message: %w", err)
			}
		}

		count++
	}

	return count, nil
}

// GenerateExportFilename generates a filename for export
func GenerateExportFilename(title string, format ExportFormat) string {
	// Sanitize title for filename
	sanitized := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, title)

	// Truncate if too long
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	// Add timestamp and extension
	timestamp := time.Now().Format("20060102_150405")
	ext := string(format)
	if format == FormatMarkdown {
		ext = "md"
	}

	return fmt.Sprintf("%s_%s.%s", sanitized, timestamp, ext)
}

// GetDefaultExportPath returns the default export directory
func GetDefaultExportPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	exportDir := filepath.Join(homeDir, "Documents", "LLM_Exports")
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", err
	}

	return exportDir, nil
}
