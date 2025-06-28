package userrepository_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/knightazura/kumote/internal/assistant/adapters/userrepository"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in tests
	}))
}

func TestUserRepository_NewUserRepository_WithEnvVar(t *testing.T) {
	// Set environment variable for test
	os.Setenv("ALLOWED_USER_IDS", "123456789,987654321")
	defer os.Unsetenv("ALLOWED_USER_IDS")

	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()

	// Test that users from env var are allowed
	assert.True(t, repo.IsUserAllowed(ctx, 123456789))
	assert.True(t, repo.IsUserAllowed(ctx, 987654321))
	assert.False(t, repo.IsUserAllowed(ctx, 111111111))

	assert.Equal(t, 2, repo.GetAllowedUserCount())
}

func TestUserRepository_NewUserRepository_WithOwnerEnvVar(t *testing.T) {
	// Clear ALLOWED_USER_IDS and set OWNER_USER_ID
	os.Unsetenv("ALLOWED_USER_IDS")
	os.Setenv("OWNER_USER_ID", "555666777")
	defer os.Unsetenv("OWNER_USER_ID")

	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()

	// Test that owner user is allowed
	assert.True(t, repo.IsUserAllowed(ctx, 555666777))
	assert.False(t, repo.IsUserAllowed(ctx, 123456789))

	assert.Equal(t, 1, repo.GetAllowedUserCount())
}

func TestUserRepository_GetUser_AllowedUser(t *testing.T) {
	os.Setenv("ALLOWED_USER_IDS", "123456789")
	defer os.Unsetenv("ALLOWED_USER_IDS")

	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()
	userID := int64(123456789)

	// Get user - should create new user since they're allowed
	user, err := repo.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, "Remote User", user.FirstName)
	assert.True(t, user.IsAllowed)
}

func TestUserRepository_GetUser_NotAllowedUser(t *testing.T) {
	os.Setenv("ALLOWED_USER_IDS", "123456789")
	defer os.Unsetenv("ALLOWED_USER_IDS")

	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()
	userID := int64(987654321) // Not in allowed list

	// Get user - should return error
	user, err := repo.GetUser(ctx, userID)
	assert.Error(t, err)
	assert.Equal(t, core.ErrUserNotFound, err)
	assert.Nil(t, user)
}

func TestUserRepository_SaveUser_Valid(t *testing.T) {
	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()
	user := &core.User{
		ID:        123456789,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		IsAllowed: true,
	}

	// Save user
	err := repo.SaveUser(ctx, user)
	require.NoError(t, err)

	// Verify user was saved and is now allowed
	assert.True(t, repo.IsUserAllowed(ctx, user.ID))

	// Retrieve user
	savedUser, err := repo.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, savedUser.ID)
	assert.Equal(t, user.Username, savedUser.Username)
	assert.Equal(t, user.FirstName, savedUser.FirstName)
	assert.Equal(t, user.LastName, savedUser.LastName)
	assert.True(t, savedUser.IsAllowed)
}

func TestUserRepository_SaveUser_Invalid(t *testing.T) {
	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()

	// Test with invalid user (zero ID)
	invalidUser := &core.User{
		ID:        0, // Invalid
		FirstName: "Test",
		IsAllowed: true,
	}

	err := repo.SaveUser(ctx, invalidUser)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID cannot be zero")
}

func TestUserRepository_GetAllowedUsers(t *testing.T) {
	os.Setenv("ALLOWED_USER_IDS", "123456789,987654321")
	defer os.Unsetenv("ALLOWED_USER_IDS")

	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()

	// Save some user info
	user1 := &core.User{
		ID:        123456789,
		FirstName: "User",
		LastName:  "One",
		IsAllowed: true,
	}
	user2 := &core.User{
		ID:        987654321,
		FirstName: "User",
		LastName:  "Two",
		IsAllowed: true,
	}

	require.NoError(t, repo.SaveUser(ctx, user1))
	require.NoError(t, repo.SaveUser(ctx, user2))

	// Get all allowed users
	users, err := repo.GetAllowedUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Verify all users are allowed
	for _, user := range users {
		assert.True(t, user.IsAllowed)
		assert.Contains(t, []int64{123456789, 987654321}, user.ID)
	}
}

func TestUserRepository_AddRemoveAllowedUser(t *testing.T) {
	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()
	userID := int64(123456789)

	// Initially user should not be allowed
	assert.False(t, repo.IsUserAllowed(ctx, userID))

	// Add user to allowed list
	err := repo.AddAllowedUser(ctx, userID)
	require.NoError(t, err)

	// Now user should be allowed
	assert.True(t, repo.IsUserAllowed(ctx, userID))
	assert.Equal(t, 1, repo.GetAllowedUserCount())

	// Remove user from allowed list
	err = repo.RemoveAllowedUser(ctx, userID)
	require.NoError(t, err)

	// User should no longer be allowed
	assert.False(t, repo.IsUserAllowed(ctx, userID))
	assert.Equal(t, 0, repo.GetAllowedUserCount())
}

func TestUserRepository_AddRemoveAllowedUser_WithExistingUser(t *testing.T) {
	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()
	userID := int64(123456789)

	// Save user first (not allowed initially)
	user := &core.User{
		ID:        userID,
		FirstName: "Test",
		IsAllowed: false,
	}
	require.NoError(t, repo.SaveUser(ctx, user))

	// Add user to allowed list
	err := repo.AddAllowedUser(ctx, userID)
	require.NoError(t, err)

	// Verify user's allowed status was updated
	updatedUser, err := repo.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.True(t, updatedUser.IsAllowed)

	// Remove user from allowed list
	err = repo.RemoveAllowedUser(ctx, userID)
	require.NoError(t, err)

	// Verify user's allowed status was updated
	updatedUser, err = repo.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.False(t, updatedUser.IsAllowed)
}

func TestUserRepository_ParseUserIDsFromEnv_InvalidIDs(t *testing.T) {
	// Set environment variable with some invalid IDs
	os.Setenv("ALLOWED_USER_IDS", "123456789,invalid,987654321,")
	defer os.Unsetenv("ALLOWED_USER_IDS")

	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	ctx := context.Background()

	// Should only load valid IDs
	assert.True(t, repo.IsUserAllowed(ctx, 123456789))
	assert.True(t, repo.IsUserAllowed(ctx, 987654321))
	assert.False(t, repo.IsUserAllowed(ctx, 0)) // Invalid ID not loaded

	assert.Equal(t, 2, repo.GetAllowedUserCount())
}

func TestUserRepository_NoConfiguredUsers(t *testing.T) {
	// Clear all environment variables
	os.Unsetenv("ALLOWED_USER_IDS")
	os.Unsetenv("OWNER_USER_ID")

	logger := getTestLogger()
	repo := userrepository.NewUserRepository(logger)

	// Should have no allowed users
	assert.Equal(t, 0, repo.GetAllowedUserCount())

	ctx := context.Background()
	assert.False(t, repo.IsUserAllowed(ctx, 123456789))
}
