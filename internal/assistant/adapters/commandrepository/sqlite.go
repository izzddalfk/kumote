package commandrepository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/izzddalfk/kumote/internal/assistant/core"
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
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	r.logger.DebugContext(ctx, "Saving command",
		"command_id", cmd.ID,
		"user_id", cmd.UserID,
		"text_length", len(cmd.Text),
	)

	query := `
		INSERT INTO commands (id, user_id, text, timestamp, processed_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		cmd.ID,
		cmd.UserID,
		cmd.Text,
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

// initSchema initializes the database schema
func (r *CommandRepository) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS commands (
id TEXT PRIMARY KEY,
user_id INTEGER NOT NULL,
text TEXT,
timestamp DATETIME NOT NULL,
processed_at DATETIME
);
	CREATE INDEX IF NOT EXISTS idx_commands_user_id ON commands(user_id);
	CREATE INDEX IF NOT EXISTS idx_commands_timestamp ON commands(timestamp);
	`

	_, err := r.db.Exec(schema)
	return err
}
