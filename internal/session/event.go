package session

import (
	"encoding/json"
	"time"
)

// EventType classifies the type of terminal event.
type EventType string

const (
	EventOutput EventType = "output"
	EventExit   EventType = "exit"
	EventError  EventType = "error"
	EventState  EventType = "state_change"
)

// Event represents a terminal event emitted during a session's lifetime.
type Event struct {
	Type      EventType       `json:"type"`
	SessionID string          `json:"session_id"`
	Data      json.RawMessage `json:"data"`
	Seq       uint64          `json:"seq,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// OutputData is the payload for EventOutput.
type OutputData struct {
	Stream string `json:"stream"` // "stdout" or "stderr"
	Data   string `json:"data"`   // base64 encoded
}

// ExitData is the payload for EventExit.
type ExitData struct {
	Code int `json:"code"`
}

// ErrorData is the payload for EventError.
type ErrorData struct {
	Message string `json:"message"`
}

// StateData is the payload for EventState.
type StateData struct {
	Status SessionState `json:"status"`
}

// SessionState represents the lifecycle state of a session.
type SessionState string

const (
	StateStarting SessionState = "starting"
	StateRunning  SessionState = "running"
	StateExited   SessionState = "exited"
	StateKilled   SessionState = "killed"
	StateError    SessionState = "error"
)

// NewOutputEvent creates an output event.
func NewOutputEvent(sessionID string, stream string, data string, seq uint64) Event {
	payload, _ := json.Marshal(OutputData{Stream: stream, Data: data})
	return Event{
		Type:      EventOutput,
		SessionID: sessionID,
		Data:      payload,
		Seq:       seq,
		Timestamp: time.Now(),
	}
}

// NewExitEvent creates an exit event.
func NewExitEvent(sessionID string, code int) Event {
	payload, _ := json.Marshal(ExitData{Code: code})
	return Event{
		Type:      EventExit,
		SessionID: sessionID,
		Data:      payload,
		Timestamp: time.Now(),
	}
}

// NewErrorEvent creates an error event.
func NewErrorEvent(sessionID string, message string) Event {
	payload, _ := json.Marshal(ErrorData{Message: message})
	return Event{
		Type:      EventError,
		SessionID: sessionID,
		Data:      payload,
		Timestamp: time.Now(),
	}
}

// NewStateEvent creates a state change event.
func NewStateEvent(sessionID string, state SessionState) Event {
	payload, _ := json.Marshal(StateData{Status: state})
	return Event{
		Type:      EventState,
		SessionID: sessionID,
		Data:      payload,
		Timestamp: time.Now(),
	}
}
