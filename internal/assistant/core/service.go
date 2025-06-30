package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Service implements the AssistantService interface
type Service struct {
	projectScanner   ProjectScanner
	aiExecutor       AICodeExecutor
	telegramNotifier TelegramNotifier
	audioTranscriber AudioTranscriber
	userRepo         UserRepository
	commandRepo      CommandRepository
	metricsCollector MetricsCollector
	configProvider   ConfigProvider
	rateLimiter      RateLimiter
}

// NewService creates a new assistant service with all dependencies
func NewService(
	projectScanner ProjectScanner,
	aiExecutor AICodeExecutor,
	telegramNotifier TelegramNotifier,
	audioTranscriber AudioTranscriber,
	userRepo UserRepository,
	commandRepo CommandRepository,
	metricsCollector MetricsCollector,
	configProvider ConfigProvider,
	rateLimiter RateLimiter,
) *Service {
	return &Service{
		projectScanner:   projectScanner,
		aiExecutor:       aiExecutor,
		telegramNotifier: telegramNotifier,
		audioTranscriber: audioTranscriber,
		userRepo:         userRepo,
		commandRepo:      commandRepo,
		metricsCollector: metricsCollector,
		configProvider:   configProvider,
		rateLimiter:      rateLimiter,
	}
}

// ProcessCommand processes a user command and returns the result
func (s *Service) ProcessCommand(ctx context.Context, cmd Command) (*QueryResult, error) {
	startTime := time.Now()

	// Check rate limiting
	if !s.rateLimiter.IsAllowed(ctx, cmd.UserID) {
		result := &QueryResult{
			Success: false,
			Error:   "Rate limit exceeded. Please try again later.",
		}
		// Respond to the user with the rate limit error
		if err := s.RespondToCommand(ctx, cmd.UserID, result); err != nil {
			slog.ErrorContext(ctx, "Failed to send rate limit error response",
				slog.Int64("user_id", cmd.UserID),
				slog.String("error", err.Error()))
		}
		return result, nil
	}

	// Record the request
	if err := s.rateLimiter.RecordRequest(ctx, cmd.UserID); err != nil {
		slog.WarnContext(ctx, "Failed to record rate limit request",
			slog.Int64("user_id", cmd.UserID),
			slog.String("error", err.Error()))
	}

	// Check user permissions
	user, err := s.GetUserPermissions(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check user permissions: %w", err)
	}

	if !user.IsAllowed {
		result := &QueryResult{
			Success: false,
			Error:   "You are not authorized to use this assistant.",
		}
		// Respond to the user with the authorization error
		if err := s.RespondToCommand(ctx, cmd.UserID, result); err != nil {
			slog.ErrorContext(ctx, "Failed to send authorization error response",
				slog.Int64("user_id", cmd.UserID),
				slog.String("error", err.Error()))
		}
		return result, nil
	}

	// Save command to history
	if err := s.commandRepo.SaveCommand(ctx, &cmd); err != nil {
		slog.WarnContext(ctx, "Failed to save command",
			slog.String("command_id", cmd.ID),
			slog.String("error", err.Error()))
	}

	// Process the command
	result, err := s.processUserQuery(ctx, cmd.Text, cmd.UserID)
	if err != nil {
		s.recordMetrics(ctx, cmd, startTime, false, "")
		return nil, fmt.Errorf("failed to process command: %w", err)
	}

	// Record metrics
	projectUsed := ""
	if len(result.Projects) > 0 {
		projectUsed = result.Projects[0].Name
	}
	s.recordMetrics(ctx, cmd, startTime, result.Success, projectUsed)

	// Respond to the user with the result
	if err := s.RespondToCommand(ctx, cmd.UserID, result); err != nil {
		slog.ErrorContext(ctx, "Failed to send command response",
			slog.Int64("user_id", cmd.UserID),
			slog.String("error", err.Error()))
		// Don't return the error here to ensure we still return the result
	}

	return result, nil
}

// ProcessAudioCommand processes audio command by converting to text first
func (s *Service) ProcessAudioCommand(ctx context.Context, cmd Command) (*QueryResult, error) {
	if cmd.AudioFileID == "" {
		return &QueryResult{
			Success: false,
			Error:   "No audio file provided",
		}, nil
	}

	// TODO: Get audio file from Telegram and transcribe
	// This would be implemented in the Telegram adapter
	// For now, return an error indicating audio processing is not yet implemented

	return &QueryResult{
		Success: false,
		Error:   "Audio command processing is not yet implemented",
	}, nil
}

// GetProjectByShortcut retrieves project information by shortcut
func (s *Service) GetProjectByShortcut(ctx context.Context, shortcut string) (*Project, error) {
	index, err := s.projectScanner.GetProjectIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get project index: %w", err)
	}

	projectName, exists := index.Shortcuts[shortcut]
	if !exists {
		return nil, fmt.Errorf("shortcut '%s' not found", shortcut)
	}

	return s.GetProjectByName(ctx, projectName)
}

// GetProjectByName retrieves project information by name
func (s *Service) GetProjectByName(ctx context.Context, name string) (*Project, error) {
	index, err := s.projectScanner.GetProjectIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get project index: %w", err)
	}

	for _, project := range index.Projects {
		if strings.EqualFold(project.Name, name) {
			return &project, nil
		}
	}

	return nil, fmt.Errorf("project '%s' not found", name)
}

// ListProjects returns all available projects
func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	index, err := s.projectScanner.GetProjectIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get project index: %w", err)
	}

	return index.Projects, nil
}

// RefreshProjects triggers a manual refresh of project index
func (s *Service) RefreshProjects(ctx context.Context) error {
	slog.InfoContext(ctx, "Starting manual project refresh")

	_, err := s.projectScanner.UpdateIndex(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to refresh projects",
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to refresh projects: %w", err)
	}

	slog.InfoContext(ctx, "Project refresh completed successfully")
	return nil
}

// GetUserPermissions checks if user is allowed to use the assistant
func (s *Service) GetUserPermissions(ctx context.Context, userID int64) (*User, error) {
	user, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		// If user doesn't exist, check if they're in allowed list
		isAllowed := s.userRepo.IsUserAllowed(ctx, userID)
		return &User{
			ID:        userID,
			IsAllowed: isAllowed,
		}, nil
	}

	return user, nil
}

func (s *Service) RespondToCommand(ctx context.Context, userID int64, result *QueryResult) error {
	if result == nil {
		return s.sendErrorToUser(ctx, userID, "No result received.")
	}

	// Handle error results
	if !result.Success {
		slog.ErrorContext(ctx, "Command execution failed",
			slog.Int64("user_id", userID),
			slog.String("error", result.Error),
		)
		message := result.Error
		if message == "" {
			message = "Command failed without specific error message."
		}
		return s.telegramNotifier.SendMessage(ctx, userID, fmt.Sprintf("❌ %s", message))
	}

	// Send files if any
	for _, file := range result.Files {
		if file.Content != "" {
			// By default send as code block. Revisit later based on content length
			fileReader := strings.NewReader(file.Content)
			if err := s.telegramNotifier.SendFile(ctx, userID, fileReader, file.Name); err != nil {
				slog.ErrorContext(ctx, "Failed to send file",
					"user_id", userID,
					"filename", file.Name,
					"error", err.Error(),
				)
				// Continue with other files instead of returning error
			}
		}
	}

	// TODO: Send confirmation request if needed

	// Send main response
	if result.Response != "" {
		err := s.telegramNotifier.SendFormattedMessage(ctx, userID, result.Response, "HTML")
		if err != nil {
			slog.ErrorContext(ctx, "Failed to send response message",
				"user_id", userID,
				"error", err.Error(),
			)
			return err
		}
	}

	return nil
}

// processUserQuery handles the core logic of processing user queries
func (s *Service) processUserQuery(ctx context.Context, query string, userID int64) (*QueryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return &QueryResult{
			Success: false,
			Error:   "Empty query provided",
		}, nil
	}

	slog.InfoContext(ctx, "Processing user query",
		slog.Int64("user_id", userID),
		slog.String("query", query))

	// Check if AI Code Executor is available
	if !s.aiExecutor.IsAvailable(ctx) {
		return &QueryResult{
			Success: false,
			Error:   "AI Code Executor is not available. Please check the configuration.",
		}, nil
	}

	// Parse the query to determine intent and extract project information
	intent, projectInfo, enhancedQuery := s.parseQuery(ctx, query)

	// Create and set up execution context with project path if available
	execCtx := s.setupExecutionContext(ctx, userID, projectInfo)

	// Execute based on intent
	switch intent {
	case "project_list":
		return s.handleProjectListQuery(ctx)
	case "project_info":
		return s.handleProjectInfoQuery(ctx, projectInfo)
	case "file_operation":
		return s.handleFileOperation(ctx, enhancedQuery, execCtx)
	case "git_operation":
		return s.handleGitOperation(ctx, enhancedQuery, execCtx)
	case "general_query":
		return s.handleGeneralQuery(ctx, enhancedQuery, execCtx)
	case "ambiguous":
		return s.handleAmbiguousQuery(ctx, query)
	default:
		return s.handleGeneralQuery(ctx, enhancedQuery, execCtx)
	}
}

// setupExecutionContext creates and initializes an execution context with appropriate settings
func (s *Service) setupExecutionContext(ctx context.Context, userID int64, project *Project) ExecutionContext {
	execCtx := ExecutionContext{
		UserID:      userID,
		WorkingDir:  "/", // Default working directory
		Timeout:     30 * time.Second,
		Environment: make(map[string]string),
	}

	// Set project path if identified
	if project != nil {
		slog.InfoContext(ctx, "Setting execution context to project directory",
			slog.String("project", project.Name),
			slog.String("path", project.Path))

		execCtx.ProjectPath = project.Path
		execCtx.WorkingDir = project.Path

		// Add project-specific environment variables if needed
		execCtx.Environment["PROJECT_NAME"] = project.Name
		execCtx.Environment["PROJECT_TYPE"] = string(project.Type)
	}

	return execCtx
}

// parseQuery analyzes the user query and determines intent
func (s *Service) parseQuery(ctx context.Context, query string) (intent string, project *Project, enhancedQuery string) {
	query = strings.ToLower(strings.TrimSpace(query))

	// Check for project listing requests
	if strings.Contains(query, "list project") || strings.Contains(query, "show project") || query == "projects" {
		return "project_list", nil, query
	}

	// Try to identify project from shortcuts or names
	project = s.identifyProjectFromQuery(ctx, query)

	// Check for git operations
	if strings.Contains(query, "git ") {
		return "git_operation", project, query
	}

	// Check for file operations
	if s.isFileOperation(query) {
		return "file_operation", project, query
	}

	// Check for project info requests
	if project != nil && (strings.Contains(query, "info") || strings.Contains(query, "about")) {
		return "project_info", project, query
	}

	// If multiple projects might be involved
	if strings.Contains(query, "all") || strings.Contains(query, "every") {
		return "ambiguous", nil, query
	}

	return "general_query", project, query
}

// identifyProjectFromQuery tries to find a project mentioned in the query
func (s *Service) identifyProjectFromQuery(ctx context.Context, query string) *Project {
	index, err := s.projectScanner.GetProjectIndex(ctx)
	if err != nil {
		return nil
	}

	slog.DebugContext(ctx, "Identifying project from query",
		slog.String("query", query),
		slog.Int("project_count", len(index.Projects)),
		slog.Int("shortcut_count", len(index.Shortcuts)),
		slog.Any("projects", index.Projects),
		slog.Any("shortcuts", index.Shortcuts),
	)

	// Check shortcuts first
	for shortcut, projectName := range index.Shortcuts {
		if strings.Contains(query, shortcut) {
			project, _ := s.GetProjectByName(ctx, projectName)
			return project
		}
	}

	// Check project names
	for _, project := range index.Projects {
		if strings.Contains(query, strings.ToLower(project.Name)) {
			return &project
		}
	}

	return nil
}

// isFileOperation checks if the query involves file operations
func (s *Service) isFileOperation(query string) bool {
	fileOps := []string{"read", "show", "cat", "list", "ls", "write", "create", "edit", "delete", "rm"}
	for _, op := range fileOps {
		if strings.Contains(query, op) {
			return true
		}
	}
	return false
}

// Query handlers
func (s *Service) handleProjectListQuery(ctx context.Context) (*QueryResult, error) {
	projects, err := s.ListProjects(ctx)
	if err != nil {
		return &QueryResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to list projects: %v", err),
		}, nil
	}

	response := s.formatProjectList(projects)
	return &QueryResult{
		Success:  true,
		Response: response,
		Projects: projects,
	}, nil
}

func (s *Service) handleProjectInfoQuery(ctx context.Context, project *Project) (*QueryResult, error) {
	if project == nil {
		return &QueryResult{
			Success: false,
			Error:   "Project not specified or not found",
		}, nil
	}

	response := s.formatProjectInfo(*project)
	return &QueryResult{
		Success:  true,
		Response: response,
		Projects: []Project{*project},
	}, nil
}

func (s *Service) handleFileOperation(ctx context.Context, query string, execCtx ExecutionContext) (*QueryResult, error) {
	// Enhance query with project context if available
	if execCtx.ProjectPath != "" {
		query = fmt.Sprintf("In project at %s: %s", execCtx.ProjectPath, query)
		slog.InfoContext(ctx, "Executing file operation in project directory",
			slog.String("project_path", execCtx.ProjectPath),
			slog.String("working_dir", execCtx.WorkingDir))
	} else {
		slog.InfoContext(ctx, "Executing file operation with no specific project context")
	}

	return s.aiExecutor.ExecuteCommand(ctx, query, execCtx)
}

func (s *Service) handleGitOperation(ctx context.Context, query string, execCtx ExecutionContext) (*QueryResult, error) {
	// Extract git command
	gitCmd := strings.TrimPrefix(query, "git ")

	// Log git operation details
	if execCtx.ProjectPath != "" {
		slog.InfoContext(ctx, "Executing git command in project directory",
			slog.String("git_command", gitCmd),
			slog.String("project_path", execCtx.ProjectPath),
			slog.String("working_dir", execCtx.WorkingDir))
	} else {
		slog.InfoContext(ctx, "Executing git command with no specific project context",
			slog.String("git_command", gitCmd))
	}

	return s.aiExecutor.ExecuteGitCommand(ctx, gitCmd, execCtx)
}

func (s *Service) handleGeneralQuery(ctx context.Context, query string, execCtx ExecutionContext) (*QueryResult, error) {
	// Log general query execution details
	if execCtx.ProjectPath != "" {
		slog.InfoContext(ctx, "Executing general query in project directory",
			slog.String("query", query),
			slog.String("project_path", execCtx.ProjectPath),
			slog.String("working_dir", execCtx.WorkingDir))
	} else {
		slog.InfoContext(ctx, "Executing general query with no specific project context",
			slog.String("query", query))
	}

	return s.aiExecutor.ExecuteCommand(ctx, query, execCtx)
}

func (s *Service) handleAmbiguousQuery(ctx context.Context, query string) (*QueryResult, error) {
	projects, err := s.ListProjects(ctx)
	if err != nil {
		return &QueryResult{
			Success: false,
			Error:   "Failed to retrieve project list for clarification",
		}, nil
	}

	suggestions := make([]string, len(projects))
	for i, project := range projects {
		suggestions[i] = fmt.Sprintf("%d. %s", i+1, project.Name)
	}

	return &QueryResult{
		Success:     false,
		Response:    "Your query affects multiple projects. Please specify which project or reply with 'all' for all projects.",
		Projects:    projects,
		Suggestions: suggestions,
	}, nil
}

// Helper methods for formatting responses
func (s *Service) formatProjectList(projects []Project) string {
	if len(projects) == 0 {
		return "No projects found."
	}

	var builder strings.Builder
	builder.WriteString("📁 **Available Projects:**\n\n")

	for _, project := range projects {
		builder.WriteString(fmt.Sprintf("**%s** (%s)\n", project.Name, project.Type))
		if len(project.Shortcuts) > 0 {
			builder.WriteString(fmt.Sprintf("Shortcuts: %s\n", strings.Join(project.Shortcuts, ", ")))
		}
		if project.Purpose != "" {
			builder.WriteString(fmt.Sprintf("Purpose: %s\n", project.Purpose))
		}
		builder.WriteString(fmt.Sprintf("Path: `%s`\n\n", project.Path))
	}

	return builder.String()
}

func (s *Service) formatProjectInfo(project Project) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("📁 **%s**\n\n", project.Name))
	builder.WriteString(fmt.Sprintf("**Type:** %s\n", project.Type))
	builder.WriteString(fmt.Sprintf("**Status:** %s\n", project.Status))
	builder.WriteString(fmt.Sprintf("**Path:** `%s`\n", project.Path))

	if len(project.TechStack) > 0 {
		builder.WriteString(fmt.Sprintf("**Tech Stack:** %s\n", strings.Join(project.TechStack, ", ")))
	}

	if project.Purpose != "" {
		builder.WriteString(fmt.Sprintf("**Purpose:** %s\n", project.Purpose))
	}

	if len(project.Shortcuts) > 0 {
		builder.WriteString(fmt.Sprintf("**Shortcuts:** %s\n", strings.Join(project.Shortcuts, ", ")))
	}

	if len(project.KeyFiles) > 0 {
		builder.WriteString(fmt.Sprintf("**Key Files:** %s\n", strings.Join(project.KeyFiles, ", ")))
	}

	if project.LastCommit != nil {
		builder.WriteString(fmt.Sprintf("**Last Commit:** %s\n", project.LastCommit.Format("2006-01-02 15:04:05")))
	}

	return builder.String()
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

// sendErrorToUser sends error message to user
func (s *Service) sendErrorToUser(ctx context.Context, userID int64, message string) error {
	return s.telegramNotifier.SendMessage(ctx, userID, fmt.Sprintf("❌ %s", message))
}
