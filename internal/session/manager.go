package session

import (
	"fmt"
	"sync"
	"time"
)

// Manager orchestrates the lifecycle of terminal sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	cfg      ManagerConfig
	stopCh   chan struct{}
}

// ManagerConfig holds configuration for the session manager.
type ManagerConfig struct {
	MaxSessions int
	SessionTTL  time.Duration
}

// NewManager creates a session manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxSessions == 0 {
		cfg.MaxSessions = 50
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 5 * time.Minute
	}

	m := &Manager{
		sessions: make(map[string]*Session),
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}

	go m.reapLoop()

	return m
}

// Create starts a new terminal session.
func (m *Manager) Create(cfg SessionConfig) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.cfg.MaxSessions {
		return nil, fmt.Errorf("max sessions reached (%d)", m.cfg.MaxSessions)
	}

	s, err := NewSession(cfg)
	if err != nil {
		return nil, err
	}

	m.sessions[s.ID] = s
	return s, nil
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return s, nil
}

// List returns info for all active sessions.
func (m *Manager) List() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s.Info())
	}
	return result
}

// Kill terminates and removes a session.
func (m *Manager) Kill(id string) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}

	if err := s.Kill(); err != nil {
		return fmt.Errorf("kill session %s: %w", id, err)
	}

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	return nil
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// reapLoop periodically cleans up expired sessions.
func (m *Manager) reapLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.reap()
		case <-m.stopCh:
			return
		}
	}
}

// reap removes sessions that have been idle past their TTL.
func (m *Manager) reap() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, s := range m.sessions {
		if s.Status != StateRunning {
			if now.Sub(s.UpdatedAt) > m.cfg.SessionTTL {
				_ = s.Kill()
				delete(m.sessions, id)
			}
		}
	}
}

// Shutdown gracefully terminates all sessions.
func (m *Manager) Shutdown() {
	close(m.stopCh)

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, s := range m.sessions {
		_ = s.Kill()
		delete(m.sessions, id)
	}
}
