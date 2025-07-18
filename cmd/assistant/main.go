// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/izzddalfk/kumote/internal/assistant/adapters/codecompletion"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/commandrepository"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/metricscollector"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/ratelimiter"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/scanner"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/telegram"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/userrepository"
	"github.com/izzddalfk/kumote/internal/assistant/core"
	"github.com/izzddalfk/kumote/internal/assistant/presentation/config"
	"github.com/izzddalfk/kumote/internal/assistant/presentation/server"
)

func main() {
	// Setup context
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logger
	logger := setupLogger(cfg.LogLevel, cfg.IsDevelopment())

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		logger.ErrorContext(ctx, "Configuration validation failed", "error", err)
		os.Exit(1)
	}

	// Initialize dependencies
	deps, err := initializeDependencies(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}

	// Initialize assistant service
	assistantService, err := initializeAssistantService(ctx, deps, logger)
	if err != nil {
		log.Fatalf("Failed to initialize assistant service: %v", err)
	}

	// Create and start HTTP server
	httpServer := server.NewServer(cfg, logger, assistantService)

	logger.InfoContext(ctx, "Starting Remote Work Telegram Assistant",
		"version", cfg.Version,
		"environment", cfg.Environment,
		"port", cfg.Port,
	)

	// Start server (this blocks until shutdown)
	if err := httpServer.Start(ctx); err != nil {
		logger.ErrorContext(ctx, "Server error", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "Application shutdown completed")
}

// Dependencies holds all initialized dependencies
type Dependencies struct {
	AICodeExecutor    core.AIAgent
	UserRepository    core.UserRepository
	CommandRepository core.CommandRepository
	MetricsCollector  core.MetricsCollector
	RateLimiter       core.RateLimiter
}

// Cleanup cleans up all dependencies
func (d *Dependencies) Cleanup() {
	// Close any connections or resources that need explicit cleanup
	// Most of our dependencies don't require explicit cleanup
}

// setupLogger creates and configures the logger
func setupLogger(logLevel string, isDevelopment bool) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Use JSON handler
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	return logger
}

// validateConfig validates the loaded configuration
func validateConfig(cfg *config.ServerConfig) error {
	// Basic validation is already done in config.Validate()
	// Add any additional validation here if needed

	if len(cfg.AllowedUserIDs) == 0 {
		return fmt.Errorf("ALLOWED_USER_IDS environment variable is required")
	}

	// Validate paths exist
	if _, err := os.Stat(cfg.DevelopmentPath); os.IsNotExist(err) {
		return fmt.Errorf("development path does not exist: %s", cfg.DevelopmentPath)
	}

	return nil
}

// initializeDependencies initializes all external dependencies
func initializeDependencies(cfg *config.ServerConfig, logger *slog.Logger) (*core.ServiceConfig, error) {
	// Create data directories if they don't exist
	dataPath := "data"
	os.MkdirAll(dataPath, 0755)

	// Initialize metrics collector
	metricsDbPath := filepath.Join(dataPath, "metrics.db")
	metricsCollector, err := metricscollector.NewMetricsCollector(metricsDbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics collector: %w", err)
	}

	// Initialize command repository
	commandsDbPath := filepath.Join(dataPath, "commands.db")
	commandRepo, err := commandrepository.NewCommandRepository(commandsDbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize command repository: %w", err)
	}

	// Initialize AI code executor
	claudeExecutable := "claude"
	if cfg.ClaudeCodePath != "" {
		claudeExecutable = cfg.ClaudeCodePath
	}
	aiExecutor := codecompletion.NewClaudeExecutor(claudeExecutable, "sonnet", cfg.DevelopmentPath, cfg.IsDevelopment())

	// Initialize project scanner
	projectScanner, err := scanner.NewFileSystemScanner(scanner.FileSystemScannerConfig{
		ProjectIndexPath:       os.Getenv("PROJECT_INDEX_PATH"),
		WordProximityThreshold: 0.5, // use lower threshold for less permissive matching
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project scanner: %w", err)
	}

	// Initialize Telegram storage
	telegramStorage, err := telegram.NewClient(telegram.ClientConfig{
		BaseURL:  cfg.TelegramBaseURL,
		BotToken: cfg.TelegramBotToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Telegram client: %w", err)
	}

	return &core.ServiceConfig{
		AiExecutor:       aiExecutor,
		Telegram:         telegramStorage,
		ProjectScanner:   projectScanner,
		CommandRepo:      commandRepo,
		MetricsCollector: metricsCollector,
		UserRepo:         userrepository.NewUserRepository(logger),
		RateLimiter:      ratelimiter.NewRateLimiter(cfg.RateLimitPerMinute, logger),
	}, nil
}

// initializeAssistantService creates the main assistant service
func initializeAssistantService(ctx context.Context, config *core.ServiceConfig, logger *slog.Logger) (core.AssistantService, error) {
	logger.InfoContext(ctx, "Initializing assistant service")

	// Create the actual service with all dependencies
	service, err := core.NewService(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to create assistant service: %w", err)
	}

	logger.InfoContext(ctx, "Assistant service initialized successfully")

	return service, nil
}
