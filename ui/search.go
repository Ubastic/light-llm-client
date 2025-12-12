package ui

import (
	"light-llm-client/db"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SearchView represents the search interface
type SearchView struct {
	app              *App
	searchEntry      *widget.Entry
	searchButton     *widget.Button
	resultsContainer *fyne.Container
	resultsList      *widget.List
	searchResults    []*db.SearchResult
	statusLabel      *widget.Label
	
	// Filter widgets
	providerSelect   *widget.Select
	categorySelect   *widget.Select
	dateRangeSelect  *widget.Select
	showFilters      bool
	filtersContainer *fyne.Container
}

// NewSearchView creates a new search view
func NewSearchView(app *App) *SearchView {
	sv := &SearchView{
		app:           app,
		searchResults: []*db.SearchResult{},
	}
	return sv
}

// Build builds the search view UI
func (sv *SearchView) Build() fyne.CanvasObject {
	// Search input
	sv.searchEntry = widget.NewEntry()
	sv.searchEntry.SetPlaceHolder("搜索对话内容...")
	sv.searchEntry.OnSubmitted = func(query string) {
		sv.performSearch()
	}

	sv.searchButton = widget.NewButton("搜索", func() {
		sv.performSearch()
	})
	sv.searchButton.Importance = widget.HighImportance
	
	// Filter toggle button
	filterButton := widget.NewButton("筛选", func() {
		sv.showFilters = !sv.showFilters
		if sv.showFilters {
			sv.filtersContainer.Show()
		} else {
			sv.filtersContainer.Hide()
		}
	})
	filterButton.Importance = widget.LowImportance

	// Search bar
	searchBar := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(filterButton, sv.searchButton),
		sv.searchEntry,
	)
	
	// Filter widgets
	providerOptions := []string{"全部提供商"}
	for name := range sv.app.providers {
		providerOptions = append(providerOptions, name)
	}
	sv.providerSelect = widget.NewSelect(providerOptions, func(value string) {
		sv.app.logger.Info("Provider filter changed: %s", value)
	})
	sv.providerSelect.SetSelected("全部提供商")
	
	// Get categories from database
	categoryOptions := []string{"全部分类"}
	categories, err := sv.app.db.GetCategories()
	if err == nil && len(categories) > 0 {
		categoryOptions = append(categoryOptions, categories...)
	} else if err != nil {
		sv.app.logger.Warn("Failed to load categories: %v", err)
	}
	sv.categorySelect = widget.NewSelect(categoryOptions, func(value string) {
		sv.app.logger.Info("Category filter changed: %s", value)
	})
	sv.categorySelect.SetSelected("全部分类")
	
	sv.dateRangeSelect = widget.NewSelect([]string{
		"全部时间",
		"今天",
		"最近7天",
		"最近30天",
		"最近90天",
	}, func(value string) {
		sv.app.logger.Info("Date range filter changed: %s", value)
	})
	sv.dateRangeSelect.SetSelected("全部时间")
	
	// Filters container
	sv.filtersContainer = container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("提供商", sv.providerSelect),
			widget.NewFormItem("分类", sv.categorySelect),
			widget.NewFormItem("时间范围", sv.dateRangeSelect),
		),
		widget.NewSeparator(),
	)
	// Don't hide here - will be hidden after adding to UI tree

	// Status label
	sv.statusLabel = widget.NewLabel("输入关键词开始搜索")
	sv.statusLabel.Alignment = fyne.TextAlignCenter

	// Results list
	sv.resultsList = widget.NewList(
		func() int {
			return len(sv.searchResults)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("Title"),
				widget.NewLabel("Snippet"),
				widget.NewSeparator(),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(sv.searchResults) {
				return
			}
			result := sv.searchResults[id]
			box := obj.(*fyne.Container)
			
			// Get conversation title
			conv, err := sv.app.db.GetConversation(result.ConversationID)
			convTitle := "Unknown"
			if err == nil && conv != nil {
				convTitle = conv.Title
			}
			
			// Title with conversation info
			titleLabel := box.Objects[0].(*widget.Label)
			titleLabel.SetText(convTitle)
			titleLabel.TextStyle = fyne.TextStyle{Bold: true}
			
			// Snippet with highlighting
			snippetLabel := box.Objects[1].(*widget.Label)
			snippetLabel.SetText(sv.formatSnippet(result.Snippet))
			snippetLabel.Wrapping = fyne.TextWrapWord
		},
	)

	sv.resultsList.OnSelected = func(id widget.ListItemID) {
		if id >= len(sv.searchResults) {
			return
		}
		result := sv.searchResults[id]
		sv.app.openChatTab(result.ConversationID)
		sv.resultsList.UnselectAll()
	}

	// Results container
	sv.resultsContainer = container.NewVBox(
		sv.statusLabel,
		sv.resultsList,
	)

	resultsScroll := container.NewScroll(sv.resultsContainer)
	resultsScroll.SetMinSize(fyne.NewSize(600, 400))
	
	// Top section with search bar and filters
	topSection := container.NewVBox(
		searchBar,
		sv.filtersContainer,
	)

	// Main layout
	mainLayout := container.NewBorder(
		topSection,
		nil,
		nil,
		nil,
		resultsScroll,
	)
	
	// Hide filters by default after UI is built
	sv.filtersContainer.Hide()
	
	return mainLayout
}

// performSearch executes the search query with filters
func (sv *SearchView) performSearch() {
	query := strings.TrimSpace(sv.searchEntry.Text)
	if query == "" {
		sv.statusLabel.SetText("请输入搜索关键词")
		sv.searchResults = []*db.SearchResult{}
		sv.resultsList.Refresh()
		return
	}

	sv.statusLabel.SetText("搜索中...")
	
	// Get filter values
	provider := sv.providerSelect.Selected
	category := sv.categorySelect.Selected
	dateRange := sv.dateRangeSelect.Selected
	
	// Convert date range to days
	daysAgo := 0
	switch dateRange {
	case "今天":
		daysAgo = 1
	case "最近7天":
		daysAgo = 7
	case "最近30天":
		daysAgo = 30
	case "最近90天":
		daysAgo = 90
	}
	
	sv.app.logger.Info("Searching for: %s (provider: %s, category: %s, days: %d)", query, provider, category, daysAgo)

	// Perform search with filters
	results, err := sv.app.db.SearchMessagesWithFilters(query, provider, category, daysAgo, 50)
	if err != nil {
		sv.app.logger.Error("Search failed: %v", err)
		sv.statusLabel.SetText("搜索失败: " + err.Error())
		sv.searchResults = []*db.SearchResult{}
		sv.resultsList.Refresh()
		return
	}

	sv.searchResults = results
	sv.resultsList.Refresh()

	if len(results) == 0 {
		sv.statusLabel.SetText("未找到匹配结果")
	} else {
		filterInfo := ""
		if provider != "全部提供商" || category != "全部分类" || daysAgo > 0 {
			filterInfo = " (已筛选)"
		}
		sv.statusLabel.SetText("找到 " + formatInt(len(results)) + " 条结果" + filterInfo)
	}

	sv.app.logger.Info("Search completed: %d results", len(results))
}

// formatSnippet formats the search result snippet with context
func (sv *SearchView) formatSnippet(snippet string) string {
	// Truncate long content
	maxLength := 200
	if len(snippet) > maxLength {
		// Try to find a good breaking point
		truncated := snippet[:maxLength]
		lastSpace := strings.LastIndex(truncated, " ")
		if lastSpace > maxLength/2 {
			truncated = truncated[:lastSpace]
		}
		snippet = truncated + "..."
	}

	// Add relevance indicator based on snippet length (simple heuristic)
	relevance := ""
	if len(snippet) > 100 {
		relevance = "🔥 "
	} else if len(snippet) > 50 {
		relevance = "⭐ "
	}

	return relevance + snippet
}

// formatInt converts an integer to string
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	
	result := ""
	for n > 0 {
		result = string(rune('0'+(n%10))) + result
		n /= 10
	}
	return result
}

// Clear clears the search results
func (sv *SearchView) Clear() {
	sv.searchEntry.SetText("")
	sv.searchResults = []*db.SearchResult{}
	sv.resultsList.Refresh()
	sv.statusLabel.SetText("输入关键词开始搜索")
}
