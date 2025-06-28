package configprovider_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/knightazura/kumote/internal/assistant/adapters/configprovider"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in tests
	}))
}

func TestConfigProvider_NewConfigProvider_CreatesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	logger := getTestLogger()

	// Create config provider - should create default config
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)
	assert.NotNil(t, cp)

	// Verify config file was created
	assert.FileExists(t, configPath)

	// Verify default scan config
	ctx := context.Background()
	scanConfig, err := cp.GetScanConfig(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, scanConfig.BasePath)
	assert.Contains(t, scanConfig.Indicators, "go.mod")
	assert.Contains(t, scanConfig.Indicators, "package.json")
	assert.Equal(t, 3, scanConfig.MaxDepth)
	assert.Equal(t, int64(1024), scanConfig.MinProjectSize)
}

func TestConfigProvider_LoadExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "existing-config.yaml")

	// Create a test config file
	configContent := `
scan:
  base_path: /test/development
  indicators:
    - go.mod
    - package.json
  excluded_dirs:
    - node_modules
    - .git
  max_depth: 5
  min_project_size: 2048
  shortcuts:
    test: TestProject
  update_schedule: "0 10 * * *"

users:
  allowed_user_ids:
    - 123456789
    - 987654321

rate_limit:
  requests_per_minute: 20
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := getTestLogger()

	// Load existing config
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)
	assert.NotNil(t, cp)

	ctx := context.Background()

	// Verify scan config
	scanConfig, err := cp.GetScanConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "/test/development", scanConfig.BasePath)
	assert.Equal(t, 5, scanConfig.MaxDepth)
	assert.Equal(t, int64(2048), scanConfig.MinProjectSize)
	assert.Equal(t, "TestProject", scanConfig.Shortcuts["test"])

	// Verify user config
	userIDs, err := cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 2)
	assert.Contains(t, userIDs, int64(123456789))
	assert.Contains(t, userIDs, int64(987654321))

	// Verify rate limit config
	rateLimit, err := cp.GetRateLimit(ctx)
	require.NoError(t, err)
	assert.Equal(t, 20, rateLimit)
}

func TestConfigProvider_UpdateScanConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "update-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Update scan config
	newConfig := &core.ScanConfig{
		BasePath: "/new/path",
		Indicators: []string{
			"go.mod",
			"requirements.txt",
		},
		ExcludedDirs:   []string{"vendor", "dist"},
		MaxDepth:       4,
		MinProjectSize: 512,
		Shortcuts: map[string]string{
			"new": "NewProject",
		},
		UpdateSchedule: "0 8 * * *",
	}

	err = cp.UpdateScanConfig(ctx, newConfig)
	require.NoError(t, err)

	// Verify changes were saved
	reloadedConfig, err := cp.GetScanConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "/new/path", reloadedConfig.BasePath)
	assert.Equal(t, 4, reloadedConfig.MaxDepth)
	assert.Equal(t, int64(512), reloadedConfig.MinProjectSize)
	assert.Equal(t, "NewProject", reloadedConfig.Shortcuts["new"])
	assert.Len(t, reloadedConfig.Indicators, 2)
}

func TestConfigProvider_AddRemoveAllowedUser(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "user-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Initially no users
	userIDs, err := cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 0)

	// Add a user
	err = cp.AddAllowedUser(ctx, 123456789)
	require.NoError(t, err)

	// Verify user was added
	userIDs, err = cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 1)
	assert.Contains(t, userIDs, int64(123456789))

	// Add another user
	err = cp.AddAllowedUser(ctx, 987654321)
	require.NoError(t, err)

	// Verify both users
	userIDs, err = cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 2)
	assert.Contains(t, userIDs, int64(123456789))
	assert.Contains(t, userIDs, int64(987654321))

	// Try adding same user again (should be no-op)
	err = cp.AddAllowedUser(ctx, 123456789)
	require.NoError(t, err)

	userIDs, err = cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 2) // Still 2 users

	// Remove a user
	err = cp.RemoveAllowedUser(ctx, 123456789)
	require.NoError(t, err)

	// Verify user was removed
	userIDs, err = cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 1)
	assert.Contains(t, userIDs, int64(987654321))
	assert.NotContains(t, userIDs, int64(123456789))

	// Try removing non-existent user (should be no-op)
	err = cp.RemoveAllowedUser(ctx, 111111111)
	require.NoError(t, err)

	userIDs, err = cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 1) // Still 1 user
}

func TestConfigProvider_EnvironmentOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "env-config.yaml")

	// Set environment variables
	os.Setenv("DEVELOPMENT_PATH", "/env/development")
	os.Setenv("SCAN_MAX_DEPTH", "7")
	os.Setenv("MIN_PROJECT_SIZE", "4096")
	os.Setenv("ALLOWED_USER_IDS", "111222333,444555666")
	os.Setenv("OWNER_USER_ID", "777888999")
	os.Setenv("RATE_LIMIT_PER_MINUTE", "50")

	defer func() {
		os.Unsetenv("DEVELOPMENT_PATH")
		os.Unsetenv("SCAN_MAX_DEPTH")
		os.Unsetenv("MIN_PROJECT_SIZE")
		os.Unsetenv("ALLOWED_USER_IDS")
		os.Unsetenv("OWNER_USER_ID")
		os.Unsetenv("RATE_LIMIT_PER_MINUTE")
	}()

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test scan config with environment overrides
	scanConfig, err := cp.GetScanConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "/env/development", scanConfig.BasePath)
	assert.Equal(t, 7, scanConfig.MaxDepth)
	assert.Equal(t, int64(4096), scanConfig.MinProjectSize)

	// Test user IDs from environment (ALLOWED_USER_IDS takes precedence)
	userIDs, err := cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 2)
	assert.Contains(t, userIDs, int64(111222333))
	assert.Contains(t, userIDs, int64(444555666))

	// Test rate limit from environment
	rateLimit, err := cp.GetRateLimit(ctx)
	require.NoError(t, err)
	assert.Equal(t, 50, rateLimit)
}

func TestConfigProvider_OwnerUserIDFallback(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "owner-config.yaml")

	// Set only OWNER_USER_ID (no ALLOWED_USER_IDS)
	os.Setenv("OWNER_USER_ID", "555666777")
	defer os.Unsetenv("OWNER_USER_ID")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Should use OWNER_USER_ID as single allowed user
	userIDs, err := cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 1)
	assert.Contains(t, userIDs, int64(555666777))
}

func TestConfigProvider_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-config.yaml")

	// Create invalid config file
	invalidContent := `
scan:
  base_path: "/invalid/path/that/does/not/exist/surely"  # Invalid: path doesn't exist
  max_depth: -1   # Invalid: must be positive
  min_project_size: -100  # Invalid: cannot be negative
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Should fail validation
	_, err = cp.GetScanConfig(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scan config")
}

func TestConfigProvider_ValidateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "valid-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Add some allowed users for validation
	err = cp.AddAllowedUser(ctx, 123456789)
	require.NoError(t, err)

	// Validate config - should pass
	err = cp.ValidateConfig(ctx)
	assert.NoError(t, err)
}

func TestConfigProvider_ReloadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "reload-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Get initial rate limit
	initialRateLimit, err := cp.GetRateLimit(ctx)
	require.NoError(t, err)

	// Manually modify config file
	modifiedContent := `
scan:
  base_path: /home/user/Development
  indicators:
    - go.mod
    - package.json
  excluded_dirs:
    - node_modules
  max_depth: 3
  min_project_size: 1024
  shortcuts: {}
  update_schedule: "0 9 * * *"

users:
  allowed_user_ids: []

rate_limit:
  requests_per_minute: 99  # Changed value
`

	err = os.WriteFile(configPath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Reload config
	err = cp.ReloadConfig(ctx)
	require.NoError(t, err)

	// Verify new rate limit is loaded
	newRateLimit, err := cp.GetRateLimit(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, initialRateLimit, newRateLimit)
	assert.Equal(t, 99, newRateLimit)
}

func TestConfigProvider_GetConfigSummary(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "summary-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Add some users
	err = cp.AddAllowedUser(ctx, 123456789)
	require.NoError(t, err)
	err = cp.AddAllowedUser(ctx, 987654321)
	require.NoError(t, err)

	// Get config summary
	summary := cp.GetConfigSummary(ctx)
	require.NotNil(t, summary)

	// Verify summary structure
	assert.Contains(t, summary, "scan")
	assert.Contains(t, summary, "users")
	assert.Contains(t, summary, "rate_limit")
	assert.Contains(t, summary, "config_file")

	// Verify scan summary
	scanSummary := summary["scan"].(map[string]interface{})
	assert.Contains(t, scanSummary, "base_path")
	assert.Contains(t, scanSummary, "max_depth")
	assert.Contains(t, scanSummary, "indicators_count")

	// Verify users summary
	usersSummary := summary["users"].(map[string]interface{})
	assert.Equal(t, 2, usersSummary["allowed_count"])

	// Verify rate limit summary
	rateLimitSummary := summary["rate_limit"].(map[string]interface{})
	assert.Contains(t, rateLimitSummary, "requests_per_minute")

	// Verify config file info
	configFileSummary := summary["config_file"].(map[string]interface{})
	assert.Equal(t, configPath, configFileSummary["path"])
}

func TestConfigProvider_ParseUserIDsFromEnv_InvalidIDs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "parse-config.yaml")

	// Set environment with some invalid IDs
	os.Setenv("ALLOWED_USER_IDS", "123456789,invalid,987654321,,notanumber")
	defer os.Unsetenv("ALLOWED_USER_IDS")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Should only parse valid IDs
	userIDs, err := cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, 2) // Only 2 valid IDs
	assert.Contains(t, userIDs, int64(123456789))
	assert.Contains(t, userIDs, int64(987654321))
}

func TestConfigProvider_GetConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "path-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	// Verify path is returned correctly
	assert.Equal(t, configPath, cp.GetConfigPath())
}

func TestConfigProvider_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "malformed-config.yaml")

	// Create malformed YAML
	malformedContent := `
scan:
  base_path: /test
  indicators:
    - go.mod
    - package.json
  max_depth: invalid_yaml_structure
    missing_colon_here
      - nested_incorrectly
users:
  allowed_user_ids: [123456789
rate_limit:
  requests_per_minute: "not_a_number"
`

	err := os.WriteFile(configPath, []byte(malformedContent), 0644)
	require.NoError(t, err)

	logger := getTestLogger()

	// Should fail to create config provider due to malformed YAML
	_, err = configprovider.NewConfigProvider(configPath, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML config")
}

func TestConfigProvider_UpdateScanConfig_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-update-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Try to update with invalid config
	invalidConfig := &core.ScanConfig{
		BasePath:       "",         // Invalid: empty path
		Indicators:     []string{}, // Invalid: no indicators
		ExcludedDirs:   []string{},
		MaxDepth:       0,  // Invalid: must be at least 1
		MinProjectSize: -1, // Invalid: cannot be negative
		Shortcuts:      map[string]string{},
	}

	err = cp.UpdateScanConfig(ctx, invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scan config")
}

func TestConfigProvider_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "concurrent-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test concurrent reads and writes
	const numGoroutines = 10
	errChan := make(chan error, numGoroutines*2)

	// Start goroutines for concurrent access
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			// Concurrent reads
			_, err := cp.GetScanConfig(ctx)
			errChan <- err

			// Concurrent user additions
			err = cp.AddAllowedUser(ctx, int64(1000000+id))
			errChan <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines*2; i++ {
		err := <-errChan
		// Some operations might conflict, but shouldn't crash
		if err != nil {
			t.Logf("Concurrent operation error (expected): %v", err)
		}
	}

	// Verify final state is consistent
	userIDs, err := cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	// Should have at least some users added (exact count may vary due to concurrency)
	assert.GreaterOrEqual(t, len(userIDs), 1)
}

func TestConfigProvider_EmptyConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty-config.yaml")

	// Create empty config file
	err := os.WriteFile(configPath, []byte(""), 0644)
	require.NoError(t, err)

	logger := getTestLogger()

	// Should handle empty file gracefully by treating it as missing
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Should work with default values
	scanConfig, err := cp.GetScanConfig(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, scanConfig.BasePath)
}

func TestConfigProvider_DirectoryPermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()

	// Create directory with no write permissions
	restrictedDir := filepath.Join(tmpDir, "restricted")
	err := os.MkdirAll(restrictedDir, 0444) // Read-only
	require.NoError(t, err)

	configPath := filepath.Join(restrictedDir, "config.yaml")

	logger := getTestLogger()

	// Should fail to create config in read-only directory
	_, err = configprovider.NewConfigProvider(configPath, logger)
	assert.Error(t, err)
}

func TestConfigProvider_LargeUserList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large user list test in short mode")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "large-user-config.yaml")

	logger := getTestLogger()
	cp, err := configprovider.NewConfigProvider(configPath, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Add many users
	const numUsers = 1000
	for i := 0; i < numUsers; i++ {
		err := cp.AddAllowedUser(ctx, int64(1000000+i))
		require.NoError(t, err)
	}

	// Verify all users are present
	userIDs, err := cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, numUsers)

	// Test removal performance
	for i := 0; i < 100; i++ {
		err := cp.RemoveAllowedUser(ctx, int64(1000000+i))
		require.NoError(t, err)
	}

	// Verify removals
	userIDs, err = cp.GetAllowedUserIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, userIDs, numUsers-100)
}
