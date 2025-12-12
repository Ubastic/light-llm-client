package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"light-llm-client/llm"
	"light-llm-client/utils"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// customEntry extends Entry to handle Ctrl+Enter and Ctrl+V paste
type customEntry struct {
	widget.Entry
	onCtrlEnter func()
	onPaste     func() // Called when Ctrl+V is pressed to handle clipboard paste
}

// TypedShortcut handles keyboard shortcuts
func (e *customEntry) TypedShortcut(shortcut fyne.Shortcut) {
	// Check for paste shortcut (Ctrl+V)
	if _, ok := shortcut.(*fyne.ShortcutPaste); ok {
		if e.onPaste != nil {
			e.onPaste()
		}
		// Still allow default paste behavior for text
		e.Entry.TypedShortcut(shortcut)
		return
	}
	
	// Check if it's a custom keyboard shortcut
	if ks, ok := shortcut.(*desktop.CustomShortcut); ok {
		// Check for Ctrl+Return or Ctrl+Enter
		if (ks.KeyName == fyne.KeyReturn || ks.KeyName == fyne.KeyEnter) &&
			ks.Modifier == desktop.ControlModifier {
			if e.onCtrlEnter != nil {
				e.onCtrlEnter()
				return
			}
		}
	}
	// Let the parent Entry handle other shortcuts
	e.Entry.TypedShortcut(shortcut)
}

// TypedKey intercepts key events as a fallback
func (e *customEntry) TypedKey(key *fyne.KeyEvent) {
	// Check for Enter/Return with Ctrl modifier
	if key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter {
		// Try to get current modifiers from desktop driver
		if drv, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
			mods := drv.CurrentKeyModifiers()
			if mods&fyne.KeyModifierControl != 0 {
				if e.onCtrlEnter != nil {
					e.onCtrlEnter()
					return
				}
			}
		}
	}
	// Let the parent Entry handle other keys
	e.Entry.TypedKey(key)
}

// newSelectableText creates a selectable text widget using RichText for plain text
// RichText naturally supports text selection and has no borders
// This function does NOT parse markdown - use for user messages
func newSelectableText(text string) *widget.RichText {
	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapBreak
	
	// Always use plain text segment - do NOT parse markdown
	// This prevents Fyne's markdown parser bugs from causing layout issues
	segment := &widget.TextSegment{
		Text:  text,
		Style: widget.RichTextStyle{},
	}
	richText.Segments = []widget.RichTextSegment{segment}
	richText.Refresh()
	
	return richText
}

// newSelectableMarkdownText creates a selectable text widget with markdown parsing
// Use this for assistant messages that need markdown formatting
func newSelectableMarkdownText(text string) *widget.RichText {
	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapBreak
	
	// Parse markdown for assistant messages
	richText.ParseMarkdown(text)
	
	return richText
}

// newSelectableCodeText creates a selectable text widget for code with monospace font
func newSelectableCodeText(text string) *widget.RichText {
	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapBreak
	// Create text segment with monospace style
	segment := &widget.TextSegment{
		Text:  text,
		Style: widget.RichTextStyle{TextStyle: fyne.TextStyle{Monospace: true}},
	}
	richText.Segments = []widget.RichTextSegment{segment}
	richText.Refresh()
	return richText
}

// ChatView represents the chat interface
type ChatView struct {
	app              *App
	conversationID   int64
	currentProvider  string
	messagesContainer *fyne.Container
	inputEntry       *customEntry
	sendButton       *widget.Button
	providerSelect   *widget.Select
	fileUploadArea   *FileUploadArea
}

// streamChatWithRetry attempts to stream chat with retry logic
func (cv *ChatView) streamChatWithRetry(ctx context.Context, provider llm.Provider, messages []llm.Message, maxRetries int) (<-chan llm.StreamResponse, error) {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			waitTime := time.Duration(1<<uint(attempt-1)) * time.Second
			cv.app.logger.Info("Retrying in %v (attempt %d/%d)...", waitTime, attempt, maxRetries)
			time.Sleep(waitTime)
		}
		
		stream, err := provider.StreamChat(ctx, messages)
		if err == nil {
			if attempt > 0 {
				cv.app.logger.Info("Retry successful on attempt %d", attempt+1)
			}
			return stream, nil
		}
		
		lastErr = err
		cv.app.logger.Warn("Stream chat attempt %d failed: %v", attempt+1, err)
		
		// Check if error is retryable (network errors, timeouts, etc.)
		if !isRetryableError(err) {
			cv.app.logger.Info("Error is not retryable, stopping retry attempts")
			break
		}
	}
	
	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}

// isRetryableError checks if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Network-related errors that should be retried
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"network",
		"dial tcp",
		"i/o timeout",
		"no such host",
		"connection timed out",
		"eof",
	}
	
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	
	return false
}

// NewChatView creates a new chat view
func NewChatView(app *App) *ChatView {
	cv := &ChatView{
		app:             app,
		conversationID:  0,
		currentProvider: "",
	}

	return cv
}

// Build builds the chat view UI
func (cv *ChatView) Build() fyne.CanvasObject {
	// Messages container (scrollable)
	cv.messagesContainer = container.NewVBox()
	messagesScroll := container.NewScroll(cv.messagesContainer)
	messagesScroll.SetMinSize(fyne.NewSize(600, 400))

	// Provider selection
	providerOptions := []string{}
	for name := range cv.app.providers {
		providerOptions = append(providerOptions, name)
	}
	if len(providerOptions) == 0 {
		providerOptions = []string{"请在配置文件中启用 LLM 提供商"}
	}

	cv.providerSelect = widget.NewSelect(providerOptions, func(value string) {
		cv.currentProvider = value
		cv.app.logger.Info("Selected provider: %s", value)
	})
	if len(providerOptions) > 0 && providerOptions[0] != "请在配置文件中启用 LLM 提供商" {
		cv.providerSelect.SetSelected(providerOptions[0])
		cv.currentProvider = providerOptions[0]
	}

	// File upload area (create first so we can reference it in inputEntry)
	cv.fileUploadArea = NewFileUploadArea(cv.app, func(attachments []*llm.Attachment) {
		cv.app.logger.Info("Attachments updated: %d files", len(attachments))
	})

	// Input area - use custom entry to handle Ctrl+Enter and Ctrl+V
	cv.inputEntry = &customEntry{}
	cv.inputEntry.MultiLine = true
	cv.inputEntry.Wrapping = fyne.TextWrapBreak
	cv.inputEntry.SetPlaceHolder("输入消息... (Ctrl+Enter 发送, Ctrl+V 粘贴图片/文件)")
	cv.inputEntry.SetMinRowsVisible(3)
	cv.inputEntry.onCtrlEnter = func() {
		cv.sendMessage()
	}
	cv.inputEntry.onPaste = func() {
		// Handle clipboard paste for images and files
		cv.fileUploadArea.HandleClipboardPaste()
	}
	cv.inputEntry.ExtendBaseWidget(cv.inputEntry)

	cv.sendButton = widget.NewButton("发送", func() {
		cv.sendMessage()
	})

	// Input area with file upload
	inputWithFiles := container.NewBorder(
		cv.fileUploadArea,
		nil,
		nil,
		nil,
		cv.inputEntry,
	)

	inputContainer := container.NewBorder(
		nil,
		nil,
		nil,
		cv.sendButton,
		inputWithFiles,
	)

	// Fork button
	forkButton := widget.NewButton("🔀 分叉对话", func() {
		if cv.conversationID == 0 {
			cv.app.showError("请先创建对话")
			return
		}
		ShowForkDialog(cv.app, cv.conversationID)
	})

	// Top bar with provider selection and fork button
	topBar := container.NewBorder(
		nil,
		nil,
		widget.NewLabel("模型提供商:"),
		forkButton,
		cv.providerSelect,
	)

	// Main layout
	return container.NewBorder(
		topBar,
		inputContainer,
		nil,
		nil,
		messagesScroll,
	)
}

// SetConversation sets the current conversation
func (cv *ChatView) SetConversation(conversationID int64) {
	cv.conversationID = conversationID
	cv.loadMessages()
}

// loadMessages loads messages for the current conversation
func (cv *ChatView) loadMessages() {
	if cv.conversationID == 0 {
		fyne.Do(func() {
			cv.messagesContainer.Objects = []fyne.CanvasObject{}
			cv.messagesContainer.Refresh()
		})
		return
	}

	// Check if UI is already cached (fastest path)
	if cachedUI, cached := cv.app.uiCache[cv.conversationID]; cached {
		cv.app.updateCacheAccess(cv.conversationID) // Update LRU
		cv.app.logger.Info("Using cached UI for conversation %d (%d objects)", cv.conversationID, len(cachedUI))
		
		// For large conversations, use progressive rendering
		if len(cachedUI) > 10 {
			// Show first 5 messages immediately
			initialBatch := cachedUI[:5]
			cv.messagesContainer.Objects = initialBatch
			cv.messagesContainer.Refresh()
			
			// Load rest progressively in background
			utils.SafeGo(cv.app.logger, "progressive-render", func() {
				// Small delay to let UI settle
				time.Sleep(10 * time.Millisecond)
				
				fyne.Do(func() {
					cv.messagesContainer.Objects = cachedUI
					cv.messagesContainer.Refresh()
				})
			})
		} else {
			// Small conversation, load all at once
			cv.messagesContainer.Objects = cachedUI
			cv.messagesContainer.Refresh()
		}
		return
	}
	
	// Check if messages are already cached (fast path)
	if cachedMessages, cached := cv.app.messageCache[cv.conversationID]; cached {
		cv.app.updateCacheAccess(cv.conversationID) // Update LRU
		cv.app.logger.Info("Using cached messages for conversation %d", cv.conversationID)
		// Build UI from cached messages in background
		utils.SafeGo(cv.app.logger, "loadMessages-cached", func() {
			uiObjects := make([]fyne.CanvasObject, 0, len(cachedMessages)*4)
			for i, msg := range cachedMessages {
				messageBox := cv.buildMessageUI(msg.Role, msg.Content, msg.Model, i)
				uiObjects = append(uiObjects, messageBox)
			}
			
			// Cache the UI objects for next time
			cv.app.uiCache[cv.conversationID] = uiObjects
			
			fyne.Do(func() {
				cv.messagesContainer.Objects = uiObjects
				cv.messagesContainer.Refresh()
			})
		})
		return
	}

	// Immediately clear and show loading indicator
	loadingLabel := widget.NewLabel("📖 加载消息中...")
	loadingLabel.TextStyle = fyne.TextStyle{Italic: true}
	fyne.Do(func() {
		cv.messagesContainer.Objects = []fyne.CanvasObject{loadingLabel}
		cv.messagesContainer.Refresh()
	})

	// Load messages asynchronously to avoid blocking UI
	utils.SafeGo(cv.app.logger, "loadMessages", func() {
		messages, err := cv.app.db.ListMessages(cv.conversationID)
		if err != nil {
			cv.app.logger.Error("Failed to load messages: %v", err)
			fyne.Do(func() {
				cv.messagesContainer.Objects = []fyne.CanvasObject{
					widget.NewLabel("❌ 加载失败: " + err.Error()),
				}
				cv.messagesContainer.Refresh()
			})
			return
		}

		// Cache the messages for future use
		cv.app.messageCache[cv.conversationID] = messages
		cv.app.updateCacheAccess(cv.conversationID) // Update LRU

		// Build all UI objects in background
		uiObjects := make([]fyne.CanvasObject, 0, len(messages)*4) // Pre-allocate capacity
		for i, msg := range messages {
			messageBox := cv.buildMessageUI(msg.Role, msg.Content, msg.Model, i)
			uiObjects = append(uiObjects, messageBox)
		}
		
		// Cache the UI objects for future use
		cv.app.uiCache[cv.conversationID] = uiObjects

		// Update UI in one batch operation
		fyne.Do(func() {
			cv.messagesContainer.Objects = uiObjects
			cv.messagesContainer.Refresh()
		})
	})
}

// sendMessage sends the current message
func (cv *ChatView) sendMessage() {
	content := strings.TrimSpace(cv.inputEntry.Text)
	attachments := cv.fileUploadArea.GetAttachments()
	
	if content == "" && len(attachments) == 0 {
		return
	}

	// Create conversation if needed
	if cv.conversationID == 0 {
		conv, err := cv.app.db.CreateConversation("New Chat", "")
		if err != nil {
			cv.app.logger.Error("Failed to create conversation: %v", err)
			cv.app.showError("Failed to create conversation: " + err.Error())
			return
		}
		cv.conversationID = conv.ID
		cv.app.RefreshSidebar()
	}

	// Process attachments: separate images from text files
	var imageAttachments []llm.Attachment
	var textFileContents []string
	
	for _, att := range attachments {
		if att.Type == "image" {
			// Images are sent as attachments
			imageAttachments = append(imageAttachments, *att)
		} else if att.Type == "file" {
			// Text files: include content in message
			fileContent := utils.GetTextContent(att)
			textFileContents = append(textFileContents, fmt.Sprintf("\n\n--- 文件: %s ---\n%s\n--- 文件结束 ---\n", att.Filename, fileContent))
		}
	}
	
	// Combine user message with text file contents
	fullContent := content
	if len(textFileContents) > 0 {
		fullContent = content + strings.Join(textFileContents, "")
	}
	
	// Serialize all attachments to JSON for database storage
	attachmentsJSON := ""
	if len(attachments) > 0 {
		attachmentsData, err := json.Marshal(attachments)
		if err != nil {
			cv.app.logger.Error("Failed to marshal attachments: %v", err)
		} else {
			attachmentsJSON = string(attachmentsData)
		}
	}

	// Save user message with attachments (save original content + attachments JSON)
	_, err := cv.app.db.CreateMessage(cv.conversationID, "user", fullContent, "", "", attachmentsJSON, 0)
	if err != nil {
		cv.app.logger.Error("Failed to save user message: %v", err)
		cv.app.showError("Failed to save user message: " + err.Error())
		return
	}
	
	// Invalidate cache since we added a new message
	delete(cv.app.messageCache, cv.conversationID)
	delete(cv.app.uiCache, cv.conversationID)

	// Add to UI (use -1 for new messages that aren't in the list yet)
	cv.addMessageToUI("user", fullContent, "", -1)
	cv.inputEntry.SetText("")
	
	// Clear attachments after sending
	cv.fileUploadArea.Clear()

	// Get provider
	provider, ok := cv.app.providers[cv.currentProvider]
	if !ok {
		cv.app.logger.Error("Provider not found: %s", cv.currentProvider)
		cv.addMessageToUI("assistant", "错误: 提供商未配置", "", -1)
		return
	}

	// Prepare messages for LLM
	dbMessages, err := cv.app.db.ListMessages(cv.conversationID)
	if err != nil {
		cv.app.logger.Error("Failed to load messages: %v", err)
		return
	}

	llmMessages := []llm.Message{}
	for _, msg := range dbMessages {
		// Parse attachments from JSON if present
		var allAttachments []llm.Attachment
		if msg.Attachments != "" {
			if err := json.Unmarshal([]byte(msg.Attachments), &allAttachments); err != nil {
				cv.app.logger.Warn("Failed to parse attachments for message %d: %v", msg.ID, err)
			}
		}
		
		// Only include image attachments for LLM (text files are already in content)
		var imageAttachments []llm.Attachment
		for _, att := range allAttachments {
			if att.Type == "image" {
				imageAttachments = append(imageAttachments, att)
			}
		}
		
		llmMessages = append(llmMessages, llm.Message{
			Role:        msg.Role,
			Content:     msg.Content,
			Attachments: imageAttachments,
		})
	}

	// Create placeholder for assistant response with RichText
	assistantRichText := widget.NewRichText()
	assistantRichText.Wrapping = fyne.TextWrapBreak
	assistantRoleLabel := widget.NewLabel("🤖 助手 (" + cv.currentProvider + ")")
	assistantRoleLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	// Add initial "thinking" message
	assistantRichText.ParseMarkdown("*思考中...*")
	
	fyne.Do(func() {
		cv.messagesContainer.Add(container.NewVBox(
			assistantRoleLabel,
			container.NewPadded(assistantRichText),
			widget.NewSeparator(),
		))
		cv.messagesContainer.Refresh()
	})

	// Send to LLM (streaming with retry) - wrapped with panic recovery
	utils.SafeGo(cv.app.logger, "sendMessage LLM streaming", func() {
		ctx := context.Background()
		// Use retry mechanism with max 3 attempts
		stream, err := cv.streamChatWithRetry(ctx, provider, llmMessages, 2)
		if err != nil {
			cv.app.logger.Error("Failed to start chat: %v", err)
			errorMsg := "**错误**: " + err.Error()
			fyne.Do(func() {
				assistantRichText.ParseMarkdown(errorMsg)
			})
			// Save error message so it can be retried
			_, saveErr := cv.app.db.CreateMessage(
				cv.conversationID,
				"assistant",
				errorMsg,
				cv.currentProvider,
				cv.currentProvider,
				"",
				0,
			)
			if saveErr != nil {
				cv.app.logger.Error("Failed to save error message: %v", saveErr)
			}
			// Reload to show retry button
			cv.loadMessages()
			return
		}

		var fullResponse strings.Builder
		for chunk := range stream {
			if chunk.Error != nil {
				cv.app.logger.Error("Stream error: %v", chunk.Error)
				errorMsg := "**错误**: " + chunk.Error.Error()
				fyne.Do(func() {
					assistantRichText.ParseMarkdown(errorMsg)
				})
				// Save error message so it can be retried
				_, saveErr := cv.app.db.CreateMessage(
					cv.conversationID,
					"assistant",
					errorMsg,
					cv.currentProvider,
					cv.currentProvider,
					"",
					0,
				)
				if saveErr != nil {
					cv.app.logger.Error("Failed to save error message: %v", saveErr)
				}
				// Reload to show retry button
				cv.loadMessages()
				break
			}

			if chunk.Content != "" {
				fullResponse.WriteString(chunk.Content)
				// Update RichText with accumulated markdown content
				// ParseMarkdown re-renders the entire content for proper markdown context
				content := fullResponse.String()
				fyne.Do(func() {
					assistantRichText.ParseMarkdown(content)
				})
			}

			if chunk.Done {
				// Save assistant message
				_, err := cv.app.db.CreateMessage(
					cv.conversationID,
					"assistant",
					fullResponse.String(),
					cv.currentProvider,
					cv.currentProvider,
					"",
					0,
				)
				if err != nil {
					cv.app.logger.Error("Failed to save assistant message: %v", err)
				}
				
				// Auto-generate title if this is the first exchange
				utils.SafeGo(cv.app.logger, "autoGenerateTitle", cv.autoGenerateTitle)
				
				// Reload messages to show the new one with proper action buttons
				cv.loadMessages()
				break
			}
		}
	})
}

// autoGenerateTitle automatically generates a title for the conversation
func (cv *ChatView) autoGenerateTitle() {
	if cv.conversationID == 0 {
		return
	}
	
	// Get the conversation to check current title
	conv, err := cv.app.db.GetConversation(cv.conversationID)
	if err != nil {
		cv.app.logger.Error("Failed to get conversation: %v", err)
		return
	}
	
	// Only generate title if it's still "New Chat"
	if conv.Title != "New Chat" {
		return
	}
	
	// Get messages
	dbMessages, err := cv.app.db.ListMessages(cv.conversationID)
	if err != nil {
		cv.app.logger.Error("Failed to load messages for title generation: %v", err)
		return
	}
	
	// Only generate if we have at least 2 messages (user + assistant)
	if len(dbMessages) < 2 {
		return
	}
	
	// Get provider
	provider, ok := cv.app.providers[cv.currentProvider]
	if !ok {
		cv.app.logger.Warn("Provider not found for title generation: %s", cv.currentProvider)
		return
	}
	
	// Convert to LLM messages
	llmMessages := []llm.Message{}
	for _, msg := range dbMessages {
		llmMessages = append(llmMessages, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	
	// Generate title with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	title, err := provider.GenerateTitle(ctx, llmMessages)
	if err != nil {
		cv.app.logger.Error("Failed to generate title: %v", err)
		return
	}
	
	// Update conversation title
	err = cv.app.db.UpdateConversation(cv.conversationID, title, conv.Category)
	if err != nil {
		cv.app.logger.Error("Failed to update conversation title: %v", err)
		return
	}
	
	cv.app.logger.Info("Auto-generated title: %s", title)
	
	// Refresh sidebar to show new title
	cv.app.RefreshSidebar()
	
	// Update tab title if open
	fyne.Do(func() {
		if tabItem, exists := cv.app.tabItems[cv.conversationID]; exists {
			tabItem.Title = title
			cv.app.tabs.refreshTabBar()
		}
	})
}

// buildMessageUI builds a message UI component without adding it to the container
// This is used for batch loading messages
func (cv *ChatView) buildMessageUI(role, content, model string, messageIndex int) fyne.CanvasObject {
	var roleLabel string
	if role == "user" {
		roleLabel = "👤 用户"
	} else {
		roleLabel = "🤖 助手"
		if model != "" {
			roleLabel += fmt.Sprintf(" (%s)", model)
		}
	}

	// Create role label with bold style
	roleWidget := widget.NewLabel(roleLabel)
	roleWidget.TextStyle = fyne.TextStyle{Bold: true}
	
	var contentWidget fyne.CanvasObject
	if role == "assistant" {
		// Assistant messages: parse and render with code block copy buttons
		contentWidget = cv.renderAssistantMessage(content)
	} else {
		// User messages use selectable text
		contentWidget = newSelectableText(content)
	}

	// Create action buttons
	var actionButtons *fyne.Container
	if role == "assistant" {
		// For assistant messages, provide copy, edit and regenerate options
		copyTextButton := widget.NewButton("📋 复制文本", func() {
			// Convert markdown to plain text (simple conversion)
			plainText := cv.markdownToPlainText(content)
			cv.app.window.Clipboard().SetContent(plainText)
			cv.app.logger.Info("Message text copied to clipboard")
		})
		copyTextButton.Importance = widget.LowImportance
		
		copyMarkdownButton := widget.NewButton("📄 复制 Markdown", func() {
			// Copy original markdown content
			cv.app.window.Clipboard().SetContent(content)
			cv.app.logger.Info("Message markdown copied to clipboard")
		})
		copyMarkdownButton.Importance = widget.LowImportance
		
		// Capture messageIndex in closure
		idx := messageIndex
		
		editButton := widget.NewButton("✏️ 编辑", func() {
			cv.editMessage(idx, content)
		})
		editButton.Importance = widget.LowImportance
		
		regenerateButton := widget.NewButton("🔄 重新生成", func() {
			cv.regenerateMessage(idx)
		})
		regenerateButton.Importance = widget.LowImportance
		
		actionButtons = container.NewHBox(copyTextButton, copyMarkdownButton, editButton, regenerateButton)
	} else {
		// For user messages, add copy, edit, and delete buttons
		copyButton := widget.NewButton("📋 复制", func() {
			cv.app.window.Clipboard().SetContent(content)
			cv.app.logger.Info("Message copied to clipboard")
		})
		copyButton.Importance = widget.LowImportance
		
		// Capture messageIndex in closure
		idx := messageIndex
		editButton := widget.NewButton("✏️ 编辑", func() {
			cv.editMessage(idx, content)
		})
		editButton.Importance = widget.LowImportance
		
		deleteButton := widget.NewButton("🗑️ 删除", func() {
			cv.deleteMessage(idx)
		})
		deleteButton.Importance = widget.LowImportance
		
		actionButtons = container.NewHBox(copyButton, editButton, deleteButton)
	}

	messageBox := container.NewVBox(
		roleWidget,
		container.NewPadded(contentWidget),
		actionButtons,
		widget.NewSeparator(),
	)

	return messageBox
}

// addMessageToUI adds a message to the UI
// messageIndex is the index in the messages list, or -1 for new messages
func (cv *ChatView) addMessageToUI(role, content, model string, messageIndex int) {
	messageBox := cv.buildMessageUI(role, content, model, messageIndex)
	fyne.Do(func() {
		cv.messagesContainer.Add(messageBox)
		cv.messagesContainer.Refresh()
	})
}

// renderAssistantMessage renders assistant message with code block copy buttons, tables, and thinking sections
func (cv *ChatView) renderAssistantMessage(content string) fyne.CanvasObject {
	// Aggressive quick path: if no special markers, render as plain text
	// This avoids expensive parsing for most messages
	hasCodeBlock := strings.Contains(content, "```")
	hasTable := strings.Contains(content, "|")
	hasThinking := strings.Contains(content, "<think>")
	
	if !hasCodeBlock && !hasTable && !hasThinking {
		// Pure text message with markdown formatting
		return newSelectableMarkdownText(content)
	}
	
	// Parse content to find code blocks, tables, and thinking sections
	parts := cv.parseMarkdownContent(content)
	
	if len(parts) == 1 && parts[0].isCode == false && parts[0].isTable == false && parts[0].isThinking == false {
		// No special content after parsing, use markdown text
		return newSelectableMarkdownText(content)
	}
	
	// Build UI with special sections
	contentContainer := container.NewVBox()
	
	for _, part := range parts {
		if part.isThinking {
			// Create collapsible thinking section
			thinkingWidget := cv.createThinkingSection(part.content)
			contentContainer.Add(thinkingWidget)
		} else if part.isCode {
			// Create language label if language is specified
			var languageLabel *widget.Label
			if part.language != "" {
				languageLabel = widget.NewLabel("📝 " + part.language)
				languageLabel.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
			}
			
			// Create code block with copy button - use monospace text
			codeText := newSelectableCodeText(part.content)
			
			// Capture code content in closure
			code := part.content
			copyCodeButton := widget.NewButton("📋 复制代码", func() {
				cv.app.window.Clipboard().SetContent(code)
				cv.app.logger.Info("Code copied to clipboard")
			})
			copyCodeButton.Importance = widget.LowImportance
			
			// Create header with language and copy button
			var header fyne.CanvasObject
			if languageLabel != nil {
				header = container.NewBorder(nil, nil, languageLabel, copyCodeButton, widget.NewLabel(""))
			} else {
				header = container.NewHBox(copyCodeButton)
			}
			
			// Code block container with button (no scroll, let it expand naturally)
			codeBlock := container.NewBorder(
				header, // Put language and button at top
				nil,
				nil,
				nil,
				codeText,
			)
			
			contentContainer.Add(codeBlock)
		} else if part.isTable {
			// Render table
			tableWidget := cv.renderMarkdownTable(part.content)
			if tableWidget != nil {
				contentContainer.Add(tableWidget)
			}
		} else {
			// Regular content - use markdown text for assistant messages
			if strings.TrimSpace(part.content) != "" {
				contentContainer.Add(newSelectableMarkdownText(part.content))
			}
		}
	}
	
	return contentContainer
}

// createThinkingSection creates a collapsible thinking section
func (cv *ChatView) createThinkingSection(thinkingContent string) fyne.CanvasObject {
	// Create the thinking content widget (initially hidden)
	thinkingText := newSelectableMarkdownText(thinkingContent)
	thinkingContainer := container.NewVBox(thinkingText)
	thinkingContainer.Hide() // Initially hidden
	
	// Create toggle button
	isExpanded := false
	toggleButton := widget.NewButton("💭 显示思考过程", nil)
	toggleButton.Importance = widget.LowImportance
	
	// Set up toggle functionality
	toggleButton.OnTapped = func() {
		isExpanded = !isExpanded
		if isExpanded {
			thinkingContainer.Show()
			toggleButton.SetText("💭 隐藏思考过程")
		} else {
			thinkingContainer.Hide()
			toggleButton.SetText("💭 显示思考过程")
		}
		toggleButton.Refresh()
	}
	
	// Create a styled container for the thinking section
	thinkingSection := container.NewVBox(
		toggleButton,
		thinkingContainer,
		widget.NewSeparator(),
	)
	
	return thinkingSection
}

// markdownPart represents a part of markdown content
type markdownPart struct {
	content    string
	isCode     bool
	isTable    bool
	isThinking bool
	language   string
}

// parseMarkdownContent parses markdown and extracts code blocks, tables, and thinking sections
func (cv *ChatView) parseMarkdownContent(markdown string) []markdownPart {
	var parts []markdownPart
	
	// First pass: extract <think> tags
	thinkingParts := cv.extractThinkingTags(markdown)
	
	// Second pass: extract code blocks and tables from each part
	for _, part := range thinkingParts {
		if part.isThinking {
			// Keep thinking parts as-is
			parts = append(parts, part)
		} else {
			// Process regular content for code blocks and tables
			processedParts := cv.extractCodeBlocksAndTables(part.content)
			parts = append(parts, processedParts...)
		}
	}
	
	return parts
}

// extractThinkingTags extracts <think> sections from content
func (cv *ChatView) extractThinkingTags(content string) []markdownPart {
	var parts []markdownPart
	var currentPart strings.Builder
	
	// Use regex-like approach to find <think> tags
	for {
		thinkStart := strings.Index(content, "<think>")
		if thinkStart == -1 {
			// No more thinking tags, add remaining content
			if len(content) > 0 || currentPart.Len() > 0 {
				currentPart.WriteString(content)
				if currentPart.Len() > 0 {
					parts = append(parts, markdownPart{
						content:    currentPart.String(),
						isThinking: false,
					})
				}
			}
			break
		}
		
		// Add content before <think>
		if thinkStart > 0 || currentPart.Len() > 0 {
			currentPart.WriteString(content[:thinkStart])
			if currentPart.Len() > 0 {
				parts = append(parts, markdownPart{
					content:    currentPart.String(),
					isThinking: false,
				})
				currentPart.Reset()
			}
		}
		
		// Find closing </think>
		content = content[thinkStart+7:] // Skip "<think>"
		thinkEnd := strings.Index(content, "</think>")
		if thinkEnd == -1 {
			// No closing tag, treat rest as thinking content
			parts = append(parts, markdownPart{
				content:    content,
				isThinking: true,
			})
			break
		}
		
		// Add thinking content
		thinkingContent := content[:thinkEnd]
		parts = append(parts, markdownPart{
			content:    strings.TrimSpace(thinkingContent),
			isThinking: true,
		})
		
		// Continue with content after </think>
		content = content[thinkEnd+8:] // Skip "</think>"
	}
	
	return parts
}

// extractCodeBlocksAndTables extracts code blocks and tables from text
func (cv *ChatView) extractCodeBlocksAndTables(markdown string) []markdownPart {
	var parts []markdownPart
	var currentPart strings.Builder
	inCodeBlock := false
	var currentLanguage string
	
	lines := strings.Split(markdown, "\n")
	
	// Extract code blocks
	i := 0
	for i < len(lines) {
		line := lines[i]
		
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				parts = append(parts, markdownPart{
					content:  currentPart.String(),
					isCode:   true,
					language: currentLanguage,
				})
				currentPart.Reset()
				inCodeBlock = false
				currentLanguage = ""
			} else {
				// Start of code block
				if currentPart.Len() > 0 {
					// Process the accumulated content for tables
					processedParts := cv.extractTablesFromText(currentPart.String())
					parts = append(parts, processedParts...)
					currentPart.Reset()
				}
				inCodeBlock = true
				// Extract language if specified
				if len(line) > 3 {
					currentLanguage = strings.TrimSpace(line[3:])
				}
			}
		} else {
			currentPart.WriteString(line)
			if i < len(lines)-1 {
				currentPart.WriteString("\n")
			}
		}
		i++
	}
	
	// Add remaining content
	if currentPart.Len() > 0 {
		if inCodeBlock {
			parts = append(parts, markdownPart{
				content: currentPart.String(),
				isCode:  true,
			})
		} else {
			// Process for tables
			processedParts := cv.extractTablesFromText(currentPart.String())
			parts = append(parts, processedParts...)
		}
	}
	
	return parts
}

// extractTablesFromText extracts tables from text content
func (cv *ChatView) extractTablesFromText(text string) []markdownPart {
	var parts []markdownPart
	var currentPart strings.Builder
	var tableLines []string
	inTable := false
	
	lines := strings.Split(text, "\n")
	
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		
		// Check if line looks like a table row (contains |)
		isTableLine := strings.Contains(trimmedLine, "|") && len(trimmedLine) > 0
		
		if isTableLine {
			if !inTable {
				// Start of table - save previous content
				if currentPart.Len() > 0 {
					parts = append(parts, markdownPart{
						content: currentPart.String(),
						isCode:  false,
						isTable: false,
					})
					currentPart.Reset()
				}
				inTable = true
			}
			tableLines = append(tableLines, line)
		} else {
			if inTable {
				// End of table - validate and save
				if cv.isValidMarkdownTable(tableLines) {
					parts = append(parts, markdownPart{
						content: strings.Join(tableLines, "\n"),
						isCode:  false,
						isTable: true,
					})
				} else {
					// Not a valid table, treat as regular content
					currentPart.WriteString(strings.Join(tableLines, "\n"))
					currentPart.WriteString("\n")
				}
				tableLines = nil
				inTable = false
			}
			
			currentPart.WriteString(line)
			if i < len(lines)-1 {
				currentPart.WriteString("\n")
			}
		}
	}
	
	// Handle remaining table
	if inTable && len(tableLines) > 0 {
		if cv.isValidMarkdownTable(tableLines) {
			parts = append(parts, markdownPart{
				content: strings.Join(tableLines, "\n"),
				isCode:  false,
				isTable: true,
			})
		} else {
			currentPart.WriteString(strings.Join(tableLines, "\n"))
		}
	}
	
	// Add remaining content
	if currentPart.Len() > 0 {
		parts = append(parts, markdownPart{
			content: currentPart.String(),
			isCode:  false,
			isTable: false,
		})
	}
	
	return parts
}

// isValidMarkdownTable checks if lines form a valid markdown table
func (cv *ChatView) isValidMarkdownTable(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	
	// Check if second line is a separator line (contains dashes and pipes)
	secondLine := strings.TrimSpace(lines[1])
	if !strings.Contains(secondLine, "-") || !strings.Contains(secondLine, "|") {
		return false
	}
	
	// Validate separator line format
	parts := strings.Split(secondLine, "|")
	validSeparators := 0
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		// Check if it's mostly dashes (with optional colons for alignment)
		cleaned := strings.ReplaceAll(trimmed, "-", "")
		cleaned = strings.ReplaceAll(cleaned, ":", "")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			validSeparators++
		}
	}
	
	return validSeparators >= 1
}

// renderMarkdownTable renders a markdown table as a Fyne widget
func (cv *ChatView) renderMarkdownTable(tableMarkdown string) fyne.CanvasObject {
	lines := strings.Split(strings.TrimSpace(tableMarkdown), "\n")
	if len(lines) < 2 {
		return nil
	}
	
	// Parse table rows
	var rows [][]string
	for i, line := range lines {
		if i == 1 {
			// Skip separator line
			continue
		}
		
		// Split by | and clean up
		cells := strings.Split(line, "|")
		var cleanCells []string
		for _, cell := range cells {
			trimmed := strings.TrimSpace(cell)
			if trimmed != "" {
				cleanCells = append(cleanCells, trimmed)
			}
		}
		
		if len(cleanCells) > 0 {
			rows = append(rows, cleanCells)
		}
	}
	
	if len(rows) == 0 {
		return nil
	}
	
	// Get number of columns
	numCols := len(rows[0])
	
	// Create table container
	tableContainer := container.NewVBox()
	
	// Add header row (first row) with bold style
	if len(rows) > 0 {
		headerCells := []fyne.CanvasObject{}
		for _, cell := range rows[0] {
			// Create rich text with bold style for header
			cellText := widget.NewRichText()
			cellText.Wrapping = fyne.TextWrapWord
			segment := &widget.TextSegment{
				Text:  cell,
				Style: widget.RichTextStyle{TextStyle: fyne.TextStyle{Bold: true}},
			}
			cellText.Segments = []widget.RichTextSegment{segment}
			cellContainer := container.NewPadded(cellText)
			headerCells = append(headerCells, cellContainer)
		}
		headerRow := container.NewGridWithColumns(numCols, headerCells...)
		tableContainer.Add(headerRow)
		tableContainer.Add(widget.NewSeparator())
	}
	
	// Add data rows
	for i := 1; i < len(rows); i++ {
		dataCells := []fyne.CanvasObject{}
		for j := 0; j < numCols; j++ {
			cellContent := ""
			if j < len(rows[i]) {
				cellContent = rows[i][j]
			}
			// Use plain rich text for data cells
			cellText := newSelectableText(cellContent)
			cellContainer := container.NewPadded(cellText)
			dataCells = append(dataCells, cellContainer)
		}
		dataRow := container.NewGridWithColumns(numCols, dataCells...)
		tableContainer.Add(dataRow)
		if i < len(rows)-1 {
			// Add subtle separator between rows
			tableContainer.Add(widget.NewSeparator())
		}
	}
	
	// Wrap in a bordered container for table appearance
	return container.NewPadded(tableContainer)
}

// markdownToPlainText converts markdown to plain text (basic conversion)
func (cv *ChatView) markdownToPlainText(markdown string) string {
	// This is a simple conversion - removes common markdown syntax
	text := markdown
	
	// Remove code blocks (```...```)
	text = strings.ReplaceAll(text, "```", "")
	
	// Remove inline code (`...`)
	for strings.Contains(text, "`") {
		start := strings.Index(text, "`")
		end := strings.Index(text[start+1:], "`")
		if end == -1 {
			break
		}
		text = text[:start] + text[start+1:start+1+end] + text[start+2+end:]
	}
	
	// Remove bold (**...**)
	text = strings.ReplaceAll(text, "**", "")
	
	// Remove italic (*...*)
	text = strings.ReplaceAll(text, "*", "")
	
	// Remove headers (# )
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			lines[i] = strings.TrimLeft(line, "# ")
		}
	}
	text = strings.Join(lines, "\n")
	
	// Remove links [text](url) -> text
	for strings.Contains(text, "[") && strings.Contains(text, "](") {
		start := strings.Index(text, "[")
		middle := strings.Index(text[start:], "](")
		if middle == -1 {
			break
		}
		end := strings.Index(text[start+middle+2:], ")")
		if end == -1 {
			break
		}
		linkText := text[start+1 : start+middle]
		text = text[:start] + linkText + text[start+middle+3+end:]
	}
	
	return text
}

// regenerateMessage regenerates a specific assistant message
// messageIndex is the index of the message to regenerate in the current conversation
func (cv *ChatView) regenerateMessage(messageIndex int) {
	if cv.conversationID == 0 {
		return
	}

	// Get all messages
	dbMessages, err := cv.app.db.ListMessages(cv.conversationID)
	if err != nil {
		cv.app.logger.Error("Failed to load messages: %v", err)
		cv.app.showError("Failed to load messages: " + err.Error())
		return
	}

	// Validate message index
	if messageIndex < 0 || messageIndex >= len(dbMessages) {
		cv.app.logger.Warn("Invalid message index: %d", messageIndex)
		return
	}

	// Check if it's an assistant message
	if dbMessages[messageIndex].Role != "assistant" {
		cv.app.logger.Warn("Cannot regenerate non-assistant message")
		return
	}

	cv.app.logger.Info("Regenerating message at index %d", messageIndex)

	// Get provider
	provider, ok := cv.app.providers[cv.currentProvider]
	if !ok {
		cv.app.logger.Error("Provider not found: %s", cv.currentProvider)
		cv.app.showError("Provider not configured: " + cv.currentProvider)
		return
	}

	// Prepare messages for LLM (exclude the message to regenerate and all after it)
	llmMessages := []llm.Message{}
	for i := 0; i < messageIndex; i++ {
		llmMessages = append(llmMessages, llm.Message{
			Role:    dbMessages[i].Role,
			Content: dbMessages[i].Content,
		})
	}

	// Remove all messages from the regenerated one onwards from UI
	if messageIndex < len(cv.messagesContainer.Objects) {
		fyne.Do(func() {
			cv.messagesContainer.Objects = cv.messagesContainer.Objects[:messageIndex]
			cv.messagesContainer.Refresh()
		})
	}

	// Create placeholder for new assistant response
	assistantRichText := widget.NewRichText()
	assistantRichText.Wrapping = fyne.TextWrapBreak
	assistantRoleLabel := widget.NewLabel("🤖 助手 (" + cv.currentProvider + ")")
	assistantRoleLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	assistantRichText.ParseMarkdown("*重新生成中...*")
	
	fyne.Do(func() {
		cv.messagesContainer.Add(container.NewVBox(
			assistantRoleLabel,
			container.NewPadded(assistantRichText),
			widget.NewSeparator(),
		))
		cv.messagesContainer.Refresh()
	})

	// Send to LLM (streaming with retry) - wrapped with panic recovery
	utils.SafeGo(cv.app.logger, "regenerateMessage LLM streaming", func() {
		ctx := context.Background()
		// Use retry mechanism with max 3 attempts
		stream, err := cv.streamChatWithRetry(ctx, provider, llmMessages, 2)
		if err != nil {
			cv.app.logger.Error("Failed to start chat: %v", err)
			errorMsg := "**错误**: " + err.Error()
			fyne.Do(func() {
				assistantRichText.ParseMarkdown(errorMsg)
			})
			// Save error message so it can be retried
			_, saveErr := cv.app.db.CreateMessage(
				cv.conversationID,
				"assistant",
				errorMsg,
				cv.currentProvider,
				cv.currentProvider,
				"",
				0,
			)
			if saveErr != nil {
				cv.app.logger.Error("Failed to save error message: %v", saveErr)
			}
			// Reload to show retry button
			cv.loadMessages()
			return
		}

		var fullResponse strings.Builder
		for chunk := range stream {
			if chunk.Error != nil {
				cv.app.logger.Error("Stream error: %v", chunk.Error)
				errorMsg := "**错误**: " + chunk.Error.Error()
				fyne.Do(func() {
					assistantRichText.ParseMarkdown(errorMsg)
				})
				// Save error message so it can be retried
				_, saveErr := cv.app.db.CreateMessage(
					cv.conversationID,
					"assistant",
					errorMsg,
					cv.currentProvider,
					cv.currentProvider,
					"",
					0,
				)
				if saveErr != nil {
					cv.app.logger.Error("Failed to save error message: %v", saveErr)
				}
				// Reload to show retry button
				cv.loadMessages()
				break
			}

			if chunk.Content != "" {
				fullResponse.WriteString(chunk.Content)
				content := fullResponse.String()
				fyne.Do(func() {
					assistantRichText.ParseMarkdown(content)
				})
			}

			if chunk.Done {
				// Save new assistant message
				_, err := cv.app.db.CreateMessage(
					cv.conversationID,
					"assistant",
					fullResponse.String(),
					cv.currentProvider,
					cv.currentProvider,
					"",
					0,
				)
				if err != nil {
					cv.app.logger.Error("Failed to save assistant message: %v", err)
				}
				
				// Auto-generate title if needed
				utils.SafeGo(cv.app.logger, "autoGenerateTitle", cv.autoGenerateTitle)
				
				// Reload messages to show the new one with action buttons
				cv.loadMessages()
				break
			}
		}
	})
}

// RefreshProviderList refreshes the provider selection dropdown
func (cv *ChatView) RefreshProviderList() {
	if cv.providerSelect == nil {
		return
	}
	
	// Get current providers
	providerOptions := []string{}
	for name := range cv.app.providers {
		providerOptions = append(providerOptions, name)
	}
	if len(providerOptions) == 0 {
		providerOptions = []string{"请在配置文件中启用 LLM 提供商"}
	}
	
	// Update select options
	cv.providerSelect.Options = providerOptions
	
	// Update current selection if needed
	if cv.currentProvider == "" || !cv.providerExists(cv.currentProvider) {
		if len(providerOptions) > 0 && providerOptions[0] != "请在配置文件中启用 LLM 提供商" {
			cv.providerSelect.SetSelected(providerOptions[0])
			cv.currentProvider = providerOptions[0]
		}
	} else {
		// Keep current selection
		cv.providerSelect.SetSelected(cv.currentProvider)
	}
	
	cv.providerSelect.Refresh()
	cv.app.logger.Info("Provider list refreshed, %d providers available", len(providerOptions))
}

// providerExists checks if a provider exists in the app
func (cv *ChatView) providerExists(name string) bool {
	_, exists := cv.app.providers[name]
	return exists
}

// editMessage allows editing a user message
func (cv *ChatView) editMessage(messageIndex int, currentContent string) {
	if cv.conversationID == 0 {
		return
	}

	// Get all messages
	dbMessages, err := cv.app.db.ListMessages(cv.conversationID)
	if err != nil {
		cv.app.logger.Error("Failed to load messages: %v", err)
		cv.app.showError("Failed to load messages: " + err.Error())
		return
	}

	// Validate message index
	if messageIndex < 0 || messageIndex >= len(dbMessages) {
		cv.app.logger.Warn("Invalid message index: %d", messageIndex)
		return
	}

	messageID := dbMessages[messageIndex].ID

	// Create edit dialog with larger text area
	editEntry := widget.NewMultiLineEntry()
	editEntry.SetText(currentContent)
	editEntry.Wrapping = fyne.TextWrapWord
	editEntry.SetMinRowsVisible(15) // 增加可见行数

	// Create title based on role
	title := "编辑消息"
	if dbMessages[messageIndex].Role == "assistant" {
		title = "编辑助手消息"
	}

	var dialog *widget.PopUp
	// Use border container to give edit entry more space
	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabel(title),
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewHBox(
				widget.NewButton("取消", func() {
					dialog.Hide()
				}),
				widget.NewButton("保存", func() {
					newContent := editEntry.Text
					if newContent == "" {
						cv.app.showError("消息内容不能为空")
						return
					}

					// Update in database
					if err := cv.app.db.UpdateMessage(messageID, newContent); err != nil {
						cv.app.logger.Error("Failed to update message: %v", err)
						cv.app.showError("更新失败: " + err.Error())
						return
					}

					cv.app.logger.Info("Message updated: %d", messageID)

					// Reload messages to show the updated content
					cv.loadMessages()

					dialog.Hide()
				}),
			),
		),
		nil,
		nil,
		editEntry,
	)

	dialog = widget.NewModalPopUp(
		content,
		cv.app.window.Canvas(),
	)
	// 增大对话框尺寸
	dialog.Resize(fyne.NewSize(700, 500))
	dialog.Show()
}

// deleteMessage deletes a message and all subsequent messages
func (cv *ChatView) deleteMessage(messageIndex int) {
	if cv.conversationID == 0 {
		return
	}

	// Get all messages
	dbMessages, err := cv.app.db.ListMessages(cv.conversationID)
	if err != nil {
		cv.app.logger.Error("Failed to load messages: %v", err)
		cv.app.showError("Failed to load messages: " + err.Error())
		return
	}

	// Validate message index
	if messageIndex < 0 || messageIndex >= len(dbMessages) {
		cv.app.logger.Warn("Invalid message index: %d", messageIndex)
		return
	}

	// Check if it's a user message
	if dbMessages[messageIndex].Role != "user" {
		cv.app.logger.Warn("Cannot delete non-user message")
		cv.app.showError("只能删除用户消息")
		return
	}

	// Create confirmation dialog
	var dialog *widget.PopUp
	dialog = widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabel("确认删除"),
			widget.NewLabel("确定要删除此消息及之后的所有消息吗？"),
			widget.NewLabel("此操作不可撤销！"),
			container.NewHBox(
				widget.NewButton("取消", func() {
					dialog.Hide()
				}),
				widget.NewButton("删除", func() {
					// Delete this message and all subsequent messages
					for i := messageIndex; i < len(dbMessages); i++ {
						if err := cv.app.db.DeleteMessage(dbMessages[i].ID); err != nil {
							cv.app.logger.Error("Failed to delete message: %v", err)
							cv.app.showError("删除失败: " + err.Error())
							return
						}
					}

					cv.app.logger.Info("Deleted %d messages starting from index %d", len(dbMessages)-messageIndex, messageIndex)

					// Reload messages
					cv.loadMessages()

					dialog.Hide()
				}),
			),
		),
		cv.app.window.Canvas(),
	)
	dialog.Show()
}
