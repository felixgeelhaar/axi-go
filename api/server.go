package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
)

// Server is the HTTP API server. It wires use cases and repositories
// into a Gin-like router with no external dependencies.
type Server struct {
	engine         *Engine
	executeAction  *application.ExecuteActionUseCase
	registerPlugin *application.RegisterPluginContributionUseCase
	actionRepo     domain.ActionRepository
	capabilityRepo domain.CapabilityRepository
	sessionRepo    domain.SessionRepository
}

// NewServer creates a Server and registers all routes.
func NewServer(
	executeAction *application.ExecuteActionUseCase,
	registerPlugin *application.RegisterPluginContributionUseCase,
	actionRepo domain.ActionRepository,
	capabilityRepo domain.CapabilityRepository,
	sessionRepo domain.SessionRepository,
) *Server {
	s := &Server{
		engine:         New(),
		executeAction:  executeAction,
		registerPlugin: registerPlugin,
		actionRepo:     actionRepo,
		capabilityRepo: capabilityRepo,
		sessionRepo:    sessionRepo,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.engine.GET("/health", s.healthCheck)

	v1 := s.engine.Group("/api/v1")
	v1.GET("/actions", s.listActions)
	v1.GET("/actions/{name}", s.getAction)
	v1.POST("/actions/execute", s.handleExecuteAction)
	v1.GET("/sessions/{id}", s.getSession)
	v1.POST("/sessions/{id}/approve", s.approveSession)
	v1.POST("/sessions/{id}/reject", s.rejectSession)
	v1.GET("/capabilities", s.listCapabilities)
	v1.POST("/plugins", s.handleRegisterPlugin)
	v1.DELETE("/plugins/{id}", s.handleDeregisterPlugin)
}

// Engine returns the underlying Engine for testing or composition.
func (s *Server) Engine() *Engine {
	return s.engine
}

// Run starts the server on the given address with graceful shutdown.
// It listens for SIGINT/SIGTERM and drains connections before exiting.
func (s *Server) Run(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.engine.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		// Graceful shutdown with 10s timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

// RunWithContext starts the server and shuts down when the context is cancelled.
// Useful for testing and programmatic control.
func (s *Server) RunWithContext(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.engine.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
