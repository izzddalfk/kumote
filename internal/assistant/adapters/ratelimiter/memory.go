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

// getBucket gets or creates a token bucket for a user
func (rl *RateLimiter) getBucket(userID int64) *tokenBucket {
	bucket, exists := rl.buckets[userID]
	if !exists {
		now := time.Now()
		bucket = &tokenBucket{
			tokens:     rl.config.BurstSize,
			maxTokens:  rl.config.BurstSize,
			refillRate: rl.config.RequestsPerMinute,
			lastRefill: now,
			lastAccess: now,
		}
		rl.buckets[userID] = bucket
	}
	return bucket
}

// refillBucket refills tokens in the bucket based on elapsed time
func (rl *RateLimiter) refillBucket(bucket *tokenBucket) {
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)

	// Calculate how many tokens to add based on elapsed time
	tokensToAdd := int(float64(bucket.refillRate) * elapsed.Minutes())

	if tokensToAdd > 0 {
		bucket.tokens = min(bucket.tokens+tokensToAdd, bucket.maxTokens)
		bucket.lastRefill = now
	}
}

// cleanupRoutine periodically cleans up unused buckets
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes expired buckets
func (rl *RateLimiter) cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour) // Remove buckets unused for 24 hours
	for userID, bucket := range rl.buckets {
		if bucket.lastAccess.Before(cutoff) {
			delete(rl.buckets, userID)
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
