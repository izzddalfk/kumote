package metricscollector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/izzddalfk/kumote/internal/assistant/core"
	_ "github.com/mattn/go-sqlite3"
)

type MetricsCollector struct {
	db *sql.DB
}

// NewMetricsCollector creates a new metrics collector with SQLite
func NewMetricsCollector(dbPath string) (*MetricsCollector, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	collector := &MetricsCollector{
		db: db,
	}

	if err := collector.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize metrics schema: %w", err)
	}

	return collector, nil
}

// Close closes the database connection
func (mc *MetricsCollector) Close() error {
	return mc.db.Close()
}

// RecordCommandExecution records metrics for command execution
func (mc *MetricsCollector) RecordCommandExecution(ctx context.Context, metrics core.CommandMetrics) error {
	slog.DebugContext(ctx, "Recording command execution metrics",
		"command_id", metrics.CommandID,
		"user_id", metrics.UserID,
		"execution_time_ms", metrics.ExecutionTime.Milliseconds(),
		"success", metrics.Success,
	)

	query := `
		INSERT INTO command_metrics (
command_id, user_id, execution_time_ms, success,
project_used, error_type, timestamp
) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := mc.db.ExecContext(ctx, query,
		metrics.CommandID,
		metrics.UserID,
		metrics.ExecutionTime.Milliseconds(),
		metrics.Success,
		metrics.ProjectUsed,
		metrics.ErrorType,
		metrics.Timestamp,
	)

	if err != nil {
		slog.ErrorContext(ctx, "Failed to record command metrics",
			"command_id", metrics.CommandID,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to record command metrics: %w", err)
	}

	return nil
}

// initSchema initializes the database schema
func (mc *MetricsCollector) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS command_metrics (
id INTEGER PRIMARY KEY AUTOINCREMENT,
command_id TEXT NOT NULL,
user_id INTEGER NOT NULL,
execution_time_ms INTEGER NOT NULL,
success BOOLEAN NOT NULL,
project_used TEXT,
error_type TEXT,
timestamp DATETIME NOT NULL
);
	CREATE INDEX IF NOT EXISTS idx_metrics_user_id ON command_metrics(user_id);
	CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON command_metrics(timestamp);
	CREATE INDEX IF NOT EXISTS idx_metrics_command_id ON command_metrics(command_id);
	`

	_, err := mc.db.Exec(schema)
	return err
}
