package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/knightazura/kumote/internal/assistant/core"
	"github.com/knightazura/kumote/internal/assistant/presentation/config"
	"github.com/knightazura/kumote/internal/assistant/presentation/handlers"
	"github.com/knightazura/kumote/internal/assistant/presentation/middleware"
)

// Server represents the HTTP server
type Server struct {
	config           *config.ServerConfig
	logger           *slog.Logger
	assistantService core.AssistantService
	httpServer       *http.Server
	router           *http.ServeMux
}

// NewServer creates a new HTTP server
func NewServer(
	cfg *config.ServerConfig,
	logger *slog.Logger,
	assistantService core.AssistantService,
) *Server {
	return &Server{
		config:           cfg,
		logger:           logger,
		assistantService: assistantService,
		router:           http.NewServeMux(),
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	// Setup routes
	s.setupRoutes()

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         s.config.GetAddress(),
		Handler:      s.buildMiddlewareStack(),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		s.logger.InfoContext(ctx, "Starting HTTP server",
			"address", s.config.GetAddress(),
			"environment", s.config.Environment,
			"version", s.config.Version,
		)

		serverErrors <- s.httpServer.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		s.logger.InfoContext(ctx, "Shutdown signal received",
			"signal", sig.String(),
		)

		// Graceful shutdown
		return s.gracefulShutdown(ctx)

	case <-ctx.Done():
		s.logger.InfoContext(ctx, "Context cancelled, shutting down server")
		return s.gracefulShutdown(ctx)
	}
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop(ctx context.Context) error {
	return s.gracefulShutdown(ctx)
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	healthHandler := handlers.NewHealthHandler(
		s.assistantService,
		s.logger,
		s.config.Version,
	)
	s.router.Handle("/health", healthHandler)

	// Telegram webhook endpoint
	telegramHandler := handlers.NewTelegramWebhookHandler(
		s.assistantService,
		s.logger,
		s.config.TelegramWebhookSecret,
		s.config.AllowedUserIDs,
	)
	s.router.Handle("/telegram", telegramHandler)

	// Root endpoint with basic info
	s.router.HandleFunc("/", s.handleRoot)
}

// buildMiddlewareStack creates the middleware stack
func (s *Server) buildMiddlewareStack() http.Handler {
	var handler http.Handler = s.router

	// Apply middleware in reverse order (last applied is executed first)

	// Security headers
	handler = middleware.Security(handler)

	// CORS (if needed)
	handler = middleware.CORS(handler)

	// Rate limiting
	handler = middleware.RateLimit(s.logger, s.config.RateLimitPerMinute)(handler)

	// Request timeout
	handler = middleware.Timeout(s.config.WriteTimeout)(handler)

	// Logging
	handler = middleware.Logging(s.logger)(handler)

	// Recovery (should be first to catch all panics)
	handler = middleware.Recovery(s.logger)(handler)

	// Request ID (should be very first to add ID to all requests)
	handler = middleware.RequestID(handler)

	return handler
}

// handleRoot handles requests to the root endpoint
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	info := map[string]interface{}{
		"service":     "Remote Work Telegram Assistant",
		"version":     s.config.Version,
		"environment": s.config.Environment,
		"status":      "running",
		"timestamp":   time.Now().Format(time.RFC3339),
		"endpoints": map[string]string{
			"health": "/health",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Use proper JSON encoding with the encoding/json package
	if err := json.NewEncoder(w).Encode(info); err != nil {
		s.logger.ErrorContext(r.Context(), "Error encoding JSON response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// gracefulShutdown performs graceful shutdown of the server
func (s *Server) gracefulShutdown(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Starting graceful shutdown")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.ErrorContext(ctx, "Error during server shutdown", "error", err)

		// Force close if graceful shutdown failed
		if closeErr := s.httpServer.Close(); closeErr != nil {
			s.logger.ErrorContext(ctx, "Error during server force close", "error", closeErr)
		}

		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.InfoContext(ctx, "Server shutdown completed successfully")
	return nil
}

// GetAddress returns the server address
func (s *Server) GetAddress() string {
	if s.httpServer != nil {
		return s.httpServer.Addr
	}
	return s.config.GetAddress()
}
