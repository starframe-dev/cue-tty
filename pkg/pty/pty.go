package pty

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creack/pty"
)

// Backend wraps a PTY master file and process state.
type Backend struct {
	master *os.File
	cmd    *exec.Cmd

	closeOnce sync.Once
	waitOnce  sync.Once
	writeMu   sync.Mutex
	closeErr  error
	waitErr   error
}

// Start runs command with args inside a pseudo-terminal.
func Start(command string, args []string, env []string, dir string, cols, rows int) (*Backend, error) {
	if dir != "" {
		absoluteDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		dir = absoluteDir
	}
	if dir != "" && !filepath.IsAbs(command) && (strings.ContainsRune(command, filepath.Separator) || strings.HasPrefix(command, ".")) {
		command = filepath.Join(dir, command)
	}
	cmd := exec.Command(command, args...)
	cmd.Env = env
	if dir != "" {
		cmd.Dir = dir
	}

	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, err
	}
	return &Backend{master: master, cmd: cmd}, nil
}

// Read reads raw bytes from the PTY master.
func (b *Backend) Read(p []byte) (int, error) { return b.master.Read(p) }

// Write writes bytes to the PTY master, handling short writes.
func (b *Backend) Write(p []byte) (int, error) {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	written := 0
	for written < len(p) {
		n, err := b.master.Write(p[written:])
		written += n
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, os.ErrInvalid
		}
	}
	return written, nil
}

// Resize changes the terminal size.
func (b *Backend) Resize(cols, rows int) error {
	return pty.Setsize(b.master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Close kills the process, closes the PTY, and waits for process reaping.
// It is safe to call repeatedly and concurrently.
func (b *Backend) Close() error {
	b.closeOnce.Do(func() {
		if b.cmd != nil && b.cmd.Process != nil {
			if err := killProcessTree(b.cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
				b.closeErr = err
			}
		}
		if b.master != nil {
			if err := b.master.Close(); err != nil && b.closeErr == nil {
				b.closeErr = err
			}
		}
		_ = b.Wait()
	})
	return b.closeErr
}

// Wait blocks until the underlying process exits and reaps it exactly once.
func (b *Backend) Wait() error {
	b.waitOnce.Do(func() {
		if b.cmd != nil {
			b.waitErr = b.cmd.Wait()
		}
	})
	return b.waitErr
}

// ProcessState returns the process state after Wait.
func (b *Backend) ProcessState() *os.ProcessState {
	if b.cmd == nil {
		return nil
	}
	return b.cmd.ProcessState
}
