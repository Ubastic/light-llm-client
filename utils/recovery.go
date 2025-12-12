package utils

import (
	"fmt"
	"runtime/debug"
)

// RecoverFromPanic recovers from panics and logs them
func RecoverFromPanic(logger *Logger, context string) {
	if r := recover(); r != nil {
		stack := debug.Stack()
		logger.Error("Panic recovered in %s: %v\nStack trace:\n%s", context, r, string(stack))
	}
}

// SafeGo runs a goroutine with panic recovery
func SafeGo(logger *Logger, context string, fn func()) {
	go func() {
		defer RecoverFromPanic(logger, context)
		fn()
	}()
}

// SafeGoWithError runs a goroutine with panic recovery and error handling
func SafeGoWithError(logger *Logger, context string, fn func() error, onError func(error)) {
	go func() {
		defer RecoverFromPanic(logger, context)
		if err := fn(); err != nil {
			logger.Error("Error in %s: %v", context, err)
			if onError != nil {
				onError(err)
			}
		}
	}()
}

// WrapError wraps an error with additional context
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}
