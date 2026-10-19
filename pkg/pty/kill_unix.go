//go:build darwin || linux

package pty

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	if err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
