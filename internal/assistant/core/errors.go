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

	// Project related errors
	ErrProjectNotFound     = errors.New("project not found")
	ErrProjectScanFailed   = errors.New("project scan failed")
	ErrProjectIndexCorrupt = errors.New("project index is corrupt")
	ErrInvalidProjectPath  = errors.New("invalid project path")

	// Command related errors
	ErrInvalidCommand  = errors.New("invalid command")
	ErrCommandTimeout  = errors.New("command execution timeout")
	ErrCommandFailed   = errors.New("command execution failed")
	ErrEmptyQuery      = errors.New("empty query provided")
	ErrAmbiguousQuery  = errors.New("ambiguous query requires clarification")
	ErrCommandNotFound = errors.New("command not found")

	// File related errors
	ErrFileNotFound        = errors.New("file not found")
	ErrFileAccessDenied    = errors.New("file access denied")
	ErrInvalidFilePath     = errors.New("invalid file path")
	ErrFileOperationFailed = errors.New("file operation failed")

	// External service errors
	ErrClaudeCodeUnavailable    = errors.New("claude code cli is unavailable")
	ErrTelegramAPIError         = errors.New("telegram api error")
	ErrAudioTranscriptionFailed = errors.New("audio transcription failed")

	// Configuration errors
	ErrInvalidConfiguration  = errors.New("invalid configuration")
	ErrConfigurationNotFound = errors.New("configuration not found")
)

// Error types for better error handling

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

type AuthorizationError struct {
	UserID int64
	Action string
	Reason string
}

func (e AuthorizationError) Error() string {
	return fmt.Sprintf("user %d not authorized for action '%s': %s", e.UserID, e.Action, e.Reason)
}

type ProjectError struct {
	ProjectName string
	Operation   string
	Cause       error
}

func (e ProjectError) Error() string {
	return fmt.Sprintf("project '%s' error during '%s': %v", e.ProjectName, e.Operation, e.Cause)
}

func (e ProjectError) Unwrap() error {
	return e.Cause
}

type ExecutionError struct {
	Command string
	Output  string
	Cause   error
}

func (e ExecutionError) Error() string {
	return fmt.Sprintf("execution error for command '%s': %v", e.Command, e.Cause)
}

func (e ExecutionError) Unwrap() error {
	return e.Cause
}

type ExternalServiceError struct {
	Service string
	Cause   error
}

func (e ExternalServiceError) Error() string {
	return fmt.Sprintf("external service '%s' error: %v", e.Service, e.Cause)
}

func (e ExternalServiceError) Unwrap() error {
	return e.Cause
}

// Helper functions for creating specific errors

func NewValidationError(field, message string) error {
	return ValidationError{Field: field, Message: message}
}

func NewAuthorizationError(userID int64, action, reason string) error {
	return AuthorizationError{UserID: userID, Action: action, Reason: reason}
}

func NewProjectError(projectName, operation string, cause error) error {
	return ProjectError{ProjectName: projectName, Operation: operation, Cause: cause}
}

func NewExecutionError(command, output string, cause error) error {
	return ExecutionError{Command: command, Output: output, Cause: cause}
}

func NewExternalServiceError(service string, cause error) error {
	return ExternalServiceError{Service: service, Cause: cause}
}

// Error checking helpers

func IsUserError(err error) bool {
	return errors.Is(err, ErrUserNotFound) ||
		errors.Is(err, ErrUserNotAuthorized) ||
		errors.Is(err, ErrUserBlocked) ||
		errors.Is(err, ErrRateLimitExceeded)
}

func IsProjectError(err error) bool {
	return errors.Is(err, ErrProjectNotFound) ||
		errors.Is(err, ErrProjectScanFailed) ||
		errors.Is(err, ErrProjectIndexCorrupt) ||
		errors.Is(err, ErrInvalidProjectPath)
}

func IsCommandError(err error) bool {
	return errors.Is(err, ErrInvalidCommand) ||
		errors.Is(err, ErrCommandTimeout) ||
		errors.Is(err, ErrCommandFailed) ||
		errors.Is(err, ErrEmptyQuery) ||
		errors.Is(err, ErrAmbiguousQuery)
}

func IsFileError(err error) bool {
	return errors.Is(err, ErrFileNotFound) ||
		errors.Is(err, ErrFileAccessDenied) ||
		errors.Is(err, ErrInvalidFilePath) ||
		errors.Is(err, ErrFileOperationFailed)
}

func IsExternalServiceError(err error) bool {
	return errors.Is(err, ErrClaudeCodeUnavailable) ||
		errors.Is(err, ErrTelegramAPIError) ||
		errors.Is(err, ErrAudioTranscriptionFailed)
}

func IsConfigurationError(err error) bool {
	return errors.Is(err, ErrInvalidConfiguration) ||
		errors.Is(err, ErrConfigurationNotFound)
}
