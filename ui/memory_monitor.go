package ui

import (
	"fmt"
	"light-llm-client/db"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MemoryMonitor provides memory usage information and cache management
type MemoryMonitor struct {
	app *App
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(app *App) *MemoryMonitor {
	return &MemoryMonitor{app: app}
}

// Show displays the memory monitor window
func (mm *MemoryMonitor) Show() {
	win := mm.app.fyneApp.NewWindow("内存监控")
	
	// Memory stats labels
	allocLabel := widget.NewLabel("")
	sysLabel := widget.NewLabel("")
	gcLabel := widget.NewLabel("")
	cacheLabel := widget.NewLabel("")
	
	// Update function
	updateStats := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		allocLabel.SetText(fmt.Sprintf("已分配内存: %.2f MB", float64(m.Alloc)/1024/1024))
		sysLabel.SetText(fmt.Sprintf("系统内存: %.2f MB", float64(m.Sys)/1024/1024))
		gcLabel.SetText(fmt.Sprintf("GC 次数: %d", m.NumGC))
		
		// Cache stats
		cacheSize := len(mm.app.messageCache)
		uiCacheSize := len(mm.app.uiCache)
		openTabs := len(mm.app.chatViews)
		cacheLabel.SetText(fmt.Sprintf("缓存对话数: %d (UI缓存: %d) | 打开标签页: %d | 缓存上限: %d", 
			cacheSize, uiCacheSize, openTabs, mm.app.cacheMaxSize))
	}
	
	// Initial update
	updateStats()
	
	// Refresh button
	refreshButton := widget.NewButton("🔄 刷新", func() {
		updateStats()
	})
	
	// Force GC button
	gcButton := widget.NewButton("🗑️ 强制垃圾回收", func() {
		runtime.GC()
		mm.app.logger.Info("Forced garbage collection")
		updateStats()
	})
	
	// Clear unused cache button
	clearCacheButton := widget.NewButton("🧹 清理未使用缓存", func() {
		mm.app.clearUnusedCache()
		runtime.GC()
		mm.app.logger.Info("Cleared unused cache and ran GC")
		updateStats()
	})
	
	// Clear all cache button
	clearAllButton := widget.NewButton("⚠️ 清空所有缓存", func() {
		// Clear all caches
		mm.app.messageCache = make(map[int64][]*db.Message)
		mm.app.uiCache = make(map[int64][]fyne.CanvasObject)
		mm.app.cacheAccessOrder = make([]int64, 0, mm.app.cacheMaxSize)
		
		runtime.GC()
		mm.app.logger.Info("Cleared all caches and ran GC")
		updateStats()
	})
	clearAllButton.Importance = widget.DangerImportance
	
	// Cache size slider
	cacheSizeLabel := widget.NewLabel(fmt.Sprintf("缓存上限: %d 个对话", mm.app.cacheMaxSize))
	cacheSizeSlider := widget.NewSlider(3, 20)
	cacheSizeSlider.Value = float64(mm.app.cacheMaxSize)
	cacheSizeSlider.Step = 1
	cacheSizeSlider.OnChanged = func(value float64) {
		mm.app.cacheMaxSize = int(value)
		cacheSizeLabel.SetText(fmt.Sprintf("缓存上限: %d 个对话", mm.app.cacheMaxSize))
		
		// Evict if necessary
		for len(mm.app.messageCache) > mm.app.cacheMaxSize {
			mm.app.evictOldestCache()
		}
		
		updateStats()
	}
	
	// Layout
	content := container.NewVBox(
		widget.NewLabel("内存使用情况"),
		widget.NewSeparator(),
		allocLabel,
		sysLabel,
		gcLabel,
		widget.NewSeparator(),
		cacheLabel,
		widget.NewSeparator(),
		cacheSizeLabel,
		cacheSizeSlider,
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			refreshButton,
			gcButton,
		),
		container.NewGridWithColumns(2,
			clearCacheButton,
			clearAllButton,
		),
		widget.NewSeparator(),
		widget.NewLabel("💡 提示:"),
		widget.NewLabel("• 缓存用于加速对话加载"),
		widget.NewLabel("• 关闭标签页会自动清理该对话缓存"),
		widget.NewLabel("• 降低缓存上限可减少内存占用"),
		widget.NewLabel("• 强制垃圾回收可立即释放内存"),
	)
	
	win.SetContent(content)
	win.Resize(fyne.NewSize(500, 600))
	win.Show()
}
