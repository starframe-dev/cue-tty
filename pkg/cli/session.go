// Package cli provides the interactive REPL for cue-tty.
package cli

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/starframe-dev/cue-tty/pkg/cue"
)

// Session represents a named TUI session.
type Session struct {
	Name     string
	App      *cue.App
	Page     *cue.Page
	HTTPAddr string // HTTP address of the app's element tree API
	WorkDir  string // working directory of the client (for save/artifacts)
}

// SessionManager manages named sessions.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// GetOrCreate returns an existing session or creates a new one.
func (sm *SessionManager) GetOrCreate(name string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.sessions[name]; ok {
		return s
	}

	s := &Session{Name: name}
	sm.sessions[name] = s
	return s
}

// Get returns a session by name.
func (sm *SessionManager) Get(name string) (*Session, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.sessions[name]
	return s, ok
}

// List returns all session names.
func (sm *SessionManager) List() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	names := make([]string, 0, len(sm.sessions))
	for name := range sm.sessions {
		names = append(names, name)
	}
	return names
}

// Close closes a session by name.
func (sm *SessionManager) Close(name string) error {
	sm.mu.Lock()
	s, ok := sm.sessions[name]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session %q not found", name)
	}
	delete(sm.sessions, name)
	sm.mu.Unlock()

	if s.App != nil {
		return s.App.Close()
	}
	return nil
}

// CloseAll closes all sessions.
func (sm *SessionManager) CloseAll() []error {
	sm.mu.Lock()
	names := make([]string, 0, len(sm.sessions))
	for name := range sm.sessions {
		names = append(names, name)
	}
	sm.mu.Unlock()

	var errs []error
	for _, name := range names {
		if err := sm.Close(name); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Launch starts a TUI application in the session.
// If the app supports HTTP element tree (warp-based), cuetty-cli allocates
// a free port and passes it via WARP_HTTP_PORT env variable.
func (s *Session) Launch(command string, args ...string) error {
	if s.App != nil {
		return fmt.Errorf("app already running in session %q, use close first", s.Name)
	}

	// Allocate a free port for the app's HTTP element tree
	port := findFreePort()
	if port > 0 {
		s.HTTPAddr = "http://localhost:" + strconv.Itoa(port)
	}

	opts := []cue.Option{
		cue.WithArgs(args...),
		cue.WithSize(80, 24),
	}
	if port > 0 {
		opts = append(opts, cue.WithEnv("WARP_HTTP_PORT="+strconv.Itoa(port)))
	}

	app, err := cue.Launch(command, opts...)
	if err != nil {
		return fmt.Errorf("launch: %w", err)
	}

	s.App = app
	s.Page = app.Page()
	return nil
}

// findFreePort finds a free TCP port by listening on :0.
func findFreePort() int {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// Close stops the application in this session.
func (s *Session) Close() error {
	if s.App == nil {
		return nil
	}
	return s.App.Close()
}
