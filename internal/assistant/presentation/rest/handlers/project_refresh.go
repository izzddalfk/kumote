package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/izzddalfk/kumote/internal/assistant/core"
)

// ProjectRefreshHandler handles manual project refresh requests
type ProjectRefreshHandler struct {
	assistantService core.AssistantService
	logger           *slog.Logger
	allowedUserIDs   []int64
}

// RefreshRequest represents a refresh request
type RefreshRequest struct {
	UserID int64  `json:"user_id"`
	Force  bool   `json:"force,omitempty"`
	Path   string `json:"path,omitempty"`
}

// RefreshResponse represents refresh response
type RefreshResponse struct {
	Success      bool                   `json:"success"`
	Message      string                 `json:"message"`
	ProjectCount int                    `json:"project_count,omitempty"`
	Projects     []ProjectSummary       `json:"projects,omitempty"`
	Duration     string                 `json:"duration"`
	Timestamp    time.Time              `json:"timestamp"`
	Errors       []string               `json:"errors,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ProjectSummary represents a summary of discovered project
type ProjectSummary struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Type         string            `json:"type"`
	TechStack    []string          `json:"tech_stack"`
	LastModified time.Time         `json:"last_modified"`
	Shortcuts    []string          `json:"shortcuts"`
	FileCount    int               `json:"file_count"`
	SizeBytes    int64             `json:"size_bytes"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// NewProjectRefreshHandler creates a new project refresh handler
func NewProjectRefreshHandler(
	service core.AssistantService,
	logger *slog.Logger,
	allowedUserIDs []int64,
) *ProjectRefreshHandler {
	return &ProjectRefreshHandler{
		assistantService: service,
		logger:           logger,
		allowedUserIDs:   allowedUserIDs,
	}
}

// ServeHTTP handles HTTP requests for project refresh
func (h *ProjectRefreshHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()

	h.logger.InfoContext(ctx, "Project refresh request received",
		"method", r.Method,
		"remote_addr", r.RemoteAddr,
	)

	// Only allow POST requests
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}

	// Parse request
	var req RefreshRequest
	if err := h.parseRequest(r, &req); err != nil {
		h.logger.ErrorContext(ctx, "Failed to parse refresh request", "error", err)
		h.sendErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Validate user authorization
	if !h.isUserAllowed(req.UserID) {
		h.logger.WarnContext(ctx, "Unauthorized project refresh attempt",
			"user_id", req.UserID,
			"remote_addr", r.RemoteAddr,
		)
		h.sendErrorResponse(w, http.StatusForbidden, "User not authorized")
		return
	}

	// Perform project refresh
	response := h.performRefresh(ctx, &req, startTime)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	statusCode := http.StatusOK
	if !response.Success {
		statusCode = http.StatusInternalServerError
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.ErrorContext(ctx, "Failed to encode refresh response", "error", err)
	}

	h.logger.InfoContext(ctx, "Project refresh completed",
		"success", response.Success,
		"project_count", response.ProjectCount,
		"duration", response.Duration,
		"user_id", req.UserID,
	)
}

// parseRequest parses the incoming refresh request
func (h *ProjectRefreshHandler) parseRequest(r *http.Request, req *RefreshRequest) error {
	// Handle different content types
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Parse JSON request
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return fmt.Errorf("failed to decode JSON: %w", err)
		}
	} else {
		// Parse form data or query parameters
		if err := r.ParseForm(); err != nil {
			return fmt.Errorf("failed to parse form: %w", err)
		}

		// Extract user_id
		if userIDStr := r.FormValue("user_id"); userIDStr != "" {
			userID, err := strconv.ParseInt(userIDStr, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid user_id: %w", err)
			}
			req.UserID = userID
		}

		// Extract force flag
		req.Force = r.FormValue("force") == "true"

		// Extract path
		req.Path = r.FormValue("path")
	}

	// Validate required fields
	if req.UserID == 0 {
		return fmt.Errorf("user_id is required")
	}

	return nil
}

// performRefresh performs the actual project refresh
func (h *ProjectRefreshHandler) performRefresh(ctx context.Context, req *RefreshRequest, startTime time.Time) *RefreshResponse {
	response := &RefreshResponse{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	h.logger.InfoContext(ctx, "Starting project refresh",
		"user_id", req.UserID,
		"force", req.Force,
		"path", req.Path,
	)

	// TODO: Implement actual project refresh logic
	// This would call the project scanner service to refresh the project index
	// For now, return a mock response

	// Simulate refresh process
	time.Sleep(500 * time.Millisecond) // Simulate processing time

	// Mock project data
	projects := []ProjectSummary{
		{
			Name:         "TaqwaBoard",
			Path:         "~/Development/taqwaboard",
			Type:         "Go Web Application",
			TechStack:    []string{"Go", "Vue.js", "PostgreSQL"},
			LastModified: time.Now().Add(-2 * time.Hour),
			Shortcuts:    []string{"taqwa"},
			FileCount:    45,
			SizeBytes:    2048576,
			Metadata: map[string]string{
				"go_version": "1.21",
				"main_file":  "main.go",
			},
		},
		{
			Name:         "CarLogbook",
			Path:         "~/Development/car-logbook",
			Type:         "Vue.js Application",
			TechStack:    []string{"Vue.js", "Node.js", "JavaScript"},
			LastModified: time.Now().Add(-1 * time.Hour),
			Shortcuts:    []string{"car"},
			FileCount:    38,
			SizeBytes:    1536000,
			Metadata: map[string]string{
				"node_version": "18.17.0",
				"package_file": "package.json",
			},
		},
	}

	response.Success = true
	response.Message = "Project refresh completed successfully"
	response.ProjectCount = len(projects)
	response.Projects = projects
	response.Duration = time.Since(startTime).String()

	// Add metadata
	response.Metadata["scan_path"] = req.Path
	response.Metadata["force_refresh"] = req.Force
	response.Metadata["scan_method"] = "automatic"
	response.Metadata["total_files_scanned"] = 83

	h.logger.InfoContext(ctx, "Project refresh completed",
		"projects_found", len(projects),
		"scan_duration", response.Duration,
	)

	return response
}

// isUserAllowed checks if user is authorized for project refresh
func (h *ProjectRefreshHandler) isUserAllowed(userID int64) bool {
	for _, allowedID := range h.allowedUserIDs {
		if allowedID == userID {
			return true
		}
	}
	return false
}

// sendErrorResponse sends error response
func (h *ProjectRefreshHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := RefreshResponse{
		Success:   false,
		Message:   message,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
