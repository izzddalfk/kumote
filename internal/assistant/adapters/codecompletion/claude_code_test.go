package codecompletion_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/knightazura/kumote/internal/assistant/adapters/codecompletion"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
)

// TestClaudeExecutor_ExecuteCommand tests the ExecuteCommand method
func TestClaudeExecutor_ExecuteCommand(t *testing.T) {
	// Skip if running in CI
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping test in CI environment")
	}

	// Create a temporary directory for our mock claude script
	tmpDir, err := os.MkdirTemp("", "claude-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock claude script that echoes a JSON response
	mockClaudePath := filepath.Join(tmpDir, "mock-claude")
	mockScript := `#!/bin/sh
if echo "$*" | grep -q -- "--output-format json"; then
  # If the command includes JSON output format flag, return JSON
  echo '{"content":"This is a mock response from Claude CLI"}'
else
  # Otherwise just echo back the command for debugging
  echo "Command received: $*"
fi
`
	err = os.WriteFile(mockClaudePath, []byte(mockScript), 0755)
	if err != nil {
		t.Fatalf("Failed to create mock claude script: %v", err)
	}

	// Test cases
	testCases := []struct {
		name           string
		command        string
		expectError    bool
		expectedOutput string
	}{
		{
			name:           "successful command",
			command:        "Tell me about Go programming",
			expectError:    false,
			expectedOutput: "This is a mock response from Claude CLI",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create execution context
			ctx := context.Background()
			execCtx := core.ExecutionContext{
				UserID:      123,
				WorkingDir:  tmpDir,
				ProjectPath: tmpDir,
			}

			// Create the executor with our mock claude script - using the actual implementation
			executor := codecompletion.NewClaudeExecutor(mockClaudePath, "sonnet", tmpDir, false)

			// Execute command
			result, err := executor.ExecuteCommand(ctx, tc.command, execCtx)

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.Success)
				assert.Equal(t, tc.expectedOutput, result.Response)
			}
		})
	}
}

// TestClaudeExecutor_Error tests the error case
func TestClaudeExecutor_Error(t *testing.T) {
	// Skip if running in CI
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping test in CI environment")
	}

	// Create execution context
	ctx := context.Background()
	execCtx := core.ExecutionContext{
		UserID:      123,
		WorkingDir:  "/tmp/test",
		ProjectPath: "/tmp/test",
	}

	// Create an executor with a non-existent command
	executor := codecompletion.NewClaudeExecutor("non_existent_command", "", "/tmp", false)

	// Execute command
	result, err := executor.ExecuteCommand(ctx, "test command", execCtx)

	// Validate results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to execute command with Claude")
}
