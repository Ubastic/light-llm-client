//go:build windows

package main

import "syscall"

func init() {
	// Set UTF-8 encoding for Windows console
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	setConsoleOutputCP.Call(uintptr(65001)) // 65001 is UTF-8
}
