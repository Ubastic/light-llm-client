package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// customTheme wraps the default theme with custom font sizes
type customTheme struct {
	baseFontSize float32
	baseTheme    fyne.Theme
}

// newCustomTheme creates a new custom theme with the specified font size
func newCustomTheme(baseFontSize int, isDark bool) fyne.Theme {
	var base fyne.Theme
	if isDark {
		base = theme.DarkTheme()
	} else {
		base = theme.LightTheme()
	}
	
	return &customTheme{
		baseFontSize: float32(baseFontSize),
		baseTheme:    base,
	}
}

func (t *customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Make disabled input background transparent/same as normal background
	if name == theme.ColorNameInputBackground {
		return t.baseTheme.Color(theme.ColorNameBackground, variant)
	}
	// Make disabled text same color as normal text
	if name == theme.ColorNameDisabled {
		return t.baseTheme.Color(theme.ColorNameForeground, variant)
	}
	return t.baseTheme.Color(name, variant)
}

func (t *customTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.baseTheme.Font(style)
}

func (t *customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.baseTheme.Icon(name)
}

func (t *customTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return t.baseFontSize
	case theme.SizeNameHeadingText:
		return t.baseFontSize * 1.5
	case theme.SizeNameSubHeadingText:
		return t.baseFontSize * 1.2
	case theme.SizeNameCaptionText:
		return t.baseFontSize * 0.85
	default:
		return t.baseTheme.Size(name)
	}
}
