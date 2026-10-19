//go:build darwin || linux

package privatefs

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// WriteFile writes a private file without following a pre-existing symlink.
func WriteFile(path string, data []byte) error {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
