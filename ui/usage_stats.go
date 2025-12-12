package ui

import (
	"fmt"
	"image/color"
	"light-llm-client/db"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// UsageStatsView represents the usage statistics interface
type UsageStatsView struct {
	app                    *App
	statsLabel             *widget.Label
	providerList           *widget.List
	modelList              *widget.List
	providerStatsContainer *fyne.Container
	modelStatsContainer    *fyne.Container
	chartCanvas            *fyne.Container
	dateRangeSelect        *widget.Select
	
	// Current stats
	currentStats  *db.UsageStats
	startDate     time.Time
	endDate       time.Time
}

// NewUsageStatsView creates a new usage statistics view
func NewUsageStatsView(app *App) *UsageStatsView {
	usv := &UsageStatsView{
		app: app,
	}
	
	// Default to last 30 days
	usv.endDate = time.Now()
	usv.startDate = usv.endDate.AddDate(0, 0, -30)
	
	return usv
}

// Build builds the usage statistics view UI
func (usv *UsageStatsView) Build() fyne.CanvasObject {
	// Overall statistics - create this FIRST
	usv.statsLabel = widget.NewLabel("Loading statistics...")
	usv.statsLabel.Wrapping = fyne.TextWrapWord
	
	// Date range selector
	usv.dateRangeSelect = widget.NewSelect(
		[]string{"Last 7 Days", "Last 30 Days", "Last 90 Days", "This Month", "Last Month", "All Time"},
		func(value string) {
			usv.updateDateRange(value)
			usv.refreshStats()
		},
	)
	
	// Refresh button
	refreshBtn := widget.NewButton("Refresh", func() {
		usv.refreshStats()
	})
	
	// Header with date range and refresh
	header := container.NewBorder(
		nil, nil,
		widget.NewLabel("Date Range:"),
		refreshBtn,
		usv.dateRangeSelect,
	)
	
	overallCard := usv.createCard("Overall Statistics", usv.statsLabel)
	
	// Provider statistics - use VBox for flat layout instead of List
	usv.providerStatsContainer = container.NewVBox()
	providerCard := usv.createCard("Provider Breakdown", usv.providerStatsContainer)
	
	// Model statistics - use VBox for flat layout instead of List
	usv.modelStatsContainer = container.NewVBox()
	modelCard := usv.createCard("Model Breakdown", usv.modelStatsContainer)
	
	// Chart canvas
	usv.chartCanvas = container.NewVBox()
	chartCard := usv.createCard("Usage Over Time", usv.chartCanvas)
	
	// Layout - use VBox with better spacing
	leftPanel := container.NewVBox(
		overallCard,
		providerCard,
	)
	
	rightPanel := container.NewVBox(
		chartCard,
		modelCard,
	)
	
	content := container.NewHSplit(leftPanel, rightPanel)
	content.SetOffset(0.4)
	
	mainContent := container.NewBorder(
		header,
		nil, nil, nil,
		content,
	)
	
	// Set default selection AFTER all widgets are created
	usv.dateRangeSelect.SetSelected("Last 30 Days")
	
	return container.NewVScroll(mainContent)
}

// createCard creates a card-style container
func (usv *UsageStatsView) createCard(title string, content fyne.CanvasObject) *fyne.Container {
	titleLabel := widget.NewLabel(title)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	return container.NewBorder(
		container.NewVBox(
			titleLabel,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		content,
	)
}

// updateDateRange updates the date range based on selection
func (usv *UsageStatsView) updateDateRange(selection string) {
	now := time.Now()
	usv.endDate = now
	
	switch selection {
	case "Last 7 Days":
		usv.startDate = now.AddDate(0, 0, -7)
	case "Last 30 Days":
		usv.startDate = now.AddDate(0, 0, -30)
	case "Last 90 Days":
		usv.startDate = now.AddDate(0, 0, -90)
	case "This Month":
		usv.startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "Last Month":
		lastMonth := now.AddDate(0, -1, 0)
		usv.startDate = time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, now.Location())
		usv.endDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)
	case "All Time":
		usv.startDate = time.Date(2020, 1, 1, 0, 0, 0, 0, now.Location())
	}
}

// refreshStats refreshes the statistics
func (usv *UsageStatsView) refreshStats() {
	stats, err := usv.app.db.GetUsageStats(usv.startDate, usv.endDate)
	if err != nil {
		usv.app.logger.Error("Failed to get usage stats: %v", err)
		usv.statsLabel.SetText("Failed to load statistics")
		return
	}
	
	usv.currentStats = stats
	
	// Update overall stats
	totalCost := 0.0
	for _, ps := range stats.ProviderStats {
		totalCost += ps.EstimatedCost
	}
	
	overallText := fmt.Sprintf(
		"Total Tokens: %s\n"+
		"Total Messages: %d\n"+
		"Estimated Total Cost: $%.4f\n"+
		"Date Range: %s to %s",
		formatNumber(stats.TotalTokens),
		stats.TotalMessages,
		totalCost,
		usv.startDate.Format("2006-01-02"),
		usv.endDate.Format("2006-01-02"),
	)
	usv.statsLabel.SetText(overallText)
	
	// Update provider stats
	usv.updateProviderStats()
	
	// Update model stats
	usv.updateModelStats()
	
	// Update chart
	usv.updateChart()
}

// updateProviderStats updates the provider statistics display
func (usv *UsageStatsView) updateProviderStats() {
	usv.providerStatsContainer.Objects = nil
	
	if usv.currentStats == nil || len(usv.currentStats.ProviderStats) == 0 {
		usv.providerStatsContainer.Add(widget.NewLabel("No provider data available"))
		usv.providerStatsContainer.Refresh()
		return
	}
	
	// Get sorted providers
	providers := usv.getSortedProviders()
	
	for _, provider := range providers {
		stats := usv.currentStats.ProviderStats[provider]
		
		// Provider name label
		nameLabel := widget.NewLabel(fmt.Sprintf("📊 %s", stats.Provider))
		nameLabel.TextStyle = fyne.TextStyle{Bold: true}
		
		// Details label
		details := fmt.Sprintf(
			"   Tokens: %s | Messages: %d | Est. Cost: $%.4f",
			formatNumber(stats.TotalTokens),
			stats.MessageCount,
			stats.EstimatedCost,
		)
		detailsLabel := widget.NewLabel(details)
		
		usv.providerStatsContainer.Add(nameLabel)
		usv.providerStatsContainer.Add(detailsLabel)
		usv.providerStatsContainer.Add(widget.NewSeparator())
	}
	
	usv.providerStatsContainer.Refresh()
}

// updateModelStats updates the model statistics display
func (usv *UsageStatsView) updateModelStats() {
	usv.modelStatsContainer.Objects = nil
	
	if usv.currentStats == nil || len(usv.currentStats.ModelStats) == 0 {
		usv.modelStatsContainer.Add(widget.NewLabel("No model data available"))
		usv.modelStatsContainer.Refresh()
		return
	}
	
	// Get sorted models
	models := usv.getSortedModels()
	
	for _, key := range models {
		stats := usv.currentStats.ModelStats[key]
		
		// Model name label
		nameLabel := widget.NewLabel(fmt.Sprintf("🤖 %s (%s)", stats.Model, stats.Provider))
		nameLabel.TextStyle = fyne.TextStyle{Bold: true}
		
		// Details label
		details := fmt.Sprintf(
			"   Tokens: %s | Messages: %d | Est. Cost: $%.4f",
			formatNumber(stats.TotalTokens),
			stats.MessageCount,
			stats.EstimatedCost,
		)
		detailsLabel := widget.NewLabel(details)
		
		usv.modelStatsContainer.Add(nameLabel)
		usv.modelStatsContainer.Add(detailsLabel)
		usv.modelStatsContainer.Add(widget.NewSeparator())
	}
	
	usv.modelStatsContainer.Refresh()
}

// updateChart updates the usage chart
func (usv *UsageStatsView) updateChart() {
	usv.chartCanvas.Objects = nil
	
	if usv.currentStats == nil || len(usv.currentStats.DailyStats) == 0 {
		usv.chartCanvas.Add(widget.NewLabel("No data available for chart"))
		usv.chartCanvas.Refresh()
		return
	}
	
	// Create a simple bar chart using canvas rectangles
	chart := usv.createBarChart(usv.currentStats.DailyStats)
	usv.chartCanvas.Add(chart)
	usv.chartCanvas.Refresh()
}

// createBarChart creates a simple bar chart
func (usv *UsageStatsView) createBarChart(dailyStats []*db.DailyUsageStats) fyne.CanvasObject {
	if len(dailyStats) == 0 {
		return widget.NewLabel("No data available")
	}
	
	// Find max tokens for scaling
	maxTokens := int64(1)
	for _, stat := range dailyStats {
		if stat.TotalTokens > maxTokens {
			maxTokens = stat.TotalTokens
		}
	}
	
	// Chart dimensions
	chartHeight := float32(200)
	barWidth := float32(40)
	barSpacing := float32(10)
	
	// Create bars
	bars := container.NewWithoutLayout()
	
	for i, stat := range dailyStats {
		// Calculate bar height (proportional to tokens)
		barHeight := float32(stat.TotalTokens) / float32(maxTokens) * chartHeight
		if barHeight < 1 {
			barHeight = 1
		}
		
		// Create bar
		bar := canvas.NewRectangle(color.RGBA{R: 100, G: 150, B: 255, A: 255})
		bar.Resize(fyne.NewSize(barWidth, barHeight))
		bar.Move(fyne.NewPos(float32(i)*(barWidth+barSpacing), chartHeight-barHeight))
		bars.Add(bar)
		
		// Add date label
		dateLabel := widget.NewLabel(stat.Date.Format("01/02"))
		dateLabel.Resize(fyne.NewSize(barWidth+barSpacing, 20))
		dateLabel.Move(fyne.NewPos(float32(i)*(barWidth+barSpacing), chartHeight+5))
		bars.Add(dateLabel)
		
		// Add token count label
		tokenLabel := widget.NewLabel(formatNumber(stat.TotalTokens))
		tokenLabel.Resize(fyne.NewSize(barWidth+barSpacing, 20))
		tokenLabel.Move(fyne.NewPos(float32(i)*(barWidth+barSpacing), chartHeight-barHeight-20))
		bars.Add(tokenLabel)
	}
	
	// Calculate total width
	totalWidth := float32(len(dailyStats))*(barWidth+barSpacing) + barSpacing
	bars.Resize(fyne.NewSize(totalWidth, chartHeight+30))
	
	return container.NewHScroll(bars)
}

// getSortedProviders returns provider names sorted by token usage
func (usv *UsageStatsView) getSortedProviders() []string {
	if usv.currentStats == nil {
		return nil
	}
	
	providers := make([]string, 0, len(usv.currentStats.ProviderStats))
	for provider := range usv.currentStats.ProviderStats {
		providers = append(providers, provider)
	}
	
	sort.Slice(providers, func(i, j int) bool {
		return usv.currentStats.ProviderStats[providers[i]].TotalTokens >
			usv.currentStats.ProviderStats[providers[j]].TotalTokens
	})
	
	return providers
}

// getSortedModels returns model keys sorted by token usage
func (usv *UsageStatsView) getSortedModels() []string {
	if usv.currentStats == nil {
		return nil
	}
	
	models := make([]string, 0, len(usv.currentStats.ModelStats))
	for key := range usv.currentStats.ModelStats {
		models = append(models, key)
	}
	
	sort.Slice(models, func(i, j int) bool {
		return usv.currentStats.ModelStats[models[i]].TotalTokens >
			usv.currentStats.ModelStats[models[j]].TotalTokens
	})
	
	return models
}

// formatNumber formats a number with thousand separators
func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	
	// Convert to string and add commas
	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}
