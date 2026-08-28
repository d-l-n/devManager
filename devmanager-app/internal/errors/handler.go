package errors

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrorHandler handles error logging and reporting
type ErrorHandler struct {
	logFile    *os.File
	logMutex   sync.Mutex
	errorCount map[ErrorCode]int
}

// NewErrorHandler creates a new error handler
func NewErrorHandler() (*ErrorHandler, error) {
	// Create logs directory if it doesn't exist
	logDir := filepath.Join(os.Getenv("APPDATA"), "devManager", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	logFile := filepath.Join(logDir, "errors.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &ErrorHandler{
		logFile:    file,
		errorCount: make(map[ErrorCode]int),
	}, nil
}

// Handle handles an application error
func (h *ErrorHandler) Handle(err error) {
	if err == nil {
		return
	}

	appErr, isAppErr := IsAppError(err)
	if !isAppErr {
		// Wrap standard errors
		appErr = InternalError("unknown", err)
	}

	// Increment error count
	h.logMutex.Lock()
	h.errorCount[appErr.Code]++
	h.logMutex.Unlock()

	// Log the error
	h.logError(appErr)

	// Log to console for development
	if os.Getenv("DEVMANAGER_DEBUG") == "true" {
		log.Printf("[ERROR] %s", appErr.Error())
		if appErr.Cause != nil {
			log.Printf("[CAUSE] %s", appErr.Cause.Error())
		}
	}
}

// logError writes the error to the log file
func (h *ErrorHandler) logError(err *AppError) {
	h.logMutex.Lock()
	defer h.logMutex.Unlock()

	// Create log entry
	logEntry := map[string]interface{}{
		"timestamp":    err.Timestamp.Format(time.RFC3339),
		"code":         string(err.Code),
		"message":      err.Message,
		"user_message": err.UserMessage,
		"severity":     err.GetSeverity(),
		"details":      err.Details,
		"stack":        err.Stack,
	}

	if err.Cause != nil {
		logEntry["cause"] = err.Cause.Error()
	}

	// Convert to JSON
	jsonData, jsonErr := json.Marshal(logEntry)
	if jsonErr != nil {
		// Fallback to simple logging
		fmt.Fprintf(h.logFile, "[%s] %s: %s\n", 
			err.Timestamp.Format(time.RFC3339), 
			err.Code, 
			err.Message)
		return
	}

	// Write to log file
	fmt.Fprintf(h.logFile, "%s\n", string(jsonData))
	h.logFile.Sync()
}

// GetErrorStats returns error statistics
func (h *ErrorHandler) GetErrorStats() map[string]int {
	h.logMutex.Lock()
	defer h.logMutex.Unlock()

	stats := make(map[string]int)
	for code, count := range h.errorCount {
		stats[string(code)] = count
	}
	return stats
}

// GetRecentErrors returns recent errors from the log file
func (h *ErrorHandler) GetRecentErrors(limit int) ([]*AppError, error) {
	// This is a simplified implementation
	// In a real app, you might want to use a proper log parsing library
	return []*AppError{}, nil
}

// Close closes the error handler
func (h *ErrorHandler) Close() error {
	if h.logFile != nil {
		return h.logFile.Close()
	}
	return nil
}

// Global error handler instance
var globalErrorHandler *ErrorHandler
var errorHandlerOnce sync.Once

// GetErrorHandler returns the global error handler
func GetErrorHandler() *ErrorHandler {
	errorHandlerOnce.Do(func() {
		handler, err := NewErrorHandler()
		if err != nil {
			log.Printf("Failed to create error handler: %v", err)
			return
		}
		globalErrorHandler = handler
	})
	return globalErrorHandler
}

// Handle is a convenience function to handle errors globally
func Handle(err error) {
	if err != nil {
		GetErrorHandler().Handle(err)
	}
}

// HandleWithRecovery recovers from panics and handles them
func HandleWithRecovery() {
	if r := recover(); r != nil {
		var err error
		switch x := r.(type) {
		case string:
			err = fmt.Errorf("panic: %s", x)
		case error:
			err = x
		default:
			err = fmt.Errorf("panic: %v", x)
		}
		
		Handle(InternalError("panic", err))
	}
}

// SafeExecute executes a function and handles any errors/panics
func SafeExecute(fn func() error) (err error) {
	defer HandleWithRecovery()
	
	err = fn()
	if err != nil {
		Handle(err)
	}
	return err
}

// ErrorReporter provides methods for reporting errors to external services
type ErrorReporter struct {
	handler *ErrorHandler
}

// NewErrorReporter creates a new error reporter
func NewErrorReporter() *ErrorReporter {
	return &ErrorReporter{
		handler: GetErrorHandler(),
	}
}

// ReportUserError reports an error that should be shown to the user
func (r *ErrorReporter) ReportUserError(err error, context string) {
	if err == nil {
		return
	}

	appErr, isAppErr := IsAppError(err)
	if !isAppErr {
		appErr = InternalError(context, err)
	}

	// Add context
	appErr.WithDetail("context", context)

	// Handle the error
	r.handler.Handle(appErr)
}

// ReportSilentError reports an error that shouldn't be shown to the user
func (r *ErrorReporter) ReportSilentError(err error, context string) {
	if err == nil {
		return
	}

	appErr := InternalError(context, err)
	appErr.WithDetail("silent", true)
	appErr.WithDetail("context", context)

	r.handler.Handle(appErr)
}

// ReportCriticalError reports a critical error that needs immediate attention
func (r *ErrorReporter) ReportCriticalError(err error, context string) {
	if err == nil {
		return
	}

	appErr := InternalError(context, err)
	appErr.WithDetail("critical", true)
	appErr.WithDetail("context", context)

	r.handler.Handle(appErr)

	// In a real app, you might want to send critical errors to a monitoring service
	log.Printf("[CRITICAL] %s: %v", context, err)
}