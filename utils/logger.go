package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Logger provides logging functionality
type Logger struct {
	file   *os.File
	logger *log.Logger
}

// NewLogger creates a new logger
func NewLogger(logPath string) (*Logger, error) {
	// Ensure directory exists
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := log.New(file, "", log.LstdFlags)

	return &Logger{
		file:   file,
		logger: logger,
	}, nil
}

// Close closes the logger
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	msg := fmt.Sprintf("[INFO] "+format, v...)
	l.logger.Println(msg)
	fmt.Println(msg)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	msg := fmt.Sprintf("[ERROR] "+format, v...)
	l.logger.Println(msg)
	fmt.Println(msg)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	msg := fmt.Sprintf("[DEBUG] "+format, v...)
	l.logger.Println(msg)
	fmt.Println(msg)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	msg := fmt.Sprintf("[WARN] "+format, v...)
	l.logger.Println(msg)
	fmt.Println(msg)
}

// GetLogPath returns the default log path
func GetLogPath() string {
	return filepath.Join(".", "logs", fmt.Sprintf("app-%s.log", time.Now().Format("2006-01-02")))
}
