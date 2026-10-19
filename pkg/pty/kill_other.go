//go:build !darwin && !linux

package pty

import (
	"os/exec"
)

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
