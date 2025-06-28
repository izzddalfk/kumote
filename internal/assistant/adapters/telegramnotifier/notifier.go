// internal/assistant/adapters/telegramnotifier/bot.go
package telegramnotifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// TelegramNotifier implements notification sending via Telegram Bot API
type TelegramNotifier struct {
	botToken   string
	apiBaseURL string
	httpClient *http.Client
	logger     *slog.Logger
	config     NotifierConfig
}

// NotifierConfig holds configuration for Telegram notifications
type NotifierConfig struct {
	MaxMessageLength int           // Maximum message length (Telegram limit: 4096)
	RequestTimeout   time.Duration // HTTP request timeout
	MaxRetries       int           // Maximum retry attempts
	RetryDelay       time.Duration // Delay between retries
	EnablePreview    bool          // Enable link previews in messages
	ParseMode        string        // Default parse mode (HTML, Markdown, MarkdownV2)
}

// TelegramResponse represents a response from Telegram Bot API
type TelegramResponse struct {
	OK          bool        `json:"ok"`
	Result      interface{} `json:"result,omitempty"`
	ErrorCode   int         `json:"error_code,omitempty"`
	Description string      `json:"description,omitempty"`
}

// SendMessageRequest represents a Telegram sendMessage API request
type SendMessageRequest struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
	DisableNotification   bool   `json:"disable_notification,omitempty"`
}

// SendDocumentRequest represents a Telegram sendDocument API request
type SendDocumentRequest struct {
	ChatID              int64  `json:"chat_id"`
	Document            string `json:"document"` // file_id or URL
	Caption             string `json:"caption,omitempty"`
	ParseMode           string `json:"parse_mode,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// InlineKeyboardMarkup represents an inline keyboard
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton represents a button in an inline keyboard
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// NewTelegramNotifier creates a new Telegram notifier
func NewTelegramNotifier(botToken string, logger *slog.Logger) (*TelegramNotifier, error) {
	if botToken == "" {
		return nil, fmt.Errorf("bot token cannot be empty")
	}

	config := NotifierConfig{
		MaxMessageLength: core.TelegramMaxMessageLength,
		RequestTimeout:   30 * time.Second,
		MaxRetries:       3,
		RetryDelay:       1 * time.Second,
		EnablePreview:    false,
		ParseMode:        core.ParseModeHTML,
	}

	httpClient := &http.Client{
		Timeout: config.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			MaxIdleConnsPerHost: 2,
		},
	}

	tn := &TelegramNotifier{
		botToken:   botToken,
		apiBaseURL: "https://api.telegram.org/bot" + botToken,
		httpClient: httpClient,
		logger:     logger,
		config:     config,
	}

	logger.InfoContext(context.Background(), "Telegram notifier initialized",
		"api_base_url", fmt.Sprintf("https://api.telegram.org/bot%s***", botToken[:10]),
		"max_message_length", config.MaxMessageLength,
	)

	return tn, nil
}

// SetConfig updates the notifier configuration
func (tn *TelegramNotifier) SetConfig(config NotifierConfig) {
	tn.config = config
	tn.httpClient.Timeout = config.RequestTimeout

	tn.logger.InfoContext(context.Background(), "Telegram notifier configuration updated",
		"max_message_length", config.MaxMessageLength,
		"request_timeout", config.RequestTimeout,
		"max_retries", config.MaxRetries,
	)
}

// SendMessage sends a text message to user
func (tn *TelegramNotifier) SendMessage(ctx context.Context, userID int64, message string) error {
	return tn.SendFormattedMessage(ctx, userID, message, tn.config.ParseMode)
}

// SendFormattedMessage sends a message with specific formatting
func (tn *TelegramNotifier) SendFormattedMessage(ctx context.Context, userID int64, message string, parseMode string) error {
	tn.logger.DebugContext(ctx, "Sending formatted message",
		"user_id", userID,
		"message_length", len(message),
		"parse_mode", parseMode,
	)

	// Validate message length and split if necessary
	messages := tn.splitLongMessage(message)

	for i, msg := range messages {
		if err := tn.sendSingleMessage(ctx, userID, msg, parseMode); err != nil {
			return fmt.Errorf("failed to send message part %d/%d: %w", i+1, len(messages), err)
		}

		// Add small delay between messages to avoid rate limiting
		if i < len(messages)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	tn.logger.InfoContext(ctx, "Formatted message sent successfully",
		"user_id", userID,
		"parts", len(messages),
		"total_length", len(message),
	)

	return nil
}

// SendFile sends a file to user
func (tn *TelegramNotifier) SendFile(ctx context.Context, userID int64, file io.Reader, filename string) error {
	tn.logger.DebugContext(ctx, "Sending file",
		"user_id", userID,
		"filename", filename,
	)

	// Read file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	if len(fileContent) == 0 {
		return fmt.Errorf("file content is empty")
	}

	// Check file size (Telegram limit: 50MB for bots)
	const maxFileSize = 50 * 1024 * 1024 // 50MB
	if len(fileContent) > maxFileSize {
		return fmt.Errorf("file size %d exceeds Telegram limit of %d bytes", len(fileContent), maxFileSize)
	}

	// Create multipart form
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add chat_id field
	if err := writer.WriteField("chat_id", fmt.Sprintf("%d", userID)); err != nil {
		return fmt.Errorf("failed to write chat_id field: %w", err)
	}

	// Add file field
	fileWriter, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := fileWriter.Write(fileContent); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	// Add caption if filename suggests it's a code file
	if tn.isCodeFile(filename) {
		caption := fmt.Sprintf("📄 <b>%s</b>", tn.escapeHTML(filename))
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("failed to write caption field: %w", err)
		}
		if err := writer.WriteField("parse_mode", "HTML"); err != nil {
			return fmt.Errorf("failed to write parse_mode field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Send request with retries
	url := tn.apiBaseURL + "/sendDocument"
	contentType := writer.FormDataContentType()

	err = tn.executeWithRetry(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, "POST", url, &requestBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", contentType)
		return tn.executeRequest(req)
	})

	if err != nil {
		tn.logger.ErrorContext(ctx, "Failed to send file",
			"user_id", userID,
			"filename", filename,
			"error", err.Error(),
		)
		return err
	}

	tn.logger.InfoContext(ctx, "File sent successfully",
		"user_id", userID,
		"filename", filename,
		"size", len(fileContent),
	)

	return nil
}

// SendConfirmationRequest sends a confirmation request with inline keyboard
func (tn *TelegramNotifier) SendConfirmationRequest(ctx context.Context, userID int64, message string, options []string) error {
	tn.logger.DebugContext(ctx, "Sending confirmation request",
		"user_id", userID,
		"options", len(options),
	)

	if len(options) == 0 {
		return fmt.Errorf("options cannot be empty")
	}

	if len(options) > 10 {
		return fmt.Errorf("too many options, maximum is 10")
	}

	// Create inline keyboard
	keyboard := tn.createInlineKeyboard(options)

	// Prepare request
	request := struct {
		ChatID      int64                 `json:"chat_id"`
		Text        string                `json:"text"`
		ParseMode   string                `json:"parse_mode"`
		ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup"`
	}{
		ChatID:      userID,
		Text:        message,
		ParseMode:   tn.config.ParseMode,
		ReplyMarkup: keyboard,
	}

	// Send request with retries
	err := tn.executeWithRetry(ctx, func() error {
		return tn.sendJSONRequest(ctx, "/sendMessage", request)
	})

	if err != nil {
		tn.logger.ErrorContext(ctx, "Failed to send confirmation request",
			"user_id", userID,
			"error", err.Error(),
		)
		return err
	}

	tn.logger.InfoContext(ctx, "Confirmation request sent successfully",
		"user_id", userID,
		"options", len(options),
	)

	return nil
}

// GetBotInfo retrieves bot information for validation
func (tn *TelegramNotifier) GetBotInfo(ctx context.Context) (map[string]interface{}, error) {
	tn.logger.DebugContext(ctx, "Getting bot info")

	var response TelegramResponse
	err := tn.executeWithRetry(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", tn.apiBaseURL+"/getMe", nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := tn.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if !response.OK {
			return fmt.Errorf("telegram API error: %s", response.Description)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}

	botInfo, ok := response.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	tn.logger.InfoContext(ctx, "Bot info retrieved successfully",
		"bot_username", botInfo["username"],
		"bot_id", botInfo["id"],
	)

	return botInfo, nil
}

// ValidateConnection validates the bot token and connection
func (tn *TelegramNotifier) ValidateConnection(ctx context.Context) error {
	tn.logger.InfoContext(ctx, "Validating Telegram bot connection")

	_, err := tn.GetBotInfo(ctx)
	if err != nil {
		return fmt.Errorf("telegram connection validation failed: %w", err)
	}

	tn.logger.InfoContext(ctx, "Telegram bot connection validated successfully")
	return nil
}

// Helper methods

// sendSingleMessage sends a single message (internal use)
func (tn *TelegramNotifier) sendSingleMessage(ctx context.Context, userID int64, message, parseMode string) error {
	request := SendMessageRequest{
		ChatID:                userID,
		Text:                  message,
		ParseMode:             parseMode,
		DisableWebPagePreview: !tn.config.EnablePreview,
		DisableNotification:   false,
	}

	return tn.executeWithRetry(ctx, func() error {
		return tn.sendJSONRequest(ctx, "/sendMessage", request)
	})
}

// sendJSONRequest sends a JSON request to Telegram API
func (tn *TelegramNotifier) sendJSONRequest(ctx context.Context, endpoint string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tn.apiBaseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return tn.executeRequest(req)
}

// executeRequest executes HTTP request and handles response
func (tn *TelegramNotifier) executeRequest(req *http.Request) error {
	resp, err := tn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var response TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.OK {
		// Handle specific Telegram errors
		switch response.ErrorCode {
		case 400:
			return core.NewValidationError("telegram_request", response.Description)
		case 401:
			return fmt.Errorf("unauthorized: invalid bot token")
		case 403:
			return fmt.Errorf("forbidden: bot was blocked by user or chat not found")
		case 429:
			return fmt.Errorf("rate limited: %s", response.Description)
		default:
			return fmt.Errorf("telegram API error (%d): %s", response.ErrorCode, response.Description)
		}
	}

	return nil
}

// executeWithRetry executes operation with retry logic
func (tn *TelegramNotifier) executeWithRetry(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= tn.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(tn.config.RetryDelay * time.Duration(attempt)):
				// Continue with retry
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = operation()
		if lastErr == nil {
			return nil // Success
		}

		// Don't retry on certain errors
		if strings.Contains(lastErr.Error(), "unauthorized") ||
			strings.Contains(lastErr.Error(), "forbidden") ||
			strings.Contains(lastErr.Error(), "validation error") {
			break
		}

		tn.logger.WarnContext(ctx, "Operation failed, retrying",
			"attempt", attempt+1,
			"max_retries", tn.config.MaxRetries,
			"error", lastErr.Error(),
		)
	}

	return lastErr
}

// splitLongMessage splits long messages into chunks
func (tn *TelegramNotifier) splitLongMessage(message string) []string {
	if len(message) <= tn.config.MaxMessageLength {
		return []string{message}
	}

	var messages []string
	remaining := message

	for len(remaining) > 0 {
		chunk := remaining
		if len(chunk) > tn.config.MaxMessageLength {
			// Find a good break point (prefer line breaks)
			breakPoint := tn.config.MaxMessageLength
			for i := tn.config.MaxMessageLength - 1; i > tn.config.MaxMessageLength/2; i-- {
				if remaining[i] == '\n' || remaining[i] == ' ' {
					breakPoint = i
					break
				}
			}
			chunk = remaining[:breakPoint]
		}

		messages = append(messages, chunk)
		remaining = remaining[len(chunk):]

		// Skip leading whitespace in next chunk
		remaining = strings.TrimLeft(remaining, " \n")
	}

	return messages
}

// createInlineKeyboard creates inline keyboard from options
func (tn *TelegramNotifier) createInlineKeyboard(options []string) *InlineKeyboardMarkup {
	var rows [][]InlineKeyboardButton

	// Create buttons in rows (max 3 per row for better UX)
	const maxButtonsPerRow = 3
	for i := 0; i < len(options); i += maxButtonsPerRow {
		end := i + maxButtonsPerRow
		if end > len(options) {
			end = len(options)
		}

		var row []InlineKeyboardButton
		for j := i; j < end; j++ {
			button := InlineKeyboardButton{
				Text:         options[j],
				CallbackData: fmt.Sprintf("option_%d", j),
			}
			row = append(row, button)
		}
		rows = append(rows, row)
	}

	return &InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

// isCodeFile checks if filename suggests it's a code file
func (tn *TelegramNotifier) isCodeFile(filename string) bool {
	codeExtensions := []string{
		".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp",
		".cs", ".php", ".rb", ".rs", ".swift", ".kt", ".scala",
		".html", ".css", ".scss", ".vue", ".jsx", ".tsx",
		".json", ".yaml", ".yml", ".xml", ".toml", ".ini",
		".sh", ".bash", ".sql", ".md", ".txt", ".log",
	}

	filename = strings.ToLower(filename)
	for _, ext := range codeExtensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}

	return false
}

// escapeHTML escapes HTML special characters
func (tn *TelegramNotifier) escapeHTML(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#x27;",
	)
	return replacer.Replace(text)
}

// GetStats returns notifier statistics
func (tn *TelegramNotifier) GetStats(ctx context.Context) map[string]interface{} {
	stats := map[string]interface{}{
		"api_base_url":       fmt.Sprintf("https://api.telegram.org/bot%s***", tn.botToken[:10]),
		"max_message_length": tn.config.MaxMessageLength,
		"request_timeout":    tn.config.RequestTimeout.String(),
		"max_retries":        tn.config.MaxRetries,
		"retry_delay":        tn.config.RetryDelay.String(),
		"enable_preview":     tn.config.EnablePreview,
		"parse_mode":         tn.config.ParseMode,
	}

	tn.logger.DebugContext(ctx, "Retrieved notifier stats", "stats", stats)
	return stats
}

// Close cleans up resources
func (tn *TelegramNotifier) Close() error {
	tn.logger.InfoContext(context.Background(), "Telegram notifier closed")
	return nil
}
