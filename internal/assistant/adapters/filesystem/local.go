package filesystem

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// FileSystem implements safe local file system operations
type FileSystem struct {
	basePath    string
	allowedDirs []string
	logger      *slog.Logger
	config      FileSystemConfig
}

// FileSystemConfig holds configuration for file system operations
type FileSystemConfig struct {
	MaxFileSize       int64         // Maximum file size in bytes
	MaxDirDepth       int           // Maximum directory traversal depth
	AllowedExtensions []string      // Allowed file extensions (empty = all allowed)
	ReadOnly          bool          // If true, no write operations allowed
	FollowSymlinks    bool          // Whether to follow symbolic links
	Timeout           time.Duration // Operation timeout
}

// NewFileSystem creates a new file system adapter
func NewFileSystem(basePath string, allowedDirs []string, logger *slog.Logger) (*FileSystem, error) {
	// Validate base path
	if basePath == "" {
		return nil, fmt.Errorf("base path cannot be empty")
	}

	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base path: %w", err)
	}

	// Validate allowed directories
	absAllowedDirs := make([]string, len(allowedDirs))
	for i, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve allowed directory %s: %w", dir, err)
		}
		absAllowedDirs[i] = absDir
	}

	config := FileSystemConfig{
		MaxFileSize:       core.MaxFileSize,
		MaxDirDepth:       10,
		AllowedExtensions: []string{}, // Empty means all allowed
		ReadOnly:          false,
		FollowSymlinks:    false,
		Timeout:           30 * time.Second,
	}

	fs := &FileSystem{
		basePath:    absBasePath,
		allowedDirs: absAllowedDirs,
		logger:      logger,
		config:      config,
	}

	logger.InfoContext(context.Background(), "File system adapter initialized",
		"base_path", absBasePath,
		"allowed_dirs", len(absAllowedDirs),
		"max_file_size", config.MaxFileSize,
	)

	return fs, nil
}

// SetConfig updates the file system configuration
func (fs *FileSystem) SetConfig(config FileSystemConfig) {
	fs.config = config
	fs.logger.InfoContext(context.Background(), "File system configuration updated",
		"max_file_size", config.MaxFileSize,
		"max_dir_depth", config.MaxDirDepth,
		"read_only", config.ReadOnly,
	)
}

// Exists checks if file or directory exists
func (fs *FileSystem) Exists(ctx context.Context, path string) bool {
	// First check if context is canceled
	if err := ctx.Err(); err != nil {
		return false // Return false on context cancellation
	}

	cleanPath, err := fs.validateAndCleanPath(path)
	if err != nil {
		fs.logger.WarnContext(ctx, "Invalid path in Exists check",
			"path", path,
			"error", err.Error(),
		)
		return false
	}

	fs.logger.DebugContext(ctx, "Checking if path exists",
		"path", cleanPath,
	)

	_, err = os.Stat(cleanPath)
	exists := err == nil

	fs.logger.DebugContext(ctx, "Path existence check completed",
		"path", cleanPath,
		"exists", exists,
	)

	return exists
}

// ReadFile reads file content
func (fs *FileSystem) ReadFile(ctx context.Context, path string) ([]byte, error) {
	// First check if context is canceled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cleanPath, err := fs.validateAndCleanPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	fs.logger.DebugContext(ctx, "Reading file",
		"path", cleanPath,
	)

	// Check if file exists and get info
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a directory
	if info.IsDir() {
		return nil, core.NewValidationError("path", "path is a directory, not a file")
	}

	// Check file size
	if info.Size() > fs.config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", info.Size(), fs.config.MaxFileSize)
	}

	// Check file extension if restrictions are configured
	if err := fs.validateFileExtension(cleanPath); err != nil {
		return nil, err
	}

	// Read file with timeout
	done := make(chan struct {
		data []byte
		err  error
	}, 1)

	go func() {
		data, err := os.ReadFile(cleanPath)
		done <- struct {
			data []byte
			err  error
		}{data, err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			fs.logger.ErrorContext(ctx, "Failed to read file",
				"path", cleanPath,
				"error", result.err.Error(),
			)
			return nil, fmt.Errorf("failed to read file: %w", result.err)
		}

		fs.logger.InfoContext(ctx, "File read successfully",
			"path", cleanPath,
			"size", len(result.data),
		)

		return result.data, nil

	case <-time.After(fs.config.Timeout):
		return nil, fmt.Errorf("file read timeout after %v", fs.config.Timeout)

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WriteFile writes content to file
func (fs *FileSystem) WriteFile(ctx context.Context, path string, content []byte) error {
	// First check if context is canceled
	if err := ctx.Err(); err != nil {
		return err
	}

	if fs.config.ReadOnly {
		return fmt.Errorf("file system is in read-only mode")
	}

	cleanPath, err := fs.validateAndCleanPath(path)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	fs.logger.DebugContext(ctx, "Writing file",
		"path", cleanPath,
		"size", len(content),
	)

	// Validate content size
	if int64(len(content)) > fs.config.MaxFileSize {
		return fmt.Errorf("content size %d exceeds maximum allowed size %d", len(content), fs.config.MaxFileSize)
	}

	// Check file extension if restrictions are configured
	if err := fs.validateFileExtension(cleanPath); err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(cleanPath)
	if err := fs.ensureDirectoryExists(ctx, dir); err != nil {
		return fmt.Errorf("failed to ensure directory exists: %w", err)
	}

	// Write file with timeout
	done := make(chan error, 1)

	go func() {
		err := os.WriteFile(cleanPath, content, 0644)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			fs.logger.ErrorContext(ctx, "Failed to write file",
				"path", cleanPath,
				"error", err.Error(),
			)
			return fmt.Errorf("failed to write file: %w", err)
		}

		fs.logger.InfoContext(ctx, "File written successfully",
			"path", cleanPath,
			"size", len(content),
		)

		return nil

	case <-time.After(fs.config.Timeout):
		return fmt.Errorf("file write timeout after %v", fs.config.Timeout)

	case <-ctx.Done():
		return ctx.Err()
	}
}

// ListDir lists directory contents
func (fs *FileSystem) ListDir(ctx context.Context, path string) ([]string, error) {
	// First check if context is canceled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cleanPath, err := fs.validateAndCleanPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid directory path: %w", err)
	}

	fs.logger.DebugContext(ctx, "Listing directory",
		"path", cleanPath,
	)

	// Check if path exists and is directory
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, core.NewValidationError("path", "path is not a directory")
	}

	// Read directory with timeout
	done := make(chan struct {
		entries []string
		err     error
	}, 1)

	go func() {
		entries, err := os.ReadDir(cleanPath)
		if err != nil {
			done <- struct {
				entries []string
				err     error
			}{nil, err}
			return
		}

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			// Skip hidden files unless explicitly allowed
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			names = append(names, entry.Name())

			// Limit number of entries to prevent memory issues
			if len(names) >= core.MaxDirectoryListing {
				break
			}
		}

		done <- struct {
			entries []string
			err     error
		}{names, nil}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			fs.logger.ErrorContext(ctx, "Failed to list directory",
				"path", cleanPath,
				"error", result.err.Error(),
			)
			return nil, fmt.Errorf("failed to list directory: %w", result.err)
		}

		fs.logger.InfoContext(ctx, "Directory listed successfully",
			"path", cleanPath,
			"entries", len(result.entries),
		)

		return result.entries, nil

	case <-time.After(fs.config.Timeout):
		return nil, fmt.Errorf("directory listing timeout after %v", fs.config.Timeout)

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetFileInfo returns file information
func (fs *FileSystem) GetFileInfo(ctx context.Context, path string) (*core.FileContent, error) {
	// First check if context is canceled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cleanPath, err := fs.validateAndCleanPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	fs.logger.DebugContext(ctx, "Getting file info",
		"path", cleanPath,
	)

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Determine language from extension
	language := fs.detectLanguage(cleanPath)

	fileContent := &core.FileContent{
		Path:        cleanPath,
		Name:        info.Name(),
		Content:     "", // Don't read content for info only
		Size:        info.Size(),
		ModifiedAt:  info.ModTime(),
		IsDirectory: info.IsDir(),
		Language:    language,
	}

	fs.logger.DebugContext(ctx, "File info retrieved",
		"path", cleanPath,
		"size", info.Size(),
		"is_directory", info.IsDir(),
	)

	return fileContent, nil
}

// CreateDir creates directory
func (fs *FileSystem) CreateDir(ctx context.Context, path string) error {
	// First check if context is canceled
	if err := ctx.Err(); err != nil {
		return err
	}

	if fs.config.ReadOnly {
		return fmt.Errorf("file system is in read-only mode")
	}

	cleanPath, err := fs.validateAndCleanPath(path)
	if err != nil {
		return fmt.Errorf("invalid directory path: %w", err)
	}

	fs.logger.DebugContext(ctx, "Creating directory",
		"path", cleanPath,
	)

	return fs.ensureDirectoryExists(ctx, cleanPath)
}

// Helper methods

// validateAndCleanPath validates and cleans a file path
func (fs *FileSystem) validateAndCleanPath(path string) (string, error) {
	// Basic path validation
	if err := core.ValidateFilePath(path); err != nil {
		return "", err
	}

	// Clean and resolve path
	cleanPath := filepath.Clean(path)

	// Convert to absolute path
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(fs.basePath, cleanPath)
	}

	// Ensure path is within allowed directories
	if err := fs.validatePathAccess(absPath); err != nil {
		return "", err
	}

	return absPath, nil
}

// validatePathAccess ensures path is within allowed directories
func (fs *FileSystem) validatePathAccess(path string) error {
	// If no allowed directories configured, allow access to base path
	if len(fs.allowedDirs) == 0 {
		if !strings.HasPrefix(path, fs.basePath) {
			return core.NewValidationError("path", "path is outside base directory")
		}
		return nil
	}

	// Check if path is within any allowed directory
	for _, allowedDir := range fs.allowedDirs {
		if strings.HasPrefix(path, allowedDir) {
			return nil
		}
	}

	return core.NewValidationError("path", "path is not within allowed directories")
}

// validateFileExtension validates file extension if restrictions are configured
func (fs *FileSystem) validateFileExtension(path string) error {
	if len(fs.config.AllowedExtensions) == 0 {
		return nil // No restrictions
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, allowedExt := range fs.config.AllowedExtensions {
		if ext == strings.ToLower(allowedExt) {
			return nil
		}
	}

	return fmt.Errorf("file extension %s is not allowed", ext)
}

// ensureDirectoryExists creates directory if it doesn't exist
func (fs *FileSystem) ensureDirectoryExists(ctx context.Context, dirPath string) error {
	info, err := os.Stat(dirPath)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", dirPath)
		}
		return nil // Directory already exists
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check directory: %w", err)
	}

	// Create directory with timeout
	done := make(chan error, 1)

	go func() {
		err := os.MkdirAll(dirPath, 0755)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			fs.logger.ErrorContext(ctx, "Failed to create directory",
				"path", dirPath,
				"error", err.Error(),
			)
			return fmt.Errorf("failed to create directory: %w", err)
		}

		fs.logger.InfoContext(ctx, "Directory created successfully",
			"path", dirPath,
		)

		return nil

	case <-time.After(fs.config.Timeout):
		return fmt.Errorf("directory creation timeout after %v", fs.config.Timeout)

	case <-ctx.Done():
		return ctx.Err()
	}
}

// detectLanguage detects programming language from file extension
func (fs *FileSystem) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	languageMap := map[string]string{
		".go":         "go",
		".js":         "javascript",
		".ts":         "typescript",
		".vue":        "vue",
		".jsx":        "jsx",
		".tsx":        "tsx",
		".py":         "python",
		".java":       "java",
		".c":          "c",
		".cpp":        "cpp",
		".h":          "c",
		".hpp":        "cpp",
		".cs":         "csharp",
		".php":        "php",
		".rb":         "ruby",
		".rs":         "rust",
		".swift":      "swift",
		".kt":         "kotlin",
		".scala":      "scala",
		".r":          "r",
		".m":          "matlab",
		".sh":         "bash",
		".bash":       "bash",
		".zsh":        "zsh",
		".fish":       "fish",
		".ps1":        "powershell",
		".bat":        "batch",
		".cmd":        "batch",
		".html":       "html",
		".htm":        "html",
		".css":        "css",
		".scss":       "scss",
		".sass":       "sass",
		".less":       "less",
		".xml":        "xml",
		".json":       "json",
		".yaml":       "yaml",
		".yml":        "yaml",
		".toml":       "toml",
		".ini":        "ini",
		".conf":       "conf",
		".cfg":        "conf",
		".md":         "markdown",
		".txt":        "text",
		".log":        "log",
		".sql":        "sql",
		".dockerfile": "dockerfile",
		".gitignore":  "gitignore",
		".env":        "env",
	}

	if language, exists := languageMap[ext]; exists {
		return language
	}

	return "text" // Default fallback
}

// GetStats returns file system usage statistics
func (fs *FileSystem) GetStats(ctx context.Context) map[string]interface{} {
	// First check if context is canceled
	if err := ctx.Err(); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	stats := map[string]interface{}{
		"base_path":       fs.basePath,
		"allowed_dirs":    len(fs.allowedDirs),
		"max_file_size":   fs.config.MaxFileSize,
		"max_dir_depth":   fs.config.MaxDirDepth,
		"read_only":       fs.config.ReadOnly,
		"follow_symlinks": fs.config.FollowSymlinks,
		"timeout":         fs.config.Timeout.String(),
	}

	if len(fs.config.AllowedExtensions) > 0 {
		stats["allowed_extensions"] = fs.config.AllowedExtensions
	}

	fs.logger.DebugContext(ctx, "Retrieved file system stats", "stats", stats)

	return stats
}

// Close cleans up resources (no-op for local filesystem)
func (fs *FileSystem) Close() error {
	fs.logger.InfoContext(context.Background(), "File system adapter closed")
	return nil
}
