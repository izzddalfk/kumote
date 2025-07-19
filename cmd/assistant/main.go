// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/izzddalfk/kumote/internal/assistant/adapters/agents"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/metricscollector"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/ratelimiter"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/scanner"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/telegram"
	"github.com/izzddalfk/kumote/internal/assistant/adapters/userrepository"
	"github.com/izzddalfk/kumote/internal/assistant/config"
	"github.com/izzddalfk/kumote/internal/assistant/core"
	"github.com/izzddalfk/kumote/internal/assistant/presentation/rest"
)

func main() {
	// Setup context
	ctx := context.Background()

	// Load configuration
	configs, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Setup logger
	setupLogger(configs.ApplicationConfig.LogLevel)

	// Initialize dependencies
	deps, err := initializeDependencies(configs)
	if err != nil {
		log.Fatalf("failed to initialize dependencies: %v", err)
	}

	// Initialize assistant service
	assistantService, err := core.NewService(*deps)
	if err != nil {
		log.Fatalf("failed to initialize assistant service: %v", err)
	}

	// Create and start HTTP server
	httpServer, err := rest.NewServer(rest.ServerConfig{
		AssistantService: assistantService,
		Port:             fmt.Sprintf(":%d", configs.ServerConfig.Port),
		ReadTimeout:      time.Second * 5,
		WriteTimeout:     time.Second * 30,
	})
	if err != nil {
		log.Fatalf("failed to create HTTP server: %v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("Kumote started at port: %d", configs.ServerConfig.Port))

	// Start server (this blocks until shutdown)
	if err := httpServer.Start(); err != nil {
		slog.ErrorContext(ctx, "Server error", "error", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "Application shutdown completed")
}

// Dependencies holds all initialized dependencies
type Dependencies struct {
	AICodeExecutor   core.AIAgent
	UserRepository   core.UserRepository
	MetricsCollector core.MetricsCollector
	RateLimiter      core.RateLimiter
}

// Cleanup cleans up all dependencies
func (d *Dependencies) Cleanup() {
	// Close any connections or resources that need explicit cleanup
	// Most of our dependencies don't require explicit cleanup
}

// setupLogger creates and configures the logger
func setupLogger(logLevel string) {
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
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
}

// initializeDependencies initializes all external dependencies
func initializeDependencies(cfg *config.Configs) (*core.ServiceConfig, error) {
	// Create data directories if they don't exist
	dataPath := "data"
	os.MkdirAll(dataPath, 0755)

	// Initialize metrics collector
	metricsDbPath := filepath.Join(dataPath, "metrics.db")
	metricsCollector, err := metricscollector.NewMetricsCollector(metricsDbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics collector: %w", err)
	}

	// Initialize AI Agent
	aiExecutor, err := agents.NewClaudeCodeAgent(agents.ClaudeCodeAgentConfig{
		ExecutablePath: cfg.ApplicationConfig.ClaudeCodePath,
		DefaultModel:   "sonnet",
		BaseWorkDir:    cfg.ApplicationConfig.ProjectsPath,
		Debug:          true, // TODO: Setup this flag
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ai agent: %w", err)
	}

	// Initialize project scanner
	projectScanner, err := scanner.NewFileSystemScanner(scanner.FileSystemScannerConfig{
		ProjectIndexPath:       cfg.ApplicationConfig.ProjectIndexPath,
		WordProximityThreshold: 0.5, // use lower threshold for less permissive matching
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project scanner: %w", err)
	}

	// Initialize Telegram storage
	telegramStorage, err := telegram.NewClient(telegram.ClientConfig{
		BaseURL:  cfg.ApplicationConfig.TelegramBaseURL,
		BotToken: cfg.ApplicationConfig.TelegramBotToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Telegram client: %w", err)
	}

	// Initialize user repository
	userRepo, err := userrepository.NewUserRepository(userrepository.UserRepositoryConfig{
		AllowedUserIDsString: cfg.ApplicationConfig.TelegramAllowedUserIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user repository: %w", err)
	}

	return &core.ServiceConfig{
		AiExecutor:       aiExecutor,
		Telegram:         telegramStorage,
		ProjectScanner:   projectScanner,
		MetricsCollector: metricsCollector,
		UserRepo:         userRepo,
		RateLimiter:      ratelimiter.NewRateLimiter(2), // TODO: revisit this value later
	}, nil
}
