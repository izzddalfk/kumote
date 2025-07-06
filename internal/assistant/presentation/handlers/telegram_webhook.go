package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// TelegramWebhookHandler handles incoming Telegram webhook requests
type TelegramWebhookHandler struct {
	assistantService core.AssistantService
	logger           *slog.Logger
	webhookSecret    string
	allowedUserIDs   []int64
}

// TelegramUpdate represents incoming Telegram update
type TelegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  struct {
		MessageID int64 `json:"message_id"`
		From      struct {
			ID           int64  `json:"id"`
			IsBot        bool   `json:"is_bot"`
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name,omitempty"`
			Username     string `json:"username,omitempty"`
			LanguageCode string `json:"language_code,omitempty"`
		} `json:"from"`
		Chat struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name,omitempty"`
			Username  string `json:"username,omitempty"`
			Type      string `json:"type"`
		} `json:"chat"`
		Date int64  `json:"date"`
		Text string `json:"text,omitempty"`
	} `json:"message,omitempty"`
}

// WebhookResponse represents response to Telegram webhook
type WebhookResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// NewTelegramWebhookHandler creates a new webhook handler
func NewTelegramWebhookHandler(
	service core.AssistantService,
	logger *slog.Logger,
	webhookSecret string,
	allowedUserIDs []int64,
) *TelegramWebhookHandler {
	return &TelegramWebhookHandler{
		assistantService: service,
		logger:           logger,
		webhookSecret:    webhookSecret,
		allowedUserIDs:   allowedUserIDs,
	}
}

// ServeHTTP handles HTTP requests for Telegram webhook
func (h *TelegramWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()

	// Log request
	h.logger.InfoContext(ctx, "Telegram webhook request received",
		"method", r.Method,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	)

	// Verify HTTP method
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}

	// Verify Content-Type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		h.sendErrorResponse(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to read request body", "error", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// Verify webhook signature if secret is configured
	if h.webhookSecret != "" {
		if !h.verifyWebhookSignature(r, body) {
			h.logger.WarnContext(ctx, "Invalid webhook signature",
				"remote_addr", r.RemoteAddr,
			)
			h.sendErrorResponse(w, http.StatusUnauthorized, "Invalid webhook signature")
			return
		}
	}

	// Parse Telegram update
	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		h.logger.ErrorContext(ctx, "Failed to parse Telegram update",
			"error", err,
			"body", string(body),
		)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Check if this is a text message
	if update.Message.Text == "" {
		h.logger.InfoContext(ctx, "Received non-text message, ignoring",
			"update_id", update.UpdateID,
		)
		h.sendSuccessResponse(w, "Non-text messages are not supported")
		return
	}

	// Process the text message
	result, err := h.processTextMessage(ctx, &update)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to process text message",
			"error", err,
			"update_id", update.UpdateID,
		)
		h.sendSuccessResponse(w, "Error processing message")
		return
	}

	if result == nil || !result.Success {
		errorMsg := "Unknown error"
		if result != nil && result.Error != "" {
			errorMsg = result.Error
		}
		h.logger.WarnContext(ctx, "Command processing failed",
			"update_id", update.UpdateID,
			"error", errorMsg,
		)
		h.sendSuccessResponse(w, fmt.Sprintf("Command processing failed: %s", errorMsg))
		return
	}

	// Log successful processing
	h.logger.InfoContext(ctx, "Telegram webhook processed successfully",
		"update_id", update.UpdateID,
		"processing_time", time.Since(startTime),
	)

	h.sendSuccessResponse(w, result.Response)
}

// processTextMessage processes text messages
func (h *TelegramWebhookHandler) processTextMessage(ctx context.Context, update *TelegramUpdate) (*core.QueryResult, error) {
	userID := update.Message.From.ID

	// Check if user is authorized
	if !h.isUserAllowed(userID) {
		h.logger.WarnContext(ctx, "Unauthorized user attempted to use bot",
			"user_id", userID,
			"username", update.Message.From.Username,
		)
		return &core.QueryResult{
			Success: false,
			Error:   "You are not authorized to use this assistant.",
		}, nil
	}

	// Validate the message text
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		return &core.QueryResult{
			Success: false,
			Error:   "Empty message received.",
		}, nil
	}

	// Create command from message
	command := core.Command{
		ID:        fmt.Sprintf("msg_%d_%d", update.Message.MessageID, time.Now().Unix()),
		Text:      text,
		UserID:    userID,
		Timestamp: time.Unix(update.Message.Date, 0),
	}

	// Process command
	result, err := h.assistantService.ProcessCommand(ctx, command)
	if err != nil {
		return &core.QueryResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to process command: %v", err),
		}, err
	}

	return result, nil
}

// isUserAllowed checks if user is in the allowed list
func (h *TelegramWebhookHandler) isUserAllowed(userID int64) bool {
	if len(h.allowedUserIDs) == 0 {
		// If no allowed users are configured, deny all requests
		h.logger.Warn("No allowed users configured, denying all requests")
		return false
	}

	for _, allowedID := range h.allowedUserIDs {
		if allowedID == userID {
			return true
		}
	}
	return false
}

// verifyWebhookSignature verifies Telegram webhook secret token
func (h *TelegramWebhookHandler) verifyWebhookSignature(r *http.Request, body []byte) bool {
	token := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if token == "" {
		return false
	}

	// For Telegram webhooks, we simply compare the token with our configured secret
	// This is a string comparison, not an HMAC verification
	return token == h.webhookSecret
}

// sendSuccessResponse sends successful response to Telegram
func (h *TelegramWebhookHandler) sendSuccessResponse(w http.ResponseWriter, message string) {
	response := WebhookResponse{
		Success:   true,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
	}
}

// sendErrorResponse sends error response to Telegram
func (h *TelegramWebhookHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := WebhookResponse{
		Success:   false,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode error response", "error", err)
	}
}
