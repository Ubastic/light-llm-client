package ui

import (
	"fmt"
	"image/color"
	"light-llm-client/db"
	"light-llm-client/llm"
	"light-llm-client/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// App represents the main application
type App struct {
	fyneApp    fyne.App
	window     fyne.Window
	config     *utils.Config
	configPath string
	db         *db.DB
	logger     *utils.Logger
	providers  map[string]llm.Provider

	// UI components
	sidebar               *ConversationSidebar
	conversations         []*db.Conversation
	selectedConversationID int64
	settingsView          *SettingsView
	searchView            *SearchView
	tabs                  *CustomTabs
	
	// Multi-tab support
	chatViews             map[int64]*ChatView // conversationID -> ChatView
	tabItems              map[int64]*CustomTab // conversationID -> CustomTab
	searchTabItem         *CustomTab // Search tab
	forkTabItem           *CustomTab // Fork conversation tab
	
	// Message cache for preloading
	messageCache          map[int64][]*db.Message // conversationID -> messages
	uiCache               map[int64][]fyne.CanvasObject // conversationID -> UI objects
	cacheMaxSize          int // Maximum number of conversations to cache
	cacheAccessOrder      []int64 // LRU tracking for cache eviction
}

// NewApp creates a new application instance
func NewApp(config *utils.Config, configPath string, database *db.DB, logger *utils.Logger) *App {
	fyneApp := app.NewWithID("light-llm-client")
	window := fyneApp.NewWindow("Light LLM Client")

	// Set window size from config
	window.Resize(fyne.NewSize(
		float32(config.UI.WindowWidth),
		float32(config.UI.WindowHeight),
	))

	application := &App{
		fyneApp:    fyneApp,
		window:     window,
		config:     config,
		configPath: configPath,
		db:         database,
		logger:     logger,
		providers:  make(map[string]llm.Provider),
		chatViews:  make(map[int64]*ChatView),
		tabItems:   make(map[int64]*CustomTab),
		messageCache: make(map[int64][]*db.Message),
		uiCache:      make(map[int64][]fyne.CanvasObject),
		cacheMaxSize: 10, // Limit cache to 10 conversations
		cacheAccessOrder: make([]int64, 0, 10),
	}

	// Set up window resize callback to save window size
	window.SetOnClosed(func() {
		// Save window size when closing
		size := window.Canvas().Size()
		application.config.UI.WindowWidth = int(size.Width)
		application.config.UI.WindowHeight = int(size.Height)
		if err := utils.SaveConfig(application.configPath, application.config); err != nil {
			application.logger.Error("Failed to save window size: %v", err)
		} else {
			application.logger.Info("Window size saved: %dx%d", application.config.UI.WindowWidth, application.config.UI.WindowHeight)
		}
	})

	// Apply theme from config
	application.applyThemeFromConfig()

	// Initialize LLM providers
	application.initProviders()

	// Build UI
	application.buildUI()

	// Setup system tray
	application.SetupSystemTray()
	
	// Enable minimize to tray if configured
	if application.config.UI.MinimizeToTray {
		application.EnableMinimizeToTray()
		application.logger.Info("Minimize to tray enabled")
	}

	return application
}

// initProviders initializes LLM providers from config
func (a *App) initProviders() {
	// Iterate through all providers in config
	for name, providerConfig := range a.config.LLMProviders {
		if !providerConfig.Enabled {
			a.logger.Info("Provider %s is disabled in config", name)
			continue
		}

		// Use display name if available, otherwise use config key
		displayName := providerConfig.DisplayName
		if displayName == "" {
			displayName = name
		}

		// Initialize based on provider type
		if name == "ollama" {
			// Ollama provider (no API key required)
			provider, err := llm.NewOllamaProvider(llm.Config{
				ProviderName: displayName,
				BaseURL:      providerConfig.BaseURL,
				Model:        providerConfig.DefaultModel,
				Models:       providerConfig.Models,
			})
			if err != nil {
				a.logger.Error("Failed to initialize %s provider: %v", name, err)
			} else {
				a.providers[name] = provider
				a.logger.Info("%s provider initialized successfully", name)
			}
		} else if name == "claude" || name == "anthropic" {
			// Claude/Anthropic provider
			provider, err := llm.NewClaudeProvider(llm.Config{
				ProviderName: displayName,
				APIKey:       providerConfig.APIKey,
				BaseURL:      providerConfig.BaseURL,
				Model:        providerConfig.DefaultModel,
				Models:       providerConfig.Models,
				MaxTokens:    providerConfig.MaxTokens,
				Temperature:  providerConfig.Temperature,
			})
			if err != nil {
				a.logger.Error("Failed to initialize %s provider: %v", name, err)
			} else {
				a.providers[name] = provider
				a.logger.Info("%s provider initialized successfully", name)
			}
		} else if name == "gemini" {
			// Google Gemini provider
			provider, err := llm.NewGeminiProvider(llm.Config{
				ProviderName: displayName,
				APIKey:       providerConfig.APIKey,
				BaseURL:      providerConfig.BaseURL,
				Model:        providerConfig.DefaultModel,
				Models:       providerConfig.Models,
				MaxTokens:    providerConfig.MaxTokens,
				Temperature:  providerConfig.Temperature,
			})
			if err != nil {
				a.logger.Error("Failed to initialize %s provider: %v", name, err)
			} else {
				a.providers[name] = provider
				a.logger.Info("%s provider initialized successfully", name)
			}
		} else {
			// All other providers are treated as OpenAI-compatible
			// No validation - let the provider itself validate
			provider, err := llm.NewOpenAIProvider(llm.Config{
				ProviderName: displayName,
				APIKey:       providerConfig.APIKey,
				BaseURL:      providerConfig.BaseURL,
				Model:        providerConfig.DefaultModel,
				Models:       providerConfig.Models,
				MaxTokens:    providerConfig.MaxTokens,
				Temperature:  providerConfig.Temperature,
			})
			if err != nil {
				a.logger.Error("Failed to initialize %s provider: %v", name, err)
			} else {
				a.providers[name] = provider
				a.logger.Info("%s provider initialized successfully", name)
			}
		}
	}

	if len(a.providers) == 0 {
		a.logger.Warn("No providers initialized - check your configuration")
	}
}

// buildUI builds the main UI
func (a *App) buildUI() {
	// Create sidebar for conversation history
	a.sidebar = a.createSidebar()

	// Create settings view
	a.settingsView = NewSettingsView(a)

	// Create search view
	a.searchView = NewSearchView(a)

	// Create settings button
	settingsButton := widget.NewButton("⚙️ 设置", func() {
		a.showSettings()
	})

	// Create search button
	searchButton := widget.NewButton("🔍 搜索", func() {
		a.showSearch()
	})

	// Create custom tabs container for chat views
	a.tabs = NewCustomTabs()
	
	// Add tab change listener to update sidebar highlighting
	a.tabs.OnChanged = func(tab *CustomTab) {
		// Find which conversation this tab belongs to
		activeConvID := a.getActiveConversationID()
		if activeConvID != 0 {
			a.sidebar.updateHighlight(activeConvID)
		}
	}
	
	// Create import/export buttons
	importButton := widget.NewButton("📥 导入", func() {
		a.showImportDialog()
	})
	importButton.Importance = widget.LowImportance
	
	exportAllButton := widget.NewButton("📤 导出全部", func() {
		a.exportAllConversations()
	})
	exportAllButton.Importance = widget.LowImportance
	
	// Create fork button
	forkButton := widget.NewButton("🔀 分叉对话", func() {
		activeConvID := a.getActiveConversationID()
		if activeConvID == 0 {
			a.showError("请先选择一个对话")
			return
		}
		ShowForkDialog(a, activeConvID)
	})
	forkButton.Importance = widget.MediumImportance
	
	// Create sidebar container
	sidebarContainer := container.NewBorder(
		nil,
		container.NewVBox(
			container.NewGridWithColumns(2, importButton, exportAllButton),
			forkButton,
			searchButton,
			settingsButton,
			a.createNewChatButton(),
		),
		nil,
		nil,
		a.sidebar,
	)
	
	// Use HSplit to give sidebar proper width (25% of window)
	split := container.NewHSplit(sidebarContainer, a.tabs)
	split.SetOffset(0.25)
	
	mainContent := split

	a.window.SetContent(mainContent)
	
	// Setup keyboard shortcuts
	a.setupKeyboardShortcuts()
}

// setupKeyboardShortcuts sets up global keyboard shortcuts
func (a *App) setupKeyboardShortcuts() {
	// Ctrl+N: New conversation
	a.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyN,
		Modifier: desktop.ControlModifier,
	}, func(shortcut fyne.Shortcut) {
		a.logger.Info("Keyboard shortcut: Ctrl+N - New conversation")
		a.createNewConversation()
	})
	
	// Ctrl+F: Search
	a.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyF,
		Modifier: desktop.ControlModifier,
	}, func(shortcut fyne.Shortcut) {
		a.logger.Info("Keyboard shortcut: Ctrl+F - Search")
		a.showSearch()
	})
	
	// Ctrl+Comma: Settings
	a.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyComma,
		Modifier: desktop.ControlModifier,
	}, func(shortcut fyne.Shortcut) {
		a.logger.Info("Keyboard shortcut: Ctrl+, - Settings")
		a.showSettings()
	})
	
	// Ctrl+W: Close current tab
	a.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyW,
		Modifier: desktop.ControlModifier,
	}, func(shortcut fyne.Shortcut) {
		a.logger.Info("Keyboard shortcut: Ctrl+W - Close tab")
		a.closeCurrentTab()
	})
	
	// Ctrl+Tab: Next tab
	a.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyTab,
		Modifier: desktop.ControlModifier,
	}, func(shortcut fyne.Shortcut) {
		a.logger.Info("Keyboard shortcut: Ctrl+Tab - Next tab")
		a.nextTab()
	})
	
	// Ctrl+Shift+Tab: Previous tab
	a.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyTab,
		Modifier: desktop.ControlModifier | desktop.ShiftModifier,
	}, func(shortcut fyne.Shortcut) {
		a.logger.Info("Keyboard shortcut: Ctrl+Shift+Tab - Previous tab")
		a.previousTab()
	})
	
	// Ctrl+Shift+F: Fork conversation
	a.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyF,
		Modifier: desktop.ControlModifier | desktop.ShiftModifier,
	}, func(shortcut fyne.Shortcut) {
		a.logger.Info("Keyboard shortcut: Ctrl+Shift+F - Fork conversation")
		activeConvID := a.getActiveConversationID()
		if activeConvID == 0 {
			a.showError("请先选择一个对话")
			return
		}
		ShowForkDialog(a, activeConvID)
	})
	
	a.logger.Info("Keyboard shortcuts registered")
}

// nextTab switches to the next tab
func (a *App) nextTab() {
	if a.tabs.TabCount() == 0 {
		return
	}
	
	// Find current tab index
	currentTab := a.tabs.GetActiveTab()
	if currentTab == nil {
		return
	}
	
	currentIndex := -1
	for i, tab := range a.tabs.tabs {
		if tab == currentTab {
			currentIndex = i
			break
		}
	}
	
	if currentIndex >= 0 {
		nextIndex := (currentIndex + 1) % len(a.tabs.tabs)
		a.tabs.SelectTab(a.tabs.tabs[nextIndex])
	}
}

// previousTab switches to the previous tab
func (a *App) previousTab() {
	if a.tabs.TabCount() == 0 {
		return
	}
	
	// Find current tab index
	currentTab := a.tabs.GetActiveTab()
	if currentTab == nil {
		return
	}
	
	currentIndex := -1
	for i, tab := range a.tabs.tabs {
		if tab == currentTab {
			currentIndex = i
			break
		}
	}
	
	if currentIndex >= 0 {
		previousIndex := currentIndex - 1
		if previousIndex < 0 {
			previousIndex = len(a.tabs.tabs) - 1
		}
		a.tabs.SelectTab(a.tabs.tabs[previousIndex])
	}
}

// createSidebar creates the conversation history sidebar
func (a *App) createSidebar() *ConversationSidebar {
	return NewConversationSidebar(a)
}

// createNewChatButton creates the new chat button
func (a *App) createNewChatButton() *widget.Button {
	return widget.NewButton("新建对话", func() {
		a.createNewConversation()
	})
}

// createNewConversation creates a new conversation
func (a *App) createNewConversation() {
	conv, err := a.db.CreateConversation("New Chat", "")
	if err != nil {
		a.logger.Error("Failed to create conversation: %v", err)
		a.showError("Failed to create conversation: " + err.Error())
		return
	}

	a.logger.Info("Created new conversation: %d", conv.ID)
	a.RefreshSidebar()
	a.openChatTab(conv.ID)
}

// openChatTab opens or focuses a chat tab for the given conversation
func (a *App) openChatTab(conversationID int64) {
	// Check if tab already exists
	if tabItem, exists := a.tabItems[conversationID]; exists {
		// Focus existing tab
		a.tabs.SelectTab(tabItem)
		a.logger.Info("Focused existing chat tab: %d", conversationID)
		// Update sidebar highlighting
		a.sidebar.updateHighlight(conversationID)
		return
	}
	
	// Get conversation info
	conv, err := a.db.GetConversation(conversationID)
	if err != nil {
		a.logger.Error("Failed to get conversation: %v", err)
		return
	}
	
	// Create new chat view
	a.logger.Info("Opening new chat tab: %d", conversationID)
	chatView := NewChatView(a)
	
	// Create tab with close button
	tabItem := a.tabs.Append(conv.Title, chatView.Build(), func() {
		a.closeChatTab(conversationID)
	})
	
	// Store references
	a.chatViews[conversationID] = chatView
	a.tabItems[conversationID] = tabItem
	
	// Update sidebar highlighting
	a.sidebar.updateHighlight(conversationID)
	
	// Load conversation after UI is built
	chatView.SetConversation(conversationID)
}

// renameConversationByID renames a conversation by ID
func (a *App) renameConversationByID(conversationID int64) {
	// Get the conversation from database
	conv, err := a.db.GetConversation(conversationID)
	if err != nil {
		a.showError("对话不存在")
		return
	}
	
	a.renameConversationWithData(conv)
}

// renameConversation renames the selected conversation
func (a *App) renameConversation() {
	if a.selectedConversationID == 0 {
		a.showError("请先选择一个对话")
		return
	}

	// Find the conversation in the list
	var conv *db.Conversation
	for _, c := range a.conversations {
		if c.ID == a.selectedConversationID {
			conv = c
			break
		}
	}

	if conv == nil {
		a.showError("对话不存在")
		return
	}
	
	a.renameConversationWithData(conv)
}

// renameConversationWithData renames a conversation with the given data
func (a *App) renameConversationWithData(conv *db.Conversation) {
	
	// Create rename dialog
	nameEntry := widget.NewEntry()
	nameEntry.SetText(conv.Title)
	
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("重命名对话"),
			nameEntry,
			container.NewHBox(
				widget.NewButton("取消", func() {
					dialog.Hide()
				}),
				widget.NewButton("确定", func() {
					newTitle := nameEntry.Text
					if newTitle == "" {
						a.showError("对话标题不能为空")
						return
					}
					
					// Update in database
					if err := a.db.UpdateConversation(conv.ID, newTitle, conv.Category); err != nil {
						a.logger.Error("Failed to rename conversation: %v", err)
						a.showError("重命名失败: " + err.Error())
						return
					}
					
					a.logger.Info("Conversation renamed to: %s", newTitle)
					a.RefreshSidebar()
					
					// Update tab title if open
					if tabItem, exists := a.tabItems[conv.ID]; exists {
						tabItem.Title = newTitle
						a.tabs.refreshTabBar()
					}
					
					dialog.Hide()
				}),
			),
		),
		a.window.Canvas(),
	)
	dialog.Show()
}

// deleteConversationByID deletes a conversation by ID
func (a *App) deleteConversationByID(conversationID int64) {
	// Get the conversation from database
	conv, err := a.db.GetConversation(conversationID)
	if err != nil {
		a.showError("对话不存在")
		return
	}
	
	a.deleteConversationWithData(conv)
}

// deleteConversation deletes the selected conversation
func (a *App) deleteConversation() {
	if a.selectedConversationID == 0 {
		a.showError("请先选择一个对话")
		return
	}

	// Find the conversation in the list
	var conv *db.Conversation
	for _, c := range a.conversations {
		if c.ID == a.selectedConversationID {
			conv = c
			break
		}
	}

	if conv == nil {
		a.showError("对话不存在")
		return
	}
	
	a.deleteConversationWithData(conv)
}

// deleteConversationWithData deletes a conversation with the given data
func (a *App) deleteConversationWithData(conv *db.Conversation) {
	
	// Create confirmation dialog
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("确认删除"),
			widget.NewLabel("确定要删除对话 \"" + conv.Title + "\" 吗？"),
			widget.NewLabel("此操作不可撤销！"),
			container.NewHBox(
				widget.NewButton("取消", func() {
					dialog.Hide()
				}),
				widget.NewButton("删除", func() {
					// Delete from database
					if err := a.db.DeleteConversation(conv.ID); err != nil {
						a.logger.Error("Failed to delete conversation: %v", err)
						a.showError("删除失败: " + err.Error())
						return
					}
					
					a.logger.Info("Conversation deleted: %d", conv.ID)
					
					// Close chat tab if open
					a.closeChatTab(conv.ID)
					
					a.RefreshSidebar()
					dialog.Hide()
				}),
			),
		),
		a.window.Canvas(),
	)
	dialog.Show()
}

// closeChatTab closes a chat tab
func (a *App) closeChatTab(conversationID int64) {
	if tabItem, exists := a.tabItems[conversationID]; exists {
		a.tabs.Remove(tabItem)
		delete(a.chatViews, conversationID)
		delete(a.tabItems, conversationID)
		
		// Clear cache for this conversation to free memory
		delete(a.messageCache, conversationID)
		delete(a.uiCache, conversationID)
		
		// Remove from access order
		for i, id := range a.cacheAccessOrder {
			if id == conversationID {
				a.cacheAccessOrder = append(a.cacheAccessOrder[:i], a.cacheAccessOrder[i+1:]...)
				break
			}
		}
		
		a.logger.Info("Closed chat tab and cleared cache: %d", conversationID)
	}
}

// showSettings shows the settings window
func (a *App) showSettings() {
	settingsWin := a.fyneApp.NewWindow("设置")
	a.settingsView.SetWindow(settingsWin)
	settingsWin.SetContent(a.settingsView.Build())
	settingsWin.Resize(fyne.NewSize(800, 600))
	settingsWin.Show()
}

// showSearch shows the search tab
func (a *App) showSearch() {
	// Check if search tab already exists
	if a.searchTabItem != nil {
		// Focus existing search tab
		a.tabs.SelectTab(a.searchTabItem)
		a.logger.Info("Focused existing search tab")
		return
	}
	
	// Create tab with close button
	a.searchTabItem = a.tabs.Append("🔍 搜索", a.searchView.Build(), func() {
		a.closeSearchTab()
	})
	
	a.logger.Info("Opened search tab")
}

// closeSearchTab closes the search tab
func (a *App) closeSearchTab() {
	if a.searchTabItem != nil {
		a.tabs.Remove(a.searchTabItem)
		a.searchTabItem = nil
		a.logger.Info("Closed search tab")
	}
}

// closeForkTab closes the fork conversation tab
func (a *App) closeForkTab() {
	if a.forkTabItem != nil {
		a.tabs.Remove(a.forkTabItem)
		a.forkTabItem = nil
		a.logger.Info("Closed fork conversation tab")
	}
}

// closeCurrentTab closes the currently active tab
func (a *App) closeCurrentTab() {
	if a.tabs == nil || a.tabs.GetActiveTab() == nil {
		return
	}
	
	selectedTab := a.tabs.GetActiveTab()
	
	// Check if it's the search tab
	if a.searchTabItem != nil && selectedTab == a.searchTabItem {
		a.closeSearchTab()
		return
	}
	
	// Check if it's the fork tab
	if a.forkTabItem != nil && selectedTab == a.forkTabItem {
		a.closeForkTab()
		return
	}
	
	// Check if it's a conversation tab
	for convID, tabItem := range a.tabItems {
		if tabItem == selectedTab {
			a.closeChatTab(convID)
			return
		}
	}
	
	a.logger.Warn("Could not identify tab to close")
}

// Run starts the application
func (a *App) Run() {
	a.window.ShowAndRun()
}

// RefreshSidebar refreshes the conversation list in the sidebar
func (a *App) RefreshSidebar() {
	conversations, err := a.db.ListConversations(100, 0)
	if err != nil {
		a.logger.Error("Failed to load conversations: %v", err)
		return
	}
	a.conversations = conversations
	a.sidebar.Refresh()
	// Note: preloading is now handled by sidebar.preloadVisibleConversations()
}

// preloadConversations preloads messages for the first N conversations
func (a *App) preloadConversations(count int) {
	utils.SafeGo(a.logger, "preloadConversations", func() {
		for i := 0; i < count && i < len(a.conversations); i++ {
			convID := a.conversations[i].ID
			
			// Skip if already cached or tab is open
			if _, cached := a.messageCache[convID]; cached {
				continue
			}
			if _, tabOpen := a.chatViews[convID]; tabOpen {
				continue
			}
			
			// Load messages
			messages, err := a.db.ListMessages(convID)
			if err != nil {
				a.logger.Warn("Failed to preload conversation %d: %v", convID, err)
				continue
			}
			
			// Cache the messages
			a.messageCache[convID] = messages
			a.logger.Info("Preloaded conversation %d (%d messages)", convID, len(messages))
		}
	})
}

// preloadConversationByID preloads a specific conversation
func (a *App) preloadConversationByID(conversationID int64) {
	// Skip if already cached or tab is open
	if _, cached := a.messageCache[conversationID]; cached {
		return
	}
	if _, tabOpen := a.chatViews[conversationID]; tabOpen {
		return
	}
	
	utils.SafeGo(a.logger, "preloadConversationByID", func() {
		messages, err := a.db.ListMessages(conversationID)
		if err != nil {
			a.logger.Warn("Failed to preload conversation %d: %v", conversationID, err)
			return
		}
		
		a.messageCache[conversationID] = messages
		a.logger.Info("Preloaded conversation %d (%d messages)", conversationID, len(messages))
	})
}

// showError shows an error dialog
func (a *App) showError(message string) {
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("❌ Error"),
			widget.NewLabel(message),
			widget.NewButton("OK", func() {
				dialog.Hide()
			}),
		),
		a.window.Canvas(),
	)
	dialog.Show()
}

// showSuccess shows a success dialog
func (a *App) showSuccess(message string) {
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("✅ Success"),
			widget.NewLabel(message),
			widget.NewButton("OK", func() {
				dialog.Hide()
			}),
		),
		a.window.Canvas(),
	)
	dialog.Show()
}

// showInfo shows an info dialog
func (a *App) showInfo(message string) {
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("ℹ️ 信息"),
			widget.NewLabel(message),
			widget.NewButton("确定", func() {
				dialog.Hide()
			}),
		),
		a.window.Canvas(),
	)
	dialog.Show()
}

// lightTheme is a simple light theme
type lightTheme struct{}

func (t *lightTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

func (t *lightTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *lightTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *lightTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// darkTheme is a simple dark theme
type darkTheme struct{}

func (t *darkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (t *darkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *darkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *darkTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// applyThemeFromConfig applies the theme from config
func (a *App) applyThemeFromConfig() {
	isDark := a.config.UI.Theme == "dark"
	fontSize := a.config.UI.FontSize
	if fontSize < 10 {
		fontSize = 14 // Default font size
	}
	
	customTheme := newCustomTheme(fontSize, isDark)
	a.fyneApp.Settings().SetTheme(customTheme)
	
	if isDark {
		a.logger.Info("Applied dark theme with font size %d", fontSize)
	} else {
		a.logger.Info("Applied light theme with font size %d", fontSize)
	}
}

// getActiveConversationID returns the ID of the currently active conversation tab
func (a *App) getActiveConversationID() int64 {
	if a.tabs == nil || a.tabs.GetActiveTab() == nil {
		return 0
	}
	
	// Find which conversation tab is currently selected
	selectedTab := a.tabs.GetActiveTab()
	for convID, tabItem := range a.tabItems {
		if tabItem == selectedTab {
			return convID
		}
	}
	
	return 0
}

// exportConversation exports a conversation to a file
func (a *App) exportConversation(conversationID int64, format utils.ExportFormat) {
	// Get conversation for filename
	conv, err := a.db.GetConversation(conversationID)
	if err != nil {
		a.showError("Failed to get conversation: " + err.Error())
		return
	}

	// Get default export path
	exportDir, err := utils.GetDefaultExportPath()
	if err != nil {
		a.showError("Failed to get export directory: " + err.Error())
		return
	}

	// Generate filename
	filename := utils.GenerateExportFilename(conv.Title, format)
	filepath := exportDir + "/" + filename

	// Export based on format
	var exportErr error
	if format == utils.FormatJSON {
		exportErr = utils.ExportConversationToJSON(a.db, conversationID, filepath)
	} else if format == utils.FormatMarkdown {
		exportErr = utils.ExportConversationToMarkdown(a.db, conversationID, filepath)
	}

	if exportErr != nil {
		a.showError("Export failed: " + exportErr.Error())
		return
	}

	a.logger.Info("Exported conversation %d to %s", conversationID, filepath)
	a.showInfo("导出成功!\n文件保存在: " + filepath)
}

// exportAllConversations exports all conversations to a JSON file
func (a *App) exportAllConversations() {
	// Get default export path
	exportDir, err := utils.GetDefaultExportPath()
	if err != nil {
		a.showError("Failed to get export directory: " + err.Error())
		return
	}

	// Generate filename
	filename := utils.GenerateExportFilename("all_conversations", utils.FormatJSON)
	filepath := exportDir + "/" + filename

	// Export
	if err := utils.ExportAllConversations(a.db, filepath); err != nil {
		a.showError("Export failed: " + err.Error())
		return
	}

	a.logger.Info("Exported all conversations to %s", filepath)
	a.showInfo("导出成功!\n文件保存在: " + filepath)
}

// importConversation imports a conversation from a file
func (a *App) importConversation(filepath string) {
	conv, err := utils.ImportConversation(a.db, filepath)
	if err != nil {
		a.showError("Import failed: " + err.Error())
		return
	}

	a.logger.Info("Imported conversation: %s (ID: %d)", conv.Title, conv.ID)
	a.RefreshSidebar()
	a.showInfo("导入成功!\n对话: " + conv.Title)
}

// importAllConversations imports multiple conversations from a file
func (a *App) importAllConversations(filepath string) {
	count, err := utils.ImportAllConversations(a.db, filepath)
	if err != nil {
		a.showError("Import failed: " + err.Error())
		return
	}

	a.logger.Info("Imported %d conversations", count)
	a.RefreshSidebar()
	a.showInfo(fmt.Sprintf("导入成功!\n共导入 %d 个对话", count))
}

// showImportDialog shows a dialog to import conversations
func (a *App) showImportDialog() {
	// Create file open dialog
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			a.showError("Failed to open file: " + err.Error())
			return
		}
		if reader == nil {
			return // User cancelled
		}
		defer reader.Close()
		
		filepath := reader.URI().Path()
		a.logger.Info("Importing from: %s", filepath)
		
		// Try to import as multiple conversations first
		count, err := utils.ImportAllConversations(a.db, filepath)
		if err == nil && count > 0 {
			a.logger.Info("Imported %d conversations", count)
			a.RefreshSidebar()
			a.showInfo(fmt.Sprintf("导入成功!\n共导入 %d 个对话", count))
			return
		}
		
		// If that fails, try importing as single conversation
		conv, err := utils.ImportConversation(a.db, filepath)
		if err != nil {
			a.showError("导入失败: " + err.Error())
			return
		}
		
		a.logger.Info("Imported conversation: %s (ID: %d)", conv.Title, conv.ID)
		a.RefreshSidebar()
		a.showInfo("导入成功!\n对话: " + conv.Title)
	}, a.window)
	
	fileDialog.Show()
}

// setCategoryForConversation shows a dialog to set category for a conversation
func (a *App) setCategoryForConversation(conversationID int64) {
	// Get the conversation from database
	conv, err := a.db.GetConversation(conversationID)
	if err != nil {
		a.showError("对话不存在")
		return
	}
	
	// Get all existing categories
	categories, err := a.db.GetCategories()
	if err != nil {
		a.logger.Error("Failed to get categories: %v", err)
		categories = []string{}
	}
	
	// Add "无分类" option
	categoryOptions := append([]string{"无分类"}, categories...)
	
	// Create category select widget
	categorySelect := widget.NewSelect(categoryOptions, nil)
	if conv.Category != "" {
		categorySelect.SetSelected(conv.Category)
	} else {
		categorySelect.SetSelected("无分类")
	}
	
	// Create new category entry
	newCategoryEntry := widget.NewEntry()
	newCategoryEntry.SetPlaceHolder("或输入新分类...")
	
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("设置对话分类"),
			widget.NewLabel("选择已有分类:"),
			categorySelect,
			widget.NewLabel("或创建新分类:"),
			newCategoryEntry,
			container.NewHBox(
				widget.NewButton("取消", func() {
					dialog.Hide()
				}),
				widget.NewButton("确定", func() {
					var selectedCategory string
					
					// Prefer new category if entered
					if newCategoryEntry.Text != "" {
						selectedCategory = newCategoryEntry.Text
					} else if categorySelect.Selected == "无分类" {
						selectedCategory = ""
					} else {
						selectedCategory = categorySelect.Selected
					}
					
					// Update in database
					if err := a.db.UpdateConversation(conv.ID, conv.Title, selectedCategory); err != nil {
						a.logger.Error("Failed to update conversation category: %v", err)
						a.showError("设置分类失败: " + err.Error())
						return
					}
					
					a.logger.Info("Conversation category updated to: %s", selectedCategory)
					a.RefreshSidebar()
					
					dialog.Hide()
				}),
			),
		),
		a.window.Canvas(),
	)
	dialog.Show()
}

// evictOldestCache removes the oldest cached conversation to free memory
func (a *App) evictOldestCache() {
	if len(a.cacheAccessOrder) == 0 {
		return
	}
	
	// Find the oldest conversation that is not currently open in a tab
	for i := 0; i < len(a.cacheAccessOrder); i++ {
		convID := a.cacheAccessOrder[i]
		
		// Skip if this conversation has an open tab
		if _, tabOpen := a.chatViews[convID]; tabOpen {
			continue
		}
		
		// Remove from cache
		delete(a.messageCache, convID)
		delete(a.uiCache, convID)
		
		// Remove from access order
		a.cacheAccessOrder = append(a.cacheAccessOrder[:i], a.cacheAccessOrder[i+1:]...)
		
		a.logger.Info("Evicted conversation %d from cache to free memory", convID)
		return
	}
	
	// If all cached conversations have open tabs, just remove the oldest one anyway
	if len(a.cacheAccessOrder) > 0 {
		convID := a.cacheAccessOrder[0]
		delete(a.messageCache, convID)
		delete(a.uiCache, convID)
		a.cacheAccessOrder = a.cacheAccessOrder[1:]
		a.logger.Info("Evicted conversation %d from cache (had open tab)", convID)
	}
}

// updateCacheAccess updates the LRU access order for a conversation
func (a *App) updateCacheAccess(conversationID int64) {
	// Remove from current position if exists
	for i, id := range a.cacheAccessOrder {
		if id == conversationID {
			a.cacheAccessOrder = append(a.cacheAccessOrder[:i], a.cacheAccessOrder[i+1:]...)
			break
		}
	}
	
	// Add to end (most recently used)
	a.cacheAccessOrder = append(a.cacheAccessOrder, conversationID)
	
	// Evict oldest if cache is too large
	cacheSize := len(a.messageCache)
	if cacheSize > a.cacheMaxSize {
		a.evictOldestCache()
	}
}

// clearUnusedCache clears cache for conversations that don't have open tabs
func (a *App) clearUnusedCache() {
	cleared := 0
	for convID := range a.messageCache {
		// Keep cache if tab is open
		if _, tabOpen := a.chatViews[convID]; tabOpen {
			continue
		}
		
		delete(a.messageCache, convID)
		delete(a.uiCache, convID)
		cleared++
	}
	
	// Rebuild access order to only include cached items
	newAccessOrder := make([]int64, 0, len(a.messageCache))
	for _, convID := range a.cacheAccessOrder {
		if _, exists := a.messageCache[convID]; exists {
			newAccessOrder = append(newAccessOrder, convID)
		}
	}
	a.cacheAccessOrder = newAccessOrder
	
	if cleared > 0 {
		a.logger.Info("Cleared %d unused conversations from cache", cleared)
	}
}

// Cleanup performs cleanup before exit
func (a *App) Cleanup() {
	// Clear all caches to free memory
	a.messageCache = nil
	a.uiCache = nil
	a.cacheAccessOrder = nil
	
	if a.db != nil {
		a.db.Close()
	}
	if a.logger != nil {
		a.logger.Close()
	}
}
