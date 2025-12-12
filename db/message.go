package db

import (
	"fmt"
	"time"
)

// CreateMessage creates a new message in a conversation
func (db *DB) CreateMessage(conversationID int64, role, content, provider, model, attachments string, tokensUsed int) (*Message, error) {
	result, err := db.conn.Exec(
		"INSERT INTO messages (conversation_id, role, content, provider, model, attachments, tokens_used, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		conversationID, role, content, provider, model, attachments, tokensUsed, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get message ID: %w", err)
	}

	// Update conversation's updated_at timestamp
	if err := db.TouchConversation(conversationID); err != nil {
		return nil, err
	}

	return &Message{
		ID:             id,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Provider:       provider,
		Model:          model,
		Attachments:    attachments,
		TokensUsed:     tokensUsed,
		CreatedAt:      time.Now(),
	}, nil
}

// GetMessage retrieves a message by ID
func (db *DB) GetMessage(id int64) (*Message, error) {
	var msg Message
	err := db.conn.QueryRow(
		"SELECT id, conversation_id, role, content, provider, model, attachments, tokens_used, created_at FROM messages WHERE id = ?",
		id,
	).Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.Provider, &msg.Model, &msg.Attachments, &msg.TokensUsed, &msg.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &msg, nil
}

// ListMessages retrieves all messages in a conversation
func (db *DB) ListMessages(conversationID int64) ([]*Message, error) {
	rows, err := db.conn.Query(
		"SELECT id, conversation_id, role, content, provider, model, attachments, tokens_used, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC",
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.Provider, &msg.Model, &msg.Attachments, &msg.TokensUsed, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// UpdateMessage updates a message's content
func (db *DB) UpdateMessage(id int64, content string) error {
	_, err := db.conn.Exec(
		"UPDATE messages SET content = ? WHERE id = ?",
		content, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}
	return nil
}

// DeleteMessage deletes a message
func (db *DB) DeleteMessage(id int64) error {
	_, err := db.conn.Exec("DELETE FROM messages WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}
