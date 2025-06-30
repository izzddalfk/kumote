package core

import (
	"context"
	"io"
)

// Primary Ports (APIs that drive our application)

// AssistantService defines the main business logic interface
type AssistantService interface {
	// ProcessCommand processes a user command and returns the result
	ProcessCommand(ctx context.Context, cmd Command) (*QueryResult, error)

	// ProcessAudioCommand processes audio command by converting to text first
	ProcessAudioCommand(ctx context.Context, cmd Command) (*QueryResult, error)

	// GetProjectByShortcut retrieves project information by shortcut
	GetProjectByShortcut(ctx context.Context, shortcut string) (*Project, error)

	// GetProjectByName retrieves project information by name
	GetProjectByName(ctx context.Context, name string) (*Project, error)

	// ListProjects returns all available projects
	ListProjects(ctx context.Context) ([]Project, error)

	// RefreshProjects triggers a manual refresh of project index
	RefreshProjects(ctx context.Context) error

	// GetUserPermissions checks if user is allowed to use the assistant
	GetUserPermissions(ctx context.Context, userID int64) (*User, error)

	// RespondToCommand processes a command result to notify the user about the result
	RespondToCommand(ctx context.Context, userID int64, result *QueryResult) error
}

// Secondary Ports (SPIs that are driven by our application)

// ProjectScanner defines interface for discovering and indexing projects
type ProjectScanner interface {
	// ScanProjects scans the base directory and returns discovered projects
	ScanProjects(ctx context.Context, config ScanConfig) (*ProjectIndex, error)

	// UpdateIndex refreshes the project index
	UpdateIndex(ctx context.Context) (*ProjectIndex, error)

	// GetProjectIndex returns the current project index
	GetProjectIndex(ctx context.Context) (*ProjectIndex, error)

	// SaveProjectIndex persists the project index
	SaveProjectIndex(ctx context.Context, index *ProjectIndex) error

	// LoadProjectIndex loads the persisted project index
	LoadProjectIndex(ctx context.Context) (*ProjectIndex, error)
}

// AICodeExecutor defines interface for interacting with AI-powered code execution tools
type AICodeExecutor interface {
	// ExecuteCommand runs an AI code command and returns the result
	ExecuteCommand(ctx context.Context, command string, execCtx ExecutionContext) (*QueryResult, error)

	// ReadFile reads file content using AI assistance
	ReadFile(ctx context.Context, filePath string, execCtx ExecutionContext) (*FileContent, error)

	// WriteFile writes content to file using AI assistance
	WriteFile(ctx context.Context, filePath, content string, execCtx ExecutionContext) error

	// ListFiles lists files in a directory using AI assistance
	ListFiles(ctx context.Context, dirPath string, execCtx ExecutionContext) ([]FileContent, error)

	// ExecuteGitCommand executes git commands safely
	ExecuteGitCommand(ctx context.Context, gitCmd string, execCtx ExecutionContext) (*QueryResult, error)

	// IsAvailable checks if AI code executor is available and working
	IsAvailable(ctx context.Context) bool
}

// TelegramNotifier defines interface for sending notifications back to Telegram
type TelegramNotifier interface {
	// SendMessage sends a text message to user
	SendMessage(ctx context.Context, userID int64, message string) error

	// SendFile sends a file to user
	SendFile(ctx context.Context, userID int64, file io.Reader, filename string) error

	// SendFormattedMessage sends a message with formatting (markdown/HTML)
	SendFormattedMessage(ctx context.Context, userID int64, message string, parseMode string) error

	// SendConfirmationRequest sends a confirmation request with inline keyboard
	SendConfirmationRequest(ctx context.Context, userID int64, message string, options []string) error
}

// AudioTranscriber defines interface for converting audio to text
type AudioTranscriber interface {
	// TranscribeAudio converts audio file to text
	TranscribeAudio(ctx context.Context, audioData io.Reader) (string, error)

	// IsSupported checks if the audio format is supported
	IsSupported(ctx context.Context, format string) bool
}

// UserRepository defines interface for managing user data
type UserRepository interface {
	// GetUser retrieves user by ID
	GetUser(ctx context.Context, userID int64) (*User, error)

	// SaveUser saves or updates user information
	SaveUser(ctx context.Context, user *User) error

	// IsUserAllowed checks if user is in the allowed list
	IsUserAllowed(ctx context.Context, userID int64) bool

	// GetAllowedUsers returns list of all allowed users
	GetAllowedUsers(ctx context.Context) ([]User, error)
}

// CommandRepository defines interface for managing command history
type CommandRepository interface {
	// SaveCommand saves command to history
	SaveCommand(ctx context.Context, cmd *Command) error

	// GetCommandHistory retrieves command history for user
	GetCommandHistory(ctx context.Context, userID int64, limit int) ([]Command, error)

	// GetCommandByID retrieves specific command by ID
	GetCommandByID(ctx context.Context, commandID string) (*Command, error)
}

// MetricsCollector defines interface for collecting usage metrics
type MetricsCollector interface {
	// RecordCommandExecution records metrics for command execution
	RecordCommandExecution(ctx context.Context, metrics CommandMetrics) error

	// GetUsageStats returns usage statistics
	GetUsageStats(ctx context.Context, userID int64, period string) (map[string]any, error)

	// GetSystemHealth returns system health metrics
	GetSystemHealth(ctx context.Context) (map[string]any, error)
}

// ConfigProvider defines interface for configuration management
type ConfigProvider interface {
	// GetScanConfig returns project scanning configuration
	GetScanConfig(ctx context.Context) (*ScanConfig, error)

	// UpdateScanConfig updates project scanning configuration
	UpdateScanConfig(ctx context.Context, config *ScanConfig) error

	// GetAllowedUsers returns list of allowed user IDs
	GetAllowedUserIDs(ctx context.Context) ([]int64, error)

	// GetRateLimit returns rate limiting configuration
	GetRateLimit(ctx context.Context) (int, error)
}

// FileSystem defines interface for file system operations
type FileSystem interface {
	// Exists checks if file or directory exists
	Exists(ctx context.Context, path string) bool

	// ReadFile reads file content
	ReadFile(ctx context.Context, path string) ([]byte, error)

	// WriteFile writes content to file
	WriteFile(ctx context.Context, path string, content []byte) error

	// ListDir lists directory contents
	ListDir(ctx context.Context, path string) ([]string, error)

	// GetFileInfo returns file information
	GetFileInfo(ctx context.Context, path string) (*FileContent, error)

	// CreateDir creates directory
	CreateDir(ctx context.Context, path string) error
}

// RateLimiter defines interface for rate limiting
type RateLimiter interface {
	// IsAllowed checks if request is within rate limit
	IsAllowed(ctx context.Context, userID int64) bool

	// RecordRequest records a request for rate limiting
	RecordRequest(ctx context.Context, userID int64) error

	// GetRemainingRequests returns remaining requests for user
	GetRemainingRequests(ctx context.Context, userID int64) (int, error)
}
