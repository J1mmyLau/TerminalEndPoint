package session

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewOutputEvent(t *testing.T) {
	event := NewOutputEvent("sid-1", "stdout", "SGVsbG8=", 42)

	if event.Type != EventOutput {
		t.Fatalf("expected EventOutput, got %s", event.Type)
	}
	if event.SessionID != "sid-1" {
		t.Fatalf("expected sid-1, got %s", event.SessionID)
	}
	if event.Seq != 42 {
		t.Fatalf("expected seq 42, got %d", event.Seq)
	}
	if event.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}

	var data OutputData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal output data: %v", err)
	}
	if data.Stream != "stdout" {
		t.Fatalf("expected stdout, got %s", data.Stream)
	}
	if data.Data != "SGVsbG8=" {
		t.Fatalf("expected SGVsbG8=, got %s", data.Data)
	}
}

func TestNewExitEvent(t *testing.T) {
	event := NewExitEvent("sid-1", 42)

	if event.Type != EventExit {
		t.Fatalf("expected EventExit, got %s", event.Type)
	}

	var data ExitData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal exit data: %v", err)
	}
	if data.Code != 42 {
		t.Fatalf("expected code 42, got %d", data.Code)
	}
}

func TestNewErrorEvent(t *testing.T) {
	event := NewErrorEvent("sid-1", "something went wrong")

	if event.Type != EventError {
		t.Fatalf("expected EventError, got %s", event.Type)
	}

	var data ErrorData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal error data: %v", err)
	}
	if data.Message != "something went wrong" {
		t.Fatalf("unexpected message: %s", data.Message)
	}
}

func TestNewStateEvent(t *testing.T) {
	event := NewStateEvent("sid-1", StateKilled)

	if event.Type != EventState {
		t.Fatalf("expected EventState, got %s", event.Type)
	}

	var data StateData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal state data: %v", err)
	}
	if data.Status != StateKilled {
		t.Fatalf("expected StateKilled, got %s", data.Status)
	}
}

func TestEventRoundtrip(t *testing.T) {
	event := NewOutputEvent("sid-1", "stdout", "dGVzdA==", 1)

	marshaled, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var unmarshaled Event
	if err := json.Unmarshal(marshaled, &unmarshaled); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if unmarshaled.Type != event.Type {
		t.Fatalf("type mismatch after roundtrip")
	}
	if unmarshaled.Seq != event.Seq {
		t.Fatalf("seq mismatch after roundtrip")
	}
	if !unmarshaled.Timestamp.Equal(event.Timestamp) {
		t.Fatalf("timestamp mismatch after roundtrip")
	}
}

func TestSessionStateString(t *testing.T) {
	if StateRunning != "running" {
		t.Fatalf("expected 'running', got '%s'", StateRunning)
	}
	if StateExited != "exited" {
		t.Fatalf("expected 'exited', got '%s'", StateExited)
	}
	if StateKilled != "killed" {
		t.Fatalf("expected 'killed', got '%s'", StateKilled)
	}
}

func TestEventTimestamp(t *testing.T) {
	before := time.Now()
	event := NewOutputEvent("sid", "stdout", "", 0)
	after := time.Now()

	if event.Timestamp.Before(before) || event.Timestamp.After(after) {
		t.Fatal("timestamp should be between before and after")
	}
}
