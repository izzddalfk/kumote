package codecompletion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/izzddalfk/kumote/internal/assistant/core"
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

type claudeCodeResponse struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	SubType   string `json:"subtype,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
	Result    string `json:"result,omitempty"`
}

// ExecuteCommand runs an AI code command and returns the result
func (c *ClaudeExecutor) ExecuteCommand(ctx context.Context, input core.AgentCommandInput) (*core.QueryResult, error) {
	// Execute Claude CLI command
	rawOutput, err := c.runClaudeCommand(ctx, input)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var response claudeCodeResponse
	if err := json.Unmarshal([]byte(rawOutput), &response); err != nil {
		slog.WarnContext(ctx, "failed to parse Claude Code output", slog.String("output", rawOutput))
		// If JSON parsing fails, return the raw output
		return &core.QueryResult{
			Success:  true,
			Response: string(rawOutput),
		}, nil
	}

	return &core.QueryResult{
		Success:  true,
		Response: response.Result,
	}, nil
}

// IsAvailable checks if Claude CLI is available
func (c *ClaudeExecutor) IsAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, c.executablePath, "--version")
	err := cmd.Run()
	return err == nil
}

// runClaudeCommand executes the Claude CLI with the given prompt
func (c *ClaudeExecutor) runClaudeCommand(ctx context.Context, input core.AgentCommandInput, args ...string) (string, error) {
	// Construct the command
	cmdArgs := []string{
		"--model", c.defaultModel,
		"--output-format", "json",
	}
	// If the session ID is provided, add it to the command
	if input.SessionID != nil {
		cmdArgs = append(cmdArgs, "--resume", *input.SessionID)
	}
	cmdArgs = append(cmdArgs, args...)

	// always add the prompt as the last argument
	cmdArgs = append(cmdArgs, "-p", input.Prompt)

	// Create the command
	cmd := exec.CommandContext(ctx, c.executablePath, cmdArgs...)

	// Set working directory if specified
	cmd.Dir = input.ExecutionContext.WorkingDir

	// Set the prompt as input
	cmd.Stdin = strings.NewReader(input.Prompt)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute claude command: %w", err)
	}

	return string(output), nil
}
