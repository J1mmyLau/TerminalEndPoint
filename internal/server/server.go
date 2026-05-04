package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/J1mmyLau/TerminalEndPoint/internal/config"
	"github.com/J1mmyLau/TerminalEndPoint/internal/session"
)

// Server is the HTTP server for the terminal endpoint.
type Server struct {
	http    *http.Server
	manager *session.Manager
	cfg     *config.Config
}

// New creates a new Server instance.
func New(manager *session.Manager, cfg *config.Config) *Server {
	s := &Server{
		manager: manager,
		cfg:     cfg,
	}

	mux := NewRouter(s)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	s.http = &http.Server{
		Addr:         addr,
		Handler:      withMiddleware(mux, cfg),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start begins listening for HTTP connections. Blocks until the server stops.
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// Addr returns the server's listening address.
func (s *Server) Addr() string {
	return s.http.Addr
}
