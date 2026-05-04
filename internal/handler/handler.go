package handler

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/J1mmyLau/TerminalEndPoint/internal/config"
	"github.com/J1mmyLau/TerminalEndPoint/internal/session"
)

type Handler struct {
	manager *session.Manager
	cfg     *config.Config
}

func New(manager *session.Manager, cfg *config.Config) *Handler {
	return &Handler{manager: manager, cfg: cfg}
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func parseUintQuery(r *http.Request, key string, fallback uint64) uint64 {
	s := r.URL.Query().Get(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func parseIntQuery(r *http.Request, key string, fallback int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
