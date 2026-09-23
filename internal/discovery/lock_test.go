package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLock_SecondAttemptIsRefused(t *testing.T) {
	t.Setenv(EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))

	first, err := AcquireLock("hub")
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer first.Release()

	if _, err := AcquireLock("hub"); !errors.Is(err, ErrLocked) {
		t.Errorf("second AcquireLock = %v, want ErrLocked", err)
	}
}

func TestAcquireLock_ReleaseLetsTheNextOneIn(t *testing.T) {
	t.Setenv(EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))

	first, err := AcquireLock("hub")
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	second, err := AcquireLock("hub")
	if err != nil {
		t.Fatalf("AcquireLock after Release: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Errorf("second Release: %v", err)
	}
}

// Different names are different locks: the daemon guard must not collide with
// whatever else wants one later.
func TestAcquireLock_NamesAreIndependent(t *testing.T) {
	t.Setenv(EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))

	a, err := AcquireLock("hub")
	if err != nil {
		t.Fatalf("AcquireLock(hub): %v", err)
	}
	defer a.Release()

	b, err := AcquireLock("other")
	if err != nil {
		t.Fatalf("AcquireLock(other): %v", err)
	}
	defer b.Release()
}

// The lock file stays put on release. Deleting it would let the next process
// create a fresh file and lock that one instead, so two processes would each
// hold "the" lock.
func TestAcquireLock_ReleaseKeepsTheFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runtime")
	t.Setenv(EnvDirOverride, dir)

	l, err := AcquireLock("hub")
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	path := l.Path()
	if err := l.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("lock file gone after Release: %v", err)
	}
}

func TestRelease_IsSafeToRepeat(t *testing.T) {
	t.Setenv(EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))

	l, err := AcquireLock("hub")
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Errorf("second Release: %v", err)
	}
}
