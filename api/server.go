package api

import (
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
}

// Engine returns the underlying Engine for testing or composition.
func (s *Server) Engine() *Engine {
	return s.engine
}

// Run starts the server on the given address.
func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}
