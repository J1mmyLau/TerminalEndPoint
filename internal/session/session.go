package session

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/J1mmyLau/TerminalEndPoint/internal/pty"

	"github.com/google/uuid"
)

// Session represents an active terminal session.
type Session struct {
	ID        string
	Label     string
	WorkDir   string
	Shell     string
	Env       []string
	Status    SessionState
	ExitCode  int
	CreatedAt time.Time
	UpdatedAt time.Time

	terminal *pty.Terminal
	buffer   *RingBuffer
	subs     map[string]chan Event
	subMu    sync.RWMutex
	mu       sync.RWMutex
	cancel   context.CancelFunc
}

// NewSession creates and starts a new terminal session.
func NewSession(cfg SessionConfig) (*Session, error) {
	id := uuid.New().String()
	now := time.Now()

	shell := cfg.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	rows := cfg.Rows
	if rows == 0 {
		rows = 24
	}
	cols := cfg.Cols
	if cols == 0 {
		cols = 80
	}
	bufferSize := cfg.BufferSize
	if bufferSize == 0 {
		bufferSize = 1000
	}

	term, err := pty.New(pty.Options{
		Command: shell,
		Args:    cfg.Args,
		Dir:     cfg.WorkDir,
		Env:     cfg.Env,
		Rows:    rows,
		Cols:    cols,
	})
	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}

	s := &Session{
		ID:        id,
		Label:     cfg.Label,
		WorkDir:   cfg.WorkDir,
		Shell:     shell,
		Env:       cfg.Env,
		Status:    StateRunning,
		ExitCode:  -1,
		CreatedAt: now,
		UpdatedAt: now,
		terminal:  term,
		buffer:    NewRingBuffer(bufferSize),
		subs:      make(map[string]chan Event),
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go s.readLoop(ctx, cfg.FlushInterval, cfg.MaxFlushBytes)

	go func() {
		<-term.Done()
		s.mu.Lock()
		s.Status = StateExited
		s.ExitCode = term.ExitCode()
		s.UpdatedAt = time.Now()
		exitCode := s.ExitCode
		s.mu.Unlock()

		s.broadcast(NewExitEvent(s.ID, exitCode))
	}()

	return s, nil
}

type readResult struct {
	data []byte
	err  error
}

func (s *Session) readLoop(ctx context.Context, flushInterval time.Duration, maxFlushBytes int) {
	if flushInterval <= 0 {
		flushInterval = 50 * time.Millisecond
	}
	if maxFlushBytes <= 0 {
		maxFlushBytes = 16384
	}

	readCh := make(chan readResult, 8)
	go func() {
		defer close(readCh)
		buf := make([]byte, 4096)
		responder := newTerminalResponder(s.terminal.Write)
		for {
			n, err := s.terminal.Read(buf)
			if n > 0 {
				responder.feed(buf[:n])
				data := make([]byte, n)
				copy(data, buf[:n])
				readCh <- readResult{data: data}
			}
			if err != nil {
				readCh <- readResult{err: err}
				return
			}
		}
	}()

	var flushBuf []byte
	flushTimer := time.NewTimer(flushInterval)
	defer flushTimer.Stop()
	timerActive := false

	for {
		select {
		case <-ctx.Done():
			s.flush(&flushBuf)
			return
		case res, ok := <-readCh:
			if !ok {
				s.flush(&flushBuf)
				return
			}
			if res.err != nil {
				if res.err != io.EOF {
					s.broadcast(NewErrorEvent(s.ID, res.err.Error()))
				}
				s.flush(&flushBuf)
				return
			}
			flushBuf = append(flushBuf, res.data...)
			if len(flushBuf) >= maxFlushBytes {
				s.flush(&flushBuf)
				if timerActive {
					if !flushTimer.Stop() {
						select {
						case <-flushTimer.C:
						default:
						}
					}
					timerActive = false
				}
			} else if !timerActive {
				flushTimer.Reset(flushInterval)
				timerActive = true
			}
		case <-flushTimer.C:
			timerActive = false
			s.flush(&flushBuf)
		}
	}
}

// flush encodes and broadcasts accumulated output.
func (s *Session) flush(buf *[]byte) {
	if len(*buf) == 0 {
		return
	}
	data := make([]byte, len(*buf))
	copy(data, *buf)
	*buf = (*buf)[:0]

	encoded := base64.StdEncoding.EncodeToString(data)
	seq := s.buffer.Push(data)
	s.broadcast(NewOutputEvent(s.ID, "stdout", encoded, seq))
}

// Write sends input to the terminal.
func (s *Session) Write(data []byte) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Status != StateRunning {
		return 0, fmt.Errorf("session %s is not running (status: %s)", s.ID, s.Status)
	}
	return s.terminal.Write(data)
}

// Resize changes the terminal dimensions.
func (s *Session) Resize(rows, cols uint16) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.terminal.Resize(rows, cols)
}

// Signal sends a signal to the terminal process.
func (s *Session) Signal(sig os.Signal) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.terminal.Signal(sig)
}

// Kill terminates the session immediately.
func (s *Session) Kill() error {
	s.mu.Lock()
	if s.Status == StateKilled || s.Status == StateExited {
		s.mu.Unlock()
		return nil
	}
	s.Status = StateKilled
	s.mu.Unlock()

	s.cancel()
	err := s.terminal.Close()
	s.broadcast(NewStateEvent(s.ID, StateKilled))
	return err
}

// Subscribe registers a subscriber for session events.
func (s *Session) Subscribe(id string, ch chan Event) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.subs[id] = ch
}

// Unsubscribe removes a subscriber.
func (s *Session) Unsubscribe(id string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	delete(s.subs, id)
}

// broadcast sends an event to all subscribers.
func (s *Session) broadcast(event Event) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for id, ch := range s.subs {
		select {
		case ch <- event:
		default:
			// subscriber too slow, drop event for this subscriber
			_ = id
		}
	}
}

// Info returns a read-only snapshot of session metadata.
func (s *Session) Info() SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionInfo{
		ID:        s.ID,
		Label:     s.Label,
		WorkDir:   s.WorkDir,
		Shell:     s.Shell,
		Status:    s.Status,
		ExitCode:  s.ExitCode,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// OutputSince returns buffered output entries since the given sequence number.
func (s *Session) OutputSince(since uint64, limit int) []Entry {
	return s.buffer.Since(since, limit)
}

// LatestSeq returns the latest output sequence number.
func (s *Session) LatestSeq() uint64 {
	return s.buffer.LatestSeq()
}

// Alive returns true if the terminal process is still running.
func (s *Session) Alive() bool {
	return s.terminal.Alive()
}

// SessionConfig holds parameters for creating a new session.
type SessionConfig struct {
	Label         string
	WorkDir       string
	Shell         string
	Args          []string
	Env           []string
	Rows          uint16
	Cols          uint16
	BufferSize    int
	FlushInterval time.Duration
	MaxFlushBytes int
}

// SessionInfo is a read-only snapshot of a session's state.
type SessionInfo struct {
	ID        string       `json:"id"`
	Label     string       `json:"label,omitempty"`
	WorkDir   string       `json:"work_dir"`
	Shell     string       `json:"shell"`
	Status    SessionState `json:"status"`
	ExitCode  int          `json:"exit_code"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}
