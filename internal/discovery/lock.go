package discovery

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrLocked means another live process holds the lock.
var ErrLocked = errors.New("discovery: lock held by another process")

// FileLock is an exclusive lock on a file in the per-user runtime directory.
//
// It exists for the session daemon's single-instance guard. Checking the
// discovery record first is not enough: two clients starting at the same
// moment both find no record, both spawn a daemon, and the second one's
// Publish overwrites the first, leaving an invisible daemon holding sessions
// nobody can see. An OS lock decides that race in the kernel, where there is
// no window between looking and taking.
//
// The lock is released when the process exits, however it exits, so a crash
// does not leave it stuck the way a stale record would.
type FileLock struct {
	f    *os.File
	path string
}

// AcquireLock takes the named lock without waiting. It returns ErrLocked if
// another process holds it.
func AcquireLock(name string) (*FileLock, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("discovery: create %s: %w", dir, err)
	}
	path := filepath.Join(dir, name+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("discovery: open %s: %w", path, err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &FileLock{f: f, path: path}, nil
}

// Release drops the lock. The file itself stays: removing it would let a
// second process create a new one and lock that instead, which is the classic
// way a lock file stops locking anything.
func (l *FileLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := unlockFile(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	l.f = nil
	return err
}

// Path returns the lock file's path, for diagnostics.
func (l *FileLock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
