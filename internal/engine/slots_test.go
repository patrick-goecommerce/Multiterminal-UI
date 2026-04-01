package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// initTestRepo creates a temporary git repo with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	// Create initial commit so branches can be created.
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("init"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func TestAllocateReturnsValidWorkDir(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 3)

	_, workDir, err := mgr.Allocate(context.Background(), "mtui/card1/step1")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		t.Fatalf("workDir does not exist: %s", workDir)
	}
}

func TestAllocateCreatesGitWorktree(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 3)

	_, workDir, err := mgr.Allocate(context.Background(), "mtui/card1/step2")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	// A git worktree has a .git file (not directory).
	gitFile := filepath.Join(workDir, ".git")
	info, err := os.Stat(gitFile)
	if err != nil {
		t.Fatalf(".git not found in worktree: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected .git to be a file (worktree), not a directory")
	}
}

func TestReleaseRemovesWorktree(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 3)

	slotID, workDir, err := mgr.Allocate(context.Background(), "mtui/card1/step3")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if err := mgr.Release(slotID); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatalf("workDir still exists after Release: %s", workDir)
	}
}

func TestAllocateReusesSlotAfterRelease(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 1)

	slotID1, _, err := mgr.Allocate(context.Background(), "mtui/card1/a")
	if err != nil {
		t.Fatalf("Allocate 1: %v", err)
	}
	if err := mgr.Release(slotID1); err != nil {
		t.Fatalf("Release: %v", err)
	}

	slotID2, _, err := mgr.Allocate(context.Background(), "mtui/card1/b")
	if err != nil {
		t.Fatalf("Allocate 2: %v", err)
	}
	if slotID2 != slotID1 {
		t.Fatalf("expected slot reuse: got %d, want %d", slotID2, slotID1)
	}
}

func TestPoolExhaustionBlocks(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 1)

	_, _, err := mgr.Allocate(context.Background(), "mtui/card1/x")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// Second allocate with a short timeout should fail.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _, err = mgr.Allocate(ctx, "mtui/card1/y")
	if err == nil {
		t.Fatal("expected Allocate to block and fail on timeout")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestParkKeepsDirectoryButFreesSlot(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 1)

	slotID, workDir, err := mgr.Allocate(context.Background(), "mtui/card1/p")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if err := mgr.Park(slotID); err != nil {
		t.Fatalf("Park: %v", err)
	}

	// Directory should still exist.
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		t.Fatal("parked worktree directory was removed")
	}
	// Slot freed — ActiveSlots should be 0.
	if n := mgr.ActiveSlots(); n != 0 {
		t.Fatalf("ActiveSlots after Park: got %d, want 0", n)
	}
}

func TestPruneRemovesParkedWorktrees(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 2)

	slotID, workDir, err := mgr.Allocate(context.Background(), "mtui/card1/pr")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if err := mgr.Park(slotID); err != nil {
		t.Fatalf("Park: %v", err)
	}
	if err := mgr.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatal("parked worktree should have been removed by Prune")
	}
}

func TestActiveSlots(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 3)

	if n := mgr.ActiveSlots(); n != 0 {
		t.Fatalf("initial ActiveSlots: got %d, want 0", n)
	}

	id1, _, _ := mgr.Allocate(context.Background(), "mtui/c/s1")
	id2, _, _ := mgr.Allocate(context.Background(), "mtui/c/s2")

	if n := mgr.ActiveSlots(); n != 2 {
		t.Fatalf("ActiveSlots after 2 allocs: got %d, want 2", n)
	}

	mgr.Release(id1)
	if n := mgr.ActiveSlots(); n != 1 {
		t.Fatalf("ActiveSlots after 1 release: got %d, want 1", n)
	}

	mgr.Release(id2)
	if n := mgr.ActiveSlots(); n != 0 {
		t.Fatalf("ActiveSlots after all released: got %d, want 0", n)
	}
}

func TestContextCancellationUnblocksAllocate(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 1)

	_, _, _ = mgr.Allocate(context.Background(), "mtui/c/block")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := mgr.Allocate(ctx, "mtui/c/wait")
		done <- err
	}()

	// Give goroutine time to block, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Allocate did not unblock after context cancellation")
	}
}

func TestMultipleAllocateReleaseCycles(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 2)

	for i := 0; i < 5; i++ {
		branch := "mtui/cycle/" + string(rune('a'+i))
		id, workDir, err := mgr.Allocate(context.Background(), branch)
		if err != nil {
			t.Fatalf("cycle %d Allocate: %v", i, err)
		}
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			t.Fatalf("cycle %d workDir missing", i)
		}
		if err := mgr.Release(id); err != nil {
			t.Fatalf("cycle %d Release: %v", i, err)
		}
	}
}

func TestReleaseUnblocksWaitingAllocate(t *testing.T) {
	repo := initTestRepo(t)
	mgr := NewWorktreeSlotManager(repo, 1)

	id, _, _ := mgr.Allocate(context.Background(), "mtui/c/first")

	done := make(chan error, 1)
	go func() {
		_, _, err := mgr.Allocate(context.Background(), "mtui/c/second")
		done <- err
	}()

	// Let the goroutine block, then release.
	time.Sleep(50 * time.Millisecond)
	mgr.Release(id)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second Allocate failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second Allocate did not unblock after Release")
	}
}
