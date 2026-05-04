package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/J1mmyLau/TerminalEndPoint/internal/session"
	"github.com/J1mmyLau/TerminalEndPoint/pkg/protocol"
)

func (h *Handler) Exec(w http.ResponseWriter, r *http.Request) {
	var req protocol.ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = int(h.cfg.MaxExecTimeout.Seconds())
	}

	env := os.Environ()
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}

	s, err := h.manager.Create(session.SessionConfig{
		Shell:   "/bin/sh",
		Args:    []string{"-c", req.Command},
		WorkDir: req.WorkDir,
		Env:     env,
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
		return
	}
	defer h.manager.Kill(s.ID)

	start := time.Now()
	truncated := false

	deadline := time.After(time.Duration(timeout) * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-deadline:
			truncated = true
			break loop
		case <-ticker.C:
			if !s.Alive() {
				break loop
			}
		}
	}

	var stdout bytes.Buffer
	maxOutput := h.cfg.MaxExecOutputKB * 1024

	entries := s.OutputSince(0, 0)
	for _, e := range entries {
		decoded, _ := decodeBase64(e.Data)
		stdout.Write(decoded)
		if stdout.Len() > maxOutput {
			truncated = true
			break
		}
	}

	output := stdout.String()
	if truncated && len(output) > maxOutput {
		output = output[:maxOutput]
	}

	writeJSON(w, http.StatusOK, protocol.ExecResponse{
		ExitCode:  s.Info().ExitCode,
		Stdout:    output,
		Duration:  time.Since(start).Milliseconds(),
		Truncated: truncated,
	})
}
