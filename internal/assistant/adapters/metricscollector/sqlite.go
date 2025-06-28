package metricscollector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
	_ "github.com/mattn/go-sqlite3"
)

type MetricsCollector struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewMetricsCollector creates a new metrics collector with SQLite
func NewMetricsCollector(dbPath string, logger *slog.Logger) (*MetricsCollector, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	collector := &MetricsCollector{
		db:     db,
		logger: logger,
	}

	if err := collector.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize metrics schema: %w", err)
	}

	logger.InfoContext(context.Background(), "Metrics collector initialized",
		"db_path", dbPath,
	)

	return collector, nil
}

// Close closes the database connection
func (mc *MetricsCollector) Close() error {
	return mc.db.Close()
}

// RecordCommandExecution records metrics for command execution
func (mc *MetricsCollector) RecordCommandExecution(ctx context.Context, metrics core.CommandMetrics) error {
	mc.logger.DebugContext(ctx, "Recording command execution metrics",
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
		mc.logger.ErrorContext(ctx, "Failed to record command metrics",
			"command_id", metrics.CommandID,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to record command metrics: %w", err)
	}

	mc.logger.DebugContext(ctx, "Command metrics recorded successfully",
		"command_id", metrics.CommandID,
	)

	return nil
}

// GetUsageStats returns usage statistics for a user within a time period
func (mc *MetricsCollector) GetUsageStats(ctx context.Context, userID int64, period string) (map[string]any, error) {
	mc.logger.DebugContext(ctx, "Getting usage stats",
		"user_id", userID,
		"period", period,
	)

	stats := make(map[string]any)

	// Calculate time window
	var timeWindow time.Duration
	switch strings.ToLower(period) {
	case "hour":
		timeWindow = time.Hour
	case "day":
		timeWindow = 24 * time.Hour
	case "week":
		timeWindow = 7 * 24 * time.Hour
	case "month":
		timeWindow = 30 * 24 * time.Hour
	default:
		timeWindow = 24 * time.Hour // Default to day
	}

	cutoffTime := time.Now().Add(-timeWindow)

	// Total commands executed
	var totalCommands int64
	err := mc.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM command_metrics 
		WHERE user_id = ? AND timestamp > ?
	`, userID, cutoffTime).Scan(&totalCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get total commands: %w", err)
	}
	stats["commands_executed"] = totalCommands

	if totalCommands == 0 {
		// No commands in period, return empty stats
		stats["avg_execution_time_ms"] = 0
		stats["success_rate"] = 0.0
		stats["most_used_project"] = ""
		stats["error_count"] = 0
		return stats, nil
	}

	// Average execution time
	var avgExecutionTime float64
	err = mc.db.QueryRowContext(ctx, `
		SELECT AVG(execution_time_ms) FROM command_metrics 
		WHERE user_id = ? AND timestamp > ?
	`, userID, cutoffTime).Scan(&avgExecutionTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get avg execution time: %w", err)
	}
	stats["avg_execution_time_ms"] = avgExecutionTime

	// Success rate
	var successfulCommands int64
	err = mc.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM command_metrics 
		WHERE user_id = ? AND timestamp > ? AND success = 1
	`, userID, cutoffTime).Scan(&successfulCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get successful commands: %w", err)
	}
	stats["success_rate"] = float64(successfulCommands) / float64(totalCommands)

	// Most used project
	var mostUsedProject sql.NullString
	err = mc.db.QueryRowContext(ctx, `
		SELECT project_used FROM command_metrics 
		WHERE user_id = ? AND timestamp > ? AND project_used IS NOT NULL AND project_used != ''
		GROUP BY project_used 
		ORDER BY COUNT(*) DESC 
		LIMIT 1
	`, userID, cutoffTime).Scan(&mostUsedProject)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get most used project: %w", err)
	}
	if mostUsedProject.Valid {
		stats["most_used_project"] = mostUsedProject.String
	} else {
		stats["most_used_project"] = ""
	}

	// Error count
	var errorCount int64
	err = mc.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM command_metrics 
		WHERE user_id = ? AND timestamp > ? AND success = 0
	`, userID, cutoffTime).Scan(&errorCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get error count: %w", err)
	}
	stats["error_count"] = errorCount

	// Top error types
	errorTypes, err := mc.getTopErrorTypes(ctx, userID, cutoffTime, 3)
	if err != nil {
		mc.logger.WarnContext(ctx, "Failed to get error types", "error", err.Error())
	} else {
		stats["top_error_types"] = errorTypes
	}

	mc.logger.InfoContext(ctx, "Retrieved usage stats",
		"user_id", userID,
		"period", period,
		"commands_executed", totalCommands,
	)

	return stats, nil
}

// GetSystemHealth returns system health metrics
func (mc *MetricsCollector) GetSystemHealth(ctx context.Context) (map[string]any, error) {
	mc.logger.DebugContext(ctx, "Getting system health metrics")

	health := make(map[string]any)
	cutoffTime := time.Now().Add(-24 * time.Hour) // Last 24 hours

	// Total commands in last 24h
	var totalCommands int64
	err := mc.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM command_metrics WHERE timestamp > ?
	`, cutoffTime).Scan(&totalCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get total commands: %w", err)
	}
	health["commands_last_24h"] = totalCommands

	if totalCommands > 0 {
		// Average response time
		var avgResponseTime float64
		err = mc.db.QueryRowContext(ctx, `
			SELECT AVG(execution_time_ms) FROM command_metrics WHERE timestamp > ?
		`, cutoffTime).Scan(&avgResponseTime)
		if err != nil {
			return nil, fmt.Errorf("failed to get avg response time: %w", err)
		}
		health["avg_response_time_ms"] = avgResponseTime

		// Error rate
		var errorCount int64
		err = mc.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM command_metrics WHERE timestamp > ? AND success = 0
		`, cutoffTime).Scan(&errorCount)
		if err != nil {
			return nil, fmt.Errorf("failed to get error count: %w", err)
		}
		health["error_rate"] = float64(errorCount) / float64(totalCommands)

		// Active users
		var activeUsers int64
		err = mc.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT user_id) FROM command_metrics WHERE timestamp > ?
		`, cutoffTime).Scan(&activeUsers)
		if err != nil {
			return nil, fmt.Errorf("failed to get active users: %w", err)
		}
		health["active_users"] = activeUsers
	} else {
		health["avg_response_time_ms"] = 0
		health["error_rate"] = 0.0
		health["active_users"] = 0
	}

	// Database size
	dbStats, err := mc.getDatabaseStats(ctx)
	if err != nil {
		mc.logger.WarnContext(ctx, "Failed to get database stats", "error", err.Error())
	} else {
		health["db_size_bytes"] = dbStats["db_size_bytes"]
		health["total_metrics_records"] = dbStats["total_metrics_records"]
	}

	// System metrics
	sysMetrics, err := mc.getLatestSystemMetrics(ctx)
	if err != nil {
		mc.logger.WarnContext(ctx, "Failed to get system metrics", "error", err.Error())
	} else {
		for key, value := range sysMetrics {
			health[key] = value
		}
	}

	mc.logger.InfoContext(ctx, "Retrieved system health metrics",
		"commands_last_24h", totalCommands,
	)

	return health, nil
}

// RecordSystemMetric records a system metric (e.g., memory usage, AI availability)
func (mc *MetricsCollector) RecordSystemMetric(ctx context.Context, name string, value float64) error {
	mc.logger.DebugContext(ctx, "Recording system metric",
		"metric_name", name,
		"metric_value", value,
	)

	query := `
		INSERT INTO system_metrics (metric_name, metric_value, timestamp)
		VALUES (?, ?, ?)
	`

	_, err := mc.db.ExecContext(ctx, query, name, value, time.Now())
	if err != nil {
		mc.logger.ErrorContext(ctx, "Failed to record system metric",
			"metric_name", name,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to record system metric: %w", err)
	}

	return nil
}

// GetProjectUsageStats returns usage statistics for projects
func (mc *MetricsCollector) GetProjectUsageStats(ctx context.Context, period string) (map[string]any, error) {
	mc.logger.DebugContext(ctx, "Getting project usage stats", "period", period)

	var timeWindow time.Duration
	switch strings.ToLower(period) {
	case "hour":
		timeWindow = time.Hour
	case "day":
		timeWindow = 24 * time.Hour
	case "week":
		timeWindow = 7 * 24 * time.Hour
	case "month":
		timeWindow = 30 * 24 * time.Hour
	default:
		timeWindow = 7 * 24 * time.Hour // Default to week
	}

	cutoffTime := time.Now().Add(-timeWindow)

	query := `
		SELECT project_used, COUNT(*) as usage_count, AVG(execution_time_ms) as avg_time
		FROM command_metrics 
		WHERE timestamp > ? AND project_used IS NOT NULL AND project_used != ''
		GROUP BY project_used
		ORDER BY usage_count DESC
	`

	rows, err := mc.db.QueryContext(ctx, query, cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query project usage: %w", err)
	}
	defer rows.Close()

	projects := make([]map[string]any, 0)
	totalUsage := int64(0)

	for rows.Next() {
		var projectName string
		var usageCount int64
		var avgTime float64

		err := rows.Scan(&projectName, &usageCount, &avgTime)
		if err != nil {
			mc.logger.WarnContext(ctx, "Failed to scan project usage row", "error", err.Error())
			continue
		}

		projects = append(projects, map[string]any{
			"project_name": projectName,
			"usage_count":  usageCount,
			"avg_time_ms":  avgTime,
		})

		totalUsage += usageCount
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project usage rows: %w", err)
	}

	// Calculate percentages
	for _, project := range projects {
		usageCount := project["usage_count"].(int64)
		if totalUsage > 0 {
			project["usage_percentage"] = float64(usageCount) / float64(totalUsage) * 100
		} else {
			project["usage_percentage"] = 0.0
		}
	}

	stats := map[string]any{
		"projects":    projects,
		"total_usage": totalUsage,
		"period":      period,
	}

	mc.logger.InfoContext(ctx, "Retrieved project usage stats",
		"period", period,
		"projects_count", len(projects),
		"total_usage", totalUsage,
	)

	return stats, nil
}

// getTopErrorTypes returns the most common error types for a user
func (mc *MetricsCollector) getTopErrorTypes(ctx context.Context, userID int64, cutoffTime time.Time, limit int) ([]map[string]any, error) {
	query := `
		SELECT error_type, COUNT(*) as error_count
		FROM command_metrics 
		WHERE user_id = ? AND timestamp > ? AND success = 0 AND error_type IS NOT NULL AND error_type != ''
		GROUP BY error_type
		ORDER BY error_count DESC
		LIMIT ?
	`

	rows, err := mc.db.QueryContext(ctx, query, userID, cutoffTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query error types: %w", err)
	}
	defer rows.Close()

	errorTypes := make([]map[string]any, 0)
	for rows.Next() {
		var errorType string
		var errorCount int64

		err := rows.Scan(&errorType, &errorCount)
		if err != nil {
			continue
		}

		errorTypes = append(errorTypes, map[string]any{
			"error_type":  errorType,
			"error_count": errorCount,
		})
	}

	return errorTypes, rows.Err()
}

// getDatabaseStats returns database statistics
func (mc *MetricsCollector) getDatabaseStats(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)

	// Total metrics records
	var totalRecords int64
	err := mc.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM command_metrics").Scan(&totalRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to get total records: %w", err)
	}
	stats["total_metrics_records"] = totalRecords

	// Database file size (SQLite specific)
	var pageCount, pageSize int64
	err = mc.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	if err == nil {
		err = mc.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
		if err == nil {
			stats["db_size_bytes"] = pageCount * pageSize
		}
	}

	return stats, nil
}

// getLatestSystemMetrics returns the latest system metrics
func (mc *MetricsCollector) getLatestSystemMetrics(ctx context.Context) (map[string]any, error) {
	metrics := make(map[string]any)

	query := `
		SELECT metric_name, metric_value 
		FROM system_metrics 
		WHERE timestamp > datetime('now', '-1 hour')
		ORDER BY timestamp DESC
	`

	rows, err := mc.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query system metrics: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	for rows.Next() {
		var name string
		var value float64

		err := rows.Scan(&name, &value)
		if err != nil {
			continue
		}

		// Only take the latest value for each metric name
		if !seen[name] {
			metrics[name] = value
			seen[name] = true
		}
	}

	return metrics, rows.Err()
}

// CleanupOldMetrics deletes metrics older than the specified duration
func (mc *MetricsCollector) CleanupOldMetrics(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	mc.logger.InfoContext(ctx, "Cleaning up old metrics",
		"cutoff_time", cutoffTime,
		"older_than", olderThan.String(),
	)

	// Delete old command metrics
	result, err := mc.db.ExecContext(ctx, "DELETE FROM command_metrics WHERE timestamp < ?", cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old command metrics: %w", err)
	}

	deletedCommands, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get deleted command metrics count: %w", err)
	}

	// Delete old system metrics
	result, err = mc.db.ExecContext(ctx, "DELETE FROM system_metrics WHERE timestamp < ?", cutoffTime)
	if err != nil {
		return deletedCommands, fmt.Errorf("failed to delete old system metrics: %w", err)
	}

	deletedSystem, err := result.RowsAffected()
	if err != nil {
		return deletedCommands, fmt.Errorf("failed to get deleted system metrics count: %w", err)
	}

	totalDeleted := deletedCommands + deletedSystem

	mc.logger.InfoContext(ctx, "Old metrics cleanup completed",
		"deleted_command_metrics", deletedCommands,
		"deleted_system_metrics", deletedSystem,
		"total_deleted", totalDeleted,
	)

	return totalDeleted, nil
}

// initSchema initializes the database schema
func (mc *MetricsCollector) initSchema() error {
	schema := `
		-- Command execution metrics
		CREATE TABLE IF NOT EXISTS command_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command_id TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			execution_time_ms INTEGER NOT NULL,
			success BOOLEAN NOT NULL,
			project_used TEXT,
			error_type TEXT,
			timestamp DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- System health and performance metrics
		CREATE TABLE IF NOT EXISTS system_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			metric_name TEXT NOT NULL,
			metric_value REAL NOT NULL,
			timestamp DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Indexes for performance
		CREATE INDEX IF NOT EXISTS idx_command_metrics_user_id ON command_metrics(user_id);
		CREATE INDEX IF NOT EXISTS idx_command_metrics_timestamp ON command_metrics(timestamp);
		CREATE INDEX IF NOT EXISTS idx_command_metrics_success ON command_metrics(success);
		CREATE INDEX IF NOT EXISTS idx_command_metrics_project ON command_metrics(project_used);
		CREATE INDEX IF NOT EXISTS idx_command_metrics_error_type ON command_metrics(error_type);
		
		CREATE INDEX IF NOT EXISTS idx_system_metrics_name ON system_metrics(metric_name);
		CREATE INDEX IF NOT EXISTS idx_system_metrics_timestamp ON system_metrics(timestamp);
	`

	_, err := mc.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create metrics schema: %w", err)
	}

	mc.logger.InfoContext(context.Background(), "Metrics database schema initialized")
	return nil
}
