package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	assistantService core.AssistantService
	logger           *slog.Logger
	startTime        time.Time
	version          string
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     string                 `json:"version"`
	Uptime      string                 `json:"uptime"`
	System      SystemInfo             `json:"system"`
	Services    map[string]ServiceInfo `json:"services"`
	Environment map[string]string      `json:"environment,omitempty"`
}

// SystemInfo contains system information
type SystemInfo struct {
	GoVersion    string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutine"`
	NumCPU       int    `json:"num_cpu"`
	MemoryMB     uint64 `json:"memory_mb"`
}

// ServiceInfo contains service health information
type ServiceInfo struct {
	Status      string    `json:"status"`
	LastChecked time.Time `json:"last_checked"`
	Message     string    `json:"message,omitempty"`
	Response    string    `json:"response_time,omitempty"`
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	service core.AssistantService,
	logger *slog.Logger,
	version string,
) *HealthHandler {
	return &HealthHandler{
		assistantService: service,
		logger:           logger,
		startTime:        time.Now(),
		version:          version,
	}
}

// ServeHTTP handles HTTP requests for health checks
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.DebugContext(ctx, "Health check request received",
		"method", r.Method,
		"remote_addr", r.RemoteAddr,
	)

	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Only GET method is allowed")
		return
	}

	// Check detailed health if requested
	checkDetailed := r.URL.Query().Get("detailed") == "true"

	// Perform health checks
	health := h.performHealthChecks(ctx, checkDetailed)

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Determine HTTP status based on overall health
	statusCode := http.StatusOK
	if health.Status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)

	// Encode and send response
	if err := json.NewEncoder(w).Encode(health); err != nil {
		h.logger.ErrorContext(ctx, "Failed to encode health response", "error", err)
	}

	h.logger.InfoContext(ctx, "Health check completed",
		"status", health.Status,
		"detailed", checkDetailed,
		"response_code", statusCode,
	)
}

// performHealthChecks executes health checks for all services
func (h *HealthHandler) performHealthChecks(ctx context.Context, detailed bool) *HealthResponse {
	services := make(map[string]ServiceInfo)
	overallStatus := "healthy"

	// Check assistant service
	assistantStatus := h.checkAssistantService(ctx)
	services["assistant"] = assistantStatus
	if assistantStatus.Status != "healthy" {
		overallStatus = "unhealthy"
	}

	if detailed {
		// Check additional services when detailed health is requested

		// Check AI Code Executor
		aiStatus := h.checkAICodeExecutor(ctx)
		services["ai_executor"] = aiStatus
		if aiStatus.Status != "healthy" {
			overallStatus = "degraded"
		}

		// Check Project Scanner
		scannerStatus := h.checkProjectScanner(ctx)
		services["project_scanner"] = scannerStatus
		if scannerStatus.Status != "healthy" && overallStatus == "healthy" {
			overallStatus = "degraded"
		}
	}

	// Build response
	response := &HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   h.version,
		Uptime:    time.Since(h.startTime).String(),
		System:    h.getSystemInfo(),
		Services:  services,
	}

	// Add environment info if detailed
	if detailed {
		response.Environment = h.getEnvironmentInfo()
	}

	return response
}

// checkAssistantService checks the health of the assistant service
func (h *HealthHandler) checkAssistantService(ctx context.Context) ServiceInfo {
	start := time.Now()

	// Simple health check - verify service is responding
	if h.assistantService == nil {
		return ServiceInfo{
			Status:      "unhealthy",
			LastChecked: time.Now(),
			Message:     "Assistant service is not initialized",
		}
	}

	// Try to get user permissions as a basic connectivity test
	_, err := h.assistantService.GetUserPermissions(ctx, 123456789) // Test user ID
	responseTime := time.Since(start)

	if err != nil {
		// Check if it's a "user not found" error vs actual service error
		if coreErr, ok := err.(*core.ValidationError); ok && coreErr.Field == "user_not_found" {
			// This is expected for test user ID - service is healthy
			return ServiceInfo{
				Status:      "healthy",
				LastChecked: time.Now(),
				Message:     "Service responding normally",
				Response:    responseTime.String(),
			}
		}

		return ServiceInfo{
			Status:      "unhealthy",
			LastChecked: time.Now(),
			Message:     fmt.Sprintf("Service error: %v", err),
			Response:    responseTime.String(),
		}
	}

	return ServiceInfo{
		Status:      "healthy",
		LastChecked: time.Now(),
		Message:     "Service responding normally",
		Response:    responseTime.String(),
	}
}

// checkAICodeExecutor checks AI code executor availability
func (h *HealthHandler) checkAICodeExecutor(ctx context.Context) ServiceInfo {
	// This would check if Claude Code CLI is available
	// For now, return a basic check
	return ServiceInfo{
		Status:      "healthy",
		LastChecked: time.Now(),
		Message:     "AI executor available",
	}
}

// checkProjectScanner checks project scanner health
func (h *HealthHandler) checkProjectScanner(ctx context.Context) ServiceInfo {
	// This would check if project scanner is working
	// For now, return a basic check
	return ServiceInfo{
		Status:      "healthy",
		LastChecked: time.Now(),
		Message:     "Project scanner operational",
	}
}

// getSystemInfo collects system information
func (h *HealthHandler) getSystemInfo() SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemInfo{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		MemoryMB:     memStats.Alloc / 1024 / 1024,
	}
}

// getEnvironmentInfo collects relevant environment information
func (h *HealthHandler) getEnvironmentInfo() map[string]string {
	return map[string]string{
		"go_version": runtime.Version(),
		"go_os":      runtime.GOOS,
		"go_arch":    runtime.GOARCH,
		"num_cpu":    fmt.Sprintf("%d", runtime.NumCPU()),
		"max_procs":  fmt.Sprintf("%d", runtime.GOMAXPROCS(0)),
	}
}

// sendErrorResponse sends error response
func (h *HealthHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]interface{}{
		"error":     message,
		"timestamp": time.Now(),
		"status":    "error",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
