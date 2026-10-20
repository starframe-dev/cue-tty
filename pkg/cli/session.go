// Package cli provides the interactive REPL for cue-tty.
package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/cue"
)

// Session represents a named TUI session.
type Session struct {
	Name           string
	App            *cue.App
	Page           *cue.Page
	HTTPAddr       string // HTTP address of the app's element tree API
	httpToken      string // per-session bearer token for the element tree API
	WorkDir        string // working directory of the client (for save/artifacts)
	mu             sync.Mutex
	cancelMu       sync.Mutex
	activeCancel   context.CancelFunc
	commandContext context.Context
}

// SessionManager manages named sessions.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// SessionSnapshot is an immutable dashboard view.
type SessionSnapshot struct {
	Name    string
	Running bool
	Text    string
}

// NewSessionManager creates a new session manager.
func validSessionName(name string) bool {
	return name != "" && name != "." && name != ".." && !filepath.IsAbs(name) && filepath.Base(name) == name
}

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

// IsCurrent reports whether s is still the manager's current session for name.
func (sm *SessionManager) IsCurrent(name string, s *Session) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessions[name] == s
}

// Snapshots copies manager pointers before acquiring any session locks.
func (sm *SessionManager) Snapshots() []SessionSnapshot {
	sm.mu.Lock()
	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	sm.mu.Unlock()

	snapshots := make([]SessionSnapshot, 0, len(sessions))
	for _, s := range sessions {
		snapshots = append(snapshots, s.Snapshot())
	}
	return snapshots
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
	sm.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %q not found", name)
	}

	s.cancelCurrent()
	s.mu.Lock()
	defer s.mu.Unlock()
	sm.mu.Lock()
	if sm.sessions[name] == s {
		delete(sm.sessions, name)
	}
	sm.mu.Unlock()
	return s.Close()
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

// Launch starts a TUI application in the session. Warp-compatible apps receive
// an already-bound loopback listener on descriptor 3, avoiding a port rebind race.
func (s *Session) Launch(command string, args ...string) error {
	if s.App != nil {
		return fmt.Errorf("app already running in session %q, use close first", s.Name)
	}

	s.HTTPAddr = ""
	s.httpToken = ""
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("reserve HTTP inspector listener: %w", err)
	}
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		_ = listener.Close()
		return fmt.Errorf("reserve HTTP inspector listener: unexpected listener type %T", listener)
	}
	listenerFile, err := tcpListener.File()
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("duplicate HTTP inspector listener: %w", err)
	}
	defer listener.Close()
	defer listenerFile.Close()

	token, err := newHTTPToken()
	if err != nil {
		return fmt.Errorf("generate HTTP inspector token: %w", err)
	}
	port := tcpListener.Addr().(*net.TCPAddr).Port
	s.httpToken = token
	s.HTTPAddr = "http://localhost:" + strconv.Itoa(port)

	opts := []cue.Option{
		cue.WithArgs(args...),
		cue.WithSize(80, 24),
		cue.WithEnv(
			"WARP_HTTP_PORT="+strconv.Itoa(port),
			"WARP_HTTP_TOKEN="+s.httpToken,
			"WARP_HTTP_FD=3",
		),
		cue.WithExtraFiles(listenerFile),
	}
	if s.WorkDir != "" {
		opts = append(opts, cue.WithDir(s.WorkDir))
	}

	app, err := cue.Launch(command, opts...)
	if err != nil {
		s.HTTPAddr = ""
		s.httpToken = ""
		return fmt.Errorf("launch: %w", err)
	}

	s.App = app
	s.Page = app.Page()
	return nil
}
func newHTTPToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func (s *Session) lockContext(ctx context.Context) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.mu.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Session) beginCommand(ctx context.Context, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	s.activeCancel = cancel
	s.cancelMu.Unlock()
	s.commandContext = ctx // caller holds s.mu
}

func (s *Session) endCommand() {
	s.cancelMu.Lock()
	s.activeCancel = nil
	s.cancelMu.Unlock()
	s.commandContext = nil // caller holds s.mu
}

func (s *Session) cancelCurrent() {
	s.cancelMu.Lock()
	cancel := s.activeCancel
	s.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Session) context() context.Context {
	if s.commandContext != nil {
		return s.commandContext
	}
	return context.Background()
}

// Snapshot returns status and screen text under the session lock.
func (s *Session) Snapshot() SessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := SessionSnapshot{Name: s.Name, Running: s.App != nil}
	if s.App != nil {
		snapshot.Text = s.App.Snapshot().Text
	}
	return snapshot
}

// Close stops the application in this session.
func (s *Session) Close() error {
	if s.App == nil {
		s.HTTPAddr = ""
		s.httpToken = ""
		s.Page = nil
		return nil
	}
	app := s.App
	s.App = nil
	s.Page = nil
	s.HTTPAddr = ""
	s.httpToken = ""
	return app.Close()
}
