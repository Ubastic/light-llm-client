package db

import "time"

// Conversation represents a chat conversation
type Conversation struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a single message in a conversation
type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	Role           string    `json:"role"` // "user" or "assistant"
	Content        string    `json:"content"`
	OriginalContent string    `json:"original_content"` // Original content before anonymization
	Provider       string    `json:"provider"` // "openai", "claude", etc.
	Model          string    `json:"model"`
	Attachments    string    `json:"attachments"` // JSON array
	TokensUsed     int       `json:"tokens_used"`
	CreatedAt      time.Time `json:"created_at"`
}

// Setting represents a configuration setting
type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
