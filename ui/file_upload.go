package ui

import (
	"fmt"
	"image/color"
	"light-llm-client/llm"
	"light-llm-client/utils"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// FileAttachmentWidget displays a file attachment with preview and remove button
type FileAttachmentWidget struct {
	widget.BaseWidget
	attachment *llm.Attachment
	onRemove   func()
	container  *fyne.Container
}

// NewFileAttachmentWidget creates a new file attachment widget
func NewFileAttachmentWidget(att *llm.Attachment, onRemove func()) *FileAttachmentWidget {
	w := &FileAttachmentWidget{
		attachment: att,
		onRemove:   onRemove,
	}
	w.ExtendBaseWidget(w)
	return w
}

// CreateRenderer creates the renderer for the file attachment widget
func (w *FileAttachmentWidget) CreateRenderer() fyne.WidgetRenderer {
	// Create icon based on file type
	var icon fyne.CanvasObject
	if w.attachment.Type == "image" {
		// For images, show a thumbnail from data
		if len(w.attachment.Data) > 0 {
			img := canvas.NewImageFromResource(fyne.NewStaticResource(
				w.attachment.Filename,
				w.attachment.Data,
			))
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(60, 60))
			icon = img
		} else {
			// Fallback to image icon if no data
			icon = widget.NewIcon(theme.FileImageIcon())
		}
	} else {
		// For other files, show a file icon
		icon = widget.NewIcon(theme.FileIcon())
	}

	// File info label
	fileInfo := widget.NewLabel(fmt.Sprintf("%s\n%s",
		w.attachment.Filename,
		utils.FormatFileSize(int64(len(w.attachment.Data))),
	))
	fileInfo.Wrapping = fyne.TextWrapWord

	// Remove button
	removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		if w.onRemove != nil {
			w.onRemove()
		}
	})
	removeBtn.Importance = widget.LowImportance

	// Create container with border
	content := container.NewBorder(
		nil,
		nil,
		icon,
		removeBtn,
		fileInfo,
	)

	// Add background
	bg := canvas.NewRectangle(color.NRGBA{R: 200, G: 200, B: 200, A: 50})
	bg.CornerRadius = 5

	w.container = container.NewStack(bg, content)

	return widget.NewSimpleRenderer(w.container)
}

// FileUploadArea represents a file upload area with drag-and-drop support
type FileUploadArea struct {
	widget.BaseWidget
	app         *App
	attachments []*llm.Attachment
	onChange    func([]*llm.Attachment)
	container   *fyne.Container
	handler     *utils.FileUploadHandler
}

// NewFileUploadArea creates a new file upload area
func NewFileUploadArea(app *App, onChange func([]*llm.Attachment)) *FileUploadArea {
	area := &FileUploadArea{
		app:         app,
		attachments: make([]*llm.Attachment, 0),
		onChange:    onChange,
		handler:     utils.NewFileUploadHandler(),
	}
	area.ExtendBaseWidget(area)
	return area
}

// CreateRenderer creates the renderer for the file upload area
func (a *FileUploadArea) CreateRenderer() fyne.WidgetRenderer {
	// Upload button
	uploadBtn := widget.NewButtonWithIcon("添加文件", theme.FileIcon(), func() {
		a.showFilePicker()
	})

	// Attachments container
	attachmentsContainer := container.NewVBox()

	// Update attachments display
	a.updateAttachmentsDisplay(attachmentsContainer)

	a.container = container.NewVBox(
		uploadBtn,
		attachmentsContainer,
	)

	return widget.NewSimpleRenderer(a.container)
}

// showFilePicker shows a file picker dialog
func (a *FileUploadArea) showFilePicker() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			a.app.showError("打开文件失败: " + err.Error())
			return
		}
		if reader == nil {
			return
		}
		
		filePath := reader.URI().Path()
		reader.Close() // Close immediately to release file handle
		
		// Small delay to ensure file handle is released on Windows
		// time.Sleep(100 * time.Millisecond)
		
		a.app.logger.Info("Selected file: %s", filePath)

		// Process file with retry for file lock issues
		var attachment *llm.Attachment
		var processErr error
		maxRetries := 3
		
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				a.app.logger.Info("Retrying file processing (attempt %d/%d)...", attempt+1, maxRetries)
				// Exponential backoff
				// time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
			
			attachment, processErr = a.handler.ProcessFile(filePath)
			if processErr == nil {
				break
			}
			
			// Check if error is due to file lock
			errStr := processErr.Error()
			if strings.Contains(errStr, "being used by another process") || 
			   strings.Contains(errStr, "access is denied") ||
			   strings.Contains(errStr, "The process cannot access the file") {
				a.app.logger.Warn("File lock detected, will retry: %v", processErr)
				continue
			}
			
			// Other errors, don't retry
			break
		}
		
		if processErr != nil {
			a.app.showError("处理文件失败: " + processErr.Error())
			return
		}

		// Add to attachments
		a.addAttachment(attachment)
	}, a.app.window)
}

// addAttachment adds an attachment to the list
func (a *FileUploadArea) addAttachment(att *llm.Attachment) {
	a.attachments = append(a.attachments, att)
	a.Refresh()
	
	if a.onChange != nil {
		a.onChange(a.attachments)
	}
}

// removeAttachment removes an attachment from the list
func (a *FileUploadArea) removeAttachment(index int) {
	if index < 0 || index >= len(a.attachments) {
		return
	}
	
	a.attachments = append(a.attachments[:index], a.attachments[index+1:]...)
	a.Refresh()
	
	if a.onChange != nil {
		a.onChange(a.attachments)
	}
}

// updateAttachmentsDisplay updates the attachments display
func (a *FileUploadArea) updateAttachmentsDisplay(container *fyne.Container) {
	container.Objects = nil
	
	for i, att := range a.attachments {
		index := i // Capture for closure
		widget := NewFileAttachmentWidget(att, func() {
			a.removeAttachment(index)
		})
		container.Add(widget)
	}
}

// Clear clears all attachments
func (a *FileUploadArea) Clear() {
	a.attachments = make([]*llm.Attachment, 0)
	a.Refresh()
	
	if a.onChange != nil {
		a.onChange(a.attachments)
	}
}

// GetAttachments returns the current attachments
func (a *FileUploadArea) GetAttachments() []*llm.Attachment {
	return a.attachments
}

// Refresh refreshes the widget
func (a *FileUploadArea) Refresh() {
	if a.container != nil && len(a.container.Objects) > 1 {
		attachmentsContainer := a.container.Objects[1].(*fyne.Container)
		a.updateAttachmentsDisplay(attachmentsContainer)
		a.container.Refresh()
	}
	a.BaseWidget.Refresh()
}

// HandleClipboardPaste handles clipboard paste events
// Note: This handles file paths and images from clipboard
func (a *FileUploadArea) HandleClipboardPaste() {
	a.app.logger.Info("=== HandleClipboardPaste called ===")
	
	// First, try to get image from clipboard (for screenshots)
	img, err := getClipboardImage()
	if err != nil {
		a.app.logger.Warn("Error reading clipboard image: %v", err)
	}
	
	if img != nil {
		a.app.logger.Info("Found image in clipboard (screenshot)")
		
		// Process image in a goroutine to avoid blocking UI
		go func() {
			// Generate filename with timestamp
			filename := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
			
			attachment, err := a.handler.ProcessImageData(img, filename)
			if err != nil {
				a.app.logger.Warn("Failed to process clipboard image: %v", err)
				fyne.Do(func() {
					a.app.showError("无法处理截图: " + err.Error())
				})
				return
			}
			
			// Add attachment on UI thread
			fyne.Do(func() {
				a.addAttachment(attachment)
				a.app.logger.Info("Clipboard image added successfully: %s", attachment.Filename)
			})
		}()
		return
	}
	
	// Second, try to get files using Windows API (for copied files in Explorer)
	files, err := getClipboardFiles()
	if err != nil {
		a.app.logger.Warn("Error reading clipboard files: %v", err)
	}
	
	if len(files) > 0 {
		a.app.logger.Info("Found %d file(s) in clipboard via Windows API", len(files))
		
		// Process each file
		for _, filePath := range files {
			a.app.logger.Info("Processing clipboard file: %s", filePath)
			
			// Process file in a goroutine to avoid blocking UI
			go func(path string) {
				attachment, err := a.handler.ProcessFile(path)
				if err != nil {
					a.app.logger.Warn("Failed to process clipboard file: %v", err)
					fyne.Do(func() {
						a.app.showError("无法处理文件: " + err.Error())
					})
					return
				}
				
				// Add attachment on UI thread
				fyne.Do(func() {
					a.addAttachment(attachment)
					a.app.logger.Info("Clipboard file added successfully: %s", attachment.Filename)
				})
			}(filePath)
		}
		return
	}
	
	// Fallback: try to get text content (for file paths as text)
	clipboard := a.app.window.Clipboard()
	content := clipboard.Content()
	
	a.app.logger.Info("Clipboard text content length: %d", len(content))
	
	if content == "" {
		a.app.logger.Info("Clipboard is empty, skipping")
		return
	}
	
	// Log first 200 chars for debugging
	logContent := content
	if len(logContent) > 200 {
		logContent = logContent[:200] + "..."
	}
	a.app.logger.Info("Clipboard text content: [%s]", logContent)
	
	// Try to parse as file path or URI
	var filePath string
	
	if strings.HasPrefix(content, "file://") {
		// Parse file URI
		a.app.logger.Info("Detected file:// URI")
		uri, err := storage.ParseURI(content)
		if err != nil {
			a.app.logger.Warn("Failed to parse clipboard URI: %v", err)
			return
		}
		filePath = uri.Path()
		a.app.logger.Info("Parsed URI to path: %s", filePath)
	} else if strings.Contains(content, ":\\") || strings.HasPrefix(content, "\\\\") {
		// Windows file path - clean it up
		a.app.logger.Info("Detected Windows file path in text")
		filePath = strings.TrimSpace(content)
		// Remove quotes if present
		filePath = strings.Trim(filePath, "\"")
		// Handle multiple lines (first line only)
		if idx := strings.Index(filePath, "\n"); idx > 0 {
			filePath = filePath[:idx]
			filePath = strings.TrimSpace(filePath)
		}
		// Handle carriage return
		if idx := strings.Index(filePath, "\r"); idx > 0 {
			filePath = filePath[:idx]
			filePath = strings.TrimSpace(filePath)
		}
		a.app.logger.Info("Cleaned file path: %s", filePath)
	} else {
		// Not a file path, allow normal text paste
		a.app.logger.Info("Clipboard content is not a file path, allowing text paste")
		return
	}
	
	// If we have a file path, try to process it
	if filePath != "" {
		a.app.logger.Info("Attempting to process clipboard file from text: %s", filePath)
		
		// Process file in a goroutine to avoid blocking UI
		go func() {
			attachment, err := a.handler.ProcessFile(filePath)
			if err != nil {
				a.app.logger.Warn("Failed to process clipboard file: %v", err)
				fyne.Do(func() {
					a.app.showError("无法处理文件: " + err.Error())
				})
				return
			}
			
			// Add attachment on UI thread
			fyne.Do(func() {
				a.addAttachment(attachment)
				a.app.logger.Info("Clipboard file added successfully: %s", attachment.Filename)
			})
		}()
	} else {
		a.app.logger.Info("File path is empty after processing")
	}
}

// showAttachmentPreview shows a preview dialog for an attachment
func showAttachmentPreview(app *App, att *llm.Attachment) {
	var content fyne.CanvasObject

	if att.Type == "image" {
		// Show image preview
		img := canvas.NewImageFromResource(fyne.NewStaticResource(
			att.Filename,
			att.Data,
		))
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(600, 400))
		content = img
	} else {
		// Show text content
		textContent := utils.GetTextContent(att)
		if len(textContent) > 10000 {
			textContent = textContent[:10000] + "\n\n... (truncated)"
		}
		
		entry := widget.NewMultiLineEntry()
		entry.SetText(textContent)
		entry.Wrapping = fyne.TextWrapWord
		entry.Disable()
		
		scroll := container.NewScroll(entry)
		scroll.SetMinSize(fyne.NewSize(600, 400))
		content = scroll
	}

	// Create dialog
	title := fmt.Sprintf("预览: %s", filepath.Base(att.Filename))
	d := dialog.NewCustom(title, "关闭", content, app.window)
	d.Resize(fyne.NewSize(700, 500))
	d.Show()
}
