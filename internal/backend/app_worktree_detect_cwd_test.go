package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// A pane can sit in a linked worktree without Claude ever having called the
// EnterWorktree tool in that session — it was restored there, resumed with
// --continue, or moved with a plain `cd`. Measured on a real installation,
// 11 of 48 worktree sessions never produced a PostToolUse:EnterWorktree hook
// event. Before this, the cwd carried on every hook event was only ever used
// to CLEAR a known worktree, so those panes fell back to the main-repo badge
// and showed the base branch forever.

func TestOnWorktreeChange_DetectsWorktreeFromCwdWithoutEnterWorktreeEvent(t *testing.T) {
	repo := initPaneTestRepo(t)
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

	// An ordinary hook event: no worktree_path, cwd inside the linked worktree.
	a.onWorktreeChange(1, "", "", wt)

	if emitted == nil {
		t.Fatal("expected WorktreeDetectedEvent derived from cwd alone")
	}
	if emitted.WorktreePath != wt {
		t.Errorf("worktreePath = %q, want %q", emitted.WorktreePath, wt)
	}
	if emitted.WorktreeBranch != "worktree-feature-a" {
		t.Errorf("worktreeBranch = %q, want worktree-feature-a", emitted.WorktreeBranch)
	}
	if emitted.TargetBranch != "alpha-main" {
		t.Errorf("targetBranch = %q, want alpha-main", emitted.TargetBranch)
	}
	path, branch, ok := a.currentWorktree(1)
	if !ok || path != wt || branch != "worktree-feature-a" {
		t.Errorf("currentWorktree = %q/%q/%v", path, branch, ok)
	}
}

func TestOnWorktreeChange_IgnoresCwdInMainRepo(t *testing.T) {
	repo := initPaneTestRepo(t)

	a := newDetectTestApp()
	events := 0
	a.emitWorktreeEvent = func(string, any) { events++ }

	a.onWorktreeChange(1, "", "", repo)
	a.onWorktreeChange(1, "", "", filepath.Join(repo, "internal"))

	if events != 0 {
		t.Errorf("events = %d, want 0 — the main checkout is not a worktree", events)
	}
}

func TestOnWorktreeChange_SwitchesBetweenWorktreesFromCwd(t *testing.T) {
	repo := initPaneTestRepo(t)
	wtA := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	wtB := filepath.Join(repo, ".claude", "worktrees", "feature-b")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wtA)
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-b", wtB)

	a := newDetectTestApp()
	a.emitWorktreeEvent = func(string, any) {}
	a.onWorktreeChange(1, wtA, "worktree-feature-a", wtA)

	var last *WorktreeDetectedEvent
	cleared := 0
	a.emitWorktreeEvent = func(name string, payload any) {
		switch ev := payload.(type) {
		case WorktreeDetectedEvent:
			last = &ev
		case WorktreeClearedEvent:
			cleared++
		}
	}

	// Claude moved to the other worktree with a plain cd — no tool call.
	a.onWorktreeChange(1, "", "", wtB)

	if cleared != 0 {
		t.Errorf("cleared = %d, want 0 — leaving A for B is a switch, not a clear", cleared)
	}
	if last == nil || last.WorktreePath != wtB || last.WorktreeBranch != "worktree-feature-b" {
		t.Fatalf("expected switch to B, got %+v", last)
	}
	path, branch, _ := a.currentWorktree(1)
	if path != wtB || branch != "worktree-feature-b" {
		t.Errorf("currentWorktree = %q/%q, want B", path, branch)
	}
}

func TestOnWorktreeChange_StillClearsWhenLeavingForMainRepo(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	a.emitWorktreeEvent = func(string, any) {}
	a.onWorktreeChange(1, wt, "worktree-feature-a", wt)

	var cleared *WorktreeClearedEvent
	a.emitWorktreeEvent = func(name string, payload any) {
		if ev, ok := payload.(WorktreeClearedEvent); ok {
			cleared = &ev
		}
	}
	a.onWorktreeChange(1, "", "", repo)

	if cleared == nil || cleared.ID != 1 {
		t.Fatalf("expected WorktreeClearedEvent, got %+v", cleared)
	}
	if _, _, ok := a.currentWorktree(1); ok {
		t.Error("currentWorktree still reports a worktree after clear")
	}
}

// The hook directory is polled every 100ms and every event reaches this path.
// Issue #192 was 85% of a core burned by unconditional per-tick filesystem work,
// so an unchanged cwd must not cost a git invocation.
func TestOnWorktreeChange_DoesNotProbeGitForUnchangedCwd(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	a.emitWorktreeEvent = func(string, any) {}
	probes := 0
	a.worktreeProbe = func(dir string) (string, string, bool) {
		probes++
		return defaultWorktreeProbe(dir)
	}

	for i := 0; i < 25; i++ {
		a.onWorktreeChange(1, "", "", wt)
	}

	if probes != 1 {
		t.Errorf("git probes = %d, want 1 for 25 events with an unchanged cwd", probes)
	}
}

// The overwhelmingly common pane has no worktree at all. A negative probe
// result must be remembered just as a positive one is, or every hook event for
// every ordinary pane pays for two git invocations.
func TestOnWorktreeChange_DoesNotReprobeMainRepoCwd(t *testing.T) {
	repo := initPaneTestRepo(t)

	a := newDetectTestApp()
	a.emitWorktreeEvent = func(string, any) {}
	probes := 0
	a.worktreeProbe = func(dir string) (string, string, bool) {
		probes++
		return defaultWorktreeProbe(dir)
	}

	for i := 0; i < 25; i++ {
		a.onWorktreeChange(1, "", "", repo)
	}

	if probes != 1 {
		t.Errorf("git probes = %d, want 1 for 25 events in an unchanged main checkout", probes)
	}
}

// A remembered negative must not blind the session to a later move into a
// worktree — the cache is keyed on the cwd, not a permanent verdict.
func TestOnWorktreeChange_ReprobesAfterCwdChanges(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	var emitted *WorktreeDetectedEvent
	a.emitWorktreeEvent = func(name string, payload any) {
		if ev, ok := payload.(WorktreeDetectedEvent); ok {
			emitted = &ev
		}
	}

	for i := 0; i < 5; i++ {
		a.onWorktreeChange(1, "", "", repo) // main checkout: negative, cached
	}
	a.onWorktreeChange(1, "", "", wt) // now in the worktree

	if emitted == nil || emitted.WorktreePath != wt {
		t.Fatalf("expected detection after moving into the worktree, got %+v", emitted)
	}
}
