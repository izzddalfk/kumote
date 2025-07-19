package core

import (
	"time"
)

// User represents a Telegram user who can interact with the assistant
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	IsAllowed bool   `json:"is_allowed"`
}

// Command represents a user command that needs to be processed
type Command struct {
	ID          string     `json:"id"`
	UserID      int64      `json:"user_id"`
	Text        string     `json:"text"`
	Timestamp   time.Time  `json:"timestamp"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	SessionID   *string    `json:"session_id,omitempty"` // Optional session ID for stateful interactions. Only supported by Claude Code.
}

// QueryResult represents the result of processing a user query
type QueryResult struct {
	Success  bool           `json:"success"`
	Response string         `json:"response"`
	Error    string         `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ExecutionContext provides context for command execution
type ExecutionContext struct {
	UserID      int64             `json:"user_id"`
	WorkingDir  string            `json:"working_dir"`
	Environment map[string]string `json:"environment"`
	Timeout     time.Duration     `json:"timeout"`
}

// CommandMetrics represents metrics for command execution
type CommandMetrics struct {
	CommandID     string        `json:"command_id"`
	UserID        int64         `json:"user_id"`
	ExecutionTime time.Duration `json:"execution_time"`
	Success       bool          `json:"success"`
	ProjectUsed   string        `json:"project_used,omitempty"`
	ErrorType     string        `json:"error_type,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

type TelegramTextMessageInput struct {
	ChatID  int64
	Message string
}

type AgentCommandInput struct {
	Prompt           string
	ExecutionContext ExecutionContext
	SessionID        *string // Optional session ID for stateful interactions. Only supported by Claude Code.
}
