package commandrepository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
	_ "github.com/mattn/go-sqlite3"
)

type CommandRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewCommandRepository creates a new command repository with SQLite
func NewCommandRepository(dbPath string, logger *slog.Logger) (*CommandRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	repo := &CommandRepository{
		db:     db,
		logger: logger,
	}

	if err := repo.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.InfoContext(context.Background(), "Command repository initialized",
		"db_path", dbPath,
	)

	return repo, nil
}

// Close closes the database connection
func (r *CommandRepository) Close() error {
	return r.db.Close()
}

// SaveCommand saves command to history
func (r *CommandRepository) SaveCommand(ctx context.Context, cmd *core.Command) error {
	if err := core.ValidateCommand(*cmd); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	r.logger.DebugContext(ctx, "Saving command",
		"command_id", cmd.ID,
		"user_id", cmd.UserID,
		"text_length", len(cmd.Text),
	)

	query := `
		INSERT INTO commands (id, user_id, text, audio_file_id, timestamp, processed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		cmd.ID,
		cmd.UserID,
		cmd.Text,
		cmd.AudioFileID,
		cmd.Timestamp,
		cmd.ProcessedAt,
	)

	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to save command",
			"command_id", cmd.ID,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to save command: %w", err)
	}

	r.logger.InfoContext(ctx, "Command saved successfully",
		"command_id", cmd.ID,
		"user_id", cmd.UserID,
	)

	return nil
}

// GetCommandHistory retrieves command history for user
func (r *CommandRepository) GetCommandHistory(ctx context.Context, userID int64, limit int) ([]core.Command, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 1000 {
		limit = 1000 // Maximum limit
	}

	r.logger.DebugContext(ctx, "Getting command history",
		"user_id", userID,
		"limit", limit,
	)

	query := `
		SELECT id, user_id, text, audio_file_id, timestamp, processed_at
		FROM commands
		WHERE user_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to query command history",
			"user_id", userID,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to query command history: %w", err)
	}
	defer rows.Close()

	var commands []core.Command
	for rows.Next() {
		var cmd core.Command
		var processedAt sql.NullTime

		err := rows.Scan(
			&cmd.ID,
			&cmd.UserID,
			&cmd.Text,
			&cmd.AudioFileID,
			&cmd.Timestamp,
			&processedAt,
		)
		if err != nil {
			r.logger.WarnContext(ctx, "Failed to scan command row",
				"error", err.Error(),
			)
			continue
		}

		if processedAt.Valid {
			cmd.ProcessedAt = &processedAt.Time
		}

		commands = append(commands, cmd)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating command rows: %w", err)
	}

	r.logger.InfoContext(ctx, "Retrieved command history",
		"user_id", userID,
		"commands_found", len(commands),
	)

	return commands, nil
}

// GetCommandByID retrieves specific command by ID
func (r *CommandRepository) GetCommandByID(ctx context.Context, commandID string) (*core.Command, error) {
	r.logger.DebugContext(ctx, "Getting command by ID", "command_id", commandID)

	query := `
		SELECT id, user_id, text, audio_file_id, timestamp, processed_at
		FROM commands
		WHERE id = ?
	`

	var cmd core.Command
	var processedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, commandID).Scan(
		&cmd.ID,
		&cmd.UserID,
		&cmd.Text,
		&cmd.AudioFileID,
		&cmd.Timestamp,
		&processedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, core.ErrCommandNotFound
		}
		r.logger.ErrorContext(ctx, "Failed to get command by ID",
			"command_id", commandID,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to get command: %w", err)
	}

	if processedAt.Valid {
		cmd.ProcessedAt = &processedAt.Time
	}

	r.logger.DebugContext(ctx, "Retrieved command by ID",
		"command_id", commandID,
		"user_id", cmd.UserID,
	)

	return &cmd, nil
}

// UpdateCommandProcessedAt updates the processed_at timestamp for a command
func (r *CommandRepository) UpdateCommandProcessedAt(ctx context.Context, commandID string, processedAt time.Time) error {
	r.logger.DebugContext(ctx, "Updating command processed_at",
		"command_id", commandID,
		"processed_at", processedAt,
	)

	query := `UPDATE commands SET processed_at = ? WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, processedAt, commandID)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to update command processed_at",
			"command_id", commandID,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to update command: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return core.ErrCommandNotFound
	}

	r.logger.InfoContext(ctx, "Command processed_at updated successfully",
		"command_id", commandID,
	)

	return nil
}

// GetCommandCount returns total number of commands for a user
func (r *CommandRepository) GetCommandCount(ctx context.Context, userID int64) (int64, error) {
	r.logger.DebugContext(ctx, "Getting command count", "user_id", userID)

	query := `SELECT COUNT(*) FROM commands WHERE user_id = ?`

	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to get command count",
			"user_id", userID,
			"error", err.Error(),
		)
		return 0, fmt.Errorf("failed to get command count: %w", err)
	}

	return count, nil
}

// GetRecentCommands returns recent commands across all users (for admin purposes)
func (r *CommandRepository) GetRecentCommands(ctx context.Context, limit int) ([]core.Command, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	r.logger.DebugContext(ctx, "Getting recent commands", "limit", limit)

	query := `
		SELECT id, user_id, text, audio_file_id, timestamp, processed_at
		FROM commands
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to query recent commands",
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to query recent commands: %w", err)
	}
	defer rows.Close()

	var commands []core.Command
	for rows.Next() {
		var cmd core.Command
		var processedAt sql.NullTime

		err := rows.Scan(
			&cmd.ID,
			&cmd.UserID,
			&cmd.Text,
			&cmd.AudioFileID,
			&cmd.Timestamp,
			&processedAt,
		)
		if err != nil {
			r.logger.WarnContext(ctx, "Failed to scan recent command row",
				"error", err.Error(),
			)
			continue
		}

		if processedAt.Valid {
			cmd.ProcessedAt = &processedAt.Time
		}

		commands = append(commands, cmd)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recent command rows: %w", err)
	}

	r.logger.InfoContext(ctx, "Retrieved recent commands", "commands_found", len(commands))

	return commands, nil
}

// DeleteOldCommands deletes commands older than the specified duration
func (r *CommandRepository) DeleteOldCommands(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	r.logger.InfoContext(ctx, "Deleting old commands",
		"cutoff_time", cutoffTime,
		"older_than", olderThan.String(),
	)

	query := `DELETE FROM commands WHERE timestamp < ?`

	result, err := r.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to delete old commands",
			"error", err.Error(),
		)
		return 0, fmt.Errorf("failed to delete old commands: %w", err)
	}

	deletedCount, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get deleted count: %w", err)
	}

	r.logger.InfoContext(ctx, "Old commands deleted successfully",
		"deleted_count", deletedCount,
		"cutoff_time", cutoffTime,
	)

	return deletedCount, nil
}

// initSchema initializes the database schema
func (r *CommandRepository) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS commands (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			text TEXT NOT NULL,
			audio_file_id TEXT,
			timestamp DATETIME NOT NULL,
			processed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_commands_user_id ON commands(user_id);
		CREATE INDEX IF NOT EXISTS idx_commands_timestamp ON commands(timestamp);
		CREATE INDEX IF NOT EXISTS idx_commands_processed_at ON commands(processed_at);
	`

	_, err := r.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	r.logger.InfoContext(context.Background(), "Database schema initialized")
	return nil
}

// GetDatabaseStats returns database statistics
func (r *CommandRepository) GetDatabaseStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total commands count
	var totalCommands int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM commands").Scan(&totalCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get total commands count: %w", err)
	}
	stats["total_commands"] = totalCommands

	// Commands in last 24 hours
	var recentCommands int64
	err = r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM commands WHERE timestamp > datetime('now', '-1 day')",
	).Scan(&recentCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent commands count: %w", err)
	}
	stats["commands_last_24h"] = recentCommands

	// Unique users
	var uniqueUsers int64
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT user_id) FROM commands").Scan(&uniqueUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique users count: %w", err)
	}
	stats["unique_users"] = uniqueUsers

	// Oldest command
	var oldestTimestamp sql.NullString
	err = r.db.QueryRowContext(ctx, "SELECT MIN(timestamp) FROM commands").Scan(&oldestTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to get oldest command: %w", err)
	}

	if oldestTimestamp.Valid && oldestTimestamp.String != "" {
		// Try parsing with multiple formats
		formats := []string{
			time.RFC3339,
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05Z07:00",
			"2006-01-02 15:04:05-07:00",
		}

		var oldestTime time.Time
		var parseErr error
		for _, format := range formats {
			oldestTime, parseErr = time.Parse(format, oldestTimestamp.String)
			if parseErr == nil {
				stats["oldest_command"] = oldestTime
				break
			}
		}

		// If all parsing attempts failed, just store as string
		if parseErr != nil {
			r.logger.WarnContext(ctx, "Failed to parse oldest command timestamp",
				"timestamp", oldestTimestamp.String,
				"error", parseErr.Error(),
			)
			stats["oldest_command"] = oldestTimestamp.String
		}
	} else {
		// No commands in the database
		stats["oldest_command"] = nil
	}

	// Database file size (SQLite specific)
	var pageCount, pageSize int64
	err = r.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	if err == nil {
		err = r.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
		if err == nil {
			stats["db_size_bytes"] = pageCount * pageSize
		}
	}

	r.logger.DebugContext(ctx, "Retrieved database stats", "stats", stats)

	return stats, nil
}
