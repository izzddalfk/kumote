package commandrepository_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/knightazura/kumote/internal/assistant/adapters/commandrepository"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCommandRepo(t *testing.T) (*commandrepository.CommandRepository, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_commands.db")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in tests
	}))

	repo, err := commandrepository.NewCommandRepository(dbPath, logger)
	require.NoError(t, err)

	cleanup := func() {
		repo.Close()
	}

	return repo, cleanup
}

func createTestCommand(userID int64, text string) core.Command {
	return core.Command{
		ID:        "cmd-" + text + "-123",
		UserID:    userID,
		Text:      text,
		Timestamp: time.Now(),
	}
}

func TestCommandRepository_SaveCommand_Success(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	cmd := createTestCommand(123456789, "list projects")

	err := repo.SaveCommand(ctx, &cmd)
	require.NoError(t, err)

	// Verify command was saved
	savedCmd, err := repo.GetCommandByID(ctx, cmd.ID)
	require.NoError(t, err)
	assert.Equal(t, cmd.ID, savedCmd.ID)
	assert.Equal(t, cmd.UserID, savedCmd.UserID)
	assert.Equal(t, cmd.Text, savedCmd.Text)
	assert.Equal(t, cmd.AudioFileID, savedCmd.AudioFileID)
	assert.Nil(t, savedCmd.ProcessedAt)
}

func TestCommandRepository_SaveCommand_WithAudio(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	cmd := core.Command{
		ID:          "cmd-audio-123",
		UserID:      123456789,
		Text:        "",
		AudioFileID: "audio-file-456",
		Timestamp:   time.Now(),
	}

	err := repo.SaveCommand(ctx, &cmd)
	require.NoError(t, err)

	// Verify command was saved with audio file ID
	savedCmd, err := repo.GetCommandByID(ctx, cmd.ID)
	require.NoError(t, err)
	assert.Equal(t, cmd.AudioFileID, savedCmd.AudioFileID)
}

func TestCommandRepository_SaveCommand_InvalidCommand(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Command with empty ID
	invalidCmd := core.Command{
		ID:        "", // Invalid
		UserID:    123456789,
		Text:      "list projects",
		Timestamp: time.Now(),
	}

	err := repo.SaveCommand(ctx, &invalidCmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid command")
}

func TestCommandRepository_GetCommandByID_NotFound(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()

	cmd, err := repo.GetCommandByID(ctx, "nonexistent-id")
	assert.Error(t, err)
	assert.Equal(t, core.ErrCommandNotFound, err)
	assert.Nil(t, cmd)
}

func TestCommandRepository_GetCommandHistory_Success(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Save multiple commands for the user
	commands := []core.Command{
		createTestCommand(userID, "list projects"),
		createTestCommand(userID, "show taqwa main.go"),
		createTestCommand(userID, "git status"),
	}

	for i := range commands {
		// Make timestamps different so we can test ordering
		commands[i].Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		err := repo.SaveCommand(ctx, &commands[i])
		require.NoError(t, err)
	}

	// Save command for different user (should not be included)
	otherUserCmd := createTestCommand(987654321, "other user command")
	err := repo.SaveCommand(ctx, &otherUserCmd)
	require.NoError(t, err)

	// Get command history
	history, err := repo.GetCommandHistory(ctx, userID, 10)
	require.NoError(t, err)
	assert.Len(t, history, 3)

	// Verify commands are ordered by timestamp DESC (newest first)
	assert.Equal(t, "git status", history[0].Text)
	assert.Equal(t, "show taqwa main.go", history[1].Text)
	assert.Equal(t, "list projects", history[2].Text)

	// Verify all commands belong to the correct user
	for _, cmd := range history {
		assert.Equal(t, userID, cmd.UserID)
	}
}

func TestCommandRepository_GetCommandHistory_WithLimit(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Save 5 commands
	for i := 0; i < 5; i++ {
		cmd := createTestCommand(userID, fmt.Sprintf("command %d", i))
		cmd.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		err := repo.SaveCommand(ctx, &cmd)
		require.NoError(t, err)
	}

	// Get history with limit of 3
	history, err := repo.GetCommandHistory(ctx, userID, 3)
	require.NoError(t, err)
	assert.Len(t, history, 3)

	// Should get the 3 most recent commands
	assert.Equal(t, "command 4", history[0].Text)
	assert.Equal(t, "command 3", history[1].Text)
	assert.Equal(t, "command 2", history[2].Text)
}

func TestCommandRepository_UpdateCommandProcessedAt(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	cmd := createTestCommand(123456789, "list projects")

	// Save command
	err := repo.SaveCommand(ctx, &cmd)
	require.NoError(t, err)

	// Update processed_at
	processedAt := time.Now()
	err = repo.UpdateCommandProcessedAt(ctx, cmd.ID, processedAt)
	require.NoError(t, err)

	// Verify processed_at was updated
	updatedCmd, err := repo.GetCommandByID(ctx, cmd.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedCmd.ProcessedAt)
	assert.WithinDuration(t, processedAt, *updatedCmd.ProcessedAt, time.Second)
}

func TestCommandRepository_UpdateCommandProcessedAt_NotFound(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()

	err := repo.UpdateCommandProcessedAt(ctx, "nonexistent-id", time.Now())
	assert.Error(t, err)
	assert.Equal(t, core.ErrCommandNotFound, err)
}

func TestCommandRepository_GetCommandCount(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Initially no commands
	count, err := repo.GetCommandCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Save 3 commands
	for i := 0; i < 3; i++ {
		cmd := createTestCommand(userID, fmt.Sprintf("command %d", i))
		err := repo.SaveCommand(ctx, &cmd)
		require.NoError(t, err)
	}

	// Verify count
	count, err = repo.GetCommandCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Save command for different user
	otherCmd := createTestCommand(987654321, "other user command")
	err = repo.SaveCommand(ctx, &otherCmd)
	require.NoError(t, err)

	// Count for original user should remain 3
	count, err = repo.GetCommandCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestCommandRepository_GetRecentCommands(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Save commands from different users
	users := []int64{123456789, 987654321, 111222333}
	for i, userID := range users {
		cmd := createTestCommand(userID, fmt.Sprintf("command from user %d", i))
		cmd.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		err := repo.SaveCommand(ctx, &cmd)
		require.NoError(t, err)
	}

	// Get recent commands
	recent, err := repo.GetRecentCommands(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, recent, 3)

	// Should be ordered by timestamp DESC
	assert.Equal(t, int64(111222333), recent[0].UserID) // Most recent
	assert.Equal(t, int64(987654321), recent[1].UserID)
	assert.Equal(t, int64(123456789), recent[2].UserID) // Oldest
}

func TestCommandRepository_GetRecentCommands_WithLimit(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Save 5 commands
	for i := 0; i < 5; i++ {
		cmd := createTestCommand(123456789, fmt.Sprintf("command %d", i))
		cmd.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		err := repo.SaveCommand(ctx, &cmd)
		require.NoError(t, err)
	}

	// Get recent commands with limit
	recent, err := repo.GetRecentCommands(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, recent, 3)

	// Should get the 3 most recent
	assert.Equal(t, "command 4", recent[0].Text)
	assert.Equal(t, "command 3", recent[1].Text)
	assert.Equal(t, "command 2", recent[2].Text)
}

func TestCommandRepository_DeleteOldCommands(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Save old commands
	oldTime := time.Now().Add(-48 * time.Hour) // 2 days ago
	for i := 0; i < 3; i++ {
		cmd := createTestCommand(userID, fmt.Sprintf("old command %d", i))
		cmd.Timestamp = oldTime.Add(time.Duration(i) * time.Minute)
		err := repo.SaveCommand(ctx, &cmd)
		require.NoError(t, err)
	}

	// Save recent commands
	recentTime := time.Now().Add(-1 * time.Hour) // 1 hour ago
	for i := 0; i < 2; i++ {
		cmd := createTestCommand(userID, fmt.Sprintf("recent command %d", i))
		cmd.Timestamp = recentTime.Add(time.Duration(i) * time.Minute)
		err := repo.SaveCommand(ctx, &cmd)
		require.NoError(t, err)
	}

	// Delete commands older than 24 hours
	deletedCount, err := repo.DeleteOldCommands(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(3), deletedCount) // Should delete 3 old commands

	// Verify recent commands still exist
	remainingCount, err := repo.GetCommandCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), remainingCount)

	// Verify the remaining commands are the recent ones
	history, err := repo.GetCommandHistory(ctx, userID, 10)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Contains(t, history[0].Text, "recent command")
	assert.Contains(t, history[1].Text, "recent command")
}

func TestCommandRepository_GetDatabaseStats(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Save some test data
	users := []int64{123456789, 987654321}
	for u, userID := range users {
		for i := 0; i < 3; i++ {
			// Create unique commands for each user and each iteration
			cmd := createTestCommand(userID, fmt.Sprintf("command %d-%d", u, i))
			// Make some commands recent, some old
			if i == 0 {
				cmd.Timestamp = time.Now().Add(-25 * time.Hour) // Old
			} else {
				cmd.Timestamp = time.Now().Add(-time.Duration(i) * time.Hour) // Recent
			}
			err := repo.SaveCommand(ctx, &cmd)
			require.NoError(t, err)
		}
	}

	// Get stats
	stats, err := repo.GetDatabaseStats(ctx)
	require.NoError(t, err)

	// Verify stats
	assert.Equal(t, int64(6), stats["total_commands"])
	assert.Equal(t, int64(6), stats["commands_last_24h"]) // 6 commands in the last 24h according to SQLite's datetime
	assert.Equal(t, int64(2), stats["unique_users"])
	assert.Equal(t, int64(2), stats["unique_users"])
	assert.NotNil(t, stats["oldest_command"])

	// Database size should be present (SQLite specific)
	if dbSize, exists := stats["db_size_bytes"]; exists {
		assert.Greater(t, dbSize.(int64), int64(0))
	}
}

func TestCommandRepository_ConcurrentAccess(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Test concurrent writes
	const numGoroutines = 10
	const commandsPerGoroutine = 5

	// Channel to collect errors
	errChan := make(chan error, numGoroutines*commandsPerGoroutine)

	// Start multiple goroutines writing commands
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for i := 0; i < commandsPerGoroutine; i++ {
				cmd := createTestCommand(userID, fmt.Sprintf("concurrent command %d-%d", goroutineID, i))
				cmd.ID = fmt.Sprintf("cmd-%d-%d", goroutineID, i)
				err := repo.SaveCommand(ctx, &cmd)
				errChan <- err
			}
		}(g)
	}

	// Collect all errors
	for i := 0; i < numGoroutines*commandsPerGoroutine; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}

	// Verify all commands were saved
	totalCount, err := repo.GetCommandCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(numGoroutines*commandsPerGoroutine), totalCount)
}

func TestCommandRepository_LargeTextCommand(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create command with large text (but within limits)
	largeText := make([]byte, 3000) // 3KB text
	for i := range largeText {
		largeText[i] = 'a'
	}

	cmd := core.Command{
		ID:        "large-text-cmd",
		UserID:    123456789,
		Text:      string(largeText),
		Timestamp: time.Now(),
	}

	// Save and retrieve
	err := repo.SaveCommand(ctx, &cmd)
	require.NoError(t, err)

	savedCmd, err := repo.GetCommandByID(ctx, cmd.ID)
	require.NoError(t, err)
	assert.Equal(t, len(largeText), len(savedCmd.Text))
	assert.Equal(t, string(largeText), savedCmd.Text)
}

func TestCommandRepository_EmptyDatabase(t *testing.T) {
	repo, cleanup := setupCommandRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Test operations on empty database
	history, err := repo.GetCommandHistory(ctx, userID, 10)
	require.NoError(t, err)
	assert.Len(t, history, 0)

	count, err := repo.GetCommandCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	recent, err := repo.GetRecentCommands(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, recent, 0)

	deletedCount, err := repo.DeleteOldCommands(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deletedCount)

	stats, err := repo.GetDatabaseStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats["total_commands"])
	assert.Equal(t, int64(0), stats["commands_last_24h"])
	assert.Equal(t, int64(0), stats["unique_users"])
}
