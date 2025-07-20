package rest

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/izzddalfk/kumote/internal/assistant/core"
	"github.com/izzddalfk/kumote/internal/assistant/presentation/rest/handlers"
	"gopkg.in/validator.v2"
)

type Server struct {
	assistantService core.AssistantService
	port             string
	readTimeout      time.Duration
	writeTimeout     time.Duration

	router *gin.Engine
}

type ServerConfig struct {
	AssistantService core.AssistantService `validate:"nonnil"`
	Port             string                `validate:"nonzero"`
	ReadTimeout      time.Duration         `validate:"nonzero"`
	WriteTimeout     time.Duration         `validate:"nonzero"`
}

func NewServer(config ServerConfig) (*Server, error) {
	if err := validator.Validate(config); err != nil {
		return nil, err
	}

	return &Server{
		assistantService: config.AssistantService,
		port:             config.Port,
		readTimeout:      config.ReadTimeout,
		writeTimeout:     config.WriteTimeout,
		router:           gin.Default(),
	}, nil
}

func (s *Server) Start() error {
	// setup the router
	s.setup()

	// start server with graceful shutdown using `server.Close` method
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	server := &http.Server{
		Addr:         s.port,
		Handler:      s.router,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}

	go func() {
		<-quit
		slog.Warn("receive interrupt signal")
		if err := server.Close(); err != nil {
			log.Fatalf("error closing server: %v", err)
		}
	}()

	err := server.ListenAndServe()
	if err != nil && err == http.ErrServerClosed {
		slog.Warn("server closed under request")
		return nil
	} else {
		return fmt.Errorf("failed to start server: %w", err)
	}
}

func (s *Server) setup() {
	s.router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, handlers.NewSuccessResponse("It's running!"))
	})

	// Telegram webhook handler
	s.router.POST("/telegram", func(ctx *gin.Context) {
		var incomingUpdate handlers.TelegramUpdate
		err := ctx.ShouldBindJSON(&incomingUpdate)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, handlers.NewErrorResponse(err.Error()))
			return
		}

		// TODO: Test whether we need verify Telegram webhook signature?

		// Check if the request is text message
		if incomingUpdate.Message.Text == "" {
			ctx.JSON(http.StatusOK, handlers.NewSuccessResponse("Message not supported"))
			return
		}

		// Process the text message
		result, err := s.assistantService.ProcessCommand(ctx, core.Command{
			ID:        fmt.Sprintf("%d", incomingUpdate.Message.MessageID),
			UserID:    incomingUpdate.Message.From.ID,
			Text:      strings.TrimSpace(incomingUpdate.Message.Text),
			Timestamp: time.Now(),
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, handlers.NewErrorResponse(err.Error()))
			return
		}

		webhookMessage := "Webhook processed successfully"
		if result == nil || !result.Success {
			webhookMessage = "Webhook failed"
			if result.Error != "" {
				webhookMessage += ": " + result.Error
			}
		}

		ctx.JSON(http.StatusOK, handlers.NewSuccessResponse(webhookMessage))
	})
}
