package db

import (
	"fmt"
	"time"
)

// UsageStats represents token usage statistics
type UsageStats struct {
	TotalTokens      int64
	TotalMessages    int64
	ProviderStats    map[string]*ProviderUsageStats
	ModelStats       map[string]*ModelUsageStats
	DailyStats       []*DailyUsageStats
	MonthlyStats     []*MonthlyUsageStats
}

// ProviderUsageStats represents usage statistics for a specific provider
type ProviderUsageStats struct {
	Provider      string
	TotalTokens   int64
	MessageCount  int64
	EstimatedCost float64 // in USD
}

// ModelUsageStats represents usage statistics for a specific model
type ModelUsageStats struct {
	Model         string
	Provider      string
	TotalTokens   int64
	MessageCount  int64
	EstimatedCost float64 // in USD
}

// DailyUsageStats represents daily usage statistics
type DailyUsageStats struct {
	Date         time.Time
	TotalTokens  int64
	MessageCount int64
}

// MonthlyUsageStats represents monthly usage statistics
type MonthlyUsageStats struct {
	Month        string // Format: "2024-12"
	TotalTokens  int64
	MessageCount int64
}

// GetUsageStats returns comprehensive usage statistics
func (db *DB) GetUsageStats(startDate, endDate time.Time) (*UsageStats, error) {
	stats := &UsageStats{
		ProviderStats: make(map[string]*ProviderUsageStats),
		ModelStats:    make(map[string]*ModelUsageStats),
	}
	
	// Get total tokens and messages
	query := `
		SELECT 
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COUNT(*) as total_messages
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
	`
	err := db.conn.QueryRow(query, startDate, endDate).Scan(&stats.TotalTokens, &stats.TotalMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to get total stats: %w", err)
	}
	
	// Get provider statistics
	providerQuery := `
		SELECT 
			provider,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COUNT(*) as message_count
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY provider
		ORDER BY total_tokens DESC
	`
	rows, err := db.conn.Query(providerQuery, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider stats: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var provider string
		var totalTokens, messageCount int64
		if err := rows.Scan(&provider, &totalTokens, &messageCount); err != nil {
			return nil, fmt.Errorf("failed to scan provider stats: %w", err)
		}
		
		stats.ProviderStats[provider] = &ProviderUsageStats{
			Provider:      provider,
			TotalTokens:   totalTokens,
			MessageCount:  messageCount,
			EstimatedCost: calculateCost(provider, "", totalTokens),
		}
	}
	
	// Get model statistics
	modelQuery := `
		SELECT 
			provider,
			model,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COUNT(*) as message_count
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY provider, model
		ORDER BY total_tokens DESC
	`
	rows, err = db.conn.Query(modelQuery, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get model stats: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var provider, model string
		var totalTokens, messageCount int64
		if err := rows.Scan(&provider, &model, &totalTokens, &messageCount); err != nil {
			return nil, fmt.Errorf("failed to scan model stats: %w", err)
		}
		
		key := provider + ":" + model
		stats.ModelStats[key] = &ModelUsageStats{
			Model:         model,
			Provider:      provider,
			TotalTokens:   totalTokens,
			MessageCount:  messageCount,
			EstimatedCost: calculateCost(provider, model, totalTokens),
		}
	}
	
	// Get daily statistics
	dailyQuery := `
		SELECT 
			DATE(created_at) as date,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COUNT(*) as message_count
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	rows, err = db.conn.Query(dailyQuery, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var dateStr string
		var totalTokens, messageCount int64
		if err := rows.Scan(&dateStr, &totalTokens, &messageCount); err != nil {
			return nil, fmt.Errorf("failed to scan daily stats: %w", err)
		}
		
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		
		stats.DailyStats = append(stats.DailyStats, &DailyUsageStats{
			Date:         date,
			TotalTokens:  totalTokens,
			MessageCount: messageCount,
		})
	}
	
	// Get monthly statistics
	monthlyQuery := `
		SELECT 
			strftime('%Y-%m', created_at) as month,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COUNT(*) as message_count
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY strftime('%Y-%m', created_at)
		ORDER BY month ASC
	`
	rows, err = db.conn.Query(monthlyQuery, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly stats: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var month string
		var totalTokens, messageCount int64
		if err := rows.Scan(&month, &totalTokens, &messageCount); err != nil {
			return nil, fmt.Errorf("failed to scan monthly stats: %w", err)
		}
		
		stats.MonthlyStats = append(stats.MonthlyStats, &MonthlyUsageStats{
			Month:        month,
			TotalTokens:  totalTokens,
			MessageCount: messageCount,
		})
	}
	
	return stats, nil
}

// calculateCost estimates the cost based on provider, model, and token count
// This is a simplified estimation - actual costs may vary
func calculateCost(provider, model string, tokens int64) float64 {
	// Cost per 1M tokens (approximate, as of 2024)
	costPer1M := 0.0
	
	switch provider {
	case "openai":
		switch model {
		case "gpt-4", "gpt-4-turbo", "gpt-4-turbo-preview":
			costPer1M = 30.0 // Average of input/output
		case "gpt-4o", "gpt-4o-mini":
			costPer1M = 5.0
		case "gpt-3.5-turbo", "gpt-3.5-turbo-16k":
			costPer1M = 1.5
		default:
			costPer1M = 10.0 // Default estimate
		}
	case "claude", "anthropic":
		switch model {
		case "claude-3-opus-20240229":
			costPer1M = 60.0
		case "claude-3-sonnet-20240229":
			costPer1M = 15.0
		case "claude-3-haiku-20240307":
			costPer1M = 1.25
		default:
			costPer1M = 15.0 // Default estimate
		}
	case "gemini":
		switch model {
		case "gemini-pro", "gemini-1.5-pro":
			costPer1M = 7.0
		case "gemini-1.5-flash":
			costPer1M = 0.35
		default:
			costPer1M = 3.5 // Default estimate
		}
	case "ollama":
		// Ollama is local, so no cost
		return 0.0
	default:
		// Unknown provider, use a conservative estimate
		costPer1M = 10.0
	}
	
	// Calculate cost: (tokens / 1,000,000) * cost_per_1M
	return (float64(tokens) / 1000000.0) * costPer1M
}

// GetProviderUsage returns usage statistics for a specific provider
func (db *DB) GetProviderUsage(provider string, startDate, endDate time.Time) (*ProviderUsageStats, error) {
	stats := &ProviderUsageStats{
		Provider: provider,
	}
	
	query := `
		SELECT 
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COUNT(*) as message_count
		FROM messages
		WHERE provider = ? AND created_at >= ? AND created_at <= ?
	`
	err := db.conn.QueryRow(query, provider, startDate, endDate).Scan(&stats.TotalTokens, &stats.MessageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider usage: %w", err)
	}
	
	stats.EstimatedCost = calculateCost(provider, "", stats.TotalTokens)
	
	return stats, nil
}

// GetTopModels returns the top N models by token usage
func (db *DB) GetTopModels(limit int, startDate, endDate time.Time) ([]*ModelUsageStats, error) {
	query := `
		SELECT 
			provider,
			model,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COUNT(*) as message_count
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY provider, model
		ORDER BY total_tokens DESC
		LIMIT ?
	`
	
	rows, err := db.conn.Query(query, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top models: %w", err)
	}
	defer rows.Close()
	
	var models []*ModelUsageStats
	for rows.Next() {
		var provider, model string
		var totalTokens, messageCount int64
		if err := rows.Scan(&provider, &model, &totalTokens, &messageCount); err != nil {
			return nil, fmt.Errorf("failed to scan model stats: %w", err)
		}
		
		models = append(models, &ModelUsageStats{
			Model:         model,
			Provider:      provider,
			TotalTokens:   totalTokens,
			MessageCount:  messageCount,
			EstimatedCost: calculateCost(provider, model, totalTokens),
		})
	}
	
	return models, nil
}
