package ui

import (
	"context"
	"fmt"
	"light-llm-client/llm"
	"light-llm-client/utils"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ForkChatView represents a side-by-side comparison chat view
type ForkChatView struct {
	app              *App
	conversationID   int64
	inputEntry       *customEntry
	sendButton       *widget.Button
	columnCount      int
	providerSelects  []*widget.Select
	columnContainers []*fyne.Container
	columnScrolls    []*container.Scroll
}

// NewForkChatView creates a new fork chat view with specified number of columns
func NewForkChatView(app *App, columnCount int) *ForkChatView {
	if columnCount < 2 {
		columnCount = 2
	}
	if columnCount > 4 {
		columnCount = 4
	}

	fv := &ForkChatView{
		app:              app,
		columnCount:      columnCount,
		providerSelects:  make([]*widget.Select, columnCount),
		columnContainers: make([]*fyne.Container, columnCount),
		columnScrolls:    make([]*container.Scroll, columnCount),
	}

	return fv
}

// Build creates the fork chat UI with side-by-side columns
func (fv *ForkChatView) Build() fyne.CanvasObject {
	// Get available providers
	providerOptions := []string{}
	for name := range fv.app.providers {
		providerOptions = append(providerOptions, name)
	}
	if len(providerOptions) == 0 {
		providerOptions = []string{"请在配置文件中启用 LLM 提供商"}
	}

	// Create columns
	columns := []fyne.CanvasObject{}
	for i := 0; i < fv.columnCount; i++ {
		colIdx := i
		
		// Provider selector for this column
		providerLabel := widget.NewLabelWithStyle(
			fmt.Sprintf("模型 %d:", i+1),
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		)
		
		fv.providerSelects[i] = widget.NewSelect(providerOptions, func(value string) {
			fv.app.logger.Info("Column %d selected provider: %s", colIdx+1, value)
		})
		
		// Set default selection
		if len(providerOptions) > i && providerOptions[i] != "请在配置文件中启用 LLM 提供商" {
			fv.providerSelects[i].SetSelected(providerOptions[i])
		} else if len(providerOptions) > 0 && providerOptions[0] != "请在配置文件中启用 LLM 提供商" {
			fv.providerSelects[i].SetSelected(providerOptions[0])
		}
		
		// Messages container for this column
		fv.columnContainers[i] = container.NewVBox()
		fv.columnScrolls[i] = container.NewScroll(fv.columnContainers[i])
		fv.columnScrolls[i].SetMinSize(fyne.NewSize(300, 400))
		
		// Column layout
		column := container.NewBorder(
			container.NewVBox(providerLabel, fv.providerSelects[i], widget.NewSeparator()),
			nil,
			nil,
			nil,
			fv.columnScrolls[i],
		)
		
		columns = append(columns, column)
	}

	// Create grid for columns
	columnsContainer := container.NewGridWithColumns(fv.columnCount, columns...)

	// Input area
	fv.inputEntry = &customEntry{}
	fv.inputEntry.MultiLine = true
	fv.inputEntry.Wrapping = fyne.TextWrapBreak
	fv.inputEntry.SetPlaceHolder("输入消息发送到所有模型... (Ctrl+Enter 发送)")
	fv.inputEntry.SetMinRowsVisible(3)
	fv.inputEntry.onCtrlEnter = func() {
		fv.sendToAllColumns()
	}
	fv.inputEntry.ExtendBaseWidget(fv.inputEntry)

	fv.sendButton = widget.NewButton(fmt.Sprintf("发送到 %d 个模型", fv.columnCount), func() {
		fv.sendToAllColumns()
	})

	inputContainer := container.NewBorder(
		nil,
		nil,
		nil,
		fv.sendButton,
		fv.inputEntry,
	)

	// Main layout
	return container.NewBorder(
		nil,
		inputContainer,
		nil,
		nil,
		columnsContainer,
	)
}

// SetConversation sets the current conversation
func (fv *ForkChatView) SetConversation(conversationID int64) {
	fv.conversationID = conversationID
	fv.loadMessages()
}

// loadMessages loads messages for the current conversation into all columns
func (fv *ForkChatView) loadMessages() {
	if fv.conversationID == 0 {
		fyne.Do(func() {
			for i := 0; i < fv.columnCount; i++ {
				fv.columnContainers[i].Objects = []fyne.CanvasObject{}
				fv.columnContainers[i].Refresh()
			}
		})
		return
	}

	messages, err := fv.app.db.ListMessages(fv.conversationID)
	if err != nil {
		fv.app.logger.Error("Failed to load messages: %v", err)
		return
	}

	fyne.Do(func() {
		// Clear all columns
		for i := 0; i < fv.columnCount; i++ {
			fv.columnContainers[i].Objects = []fyne.CanvasObject{}
		}
		
		// Add messages to all columns
		for _, msg := range messages {
			for i := 0; i < fv.columnCount; i++ {
				fv.addMessageToColumn(i, msg.Role, msg.Content, msg.Model)
			}
		}
		
		for i := 0; i < fv.columnCount; i++ {
			fv.columnContainers[i].Refresh()
		}
	})
}

// addMessageToColumn adds a message to a specific column
func (fv *ForkChatView) addMessageToColumn(columnIdx int, role, content, model string) {
	var roleLabel *widget.Label
	if role == "user" {
		roleLabel = widget.NewLabel("👤 您")
	} else {
		modelText := "🤖 助手"
		if model != "" {
			modelText = fmt.Sprintf("🤖 %s", model)
		}
		roleLabel = widget.NewLabel(modelText)
	}
	roleLabel.TextStyle = fyne.TextStyle{Bold: true}

	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapBreak
	richText.ParseMarkdown(content)

	fv.columnContainers[columnIdx].Add(container.NewVBox(
		roleLabel,
		container.NewPadded(richText),
		widget.NewSeparator(),
	))
}

// sendToAllColumns sends the message to all columns concurrently
func (fv *ForkChatView) sendToAllColumns() {
	content := strings.TrimSpace(fv.inputEntry.Text)
	if content == "" {
		return
	}

	// Create conversation if needed
	if fv.conversationID == 0 {
		conv, err := fv.app.db.CreateConversation("Fork: "+content[:min(50, len(content))], "fork")
		if err != nil {
			fv.app.logger.Error("Failed to create conversation: %v", err)
			fv.app.showError("Failed to create conversation: " + err.Error())
			return
		}
		fv.conversationID = conv.ID
		fv.app.RefreshSidebar()
	}

	// Save user message
	_, err := fv.app.db.CreateMessage(fv.conversationID, "user", content, "", "", "", 0)
	if err != nil {
		fv.app.logger.Error("Failed to save user message: %v", err)
		fv.app.showError("Failed to save user message: " + err.Error())
		return
	}

	// Add user message to all columns
	for i := 0; i < fv.columnCount; i++ {
		fv.addMessageToColumn(i, "user", content, "")
		fv.columnContainers[i].Refresh()
	}
	fv.inputEntry.SetText("")

	// Prepare messages for LLM
	dbMessages, err := fv.app.db.ListMessages(fv.conversationID)
	if err != nil {
		fv.app.logger.Error("Failed to load messages: %v", err)
		return
	}

	llmMessages := []llm.Message{}
	for _, msg := range dbMessages {
		llmMessages = append(llmMessages, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Send to all columns concurrently
	var wg sync.WaitGroup
	for i := 0; i < fv.columnCount; i++ {
		providerName := fv.providerSelects[i].Selected
		if providerName == "" || providerName == "请在配置文件中启用 LLM 提供商" {
			continue
		}
		
		wg.Add(1)
		go fv.sendToColumn(i, providerName, llmMessages, &wg)
	}

	// Wait for all to complete
	utils.SafeGo(fv.app.logger, "fork wait for all columns", func() {
		wg.Wait()
		fv.app.logger.Info("Fork conversation completed for all columns")
	})
}

// sendToColumn sends the message to a specific column's provider
func (fv *ForkChatView) sendToColumn(columnIdx int, providerName string, messages []llm.Message, wg *sync.WaitGroup) {
	defer wg.Done()

	provider, ok := fv.app.providers[providerName]
	if !ok {
		fv.app.logger.Error("Provider not found: %s", providerName)
		return
	}

	// Create placeholder for assistant response
	assistantRichText := widget.NewRichText()
	assistantRichText.Wrapping = fyne.TextWrapBreak
	assistantRoleLabel := widget.NewLabel(fmt.Sprintf("🤖 %s", provider.Name()))
	assistantRoleLabel.TextStyle = fyne.TextStyle{Bold: true}

	assistantRichText.ParseMarkdown("*思考中...*")

	fyne.Do(func() {
		fv.columnContainers[columnIdx].Add(container.NewVBox(
			assistantRoleLabel,
			container.NewPadded(assistantRichText),
			widget.NewSeparator(),
		))
		fv.columnContainers[columnIdx].Refresh()
	})

	// Stream response
	utils.SafeGo(fv.app.logger, fmt.Sprintf("fork column %d stream %s", columnIdx, providerName), func() {
		ctx := context.Background()
		stream, err := provider.StreamChat(ctx, messages)
		if err != nil {
			fv.app.logger.Error("Failed to start chat with %s: %v", providerName, err)
			errorMsg := "**错误**: " + err.Error()
			fyne.Do(func() {
				assistantRichText.ParseMarkdown(errorMsg)
			})
			return
		}

		var fullResponse strings.Builder
		for chunk := range stream {
			if chunk.Error != nil {
				fv.app.logger.Error("Stream error from %s: %v", providerName, chunk.Error)
				errorMsg := "**错误**: " + chunk.Error.Error()
				fyne.Do(func() {
					assistantRichText.ParseMarkdown(errorMsg)
				})
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
				// Save the assistant response for this column
				response := fullResponse.String()
				if response != "" {
					_, err := fv.app.db.CreateMessage(
						fv.conversationID,
						"assistant",
						response,
						providerName,
						provider.Name(),
						"",
						0,
					)
					if err != nil {
						fv.app.logger.Error("Failed to save assistant message for %s: %v", providerName, err)
					} else {
						fv.app.logger.Info("Saved assistant response for column %d (%s)", columnIdx+1, providerName)
					}
				}
				break
			}
		}
	})
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ShowForkDialog creates a fork chat view in a new tab
func ShowForkDialog(app *App, currentConversationID int64) {
	// Get the last user message from current conversation
	messages, err := app.db.ListMessages(currentConversationID)
	if err != nil || len(messages) == 0 {
		app.showError("无法获取对话消息")
		return
	}

	// Find the last user message
	var lastUserMessage string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserMessage = messages[i].Content
			break
		}
	}

	if lastUserMessage == "" {
		app.showError("未找到用户消息")
		return
	}

	// Create a new fork chat view with 2 columns by default
	forkView := NewForkChatView(app, 2)
	forkContent := forkView.Build()

	// Pre-fill the input with the last user message
	forkView.inputEntry.SetText(lastUserMessage)

	// Create tab with close button
	app.forkTabItem = app.tabs.Append("🔀 分叉对话", forkContent, func() {
		app.closeForkTab()
	})
	
	app.logger.Info("Created fork conversation tab")
}
