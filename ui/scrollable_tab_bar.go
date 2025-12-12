package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ScrollableTabBar is a custom tab bar with arrow buttons and mouse wheel support
type ScrollableTabBar struct {
	widget.BaseWidget
	tabContainer    *fyne.Container
	leftArrow       *canvas.Text
	rightArrow      *canvas.Text
	leftButton      *fyne.Container
	rightButton     *fyne.Container
	scrollOffset    float32
	viewWidth       float32
}

// Ensure ScrollableTabBar implements required interfaces
var _ fyne.Scrollable = (*ScrollableTabBar)(nil)
var _ fyne.Tappable = (*ScrollableTabBar)(nil)
var _ desktop.Mouseable = (*ScrollableTabBar)(nil)

// NewScrollableTabBar creates a new scrollable tab bar
func NewScrollableTabBar(tabContainer *fyne.Container) *ScrollableTabBar {
	stb := &ScrollableTabBar{
		tabContainer: tabContainer,
		scrollOffset: 0,
	}
	
	// Create arrow text elements
	stb.leftArrow = canvas.NewText("◀", theme.ForegroundColor())
	stb.leftArrow.TextSize = 12
	stb.leftArrow.Alignment = fyne.TextAlignCenter
	
	stb.rightArrow = canvas.NewText("▶", theme.ForegroundColor())
	stb.rightArrow.TextSize = 12
	stb.rightArrow.Alignment = fyne.TextAlignCenter
	
	// Create containers for arrows without background
	stb.leftButton = container.NewCenter(stb.leftArrow)
	stb.rightButton = container.NewCenter(stb.rightArrow)
	
	stb.ExtendBaseWidget(stb)
	return stb
}

// CreateRenderer implements the widget interface
func (stb *ScrollableTabBar) CreateRenderer() fyne.WidgetRenderer {
	return &scrollableTabBarRenderer{
		tabBar:  stb,
		// Tab container first, then arrows on top to ensure visibility
		objects: []fyne.CanvasObject{stb.tabContainer, stb.leftButton, stb.rightButton},
	}
}

// Tapped handles mouse click events on arrows
func (stb *ScrollableTabBar) Tapped(ev *fyne.PointEvent) {
	// Check if left arrow was clicked
	if stb.leftButton.Visible() {
		leftPos := stb.leftButton.Position()
		leftSize := stb.leftButton.Size()
		if ev.Position.X >= leftPos.X && ev.Position.X <= leftPos.X+leftSize.Width &&
			ev.Position.Y >= leftPos.Y && ev.Position.Y <= leftPos.Y+leftSize.Height {
			stb.scrollLeft()
			return
		}
	}
	
	// Check if right arrow was clicked
	if stb.rightButton.Visible() {
		rightPos := stb.rightButton.Position()
		rightSize := stb.rightButton.Size()
		if ev.Position.X >= rightPos.X && ev.Position.X <= rightPos.X+rightSize.Width &&
			ev.Position.Y >= rightPos.Y && ev.Position.Y <= rightPos.Y+rightSize.Height {
			stb.scrollRight()
			return
		}
	}
}

// MouseDown handles mouse button press events
func (stb *ScrollableTabBar) MouseDown(ev *desktop.MouseEvent) {
	// Only handle middle mouse button (tertiary button)
	if ev.Button != desktop.MouseButtonTertiary {
		return
	}
	
	// Calculate the adjusted position considering scroll offset
	arrowWidth := float32(30)
	leftOffset := float32(0)
	if stb.leftButton.Visible() {
		leftOffset = arrowWidth
	}
	
	// Adjust event position to tab container coordinates
	adjustedX := ev.Position.X - leftOffset + stb.scrollOffset
	adjustedPos := fyne.NewPos(adjustedX, ev.Position.Y)
	
	// Find which tab button was clicked
	for _, obj := range stb.tabContainer.Objects {
		if btn, ok := obj.(*CustomTabButton); ok {
			btnPos := btn.Position()
			btnSize := btn.Size()
			
			// Check if the adjusted position is within this button's bounds
			if adjustedPos.X >= btnPos.X && adjustedPos.X <= btnPos.X+btnSize.Width &&
				adjustedPos.Y >= btnPos.Y && adjustedPos.Y <= btnPos.Y+btnSize.Height {
				// Close the tab
				if btn.OnClose != nil {
					btn.OnClose()
				}
				return
			}
		}
	}
}

// MouseUp handles mouse button release events
func (stb *ScrollableTabBar) MouseUp(ev *desktop.MouseEvent) {
	// Not needed for middle click functionality
}

// Scrolled handles mouse wheel events
func (stb *ScrollableTabBar) Scrolled(ev *fyne.ScrollEvent) {
	// Use smooth scrolling with small increments
	scrollAmount := float32(50) // Pixels per scroll notch
	
	if ev.Scrolled.DY != 0 {
		// Convert vertical scroll to horizontal
		// Negative DY means scroll down (should scroll right)
		if ev.Scrolled.DY > 0 {
			stb.scrollOffset -= scrollAmount
		} else {
			stb.scrollOffset += scrollAmount
		}
	} else if ev.Scrolled.DX != 0 {
		// Horizontal scroll
		// Negative DX means scroll left
		if ev.Scrolled.DX > 0 {
			stb.scrollOffset -= scrollAmount
		} else {
			stb.scrollOffset += scrollAmount
		}
	}
	
	stb.clampScroll()
	stb.Refresh()
}

// scrollLeft scrolls the tab bar to the left
func (stb *ScrollableTabBar) scrollLeft() {
	stb.scrollOffset -= stb.getAverageTabWidth()
	stb.clampScroll()
	stb.Refresh()
}

// scrollRight scrolls the tab bar to the right
func (stb *ScrollableTabBar) scrollRight() {
	stb.scrollOffset += stb.getAverageTabWidth()
	stb.clampScroll()
	stb.Refresh()
}

// getAverageTabWidth calculates average tab width for scrolling
func (stb *ScrollableTabBar) getAverageTabWidth() float32 {
	if len(stb.tabContainer.Objects) == 0 {
		return 100
	}
	contentWidth := stb.tabContainer.MinSize().Width
	tabCount := float32(len(stb.tabContainer.Objects))
	return contentWidth / tabCount
}

// clampScroll ensures scroll offset is within valid bounds
func (stb *ScrollableTabBar) clampScroll() {
	if stb.scrollOffset < 0 {
		stb.scrollOffset = 0
	}
	
	maxScroll := stb.getMaxScroll()
	if stb.scrollOffset > maxScroll {
		stb.scrollOffset = maxScroll
	}
}

// getMaxScroll calculates the maximum scroll offset
func (stb *ScrollableTabBar) getMaxScroll() float32 {
	contentWidth := stb.tabContainer.MinSize().Width
	viewWidth := stb.viewWidth
	
	// If content fits in view, no scrolling needed
	if contentWidth <= viewWidth {
		return 0
	}
	
	// Reserve space for arrows (left arrow always shows when scrolling, right arrow at the end)
	arrowWidth := float32(30)
	
	// When at max scroll, we show left arrow but hide right arrow
	// So available width = viewWidth - leftArrowWidth
	availableWidth := viewWidth - arrowWidth
	
	maxScroll := contentWidth - availableWidth
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// updateArrows shows/hides arrows based on scroll position
func (stb *ScrollableTabBar) updateArrows() {
	contentWidth := stb.tabContainer.MinSize().Width
	viewWidth := stb.viewWidth
	
	// Only show arrows if content is wider than view
	if contentWidth <= viewWidth {
		if stb.leftButton.Visible() {
			stb.leftButton.Hide()
		}
		if stb.rightButton.Visible() {
			stb.rightButton.Hide()
		}
		return
	}
	
	maxScroll := stb.getMaxScroll()
	
	// Show left arrow if we can scroll left (offset > 1 to account for floating point)
	shouldShowLeft := stb.scrollOffset > 1
	if shouldShowLeft && !stb.leftButton.Visible() {
		stb.leftButton.Show()
		// log.Printf("[DEBUG] Showing left arrow at offset=%.2f", stb.scrollOffset)
	} else if !shouldShowLeft && stb.leftButton.Visible() {
		stb.leftButton.Hide()
		// log.Printf("[DEBUG] Hiding left arrow at offset=%.2f", stb.scrollOffset)
	}
	
	// Show right arrow if we can scroll right
	shouldShowRight := stb.scrollOffset < maxScroll-1
	if shouldShowRight && !stb.rightButton.Visible() {
		stb.rightButton.Show()
		// log.Printf("[DEBUG] Showing right arrow at offset=%.2f, max=%.2f", stb.scrollOffset, maxScroll)
	} else if !shouldShowRight && stb.rightButton.Visible() {
		stb.rightButton.Hide()
		// log.Printf("[DEBUG] Hiding right arrow at offset=%.2f, max=%.2f", stb.scrollOffset, maxScroll)
	}
}

// Refresh updates the tab bar display
func (stb *ScrollableTabBar) Refresh() {
	stb.updateArrows()
	
	// Directly update tab container position without triggering full layout
	arrowWidth := float32(30)
	leftOffset := float32(0)
	if stb.leftButton.Visible() {
		leftOffset = arrowWidth
	}
	stb.tabContainer.Move(fyne.NewPos(leftOffset-stb.scrollOffset, stb.tabContainer.Position().Y))
	
	canvas.Refresh(stb.tabContainer)
}

// scrollableTabBarRenderer renders the scrollable tab bar
type scrollableTabBarRenderer struct {
	tabBar   *ScrollableTabBar
	objects  []fyne.CanvasObject
	clipRect *canvas.Rectangle // Used to clip tab container overflow
}

func (r *scrollableTabBarRenderer) Layout(size fyne.Size) {
	r.tabBar.viewWidth = size.Width
	r.tabBar.updateArrows()
	
	arrowWidth := float32(30) // Increased from 24 to 30 for better visibility
	leftOffset := float32(0)
	
	// Position left arrow - always at x=0
	if r.tabBar.leftButton.Visible() {
		r.tabBar.leftButton.Resize(fyne.NewSize(arrowWidth, size.Height))
		r.tabBar.leftButton.Move(fyne.NewPos(0, 0))
		leftOffset = arrowWidth
	}
	
	// Position right arrow - always at right edge
	if r.tabBar.rightButton.Visible() {
		r.tabBar.rightButton.Resize(fyne.NewSize(arrowWidth, size.Height))
		r.tabBar.rightButton.Move(fyne.NewPos(size.Width-arrowWidth, 0))
	}
	
	// Position tab container with scroll offset
	// The tab container can extend beyond the visible area (will be clipped)
	r.tabBar.tabContainer.Resize(fyne.NewSize(r.tabBar.tabContainer.MinSize().Width, size.Height))
	r.tabBar.tabContainer.Move(fyne.NewPos(leftOffset-r.tabBar.scrollOffset, 0))
}

func (r *scrollableTabBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(100, r.tabBar.tabContainer.MinSize().Height)
}

func (r *scrollableTabBarRenderer) Refresh() {
	r.tabBar.updateArrows()
	r.tabBar.leftArrow.Color = theme.ForegroundColor()
	r.tabBar.rightArrow.Color = theme.ForegroundColor()
	canvas.Refresh(r.tabBar.leftButton)
	canvas.Refresh(r.tabBar.rightButton)
	canvas.Refresh(r.tabBar.tabContainer)
}

func (r *scrollableTabBarRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *scrollableTabBarRenderer) Destroy() {}
