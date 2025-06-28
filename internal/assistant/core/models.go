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
	AudioFileID string     `json:"audio_file_id,omitempty"`
	Timestamp   time.Time  `json:"timestamp"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// Project represents a development project in the file system
type Project struct {
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	Type       ProjectType       `json:"type"`
	TechStack  []string          `json:"tech_stack"`
	Purpose    string            `json:"purpose,omitempty"`
	KeyFiles   []string          `json:"key_files"`
	Status     ProjectStatus     `json:"status"`
	LastCommit *time.Time        `json:"last_commit,omitempty"`
	Shortcuts  []string          `json:"shortcuts"`
	Metadata   map[string]string `json:"metadata"`
}

// ProjectType defines the type of project based on detected indicators
type ProjectType string

const (
	ProjectTypeGo            ProjectType = "go"
	ProjectTypeNodeJS        ProjectType = "nodejs"
	ProjectTypeVue           ProjectType = "vue"
	ProjectTypePython        ProjectType = "python"
	ProjectTypeDocumentation ProjectType = "documentation"
	ProjectTypeUnknown       ProjectType = "unknown"
)

// ProjectStatus represents the current status of a project
type ProjectStatus string

const (
	ProjectStatusActive      ProjectStatus = "active"
	ProjectStatusMaintenance ProjectStatus = "maintenance"
	ProjectStatusArchived    ProjectStatus = "archived"
	ProjectStatusUnknown     ProjectStatus = "unknown"
)

// ProjectIndex represents the complete index of all discovered projects
type ProjectIndex struct {
	Projects   []Project         `json:"projects"`
	UpdatedAt  time.Time         `json:"updated_at"`
	TotalCount int               `json:"total_count"`
	ScanPath   string            `json:"scan_path"`
	Shortcuts  map[string]string `json:"shortcuts"` // shortcut -> project name
}

// QueryResult represents the result of processing a user query
type QueryResult struct {
	Success     bool           `json:"success"`
	Response    string         `json:"response"`
	Projects    []Project      `json:"projects,omitempty"`
	Files       []FileContent  `json:"files,omitempty"`
	Error       string         `json:"error,omitempty"`
	Suggestions []string       `json:"suggestions,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// FileContent represents content of a file with metadata
type FileContent struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Content     string    `json:"content"`
	Size        int64     `json:"size"`
	ModifiedAt  time.Time `json:"modified_at"`
	IsDirectory bool      `json:"is_directory"`
	Language    string    `json:"language,omitempty"`
	Project     string    `json:"project,omitempty"`
}

// ExecutionContext provides context for command execution
type ExecutionContext struct {
	UserID      int64             `json:"user_id"`
	ProjectPath string            `json:"project_path,omitempty"`
	WorkingDir  string            `json:"working_dir"`
	Environment map[string]string `json:"environment"`
	Timeout     time.Duration     `json:"timeout"`
}

// ScanConfig represents configuration for project scanning
type ScanConfig struct {
	BasePath       string            `json:"base_path"`
	Indicators     []string          `json:"indicators"`
	ExcludedDirs   []string          `json:"excluded_dirs"`
	MaxDepth       int               `json:"max_depth"`
	MinProjectSize int64             `json:"min_project_size"`
	Shortcuts      map[string]string `json:"shortcuts"`
	UpdateSchedule string            `json:"update_schedule"`
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
