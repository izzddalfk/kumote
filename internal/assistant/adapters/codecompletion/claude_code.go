package codecompletion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// ClaudeExecutor implements the AICodeExecutor interface using Claude CLI
type ClaudeExecutor struct {
	executablePath string
	defaultModel   string
	baseWorkDir    string
	debug          bool
}

// NewClaudeExecutor creates a new instance of ClaudeExecutor
func NewClaudeExecutor(executablePath, defaultModel, baseWorkDir string, debug bool) *ClaudeExecutor {
	if executablePath == "" {
		executablePath = "claude" // Default to "claude" if not specified
	}

	if defaultModel == "" {
		defaultModel = "sonnet" // Default to "sonnet" if not specified
	}

	return &ClaudeExecutor{
		executablePath: executablePath,
		defaultModel:   defaultModel,
		baseWorkDir:    baseWorkDir,
		debug:          debug,
	}
}

// ExecuteCommand runs an AI code command and returns the result
func (c *ClaudeExecutor) ExecuteCommand(ctx context.Context, command string, execCtx core.ExecutionContext) (*core.QueryResult, error) {
	// Construct the command with the working directory context
	promptWithContext := c.buildPromptWithContext(command, execCtx)

	// Execute Claude CLI command
	result, err := c.runClaudeCommand(ctx, promptWithContext, execCtx.WorkingDir, "--output-format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to execute command with Claude: %w", err)
	}

	// Parse JSON response
	var response struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		// If JSON parsing fails, return the raw output
		return &core.QueryResult{
			Success:  true,
			Response: result,
		}, nil
	}

	return &core.QueryResult{
		Success:  true,
		Response: response.Content,
	}, nil
}

// ReadFile reads file content using AI assistance
func (c *ClaudeExecutor) ReadFile(ctx context.Context, filePath string, execCtx core.ExecutionContext) (*core.FileContent, error) {
	// Resolve the absolute path if it's relative
	absPath := filePath
	if !filepath.IsAbs(filePath) && execCtx.WorkingDir != "" {
		absPath = filepath.Join(execCtx.WorkingDir, filePath)
	}

	// Check if file exists first using os.Stat
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("file not found or not accessible: %w", err)
	}

	// Use Claude to read and potentially interpret the file
	prompt := fmt.Sprintf("Read and interpret the file at %s. First show the raw content, then explain what this file does.", absPath)
	result, err := c.runClaudeCommand(ctx, prompt, execCtx.WorkingDir, "--output-format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to read file using Claude: %w", err)
	}

	// Also read the file directly to ensure we have the correct content
	rawContent, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file directly: %w", err)
	}

	// Parse JSON response for explanation
	var response struct {
		Content string `json:"content"`
	}
	// We don't use the explanation from Claude since our model doesn't have that field
	if err := json.Unmarshal([]byte(result), &response); err == nil {
		// Explanation from Claude might be used in the future
		// For now, we just use the file content directly
	}

	// Get file info for metadata
	fileName := filepath.Base(absPath)
	fileSize := info.Size()
	modTime := info.ModTime()

	// Try to determine language from file extension
	language := ""
	ext := strings.ToLower(filepath.Ext(absPath))
	switch ext {
	case ".go":
		language = "Go"
	case ".js", ".jsx":
		language = "JavaScript"
	case ".ts", ".tsx":
		language = "TypeScript"
	case ".py":
		language = "Python"
	case ".java":
		language = "Java"
	case ".c", ".cpp", ".h", ".hpp":
		language = "C/C++"
	case ".md", ".markdown":
		language = "Markdown"
	case ".json":
		language = "JSON"
	case ".yml", ".yaml":
		language = "YAML"
	}

	return &core.FileContent{
		Path:        filePath,
		Name:        fileName,
		Content:     string(rawContent),
		Size:        fileSize,
		ModifiedAt:  modTime,
		IsDirectory: false,
		Language:    language,
		// Note: Project field is left empty as we don't have that context
	}, nil
}

// WriteFile writes content to file using AI assistance
func (c *ClaudeExecutor) WriteFile(ctx context.Context, filePath, content string, execCtx core.ExecutionContext) error {
	// Resolve the absolute path if it's relative
	absPath := filePath
	if !filepath.IsAbs(filePath) && execCtx.WorkingDir != "" {
		absPath = filepath.Join(execCtx.WorkingDir, filePath)
	}

	// Ensure the directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Use Claude to check if the content is safe and appropriate
	prompt := fmt.Sprintf("Review this content before writing to %s. Are there any potential issues? Content:\n%s",
		absPath, content)

	result, err := c.runClaudeCommand(ctx, prompt, execCtx.WorkingDir, "--output-format", "json")
	if err != nil {
		return fmt.Errorf("failed to validate file content using Claude: %w", err)
	}

	// Parse JSON response
	var response struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(result), &response); err == nil {
		// Check if Claude found serious issues (simple heuristic)
		responseLower := strings.ToLower(response.Content)
		if strings.Contains(responseLower, "security") && strings.Contains(responseLower, "issue") ||
			strings.Contains(responseLower, "unsafe") ||
			strings.Contains(responseLower, "malicious") {
			return fmt.Errorf("unsafe content detected: %s", response.Content)
		}
	}

	// Write the file
	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ListFiles lists files in a directory using AI assistance
func (c *ClaudeExecutor) ListFiles(ctx context.Context, dirPath string, execCtx core.ExecutionContext) ([]core.FileContent, error) {
	// Resolve the absolute path if it's relative
	absPath := dirPath
	if !filepath.IsAbs(dirPath) && execCtx.WorkingDir != "" {
		absPath = filepath.Join(execCtx.WorkingDir, dirPath)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("directory not found or not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	// Use Claude to list and describe files
	prompt := fmt.Sprintf("List and briefly describe the files in directory %s. For each file, provide the filename and a short description of what it appears to contain or do.", absPath)
	result, err := c.runClaudeCommand(ctx, prompt, execCtx.WorkingDir, "--output-format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list files using Claude: %w", err)
	}

	// Also list files directly to ensure we have all files
	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Parse Claude's response for descriptions
	var response struct {
		Content string `json:"content"`
	}
	descriptions := make(map[string]string)
	if err := json.Unmarshal([]byte(result), &response); err == nil {
		// Simple parsing of Claude's response to extract descriptions
		// This is a simplistic approach and might need refinement
		lines := strings.Split(response.Content, "\n")
		currentFile := ""
		currentDesc := ""

		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Look for lines that start with a filename pattern
			if strings.Contains(line, ".") && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*") {
				// Save previous file and description
				if currentFile != "" && currentDesc != "" {
					descriptions[currentFile] = currentDesc
				}

				// Extract the new filename
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 0 {
					currentFile = strings.TrimSpace(parts[0])
					currentDesc = ""
					if len(parts) > 1 {
						currentDesc = strings.TrimSpace(parts[1])
					}
				}
			} else if currentFile != "" {
				// Add to current description
				if currentDesc != "" {
					currentDesc += " "
				}
				currentDesc += line
			}
		}

		// Save the last file and description
		if currentFile != "" && currentDesc != "" {
			descriptions[currentFile] = currentDesc
		}
	}

	// Combine the results
	var files []core.FileContent
	for _, entry := range dirEntries {
		name := entry.Name()
		path := filepath.Join(dirPath, name)
		fileInfo, err := entry.Info()
		if err != nil {
			// Skip files with errors
			continue
		}

		isDir := entry.IsDir()
		size := int64(0)
		if !isDir {
			size = fileInfo.Size()
		}

		// Try to determine language from file extension for non-directories
		language := ""
		if !isDir {
			ext := strings.ToLower(filepath.Ext(name))
			switch ext {
			case ".go":
				language = "Go"
			case ".js", ".jsx":
				language = "JavaScript"
			case ".ts", ".tsx":
				language = "TypeScript"
			case ".py":
				language = "Python"
			case ".java":
				language = "Java"
			case ".c", ".cpp", ".h", ".hpp":
				language = "C/C++"
			case ".md", ".markdown":
				language = "Markdown"
			case ".json":
				language = "JSON"
			case ".yml", ".yaml":
				language = "YAML"
			}
		}

		files = append(files, core.FileContent{
			Path:        path,
			Name:        name,
			Content:     "", // We don't load content for efficiency
			Size:        size,
			ModifiedAt:  fileInfo.ModTime(),
			IsDirectory: isDir,
			Language:    language,
		})
	}

	return files, nil
}

// ExecuteGitCommand executes git commands safely
func (c *ClaudeExecutor) ExecuteGitCommand(ctx context.Context, gitCmd string, execCtx core.ExecutionContext) (*core.QueryResult, error) {
	// Use Claude to analyze and potentially execute the git command
	prompt := fmt.Sprintf("I need to execute this git command: '%s' in directory %s. First, analyze if this command is safe (no destructive operations like force push, etc). If safe, explain what the command will do and show the expected output. If not safe, explain why and suggest alternatives.",
		gitCmd, execCtx.WorkingDir)

	result, err := c.runClaudeCommand(ctx, prompt, execCtx.WorkingDir, "--output-format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to analyze git command using Claude: %w", err)
	}

	// Parse JSON response
	var response struct {
		Content string `json:"content"`
	}
	analysis := ""
	if err := json.Unmarshal([]byte(result), &response); err == nil {
		analysis = response.Content
	} else {
		analysis = result
	}

	// Check if the command seems unsafe (simple heuristic)
	analysisLower := strings.ToLower(analysis)
	if strings.Contains(analysisLower, "unsafe") ||
		strings.Contains(analysisLower, "destructive") ||
		strings.Contains(analysisLower, "dangerous") ||
		strings.Contains(analysisLower, "not recommended") {
		return &core.QueryResult{
			Success:  false,
			Response: "Git command rejected: " + analysis,
			Error:    "Unsafe git command",
		}, nil
	}

	// For safe commands, execute with Claude using bash tool
	prompt = fmt.Sprintf("Execute this git command and show the output: cd %s && %s",
		execCtx.WorkingDir, gitCmd)

	cmdResult, err := c.runClaudeCommand(ctx, prompt, execCtx.WorkingDir, "--output-format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to execute git command: %w", err)
	}

	// Parse JSON response
	if err := json.Unmarshal([]byte(cmdResult), &response); err == nil {
		cmdResult = response.Content
	}

	return &core.QueryResult{
		Success:  true,
		Response: cmdResult,
	}, nil
}

// IsAvailable checks if AI code executor is available and working
func (c *ClaudeExecutor) IsAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, c.executablePath, "-p", "--output-format", "json", "Are you available?")
	availableResult, err := cmd.CombinedOutput()
	slog.DebugContext(ctx, fmt.Sprintf("Claude code availability result: %s", string(availableResult)))
	return err == nil
}

// runClaudeCommand executes a Claude command with the given prompt and options
func (c *ClaudeExecutor) runClaudeCommand(ctx context.Context, prompt, workDir string, extraArgs ...string) (string, error) {
	args := []string{}

	// Add model if specified
	if c.defaultModel != "" {
		args = append(args, "--model", c.defaultModel)
	}

	// Add debug flag if enabled
	// if c.debug {
	// 	args = append(args, "--debug")
	// }

	// Add extra arguments
	args = append(args, extraArgs...)

	// Add working directory context
	if workDir != "" {
		args = append(args, "--add-dir", workDir)
	}

	// IMPORTANT: Add --print option in the last position
	args = append(args, "-p")

	// Add prompt as the last argument
	args = append(args, prompt)

	slog.DebugContext(ctx, "Executing Claude command",
		slog.String("executable", c.executablePath),
		slog.Any("args", args),
	)

	// Create and execute the command
	cmd := exec.CommandContext(ctx, c.executablePath, args...)

	// Capture stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude command failed: %s: %w", string(output), err)
	}

	return string(output), nil
}

// buildPromptWithContext constructs a prompt that includes context information
func (c *ClaudeExecutor) buildPromptWithContext(command string, execCtx core.ExecutionContext) string {
	var sb strings.Builder

	// Add working directory context
	if execCtx.WorkingDir != "" {
		sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", execCtx.WorkingDir))
	}

	// Add user context if available
	if execCtx.UserID != 0 {
		sb.WriteString(fmt.Sprintf("User ID: %d\n", execCtx.UserID))
	}

	// Add project context if available
	if execCtx.ProjectPath != "" {
		sb.WriteString(fmt.Sprintf("Project path: %s\n\n", execCtx.ProjectPath))
	}

	// Add command with a clear separator
	sb.WriteString("Command/Query:\n")
	sb.WriteString(command)

	return sb.String()
}
