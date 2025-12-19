package ui

import (
	"context"
	"fmt"
	"light-llm-client/llm"
	"light-llm-client/utils"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SettingsView represents the settings interface
type SettingsView struct {
	app              *App
	settingsWindow   fyne.Window // Reference to the settings window
	providersList    *widget.List
	providerConfigs  map[string]*utils.ProviderConfig
	providerNames    []string // Cached list of provider names to maintain order
	selectedProvider string
	
	// Edit form widgets
	nameEntry        *widget.Entry
	displayNameEntry *widget.Entry
	apiKeyEntry      *widget.Entry
	baseURLEntry     *widget.Entry
	modelEntry       *widget.Entry
	modelsEntry      *widget.Entry
	enabledCheck     *widget.Check
	maxTokensEntry   *widget.Entry
	temperatureEntry *widget.Entry
	
	// UI settings widgets
	themeSelect      *widget.Select
	fontSizeSlider   *widget.Slider
	fontSizeLabel    *widget.Label
	
	editContainer    *fyne.Container
	saveButton       *widget.Button
	testButton       *widget.Button
	deleteButton     *widget.Button
	addButton        *widget.Button
}

// NewSettingsView creates a new settings view
func NewSettingsView(app *App) *SettingsView {
	sv := &SettingsView{
		app:             app,
		providerConfigs: make(map[string]*utils.ProviderConfig),
	}
	
	// Copy current provider configs
	for name, config := range app.config.LLMProviders {
		configCopy := config
		sv.providerConfigs[name] = &configCopy
	}
	
	return sv
}

// SetWindow sets the settings window reference
func (sv *SettingsView) SetWindow(window fyne.Window) {
	sv.settingsWindow = window
}

// Build builds the settings view UI
func (sv *SettingsView) Build() fyne.CanvasObject {
	// Create tabs for different settings sections
	tabs := container.NewAppTabs(
		container.NewTabItem("Providers", sv.buildProvidersTab()),
		container.NewTabItem("UI Settings", sv.buildUISettingsTab()),
		container.NewTabItem("Data", sv.buildDataSettingsTab()),
		container.NewTabItem("Usage Statistics", sv.buildUsageStatsTab()),
	)
	
	return tabs
}

// buildProvidersTab builds the providers configuration tab
func (sv *SettingsView) buildProvidersTab() fyne.CanvasObject {
	// Initialize provider names list
	sv.updateProviderNamesList()
	
	sv.providersList = widget.NewList(
		func() int {
			return len(sv.providerNames)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Provider"),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(sv.providerNames) {
				name := sv.providerNames[id]
				config := sv.providerConfigs[name]
				box := obj.(*fyne.Container)
				box.Objects[0].(*widget.Label).SetText(name)
				if config.Enabled {
					box.Objects[1].(*widget.Label).SetText("[Enabled]")
				} else {
					box.Objects[1].(*widget.Label).SetText("[Disabled]")
				}
			}
		},
	)
	
	sv.providersList.OnSelected = func(id widget.ListItemID) {
		if id < len(sv.providerNames) {
			sv.selectedProvider = sv.providerNames[id]
			sv.loadProviderConfig(sv.providerNames[id])
		}
	}
	
	// Edit form
	sv.buildEditForm()
	
	// Add new provider button
	sv.addButton = widget.NewButton("Add New Provider", func() {
		sv.showAddProviderDialog()
	})
	
	// Left panel with list and add button
	leftPanel := container.NewBorder(
		widget.NewLabel("LLM Providers"),
		sv.addButton,
		nil,
		nil,
		sv.providersList,
	)
	
	// Right panel with provider config
	rightPanel := container.NewVScroll(sv.editContainer)
	
	// Main layout
	split := container.NewHSplit(
		leftPanel,
		rightPanel,
	)
	split.SetOffset(0.3)
	
	return split
}

// buildUISettingsTab builds the UI settings tab
func (sv *SettingsView) buildUISettingsTab() fyne.CanvasObject {
	// Theme selector
	sv.themeSelect = widget.NewSelect([]string{"Light", "Dark"}, func(value string) {
		sv.applyTheme(value)
	})
	sv.themeSelect.SetSelected(sv.app.config.UI.Theme)
	
	// Font size slider (10-24 px)
	sv.fontSizeLabel = widget.NewLabel(fmt.Sprintf("Font Size: %d", sv.app.config.UI.FontSize))
	sv.fontSizeSlider = widget.NewSlider(10, 24)
	sv.fontSizeSlider.Step = 1
	sv.fontSizeSlider.Value = float64(sv.app.config.UI.FontSize)
	sv.fontSizeSlider.OnChanged = func(value float64) {
		fontSize := int(value)
		sv.fontSizeLabel.SetText(fmt.Sprintf("Font Size: %d", fontSize))
		sv.app.config.UI.FontSize = fontSize
		
		// Apply theme immediately with new font size
		sv.app.applyThemeFromConfig()
		
		// Save config
		if err := utils.SaveConfig(sv.app.configPath, sv.app.config); err != nil {
			sv.app.logger.Error("Failed to save font size: %v", err)
		} else {
			sv.app.logger.Info("Font size updated to %d", fontSize)
		}
	}
	
	fontSizeNote := widget.NewLabel("提示: 字体大小会立即应用到所有文本")
	fontSizeNote.Wrapping = fyne.TextWrapWord
	fontSizeNote.TextStyle = fyne.TextStyle{Italic: true}
	
	fontSizeContainer := container.NewVBox(
		sv.fontSizeLabel,
		sv.fontSizeSlider,
		fontSizeNote,
	)
	
	// Minimize to tray checkbox
	minimizeToTrayCheck := widget.NewCheck("最小化到系统托盘", func(checked bool) {
		sv.app.config.UI.MinimizeToTray = checked
		
		// Apply minimize to tray behavior
		if checked {
			sv.app.EnableMinimizeToTray()
			sv.app.logger.Info("Minimize to tray enabled")
		} else {
			sv.app.DisableMinimizeToTray()
			sv.app.logger.Info("Minimize to tray disabled")
		}
		
		// Save config
		if err := utils.SaveConfig(sv.app.configPath, sv.app.config); err != nil {
			sv.app.logger.Error("Failed to save minimize to tray setting: %v", err)
		}
	})
	minimizeToTrayCheck.Checked = sv.app.config.UI.MinimizeToTray
	
	// Memory monitor button
	memoryMonitorButton := widget.NewButton("📊 内存监控", func() {
		monitor := NewMemoryMonitor(sv.app)
		monitor.Show()
	})
	memoryMonitorButton.Importance = widget.HighImportance
	
	uiSettingsForm := widget.NewForm(
		widget.NewFormItem("Theme", sv.themeSelect),
		widget.NewFormItem("", fontSizeContainer),
		widget.NewFormItem("System Tray", minimizeToTrayCheck),
	)
	
	return container.NewVScroll(
		container.NewVBox(
			widget.NewLabel("UI Settings"),
			widget.NewSeparator(),
			uiSettingsForm,
			widget.NewSeparator(),
			widget.NewLabel("性能与内存"),
			memoryMonitorButton,
		),
	)
}

// buildDataSettingsTab builds the data settings tab
func (sv *SettingsView) buildDataSettingsTab() fyne.CanvasObject {
	return container.NewVScroll(sv.buildDataSettings())
}

// buildUsageStatsTab builds the usage statistics tab
func (sv *SettingsView) buildUsageStatsTab() fyne.CanvasObject {
	usageStatsView := NewUsageStatsView(sv.app)
	return usageStatsView.Build()
}

// buildEditForm builds the provider edit form
func (sv *SettingsView) buildEditForm() {
	sv.nameEntry = widget.NewEntry()
	sv.nameEntry.SetPlaceHolder("Provider key (e.g., openai, kimi)")
	sv.nameEntry.Disable() // Config key cannot be changed after creation
	
	sv.displayNameEntry = widget.NewEntry()
	sv.displayNameEntry.SetPlaceHolder("Display name (e.g., OpenAI, Kimi)")
	
	sv.apiKeyEntry = widget.NewEntry()
	sv.apiKeyEntry.SetPlaceHolder("API Key")
	
	sv.baseURLEntry = widget.NewEntry()
	sv.baseURLEntry.SetPlaceHolder("Base URL (e.g., https://api.openai.com/v1)")
	
	sv.modelEntry = widget.NewEntry()
	sv.modelEntry.SetPlaceHolder("Default Model")
	
	sv.modelsEntry = widget.NewEntry()
	sv.modelsEntry.SetPlaceHolder("Available models (comma-separated)")
	
	sv.enabledCheck = widget.NewCheck("Enabled", nil)
	
	sv.maxTokensEntry = widget.NewEntry()
	sv.maxTokensEntry.SetPlaceHolder("Max Tokens (optional)")
	
	sv.temperatureEntry = widget.NewEntry()
	sv.temperatureEntry.SetPlaceHolder("Temperature (0.0-2.0, optional)")
	
	sv.saveButton = widget.NewButton("Save Changes", func() {
		sv.saveProviderConfig()
	})
	sv.saveButton.Importance = widget.HighImportance
	
	sv.testButton = widget.NewButton("Test Connection", func() {
		sv.testProviderConnection()
	})
	
	sv.deleteButton = widget.NewButton("Delete Provider", func() {
		sv.deleteProvider()
	})
	sv.deleteButton.Importance = widget.DangerImportance
	
	form := container.NewVBox(
		widget.NewLabel("Provider Configuration"),
		widget.NewSeparator(),
		widget.NewForm(
			widget.NewFormItem("Config Key", sv.nameEntry),
			widget.NewFormItem("Display Name", sv.displayNameEntry),
			widget.NewFormItem("API Key", sv.apiKeyEntry),
			widget.NewFormItem("Base URL", sv.baseURLEntry),
			widget.NewFormItem("Default Model", sv.modelEntry),
			widget.NewFormItem("Available Models", sv.modelsEntry),
			widget.NewFormItem("Max Tokens", sv.maxTokensEntry),
			widget.NewFormItem("Temperature", sv.temperatureEntry),
			widget.NewFormItem("", sv.enabledCheck),
		),
		container.NewHBox(
			sv.saveButton,
			sv.testButton,
			sv.deleteButton,
		),
	)
	
	sv.editContainer = container.NewVBox(
		form,
	)
}

// loadProviderConfig loads a provider config into the edit form
func (sv *SettingsView) loadProviderConfig(name string) {
	config, ok := sv.providerConfigs[name]
	if !ok {
		return
	}
	
	sv.nameEntry.SetText(name)
	sv.displayNameEntry.SetText(config.DisplayName)
	sv.apiKeyEntry.SetText(config.APIKey)
	sv.baseURLEntry.SetText(config.BaseURL)
	sv.modelEntry.SetText(config.DefaultModel)
	
	// Convert models slice to comma-separated string
	if len(config.Models) > 0 {
		sv.modelsEntry.SetText(joinModels(config.Models))
	} else {
		sv.modelsEntry.SetText("")
	}
	
	sv.enabledCheck.SetChecked(config.Enabled)
	
	if config.MaxTokens > 0 {
		sv.maxTokensEntry.SetText(strconv.Itoa(config.MaxTokens))
	} else {
		sv.maxTokensEntry.SetText("")
	}
	
	if config.Temperature > 0 {
		sv.temperatureEntry.SetText(fmt.Sprintf("%.2f", config.Temperature))
	} else {
		sv.temperatureEntry.SetText("")
	}
}

// saveProviderConfig saves the current provider config
func (sv *SettingsView) saveProviderConfig() {
	if sv.selectedProvider == "" {
		sv.app.logger.Warn("No provider selected")
		return
	}
	
	config := &utils.ProviderConfig{
		DisplayName:  sv.displayNameEntry.Text,
		APIKey:       sv.apiKeyEntry.Text,
		BaseURL:      sv.baseURLEntry.Text,
		DefaultModel: sv.modelEntry.Text,
		Enabled:      sv.enabledCheck.Checked,
	}
	
	// Parse models from comma-separated string
	if sv.modelsEntry.Text != "" {
		config.Models = parseModels(sv.modelsEntry.Text)
	}
	
	// Parse optional fields
	if sv.maxTokensEntry.Text != "" {
		var maxTokens int
		_, err := fmt.Sscanf(sv.maxTokensEntry.Text, "%d", &maxTokens)
		if err == nil {
			config.MaxTokens = maxTokens
		}
	}
	
	if sv.temperatureEntry.Text != "" {
		var temp float64
		_, err := fmt.Sscanf(sv.temperatureEntry.Text, "%f", &temp)
		if err == nil {
			config.Temperature = temp
		}
	}
	
	// Update in memory
	sv.providerConfigs[sv.selectedProvider] = config
	sv.app.config.LLMProviders[sv.selectedProvider] = *config
	
	// Save to file
	configPath := utils.GetConfigPath()
	if err := utils.SaveConfig(configPath, sv.app.config); err != nil {
		sv.app.logger.Error("Failed to save config: %v", err)
		sv.showError("Failed to save configuration")
		return
	}
	
	sv.app.logger.Info("Provider %s configuration saved", sv.selectedProvider)
	sv.showSuccess("Configuration saved successfully!")
	
	// Refresh providers
	sv.refreshProvidersList()
	
	// Reinitialize providers
	sv.app.providers = make(map[string]llm.Provider)
	sv.app.initProviders()
	
	// Update all open chat tabs' provider lists
	for _, chatView := range sv.app.chatViews {
		if chatView != nil {
			chatView.RefreshProviderList()
		}
	}
}

// deleteProvider deletes the selected provider
func (sv *SettingsView) deleteProvider() {
	if sv.selectedProvider == "" {
		return
	}
	
	// Confirm deletion
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("Delete provider '"+sv.selectedProvider+"'?"),
			container.NewHBox(
				widget.NewButton("Cancel", func() {
					dialog.Hide()
				}),
				widget.NewButton("Delete", func() {
					delete(sv.providerConfigs, sv.selectedProvider)
					delete(sv.app.config.LLMProviders, sv.selectedProvider)
					
					configPath := utils.GetConfigPath()
					if err := utils.SaveConfig(configPath, sv.app.config); err != nil {
						sv.app.logger.Error("Failed to save config: %v", err)
					}
					
					sv.selectedProvider = ""
					sv.clearForm()
					sv.refreshProvidersList()
					dialog.Hide()
					
					sv.app.logger.Info("Provider deleted")
				}),
			),
		),
		sv.getCanvas(),
	)
	dialog.Show()
}

// showAddProviderDialog shows dialog to add a new provider
func (sv *SettingsView) showAddProviderDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Provider name")
	
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("Add New Provider"),
			nameEntry,
			container.NewHBox(
				widget.NewButton("Cancel", func() {
					dialog.Hide()
				}),
				widget.NewButton("Add", func() {
					name := nameEntry.Text
					if name == "" {
						return
					}
					
					if _, exists := sv.providerConfigs[name]; exists {
						sv.showError("Provider already exists")
						return
					}
					
					newConfig := &utils.ProviderConfig{
						APIKey:       "",
						BaseURL:      "",
						DefaultModel: "",
						Enabled:      false,
					}
					
					sv.providerConfigs[name] = newConfig
					sv.app.config.LLMProviders[name] = *newConfig
					sv.selectedProvider = name
					sv.loadProviderConfig(name)
					sv.refreshProvidersList()
					dialog.Hide()
				}),
			),
		),
		sv.getCanvas(),
	)
	dialog.Show()
}

// clearForm clears the edit form
func (sv *SettingsView) clearForm() {
	sv.nameEntry.SetText("")
	sv.displayNameEntry.SetText("")
	sv.apiKeyEntry.SetText("")
	sv.baseURLEntry.SetText("")
	sv.modelEntry.SetText("")
	sv.modelsEntry.SetText("")
	sv.maxTokensEntry.SetText("")
	sv.temperatureEntry.SetText("")
	sv.enabledCheck.SetChecked(false)
}

// showError shows an error message
func (sv *SettingsView) showError(message string) {
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("Error: "+message),
			widget.NewButton("OK", func() {
				dialog.Hide()
			}),
		),
		sv.getCanvas(),
	)
	dialog.Show()
}

// showSuccess shows a success message
func (sv *SettingsView) showSuccess(message string) {
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("Success: "+message),
			widget.NewButton("OK", func() {
				dialog.Hide()
			}),
		),
		sv.getCanvas(),
	)
	dialog.Show()
}

// getCanvas returns the canvas to use for dialogs
func (sv *SettingsView) getCanvas() fyne.Canvas {
	if sv.settingsWindow != nil {
		return sv.settingsWindow.Canvas()
	}
	return sv.app.window.Canvas()
}

// updateProviderNamesList updates the cached provider names list
func (sv *SettingsView) updateProviderNamesList() {
	sv.providerNames = []string{}
	for name := range sv.providerConfigs {
		sv.providerNames = append(sv.providerNames, name)
	}
}

// refreshProvidersList refreshes the provider list UI
func (sv *SettingsView) refreshProvidersList() {
	sv.updateProviderNamesList()
	sv.providersList.Refresh()
}

// parseModels parses a comma-separated string into a slice of model names
func parseModels(modelsStr string) []string {
	if modelsStr == "" {
		return nil
	}
	
	var models []string
	for _, model := range splitByComma(modelsStr) {
		trimmed := trimSpace(model)
		if trimmed != "" {
			models = append(models, trimmed)
		}
	}
	return models
}

// joinModels joins a slice of model names into a comma-separated string
func joinModels(models []string) string {
	if len(models) == 0 {
		return ""
	}
	
	result := ""
	for i, model := range models {
		if i > 0 {
			result += ", "
		}
		result += model
	}
	return result
}

// splitByComma splits a string by comma
func splitByComma(s string) []string {
	var result []string
	var current string
	for _, ch := range s {
		if ch == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)
	
	// Trim leading spaces
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	
	// Trim trailing spaces
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	
	return s[start:end]
}

// applyTheme applies the selected theme
func (sv *SettingsView) applyTheme(theme string) {
	// Normalize theme name
	themeLower := ""
	for _, ch := range theme {
		if ch >= 'A' && ch <= 'Z' {
			themeLower += string(ch + 32) // Convert to lowercase
		} else {
			themeLower += string(ch)
		}
	}
	
	// Update config
	sv.app.config.UI.Theme = themeLower
	
	// Save config
	configPath := utils.GetConfigPath()
	if err := utils.SaveConfig(configPath, sv.app.config); err != nil {
		sv.app.logger.Error("Failed to save config: %v", err)
		sv.showError("Failed to save theme setting")
		return
	}
	
	// Apply theme to app
	if themeLower == "dark" {
		sv.app.fyneApp.Settings().SetTheme(&darkTheme{})
	} else {
		sv.app.fyneApp.Settings().SetTheme(&lightTheme{})
	}
	
	sv.app.logger.Info("Theme changed to: %s", themeLower)
	sv.showSuccess("Theme changed successfully")
}

// testProviderConnection tests the provider connection with current settings
func (sv *SettingsView) testProviderConnection() {
	if sv.selectedProvider == "" {
		sv.showError("Please select a provider first")
		return
	}
	
	// Get current form values
	apiKey := sv.apiKeyEntry.Text
	baseURL := sv.baseURLEntry.Text
	model := sv.modelEntry.Text
	displayName := sv.displayNameEntry.Text
	
	if displayName == "" {
		displayName = sv.selectedProvider
	}
	
	// Validate required fields based on provider type
	if sv.selectedProvider != "ollama" && apiKey == "" {
		sv.showError("API Key is required for this provider")
		return
	}
	
	if baseURL == "" {
		sv.showError("Base URL is required")
		return
	}
	
	if model == "" {
		sv.showError("Default Model is required")
		return
	}
	
	// Parse optional fields
	var maxTokens int
	if sv.maxTokensEntry.Text != "" {
		_, err := fmt.Sscanf(sv.maxTokensEntry.Text, "%d", &maxTokens)
		if err != nil {
			sv.showError("Invalid Max Tokens value")
			return
		}
	}
	
	var temperature float64
	if sv.temperatureEntry.Text != "" {
		_, err := fmt.Sscanf(sv.temperatureEntry.Text, "%f", &temperature)
		if err != nil {
			sv.showError("Invalid Temperature value")
			return
		}
	}
	
	// Create temporary provider config
	config := llm.Config{
		ProviderName: displayName,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Model:        model,
		MaxTokens:    maxTokens,
		Temperature:  temperature,
	}
	
	// Try to initialize the provider
	var provider llm.Provider
	var err error
	
	sv.app.logger.Info("Testing connection for provider: %s", sv.selectedProvider)
	
	if sv.selectedProvider == "ollama" {
		provider, err = llm.NewOllamaProvider(config)
	} else if sv.selectedProvider == "claude" || sv.selectedProvider == "anthropic" {
		provider, err = llm.NewClaudeProvider(config)
	} else if sv.selectedProvider == "gemini" {
		provider, err = llm.NewGeminiProvider(config)
	} else {
		// Treat as OpenAI-compatible
		provider, err = llm.NewOpenAIProvider(config)
	}
	
	if err != nil {
		sv.app.logger.Error("Failed to initialize provider: %v", err)
		sv.showError("Failed to initialize provider: " + err.Error())
		return
	}
	
	// Validate config
	if err := provider.ValidateConfig(); err != nil {
		sv.app.logger.Error("Provider validation failed: %v", err)
		sv.showError("Validation failed: " + err.Error())
		return
	}
	
	// Try a simple test message
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	
	testMessages := []llm.Message{
		{Role: "user", Content: "Hello"},
	}
	
	sv.app.logger.Info("Sending test message...")
	_, err = provider.Chat(ctx, testMessages)
	if err != nil {
		sv.app.logger.Error("Test message failed: %v", err)
		sv.showError("Connection test failed: " + err.Error())
		return
	}
	
	sv.app.logger.Info("Connection test successful for: %s", sv.selectedProvider)
	sv.showSuccess("✅ Connection test successful!\n\nProvider: " + displayName + "\nModel: " + model)
}

// buildDataSettings builds the data settings section
func (sv *SettingsView) buildDataSettings() *fyne.Container {
	// Database statistics
	statsLabel := widget.NewLabel("Loading statistics...")
	statsLabel.Wrapping = fyne.TextWrapWord
	
	// Update statistics
	go sv.updateDBStats(statsLabel)
	
	// Database path (read-only, requires restart to change)
	dbPathEntry := widget.NewEntry()
	dbPathEntry.SetText(sv.app.config.Data.DBPath)
	dbPathEntry.Disable()
	
	dbPathNote := widget.NewLabel("提示: 修改数据库路径需要重启应用")
	dbPathNote.Wrapping = fyne.TextWrapWord
	dbPathNote.TextStyle = fyne.TextStyle{Italic: true}
	
	// Max history entry
	maxHistoryEntry := widget.NewEntry()
	maxHistoryEntry.SetPlaceHolder("1000")
	if sv.app.config.Data.MaxHistory > 0 {
		maxHistoryEntry.SetText(strconv.Itoa(sv.app.config.Data.MaxHistory))
	}
	
	maxHistoryNote := widget.NewLabel("保留的最大对话数量 (0 = 无限制)")
	maxHistoryNote.Wrapping = fyne.TextWrapWord
	maxHistoryNote.TextStyle = fyne.TextStyle{Italic: true}
	
	// Save max history button
	saveMaxHistoryBtn := widget.NewButton("保存历史限制", func() {
		maxHistory := 0
		if maxHistoryEntry.Text != "" {
			val, err := strconv.Atoi(maxHistoryEntry.Text)
			if err != nil || val < 0 {
				sv.showError("请输入有效的数字 (>= 0)")
				return
			}
			maxHistory = val
		}
		
		sv.app.config.Data.MaxHistory = maxHistory
		if err := utils.SaveConfig(sv.app.configPath, sv.app.config); err != nil {
			sv.app.logger.Error("Failed to save max history: %v", err)
			sv.showError("保存失败: " + err.Error())
			return
		}
		
		sv.app.logger.Info("Max history updated to %d", maxHistory)
		sv.showSuccess("历史记录限制已更新")
	})
	
	// Cleanup buttons
	cleanupOldBtn := widget.NewButton("清理旧对话 (>90天)", func() {
		sv.cleanupOldConversations(90)
	})
	cleanupOldBtn.Importance = widget.WarningImportance
	
	applyLimitBtn := widget.NewButton("应用历史限制", func() {
		sv.applyHistoryLimit()
	})
	applyLimitBtn.Importance = widget.WarningImportance
	
	vacuumBtn := widget.NewButton("优化数据库", func() {
		sv.vacuumDatabase(statsLabel)
	})
	
	refreshStatsBtn := widget.NewButton("刷新统计", func() {
		sv.updateDBStats(statsLabel)
	})
	
	// Anonymization settings
	anonymizeCheck := widget.NewCheck("启用匿名化", func(checked bool) {
		sv.app.config.Privacy.AnonymizeSensitiveData = checked
		sv.savePrivacyConfig()
	})
	anonymizeCheck.Checked = sv.app.config.Privacy.AnonymizeSensitiveData

	anonymizeURLsCheck := widget.NewCheck("匿名化 URLs", func(checked bool) {
		sv.app.config.Privacy.AnonymizeURLs = checked
		sv.savePrivacyConfig()
	})
	anonymizeURLsCheck.Checked = sv.app.config.Privacy.AnonymizeURLs

	anonymizeAPIKeysCheck := widget.NewCheck("匿名化 API Keys", func(checked bool) {
		sv.app.config.Privacy.AnonymizeAPIKeys = checked
		sv.savePrivacyConfig()
	})
	anonymizeAPIKeysCheck.Checked = sv.app.config.Privacy.AnonymizeAPIKeys

	anonymizeEmailsCheck := widget.NewCheck("匿名化 Emails", func(checked bool) {
		sv.app.config.Privacy.AnonymizeEmails = checked
		sv.savePrivacyConfig()
	})
	anonymizeEmailsCheck.Checked = sv.app.config.Privacy.AnonymizeEmails

	anonymizeIPsCheck := widget.NewCheck("匿名化 IP 地址", func(checked bool) {
		sv.app.config.Privacy.AnonymizeIPAddresses = checked
		sv.savePrivacyConfig()
	})
	anonymizeIPsCheck.Checked = sv.app.config.Privacy.AnonymizeIPAddresses

	anonymizeFilePathsCheck := widget.NewCheck("匿名化文件路径", func(checked bool) {
		sv.app.config.Privacy.AnonymizeFilePaths = checked
		sv.savePrivacyConfig()
	})
	anonymizeFilePathsCheck.Checked = sv.app.config.Privacy.AnonymizeFilePaths

	anonymizationContainer := container.NewVBox(
		widget.NewLabel("匿名化设置"),
		widget.NewSeparator(),
		anonymizeCheck,
		container.NewGridWithColumns(2,
			anonymizeURLsCheck,
			anonymizeAPIKeysCheck,
			anonymizeEmailsCheck,
			anonymizeIPsCheck,
			anonymizeFilePathsCheck,
		),
	)

	form := widget.NewForm(
		widget.NewFormItem("数据库路径", container.NewVBox(dbPathEntry, dbPathNote)),
		widget.NewFormItem("最大历史记录", container.NewVBox(maxHistoryEntry, maxHistoryNote, saveMaxHistoryBtn)),
	)

	return container.NewVBox(
		widget.NewLabel("Data Settings"),
		widget.NewSeparator(),
		form,
		widget.NewLabel("数据库统计"),
		statsLabel,
		container.NewHBox(refreshStatsBtn),
		widget.NewLabel("数据库维护"),
		container.NewGridWithColumns(2,
			cleanupOldBtn,
			applyLimitBtn,
		),
		container.NewHBox(vacuumBtn),
		widget.NewSeparator(),
		anonymizationContainer,
	)
}

// updateDBStats updates the database statistics label
func (sv *SettingsView) updateDBStats(label *widget.Label) {
	stats, err := sv.app.db.GetStats()
	if err != nil {
		sv.app.logger.Error("Failed to get DB stats: %v", err)
		label.SetText("无法获取统计信息")
		return
	}
	
	sizeKB := float64(stats.DBSizeBytes) / 1024.0
	sizeMB := sizeKB / 1024.0
	
	var sizeStr string
	if sizeMB >= 1.0 {
		sizeStr = fmt.Sprintf("%.2f MB", sizeMB)
	} else {
		sizeStr = fmt.Sprintf("%.2f KB", sizeKB)
	}
	
	statsText := fmt.Sprintf(
		"对话数: %d\n消息数: %d\n数据库大小: %s",
		stats.ConversationCount,
		stats.MessageCount,
		sizeStr,
	)
	
	label.SetText(statsText)
}

// cleanupOldConversations deletes conversations older than specified days
func (sv *SettingsView) cleanupOldConversations(days int) {
	if sv.settingsWindow == nil {
		return
	}
	
	// Confirmation dialog
	var confirmDialog *widget.PopUp
	confirmDialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel(fmt.Sprintf("确定要删除 %d 天前的对话吗？", days)),
			widget.NewLabel("此操作不可撤销！"),
			container.NewHBox(
				widget.NewButton("取消", func() {
					confirmDialog.Hide()
				}),
				widget.NewButton("确定删除", func() {
					confirmDialog.Hide()
					
					count, err := sv.app.db.DeleteOldConversations(days)
					if err != nil {
						sv.app.logger.Error("Failed to delete old conversations: %v", err)
						sv.showError("删除失败: " + err.Error())
						return
					}
					
					sv.app.logger.Info("Deleted %d old conversations", count)
					sv.showSuccess(fmt.Sprintf("已删除 %d 个旧对话", count))
					
					// Refresh conversation list in sidebar
					if sv.app.sidebar != nil {
						sv.app.sidebar.Refresh()
					}
				}),
			),
		),
		sv.settingsWindow.Canvas(),
	)
	
	confirmDialog.Show()
}

// applyHistoryLimit applies the max history limit
func (sv *SettingsView) applyHistoryLimit() {
	maxHistory := sv.app.config.Data.MaxHistory
	if maxHistory <= 0 {
		sv.showError("请先设置历史记录限制")
		return
	}
	
	// Get current count
	count, err := sv.app.db.CountConversations()
	if err != nil {
		sv.app.logger.Error("Failed to count conversations: %v", err)
		sv.showError("获取对话数量失败")
		return
	}
	
	if count <= int64(maxHistory) {
		sv.showSuccess(fmt.Sprintf("当前对话数 (%d) 未超过限制 (%d)", count, maxHistory))
		return
	}
	
	if sv.settingsWindow == nil {
		return
	}
	
	toDelete := count - int64(maxHistory)
	
	// Confirmation dialog
	var confirmDialog *widget.PopUp
	confirmDialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel(fmt.Sprintf("将删除 %d 个最旧的对话", toDelete)),
			widget.NewLabel(fmt.Sprintf("保留最近的 %d 个对话", maxHistory)),
			widget.NewLabel("此操作不可撤销！"),
			container.NewHBox(
				widget.NewButton("取消", func() {
					confirmDialog.Hide()
				}),
				widget.NewButton("确定删除", func() {
					confirmDialog.Hide()
					
					deleted, err := sv.app.db.DeleteOldestConversations(maxHistory)
					if err != nil {
						sv.app.logger.Error("Failed to apply history limit: %v", err)
						sv.showError("应用限制失败: " + err.Error())
						return
					}
					
					sv.app.logger.Info("Deleted %d conversations to apply history limit", deleted)
					sv.showSuccess(fmt.Sprintf("已删除 %d 个对话", deleted))
					
					// Refresh conversation list in sidebar
					if sv.app.sidebar != nil {
						sv.app.sidebar.Refresh()
					}
				}),
			),
		),
		sv.settingsWindow.Canvas(),
	)
	
	confirmDialog.Show()
}

// savePrivacyConfig saves the privacy configuration
func (sv *SettingsView) savePrivacyConfig() {
	if err := utils.SaveConfig(sv.app.configPath, sv.app.config); err != nil {
		sv.app.logger.Error("Failed to save privacy settings: %v", err)
		sv.showError("保存匿名化设置失败: " + err.Error())
		return
	}
	// Apply changes immediately (no restart required)
	sv.app.ApplyPrivacyConfig()
	sv.app.logger.Info("Privacy settings updated")
}

func (sv *SettingsView) vacuumDatabase(statsLabel *widget.Label) {
	sv.app.logger.Info("Starting database vacuum...")
	
	err := sv.app.db.Vacuum()
	if err != nil {
		sv.app.logger.Error("Failed to vacuum database: %v", err)
		sv.showError("优化失败: " + err.Error())
		return
	}
	
	sv.app.logger.Info("Database vacuum completed")
	sv.showSuccess("数据库已优化")
	
	// Refresh statistics
	sv.updateDBStats(statsLabel)
}
