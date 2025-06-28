package ratelimiter

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	buckets map[int64]*tokenBucket
	mutex   sync.RWMutex
	logger  *slog.Logger
	config  RateLimitConfig
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int           // Number of requests allowed per minute
	BurstSize         int           // Maximum burst requests allowed
	WindowDuration    time.Duration // Time window for rate limiting
	CleanupInterval   time.Duration // How often to clean up expired buckets
}

// tokenBucket represents a token bucket for a specific user
type tokenBucket struct {
	tokens       int       // Current number of tokens
	maxTokens    int       // Maximum tokens (burst size)
	refillRate   int       // Tokens added per minute
	lastRefill   time.Time // Last time tokens were refilled
	lastAccess   time.Time // Last time bucket was accessed
	requestCount int       // Total requests made
	blockedCount int       // Total blocked requests
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int, logger *slog.Logger) *RateLimiter {
	config := RateLimitConfig{
		RequestsPerMinute: requestsPerMinute,
		BurstSize:         requestsPerMinute / 2, // Allow burst of half the rate
		WindowDuration:    time.Minute,
		CleanupInterval:   5 * time.Minute,
	}

	// Ensure minimum burst size
	if config.BurstSize < 1 {
		config.BurstSize = 1
	}

	rl := &RateLimiter{
		buckets: make(map[int64]*tokenBucket),
		logger:  logger,
		config:  config,
	}

	// Start cleanup goroutine
	go rl.cleanupRoutine()

	logger.InfoContext(context.Background(), "Rate limiter initialized",
		"requests_per_minute", requestsPerMinute,
		"burst_size", config.BurstSize,
	)

	return rl
}

// IsAllowed checks if request is within rate limit
func (rl *RateLimiter) IsAllowed(ctx context.Context, userID int64) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	bucket := rl.getBucket(userID)
	rl.refillBucket(bucket)

	allowed := bucket.tokens > 0

	rl.logger.DebugContext(ctx, "Rate limit check",
		"user_id", userID,
		"tokens_available", bucket.tokens,
		"allowed", allowed,
	)

	if !allowed {
		bucket.blockedCount++
		rl.logger.WarnContext(ctx, "Rate limit exceeded",
			"user_id", userID,
			"blocked_count", bucket.blockedCount,
		)
	}

	return allowed
}

// RecordRequest records a request for rate limiting
func (rl *RateLimiter) RecordRequest(ctx context.Context, userID int64) error {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	bucket := rl.getBucket(userID)
	rl.refillBucket(bucket)

	bucket.lastAccess = time.Now()

	if bucket.tokens <= 0 {
		bucket.blockedCount++
		rl.logger.WarnContext(ctx, "Request blocked by rate limiter",
			"user_id", userID,
			"blocked_count", bucket.blockedCount,
		)
		return core.ErrRateLimitExceeded
	}

	// Consume token and count successful request
	bucket.tokens--
	bucket.requestCount++

	rl.logger.DebugContext(ctx, "Request recorded",
		"user_id", userID,
		"tokens_remaining", bucket.tokens,
		"total_requests", bucket.requestCount,
	)

	return nil
}

// GetRemainingRequests returns remaining requests for user
func (rl *RateLimiter) GetRemainingRequests(ctx context.Context, userID int64) (int, error) {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	bucket := rl.getBucket(userID)
	rl.refillBucket(bucket)

	remaining := bucket.tokens

	rl.logger.DebugContext(ctx, "Getting remaining requests",
		"user_id", userID,
		"remaining", remaining,
	)

	return remaining, nil
}

// GetUserStats returns statistics for a user
func (rl *RateLimiter) GetUserStats(ctx context.Context, userID int64) map[string]interface{} {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	bucket := rl.getBucket(userID)

	stats := map[string]interface{}{
		"user_id":             userID,
		"tokens_available":    bucket.tokens,
		"max_tokens":          bucket.maxTokens,
		"total_requests":      bucket.requestCount,
		"blocked_requests":    bucket.blockedCount,
		"last_access":         bucket.lastAccess,
		"requests_per_minute": rl.config.RequestsPerMinute,
	}

	rl.logger.DebugContext(ctx, "Retrieved user rate limit stats",
		"user_id", userID,
		"stats", stats,
	)

	return stats
}

// GetGlobalStats returns global rate limiting statistics
func (rl *RateLimiter) GetGlobalStats(ctx context.Context) map[string]interface{} {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	totalUsers := len(rl.buckets)
	totalRequests := 0
	totalBlocked := 0
	activeUsers := 0

	cutoff := time.Now().Add(-time.Hour) // Users active in last hour

	for _, bucket := range rl.buckets {
		totalRequests += bucket.requestCount
		totalBlocked += bucket.blockedCount
		if bucket.lastAccess.After(cutoff) {
			activeUsers++
		}
	}

	stats := map[string]interface{}{
		"total_users":         totalUsers,
		"active_users":        activeUsers,
		"total_requests":      totalRequests,
		"total_blocked":       totalBlocked,
		"requests_per_minute": rl.config.RequestsPerMinute,
		"burst_size":          rl.config.BurstSize,
	}

	if totalRequests > 0 {
		stats["block_rate"] = float64(totalBlocked) / float64(totalRequests)
	} else {
		stats["block_rate"] = 0.0
	}

	rl.logger.DebugContext(ctx, "Retrieved global rate limit stats", "stats", stats)

	return stats
}

// ResetUserLimit resets rate limit for a specific user
func (rl *RateLimiter) ResetUserLimit(ctx context.Context, userID int64) error {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	bucket := rl.getBucket(userID)
	bucket.tokens = bucket.maxTokens
	bucket.lastRefill = time.Now()
	bucket.blockedCount = 0

	rl.logger.InfoContext(ctx, "Reset rate limit for user",
		"user_id", userID,
		"tokens_restored", bucket.maxTokens,
	)

	return nil
}

// UpdateConfig updates rate limiting configuration
func (rl *RateLimiter) UpdateConfig(ctx context.Context, requestsPerMinute int) error {
	if requestsPerMinute <= 0 {
		return core.NewValidationError("requests_per_minute", "must be positive")
	}

	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	oldRate := rl.config.RequestsPerMinute
	rl.config.RequestsPerMinute = requestsPerMinute
	rl.config.BurstSize = requestsPerMinute / 2

	if rl.config.BurstSize < 1 {
		rl.config.BurstSize = 1
	}

	// Update existing buckets
	for _, bucket := range rl.buckets {
		bucket.refillRate = requestsPerMinute
		bucket.maxTokens = rl.config.BurstSize

		// Adjust current tokens proportionally
		if oldRate > 0 {
			ratio := float64(requestsPerMinute) / float64(oldRate)
			bucket.tokens = int(float64(bucket.tokens) * ratio)
		}

		if bucket.tokens > bucket.maxTokens {
			bucket.tokens = bucket.maxTokens
		}
	}

	rl.logger.InfoContext(ctx, "Updated rate limit configuration",
		"old_rate", oldRate,
		"new_rate", requestsPerMinute,
		"new_burst_size", rl.config.BurstSize,
	)

	return nil
}

// CleanupExpiredBuckets removes buckets for inactive users
func (rl *RateLimiter) CleanupExpiredBuckets(ctx context.Context, maxAge time.Duration) int {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for userID, bucket := range rl.buckets {
		if bucket.lastAccess.Before(cutoff) {
			delete(rl.buckets, userID)
			removed++
		}
	}

	if removed > 0 {
		rl.logger.InfoContext(ctx, "Cleaned up expired rate limit buckets",
			"removed_count", removed,
			"remaining_count", len(rl.buckets),
			"max_age", maxAge.String(),
		)
	}

	return removed
}

// getBucket gets or creates a token bucket for a user (must be called with lock held)
func (rl *RateLimiter) getBucket(userID int64) *tokenBucket {
	bucket, exists := rl.buckets[userID]
	if !exists {
		bucket = &tokenBucket{
			tokens:     rl.config.BurstSize,
			maxTokens:  rl.config.BurstSize,
			refillRate: rl.config.RequestsPerMinute,
			lastRefill: time.Now(),
			lastAccess: time.Now(),
		}
		rl.buckets[userID] = bucket

		rl.logger.DebugContext(context.Background(), "Created new rate limit bucket",
			"user_id", userID,
			"initial_tokens", bucket.tokens,
		)
	}

	return bucket
}

// refillBucket refills tokens in the bucket based on time elapsed (must be called with lock held)
func (rl *RateLimiter) refillBucket(bucket *tokenBucket) {
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)

	// Allow refill for any time elapsed, not just full minutes
	if elapsed <= 0 {
		return // No time has passed
	}

	// Calculate tokens to add based on elapsed time
	minutesElapsed := elapsed.Minutes()
	tokensToAdd := int(minutesElapsed * float64(bucket.refillRate))

	if tokensToAdd > 0 {
		bucket.tokens += tokensToAdd
		if bucket.tokens > bucket.maxTokens {
			bucket.tokens = bucket.maxTokens
		}
		bucket.lastRefill = now
	}
}

// cleanupRoutine periodically cleans up expired buckets
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		removed := rl.CleanupExpiredBuckets(ctx, 24*time.Hour) // Remove buckets older than 24 hours

		if removed > 0 {
			rl.logger.DebugContext(ctx, "Cleanup routine completed",
				"removed_buckets", removed,
			)
		}
	}
}

// Close stops the rate limiter and cleans up resources
func (rl *RateLimiter) Close() error {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// Clear all buckets
	rl.buckets = make(map[int64]*tokenBucket)

	rl.logger.InfoContext(context.Background(), "Rate limiter closed")
	return nil
}
