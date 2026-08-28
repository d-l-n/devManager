package errors

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorCode represents different types of application errors
type ErrorCode string

const (
	// Project errors
	ErrProjectNotFound     ErrorCode = "PROJECT_NOT_FOUND"
	ErrProjectAlreadyExists ErrorCode = "PROJECT_ALREADY_EXISTS"
	ErrProjectInvalidPath  ErrorCode = "PROJECT_INVALID_PATH"
	ErrProjectLoadFailed   ErrorCode = "PROJECT_LOAD_FAILED"

	// Server errors
	ErrServerStartFailed   ErrorCode = "SERVER_START_FAILED"
	ErrServerStopFailed    ErrorCode = "SERVER_STOP_FAILED"
	ErrServerNotRunning    ErrorCode = "SERVER_NOT_RUNNING"
	ErrServerAlreadyRunning ErrorCode = "SERVER_ALREADY_RUNNING"
	ErrPortConflict        ErrorCode = "PORT_CONFLICT"

	// Process errors
	ErrProcessNotFound     ErrorCode = "PROCESS_NOT_FOUND"
	ErrProcessStartFailed  ErrorCode = "PROCESS_START_FAILED"
	ErrProcessKillFailed   ErrorCode = "PROCESS_KILL_FAILED"

	// Configuration errors
	ErrConfigNotFound      ErrorCode = "CONFIG_NOT_FOUND"
	ErrConfigInvalid       ErrorCode = "CONFIG_INVALID"
	ErrConfigSaveFailed    ErrorCode = "CONFIG_SAVE_FAILED"

	// File system errors
	ErrFileNotFound        ErrorCode = "FILE_NOT_FOUND"
	ErrFileAccessDenied    ErrorCode = "FILE_ACCESS_DENIED"
	ErrFileCorrupted       ErrorCode = "FILE_CORRUPTED"

	// Network errors
	ErrNetworkTimeout      ErrorCode = "NETWORK_TIMEOUT"
	ErrNetworkConnectionFailed ErrorCode = "NETWORK_CONNECTION_FAILED"

	// Validation errors
	ErrValidationFailed    ErrorCode = "VALIDATION_FAILED"
	ErrInvalidInput        ErrorCode = "INVALID_INPUT"

	// Internal errors
	ErrInternal            ErrorCode = "INTERNAL_ERROR"
	ErrDatabase            ErrorCode = "DATABASE_ERROR"
	ErrTimeout             ErrorCode = "TIMEOUT_ERROR"
)

// AppError represents a structured application error
type AppError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	UserMessage string                 `json:"user_message,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Stack       []string               `json:"stack,omitempty"`
	Cause       error                  `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.UserMessage != "" {
		return fmt.Sprintf("[%s] %s", e.Code, e.UserMessage)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Stack:     getStackTrace(),
	}
}

// WithCause adds an underlying cause to the error
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// WithDetails adds additional details to the error
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithUserMessage sets a user-friendly message
func (e *AppError) WithUserMessage(message string) *AppError {
	e.UserMessage = message
	return e
}

// WithDetail adds a single detail to the error
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// IsUserFriendly checks if the error has a user-friendly message
func (e *AppError) IsUserFriendly() bool {
	return e.UserMessage != ""
}

// GetSeverity returns the severity level based on error code
func (e *AppError) GetSeverity() string {
	switch e.Code {
	case ErrProjectNotFound, ErrProjectInvalidPath, ErrFileNotFound:
		return "warning"
	case ErrServerStartFailed, ErrProcessStartFailed, ErrConfigSaveFailed:
		return "error"
	case ErrInternal, ErrDatabase:
		return "critical"
	default:
		return "info"
	}
}

// getStackTrace captures the current stack trace
func getStackTrace() []string {
	var stack []string
	for i := 2; i < 15; i++ { // Skip first 2 frames (getStackTrace and NewAppError)
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		
		// Create a readable stack frame
		frame := fmt.Sprintf("%s:%d %s", 
			shortenFile(file), 
			line, 
			fn.Name())
		stack = append(stack, frame)
	}
	return stack
}

// shortenFile shortens file paths for better readability
func shortenFile(file string) string {
	parts := strings.Split(file, "/")
	if len(parts) > 3 {
		return ".../" + strings.Join(parts[len(parts)-3:], "/")
	}
	return file
}

// Helper functions for common error patterns

// ProjectNotFound creates a project not found error
func ProjectNotFound(projectID string) *AppError {
	return NewAppError(ErrProjectNotFound, "Project not found").
		WithDetail("project_id", projectID).
		WithUserMessage("The requested project could not be found")
}

// ProjectAlreadyExists creates a project already exists error
func ProjectAlreadyExists(name string) *AppError {
	return NewAppError(ErrProjectAlreadyExists, "Project already exists").
		WithDetail("project_name", name).
		WithUserMessage("A project with this name already exists")
}

// ServerStartFailed creates a server start failed error
func ServerStartFailed(projectID string, cause error) *AppError {
	return NewAppError(ErrServerStartFailed, "Failed to start server").
		WithDetail("project_id", projectID).
		WithCause(cause).
		WithUserMessage("Could not start the development server")
}

// PortConflict creates a port conflict error
func PortConflict(port int, projectID string) *AppError {
	return NewAppError(ErrPortConflict, "Port is already in use").
		WithDetails(map[string]interface{}{
			"port":       port,
			"project_id": projectID,
		}).
		WithUserMessage(fmt.Sprintf("Port %d is already in use by another application", port))
}

// ConfigInvalid creates a config invalid error
func ConfigInvalid(field string, value interface{}) *AppError {
	return NewAppError(ErrConfigInvalid, "Invalid configuration").
		WithDetails(map[string]interface{}{
			"field": field,
			"value": value,
		}).
		WithUserMessage("The configuration is invalid. Please check your settings")
}

// ValidationError creates a validation error
func ValidationError(field string, rule string) *AppError {
	return NewAppError(ErrValidationFailed, "Validation failed").
		WithDetails(map[string]interface{}{
			"field": field,
			"rule":  rule,
		}).
		WithUserMessage(fmt.Sprintf("The %s is not valid", field))
}

// InternalError creates an internal error
func InternalError(operation string, cause error) *AppError {
	return NewAppError(ErrInternal, "Internal error occurred").
		WithDetail("operation", operation).
		WithCause(cause).
		WithUserMessage("An unexpected error occurred. Please try again")
}

// NetworkTimeout creates a network timeout error
func NetworkTimeout(operation string, timeout time.Duration) *AppError {
	return NewAppError(ErrNetworkTimeout, "Network operation timed out").
		WithDetails(map[string]interface{}{
			"operation": operation,
			"timeout":   timeout.String(),
		}).
		WithUserMessage("The operation timed out. Please check your connection")
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) (*AppError, bool) {
	if appErr, ok := err.(*AppError); ok {
		return appErr, true
	}
	return nil, false
}

// WrapError wraps a standard error into an AppError
func WrapError(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	
	appErr := NewAppError(code, message).WithCause(err)
	
	// If the original error is already an AppError, preserve some info
	if originalAppErr, ok := err.(*AppError); ok {
		appErr.Details = originalAppErr.Details
		if appErr.UserMessage == "" {
			appErr.UserMessage = originalAppErr.UserMessage
		}
	}
	
	return appErr
}