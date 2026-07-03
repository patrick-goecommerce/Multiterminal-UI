package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func newDetectTestApp() *AppService {
	return &AppService{
		worktreeState: map[int]worktreeState{},
	}
}

func TestOnWorktreeChange_DetectsNewWorktree(t *testing.T) {
	repo := initPaneTestRepo(t) // existing helper from app_worktree_pane_test.go, still present
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	var emitted *WorktreeDetectedEvent
	a.emitWorktreeEvent = func(name string, payload any) {
		if ev, ok := payload.(WorktreeDetectedEvent); ok {
			emitted = &ev
		}
	}

	a.onWorktreeChange(1, wt, "worktree-feature-a", wt)

	if emitted == nil {
		t.Fatal("expected WorktreeDetectedEvent to be emitted")
	}
	if emitted.WorktreePath != wt || emitted.WorktreeBranch != "worktree-feature-a" {
		t.Errorf("unexpected event: %+v", emitted)
	}
	if emitted.TargetBranch != "alpha-main" {
		t.Errorf("targetBranch = %q, want alpha-main (checked out in main worktree)", emitted.TargetBranch)
	}
	path, branch, ok := a.currentWorktree(1)
	if !ok || path != wt || branch != "worktree-feature-a" {
		t.Errorf("currentWorktree = %q/%q/%v", path, branch, ok)
	}
}

func TestOnWorktreeChange_ClearsWhenCwdLeavesWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(wt, 0755)
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	a.emitWorktreeEvent = func(string, any) {}
	a.onWorktreeChange(1, wt, "worktree-feature-a", wt) // enter

	var cleared *WorktreeClearedEvent
	a.emitWorktreeEvent = func(name string, payload any) {
		if ev, ok := payload.(WorktreeClearedEvent); ok {
			cleared = &ev
		}
	}
	a.onWorktreeChange(1, "", "", repo) // ordinary event, cwd back at main repo

	if cleared == nil || cleared.ID != 1 {
		t.Fatalf("expected WorktreeClearedEvent for session 1, got %+v", cleared)
	}
	if _, _, ok := a.currentWorktree(1); ok {
		t.Error("currentWorktree still reports a worktree after clear")
	}
}

func TestOnWorktreeChange_NoOpWhenCwdStaysInsideKnownWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(wt, 0755)
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	events := 0
	a.emitWorktreeEvent = func(string, any) { events++ }
	a.onWorktreeChange(1, wt, "worktree-feature-a", wt)          // enter: 1 event
	a.onWorktreeChange(1, "", "", filepath.Join(wt, "sub")) // still inside: no new event

	if events != 1 {
		t.Errorf("events = %d, want 1 (no re-emit while still inside the same worktree)", events)
	}
}
