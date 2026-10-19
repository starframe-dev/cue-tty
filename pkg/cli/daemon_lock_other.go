//go:build !darwin && !linux

package cli

func acquireDaemonLock(string) (func(), error) {
	return func() {}, nil
}
