package telegramnotifier_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/knightazura/kumote/internal/assistant/adapters/telegramnotifier"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, // Show more logs for real API testing
	}))
}

// getTestBotToken retrieves the test bot token from environment
func getTestBotToken(t *testing.T) string {
	token := os.Getenv("TELEGRAM_TEST_BOT_TOKEN")
	if token == "" {
		t.Skip("TELEGRAM_TEST_BOT_TOKEN environment variable not set, skipping Telegram API tests")
	}
	return token
}

// getTestChatID retrieves the test chat ID from environment
func getTestChatID(t *testing.T) int64 {
	chatIDStr := os.Getenv("TELEGRAM_TEST_CHAT_ID")
	if chatIDStr == "" {
		t.Skip("TELEGRAM_TEST_CHAT_ID environment variable not set, skipping Telegram API tests")
	}

	var chatID int64
	if _, err := fmt.Sscanf(chatIDStr, "%d", &chatID); err != nil {
		t.Fatalf("Invalid TELEGRAM_TEST_CHAT_ID format: %v", err)
	}
	return chatID
}

// isIntegrationTest checks if integration tests should run
func isIntegrationTest() bool {
	return os.Getenv("RUN_INTEGRATION_TESTS") == "true"
}

func setupTelegramNotifier(t *testing.T) (*telegramnotifier.TelegramNotifier, func()) {
	if !isIntegrationTest() {
		t.Skip("Integration tests disabled. Set RUN_INTEGRATION_TESTS=true to enable")
	}

	botToken := getTestBotToken(t)
	logger := getTestLogger()

	tn, err := telegramnotifier.NewTelegramNotifier(botToken, logger)
	require.NoError(t, err)

	cleanup := func() {
		tn.Close()
	}

	return tn, cleanup
}

func TestTelegramNotifier_NewTelegramNotifier_Success(t *testing.T) {
	logger := getTestLogger()

	tn, err := telegramnotifier.NewTelegramNotifier("valid_bot_token_format", logger)
	require.NoError(t, err)
	assert.NotNil(t, tn)

	tn.Close()
}

func TestTelegramNotifier_NewTelegramNotifier_EmptyToken(t *testing.T) {
	logger := getTestLogger()

	tn, err := telegramnotifier.NewTelegramNotifier("", logger)
	assert.Error(t, err)
	assert.Nil(t, tn)
	assert.Contains(t, err.Error(), "bot token cannot be empty")
}

func TestTelegramNotifier_ValidateConnection_Success(t *testing.T) {
	tn, cleanup := setupTelegramNotifier(t)
	defer cleanup()

	ctx := context.Background()

	err := tn.ValidateConnection(ctx)
	require.NoError(t, err, "Failed to validate Telegram bot connection")
}

func TestTelegramNotifier_GetBotInfo_Success(t *testing.T) {
	tn, cleanup := setupTelegramNotifier(t)
	defer cleanup()

	ctx := context.Background()

	botInfo, err := tn.GetBotInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, botInfo)

	// Verify bot info structure
	assert.Contains(t, botInfo, "id")
	assert.Contains(t, botInfo, "username")
	assert.Contains(t, botInfo, "first_name")
	assert.Contains(t, botInfo, "is_bot")

	// Bot should be marked as a bot
	assert.True(t, botInfo["is_bot"].(bool))

	t.Logf("Bot info: ID=%v, Username=%v, Name=%v",
		botInfo["id"], botInfo["username"], botInfo["first_name"])
}

func TestTelegramNotifier_SendMessage_Success(t *testing.T) {
	tn, cleanup := setupTelegramNotifier(t)
	defer cleanup()

	ctx := context.Background()
	chatID := getTestChatID(t)
	message := fmt.Sprintf("Test message from Go tests at %s", time.Now().Format(time.RFC3339))

	err := tn.SendMessage(ctx, chatID, message)
	require.NoError(t, err, "Failed to send test message")

	t.Logf("Successfully sent message to chat ID %d", chatID)
}

func TestTelegramNotifier_SendMessage_LongMessage(t *testing.T) {
	tn, cleanup := setupTelegramNotifier(t)
	defer cleanup()

	ctx := context.Background()
	chatID := getTestChatID(t)

	// Create a message longer than Telegram's 4096 character limit
	baseMessage := "This is a long test message that will be split into multiple parts. "
	longMessage := strings.Repeat(baseMessage, 100) // ~6400 characters
	longMessage += fmt.Sprintf("\n\nSent at: %s", time.Now().Format(time.RFC3339))

	err := tn.SendMessage(ctx, chatID, longMessage)
	require.NoError(t, err, "Failed to send long message")

	t.Logf("Successfully sent long message (%d chars) to chat ID %d",
		len(longMessage), chatID)
}

func TestTelegramNotifier_SendFormattedMessage_HTML(t *testing.T) {
	tn, cleanup := setupTelegramNotifier(t)
	defer cleanup()

	ctx := context.Background()
	chatID := getTestChatID(t)

	message := fmt.Sprintf(`<b>HTML Formatted Message</b>

<i>This is italic text</i>
<u>This is underlined text</u>
<code>This is monospace code</code>

<pre>
This is a code block
with multiple lines
</pre>

<a href="https://telegram.org">This is a link</a>

Test timestamp: %s`, time.Now().Format(time.RFC3339))

	err := tn.SendFormattedMessage(ctx, chatID, message, core.ParseModeHTML)
	require.NoError(t, err, "Failed to send HTML formatted message")

	t.Logf("Successfully sent HTML formatted message to chat ID %d", chatID)
}

// func TestTelegramNotifier_SendFormattedMessage_Markdown(t *testing.T) {
// 	tn, cleanup := setupTelegramNotifier(t)
// 	defer cleanup()

// 	ctx := context.Background()
// 	chatID := getTestChatID(t)

// 	message := fmt.Sprintf(`*Markdown Formatted Message*

// _
