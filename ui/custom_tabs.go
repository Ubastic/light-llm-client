package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CustomTab represents a single tab with a close button
type CustomTab struct {
	Title     string
	Content   fyne.CanvasObject
	OnClose   func()
	tabButton *CustomTabButton
	isActive  bool
}

// CustomTabs is a custom tabs container with close buttons on each tab
type CustomTabs struct {
	widget.BaseWidget
	tabs            []*CustomTab
	activeTab       *CustomTab
	tabBar          *fyne.Container
	scrollableTabBar *ScrollableTabBar
	contentArea     *fyne.Container
	mainContent     *fyne.Container
	OnChanged       func(*CustomTab)
}

// NewCustomTabs creates a new custom tabs container
func NewCustomTabs() *CustomTabs {
	ct := &CustomTabs{
		tabs:        []*CustomTab{},
		tabBar:      container.NewHBox(),
		contentArea: container.NewMax(),
	}
	ct.ExtendBaseWidget(ct)
	// Create scrollable tab bar with arrow buttons
	ct.scrollableTabBar = NewScrollableTabBar(ct.tabBar)
	ct.mainContent = container.NewBorder(ct.scrollableTabBar, nil, nil, nil, ct.contentArea)
	return ct
}

// CreateRenderer implements the widget interface
func (ct *CustomTabs) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ct.mainContent)
}

// Append adds a new tab
func (ct *CustomTabs) Append(title string, content fyne.CanvasObject, onClose func()) *CustomTab {
	tab := &CustomTab{
		Title:   title,
		Content: content,
		OnClose: onClose,
	}
	
	// Create custom tab button with integrated close button
	tab.tabButton = NewCustomTabButton(title, func() {
		ct.SelectTab(tab)
	}, func() {
		if tab.OnClose != nil {
			tab.OnClose()
		}
		ct.Remove(tab)
	})
	
	// Add tab to list
	ct.tabs = append(ct.tabs, tab)
	
	// Update tab bar
	ct.refreshTabBar()
	
	// Select the new tab
	ct.SelectTab(tab)
	
	return tab
}

// Remove removes a tab
func (ct *CustomTabs) Remove(tab *CustomTab) {
	// Find and remove the tab
	for i, t := range ct.tabs {
		if t == tab {
			ct.tabs = append(ct.tabs[:i], ct.tabs[i+1:]...)
			break
		}
	}
	
	// If this was the active tab, select another
	if ct.activeTab == tab {
		if len(ct.tabs) > 0 {
			ct.SelectTab(ct.tabs[len(ct.tabs)-1])
		} else {
			ct.activeTab = nil
			ct.contentArea.Objects = []fyne.CanvasObject{}
		}
	}
	
	// Update tab bar
	ct.refreshTabBar()
}

// SelectTab selects a tab
func (ct *CustomTabs) SelectTab(tab *CustomTab) {
	if ct.activeTab == tab {
		return
	}
	
	// Update active tab
	ct.activeTab = tab
	
	// Update content
	ct.contentArea.Objects = []fyne.CanvasObject{tab.Content}
	ct.contentArea.Refresh()
	
	// Update tab button styles
	ct.refreshTabBar()
	
	// Trigger callback
	if ct.OnChanged != nil {
		ct.OnChanged(tab)
	}
}

// refreshTabBar updates the tab bar display
func (ct *CustomTabs) refreshTabBar() {
	ct.tabBar.Objects = []fyne.CanvasObject{}
	
	for _, tab := range ct.tabs {
		// Update button title
		tab.tabButton.SetTitle(tab.Title)
		
		// Update active state
		tab.tabButton.SetActive(tab == ct.activeTab)
		
		// Add the tab button
		ct.tabBar.Add(tab.tabButton)
	}
	
	ct.tabBar.Refresh()
	// Update scrollable tab bar to show/hide arrows
	if ct.scrollableTabBar != nil {
		ct.scrollableTabBar.Refresh()
	}
}

// GetActiveTab returns the currently active tab
func (ct *CustomTabs) GetActiveTab() *CustomTab {
	return ct.activeTab
}

// TabCount returns the number of tabs
func (ct *CustomTabs) TabCount() int {
	return len(ct.tabs)
}
