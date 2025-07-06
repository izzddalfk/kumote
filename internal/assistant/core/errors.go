// internal/assistant/core/errors.go
package core

import (
"errors"
"fmt"
)

// Domain specific errors
var (
// User related errors
ErrUserNotFound      = errors.New("user not found")
ErrUserNotAuthorized = errors.New("user not authorized")
ErrUserBlocked       = errors.New("user is blocked")
ErrRateLimitExceeded = errors.New("rate limit exceeded")

// Command related errors
ErrInvalidCommand  = errors.New("invalid command")
ErrCommandTimeout  = errors.New("command execution timeout")
ErrCommandFailed   = errors.New("command execution failed")
ErrEmptyQuery      = errors.New("empty query provided")
ErrCommandNotFound = errors.New("command not found")

// External service errors
ErrClaudeCodeUnavailable = errors.New("claude code cli is unavailable")
)

// Error types for better error handling

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// ServiceError represents an error in the assistant service
type ServiceError struct {
	Code    string
	Message string
	Cause   error
}

func (e ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewServiceError creates a new service error
func NewServiceError(code, message string, cause error) ServiceError {
	return ServiceError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
