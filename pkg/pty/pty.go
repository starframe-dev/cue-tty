package pty

import (
    "os"
    "os/exec"

    "github.com/creack/pty"
)

// Backend wraps a PTY master file and process state.
type Backend struct {
    master *os.File
    cmd    *exec.Cmd
}

// Start runs command with args inside a pseudo-terminal.
// It returns the PTY master file for reading/writing.
func Start(command string, args []string, env []string, dir string, cols, rows int) (*Backend, error) {
    cmd := exec.Command(command, args...)
    cmd.Env = env
    if dir != "" {
        cmd.Dir = dir
    }

    master, err := pty.StartWithSize(cmd, &pty.Winsize{
        Rows: uint16(rows),
        Cols: uint16(cols),
    })
    if err != nil {
        return nil, err
    }

    return &Backend{master: master, cmd: cmd}, nil
}

// Read reads raw bytes from the PTY master.
func (b *Backend) Read(p []byte) (int, error) {
    return b.master.Read(p)
}

// Write writes raw bytes to the PTY master.
func (b *Backend) Write(p []byte) (int, error) {
    return b.master.Write(p)
}

// Resize changes the terminal size.
func (b *Backend) Resize(cols, rows int) error {
    return pty.Setsize(b.master, &pty.Winsize{
        Rows: uint16(rows),
        Cols: uint16(cols),
    })
}

// Close kills the process and closes the PTY.
func (b *Backend) Close() error {
    if b.cmd != nil && b.cmd.Process != nil {
        _ = b.cmd.Process.Kill()
    }
    if b.master != nil {
        return b.master.Close()
    }
    return nil
}

// Wait blocks until the underlying process exits.
func (b *Backend) Wait() error {
    return b.cmd.Wait()
}

// ProcessState returns the process state after Wait.
func (b *Backend) ProcessState() *os.ProcessState {
    return b.cmd.ProcessState
}
