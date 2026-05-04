package protocol

import "time"

// --- REST API Types ---

// CreateSessionRequest is the request body for creating a new session.
type CreateSessionRequest struct {
	WorkDir string            `json:"work_dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Shell   string            `json:"shell,omitempty"`
	Cols    uint16            `json:"cols,omitempty"`
	Rows    uint16            `json:"rows,omitempty"`
	Label   string            `json:"label,omitempty"`
	TTL     int               `json:"ttl_seconds,omitempty"`
}

// ExecRequest is the request body for executing a command in a session.
type ExecRequest struct {
	Command string            `json:"command"`
	Timeout int               `json:"timeout_seconds,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ExecResponse is returned after command execution.
type ExecResponse struct {
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Duration  int64  `json:"duration_ms"`
	Truncated bool   `json:"truncated,omitempty"`
}

// WriteRequest is the request body for writing input to a session.
type WriteRequest struct {
	Data string `json:"data"`
}

// ResizeRequest is the request body for changing terminal dimensions.
type ResizeRequest struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// SignalRequest is the request body for sending a signal.
type SignalRequest struct {
	Signal string `json:"signal"` // "SIGINT", "SIGTERM", "SIGKILL"
}

// SessionResponse is the API representation of a session.
type SessionResponse struct {
	ID        string    `json:"id"`
	Label     string    `json:"label,omitempty"`
	WorkDir   string    `json:"work_dir"`
	Shell     string    `json:"shell"`
	Status    string    `json:"status"`
	ExitCode  int       `json:"exit_code"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OutputResponse is the API representation of buffered output.
type OutputResponse struct {
	Entries  []OutputEntry `json:"entries"`
	LatestSeq uint64       `json:"latest_seq"`
}

// OutputEntry is a single output line.
type OutputEntry struct {
	Seq  uint64 `json:"seq"`
	Data string `json:"data"` // base64 encoded
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
}

// ListResponse wraps a list of sessions.
type ListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Count    int               `json:"count"`
}

// --- WebSocket Message Types ---

// WSMessage is the generic WebSocket message envelope.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// WSWrite is sent by the client to write input.
type WSWrite struct {
	Data string `json:"data"`
}

// WSResize is sent by the client to resize the terminal.
type WSResize struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// WSSignal is sent by the client to send a signal.
type WSSignal struct {
	Signal string `json:"signal"`
}

// WSOutput is sent by the server for terminal output.
type WSOutput struct {
	Stream string `json:"stream"`
	Data   string `json:"data"` // base64 encoded
	Seq    uint64 `json:"seq"`
}

// WSExit is sent by the server when the process exits.
type WSExit struct {
	Code int `json:"code"`
}

// WSError is sent by the server for errors.
type WSError struct {
	Message string `json:"message"`
}

// WSHistory is sent by the server on connect for output replay.
type WSHistory struct {
	Entries  []WSOutput `json:"entries"`
	NextSeq  uint64      `json:"next_seq,omitempty"`
}
