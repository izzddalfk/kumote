package ratelimiter_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/knightazura/kumote/internal/assistant/adapters/ratelimiter"
	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in tests
	}))
}

func TestRateLimiter_NewRateLimiter(t *testing.T) {
	logger := getTestLogger()

	rl := ratelimiter.NewRateLimiter(10, logger)
	require.NotNil(t, rl)

	ctx := context.Background()
	userID := int64(123456789)

	// Should allow initial requests
	assert.True(t, rl.IsAllowed(ctx, userID))

	// Cleanup
	rl.Close()
}

func TestRateLimiter_IsAllowed_WithinLimit(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger) // 10 requests per minute
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Should allow up to burst size (5 for rate of 10)
	for i := 0; i < 5; i++ {
		allowed := rl.IsAllowed(ctx, userID)
		assert.True(t, allowed, "Request %d should be allowed", i+1)

		// Consume a token if allowed
		if allowed {
			err := rl.RecordRequest(ctx, userID)
			require.NoError(t, err)
		}
	}

	// 6th request should be blocked (exceeds burst)
	assert.False(t, rl.IsAllowed(ctx, userID), "Request 6 should be blocked")
}

func TestRateLimiter_RecordRequest_Success(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Record successful requests
	for i := 0; i < 5; i++ {
		err := rl.RecordRequest(ctx, userID)
		assert.NoError(t, err, "Request %d should be recorded successfully", i+1)
	}

	// Check remaining requests
	remaining, err := rl.GetRemainingRequests(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining) // Should have consumed all burst tokens
}

func TestRateLimiter_RecordRequest_ExceedsLimit(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Use up all tokens
	for i := 0; i < 5; i++ {
		err := rl.RecordRequest(ctx, userID)
		require.NoError(t, err)
	}

	// Next request should be rate limited
	err := rl.RecordRequest(ctx, userID)
	assert.Error(t, err)
	assert.Equal(t, core.ErrRateLimitExceeded, err)
}

func TestRateLimiter_GetRemainingRequests(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Initially should have burst size tokens
	remaining, err := rl.GetRemainingRequests(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 5, remaining) // Burst size for rate 10

	// Record a request
	err = rl.RecordRequest(ctx, userID)
	require.NoError(t, err)

	// Should have one less token
	remaining, err = rl.GetRemainingRequests(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 4, remaining)
}

func TestRateLimiter_MultipleUsers(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	user1 := int64(123456789)
	user2 := int64(987654321)

	// Each user should have independent rate limits
	for i := 0; i < 5; i++ {
		err1 := rl.RecordRequest(ctx, user1)
		err2 := rl.RecordRequest(ctx, user2)
		assert.NoError(t, err1, "User1 request %d should succeed", i+1)
		assert.NoError(t, err2, "User2 request %d should succeed", i+1)
	}

	// Both users should be at their limit
	err1 := rl.RecordRequest(ctx, user1)
	err2 := rl.RecordRequest(ctx, user2)
	assert.Equal(t, core.ErrRateLimitExceeded, err1)
	assert.Equal(t, core.ErrRateLimitExceeded, err2)
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(60, logger) // 60 per minute = 1 per second
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Use all initial tokens
	for i := 0; i < 30; i++ { // Burst size for rate 60
		err := rl.RecordRequest(ctx, userID)
		require.NoError(t, err)
	}

	// Should be rate limited now
	err := rl.RecordRequest(ctx, userID)
	assert.Equal(t, core.ErrRateLimitExceeded, err)

	// Wait for tokens to refill (simulate 1+ minute passing)
	time.Sleep(61 * time.Second)

	// Should be able to make requests again
	err = rl.RecordRequest(ctx, userID)
	assert.NoError(t, err, "Should be able to make request after refill")
}

func TestRateLimiter_GetUserStats(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Make some requests
	for i := 0; i < 3; i++ {
		err := rl.RecordRequest(ctx, userID)
		require.NoError(t, err)
	}

	// Try to exceed limit
	for i := 0; i < 3; i++ {
		rl.RecordRequest(ctx, userID) // Will error on last ones
	}

	stats := rl.GetUserStats(ctx, userID)
	require.NotNil(t, stats)

	assert.Equal(t, userID, stats["user_id"])
	assert.Equal(t, 5, stats["total_requests"])
	assert.Equal(t, 1, stats["blocked_requests"]) // 1 blocked request
	assert.Equal(t, 10, stats["requests_per_minute"])
	assert.Contains(t, stats, "tokens_available")
	assert.Contains(t, stats, "last_access")
}

func TestRateLimiter_GetGlobalStats(t *testing.T) {
	logger := getTestLogger()
	// Use a rate limiter with a very low limit to ensure we hit the limit
	rl := ratelimiter.NewRateLimiter(4, logger) // 4 requests per minute, burst size of 2
	defer rl.Close()

	ctx := context.Background()
	user1 := int64(123456789)
	user2 := int64(987654321)

	// Make enough requests to use all tokens
	for i := 0; i < 2; i++ { // Burst size is 2
		err := rl.RecordRequest(ctx, user1)
		require.NoError(t, err)
		err = rl.RecordRequest(ctx, user2)
		require.NoError(t, err)
	}

	// Both users should now be at their limit
	// Try one more request for user1, which should be blocked
	err := rl.RecordRequest(ctx, user1)
	assert.Equal(t, core.ErrRateLimitExceeded, err, "Request should be blocked")

	// Check global stats
	stats := rl.GetGlobalStats(ctx)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats["total_users"])
	assert.Equal(t, 2, stats["active_users"])
	assert.Equal(t, 4, stats["total_requests"]) // 4 successful requests
	assert.Equal(t, 1, stats["total_blocked"], "Should have 1 blocked request")
	assert.Equal(t, 4, stats["requests_per_minute"])
	assert.Contains(t, stats, "block_rate")
}

func TestRateLimiter_ResetUserLimit(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Use all tokens
	for i := 0; i < 5; i++ {
		err := rl.RecordRequest(ctx, userID)
		require.NoError(t, err)
	}

	// Should be rate limited
	err := rl.RecordRequest(ctx, userID)
	assert.Equal(t, core.ErrRateLimitExceeded, err)

	// Reset user limit
	err = rl.ResetUserLimit(ctx, userID)
	require.NoError(t, err)

	// Should be able to make requests again
	err = rl.RecordRequest(ctx, userID)
	assert.NoError(t, err, "Should be able to make request after reset")

	// Check that blocked count is reset
	stats := rl.GetUserStats(ctx, userID)
	assert.Equal(t, 0, stats["blocked_requests"])
}

func TestRateLimiter_UpdateConfig(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Make some requests at current rate
	for i := 0; i < 3; i++ {
		err := rl.RecordRequest(ctx, userID)
		require.NoError(t, err)
	}

	// Update to higher rate
	err := rl.UpdateConfig(ctx, 20)
	require.NoError(t, err)

	// Should be able to make more requests now
	remaining, err := rl.GetRemainingRequests(ctx, userID)
	require.NoError(t, err)
	assert.Greater(t, remaining, 0, "Should have tokens after rate increase")

	// Verify stats reflect new config
	stats := rl.GetUserStats(ctx, userID)
	assert.Equal(t, 20, stats["requests_per_minute"])
}

func TestRateLimiter_UpdateConfig_Invalid(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()

	// Try to set invalid rate
	err := rl.UpdateConfig(ctx, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")

	err = rl.UpdateConfig(ctx, -5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestRateLimiter_CleanupExpiredBuckets(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	user1 := int64(123456789)
	user2 := int64(987654321)

	// Make requests from both users
	rl.RecordRequest(ctx, user1)
	rl.RecordRequest(ctx, user2)

	// Verify both users exist
	stats := rl.GetGlobalStats(ctx)
	assert.Equal(t, 2, stats["total_users"])

	// Clean up buckets older than 0 seconds (should remove all)
	removed := rl.CleanupExpiredBuckets(ctx, 0)
	assert.Equal(t, 2, removed)

	// Verify users are cleaned up
	stats = rl.GetGlobalStats(ctx)
	assert.Equal(t, 0, stats["total_users"])
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(100, logger) // High rate to reduce blocking
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	const numGoroutines = 10
	const requestsPerGoroutine = 5

	// Channel to collect results
	results := make(chan error, numGoroutines*requestsPerGoroutine)

	// Start concurrent requests
	for g := 0; g < numGoroutines; g++ {
		go func() {
			for i := 0; i < requestsPerGoroutine; i++ {
				err := rl.RecordRequest(ctx, userID)
				results <- err
			}
		}()
	}

	// Collect results
	successCount := 0
	blockedCount := 0

	for i := 0; i < numGoroutines*requestsPerGoroutine; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else if err == core.ErrRateLimitExceeded {
			blockedCount++
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	// Should have some successful and possibly some blocked requests
	assert.Greater(t, successCount, 0, "Should have some successful requests")
	total := successCount + blockedCount
	assert.Equal(t, numGoroutines*requestsPerGoroutine, total, "All requests should be accounted for")

	t.Logf("Concurrent test results: %d successful, %d blocked", successCount, blockedCount)
}

func TestRateLimiter_EdgeCases(t *testing.T) {
	logger := getTestLogger()

	// Test very low rate
	rl1 := ratelimiter.NewRateLimiter(1, logger)
	defer rl1.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Should allow at least 1 request
	err := rl1.RecordRequest(ctx, userID)
	assert.NoError(t, err)

	// Second request should be blocked
	err = rl1.RecordRequest(ctx, userID)
	assert.Equal(t, core.ErrRateLimitExceeded, err)

	// Test very high rate
	rl2 := ratelimiter.NewRateLimiter(1000, logger)
	defer rl2.Close()

	// Should allow many requests
	for i := 0; i < 500; i++ {
		err := rl2.RecordRequest(ctx, userID)
		assert.NoError(t, err, "Request %d should succeed with high rate", i+1)
	}
}

func TestRateLimiter_ZeroUserID(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(0) // Edge case: zero user ID

	// Should work normally even with zero user ID
	err := rl.RecordRequest(ctx, userID)
	assert.NoError(t, err)

	remaining, err := rl.GetRemainingRequests(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 4, remaining) // Should have consumed 1 token
}

func TestRateLimiter_NegativeUserID(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(-123) // Edge case: negative user ID

	// Should work normally even with negative user ID
	err := rl.RecordRequest(ctx, userID)
	assert.NoError(t, err)

	remaining, err := rl.GetRemainingRequests(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 4, remaining) // Should have consumed 1 token
}

func TestRateLimiter_StatsAccuracy(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(6, logger) // Rate of 6, burst of 3
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Make exactly 3 successful requests (burst size)
	for i := 0; i < 3; i++ {
		err := rl.RecordRequest(ctx, userID)
		require.NoError(t, err)
	}

	// Try 2 more requests (should be blocked)
	for i := 0; i < 2; i++ {
		err := rl.RecordRequest(ctx, userID)
		assert.Equal(t, core.ErrRateLimitExceeded, err)
	}

	// Check stats
	stats := rl.GetUserStats(ctx, userID)
	assert.Equal(t, 3, stats["total_requests"]) // Only successful requests counted
	assert.Equal(t, 2, stats["blocked_requests"])
	assert.Equal(t, 0, stats["tokens_available"])

	globalStats := rl.GetGlobalStats(ctx)
	assert.Equal(t, 3, globalStats["total_requests"])
	assert.Equal(t, 2, globalStats["total_blocked"])
	assert.Equal(t, 1, globalStats["total_users"])
	assert.Equal(t, 2.0/3.0, globalStats["block_rate"]) // 2 blocked out of 3 total
}

func TestRateLimiter_BurstSizeCalculation(t *testing.T) {
	logger := getTestLogger()

	testCases := []struct {
		rate          int
		expectedBurst int
	}{
		{rate: 10, expectedBurst: 5},   // 10/2 = 5
		{rate: 6, expectedBurst: 3},    // 6/2 = 3
		{rate: 1, expectedBurst: 1},    // 1/2 = 0, but minimum is 1
		{rate: 3, expectedBurst: 1},    // 3/2 = 1
		{rate: 100, expectedBurst: 50}, // 100/2 = 50
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("rate_%d", tc.rate), func(t *testing.T) {
			rl := ratelimiter.NewRateLimiter(tc.rate, logger)
			defer rl.Close()

			ctx := context.Background()
			userID := int64(123456789)

			// Should be able to make exactly burst size requests
			for i := 0; i < tc.expectedBurst; i++ {
				err := rl.RecordRequest(ctx, userID)
				assert.NoError(t, err, "Request %d should succeed for rate %d", i+1, tc.rate)
			}

			// Next request should be blocked
			err := rl.RecordRequest(ctx, userID)
			assert.Equal(t, core.ErrRateLimitExceeded, err, "Request after burst should be blocked for rate %d", tc.rate)

			// Verify stats
			stats := rl.GetUserStats(ctx, userID)
			assert.Equal(t, tc.expectedBurst, stats["max_tokens"])
		})
	}
}

func TestRateLimiter_RefillBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-sensitive test in short mode")
	}

	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(120, logger) // 120 per minute = 2 per second
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Use all initial tokens (burst size = 60)
	for i := 0; i < 60; i++ {
		err := rl.RecordRequest(ctx, userID)
		require.NoError(t, err)
	}

	// Should be rate limited now
	err := rl.RecordRequest(ctx, userID)
	assert.Equal(t, core.ErrRateLimitExceeded, err)

	// Wait for some refill (simulate 30 seconds = 0.5 minutes)
	time.Sleep(31 * time.Second)

	// Should have some tokens refilled (0.5 minutes * 120 rate = 60 tokens)
	// But capped at max burst size (60)
	remaining, err := rl.GetRemainingRequests(ctx, userID)
	require.NoError(t, err)
	assert.Greater(t, remaining, 0, "Should have some tokens after refill")
	assert.LessOrEqual(t, remaining, 60, "Should not exceed burst size")
}

func TestRateLimiter_MemoryLeakPrevention(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()

	// Create many buckets for different users
	const numUsers = 1000
	for i := 0; i < numUsers; i++ {
		userID := int64(1000000 + i)
		err := rl.RecordRequest(ctx, userID)
		assert.NoError(t, err)
	}

	// Verify all users were created
	stats := rl.GetGlobalStats(ctx)
	assert.Equal(t, numUsers, stats["total_users"])

	// Cleanup expired buckets (simulate all are old)
	removed := rl.CleanupExpiredBuckets(ctx, 0)
	assert.Equal(t, numUsers, removed)

	// Verify cleanup worked
	stats = rl.GetGlobalStats(ctx)
	assert.Equal(t, 0, stats["total_users"])
}

func TestRateLimiter_ConsistentBehavior(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)
	defer rl.Close()

	ctx := context.Background()
	userID := int64(123456789)

	// Test IsAllowed vs RecordRequest consistency
	for i := 0; i < 5; i++ {
		allowed := rl.IsAllowed(ctx, userID)
		if allowed {
			err := rl.RecordRequest(ctx, userID)
			assert.NoError(t, err, "If IsAllowed returns true, RecordRequest should succeed")
		} else {
			err := rl.RecordRequest(ctx, userID)
			assert.Equal(t, core.ErrRateLimitExceeded, err, "If IsAllowed returns false, RecordRequest should fail")
		}
	}
}

func TestRateLimiter_Close(t *testing.T) {
	logger := getTestLogger()
	rl := ratelimiter.NewRateLimiter(10, logger)

	ctx := context.Background()
	userID := int64(123456789)

	// Make some requests
	err := rl.RecordRequest(ctx, userID)
	assert.NoError(t, err)

	// Close the rate limiter
	err = rl.Close()
	assert.NoError(t, err)

	// Verify buckets are cleared
	stats := rl.GetGlobalStats(ctx)
	assert.Equal(t, 0, stats["total_users"])
}
