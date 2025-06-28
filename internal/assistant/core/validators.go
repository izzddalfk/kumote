// internal/assistant/core/validators.go
package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Command validation

// ValidateCommand validates a command structure
func ValidateCommand(cmd Command) error {
	if cmd.ID == "" {
		return NewValidationError("id", "command ID cannot be empty")
	}

	if cmd.UserID == 0 {
		return NewValidationError("user_id", "user ID cannot be zero")
	}

	if cmd.Text == "" && cmd.AudioFileID == "" {
		return NewValidationError("content", "command must have either text or audio content")
	}

	if cmd.Text != "" {
		if len(cmd.Text) > TelegramMaxMessageLength {
			return NewValidationError("text", fmt.Sprintf("text length exceeds maximum of %d characters", TelegramMaxMessageLength))
		}

		if !utf8.ValidString(cmd.Text) {
			return NewValidationError("text", "text contains invalid UTF-8 characters")
		}
	}

	if cmd.Timestamp.IsZero() {
		return NewValidationError("timestamp", "timestamp cannot be zero")
	}

	return nil
}

// ValidateQuery validates a user query
func ValidateQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return ErrEmptyQuery
	}

	if len(query) > TelegramMaxMessageLength {
		return NewValidationError("query", fmt.Sprintf("query length exceeds maximum of %d characters", TelegramMaxMessageLength))
	}

	if !utf8.ValidString(query) {
		return NewValidationError("query", "query contains invalid UTF-8 characters")
	}

	// Check for potentially dangerous commands
	if containsDangerousCommand(query) {
		return NewValidationError("query", "query contains potentially dangerous commands")
	}

	return nil
}

// Project validation

// ValidateProject validates a project structure
func ValidateProject(project Project) error {
	if project.Name == "" {
		return NewValidationError("name", "project name cannot be empty")
	}

	if project.Path == "" {
		return NewValidationError("path", "project path cannot be empty")
	}

	if !filepath.IsAbs(project.Path) {
		return NewValidationError("path", "project path must be absolute")
	}

	if project.Type == "" {
		return NewValidationError("type", "project type cannot be empty")
	}

	if !isValidProjectType(project.Type) {
		return NewValidationError("type", fmt.Sprintf("invalid project type: %s", project.Type))
	}

	if !isValidProjectStatus(project.Status) {
		return NewValidationError("status", fmt.Sprintf("invalid project status: %s", project.Status))
	}

	// Validate shortcuts
	for _, shortcut := range project.Shortcuts {
		if err := validateShortcut(shortcut); err != nil {
			return fmt.Errorf("invalid shortcut '%s': %w", shortcut, err)
		}
	}

	return nil
}

// ValidateProjectIndex validates a project index structure
func ValidateProjectIndex(index ProjectIndex) error {
	if index.ScanPath == "" {
		return NewValidationError("scan_path", "scan path cannot be empty")
	}

	if !filepath.IsAbs(index.ScanPath) {
		return NewValidationError("scan_path", "scan path must be absolute")
	}

	if index.UpdatedAt.IsZero() {
		return NewValidationError("updated_at", "updated_at cannot be zero")
	}

	if index.TotalCount != len(index.Projects) {
		return NewValidationError("total_count", "total_count does not match actual project count")
	}

	// Validate each project
	for i, project := range index.Projects {
		if err := ValidateProject(project); err != nil {
			return fmt.Errorf("invalid project at index %d: %w", i, err)
		}
	}

	// Validate shortcuts mapping
	for shortcut, projectName := range index.Shortcuts {
		if err := validateShortcut(shortcut); err != nil {
			return fmt.Errorf("invalid shortcut '%s': %w", shortcut, err)
		}

		found := false
		for _, project := range index.Projects {
			if project.Name == projectName {
				found = true
				break
			}
		}

		if !found {
			return NewValidationError("shortcuts", fmt.Sprintf("shortcut '%s' references non-existent project '%s'", shortcut, projectName))
		}
	}

	return nil
}

// User validation

// ValidateUser validates a user structure
func ValidateUser(user User) error {
	if user.ID == 0 {
		return NewValidationError("id", "user ID cannot be zero")
	}

	if user.FirstName == "" {
		return NewValidationError("first_name", "first name cannot be empty")
	}

	if !utf8.ValidString(user.FirstName) {
		return NewValidationError("first_name", "first name contains invalid UTF-8 characters")
	}

	if user.LastName != "" && !utf8.ValidString(user.LastName) {
		return NewValidationError("last_name", "last name contains invalid UTF-8 characters")
	}

	if user.Username != "" {
		if err := validateUsername(user.Username); err != nil {
			return fmt.Errorf("invalid username: %w", err)
		}
	}

	return nil
}

// Configuration validation

// ValidateScanConfig validates a scan configuration
func ValidateScanConfig(config ScanConfig) error {
	if config.BasePath == "" {
		return NewValidationError("base_path", "base path cannot be empty")
	}

	if !filepath.IsAbs(config.BasePath) {
		return NewValidationError("base_path", "base path must be absolute")
	}

	if config.MaxDepth < 1 {
		return NewValidationError("max_depth", "max depth must be at least 1")
	}

	if config.MaxDepth > 10 {
		return NewValidationError("max_depth", "max depth cannot exceed 10")
	}

	if config.MinProjectSize < 0 {
		return NewValidationError("min_project_size", "min project size cannot be negative")
	}

	if len(config.Indicators) == 0 {
		return NewValidationError("indicators", "at least one indicator must be specified")
	}

	// Validate shortcuts
	for shortcut, projectName := range config.Shortcuts {
		if err := validateShortcut(shortcut); err != nil {
			return fmt.Errorf("invalid shortcut '%s': %w", shortcut, err)
		}

		if projectName == "" {
			return NewValidationError("shortcuts", fmt.Sprintf("shortcut '%s' has empty project name", shortcut))
		}
	}

	// Validate cron schedule if provided
	if config.UpdateSchedule != "" {
		if err := validateCronSchedule(config.UpdateSchedule); err != nil {
			return fmt.Errorf("invalid update schedule: %w", err)
		}
	}

	return nil
}

// File validation

// ValidateFilePath validates a file path for security
func ValidateFilePath(path string) error {
	if path == "" {
		return NewValidationError("path", "file path cannot be empty")
	}

	// Prevent path traversal attacks
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return NewValidationError("path", "path traversal not allowed")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return NewValidationError("path", "null bytes not allowed in path")
	}

	// Validate UTF-8
	if !utf8.ValidString(path) {
		return NewValidationError("path", "path contains invalid UTF-8 characters")
	}

	return nil
}

// ValidateFileContent validates file content
func ValidateFileContent(content FileContent) error {
	if err := ValidateFilePath(content.Path); err != nil {
		return err
	}

	if content.Name == "" {
		return NewValidationError("name", "file name cannot be empty")
	}

	if content.Size < 0 {
		return NewValidationError("size", "file size cannot be negative")
	}

	if content.Size > MaxFileSize {
		return NewValidationError("size", fmt.Sprintf("file size exceeds maximum of %d bytes", MaxFileSize))
	}

	if content.ModifiedAt.IsZero() {
		return NewValidationError("modified_at", "modified_at cannot be zero")
	}

	return nil
}

// Execution context validation

// ValidateExecutionContext validates an execution context
func ValidateExecutionContext(ctx ExecutionContext) error {
	if ctx.UserID == 0 {
		return NewValidationError("user_id", "user ID cannot be zero")
	}

	if ctx.WorkingDir != "" {
		if err := ValidateFilePath(ctx.WorkingDir); err != nil {
			return fmt.Errorf("invalid working directory: %w", err)
		}
	}

	if ctx.ProjectPath != "" {
		if err := ValidateFilePath(ctx.ProjectPath); err != nil {
			return fmt.Errorf("invalid project path: %w", err)
		}
	}

	if ctx.Timeout <= 0 {
		return NewValidationError("timeout", "timeout must be positive")
	}

	if ctx.Timeout > LongCommandTimeout {
		return NewValidationError("timeout", fmt.Sprintf("timeout cannot exceed %v", LongCommandTimeout))
	}

	return nil
}

// Helper validation functions

func isValidProjectType(projectType ProjectType) bool {
	switch projectType {
	case ProjectTypeGo, ProjectTypeNodeJS, ProjectTypeVue, ProjectTypePython, ProjectTypeDocumentation, ProjectTypeUnknown:
		return true
	default:
		return false
	}
}

func isValidProjectStatus(status ProjectStatus) bool {
	switch status {
	case ProjectStatusActive, ProjectStatusMaintenance, ProjectStatusArchived, ProjectStatusUnknown:
		return true
	default:
		return false
	}
}

func validateShortcut(shortcut string) error {
	if shortcut == "" {
		return NewValidationError("shortcut", "shortcut cannot be empty")
	}

	if len(shortcut) > 20 {
		return NewValidationError("shortcut", "shortcut cannot exceed 20 characters")
	}

	// Only allow alphanumeric characters and underscores
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", shortcut)
	if !matched {
		return NewValidationError("shortcut", "shortcut can only contain alphanumeric characters and underscores")
	}

	return nil
}

func validateUsername(username string) error {
	if len(username) < 5 || len(username) > 32 {
		return NewValidationError("username", "username must be between 5 and 32 characters")
	}

	// Telegram username pattern
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", username)
	if !matched {
		return NewValidationError("username", "username can only contain alphanumeric characters and underscores")
	}

	return nil
}

func validateCronSchedule(schedule string) error {
	// Basic cron validation - should be enhanced with a proper cron parser
	parts := strings.Fields(schedule)
	if len(parts) != 5 && len(parts) != 6 {
		return NewValidationError("schedule", "cron schedule must have 5 or 6 fields")
	}

	return nil
}

func containsDangerousCommand(query string) bool {
	lowerQuery := strings.ToLower(query)

	for cmd := range DangerousCommands {
		if strings.Contains(lowerQuery, cmd) {
			return true
		}
	}

	// Check for shell operators that could be dangerous
	dangerousPatterns := []string{
		"rm -rf", "sudo", "chmod +x", "curl", "wget",
		">&", ">>", "$(", "`", ";", "&&", "||",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return true
		}
	}

	return false
}

// Rate limiting validation

// ValidateRateLimit validates rate limiting parameters
func ValidateRateLimit(limit int, window time.Duration) error {
	if limit <= 0 {
		return NewValidationError("limit", "rate limit must be positive")
	}

	if limit > 1000 {
		return NewValidationError("limit", "rate limit cannot exceed 1000")
	}

	if window <= 0 {
		return NewValidationError("window", "rate limit window must be positive")
	}

	if window > 24*time.Hour {
		return NewValidationError("window", "rate limit window cannot exceed 24 hours")
	}

	return nil
}

// Audio validation

// ValidateAudioFile validates audio file parameters
func ValidateAudioFile(fileID string, format string, size int64, duration time.Duration) error {
	if fileID == "" {
		return NewValidationError("file_id", "audio file ID cannot be empty")
	}

	if size <= 0 {
		return NewValidationError("size", "audio file size must be positive")
	}

	if size > MaxAudioFileSize {
		return NewValidationError("size", fmt.Sprintf("audio file size exceeds maximum of %d bytes", MaxAudioFileSize))
	}

	if duration <= 0 {
		return NewValidationError("duration", "audio duration must be positive")
	}

	if duration > MaxAudioDuration {
		return NewValidationError("duration", fmt.Sprintf("audio duration exceeds maximum of %v", MaxAudioDuration))
	}

	validFormats := map[string]bool{
		AudioFormatOGG: true,
		AudioFormatMP3: true,
		AudioFormatWAV: true,
		AudioFormatM4A: true,
	}

	if !validFormats[format] {
		return NewValidationError("format", fmt.Sprintf("unsupported audio format: %s", format))
	}

	return nil
}
