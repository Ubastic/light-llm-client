package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// CustomTabButton is a button that shows title and close button together
type CustomTabButton struct {
	widget.BaseWidget
	Title      string
	IsActive   bool
	OnTapped   func()
	OnClose    func()
	
	background *canvas.Rectangle
	titleLabel *canvas.Text
	closeLabel *canvas.Text
}

// NewCustomTabButton creates a new custom tab button
func NewCustomTabButton(title string, onTapped func(), onClose func()) *CustomTabButton {
	btn := &CustomTabButton{
		Title:    title,
		OnTapped: onTapped,
		OnClose:  onClose,
	}
	btn.ExtendBaseWidget(btn)
	return btn
}

// CreateRenderer implements the widget interface
func (b *CustomTabButton) CreateRenderer() fyne.WidgetRenderer {
	b.background = canvas.NewRectangle(theme.ButtonColor())
	
	// Create title text
	b.titleLabel = canvas.NewText(b.Title, theme.ForegroundColor())
	b.titleLabel.TextSize = 14 // Normal font size
	if b.IsActive {
		b.titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	}
	
	// Create close button text with slight spacing
	b.closeLabel = canvas.NewText(" ✕", theme.ForegroundColor())
	b.closeLabel.TextSize = 14 // Same size as title for alignment
	b.closeLabel.Alignment = fyne.TextAlignCenter
	
	// Use border container to ensure proper alignment
	content := container.NewHBox(b.titleLabel, b.closeLabel)
	
	return &customTabButtonRenderer{
		button:     b,
		background: b.background,
		content:    content,
		objects:    []fyne.CanvasObject{b.background, content},
	}
}

// Tapped handles tap events
func (b *CustomTabButton) Tapped(ev *fyne.PointEvent) {
	// Check if close button area was tapped (right side)
	size := b.Size()
	if ev.Position.X > size.Width-30 { // Close button area (approximate)
		if b.OnClose != nil {
			b.OnClose()
		}
	} else {
		if b.OnTapped != nil {
			b.OnTapped()
		}
	}
}

// MouseDown handles mouse button press events
func (b *CustomTabButton) MouseDown(ev *desktop.MouseEvent) {
	// Handle middle mouse button to close tab
	if ev.Button == desktop.MouseButtonTertiary {
		if b.OnClose != nil {
			b.OnClose()
		}
	}
}

// MouseUp handles mouse button release events
func (b *CustomTabButton) MouseUp(ev *desktop.MouseEvent) {
	// Not needed for middle click functionality
}

// SetActive updates the active state
func (b *CustomTabButton) SetActive(active bool) {
	b.IsActive = active
	if b.titleLabel != nil {
		if active {
			b.titleLabel.TextStyle = fyne.TextStyle{Bold: true}
		} else {
			b.titleLabel.TextStyle = fyne.TextStyle{}
		}
		b.titleLabel.Refresh()
	}
	b.Refresh()
}

// SetTitle updates the title
func (b *CustomTabButton) SetTitle(title string) {
	b.Title = title
	if b.titleLabel != nil {
		b.titleLabel.Text = title
		b.titleLabel.Refresh()
	}
}

// customTabButtonRenderer renders the custom tab button
type customTabButtonRenderer struct {
	button     *CustomTabButton
	background *canvas.Rectangle
	content    *fyne.Container
	objects    []fyne.CanvasObject
}

func (r *customTabButtonRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.content.Resize(size)
}

func (r *customTabButtonRenderer) MinSize() fyne.Size {
	// Smaller padding for more compact tabs
	return r.content.MinSize().Add(fyne.NewSize(theme.Padding(), theme.Padding()/2))
}

func (r *customTabButtonRenderer) Refresh() {
	// Use subtle background for active tab, transparent for inactive
	if r.button.IsActive {
		r.background.FillColor = theme.HoverColor()
	} else {
		r.background.FillColor = theme.BackgroundColor()
	}
	r.background.Refresh()
	
	// Update text colors
	if r.button.titleLabel != nil {
		r.button.titleLabel.Color = theme.ForegroundColor()
		r.button.titleLabel.Refresh()
	}
	if r.button.closeLabel != nil {
		r.button.closeLabel.Color = theme.ForegroundColor()
		r.button.closeLabel.Refresh()
	}
	
	r.content.Refresh()
}

func (r *customTabButtonRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *customTabButtonRenderer) Destroy() {}
