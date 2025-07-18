package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/izzddalfk/kumote/internal/assistant/core"
	"gopkg.in/validator.v2"
)

type Client struct {
	baseURL  string
	botToken string
}

type ClientConfig struct {
	BaseURL  string `validate:"nonzero"`
	BotToken string `validate:"nonzero"`
}

func NewClient(cfg ClientConfig) (*Client, error) {
	if err := validator.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid client configuration: %w", err)
	}

	return &Client{
		baseURL:  cfg.BaseURL,
		botToken: cfg.BotToken,
	}, nil
}

func (c *Client) botUrl() string {
	// Ensure the base URL ends with a slash
	if !strings.HasSuffix(c.baseURL, "/") {
		c.baseURL += "/"
	}
	return c.baseURL + "bot" + c.botToken
}

func (c *Client) SendTextMessage(ctx context.Context, input core.TelegramTextMessageInput) error {
	// Telegram Bot API URL
	apiURL := fmt.Sprintf("%s/sendMessage", c.botUrl())

	// Escape special characters for MarkdownV2 format
	// Characters that need escaping in MarkdownV2: '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!'
	escapedMessage := escapeMarkdownV2(input.Message)

	// Prepare the request payload
	type sendMessageRequest struct {
		ChatID    int64  `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode,omitempty"`
	}

	payload := sendMessageRequest{
		ChatID:    input.ChatID,
		Text:      escapedMessage,
		ParseMode: "MarkdownV2", // Using MarkdownV2 format
	}

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to marshal Telegram message payload",
			slog.Any("input", input),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create HTTP request for Telegram API",
			slog.Any("input", input),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to send Telegram message",
			slog.Any("input", input),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.ErrorContext(ctx, "Telegram API returned non-200 status",
			slog.Any("input", input),
			slog.Int("status_code", resp.StatusCode),
			slog.String("response", string(bodyBytes)))
		return fmt.Errorf("telegram API error: status %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// escapeMarkdownV2 escapes special characters in text for Telegram's MarkdownV2 format
func escapeMarkdownV2(text string) string {
	// Characters that need escaping in MarkdownV2:
	// '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!'
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}

	// Replace each special character with its escaped version
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}

	return result
}
