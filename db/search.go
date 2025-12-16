package db

import "fmt"

// SearchResult represents a search result
type SearchResult struct {
	Message        *Message
	ConversationID int64
	Snippet        string
}

// SearchMessages performs full-text search on messages
func (db *DB) SearchMessages(query string, limit int) ([]*SearchResult, error) {
	rows, err := db.conn.Query(`
		SELECT m.id, m.conversation_id, m.role, m.content, m.original_content, m.provider, m.model, m.attachments, m.tokens_used, m.created_at,
		       snippet(messages_fts, 0, '<mark>', '</mark>', '...', 32) as snippet
		FROM messages_fts
		JOIN messages m ON messages_fts.rowid = m.id
		WHERE messages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		var msg Message
		var snippet string
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.OriginalContent, &msg.Provider, &msg.Model, &msg.Attachments, &msg.TokensUsed, &msg.CreatedAt, &snippet); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		results = append(results, &SearchResult{
			Message:        &msg,
			ConversationID: msg.ConversationID,
			Snippet:        snippet,
		})
	}

	return results, nil
}

// SearchMessagesWithFilters performs full-text search with optional filters
func (db *DB) SearchMessagesWithFilters(query string, provider string, category string, daysAgo int, limit int) ([]*SearchResult, error) {
	// Build query with filters
	sqlQuery := `
		SELECT m.id, m.conversation_id, m.role, m.content, m.original_content, m.provider, m.model, m.attachments, m.tokens_used, m.created_at,
		       snippet(messages_fts, 0, '<mark>', '</mark>', '...', 32) as snippet
		FROM messages_fts
		JOIN messages m ON messages_fts.rowid = m.id
		JOIN conversations c ON m.conversation_id = c.id
		WHERE messages_fts MATCH ?`
	
	args := []interface{}{query}
	
	// Add provider filter if specified
	if provider != "" && provider != "全部提供商" {
		sqlQuery += " AND m.provider = ?"
		args = append(args, provider)
	}
	
	// Add category filter if specified
	if category != "" && category != "全部分类" {
		sqlQuery += " AND c.category = ?"
		args = append(args, category)
	}
	
	// Add date filter if specified
	if daysAgo > 0 {
		sqlQuery += " AND m.created_at >= datetime('now', '-' || ? || ' days')"
		args = append(args, daysAgo)
	}
	
	sqlQuery += " ORDER BY rank LIMIT ?"
	args = append(args, limit)
	
	rows, err := db.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages with filters: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		var msg Message
		var snippet string
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.OriginalContent, &msg.Provider, &msg.Model, &msg.Attachments, &msg.TokensUsed, &msg.CreatedAt, &snippet); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		results = append(results, &SearchResult{
			Message:        &msg,
			ConversationID: msg.ConversationID,
			Snippet:        snippet,
		})
	}

	return results, nil
}

// SearchConversationsByCategory searches conversations by category
func (db *DB) SearchConversationsByCategory(category string) ([]*Conversation, error) {
	rows, err := db.conn.Query(
		"SELECT id, title, category, created_at, updated_at FROM conversations WHERE category = ? ORDER BY updated_at DESC",
		category,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search conversations by category: %w", err)
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
