package ui

import (
	"fyne.io/systray"
	_ "image/png"
)

var (
	globalApp *App // Global reference for systray callbacks
)

// SetupSystemTray sets up the system tray icon and menu
func (a *App) SetupSystemTray() {
	globalApp = a
	
	// Start systray in a goroutine
	go systray.Run(onReady, onExit)
	
	a.logger.Info("System tray initialized")
}

// onReady is called when systray is ready
func onReady() {
	// Set icon
	systray.SetIcon(getIconData())
	systray.SetTitle("Light LLM Client")
	systray.SetTooltip("Light LLM Desktop Client")
	
	// Create menu items
	mShow := systray.AddMenuItem("显示窗口", "Show main window")
	mNew := systray.AddMenuItem("新建对话", "Create new conversation")
	mSearch := systray.AddMenuItem("搜索", "Search conversations")
	mSettings := systray.AddMenuItem("设置", "Open settings")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "Quit the application")
	
	// Handle menu item clicks
	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				if globalApp != nil {
					globalApp.window.Show()
					globalApp.logger.Info("Window shown from system tray")
				}
			case <-mNew.ClickedCh:
				if globalApp != nil {
					globalApp.window.Show()
					globalApp.createNewConversation()
					globalApp.logger.Info("New conversation from system tray")
				}
			case <-mSearch.ClickedCh:
				if globalApp != nil {
					globalApp.window.Show()
					globalApp.showSearch()
					globalApp.logger.Info("Search opened from system tray")
				}
			case <-mSettings.ClickedCh:
				if globalApp != nil {
					globalApp.window.Show()
					globalApp.showSettings()
					globalApp.logger.Info("Settings opened from system tray")
				}
			case <-mQuit.ClickedCh:
				if globalApp != nil {
					globalApp.logger.Info("Quit from system tray")
					globalApp.fyneApp.Quit()
				}
				systray.Quit()
				return
			}
		}
	}()
}

// onExit is called when systray exits
func onExit() {
	if globalApp != nil {
		globalApp.logger.Info("System tray exited")
	}
}

// EnableMinimizeToTray enables minimize to tray behavior
func (a *App) EnableMinimizeToTray() {
	// Override window close behavior to minimize to tray instead
	a.window.SetCloseIntercept(func() {
		a.logger.Info("Window close intercepted - minimizing to tray")
		a.window.Hide()
	})
}

// DisableMinimizeToTray disables minimize to tray behavior
func (a *App) DisableMinimizeToTray() {
	// Remove close intercept to allow normal close
	a.window.SetCloseIntercept(nil)
}

// getIconData returns the icon data for system tray
// This is a simple placeholder - you should replace with actual icon data
func getIconData() []byte {
	// Simple 16x16 PNG icon (transparent with a blue square)
	// This is a minimal PNG file - replace with your actual icon
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
		0x3B, 0x49, 0x44, 0x41, 0x54, 0x38, 0x8D, 0x63, 0x64, 0xC0, 0x0F, 0xF0,
		0x0F, 0x62, 0x62, 0x60, 0x60, 0xF8, 0xCF, 0xC0, 0xC0, 0xC0, 0xF0, 0x9F,
		0x81, 0x81, 0x81, 0xE1, 0x3F, 0x03, 0x03, 0x03, 0xC3, 0x7F, 0x06, 0x06,
		0x06, 0x86, 0xFF, 0x0C, 0x0C, 0x0C, 0x0C, 0xFF, 0x19, 0x18, 0x18, 0x18,
		0xFE, 0x33, 0x30, 0x30, 0x30, 0xFC, 0x67, 0x60, 0x60, 0x60, 0x00, 0x00,
		0x1F, 0x84, 0x01, 0x0C, 0x7A, 0x7A, 0x7A, 0x7A, 0x00, 0x00, 0x00, 0x00,
		0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}
