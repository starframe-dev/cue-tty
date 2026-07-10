package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Daemon provides persistent session management via HTTP API over Unix socket.
type Daemon struct {
	mu       sync.RWMutex
	sm       *SessionManager
	server   *http.Server
	socketPath string
}

// DaemonRequest is a command sent to the daemon.
type DaemonRequest struct {
	Session string   `json:"session"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	WorkDir string   `json:"workdir,omitempty"`
}

// DaemonResponse is the daemon's reply.
type DaemonResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// socketDir returns the directory for the daemon socket.
func socketDir() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return runtimeDir
	}
	return os.TempDir()
}

// SocketPath returns the default Unix socket path for the daemon.
func SocketPath() string {
	return filepath.Join(socketDir(), fmt.Sprintf("cuetty-%s.sock", os.Getenv("USER")))
}

// NewDaemon creates a new daemon instance.
func NewDaemon() *Daemon {
	return &Daemon{
		sm:         NewSessionManager(),
		socketPath: SocketPath(),
	}
}

// IsDaemonRunning checks if the daemon is listening on its socket.
func IsDaemonRunning() bool {
	path := SocketPath()
	if _, err := os.Stat(path); err != nil {
		return false
	}

	client := http.Client{
		Timeout: 100 * time.Millisecond,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	resp, err := client.Get("http://daemon/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// StartDaemon starts the daemon process in the background.
func StartDaemon() error {
	if IsDaemonRunning() {
		return nil
	}

	cmd := exec.Command(os.Args[0], "daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	// Wait for daemon to be ready
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if IsDaemonRunning() {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("daemon failed to start")
}

// Run starts the daemon and blocks until it is stopped.
func (d *Daemon) Run() error {
	// Remove stale socket
	_ = os.Remove(d.socketPath)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", d.handleHealthz)
	mux.HandleFunc("/cmd", d.handleCommand)
	mux.HandleFunc("/show", d.handleShow)
	mux.HandleFunc("/list", d.handleList)
	mux.HandleFunc("/stop", d.handleStop)

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", d.socketPath, err)
	}
	defer listener.Close()

	// Ensure socket is writable by the user
	if err := os.Chmod(d.socketPath, 0700); err != nil {
		return fmt.Errorf("chmod socket: %w", err)
	}

	d.server = &http.Server{Handler: mux}
	return d.server.Serve(listener)
}

// Stop stops the daemon.
func (d *Daemon) Stop() error {
	if d.server == nil {
		return nil
	}
	return d.server.Close()
}

func (d *Daemon) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (d *Daemon) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DaemonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("decode request: %w", err))
		return
	}

	d.mu.RLock()
	commands := Commands(d.sm)
	d.mu.RUnlock()

	// If the command is "stop", handle it specially.
	if req.Command == "stop" {
		writeResponse(w, "Daemon stopped")
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = d.Stop()
		}()
		return
	}

	session := d.sm.GetOrCreate(req.Session)
	session.WorkDir = req.WorkDir
	line := req.Command
	if len(req.Args) > 0 {
		line += " " + strings.Join(req.Args, " ")
	}

	result, err := runCommand(session, commands, line)
	if err != nil {
		writeError(w, err)
		return
	}

	writeResponse(w, result)
}

func (d *Daemon) handleShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	showHandler := Commands(d.sm)["show"]
	result, err := showHandler(nil, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResponse(w, result)
}

func (d *Daemon) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	listHandler := Commands(d.sm)["list"]
	result, err := listHandler(nil, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResponse(w, result)
}

func (d *Daemon) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeResponse(w, "Daemon stopped")
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = d.Stop()
	}()
}

func writeResponse(w http.ResponseWriter, output string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(DaemonResponse{Output: output})
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // keep HTTP 200, error in body
	_ = json.NewEncoder(w).Encode(DaemonResponse{Error: err.Error()})
}

// SendCommand sends a command to the daemon and returns the response.
func SendCommand(session, command string, args []string) (string, error) {
	if !IsDaemonRunning() {
		if err := StartDaemon(); err != nil {
			return "", fmt.Errorf("daemon not running: %w", err)
		}
	}

	client := http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", SocketPath())
			},
		},
	}

	req := DaemonRequest{
		Session: session,
		Command: command,
		Args:    args,
		WorkDir: mustGetwd(),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := client.Post("http://daemon/cmd", "application/json", strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("send command: %w", err)
	}
	defer resp.Body.Close()

	var result DaemonResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}

	return result.Output, nil
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
