//go:build !windows
// +build !windows

package ui

import "image"

// getClipboardFiles retrieves file paths from clipboard (non-Windows platforms)
func getClipboardFiles() ([]string, error) {
	// Not implemented for non-Windows platforms
	return nil, nil
}

// getClipboardImage retrieves an image from clipboard (non-Windows platforms)
func getClipboardImage() (image.Image, error) {
	// Not implemented for non-Windows platforms
	return nil, nil
}
