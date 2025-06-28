package core_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// Test Helper Functions

func createTestService() (*core.Service, *MockProjectScanner, *MockAICodeExecutor, *MockUserRepository, *MockRateLimiter, *MockCommandRepository, *MockLogger, *MockMetricsCollector) {
	mockProjectScanner := &MockProjectScanner{}
	mockAIExecutor := &MockAICodeExecutor{}
	mockTelegramNotifier := &MockTelegramNotifier{}
	mockAudioTranscriber := &MockAudioTranscriber{}
	mockUserRepo := &MockUserRepository{}
	mockCommandRepo := &MockCommandRepository{}
	mockMetricsCollector := &MockMetricsCollector{}
	mockConfigProvider := &MockConfigProvider{}
	mockLogger := &MockLogger{}
	mockRateLimiter := &MockRateLimiter{}

	service := core.NewService(
		mockProjectScanner,
		mockAIExecutor,
		mockTelegramNotifier,
		mockAudioTranscriber,
		mockUserRepo,
		mockCommandRepo,
		mockMetricsCollector,
		mockConfigProvider,
		mockLogger,
		mockRateLimiter,
	)

	return service, mockProjectScanner, mockAIExecutor, mockUserRepo, mockRateLimiter, mockCommandRepo, mockLogger, mockMetricsCollector
}

func createTestProjectIndex() *core.ProjectIndex {
	return &core.ProjectIndex{
		Projects: []core.Project{
			{
				Name:      "TaqwaBoard",
				Path:      "/home/user/Development/taqwaboard",
				Type:      core.ProjectTypeGo,
				TechStack: []string{"go", "vue"},
				Purpose:   "Mosque prayer times display dashboard",
				KeyFiles:  []string{"main.go", "go.mod"},
				Status:    core.ProjectStatusActive,
				Shortcuts: []string{"taqwa"},
			},
			{
				Name:      "CarLogbook",
				Path:      "/home/user/Development/car-logbook",
				Type:      core.ProjectTypeVue,
				TechStack: []string{"vue", "nodejs"},
				Purpose:   "Car maintenance and expense tracking",
				KeyFiles:  []string{"package.json", "src/main.js"},
				Status:    core.ProjectStatusActive,
				Shortcuts: []string{"car"},
			},
		},
		UpdatedAt:  time.Now(),
		TotalCount: 2,
		ScanPath:   "/home/user/Development",
		Shortcuts: map[string]string{
			"taqwa": "TaqwaBoard",
			"car":   "CarLogbook",
		},
	}
}

func createTestCommand(userID int64, text string) core.Command {
	return core.Command{
		ID:        "test-cmd-123",
		UserID:    userID,
		Text:      text,
		Timestamp: time.Now(),
	}
}

func createTestUser(userID int64, isAllowed bool) *core.User {
	return &core.User{
		ID:        userID,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		IsAllowed: isAllowed,
	}
}

// Actual Tests

func TestService_ProcessCommand_Success(t *testing.T) {
	ctx := context.Background()
	service, mockProjectScanner, mockAIExecutor, mockUserRepo, mockRateLimiter, mockCommandRepo, mockLogger, mockMetricsCollector := createTestService()

	userID := int64(123456789)
	cmd := createTestCommand(userID, "show taqwa main.go")
	testUser := createTestUser(userID, true)
	testIndex := createTestProjectIndex()

	// Setup expectations
	mockRateLimiter.On("IsAllowed", ctx, userID).Return(true)
	mockRateLimiter.On("RecordRequest", ctx, userID).Return(nil)
	mockUserRepo.On("GetUser", ctx, userID).Return(testUser, nil)
	mockProjectScanner.On("GetProjectIndex", ctx).Return(testIndex, nil)
	mockAIExecutor.On("IsAvailable", ctx).Return(true)

	// Add expectation for SaveCommand
	mockCommandRepo.On("SaveCommand", ctx, mock.AnythingOfType("*core.Command")).Return(nil)

	// Add expectations for Logger
	mockLogger.On("Info", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return()
	mockLogger.On("Debug", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return()

	// Add expectation for MetricsCollector
	mockMetricsCollector.On("RecordCommandExecution", ctx, mock.AnythingOfType("core.CommandMetrics")).Return(nil)

	expectedResult := &core.QueryResult{
		Success:  true,
		Response: "package main\n\nfunc main() {\n    // TaqwaBoard main function\n}",
		Files: []core.FileContent{
			{
				Path:    "/home/user/Development/taqwaboard/main.go",
				Name:    "main.go",
				Content: "package main\n\nfunc main() {\n    // TaqwaBoard main function\n}",
			},
		},
	}

	mockAIExecutor.On("ExecuteCommand", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("core.ExecutionContext")).Return(expectedResult, nil)

	// Execute
	result, err := service.ProcessCommand(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.Response, "main")
	assert.Len(t, result.Files, 1)

	// Verify all expectations were met
	mockRateLimiter.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
	mockProjectScanner.AssertExpectations(t)
	mockAIExecutor.AssertExpectations(t)
}

func TestService_ProcessCommand_RateLimitExceeded(t *testing.T) {
	ctx := context.Background()
	service, _, _, _, mockRateLimiter, _, _, _ := createTestService()

	userID := int64(123456789)
	cmd := createTestCommand(userID, "list projects")

	// Setup rate limit exceeded
	mockRateLimiter.On("IsAllowed", ctx, userID).Return(false)

	// Execute
	result, err := service.ProcessCommand(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Rate limit exceeded")

	mockRateLimiter.AssertExpectations(t)
}

func TestService_ProcessCommand_UserNotAuthorized(t *testing.T) {
	ctx := context.Background()
	service, _, _, mockUserRepo, mockRateLimiter, _, _, _ := createTestService()

	userID := int64(123456789)
	cmd := createTestCommand(userID, "list projects")
	unauthorizedUser := createTestUser(userID, false)

	// Setup expectations
	mockRateLimiter.On("IsAllowed", ctx, userID).Return(true)
	mockRateLimiter.On("RecordRequest", ctx, userID).Return(nil)
	mockUserRepo.On("GetUser", ctx, userID).Return(unauthorizedUser, nil)

	// Execute
	result, err := service.ProcessCommand(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "not authorized")

	mockRateLimiter.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}

func TestService_ProcessCommand_AICodeExecutorUnavailable(t *testing.T) {
	ctx := context.Background()
	service, mockProjectScanner, mockAIExecutor, mockUserRepo, mockRateLimiter, mockCommandRepo, mockLogger, mockMetricsCollector := createTestService()

	userID := int64(123456789)
	cmd := createTestCommand(userID, "show taqwa main.go")
	testUser := createTestUser(userID, true)

	// Setup expectations
	mockRateLimiter.On("IsAllowed", ctx, userID).Return(true)
	mockRateLimiter.On("RecordRequest", ctx, userID).Return(nil)
	mockUserRepo.On("GetUser", ctx, userID).Return(testUser, nil)
	mockAIExecutor.On("IsAvailable", ctx).Return(false)

	// Add expectation for SaveCommand
	mockCommandRepo.On("SaveCommand", ctx, mock.AnythingOfType("*core.Command")).Return(nil)

	// Add expectations for Logger
	mockLogger.On("Info", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return()
	mockLogger.On("Debug", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).Return()

	// Add expectation for MetricsCollector
	mockMetricsCollector.On("RecordCommandExecution", ctx, mock.AnythingOfType("core.CommandMetrics")).Return(nil)

	// Execute
	result, err := service.ProcessCommand(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "AI Code Executor is not available")

	mockRateLimiter.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
	mockProjectScanner.AssertExpectations(t)
	mockAIExecutor.AssertExpectations(t)
}

func TestService_ListProjects_Success(t *testing.T) {
	ctx := context.Background()
	service, mockProjectScanner, _, _, _, _, _, _ := createTestService()

	testIndex := createTestProjectIndex()
	mockProjectScanner.On("GetProjectIndex", ctx).Return(testIndex, nil)

	// Execute
	projects, err := service.ListProjects(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, projects, 2)
	assert.Equal(t, "TaqwaBoard", projects[0].Name)
	assert.Equal(t, "CarLogbook", projects[1].Name)

	// Verify mock expectations
	mockProjectScanner.AssertExpectations(t)
}

// Mocks for all dependencies

type MockProjectScanner struct {
	mock.Mock
}

func (m *MockProjectScanner) ScanProjects(ctx context.Context, config core.ScanConfig) (*core.ProjectIndex, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(*core.ProjectIndex), args.Error(1)
}

func (m *MockProjectScanner) UpdateIndex(ctx context.Context) (*core.ProjectIndex, error) {
	args := m.Called(ctx)
	return args.Get(0).(*core.ProjectIndex), args.Error(1)
}

func (m *MockProjectScanner) GetProjectIndex(ctx context.Context) (*core.ProjectIndex, error) {
	args := m.Called(ctx)
	return args.Get(0).(*core.ProjectIndex), args.Error(1)
}

func (m *MockProjectScanner) SaveProjectIndex(ctx context.Context, index *core.ProjectIndex) error {
	args := m.Called(ctx, index)
	return args.Error(0)
}

func (m *MockProjectScanner) LoadProjectIndex(ctx context.Context) (*core.ProjectIndex, error) {
	args := m.Called(ctx)
	return args.Get(0).(*core.ProjectIndex), args.Error(1)
}

type MockAICodeExecutor struct {
	mock.Mock
}

func (m *MockAICodeExecutor) ExecuteCommand(ctx context.Context, command string, execCtx core.ExecutionContext) (*core.QueryResult, error) {
	args := m.Called(ctx, command, execCtx)
	return args.Get(0).(*core.QueryResult), args.Error(1)
}

func (m *MockAICodeExecutor) ReadFile(ctx context.Context, filePath string, execCtx core.ExecutionContext) (*core.FileContent, error) {
	args := m.Called(ctx, filePath, execCtx)
	return args.Get(0).(*core.FileContent), args.Error(1)
}

func (m *MockAICodeExecutor) WriteFile(ctx context.Context, filePath, content string, execCtx core.ExecutionContext) error {
	args := m.Called(ctx, filePath, content, execCtx)
	return args.Error(0)
}

func (m *MockAICodeExecutor) ListFiles(ctx context.Context, dirPath string, execCtx core.ExecutionContext) ([]core.FileContent, error) {
	args := m.Called(ctx, dirPath, execCtx)
	return args.Get(0).([]core.FileContent), args.Error(1)
}

func (m *MockAICodeExecutor) ExecuteGitCommand(ctx context.Context, gitCmd string, execCtx core.ExecutionContext) (*core.QueryResult, error) {
	args := m.Called(ctx, gitCmd, execCtx)
	return args.Get(0).(*core.QueryResult), args.Error(1)
}

func (m *MockAICodeExecutor) IsAvailable(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

type MockTelegramNotifier struct {
	mock.Mock
}

func (m *MockTelegramNotifier) SendMessage(ctx context.Context, userID int64, message string) error {
	args := m.Called(ctx, userID, message)
	return args.Error(0)
}

func (m *MockTelegramNotifier) SendFile(ctx context.Context, userID int64, file io.Reader, filename string) error {
	args := m.Called(ctx, userID, file, filename)
	return args.Error(0)
}

func (m *MockTelegramNotifier) SendFormattedMessage(ctx context.Context, userID int64, message string, parseMode string) error {
	args := m.Called(ctx, userID, message, parseMode)
	return args.Error(0)
}

func (m *MockTelegramNotifier) SendConfirmationRequest(ctx context.Context, userID int64, message string, options []string) error {
	args := m.Called(ctx, userID, message, options)
	return args.Error(0)
}

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUser(ctx context.Context, userID int64) (*core.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*core.User), args.Error(1)
}

func (m *MockUserRepository) SaveUser(ctx context.Context, user *core.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) IsUserAllowed(ctx context.Context, userID int64) bool {
	args := m.Called(ctx, userID)
	return args.Bool(0)
}

func (m *MockUserRepository) GetAllowedUsers(ctx context.Context) ([]core.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]core.User), args.Error(1)
}

type MockCommandRepository struct {
	mock.Mock
}

func (m *MockCommandRepository) SaveCommand(ctx context.Context, cmd *core.Command) error {
	args := m.Called(ctx, cmd)
	return args.Error(0)
}

func (m *MockCommandRepository) GetCommandHistory(ctx context.Context, userID int64, limit int) ([]core.Command, error) {
	args := m.Called(ctx, userID, limit)
	return args.Get(0).([]core.Command), args.Error(1)
}

func (m *MockCommandRepository) GetCommandByID(ctx context.Context, commandID string) (*core.Command, error) {
	args := m.Called(ctx, commandID)
	return args.Get(0).(*core.Command), args.Error(1)
}

type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) RecordCommandExecution(ctx context.Context, metrics core.CommandMetrics) error {
	args := m.Called(ctx, metrics)
	return args.Error(0)
}

func (m *MockMetricsCollector) GetUsageStats(ctx context.Context, userID int64, period string) (map[string]any, error) {
	args := m.Called(ctx, userID, period)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *MockMetricsCollector) GetSystemHealth(ctx context.Context) (map[string]any, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]any), args.Error(1)
}

type MockConfigProvider struct {
	mock.Mock
}

func (m *MockConfigProvider) GetScanConfig(ctx context.Context) (*core.ScanConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).(*core.ScanConfig), args.Error(1)
}

func (m *MockConfigProvider) UpdateScanConfig(ctx context.Context, config *core.ScanConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockConfigProvider) GetAllowedUserIDs(ctx context.Context) ([]int64, error) {
	args := m.Called(ctx)
	return args.Get(0).([]int64), args.Error(1)
}

func (m *MockConfigProvider) GetRateLimit(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(ctx context.Context, message string, fields map[string]any) {
	m.Called(ctx, message, fields)
}

func (m *MockLogger) Error(ctx context.Context, message string, err error, fields map[string]any) {
	m.Called(ctx, message, err, fields)
}

func (m *MockLogger) Debug(ctx context.Context, message string, fields map[string]any) {
	m.Called(ctx, message, fields)
}

func (m *MockLogger) Warn(ctx context.Context, message string, fields map[string]any) {
	m.Called(ctx, message, fields)
}

type MockRateLimiter struct {
	mock.Mock
}

func (m *MockRateLimiter) IsAllowed(ctx context.Context, userID int64) bool {
	args := m.Called(ctx, userID)
	return args.Bool(0)
}

func (m *MockRateLimiter) RecordRequest(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRateLimiter) GetRemainingRequests(ctx context.Context, userID int64) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

type MockAudioTranscriber struct {
	mock.Mock
}

func (m *MockAudioTranscriber) TranscribeAudio(ctx context.Context, audioData io.Reader) (string, error) {
	args := m.Called(ctx, audioData)
	return args.String(0), args.Error(1)
}

func (m *MockAudioTranscriber) IsSupported(ctx context.Context, format string) bool {
	args := m.Called(ctx, format)
	return args.Bool(0)
}
