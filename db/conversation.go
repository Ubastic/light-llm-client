package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateConversation creates a new conversation
func (db *DB) CreateConversation(title, category string) (*Conversation, error) {
	result, err := db.conn.Exec(
		"INSERT INTO conversations (title, category, created_at, updated_at) VALUES (?, ?, ?, ?)",
		title, category, time.Now(), time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation ID: %w", err)
	}

	return &Conversation{
		ID:        id,
		Title:     title,
		Category:  category,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// GetConversation retrieves a conversation by ID
func (db *DB) GetConversation(id int64) (*Conversation, error) {
	var conv Conversation
	err := db.conn.QueryRow(
		"SELECT id, title, category, created_at, updated_at FROM conversations WHERE id = ?",
		id,
	).Scan(&conv.ID, &conv.Title, &conv.Category, &conv.CreatedAt, &conv.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return &conv, nil
}

// ListConversations retrieves all conversations ordered by update time
func (db *DB) ListConversations(limit, offset int) ([]*Conversation, error) {
	rows, err := db.conn.Query(
		"SELECT id, title, category, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.Category, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// UpdateConversation updates a conversation's title and/or category
func (db *DB) UpdateConversation(id int64, title, category string) error {
	_, err := db.conn.Exec(
		"UPDATE conversations SET title = ?, category = ?, updated_at = ? WHERE id = ?",
		title, category, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}
	return nil
}

// DeleteConversation deletes a conversation and all its messages
func (db *DB) DeleteConversation(id int64) error {
	_, err := db.conn.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	return nil
}

// CountConversations returns the total number of conversations
func (db *DB) CountConversations() (int64, error) {
	var count int64
	err := db.conn.QueryRow("SELECT COUNT(*) FROM conversations").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count conversations: %w", err)
	}
	return count, nil
}

// DeleteOldConversations deletes conversations older than the specified number of days
func (db *DB) DeleteOldConversations(daysOld int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -daysOld)
	result, err := db.conn.Exec(
		"DELETE FROM conversations WHERE updated_at < ?",
		cutoffTime,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old conversations: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	return rowsAffected, nil
}

// DeleteOldestConversations deletes the oldest conversations, keeping only the specified number
func (db *DB) DeleteOldestConversations(keepCount int) (int64, error) {
	result, err := db.conn.Exec(`
		DELETE FROM conversations 
		WHERE id NOT IN (
			SELECT id FROM conversations 
			ORDER BY updated_at DESC 
			LIMIT ?
		)
	`, keepCount)
	
	if err != nil {
		return 0, fmt.Errorf("failed to delete oldest conversations: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	return rowsAffected, nil
}

// TouchConversation updates the conversation's updated_at timestamp
func (db *DB) TouchConversation(id int64) error {
	_, err := db.conn.Exec(
		"UPDATE conversations SET updated_at = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to touch conversation: %w", err)
	}
	return nil
}

// GetCategories retrieves all unique categories from conversations
func (db *DB) GetCategories() ([]string, error) {
	rows, err := db.conn.Query(
		"SELECT DISTINCT category FROM conversations WHERE category != '' ORDER BY category",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}

	return categories, nil
}
