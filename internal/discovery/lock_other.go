//go:build !windows

package discovery

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lockFile takes an exclusive flock without blocking. The lock belongs to the
// open file description, so a second Open of the same path conflicts even
// within one process.
func lockFile(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return ErrLocked
	}
	return fmt.Errorf("discovery: flock %s: %w", f.Name(), err)
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
