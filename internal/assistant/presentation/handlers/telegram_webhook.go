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
		Date  int64  `json:"date"`
		Text  string `json:"text,omitempty"`
		Voice struct {
			FileID       string `json:"file_id"`
			FileUniqueID string `json:"file_unique_id"`
			Duration     int    `json:"duration"`
			MimeType     string `json:"mime_type,omitempty"`
			FileSize     int64  `json:"file_size,omitempty"`
		} `json:"voice,omitempty"`
	} `json:"message,omitempty"`
	CallbackQuery struct {
		ID   string `json:"id"`
		From struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username,omitempty"`
		} `json:"from"`
		Message struct {
			MessageID int64 `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
		Data string `json:"data"`
	} `json:"callback_query,omitempty"`
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

	// Process the update
	if err := h.processUpdate(ctx, &update); err != nil {
		h.logger.ErrorContext(ctx, "Failed to process update",
			"error", err,
			"update_id", update.UpdateID,
		)
		// Still return success to Telegram to avoid retries
		h.sendSuccessResponse(w, "Update processed with errors")
		return
	}

	// Log successful processing
	h.logger.InfoContext(ctx, "Telegram webhook processed successfully",
		"update_id", update.UpdateID,
		"processing_time", time.Since(startTime),
	)

	h.sendSuccessResponse(w, "Update processed successfully")
}

// processUpdate processes incoming Telegram update
func (h *TelegramWebhookHandler) processUpdate(ctx context.Context, update *TelegramUpdate) error {
	// Handle text message
	if update.Message.Text != "" {
		return h.processTextMessage(ctx, update)
	}

	// Handle voice message
	if update.Message.Voice.FileID != "" {
		return h.processVoiceMessage(ctx, update)
	}

	// Handle callback query (inline keyboard responses)
	if update.CallbackQuery.ID != "" {
		return h.processCallbackQuery(ctx, update)
	}

	// Unknown update type
	h.logger.WarnContext(ctx, "Received unknown update type",
		"update_id", update.UpdateID,
	)
	return nil
}

// processTextMessage processes text messages
func (h *TelegramWebhookHandler) processTextMessage(ctx context.Context, update *TelegramUpdate) error {
	userID := update.Message.From.ID

	// Check if user is authorized
	if !h.isUserAllowed(userID) {
		h.logger.WarnContext(ctx, "Unauthorized user attempted to use bot",
			"user_id", userID,
			"username", update.Message.From.Username,
		)
		return nil // Don't send error to unauthorized users
	}

	// Create command from message
	command := core.Command{
		ID:        fmt.Sprintf("msg_%d_%d", update.Message.MessageID, time.Now().Unix()),
		Text:      update.Message.Text,
		UserID:    userID,
		Timestamp: time.Unix(update.Message.Date, 0),
	}

	// Process command
	_, err := h.assistantService.ProcessCommand(ctx, command)
	return err
}

// processVoiceMessage processes voice messages
func (h *TelegramWebhookHandler) processVoiceMessage(ctx context.Context, update *TelegramUpdate) error {
	userID := update.Message.From.ID

	// Check if user is authorized
	if !h.isUserAllowed(userID) {
		return nil
	}

	// Create command from voice message
	command := core.Command{
		ID:          fmt.Sprintf("voice_%d_%d", update.Message.MessageID, time.Now().Unix()),
		UserID:      userID,
		AudioFileID: update.Message.Voice.FileID,
		Timestamp:   time.Unix(update.Message.Date, 0),
	}

	// Process audio command
	_, err := h.assistantService.ProcessAudioCommand(ctx, command)
	return err
}

// processCallbackQuery processes inline keyboard callbacks
func (h *TelegramWebhookHandler) processCallbackQuery(ctx context.Context, update *TelegramUpdate) error {
	userID := update.CallbackQuery.From.ID

	// Check if user is authorized
	if !h.isUserAllowed(userID) {
		return nil
	}

	h.logger.InfoContext(ctx, "Processing callback query",
		"user_id", userID,
		"callback_data", update.CallbackQuery.Data,
	)

	// Handle different callback types
	switch {
	case strings.HasPrefix(update.CallbackQuery.Data, "project_"):
		return h.handleProjectSelection(ctx, update)
	case strings.HasPrefix(update.CallbackQuery.Data, "confirm_"):
		return h.handleConfirmation(ctx, update)
	default:
		h.logger.WarnContext(ctx, "Unknown callback query data",
			"data", update.CallbackQuery.Data,
		)
	}

	return nil
}

// handleProjectSelection handles project selection from inline keyboard
func (h *TelegramWebhookHandler) handleProjectSelection(ctx context.Context, update *TelegramUpdate) error {
	// Extract project name from callback data
	projectName := strings.TrimPrefix(update.CallbackQuery.Data, "project_")

	// Create command to continue with selected project
	command := core.Command{
		ID:        fmt.Sprintf("callback_%s_%d", update.CallbackQuery.ID, time.Now().Unix()),
		Text:      projectName, // The selected project name
		UserID:    update.CallbackQuery.From.ID,
		Timestamp: time.Now(),
	}

	_, err := h.assistantService.ProcessCommand(ctx, command)
	return err
}

// handleConfirmation handles confirmation dialogs
func (h *TelegramWebhookHandler) handleConfirmation(ctx context.Context, update *TelegramUpdate) error {
	// Extract confirmation type and result
	confirmData := strings.TrimPrefix(update.CallbackQuery.Data, "confirm_")
	parts := strings.Split(confirmData, "_")

	if len(parts) < 2 {
		return fmt.Errorf("invalid confirmation data format")
	}

	confirmType := parts[0]
	result := parts[1] // "yes" or "no"

	h.logger.InfoContext(ctx, "Processing confirmation",
		"type", confirmType,
		"result", result,
		"user_id", update.CallbackQuery.From.ID,
	)

	// Create command for confirmation result
	command := core.Command{
		ID:        fmt.Sprintf("confirm_%s_%d", update.CallbackQuery.ID, time.Now().Unix()),
		Text:      fmt.Sprintf("confirmation:%s:%s", confirmType, result),
		UserID:    update.CallbackQuery.From.ID,
		Timestamp: time.Now(),
	}

	_, err := h.assistantService.ProcessCommand(ctx, command)
	return err
}

// isUserAllowed checks if user is in the allowed list
func (h *TelegramWebhookHandler) isUserAllowed(userID int64) bool {
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
	json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(response)
}
