package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// RequestID adds request ID to context
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()
		ctx := context.WithValue(r.Context(), "request_id", requestID)

		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logging logs HTTP requests
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Log request
			duration := time.Since(start)
			requestID := getRequestID(r.Context())

			logger.InfoContext(r.Context(), "HTTP request processed",
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", wrapped.statusCode,
				"duration", duration,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"request_id", requestID,
				"content_length", r.ContentLength,
			)
		})
	}
}

// Recovery recovers from panics
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic
					stack := debug.Stack()
					requestID := getRequestID(r.Context())

					logger.ErrorContext(r.Context(), "Panic recovered",
						"error", err,
						"stack", string(stack),
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", requestID,
					)

					// Send error response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, `{"error":"Internal server error","request_id":"%s","timestamp":"%s"}`,
						requestID, time.Now().Format(time.RFC3339))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// CORS adds CORS headers
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Security adds security headers
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// RateLimit applies basic rate limiting (simplified version)
func RateLimit(logger *slog.Logger, requestsPerMinute int) func(http.Handler) http.Handler {
	// This is a simplified rate limiter
	// In production, you'd want to use a more sophisticated implementation
	limiter := make(map[string][]time.Time)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)
			now := time.Now()

			// Clean old entries
			if requests, exists := limiter[clientIP]; exists {
				var validRequests []time.Time
				cutoff := now.Add(-time.Minute)

				for _, reqTime := range requests {
					if reqTime.After(cutoff) {
						validRequests = append(validRequests, reqTime)
					}
				}
				limiter[clientIP] = validRequests
			}

			// Check rate limit
			if len(limiter[clientIP]) >= requestsPerMinute {
				logger.WarnContext(r.Context(), "Rate limit exceeded",
					"client_ip", clientIP,
					"requests_per_minute", len(limiter[clientIP]),
					"limit", requestsPerMinute,
				)

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":"Rate limit exceeded","retry_after_seconds":60,"timestamp":"%s"}`,
					time.Now().Format(time.RFC3339))
				return
			}

			// Add current request
			limiter[clientIP] = append(limiter[clientIP], now)

			next.ServeHTTP(w, r)
		})
	}
}

// Timeout applies timeout to requests
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, "Request timeout")
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// getRequestID extracts request ID from context
func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value("request_id").(string); ok {
		return id
	}
	return "unknown"
}

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP if multiple are present
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Check CF-Connecting-IP header (Cloudflare)
	if cfip := r.Header.Get("CF-Connecting-IP"); cfip != "" {
		return strings.TrimSpace(cfip)
	}

	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}
