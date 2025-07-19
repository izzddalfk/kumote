package core

import (
	"context"
)

// Primary Ports (APIs that drive our application)

// AssistantService defines the main business logic interface
type AssistantService interface {
	// ProcessCommand processes a user command and returns the result
	ProcessCommand(ctx context.Context, cmd Command) (*QueryResult, error)
}

// Secondary Ports (SPIs that are driven by our application)

// Agent defines interface for interacting with AI-powered code execution tools
type Agent interface {
	// ExecuteCommand runs an AI code command and returns the result
	ExecuteCommand(ctx context.Context, input AgentCommandInput) (*QueryResult, error)

	// IsAvailable checks if AI code executor is available and working
	IsAvailable(ctx context.Context) bool
}

type TelegramStorage interface {
	SendTextMessage(ctx context.Context, input TelegramTextMessageInput) error
}

// UserRepository defines interface for managing user data
type UserRepository interface {
	// GetUser retrieves user by ID
	GetUser(ctx context.Context, userID int64) (*User, error)

	// IsUserAllowed checks if user is in the allowed list
	IsUserAllowed(ctx context.Context, userID int64) bool
}

type ProjectScanner interface {
	GetProjectDirectory(query string) (string, error)
}

// MetricsCollector defines interface for collecting usage metrics
type MetricsCollector interface {
	// RecordCommandExecution records metrics for command execution
	RecordCommandExecution(ctx context.Context, metrics CommandMetrics) error
}

// RateLimiter defines interface for rate limiting
type RateLimiter interface {
	// IsAllowed checks if request is within rate limit
	IsAllowed(ctx context.Context, userID int64) bool

	// RecordRequest records a request for rate limiting
	RecordRequest(ctx context.Context, userID int64) error
}
