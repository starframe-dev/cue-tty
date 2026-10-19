package pty

import (
	"context"
	"errors"
	"fmt"
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
	processMu sync.RWMutex
	closeErr  error
	waitErr   error
}

const (
	maxTerminalDimension = 4096
	maxTerminalCells     = 1 << 20
)

// ValidateSize rejects dimensions that cannot be represented safely by the
// PTY or that would allocate an excessive virtual screen.
func ValidateSize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("terminal dimensions must be positive: %dx%d", cols, rows)
	}
	if cols > 0xffff || rows > 0xffff || cols > maxTerminalDimension || rows > maxTerminalDimension {
		return fmt.Errorf("terminal dimensions exceed the maximum %dx%d: %dx%d", maxTerminalDimension, maxTerminalDimension, cols, rows)
	}
	if cols > maxTerminalCells/rows {
		return fmt.Errorf("terminal area exceeds the maximum of %d cells: %dx%d", maxTerminalCells, cols, rows)
	}
	return nil
}

// Start runs command with args inside a pseudo-terminal.
func Start(command string, args []string, env []string, dir string, cols, rows int) (*Backend, error) {
	if err := ValidateSize(cols, rows); err != nil {
		return nil, err
	}
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

// WriteContext writes bytes in order and closes the backend if cancellation
// must interrupt a blocked PTY write. Closing the master releases the writer.
func (b *Backend) WriteContext(ctx context.Context, p []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	type writeResult struct {
		written int
		err     error
	}
	result := make(chan writeResult, 1)
	go func() {
		written, err := b.Write(p)
		result <- writeResult{written: written, err: err}
	}()

	select {
	case res := <-result:
		return res.written, res.err
	case <-ctx.Done():
		select {
		case res := <-result:
			return res.written, res.err
		default:
		}
		_ = b.Close()
		res := <-result
		return res.written, ctx.Err()
	}
}

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

// Resize changes the terminal size after validating dimensions.
func (b *Backend) Resize(cols, rows int) error {
	if err := ValidateSize(cols, rows); err != nil {
		return err
	}
	return pty.Setsize(b.master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Size returns the PTY master dimensions as columns and rows.
func (b *Backend) Size() (cols, rows int, err error) {
	rows, cols, err = pty.Getsize(b.master)
	return cols, rows, err
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
			b.processMu.Lock()
			b.waitErr = b.cmd.Wait()
			b.processMu.Unlock()
		}
	})
	return b.waitErr
}

// ProcessState returns the process state after Wait.
func (b *Backend) ProcessState() *os.ProcessState {
	if b.cmd == nil {
		return nil
	}
	b.processMu.RLock()
	defer b.processMu.RUnlock()
	return b.cmd.ProcessState
}
