package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gopkg.in/validator.v2"
)

// Service implements the AssistantService interface
type Service struct {
	aiExecutor       AICodeExecutor
	telegram         TelegramStorage
	rateLimiter      RateLimiter
	userRepo         UserRepository
	projectScanner   ProjectScanner
	commandRepo      CommandRepository
	metricsCollector MetricsCollector
}

type ServiceConfig struct {
	AiExecutor       AICodeExecutor    `validate:"nonnil"`
	Telegram         TelegramStorage   `validate:"nonnil"`
	RateLimiter      RateLimiter       `validate:"nonnil"`
	UserRepo         UserRepository    `validate:"nonnil"`
	ProjectScanner   ProjectScanner    `validate:"nonnil"`
	CommandRepo      CommandRepository `validate:"nonnil"`
	MetricsCollector MetricsCollector  `validate:"nonnil"`
}

// NewService creates a new assistant service with all dependencies
func NewService(config ServiceConfig) (*Service, error) {
	if err := validator.Validate(config); err != nil {
		return nil, fmt.Errorf("invalid service configuration: %w", err)
	}
	return &Service{
		aiExecutor:       config.AiExecutor,
		telegram:         config.Telegram,
		rateLimiter:      config.RateLimiter,
		userRepo:         config.UserRepo,
		projectScanner:   config.ProjectScanner,
		commandRepo:      config.CommandRepo,
		metricsCollector: config.MetricsCollector,
	}, nil
}

// ProcessCommand processes a user command and returns the result
func (s *Service) ProcessCommand(ctx context.Context, cmd Command) (*QueryResult, error) {
	startTime := time.Now()

	// Check rate limit
	if !s.rateLimiter.IsAllowed(ctx, cmd.UserID) {
		result := &QueryResult{
			Success: false,
			Error:   "Rate limit exceeded. Please try again later.",
		}
		return result, nil
	}

	// Record the request for rate limiting
	if err := s.rateLimiter.RecordRequest(ctx, cmd.UserID); err != nil {
		slog.WarnContext(ctx, "Failed to record rate limit request",
			slog.Int64("user_id", cmd.UserID),
			slog.String("error", err.Error()))
	}

	// Check if the user is allowed
	user, err := s.userRepo.GetUser(ctx, cmd.UserID)
	if err != nil {
		// If user doesn't exist, check if they're in allowed list
		isAllowed := s.userRepo.IsUserAllowed(ctx, cmd.UserID)
		user = &User{
			ID:        cmd.UserID,
			IsAllowed: isAllowed,
		}
	}

	if !user.IsAllowed {
		result := &QueryResult{
			Success: false,
			Error:   "You are not authorized to use this assistant.",
		}
		return result, nil
	}

	// Save command to history
	if err := s.commandRepo.SaveCommand(ctx, &cmd); err != nil {
		slog.WarnContext(ctx, "Failed to save command",
			slog.String("command_id", cmd.ID),
			slog.String("error", err.Error()))
	}

	// use project index scanner to determine the working directory
	projectPath, err := s.projectScanner.GetProjectDirectory(cmd.Text)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("failed to get project directory: %s", err.Error()),
			slog.String("query", cmd.Text),
			slog.Int64("user_id", cmd.UserID))
		// Just send to Telegram that the project folder not found and ignore the error
		s.telegram.SendTextMessage(ctx, TelegramTextMessageInput{
			ChatID:  cmd.UserID,
			Message: "Project folder not found. Please add more specific project name in your query.",
		})
		return &QueryResult{
			Success:  true,
			Response: "Your request is being processed.",
		}, nil
	}

	// Create execution context
	execCtx := ExecutionContext{
		UserID:      cmd.UserID,
		WorkingDir:  projectPath,
		Timeout:     600 * time.Second,
		Environment: make(map[string]string),
	}
	// If the working directory not found, just return an error
	if execCtx.WorkingDir == "" {
		slog.WarnContext(ctx, "Working directory not found for command execution",
			slog.String("command_id", cmd.ID),
			slog.Int64("user_id", cmd.UserID))
		return nil, fmt.Errorf("working directory not found for command execution")
	}

	// Return early with a success response to the webhook
	// Create a copy of the context that won't be canceled when the request completes
	bgCtx, cancel := context.WithTimeout(context.Background(), execCtx.Timeout)

	// Process the command to AI assistant asynchronously in a goroutine
	go func() {
		defer cancel()
		// Process the command to AI assistant
		result, err := s.aiExecutor.ExecuteCommand(bgCtx, cmd.Text, execCtx)
		if err != nil {
			slog.ErrorContext(bgCtx, "Failed to process command asynchronously",
				slog.String("command_id", cmd.ID),
				slog.Int64("user_id", cmd.UserID),
				slog.String("error", err.Error()))
			s.recordMetrics(bgCtx, cmd, startTime, false, "")
			return
		}

		// Send the AI assistant's response via Telegram
		if err := s.telegram.SendTextMessage(bgCtx, TelegramTextMessageInput{
			ChatID:  cmd.UserID,
			Message: result.Response,
		}); err != nil {
			slog.ErrorContext(bgCtx, "Failed to send Telegram message",
				slog.String("command_id", cmd.ID),
				slog.Int64("user_id", cmd.UserID),
				slog.String("error", err.Error()))
		}

		slog.DebugContext(bgCtx, "Command processed successfully in background",
			slog.String("command_id", cmd.ID),
			slog.String("result", result.Response))

		// Record metrics
		s.recordMetrics(bgCtx, cmd, startTime, result.Success, "")
	}()

	// Return immediate success response
	return &QueryResult{
		Success:  true,
		Response: "Your request is being processed.",
	}, nil
}

// recordMetrics records command execution metrics
func (s *Service) recordMetrics(ctx context.Context, cmd Command, startTime time.Time, success bool, projectUsed string) {
	metrics := CommandMetrics{
		CommandID:     cmd.ID,
		UserID:        cmd.UserID,
		ExecutionTime: time.Since(startTime),
		Success:       success,
		ProjectUsed:   projectUsed,
		Timestamp:     time.Now(),
	}

	if err := s.metricsCollector.RecordCommandExecution(ctx, metrics); err != nil {
		slog.WarnContext(ctx, "Failed to record metrics",
			slog.String("command_id", cmd.ID),
			slog.String("error", err.Error()))
	}
}
