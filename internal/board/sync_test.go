package board

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupSyncTestRepos creates a bare "remote" repo and a local clone with
// an initial commit pushed. Returns (localDir, remoteDir).
func setupSyncTestRepos(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()

	remoteDir := filepath.Join(base, "remote.git")
	localDir := filepath.Join(base, "local")

	// Create bare remote.
	run(t, base, "git", "init", "--bare", remoteDir)

	// Clone it.
	run(t, base, "git", "clone", remoteDir, localDir)

	// Create initial commit so the repo is not empty.
	readme := filepath.Join(localDir, "README.md")
	if err := os.WriteFile(readme, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, localDir, "git", "add", ".")
	run(t, localDir, "git", "-c", "user.name=Test", "-c", "user.email=test@test.com",
		"commit", "-m", "init")
	run(t, localDir, "git", "push", "origin", "HEAD")

	return localDir, remoteDir
}

// run executes a command and fails the test on error.
func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

func TestPushThenPull(t *testing.T) {
	localDir, remoteDir := setupSyncTestRepos(t)

	// Write a board ref in local.
	rs := NewRefStore(localDir)
	if err := rs.WriteRef("refs/mtui/board/test", []byte(`{"title":"test"}`)); err != nil {
		t.Fatal(err)
	}

	// Push refs to remote.
	syncer := NewSyncer(localDir)
	if err := syncer.Push(); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Create a second clone and pull.
	base := t.TempDir()
	local2 := filepath.Join(base, "local2")
	run(t, base, "git", "clone", remoteDir, local2)

	syncer2 := NewSyncer(local2)
	if err := syncer2.Pull(); err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	// Verify the ref exists in the second clone.
	rs2 := NewRefStore(local2)
	data, err := rs2.ReadRef("refs/mtui/board/test")
	if err != nil {
		t.Fatalf("ReadRef in clone2 failed: %v", err)
	}
	if string(data) != `{"title":"test"}` {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestPullNoRefs(t *testing.T) {
	localDir, _ := setupSyncTestRepos(t)

	// Pull when remote has no board refs — should succeed with no error.
	syncer := NewSyncer(localDir)
	if err := syncer.Pull(); err != nil {
		t.Fatalf("Pull with no remote refs should not error, got: %v", err)
	}
}

func TestPushUnreachable(t *testing.T) {
	localDir, _ := setupSyncTestRepos(t)

	// Point at a non-existent remote.
	syncer := NewSyncerWithRemote(localDir, "nonexistent")
	err := syncer.Push()
	if err == nil {
		t.Fatal("Push to unreachable remote should return error")
	}
}

func TestPullUnreachable(t *testing.T) {
	localDir, _ := setupSyncTestRepos(t)

	// Point at a non-existent remote.
	syncer := NewSyncerWithRemote(localDir, "nonexistent")
	err := syncer.Pull()
	if err == nil {
		t.Fatal("Pull from unreachable remote should return error")
	}
}
