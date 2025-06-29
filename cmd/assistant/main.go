// main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/knightazura/kumote/internal/assistant/adapters/audiotranscriber"
	"github.com/knightazura/kumote/internal/assistant/adapters/codecompletion"
	"github.com/knightazura/kumote/internal/assistant/adapters/commandrepository"
	"github.com/knightazura/kumote/internal/assistant/adapters/configprovider"
	"github.com/knightazura/kumote/internal/assistant/adapters/filesystem"
	"github.com/knightazura/kumote/internal/assistant/adapters/metricscollector"
	"github.com/knightazura/kumote/internal/assistant/adapters/ratelimiter"
	"github.com/knightazura/kumote/internal/assistant/adapters/scanner"
	"github.com/knightazura/kumote/internal/assistant/adapters/telegramnotifier"
	"github.com/knightazura/kumote/internal/assistant/adapters/userrepository"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/knightazura/kumote/internal/assistant/presentation/config"
	"github.com/knightazura/kumote/internal/assistant/presentation/server"
)

func main() {
	// Setup context
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg.LogLevel, cfg.IsDevelopment())

	logger.InfoContext(ctx, "Starting Remote Work Telegram Assistant",
		"version", cfg.Version,
		"environment", cfg.Environment,
		"port", cfg.Port,
	)

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		logger.ErrorContext(ctx, "Configuration validation failed", "error", err)
		os.Exit(1)
	}

	// Initialize dependencies
	deps, err := initializeDependencies(ctx, cfg, logger)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to initialize dependencies", "error", err)
		os.Exit(1)
	}
	defer deps.Cleanup()

	// Initialize assistant service
	assistantService, err := initializeAssistantService(ctx, deps, logger)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to initialize assistant service", "error", err)
		os.Exit(1)
	}

	// Create and start HTTP server
	httpServer := server.NewServer(cfg, logger, assistantService)

	logger.InfoContext(ctx, "All systems initialized, starting server")

	// Start server (this blocks until shutdown)
	if err := httpServer.Start(ctx); err != nil {
		logger.ErrorContext(ctx, "Server error", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "Application shutdown completed")
}

// Dependencies holds all initialized dependencies
type Dependencies struct {
	TelegramNotifier  core.TelegramNotifier
	AICodeExecutor    core.AICodeExecutor
	ProjectScanner    core.ProjectScanner
	UserRepository    core.UserRepository
	CommandRepository core.CommandRepository
	MetricsCollector  core.MetricsCollector
	ConfigProvider    core.ConfigProvider
	FileSystem        core.FileSystem
	RateLimiter       core.RateLimiter
	AudioTranscriber  core.AudioTranscriber
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

	var handler slog.Handler
	if isDevelopment {
		// Use text handler for development (more human-readable)
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// Use JSON handler for production (machine-readable)
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// validateConfig validates the loaded configuration
func validateConfig(cfg *config.ServerConfig) error {
	// Basic validation is already done in config.Validate()
	// Add any additional validation here if needed

	if cfg.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required")
	}

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
func initializeDependencies(ctx context.Context, cfg *config.ServerConfig, logger *slog.Logger) (*Dependencies, error) {
	deps := &Dependencies{}

	// Create data directories if they don't exist
	dataPath := "data"
	os.MkdirAll(dataPath, 0755)

	configPath := "config"
	os.MkdirAll(configPath, 0755)

	// Initialize Telegram notifier
	telegramNotifier, err := telegramnotifier.NewTelegramNotifier(cfg.TelegramBotToken, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Telegram notifier: %w", err)
	}

	// Validate Telegram connection
	if err := telegramNotifier.ValidateConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate Telegram connection: %w", err)
	}

	deps.TelegramNotifier = telegramNotifier
	logger.InfoContext(ctx, "Telegram notifier initialized successfully")

	// Initialize filesystem adapter
	// Initialize with the development path as the base path
	fs, err := filesystem.NewFileSystem(cfg.DevelopmentPath, []string{cfg.DevelopmentPath}, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize filesystem: %w", err)
	}
	deps.FileSystem = fs
	logger.InfoContext(ctx, "Filesystem adapter initialized successfully")

	// Initialize config provider
	configFilePath := filepath.Join(configPath, "config.yaml")
	configProvider, err := configprovider.NewConfigProvider(configFilePath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize config provider: %w", err)
	}
	deps.ConfigProvider = configProvider
	logger.InfoContext(ctx, "Config provider initialized successfully")

	// Initialize user repository
	userRepo := userrepository.NewUserRepository(logger)
	deps.UserRepository = userRepo
	logger.InfoContext(ctx, "User repository initialized successfully")

	// Initialize rate limiter
	rateLimiter := ratelimiter.NewRateLimiter(cfg.RateLimitPerMinute, logger)
	deps.RateLimiter = rateLimiter
	logger.InfoContext(ctx, "Rate limiter initialized successfully")

	// Initialize metrics collector
	metricsDbPath := filepath.Join(dataPath, "metrics.db")
	metricsCollector, err := metricscollector.NewMetricsCollector(metricsDbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics collector: %w", err)
	}
	deps.MetricsCollector = metricsCollector
	logger.InfoContext(ctx, "Metrics collector initialized successfully")

	// Initialize command repository
	commandsDbPath := filepath.Join(dataPath, "commands.db")
	commandRepo, err := commandrepository.NewCommandRepository(commandsDbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize command repository: %w", err)
	}
	deps.CommandRepository = commandRepo
	logger.InfoContext(ctx, "Command repository initialized successfully")

	// Initialize project scanner
	scanConfig := &core.ScanConfig{
		BasePath:       cfg.DevelopmentPath,
		ExcludedDirs:   []string{"node_modules", ".git", "dist", "build", "vendor"},
		MaxDepth:       3,
		MinProjectSize: 1024,
		Shortcuts:      make(map[string]string),
	}
	indexPath := filepath.Join(dataPath, "projects-index.json")
	projectScanner := scanner.NewProjectScanner(scanConfig, indexPath, logger)
	deps.ProjectScanner = projectScanner
	logger.InfoContext(ctx, "Project scanner initialized successfully")

	// Initialize audio transcriber (using dummy implementation for now)
	audioTranscriber := audiotranscriber.NewDummyTranscriber()
	deps.AudioTranscriber = audioTranscriber
	logger.InfoContext(ctx, "Audio transcriber initialized successfully")

	// Initialize AI code executor
	claudeExecutable := "claude"
	if cfg.ClaudeCodePath != "" {
		claudeExecutable = cfg.ClaudeCodePath
	}
	aiExecutor := codecompletion.NewClaudeExecutor(claudeExecutable, "sonnet", cfg.DevelopmentPath, cfg.IsDevelopment())
	deps.AICodeExecutor = aiExecutor
	logger.InfoContext(ctx, "AI code executor initialized successfully")

	return deps, nil
}

// initializeAssistantService creates the main assistant service
func initializeAssistantService(ctx context.Context, deps *Dependencies, logger *slog.Logger) (core.AssistantService, error) {
	logger.InfoContext(ctx, "Initializing assistant service")

	// Create the actual service with all dependencies
	service := core.NewService(
		deps.ProjectScanner,
		deps.AICodeExecutor,
		deps.TelegramNotifier,
		deps.AudioTranscriber,
		deps.UserRepository,
		deps.CommandRepository,
		deps.MetricsCollector,
		deps.ConfigProvider,
		deps.RateLimiter,
	)

	logger.InfoContext(ctx, "Assistant service initialized successfully")

	return service, nil
}
