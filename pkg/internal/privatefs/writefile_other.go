//go:build !darwin && !linux

package privatefs

import "os"

// WriteFile writes a private file, refusing an existing symlink.
func WriteFile(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return os.ErrPermission
	}
	return os.WriteFile(path, data, 0o600)
}
