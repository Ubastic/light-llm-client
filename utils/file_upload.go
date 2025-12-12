package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"light-llm-client/llm"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
)

// FileUploadHandler handles file uploads and processing
type FileUploadHandler struct {
	maxFileSize   int64 // Maximum file size in bytes
	maxImageSize  uint  // Maximum image dimension (width or height)
	imageQuality  int   // JPEG quality (1-100)
	allowedTypes  map[string]bool
}

// NewFileUploadHandler creates a new file upload handler with default settings
func NewFileUploadHandler() *FileUploadHandler {
	return &FileUploadHandler{
		maxFileSize:  10 * 1024 * 1024, // 10MB
		maxImageSize: 1024,              // 1024px
		imageQuality: 85,                // 85% quality
		allowedTypes: map[string]bool{
			// Images
			"image/png":  true,
			"image/jpeg": true,
			"image/jpg":  true,
			"image/gif":  true,
			"image/webp": true,
			// Text files
			"text/plain":       true,
			"text/markdown":    true,
			"text/html":        true,
			"text/css":         true,
			"text/javascript":  true,
			"application/json": true,
			"application/xml":  true,
			// Code files
			"text/x-python":     true,
			"text/x-go":         true,
			"text/x-java":       true,
			"text/x-c":          true,
			"text/x-c++":        true,
			"text/x-javascript": true,
		},
	}
}

// ProcessFile processes a file and returns an attachment
func (h *FileUploadHandler) ProcessFile(filePath string) (*llm.Attachment, error) {
	// Check file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// Check file size
	if fileInfo.Size() > h.maxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d bytes)", fileInfo.Size(), h.maxFileSize)
	}

	// Detect MIME type
	mimeType, err := h.detectMimeType(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}

	// Check if file type is allowed
	if !h.allowedTypes[mimeType] {
		return nil, fmt.Errorf("file type not supported: %s", mimeType)
	}

	// Process based on type
	if strings.HasPrefix(mimeType, "image/") {
		return h.processImage(filePath, mimeType)
	} else if strings.HasPrefix(mimeType, "text/") || strings.Contains(mimeType, "json") || strings.Contains(mimeType, "xml") {
		return h.processTextFile(filePath, mimeType)
	}

	return nil, fmt.Errorf("unsupported file type: %s", mimeType)
}

// detectMimeType detects the MIME type of a file
func (h *FileUploadHandler) detectMimeType(filePath string) (string, error) {
	// First try by extension
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)
	
	if mimeType != "" {
		// Remove charset if present
		if idx := strings.Index(mimeType, ";"); idx > 0 {
			mimeType = mimeType[:idx]
		}
		return mimeType, nil
	}

	// If extension detection fails, read file header
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read first 512 bytes for content detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Detect content type
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		// Fallback to basic detection
		switch ext {
		case ".py":
			contentType = "text/x-python"
		case ".go":
			contentType = "text/x-go"
		case ".java":
			contentType = "text/x-java"
		case ".c":
			contentType = "text/x-c"
		case ".cpp", ".cc", ".cxx":
			contentType = "text/x-c++"
		case ".js":
			contentType = "text/javascript"
		case ".md":
			contentType = "text/markdown"
		default:
			// Check if it's text
			if isTextContent(buffer[:n]) {
				contentType = "text/plain"
			} else {
				contentType = "application/octet-stream"
			}
		}
	}

	return contentType, nil
}

// isTextContent checks if content is text
func isTextContent(data []byte) bool {
	// Simple heuristic: check for null bytes
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

// processImage processes an image file
func (h *FileUploadHandler) processImage(filePath string, mimeType string) (*llm.Attachment, error) {
	// Open image file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// Decode image
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Check if resize is needed
	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())
	
	needsResize := width > h.maxImageSize || height > h.maxImageSize

	if needsResize {
		// Calculate new dimensions maintaining aspect ratio
		if width > height {
			img = resize.Resize(h.maxImageSize, 0, img, resize.Lanczos3)
		} else {
			img = resize.Resize(0, h.maxImageSize, img, resize.Lanczos3)
		}
	}

	// Encode to bytes
	var buf bytes.Buffer
	switch format {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: h.imageQuality})
	default:
		// Convert to JPEG for other formats
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: h.imageQuality})
		mimeType = "image/jpeg"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return &llm.Attachment{
		Type:     "image",
		MimeType: mimeType,
		Data:     buf.Bytes(),
		Filename: filepath.Base(filePath),
	}, nil
}

// ProcessImageData processes image data directly (e.g., from clipboard)
func (h *FileUploadHandler) ProcessImageData(img image.Image, filename string) (*llm.Attachment, error) {
	// Check if resize is needed
	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())
	
	needsResize := width > h.maxImageSize || height > h.maxImageSize

	if needsResize {
		// Calculate new dimensions maintaining aspect ratio
		if width > height {
			img = resize.Resize(h.maxImageSize, 0, img, resize.Lanczos3)
		} else {
			img = resize.Resize(0, h.maxImageSize, img, resize.Lanczos3)
		}
	}

	// Encode to PNG
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return &llm.Attachment{
		Type:     "image",
		MimeType: "image/png",
		Data:     buf.Bytes(),
		Filename: filename,
	}, nil
}

// processTextFile processes a text file
func (h *FileUploadHandler) processTextFile(filePath string, mimeType string) (*llm.Attachment, error) {
	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &llm.Attachment{
		Type:     "file",
		MimeType: mimeType,
		Data:     data,
		Filename: filepath.Base(filePath),
	}, nil
}

// AttachmentToBase64 converts an attachment to base64 string
func AttachmentToBase64(att *llm.Attachment) string {
	return base64.StdEncoding.EncodeToString(att.Data)
}

// Base64ToAttachment converts a base64 string back to attachment
func Base64ToAttachment(b64 string, mimeType, filename string) (*llm.Attachment, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	attachmentType := "file"
	if strings.HasPrefix(mimeType, "image/") {
		attachmentType = "image"
	}

	return &llm.Attachment{
		Type:     attachmentType,
		MimeType: mimeType,
		Data:     data,
		Filename: filename,
	}, nil
}

// GetImageDataURL returns a data URL for an image attachment
func GetImageDataURL(att *llm.Attachment) string {
	if att.Type != "image" {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(att.Data)
	return fmt.Sprintf("data:%s;base64,%s", att.MimeType, b64)
}

// GetTextContent returns the text content of a text file attachment
func GetTextContent(att *llm.Attachment) string {
	if att.Type != "file" {
		return ""
	}
	return string(att.Data)
}

// FormatFileSize formats file size in human-readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
