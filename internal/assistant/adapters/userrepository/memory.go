package userrepository

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/knightazura/kumote/internal/assistant/core"
)

type UserRepository struct {
	allowedUserIDs map[int64]bool
	users          map[int64]*core.User
	logger         *slog.Logger
}

// NewUserRepository creates a new user repository
func NewUserRepository(logger *slog.Logger) *UserRepository {
	repo := &UserRepository{
		allowedUserIDs: make(map[int64]bool),
		users:          make(map[int64]*core.User),
		logger:         logger,
	}

	// Load allowed user IDs from environment or use default
	repo.loadAllowedUsers()

	return repo
}

// GetUser retrieves user by ID
func (r *UserRepository) GetUser(ctx context.Context, userID int64) (*core.User, error) {
	r.logger.DebugContext(ctx, "Getting user", "user_id", userID)

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
		r.logger.InfoContext(ctx, "Created new allowed user", "user_id", userID)
		return user, nil
	}

	return nil, core.ErrUserNotFound
}

// SaveUser saves or updates user information
func (r *UserRepository) SaveUser(ctx context.Context, user *core.User) error {
	if err := core.ValidateUser(*user); err != nil {
		return err
	}

	r.logger.DebugContext(ctx, "Saving user",
		"user_id", user.ID,
		"username", user.Username,
		"first_name", user.FirstName,
	)

	r.users[user.ID] = user

	// If user is allowed, add to allowed list
	if user.IsAllowed {
		r.allowedUserIDs[user.ID] = true
	}

	r.logger.InfoContext(ctx, "User saved successfully", "user_id", user.ID)
	return nil
}

// IsUserAllowed checks if user is in the allowed list
func (r *UserRepository) IsUserAllowed(ctx context.Context, userID int64) bool {
	allowed := r.allowedUserIDs[userID]

	r.logger.DebugContext(ctx, "Checking user authorization",
		"user_id", userID,
		"allowed", allowed,
	)

	return allowed
}

// GetAllowedUsers returns list of all allowed users
func (r *UserRepository) GetAllowedUsers(ctx context.Context) ([]core.User, error) {
	r.logger.DebugContext(ctx, "Getting all allowed users")

	users := make([]core.User, 0, len(r.allowedUserIDs))

	for userID := range r.allowedUserIDs {
		user, err := r.GetUser(ctx, userID)
		if err != nil {
			r.logger.WarnContext(ctx, "Failed to get allowed user",
				"user_id", userID,
				"error", err.Error(),
			)
			continue
		}
		users = append(users, *user)
	}

	r.logger.InfoContext(ctx, "Retrieved allowed users", "count", len(users))
	return users, nil
}

// loadAllowedUsers loads allowed user IDs from environment variables
func (r *UserRepository) loadAllowedUsers() {
	// Try to load from environment variable first
	if envUserIDs := os.Getenv("ALLOWED_USER_IDS"); envUserIDs != "" {
		r.parseUserIDsFromEnv(envUserIDs)
		return
	}

	// Fallback to hardcoded owner user ID
	// TODO: Replace this with your actual Telegram user ID
	ownerUserID := getOwnerUserID()
	if ownerUserID != 0 {
		r.allowedUserIDs[ownerUserID] = true
		r.logger.InfoContext(context.Background(), "Loaded hardcoded owner user ID",
			"user_id", ownerUserID,
		)
	} else {
		r.logger.WarnContext(context.Background(),
			"No user IDs configured. Set ALLOWED_USER_IDS environment variable or update hardcoded owner ID")
	}
}

// parseUserIDsFromEnv parses user IDs from environment variable
func (r *UserRepository) parseUserIDsFromEnv(envUserIDs string) {
	userIDStrings := strings.Split(envUserIDs, ",")
	loadedCount := 0

	for _, userIDStr := range userIDStrings {
		userIDStr = strings.TrimSpace(userIDStr)
		if userIDStr == "" {
			continue
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			r.logger.WarnContext(context.Background(), "Invalid user ID in environment variable",
				"user_id_string", userIDStr,
				"error", err.Error(),
			)
			continue
		}

		r.allowedUserIDs[userID] = true
		loadedCount++
	}

	r.logger.InfoContext(context.Background(), "Loaded allowed user IDs from environment",
		"count", loadedCount,
	)
}

// getOwnerUserID returns the hardcoded owner user ID
// TODO: Replace this with your actual Telegram user ID
func getOwnerUserID() int64 {
	// You can get your Telegram user ID by:
	// 1. Messaging @userinfobot on Telegram
	// 2. Or temporarily adding logging to see incoming user IDs

	// Example: return 123456789 // Replace with your actual user ID

	// For now, try to get from environment variable OWNER_USER_ID
	if ownerIDStr := os.Getenv("OWNER_USER_ID"); ownerIDStr != "" {
		if ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64); err == nil {
			return ownerID
		}
	}

	// Return 0 if not configured - this will trigger a warning
	return 0
}

// AddAllowedUser adds a user ID to the allowed list (for runtime management)
func (r *UserRepository) AddAllowedUser(ctx context.Context, userID int64) error {
	r.logger.InfoContext(ctx, "Adding user to allowed list", "user_id", userID)

	r.allowedUserIDs[userID] = true

	// If user already exists, update their allowed status
	if user, exists := r.users[userID]; exists {
		user.IsAllowed = true
		r.users[userID] = user
	}

	r.logger.InfoContext(ctx, "User added to allowed list successfully", "user_id", userID)
	return nil
}

// RemoveAllowedUser removes a user ID from the allowed list
func (r *UserRepository) RemoveAllowedUser(ctx context.Context, userID int64) error {
	r.logger.InfoContext(ctx, "Removing user from allowed list", "user_id", userID)

	delete(r.allowedUserIDs, userID)

	// If user exists, update their allowed status
	if user, exists := r.users[userID]; exists {
		user.IsAllowed = false
		r.users[userID] = user
	}

	r.logger.InfoContext(ctx, "User removed from allowed list successfully", "user_id", userID)
	return nil
}

// GetAllowedUserCount returns the number of allowed users
func (r *UserRepository) GetAllowedUserCount() int {
	return len(r.allowedUserIDs)
}
