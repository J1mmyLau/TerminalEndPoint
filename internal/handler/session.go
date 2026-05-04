package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"syscall"

	"github.com/J1mmyLau/TerminalEndPoint/internal/session"
	"github.com/J1mmyLau/TerminalEndPoint/pkg/protocol"
)

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	env := os.Environ()
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}

	s, err := h.manager.Create(session.SessionConfig{
		Label:   req.Label,
		WorkDir: req.WorkDir,
		Shell:   req.Shell,
		Env:     env,
		Rows:    req.Rows,
		Cols:    req.Cols,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
		return
	}

	info := s.Info()
	writeJSON(w, http.StatusCreated, toSessionResponse(info))
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.manager.List()
	resp := make([]protocol.SessionResponse, 0, len(sessions))
	for _, info := range sessions {
		resp = append(resp, toSessionResponse(info))
	}
	writeJSON(w, http.StatusOK, protocol.ListResponse{
		Sessions: resp,
		Count:    len(resp),
	})
}

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.manager.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toSessionResponse(s.Info()))
}

func (h *Handler) KillSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.Kill(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "killed", "id": id})
}

func (h *Handler) Resize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.manager.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req protocol.ResizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.Resize(req.Rows, req.Cols); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "resized"})
}

func (h *Handler) Signal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.manager.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req protocol.SignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	sig := parseSignal(req.Signal)
	if sig == nil {
		writeError(w, http.StatusBadRequest, "unknown signal: "+req.Signal)
		return
	}

	if err := s.Signal(sig); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "signaled", "signal": req.Signal})
}

func (h *Handler) Write(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.manager.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req protocol.WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if _, err := s.Write([]byte(req.Data)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "written"})
}

func (h *Handler) GetOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.manager.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	since := parseUintQuery(r, "since", 0)
	limit := parseIntQuery(r, "limit", 0)

	entries := s.OutputSince(since, limit)
	output := make([]protocol.OutputEntry, 0, len(entries))
	for _, e := range entries {
		output = append(output, protocol.OutputEntry{
			Seq:  e.Seq,
			Data: e.Data,
		})
	}

	writeJSON(w, http.StatusOK, protocol.OutputResponse{
		Entries:   output,
		LatestSeq: s.LatestSeq(),
	})
}

func toSessionResponse(info session.SessionInfo) protocol.SessionResponse {
	return protocol.SessionResponse{
		ID:        info.ID,
		Label:     info.Label,
		WorkDir:   info.WorkDir,
		Shell:     info.Shell,
		Status:    string(info.Status),
		ExitCode:  info.ExitCode,
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, protocol.ErrorResponse{Error: message, Code: status})
}

func parseSignal(name string) os.Signal {
	switch name {
	case "SIGINT":
		return syscall.SIGINT
	case "SIGTERM":
		return syscall.SIGTERM
	case "SIGKILL":
		return syscall.SIGKILL
	case "SIGHUP":
		return syscall.SIGHUP
	case "SIGQUIT":
		return syscall.SIGQUIT
	default:
		return nil
	}
}
