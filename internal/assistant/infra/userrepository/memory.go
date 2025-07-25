package userrepository

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/izzddalfk/kumote/internal/assistant/core"
	"gopkg.in/validator.v2"
)

type UserRepository struct {
	allowedUserIDs map[int64]bool
	users          map[int64]*core.User
}

type UserRepositoryConfig struct {
	AllowedUserIDsString string `validate:"nonzero"`
}

// NewUserRepository creates a new user repository
func NewUserRepository(config UserRepositoryConfig) (*UserRepository, error) {
	if err := validator.Validate(config); err != nil {
		return nil, err
	}

	repo := &UserRepository{
		allowedUserIDs: make(map[int64]bool),
		users:          make(map[int64]*core.User),
	}

	// Load allowed user IDs from environment or use default
	repo.loadAllowedUsers(config.AllowedUserIDsString)

	return repo, nil
}

// GetUser retrieves user by ID
func (r *UserRepository) GetUser(ctx context.Context, userID int64) (*core.User, error) {
	slog.DebugContext(ctx, "Getting user", "user_id", userID)

	if user, exists := r.users[userID]; exists {
		return user, nil
	}

	// If user doesn't exist but is allowed, create a basic user entry
	if r.IsUserAllowed(ctx, userID) {
		user := &core.User{
			ID:        userID,
			FirstName: "Remote User", // Default name since we don't have Telegram user info
			IsAllowed: true,
		}
		r.users[userID] = user
		slog.InfoContext(ctx, "Created new allowed user", "user_id", userID)
		return user, nil
	}

	return nil, core.ErrUserNotFound
}

// IsUserAllowed checks if user is in the allowed list
func (r *UserRepository) IsUserAllowed(ctx context.Context, userID int64) bool {
	allowed := r.allowedUserIDs[userID]

	slog.DebugContext(ctx, "Checking user authorization",
		"user_id", userID,
		"allowed", allowed,
	)

	return allowed
}

// loadAllowedUsers loads allowed user IDs from environment
func (r *UserRepository) loadAllowedUsers(allowedUsersStr string) {
	// Parse comma-separated list of user IDs
	for _, idStr := range strings.Split(allowedUsersStr, ",") {
		idStr = strings.TrimSpace(idStr)
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			slog.Warn("Invalid user ID in ALLOWED_USER_IDS", "id", idStr, "error", err)
			continue
		}

		r.allowedUserIDs[id] = true
		slog.Info("Added allowed user", "user_id", id)
	}
}
