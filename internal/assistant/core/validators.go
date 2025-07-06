// internal/assistant/core/validators.go
package core

import (
"fmt"
"regexp"
"strings"
"unicode/utf8"
)

// Command validation

// ValidateCommand validates a command structure
func ValidateCommand(cmd Command) error {
	if cmd.ID == "" {
		return NewValidationError("id", "command ID cannot be empty")
	}

	if cmd.UserID == 0 {
		return NewValidationError("user_id", "user ID cannot be zero")
	}

	if cmd.Text == "" {
		return NewValidationError("content", "command must have text content")
	}

	if cmd.Text != "" {
		if len(cmd.Text) > TelegramMaxMessageLength {
			return NewValidationError("text", fmt.Sprintf("text length exceeds maximum of %d characters", TelegramMaxMessageLength))
		}

		if !utf8.ValidString(cmd.Text) {
			return NewValidationError("text", "text contains invalid UTF-8 characters")
		}
	}

	if cmd.Timestamp.IsZero() {
		return NewValidationError("timestamp", "timestamp cannot be zero")
	}

	return nil
}

// ValidateQuery validates a user query
func ValidateQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return ErrEmptyQuery
	}

	if len(query) > TelegramMaxMessageLength {
		return NewValidationError("query", fmt.Sprintf("query length exceeds maximum of %d characters", TelegramMaxMessageLength))
	}

	if !utf8.ValidString(query) {
		return NewValidationError("query", "query contains invalid UTF-8 characters")
	}

	// Check for potentially dangerous commands
	if containsDangerousCommand(query) {
		return NewValidationError("query", "query contains potentially dangerous commands")
	}

	return nil
}

// containsDangerousCommand checks if a query contains potentially dangerous commands
func containsDangerousCommand(query string) bool {
	dangerousPatterns := []string{
		`rm\s+(-rf?|--force)\s+[/~]`,
		`mkfs`,
		`dd\s+if=`,
		`sudo\s+rm`,
		`format\s+[a-zA-Z]:`,
		`deltree`,
		`wget.+\|\s*sh`,
		`curl.+\|\s*sh`,
	}

	lowercaseQuery := strings.ToLower(query)
	for _, pattern := range dangerousPatterns {
		match, _ := regexp.MatchString(pattern, lowercaseQuery)
		if match {
			return true
		}
	}

	return false
}
