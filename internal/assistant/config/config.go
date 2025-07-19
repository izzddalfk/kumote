package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ApplicationConfig struct {
	ProjectsPath           string  `cfg:"projects_path" cfgRequired:"true"`
	ClaudeCodePath         string  `cfg:"claude_code_path" cfgRequired:"true"`
	ProjectIndexPath       string  `cfg:"project_index_path"`
	TelegramBotToken       string  `cfg:"kumote_telegram_bot_token" cfgRequired:"true"`
	TelegramAllowedUserIDs []int64 `cfg:"kumote_telegram_allowed_user_ids" cfgRequired:"true"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	// Server settings
	Port            int           `json:"port"`
	Host            string        `json:"host"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// Telegram settings
	TelegramBaseURL       string `cfg:"telegram_base_url" cfgDefault:"https://api.telegram.org"` // Base URL for Telegram API
	TelegramBotToken      string `json:"-"`                                                      // Don't expose in JSON
	TelegramWebhookSecret string `json:"-"`                                                      // Don't expose in JSON

	// Security settings
	AllowedUserIDs     []int64 `json:"allowed_user_ids"`
	RateLimitPerMinute int     `json:"rate_limit_per_minute"`

	// Application settings
	DevelopmentPath string `json:"development_path"`
	ClaudeCodePath  string `json:"claude_code_path"`
	LogLevel        string `json:"log_level"`
	Version         string `json:"version"`
	Environment     string `json:"environment"`

	// Feature flags
	EnableDetailedLogging bool `json:"enable_detailed_logging"`
	EnableMetrics         bool `json:"enable_metrics"`
	EnableProfiling       bool `json:"enable_profiling"`
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*ServerConfig, error) {
	config := &ServerConfig{
		// Default values
		Port:               3000,
		Host:               "0.0.0.0",
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,
		ShutdownTimeout:    30 * time.Second,
		RateLimitPerMinute: 10,
		Environment:        "development",
		Version:            "1.0.0",
		DevelopmentPath:    "~/Development",
		ClaudeCodePath:     os.Getenv("CLAUDE_CODE_PATH"),
	}

	// Load from environment variables
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Port = p
		}
	}

	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.Host = host
	}

	// Telegram configuration
	config.TelegramBaseURL = "https://api.telegram.org"
	config.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	config.TelegramWebhookSecret = os.Getenv("TELEGRAM_WEBHOOK_SECRET")

	// Parse allowed user IDs
	if userIDsStr := os.Getenv("ALLOWED_USER_IDS"); userIDsStr != "" {
		userIDs, err := parseUserIDs(userIDsStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse allowed user IDs: %w", err)
		}
		config.AllowedUserIDs = userIDs
	}

	// Rate limiting
	if rateLimitStr := os.Getenv("RATE_LIMIT_PER_MINUTE"); rateLimitStr != "" {
		if rateLimit, err := strconv.Atoi(rateLimitStr); err == nil {
			config.RateLimitPerMinute = rateLimit
		}
	}

	// Application paths
	if devPath := os.Getenv("DEVELOPMENT_PATH"); devPath != "" {
		config.DevelopmentPath = devPath
	}

	if config.ClaudeCodePath == "" {
		return nil, fmt.Errorf("CLAUDE_CODE_PATH environment variable is required")
	}

	// Logging and debugging
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	if env := os.Getenv("ENVIRONMENT"); env != "" {
		config.Environment = env
	}

	if version := os.Getenv("VERSION"); version != "" {
		config.Version = version
	}

	// Feature flags
	config.EnableDetailedLogging = os.Getenv("ENABLE_DETAILED_LOGGING") == "true"
	config.EnableMetrics = os.Getenv("ENABLE_METRICS") == "true"
	config.EnableProfiling = os.Getenv("ENABLE_PROFILING") == "true"

	// Timeouts (if specified)
	if readTimeout := os.Getenv("READ_TIMEOUT"); readTimeout != "" {
		if duration, err := time.ParseDuration(readTimeout); err == nil {
			config.ReadTimeout = duration
		}
	}

	if writeTimeout := os.Getenv("WRITE_TIMEOUT"); writeTimeout != "" {
		if duration, err := time.ParseDuration(writeTimeout); err == nil {
			config.WriteTimeout = duration
		}
	}

	if idleTimeout := os.Getenv("IDLE_TIMEOUT"); idleTimeout != "" {
		if duration, err := time.ParseDuration(idleTimeout); err == nil {
			config.IdleTimeout = duration
		}
	}

	if shutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT"); shutdownTimeout != "" {
		if duration, err := time.ParseDuration(shutdownTimeout); err == nil {
			config.ShutdownTimeout = duration
		}
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *ServerConfig) Validate() error {
	if c.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	if len(c.AllowedUserIDs) == 0 {
		return fmt.Errorf("at least one allowed user ID must be specified")
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.Port)
	}

	if c.RateLimitPerMinute < 1 {
		return fmt.Errorf("rate limit per minute must be at least 1")
	}

	return nil
}

// GetAddress returns the server address
func (c *ServerConfig) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// IsProduction returns true if running in production environment
func (c *ServerConfig) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment returns true if running in development environment
func (c *ServerConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

// parseUserIDs parses comma-separated user IDs
func parseUserIDs(userIDsStr string) ([]int64, error) {
	var userIDs []int64

	parts := strings.Split(userIDsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		userID, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID '%s': %w", part, err)
		}

		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}
