package ui

import (
	"light-llm-client/db"
	"light-llm-client/utils"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ConversationItem represents a clickable conversation item with context menu
type ConversationItem struct {
	widget.BaseWidget
	app          *App
	conversation *db.Conversation
	label        *widget.Label
	onTapped     func()
	hisighlighted bool
}

// NewConversationItem creates a new conversation item
func NewConversationItem(app *App, conv *db.Conversation, onTapped func()) *ConversationItem {
	item := &ConversationItem{
		app:          app,
		conversation: conv,
		onTapped:     onTapped,
		hisighlighted: false,
	}
	// Display title with category tag if category exists
	displayText := conv.Title
	if conv.Category != "" {
		displayText = "[" + conv.Category + "] " + conv.Title
	}
	item.label = widget.NewLabel(displayText)
	item.ExtendBaseWidget(item)
	return item
}

// CreateRenderer creates the renderer for the conversation item
func (ci *ConversationItem) CreateRenderer() fyne.WidgetRenderer {
	// Create a container with background for highlighting
	container := container.NewStack(
		ci.label,
	)
	return widget.NewSimpleRenderer(container)
}

// Tapped handles left-click
func (ci *ConversationItem) Tapped(_ *fyne.PointEvent) {
	if ci.onTapped != nil {
		ci.onTapped()
	}
}

// TappedSecondary handles right-click
func (ci *ConversationItem) TappedSecondary(pe *fyne.PointEvent) {
	ci.showContextMenu(pe.AbsolutePosition)
}

// showContextMenu shows the context menu for this conversation
func (ci *ConversationItem) showContextMenu(pos fyne.Position) {
	// Create menu items
	renameItem := fyne.NewMenuItem("重命名", func() {
		ci.app.renameConversationByID(ci.conversation.ID)
	})
	
	categoryItem := fyne.NewMenuItem("设置分类", func() {
		ci.app.setCategoryForConversation(ci.conversation.ID)
	})
	
	exportJSONItem := fyne.NewMenuItem("导出为 JSON", func() {
		ci.app.exportConversation(ci.conversation.ID, utils.FormatJSON)
	})
	
	exportMarkdownItem := fyne.NewMenuItem("导出为 Markdown", func() {
		ci.app.exportConversation(ci.conversation.ID, utils.FormatMarkdown)
	})
	
	deleteItem := fyne.NewMenuItem("删除", func() {
		ci.app.deleteConversationByID(ci.conversation.ID)
	})
	
	// Create and show popup menu
	menu := fyne.NewMenu("", renameItem, categoryItem, exportJSONItem, exportMarkdownItem, deleteItem)
	popupMenu := widget.NewPopUpMenu(menu, ci.app.window.Canvas())
	popupMenu.ShowAtPosition(pos)
}

// UpdateTitle updates the conversation title
func (ci *ConversationItem) UpdateTitle(title string) {
	ci.conversation.Title = title
	// Display title with category tag if category exists
	displayText := title
	if ci.conversation.Category != "" {
		displayText = "[" + ci.conversation.Category + "] " + title
	}
	ci.label.SetText(displayText)
	ci.Refresh()
}

// SetHighlighted sets the highlighted state
func (ci *ConversationItem) SetHighlighted(highlighted bool) {
	if ci.hisighlighted != highlighted {
		ci.hisighlighted = highlighted
		if highlighted {
			ci.label.TextStyle = fyne.TextStyle{Bold: true}
		} else {
			ci.label.TextStyle = fyne.TextStyle{}
		}
		ci.Refresh()
	}
}

// IsHighlighted returns whether the item is highlighted
func (ci *ConversationItem) IsHighlighted() bool {
	return ci.hisighlighted
}

// ConversationSidebar represents the sidebar with conversation list
type ConversationSidebar struct {
	widget.BaseWidget
	app            *App
	items          []*ConversationItem
	list           *fyne.Container
	scroll         *container.Scroll
	searchEntry    *widget.Entry
	categoryFilter *widget.Select
	filterText     string
	filterCategory string
	preloadRunning bool // Flag to prevent duplicate preloading
}

// NewConversationSidebar creates a new conversation sidebar
func NewConversationSidebar(app *App) *ConversationSidebar {
	sidebar := &ConversationSidebar{
		app:            app,
		list:           container.NewVBox(),
		filterText:     "",
		filterCategory: "",
	}
	
	// Create search entry
	sidebar.searchEntry = widget.NewEntry()
	sidebar.searchEntry.SetPlaceHolder("🔍 搜索对话...")
	sidebar.searchEntry.OnChanged = func(text string) {
		sidebar.filterText = text
		sidebar.updateList()
	}
	
	// Create category filter
	sidebar.categoryFilter = widget.NewSelect([]string{"全部分类"}, func(selected string) {
		if selected == "全部分类" {
			sidebar.filterCategory = ""
		} else {
			sidebar.filterCategory = selected
		}
		sidebar.updateList()
	})
	sidebar.categoryFilter.SetSelected("全部分类")
	
	sidebar.ExtendBaseWidget(sidebar)
	sidebar.updateList()
	return sidebar
}

// CreateRenderer creates the renderer for the sidebar
func (cs *ConversationSidebar) CreateRenderer() fyne.WidgetRenderer {
	cs.scroll = container.NewScroll(cs.list)
	// Add search entry and category filter at the top
	topContainer := container.NewVBox(
		cs.searchEntry,
		cs.categoryFilter,
	)
	content := container.NewBorder(
		topContainer,
		nil, nil, nil,
		cs.scroll,
	)
	return widget.NewSimpleRenderer(content)
}

// updateList updates the conversation list
func (cs *ConversationSidebar) updateList() {
	if cs.list == nil {
		return
	}
	
	conversations, err := cs.app.db.ListConversations(100, 0)
	if err != nil {
		cs.app.logger.Error("Failed to load conversations: %v", err)
		conversations = []*db.Conversation{}
	}
	cs.app.conversations = conversations
	
	// Update category filter options
	categories, err := cs.app.db.GetCategories()
	if err != nil {
		cs.app.logger.Error("Failed to get categories: %v", err)
		categories = []string{}
	}
	categoryOptions := append([]string{"全部分类"}, categories...)
	cs.categoryFilter.Options = categoryOptions
	
	// Clear existing items
	cs.items = []*ConversationItem{}
	cs.list.Objects = []fyne.CanvasObject{}
	
	// Get currently active conversation from tabs
	activeConvID := cs.app.getActiveConversationID()
	
	// Create new items (with filtering)
	for _, conv := range conversations {
		// Apply search text filter
		if cs.filterText != "" {
			// Case-insensitive search
			if !strings.Contains(strings.ToLower(conv.Title), strings.ToLower(cs.filterText)) {
				continue
			}
		}
		
		// Apply category filter
		if cs.filterCategory != "" {
			if conv.Category != cs.filterCategory {
				continue
			}
		}
		
		// Capture conv in closure
		conversation := conv
		item := NewConversationItem(cs.app, conversation, func() {
			cs.app.selectedConversationID = conversation.ID
			cs.app.openChatTab(conversation.ID)
			cs.updateHighlight(conversation.ID)
			
			// Preload next conversation when user clicks on one
			cs.preloadNextConversation(conversation.ID)
		})
		
		// Highlight if this is the active conversation
		if conversation.ID == activeConvID {
			item.SetHighlighted(true)
		}
		
		cs.items = append(cs.items, item)
		cs.list.Add(item)
		cs.list.Add(widget.NewSeparator())
	}
	
	// Preload all visible conversations after list is updated
	cs.preloadVisibleConversations()
}

// preloadVisibleConversations preloads conversations that are currently visible or near visible
func (cs *ConversationSidebar) preloadVisibleConversations() {
	// Check if preloading is already running
	if cs.preloadRunning {
		cs.app.logger.Info("Preloading already in progress, skipping")
		return
	}
	
	cs.preloadRunning = true
	
	// Preload first 5 conversations sequentially (reduced to save memory)
	utils.SafeGo(cs.app.logger, "preloadVisibleConversations", func() {
		defer func() {
			cs.preloadRunning = false
		}()
		
		count := 5
		if len(cs.items) < count {
			count = len(cs.items)
		}
		
		cs.app.logger.Info("Starting to preload %d visible conversations", count)
		
		for i := 0; i < count; i++ {
			if i >= len(cs.items) {
				break
			}
			
			convID := cs.items[i].conversation.ID
			
			// Skip if already cached or tab is open
			if _, cached := cs.app.messageCache[convID]; cached {
				continue
			}
			if _, tabOpen := cs.app.chatViews[convID]; tabOpen {
				continue
			}
			
			// Load messages synchronously in this goroutine (sequential loading)
			messages, err := cs.app.db.ListMessages(convID)
			if err != nil {
				cs.app.logger.Warn("Failed to preload conversation %d: %v", convID, err)
				continue
			}
			
			// Cache the messages
			cs.app.messageCache[convID] = messages
			cs.app.updateCacheAccess(convID) // Update LRU
			
			// Pre-build UI objects for instant display
			// Create a temporary ChatView just for building UI
			tempChatView := NewChatView(cs.app)
			tempChatView.conversationID = convID
			
			uiObjects := make([]fyne.CanvasObject, 0, len(messages)*4)
			for j, msg := range messages {
				messageBox := tempChatView.buildMessageUI(msg, j)
				uiObjects = append(uiObjects, messageBox)
			}
			
			// Cache the UI objects
			cs.app.uiCache[convID] = uiObjects
			
			cs.app.logger.Info("Preloaded conversation %d (%d messages, %d UI objects) [%d/%d]", convID, len(messages), len(uiObjects), i+1, count)
		}
		
		cs.app.logger.Info("Finished preloading visible conversations")
	})
}

// updateHighlight updates the highlight state for all items
func (cs *ConversationSidebar) updateHighlight(activeConvID int64) {
	for _, item := range cs.items {
		item.SetHighlighted(item.conversation.ID == activeConvID)
	}
}

// preloadNextConversation preloads the next conversation in the list
func (cs *ConversationSidebar) preloadNextConversation(currentID int64) {
	// Find current conversation index
	currentIndex := -1
	for i, conv := range cs.app.conversations {
		if conv.ID == currentID {
			currentIndex = i
			break
		}
	}
	
	if currentIndex == -1 || currentIndex >= len(cs.app.conversations)-1 {
		return
	}
	
	// Preload next 2 conversations
	for i := currentIndex + 1; i < currentIndex+3 && i < len(cs.app.conversations); i++ {
		cs.app.preloadConversationByID(cs.app.conversations[i].ID)
	}
}

// Refresh refreshes the sidebar
func (cs *ConversationSidebar) Refresh() {
	cs.updateList()
	fyne.Do(func() {
		cs.BaseWidget.Refresh()
	})
}
