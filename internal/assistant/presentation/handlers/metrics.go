package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// MetricsHandler handles metrics collection and reporting
type MetricsHandler struct {
	assistantService core.AssistantService
	logger           *slog.Logger
	startTime        time.Time
}

// MetricsResponse represents metrics response
type MetricsResponse struct {
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	System    SystemMetrics          `json:"system"`
	Service   ServiceMetrics         `json:"service"`
	Usage     UsageMetrics           `json:"usage"`
	Errors    ErrorMetrics           `json:"errors"`
	Custom    map[string]interface{} `json:"custom,omitempty"`
}

// SystemMetrics contains system-level metrics
type SystemMetrics struct {
	GoVersion     string  `json:"go_version"`
	NumGoroutine  int     `json:"num_goroutine"`
	NumCPU        int     `json:"num_cpu"`
	MemoryUsageMB float64 `json:"memory_usage_mb"`
	MemoryAllocMB float64 `json:"memory_alloc_mb"`
	GCPauses      uint32  `json:"gc_pauses"`
	NextGC        uint64  `json:"next_gc"`
}

// ServiceMetrics contains service-specific metrics
type ServiceMetrics struct {
	CommandsProcessed      int64   `json:"commands_processed"`
	SuccessfulCommands     int64   `json:"successful_commands"`
	FailedCommands         int64   `json:"failed_commands"`
	AverageResponseTimeMs  float64 `json:"average_response_time_ms"`
	ActiveUsers            int     `json:"active_users"`
	ProjectsDiscovered     int     `json:"projects_discovered"`
	LastProjectScan        string  `json:"last_project_scan"`
	AudioCommandsProcessed int64   `json:"audio_commands_processed"`
}

// UsageMetrics contains usage statistics
type UsageMetrics struct {
	RequestsPerMinute   float64           `json:"requests_per_minute"`
	TopUsers            []UserMetric      `json:"top_users"`
	TopCommands         []CommandMetric   `json:"top_commands"`
	TopProjects         []ProjectMetric   `json:"top_projects"`
	ResponseTimeDistrib []ResponseTimeBin `json:"response_time_distribution"`
	HourlyDistribution  []HourlyMetric    `json:"hourly_distribution"`
}

// ErrorMetrics contains error statistics
type ErrorMetrics struct {
	TotalErrors       int64            `json:"total_errors"`
	ErrorsLast24Hours int64            `json:"errors_last_24_hours"`
	ErrorsByType      map[string]int64 `json:"errors_by_type"`
	RecentErrors      []RecentError    `json:"recent_errors"`
	CriticalErrors    int64            `json:"critical_errors"`
}

// UserMetric represents user usage metrics
type UserMetric struct {
	UserID         int64   `json:"user_id"`
	CommandCount   int64   `json:"command_count"`
	LastActiveTime string  `json:"last_active_time"`
	SuccessRate    float64 `json:"success_rate"`
}

// CommandMetric represents command usage metrics
type CommandMetric struct {
	Command     string  `json:"command"`
	Count       int64   `json:"count"`
	AvgDuration float64 `json:"avg_duration_ms"`
	SuccessRate float64 `json:"success_rate"`
}

// ProjectMetric represents project usage metrics
type ProjectMetric struct {
	ProjectName string  `json:"project_name"`
	AccessCount int64   `json:"access_count"`
	LastAccess  string  `json:"last_access"`
	AvgDuration float64 `json:"avg_duration_ms"`
}

// ResponseTimeBin represents response time distribution
type ResponseTimeBin struct {
	MinMs float64 `json:"min_ms"`
	MaxMs float64 `json:"max_ms"`
	Count int64   `json:"count"`
}

// HourlyMetric represents hourly usage distribution
type HourlyMetric struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// RecentError represents a recent error
type RecentError struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	UserID    int64  `json:"user_id,omitempty"`
	Command   string `json:"command,omitempty"`
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(
	service core.AssistantService,
	logger *slog.Logger,
) *MetricsHandler {
	return &MetricsHandler{
		assistantService: service,
		logger:           logger,
		startTime:        time.Now(),
	}
}

// ServeHTTP handles HTTP requests for metrics
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.DebugContext(ctx, "Metrics request received",
		"method", r.Method,
		"remote_addr", r.RemoteAddr,
	)

	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Only GET method is allowed")
		return
	}

	// Collect metrics
	metrics := h.collectMetrics(ctx)

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	// Encode and send response
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		h.logger.ErrorContext(ctx, "Failed to encode metrics response", "error", err)
	}

	h.logger.InfoContext(ctx, "Metrics response sent successfully")
}

// collectMetrics gathers all metrics data
func (h *MetricsHandler) collectMetrics(ctx context.Context) *MetricsResponse {
	return &MetricsResponse{
		Timestamp: time.Now(),
		Uptime:    time.Since(h.startTime).String(),
		System:    h.collectSystemMetrics(),
		Service:   h.collectServiceMetrics(ctx),
		Usage:     h.collectUsageMetrics(ctx),
		Errors:    h.collectErrorMetrics(ctx),
		Custom:    h.collectCustomMetrics(ctx),
	}
}

// collectSystemMetrics collects system-level metrics
func (h *MetricsHandler) collectSystemMetrics() SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemMetrics{
		GoVersion:     runtime.Version(),
		NumGoroutine:  runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		MemoryUsageMB: float64(memStats.Sys) / 1024 / 1024,
		MemoryAllocMB: float64(memStats.Alloc) / 1024 / 1024,
		GCPauses:      memStats.NumGC,
		NextGC:        memStats.NextGC,
	}
}

// collectServiceMetrics collects service-specific metrics
func (h *MetricsHandler) collectServiceMetrics(ctx context.Context) ServiceMetrics {
	// In a real implementation, these would come from the metrics collector
	// For now, return mock data
	return ServiceMetrics{
		CommandsProcessed:      1250,
		SuccessfulCommands:     1198,
		FailedCommands:         52,
		AverageResponseTimeMs:  320.5,
		ActiveUsers:            8,
		ProjectsDiscovered:     15,
		LastProjectScan:        time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		AudioCommandsProcessed: 45,
	}
}

// collectUsageMetrics collects usage statistics
func (h *MetricsHandler) collectUsageMetrics(ctx context.Context) UsageMetrics {
	// Mock data - in real implementation, this would come from metrics store
	return UsageMetrics{
		RequestsPerMinute: 12.3,
		TopUsers: []UserMetric{
			{
				UserID:         123456789,
				CommandCount:   450,
				LastActiveTime: time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
				SuccessRate:    96.2,
			},
			{
				UserID:         987654321,
				CommandCount:   380,
				LastActiveTime: time.Now().Add(-25 * time.Minute).Format(time.RFC3339),
				SuccessRate:    94.8,
			},
		},
		TopCommands: []CommandMetric{
			{
				Command:     "git status",
				Count:       156,
				AvgDuration: 280.5,
				SuccessRate: 98.7,
			},
			{
				Command:     "show main.go",
				Count:       134,
				AvgDuration: 150.2,
				SuccessRate: 97.8,
			},
		},
		TopProjects: []ProjectMetric{
			{
				ProjectName: "TaqwaBoard",
				AccessCount: 89,
				LastAccess:  time.Now().Add(-15 * time.Minute).Format(time.RFC3339),
				AvgDuration: 340.2,
			},
			{
				ProjectName: "CarLogbook",
				AccessCount: 67,
				LastAccess:  time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
				AvgDuration: 290.8,
			},
		},
		ResponseTimeDistrib: []ResponseTimeBin{
			{MinMs: 0, MaxMs: 100, Count: 45},
			{MinMs: 100, MaxMs: 300, Count: 234},
			{MinMs: 300, MaxMs: 500, Count: 156},
			{MinMs: 500, MaxMs: 1000, Count: 78},
			{MinMs: 1000, MaxMs: 5000, Count: 23},
		},
		HourlyDistribution: generateHourlyDistribution(),
	}
}

// collectErrorMetrics collects error statistics
func (h *MetricsHandler) collectErrorMetrics(ctx context.Context) ErrorMetrics {
	return ErrorMetrics{
		TotalErrors:       52,
		ErrorsLast24Hours: 12,
		ErrorsByType: map[string]int64{
			"validation_error":  23,
			"timeout_error":     15,
			"file_not_found":    8,
			"permission_denied": 4,
			"network_error":     2,
		},
		RecentErrors: []RecentError{
			{
				Timestamp: time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
				Type:      "validation_error",
				Message:   "Invalid project path specified",
				UserID:    123456789,
				Command:   "show invalid/path",
			},
			{
				Timestamp: time.Now().Add(-45 * time.Minute).Format(time.RFC3339),
				Type:      "timeout_error",
				Message:   "Command execution timeout",
				UserID:    987654321,
				Command:   "git push origin main",
			},
		},
		CriticalErrors: 2,
	}
}

// collectCustomMetrics collects custom application metrics
func (h *MetricsHandler) collectCustomMetrics(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"telegram_webhook_calls": 1250,
		"project_shortcuts_used": 890,
		"claude_code_executions": 1150,
		"cache_hit_rate":         87.3,
		"avg_project_scan_time":  "2.3s",
		"webhook_response_time":  "45ms",
	}
}

// generateHourlyDistribution generates mock hourly distribution data
func generateHourlyDistribution() []HourlyMetric {
	distribution := make([]HourlyMetric, 24)
	for i := 0; i < 24; i++ {
		// Simulate typical usage patterns (higher during work hours)
		count := int64(5) // Base count
		if i >= 8 && i <= 17 {
			count += int64(i * 3) // Work hours get more traffic
		}
		if i >= 19 && i <= 22 {
			count += int64(10) // Evening spike
		}

		distribution[i] = HourlyMetric{
			Hour:  i,
			Count: count,
		}
	}
	return distribution
}

// sendErrorResponse sends error response
func (h *MetricsHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
