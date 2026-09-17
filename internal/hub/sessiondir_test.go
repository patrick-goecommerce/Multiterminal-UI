package hub

import (
	"os"
	"testing"
	"time"
)

// sessionDir returns a working directory for a test session.
//
// Not t.TempDir(), and the reason is Windows. t.TempDir registers its removal
// the moment it is called, and cleanups run last-registered-first, so a
// directory created after the host is removed BEFORE the host has released the
// sessions running in it. On Unix that is fine: a directory can be unlinked
// while it is a live process's working directory. On Windows it cannot, and
// every such test failed with
//
//	TempDir RemoveAll cleanup: ... The process cannot access the file
//	because it is being used by another process
//
// which is a cleanup error, not an assertion: 22 tests went red on CI without
// a single one of them actually testing something that was broken.
//
// Removal here is best effort and retried briefly. Correct ordering alone
// would not be enough: a killed process releases its handles a moment after it
// dies, so the first attempt can still lose that race, and a leftover temp
// directory in a CI runner is not worth failing a test over.
func sessionDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mtui-session-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		for i := 0; i < 40; i++ {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	return dir
}
