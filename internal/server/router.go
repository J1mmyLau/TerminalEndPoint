package server

import (
	"net/http"

	"github.com/J1mmyLau/TerminalEndPoint/internal/handler"
)

// NewRouter creates the HTTP router with all registered routes.
func NewRouter(s *Server) http.Handler {
	mux := http.NewServeMux()

	h := handler.New(s.manager, s.cfg)

	mux.HandleFunc("GET /health", h.Health)

	mux.HandleFunc("POST /api/v1/sessions", h.CreateSession)
	mux.HandleFunc("GET /api/v1/sessions", h.ListSessions)
	mux.HandleFunc("GET /api/v1/sessions/{id}", h.GetSession)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", h.KillSession)

	mux.HandleFunc("POST /api/v1/sessions/{id}/exec", h.Exec)
	mux.HandleFunc("POST /api/v1/sessions/{id}/write", h.Write)
	mux.HandleFunc("GET /api/v1/sessions/{id}/output", h.GetOutput)
	mux.HandleFunc("POST /api/v1/sessions/{id}/resize", h.Resize)
	mux.HandleFunc("POST /api/v1/sessions/{id}/signal", h.Signal)

	mux.HandleFunc("GET /ws/sessions/{id}", h.WebSocket)

	return mux
}
