//go:build windows
// +build windows

package ui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"syscall"
	"unsafe"
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	openClipboard              = user32.NewProc("OpenClipboard")
	closeClipboard             = user32.NewProc("CloseClipboard")
	getClipboardData           = user32.NewProc("GetClipboardData")
	isClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")

	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	globalLock   = kernel32.NewProc("GlobalLock")
	globalUnlock = kernel32.NewProc("GlobalUnlock")
	globalSize   = kernel32.NewProc("GlobalSize")

	shell32       = syscall.NewLazyDLL("shell32.dll")
	dragQueryFile = shell32.NewProc("DragQueryFileW")
)

const (
	CF_BITMAP = 2  // Bitmap format
	CF_DIB    = 8  // Device Independent Bitmap
	CF_HDROP  = 15 // File drop format
)

// BITMAPINFOHEADER structure
type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// getClipboardFiles retrieves file paths from Windows clipboard
func getClipboardFiles() ([]string, error) {
	// Open clipboard
	ret, _, _ := openClipboard.Call(0)
	if ret == 0 {
		return nil, nil // Clipboard not available
	}
	defer closeClipboard.Call()

	// Check if file drop format is available
	ret, _, _ = isClipboardFormatAvailable.Call(CF_HDROP)
	if ret == 0 {
		return nil, nil // No files in clipboard
	}

	// Get clipboard data
	hDrop, _, _ := getClipboardData.Call(CF_HDROP)
	if hDrop == 0 {
		return nil, nil
	}

	// Lock memory
	pDrop, _, _ := globalLock.Call(hDrop)
	if pDrop == 0 {
		return nil, nil
	}
	defer globalUnlock.Call(hDrop)

	// Get file count
	fileCount, _, _ := dragQueryFile.Call(pDrop, 0xFFFFFFFF, 0, 0)
	if fileCount == 0 {
		return nil, nil
	}

	// Get file paths
	files := make([]string, 0, fileCount)
	for i := uintptr(0); i < fileCount; i++ {
		// Get required buffer size
		bufSize, _, _ := dragQueryFile.Call(pDrop, i, 0, 0)
		if bufSize == 0 {
			continue
		}

		// Allocate buffer (size + 1 for null terminator)
		buf := make([]uint16, bufSize+1)
		
		// Get file path
		ret, _, _ := dragQueryFile.Call(
			pDrop,
			i,
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)
		
		if ret > 0 {
			files = append(files, syscall.UTF16ToString(buf))
		}
	}

	return files, nil
}

// getClipboardImage retrieves an image from Windows clipboard (DIB format)
func getClipboardImage() (image.Image, error) {
	// Open clipboard
	ret, _, _ := openClipboard.Call(0)
	if ret == 0 {
		return nil, fmt.Errorf("failed to open clipboard")
	}
	defer closeClipboard.Call()

	// Check if DIB format is available (used by screenshot tools)
	ret, _, _ = isClipboardFormatAvailable.Call(CF_DIB)
	if ret == 0 {
		return nil, nil // No image in clipboard
	}

	// Get clipboard data
	hMem, _, _ := getClipboardData.Call(CF_DIB)
	if hMem == 0 {
		return nil, fmt.Errorf("failed to get clipboard data")
	}

	// Lock memory
	pMem, _, _ := globalLock.Call(hMem)
	if pMem == 0 {
		return nil, fmt.Errorf("failed to lock memory")
	}
	defer globalUnlock.Call(hMem)

	// Get size
	size, _, _ := globalSize.Call(hMem)
	if size == 0 {
		return nil, fmt.Errorf("empty clipboard data")
	}

	// Copy data to Go slice
	data := make([]byte, size)
	for i := uintptr(0); i < size; i++ {
		data[i] = *(*byte)(unsafe.Pointer(pMem + i))
	}

	// Parse BITMAPINFOHEADER
	if len(data) < 40 {
		return nil, fmt.Errorf("invalid DIB data: too small")
	}

	var header BITMAPINFOHEADER
	buf := bytes.NewReader(data[:40])
	if err := binary.Read(buf, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read bitmap header: %w", err)
	}

	// Support uncompressed (BI_RGB = 0) and BI_BITFIELDS (3) formats
	// BI_BITFIELDS is commonly used by Windows screenshot tools
	const (
		BI_RGB       = 0
		BI_BITFIELDS = 3
	)

	width := int(header.Width)
	height := int(header.Height)
	if height < 0 {
		height = -height // Top-down DIB
	}
	bitCount := int(header.BitCount)

	if bitCount != 24 && bitCount != 32 {
		return nil, fmt.Errorf("unsupported bit depth: %d (only 24 and 32 supported)", bitCount)
	}

	// Pixel data offset
	pixelDataOffset := 40

	// For BI_BITFIELDS, there are 3 DWORD color masks after the header
	if header.Compression == BI_BITFIELDS {
		pixelDataOffset = 40 + 12 // header + 3 DWORDs (4 bytes each)
	} else if header.Compression != BI_RGB {
		return nil, fmt.Errorf("unsupported compression: %d (only BI_RGB and BI_BITFIELDS supported)", header.Compression)
	}

	// Calculate stride (row size must be multiple of 4)
	stride := ((width*bitCount + 31) / 32) * 4

	if len(data) < pixelDataOffset+stride*height {
		return nil, fmt.Errorf("invalid DIB data: insufficient pixel data")
	}

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// DIB is stored bottom-up by default
	bottomUp := header.Height > 0

	// Read pixel data
	for y := 0; y < height; y++ {
		// Calculate actual y position (flip if bottom-up)
		actualY := y
		if bottomUp {
			actualY = height - 1 - y
		}

		rowOffset := pixelDataOffset + y*stride

		for x := 0; x < width; x++ {
			var r, g, b, a byte

			if bitCount == 24 {
				// 24-bit: BGR format
				pixelOffset := rowOffset + x*3
				b = data[pixelOffset]
				g = data[pixelOffset+1]
				r = data[pixelOffset+2]
				a = 255
			} else if bitCount == 32 {
				// 32-bit: BGRA format
				pixelOffset := rowOffset + x*4
				b = data[pixelOffset]
				g = data[pixelOffset+1]
				r = data[pixelOffset+2]
				a = data[pixelOffset+3]
			}

			img.Set(x, actualY, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return img, nil
}
