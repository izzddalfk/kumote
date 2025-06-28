package metricscollector_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/knightazura/kumote/internal/assistant/adapters/metricscollector"
	"github.com/knightazura/kumote/internal/assistant/core"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMetricsCollector(t *testing.T) (*metricscollector.MetricsCollector, func()) {
	testColl, cleanup := setupTestCollector(t)
	return testColl.MetricsCollector, cleanup
}

func createTestCommandMetrics(userID int64, commandID string, success bool, executionTime time.Duration) core.CommandMetrics {
	return core.CommandMetrics{
		CommandID:     commandID,
		UserID:        userID,
		ExecutionTime: executionTime,
		Success:       success,
		ProjectUsed:   "TaqwaBoard",
		ErrorType:     "",
		Timestamp:     time.Now(),
	}
}

func TestMetricsCollector_RecordCommandExecution_Success(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	metrics := createTestCommandMetrics(123456789, "cmd-123", true, 1500*time.Millisecond)

	err := collector.RecordCommandExecution(ctx, metrics)
	require.NoError(t, err)

	// Verify metrics were recorded by checking usage stats
	stats, err := collector.GetUsageStats(ctx, 123456789, "day")
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats["commands_executed"])
	assert.Equal(t, 1500.0, stats["avg_execution_time_ms"])
	assert.Equal(t, 1.0, stats["success_rate"])
	assert.Equal(t, "TaqwaBoard", stats["most_used_project"])
}

func TestMetricsCollector_RecordCommandExecution_WithError(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	metrics := core.CommandMetrics{
		CommandID:     "cmd-error-123",
		UserID:        123456789,
		ExecutionTime: 2000 * time.Millisecond,
		Success:       false,
		ProjectUsed:   "TaqwaBoard",
		ErrorType:     "timeout",
		Timestamp:     time.Now(),
	}

	err := collector.RecordCommandExecution(ctx, metrics)
	require.NoError(t, err)

	// Verify error metrics
	stats, err := collector.GetUsageStats(ctx, 123456789, "day")
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats["commands_executed"])
	assert.Equal(t, 0.0, stats["success_rate"])
	assert.Equal(t, int64(1), stats["error_count"])

	// Check error types
	errorTypes := stats["top_error_types"].([]map[string]any)
	assert.Len(t, errorTypes, 1)
	assert.Equal(t, "timeout", errorTypes[0]["error_type"])
	assert.Equal(t, int64(1), errorTypes[0]["error_count"])
}

func TestMetricsCollector_GetUsageStats_MultipleCommands(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Record multiple commands
	commands := []core.CommandMetrics{
		createTestCommandMetrics(userID, "cmd-1", true, 1000*time.Millisecond),
		createTestCommandMetrics(userID, "cmd-2", true, 2000*time.Millisecond),
		createTestCommandMetrics(userID, "cmd-3", false, 500*time.Millisecond),
	}

	// Set different projects
	commands[1].ProjectUsed = "CarLogbook"
	commands[2].ErrorType = "not_found"

	for _, cmd := range commands {
		err := collector.RecordCommandExecution(ctx, cmd)
		require.NoError(t, err)
	}

	// Get usage stats
	stats, err := collector.GetUsageStats(ctx, userID, "day")
	require.NoError(t, err)

	assert.Equal(t, int64(3), stats["commands_executed"])
	assert.Equal(t, (1000.0+2000.0+500.0)/3.0, stats["avg_execution_time_ms"])
	assert.Equal(t, 2.0/3.0, stats["success_rate"])
	assert.Equal(t, int64(1), stats["error_count"])

	// Should return TaqwaBoard as most used (2 uses vs 1 for CarLogbook)
	// Note: cmd-3 failed but still used TaqwaBoard
	assert.Equal(t, "TaqwaBoard", stats["most_used_project"])
}

func TestMetricsCollector_GetUsageStats_DifferentPeriods(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Record command from 2 hours ago
	oldMetrics := createTestCommandMetrics(userID, "cmd-old", true, 1000*time.Millisecond)
	oldMetrics.Timestamp = time.Now().Add(-2 * time.Hour)
	err := collector.RecordCommandExecution(ctx, oldMetrics)
	require.NoError(t, err)

	// Record recent command
	recentMetrics := createTestCommandMetrics(userID, "cmd-recent", true, 1500*time.Millisecond)
	err = collector.RecordCommandExecution(ctx, recentMetrics)
	require.NoError(t, err)

	// Get stats for last hour - should only include recent command
	hourStats, err := collector.GetUsageStats(ctx, userID, "hour")
	require.NoError(t, err)
	assert.Equal(t, int64(1), hourStats["commands_executed"])

	// Get stats for last day - should include both commands
	dayStats, err := collector.GetUsageStats(ctx, userID, "day")
	require.NoError(t, err)
	assert.Equal(t, int64(2), dayStats["commands_executed"])
}

func TestMetricsCollector_GetUsageStats_NoCommands(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Get stats for user with no commands
	stats, err := collector.GetUsageStats(ctx, userID, "day")
	require.NoError(t, err)

	// Check types and values using type assertion
	assert.Equal(t, int64(0), toInt64(t, stats["commands_executed"]))
	assert.Equal(t, float64(0), toFloat64(t, stats["avg_execution_time_ms"]))
	assert.Equal(t, float64(0), toFloat64(t, stats["success_rate"]))
	assert.Equal(t, "", stats["most_used_project"])
	assert.Equal(t, int64(0), toInt64(t, stats["error_count"]))
}

func TestMetricsCollector_GetSystemHealth(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()

	// Record some command metrics from different users
	users := []int64{123456789, 987654321}
	for i, userID := range users {
		for j := 0; j < 3; j++ {
			metrics := createTestCommandMetrics(userID, fmt.Sprintf("cmd-%d-%d", i, j), j != 2, time.Duration(1000+j*500)*time.Millisecond)
			if j == 2 { // Make last command fail
				metrics.ErrorType = "timeout"
			}
			err := collector.RecordCommandExecution(ctx, metrics)
			require.NoError(t, err)
		}
	}

	// Record some system metrics
	err := collector.RecordSystemMetric(ctx, "ai_cli_available", 1.0)
	require.NoError(t, err)
	err = collector.RecordSystemMetric(ctx, "memory_usage_percent", 75.5)
	require.NoError(t, err)

	// Get system health
	health, err := collector.GetSystemHealth(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(6), health["commands_last_24h"])
	assert.Equal(t, int64(2), health["active_users"])
	assert.Equal(t, 2.0/6.0, health["error_rate"]) // 2 failures out of 6 commands
	assert.Greater(t, health["avg_response_time_ms"].(float64), 0.0)

	// Check system metrics
	assert.Equal(t, 1.0, health["ai_cli_available"])
	assert.Equal(t, 75.5, health["memory_usage_percent"])

	// Database stats should be present
	assert.Greater(t, health["total_metrics_records"].(int64), int64(0))
	assert.Greater(t, health["db_size_bytes"].(int64), int64(0))
}

func TestMetricsCollector_RecordSystemMetric(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()

	// Record various system metrics
	metrics := map[string]float64{
		"ai_cli_available":     1.0,
		"memory_usage_percent": 78.5,
		"cpu_usage_percent":    45.2,
		"disk_usage_percent":   62.1,
	}

	for name, value := range metrics {
		err := collector.RecordSystemMetric(ctx, name, value)
		require.NoError(t, err)
	}

	// Verify metrics are included in system health
	health, err := collector.GetSystemHealth(ctx)
	require.NoError(t, err)

	for name, expectedValue := range metrics {
		assert.Equal(t, expectedValue, health[name])
	}
}

func TestMetricsCollector_GetProjectUsageStats(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Record commands for different projects
	projects := []string{"TaqwaBoard", "TaqwaBoard", "CarLogbook", "TaqwaBoard"}
	for i, project := range projects {
		metrics := createTestCommandMetrics(userID, fmt.Sprintf("cmd-%d", i), true, time.Duration(1000+i*200)*time.Millisecond)
		metrics.ProjectUsed = project
		err := collector.RecordCommandExecution(ctx, metrics)
		require.NoError(t, err)
	}

	// Get project usage stats
	stats, err := collector.GetProjectUsageStats(ctx, "day")
	require.NoError(t, err)

	assert.Equal(t, int64(4), stats["total_usage"])
	assert.Equal(t, "day", stats["period"])

	projectStats := stats["projects"].([]map[string]any)
	assert.Len(t, projectStats, 2)

	// TaqwaBoard should be first (3 uses)
	assert.Equal(t, "TaqwaBoard", projectStats[0]["project_name"])
	assert.Equal(t, int64(3), projectStats[0]["usage_count"])
	assert.Equal(t, 75.0, projectStats[0]["usage_percentage"]) // 3/4 * 100

	// CarLogbook should be second (1 use)
	assert.Equal(t, "CarLogbook", projectStats[1]["project_name"])
	assert.Equal(t, int64(1), projectStats[1]["usage_count"])
	assert.Equal(t, 25.0, projectStats[1]["usage_percentage"]) // 1/4 * 100
}

func TestMetricsCollector_CleanupOldMetrics(t *testing.T) {
	collector, cleanup := setupTestCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Record old metrics (3 days ago)
	oldTime := time.Now().Add(-72 * time.Hour)
	for i := 0; i < 3; i++ {
		metrics := createTestCommandMetrics(userID, fmt.Sprintf("old-cmd-%d", i), true, 1000*time.Millisecond)
		metrics.Timestamp = oldTime.Add(time.Duration(i) * time.Minute)
		err := collector.RecordCommandExecution(ctx, metrics)
		require.NoError(t, err)
	}

	// Record recent metrics (1 hour ago)
	recentTime := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 2; i++ {
		metrics := createTestCommandMetrics(userID, fmt.Sprintf("recent-cmd-%d", i), true, 1500*time.Millisecond)
		metrics.Timestamp = recentTime.Add(time.Duration(i) * time.Minute)
		err := collector.RecordCommandExecution(ctx, metrics)
		require.NoError(t, err)
	}

	// Record old system metrics
	err := collector.RecordSystemMetric(ctx, "old_metric", 100.0)
	require.NoError(t, err)

	// Manually update timestamp to make it old
	_, err = collector.ExecuteSQL(ctx,
		"UPDATE system_metrics SET timestamp = ? WHERE metric_name = ?",
		oldTime, "old_metric")
	require.NoError(t, err)

	// Record recent system metric
	err = collector.RecordSystemMetric(ctx, "recent_metric", 200.0)
	require.NoError(t, err)

	// Cleanup metrics older than 48 hours
	deletedCount, err := collector.CleanupOldMetrics(ctx, 48*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(4), deletedCount) // 3 command metrics + 1 system metric

	// Verify recent metrics still exist
	stats, err := collector.GetUsageStats(ctx, userID, "day")
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats["commands_executed"]) // Only recent commands

	health, err := collector.GetSystemHealth(ctx)
	require.NoError(t, err)
	// Should only have recent system metric
	assert.Equal(t, 200.0, health["recent_metric"])
	_, hasOldMetric := health["old_metric"]
	assert.False(t, hasOldMetric)
}

func TestMetricsCollector_ConcurrentWrites(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Test concurrent metric recording
	const numGoroutines = 10
	const metricsPerGoroutine = 5

	errChan := make(chan error, numGoroutines*metricsPerGoroutine)

	// Start multiple goroutines recording metrics
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for i := 0; i < metricsPerGoroutine; i++ {
				metrics := createTestCommandMetrics(userID, fmt.Sprintf("concurrent-cmd-%d-%d", goroutineID, i), true, 1000*time.Millisecond)
				err := collector.RecordCommandExecution(ctx, metrics)
				errChan <- err
			}
		}(g)
	}

	// Collect all errors
	for i := 0; i < numGoroutines*metricsPerGoroutine; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}

	// Verify all metrics were recorded
	stats, err := collector.GetUsageStats(ctx, userID, "day")
	require.NoError(t, err)
	assert.Equal(t, int64(numGoroutines*metricsPerGoroutine), stats["commands_executed"])
}

func TestMetricsCollector_ErrorTypes_TopErrors(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Record commands with different error types
	errorCommands := []struct {
		errorType string
		count     int
	}{
		{"timeout", 5},
		{"not_found", 3},
		{"permission_denied", 2},
		{"invalid_command", 1},
	}

	for _, ec := range errorCommands {
		for i := 0; i < ec.count; i++ {
			metrics := createTestCommandMetrics(userID, fmt.Sprintf("error-cmd-%s-%d", ec.errorType, i), false, 1000*time.Millisecond)
			metrics.ErrorType = ec.errorType
			err := collector.RecordCommandExecution(ctx, metrics)
			require.NoError(t, err)
		}
	}

	// Get usage stats
	stats, err := collector.GetUsageStats(ctx, userID, "day")
	require.NoError(t, err)

	topErrors := stats["top_error_types"].([]map[string]any)
	assert.Len(t, topErrors, 3) // Limited to top 3

	// Should be ordered by count (descending)
	assert.Equal(t, "timeout", topErrors[0]["error_type"])
	assert.Equal(t, int64(5), topErrors[0]["error_count"])

	assert.Equal(t, "not_found", topErrors[1]["error_type"])
	assert.Equal(t, int64(3), topErrors[1]["error_count"])

	assert.Equal(t, "permission_denied", topErrors[2]["error_type"])
	assert.Equal(t, int64(2), topErrors[2]["error_count"])
}

func TestMetricsCollector_MultipleUsers(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()

	// Record metrics for different users
	users := []int64{123456789, 987654321, 111222333}
	for i, userID := range users {
		for j := 0; j < i+2; j++ { // Different number of commands per user
			metrics := createTestCommandMetrics(userID, fmt.Sprintf("cmd-%d-%d", i, j), true, time.Duration(1000+j*100)*time.Millisecond)
			err := collector.RecordCommandExecution(ctx, metrics)
			require.NoError(t, err)
		}
	}

	// Verify each user has correct stats
	expectedCounts := []int64{2, 3, 4} // user 0: 2 commands, user 1: 3 commands, user 2: 4 commands
	for i, userID := range users {
		stats, err := collector.GetUsageStats(ctx, userID, "day")
		require.NoError(t, err)
		assert.Equal(t, expectedCounts[i], stats["commands_executed"])
	}

	// System health should show total across all users
	health, err := collector.GetSystemHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(9), health["commands_last_24h"]) // 2+3+4 = 9
	assert.Equal(t, int64(3), health["active_users"])
}

func TestMetricsCollector_EmptyDatabase_HealthCheck(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()

	// Test system health on empty database
	health, err := collector.GetSystemHealth(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(0), toInt64(t, health["commands_last_24h"]))
	assert.Equal(t, float64(0), toFloat64(t, health["avg_response_time_ms"]))
	assert.Equal(t, float64(0), toFloat64(t, health["error_rate"]))
	assert.Equal(t, int64(0), toInt64(t, health["active_users"]))
	assert.Equal(t, int64(0), toInt64(t, health["total_metrics_records"]))
}

func TestMetricsCollector_ProjectUsageStats_NoData(t *testing.T) {
	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()

	// Get project usage stats on empty database
	stats, err := collector.GetProjectUsageStats(ctx, "week")
	require.NoError(t, err)

	assert.Equal(t, int64(0), stats["total_usage"])
	assert.Equal(t, "week", stats["period"])

	projects := stats["projects"].([]map[string]any)
	assert.Len(t, projects, 0)
}

func TestMetricsCollector_LargeDataset_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	collector, cleanup := setupMetricsCollector(t)
	defer cleanup()

	ctx := context.Background()
	userID := int64(123456789)

	// Record a large number of metrics to test performance
	const numMetrics = 1000

	start := time.Now()
	for i := 0; i < numMetrics; i++ {
		metrics := createTestCommandMetrics(userID, fmt.Sprintf("perf-cmd-%d", i), i%10 != 0, time.Duration(500+i%1000)*time.Millisecond)
		if i%3 == 0 {
			metrics.ProjectUsed = "TaqwaBoard"
		} else if i%3 == 1 {
			metrics.ProjectUsed = "CarLogbook"
		} else {
			metrics.ProjectUsed = "JDA"
		}

		err := collector.RecordCommandExecution(ctx, metrics)
		require.NoError(t, err)
	}
	insertTime := time.Since(start)

	// Test query performance
	start = time.Now()
	stats, err := collector.GetUsageStats(ctx, userID, "day")
	require.NoError(t, err)
	queryTime := time.Since(start)

	// Verify data
	assert.Equal(t, int64(numMetrics), stats["commands_executed"])
	assert.Equal(t, 0.9, stats["success_rate"]) // 90% success rate (i%10 != 0)

	// Performance assertions (these might need adjustment based on hardware)
	t.Logf("Insert time for %d metrics: %v", numMetrics, insertTime)
	t.Logf("Query time: %v", queryTime)

	assert.Less(t, insertTime, 10*time.Second, "Insert time should be reasonable")
	assert.Less(t, queryTime, 1*time.Second, "Query time should be fast")
}

// Helper function to unwrap the MetricsCollector for direct DB access in tests
func unwrapForTesting(t *testing.T, collector *metricscollector.MetricsCollector) *sql.DB {
	// This won't work with unexported fields, so we'll use an alternative approach
	t.Fatal("This method should not be called directly")
	return nil
}

// testHelpers is implemented in the test file to allow execution of SQL directly
type testHelpers interface {
	// ExecuteSQL allows executing SQL statements directly for testing purposes
	ExecuteSQL(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// Ensure MetricsCollector implements the testHelpers interface for testing
var _ testHelpers = &testCollector{}

// testCollector wraps MetricsCollector with test-specific methods
type testCollector struct {
	*metricscollector.MetricsCollector
	db *sql.DB
}

// ExecuteSQL provides direct SQL execution for testing
func (tc *testCollector) ExecuteSQL(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return tc.db.ExecContext(ctx, query, args...)
}

// setupTestCollector creates a MetricsCollector wrapped with test helpers
func setupTestCollector(t *testing.T) (*testCollector, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in tests
	}))

	// Open the database directly to have access to it
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)

	// Create the collector
	collector, err := metricscollector.NewMetricsCollector(dbPath, logger)
	require.NoError(t, err)

	// Create our test wrapper
	testColl := &testCollector{
		MetricsCollector: collector,
		db:               db,
	}

	cleanup := func() {
		collector.Close()
		db.Close()
	}

	return testColl, cleanup
}

// Helper functions for type conversion in tests
func toInt64(t *testing.T, v interface{}) int64 {
	switch i := v.(type) {
	case int:
		return int64(i)
	case int32:
		return int64(i)
	case int64:
		return i
	default:
		t.Fatalf("Expected integer type, got %T", v)
		return 0
	}
}

func toFloat64(t *testing.T, v interface{}) float64 {
	switch f := v.(type) {
	case float32:
		return float64(f)
	case float64:
		return f
	case int:
		return float64(f)
	case int64:
		return float64(f)
	default:
		t.Fatalf("Expected numeric type, got %T", v)
		return 0
	}
}
