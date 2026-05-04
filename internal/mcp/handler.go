package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/J1mmyLau/TerminalEndPoint/internal/session"
)

type ToolHandler struct {
	manager *session.Manager
	cfg     ExecConfig
}

type ExecConfig struct {
	MaxTimeout int
	MaxOutputKB int
}

func NewToolHandler(manager *session.Manager, cfg ExecConfig) *ToolHandler {
	if cfg.MaxTimeout == 0 {
		cfg.MaxTimeout = 120
	}
	if cfg.MaxOutputKB == 0 {
		cfg.MaxOutputKB = 100
	}
	return &ToolHandler{manager: manager, cfg: cfg}
}

func (h *ToolHandler) HandleExec(params json.RawMessage) (interface{}, error) {
	var args struct {
		Command string            `json:"command"`
		Timeout int               `json:"timeout"`
		WorkDir string            `json:"work_dir"`
		Env     map[string]string `json:"env"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	timeout := args.Timeout
	if timeout <= 0 {
		timeout = h.cfg.MaxTimeout
	}

	env := os.Environ()
	for k, v := range args.Env {
		env = append(env, k+"="+v)
	}

	s, err := h.manager.Create(session.SessionConfig{
		Shell:   "/bin/sh",
		Args:    []string{"-c", args.Command},
		WorkDir: args.WorkDir,
		Env:     env,
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer h.manager.Kill(s.ID)

	start := time.Now()

	deadline := time.After(time.Duration(timeout) * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-ticker.C:
			if !s.Alive() {
				break loop
			}
		}
	}

	var output string
	maxOutput := h.cfg.MaxOutputKB * 1024
	entries := s.OutputSince(0, 0)
	for _, e := range entries {
		decoded, _ := decodeBase64(e.Data)
		output += string(decoded)
		if len(output) > maxOutput {
			output = output[:maxOutput] + "\n... [truncated]"
			break
		}
	}

	info := s.Info()
	result := map[string]interface{}{
		"exit_code":   info.ExitCode,
		"stdout":      output,
		"duration_ms": time.Since(start).Milliseconds(),
	}
	if len(output) > maxOutput {
		result["truncated"] = true
	}

	return toContent(fmt.Sprintf("Exit code: %d\nDuration: %dms\n\n%s",
		info.ExitCode, time.Since(start).Milliseconds(), output)), nil
}

func (h *ToolHandler) HandleSpawn(params json.RawMessage) (interface{}, error) {
	var args struct {
		WorkDir string `json:"work_dir"`
		Shell   string `json:"shell"`
		Label   string `json:"label"`
		Cols    uint16 `json:"cols"`
		Rows    uint16 `json:"rows"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	s, err := h.manager.Create(session.SessionConfig{
		Shell:   args.Shell,
		WorkDir: args.WorkDir,
		Label:   args.Label,
		Cols:    args.Cols,
		Rows:    args.Rows,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	info := s.Info()
	return toContent(fmt.Sprintf("Session created: %s\nStatus: %s\nShell: %s",
		info.ID, info.Status, info.Shell)), nil
}

func (h *ToolHandler) HandleWrite(params json.RawMessage) (interface{}, error) {
	var args struct {
		SessionID string `json:"session_id"`
		Data      string `json:"data"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	s, err := h.manager.Get(args.SessionID)
	if err != nil {
		return nil, err
	}

	if _, err := s.Write([]byte(args.Data)); err != nil {
		return nil, err
	}

	return toContent("Written"), nil
}

func (h *ToolHandler) HandleRead(params json.RawMessage) (interface{}, error) {
	var args struct {
		SessionID string `json:"session_id"`
		Since     uint64 `json:"since"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	s, err := h.manager.Get(args.SessionID)
	if err != nil {
		return nil, err
	}

	entries := s.OutputSince(args.Since, args.Limit)
	var output string
	for _, e := range entries {
		decoded, _ := decodeBase64(e.Data)
		output += string(decoded)
	}

	if output == "" {
		output = "(no output)"
	}

	return toContent(output), nil
}

func (h *ToolHandler) HandleSignal(params json.RawMessage) (interface{}, error) {
	var args struct {
		SessionID string `json:"session_id"`
		Signal    string `json:"signal"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	s, err := h.manager.Get(args.SessionID)
	if err != nil {
		return nil, err
	}

	sig := parseSignal(args.Signal)
	if sig == nil {
		return nil, fmt.Errorf("unknown signal: %s", args.Signal)
	}

	if err := s.Signal(sig); err != nil {
		return nil, err
	}

	return toContent(fmt.Sprintf("Signal %s sent to session %s", args.Signal, args.SessionID)), nil
}

func (h *ToolHandler) HandleResize(params json.RawMessage) (interface{}, error) {
	var args struct {
		SessionID string `json:"session_id"`
		Cols      uint16 `json:"cols"`
		Rows      uint16 `json:"rows"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	s, err := h.manager.Get(args.SessionID)
	if err != nil {
		return nil, err
	}

	if err := s.Resize(args.Rows, args.Cols); err != nil {
		return nil, err
	}

	return toContent("Resized"), nil
}

func (h *ToolHandler) HandleKill(params json.RawMessage) (interface{}, error) {
	var args struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if err := h.manager.Kill(args.SessionID); err != nil {
		return nil, err
	}

	return toContent("Killed"), nil
}

func (h *ToolHandler) HandleList(params json.RawMessage) (interface{}, error) {
	sessions := h.manager.List()
	var output string
	if len(sessions) == 0 {
		output = "No active sessions"
	} else {
		for _, info := range sessions {
			output += fmt.Sprintf("%s  %-8s  %s  %s\n",
				info.ID[:8], info.Status, info.Shell, info.Label)
		}
	}
	return toContent(output), nil
}

func (h *ToolHandler) HandleInfo(params json.RawMessage) (interface{}, error) {
	var args struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	s, err := h.manager.Get(args.SessionID)
	if err != nil {
		return nil, err
	}

	info := s.Info()
	output := fmt.Sprintf(`ID: %s
Status: %s
Shell: %s
WorkDir: %s
ExitCode: %d
Label: %s
Created: %s
Updated: %s`,
		info.ID, info.Status, info.Shell, info.WorkDir,
		info.ExitCode, info.Label, info.CreatedAt, info.UpdatedAt)

	return toContent(output), nil
}

func toContent(text string) CallToolResult {
	return CallToolResult{
		Content: []ContentItem{
			{Type: "text", Text: text},
		},
	}
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
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
