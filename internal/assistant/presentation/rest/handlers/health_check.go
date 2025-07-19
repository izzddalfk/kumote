package handlers

import (
	"log/slog"
	"net/http"

	"github.com/izzddalfk/kumote/internal/assistant/core"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	assistantService core.AssistantService
	logger           *slog.Logger
	version          string
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
		version:          version,
	}
}

// ServeHTTP handles HTTP requests for health checks
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Write simple OK response
	w.Write([]byte(`{"status":"OK"}`))
}
