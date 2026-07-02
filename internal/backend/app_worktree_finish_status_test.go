package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// finishFixture: repo + pane worktree with one committed change on the branch.
func finishFixture(t *testing.T) (repo, wt string) {
	t.Helper()
	repo = initPaneTestRepo(t)
	a := &AppService{}
	info, err := a.CreatePaneWorktree(repo, "feat", "alpha-main")
	if err != nil {
		t.Fatal(err)
	}
	wt = info.Path
	if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "work.txt")
	gitRun(t, wt, "commit", "-m", "feat: work")
	return repo, wt
}

func TestFinishStatus_Ready(t *testing.T) {
	_, wt := finishFixture(t)
	a := &AppService{}
	s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main")
	if s.State != "ready" {
		t.Fatalf("state = %s (%s), want ready", s.State, s.Reason)
	}
	if len(s.Commits) != 1 || s.Stat == "" {
		t.Errorf("commits/stat not populated: %+v", s)
	}
}

func TestFinishStatus_CleanupOnly_NoCommits(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	info, _ := a.CreatePaneWorktree(repo, "empty", "alpha-main")
	s := a.GetWorktreeFinishStatus(info.Path, "terminal/empty", "alpha-main")
	if s.State != "cleanup_only" {
		t.Errorf("state = %s, want cleanup_only (0 commits ⇒ nichts zu mergen, deckt auch Crash-nach-Merge ab)", s.State)
	}
}

func TestFinishStatus_Blocked_NotRebased(t *testing.T) {
	repo, wt := finishFixture(t)
	// Move target forward so branch is no longer rebased onto it.
	if err := os.WriteFile(filepath.Join(repo, "main.txt"), []byte("y\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "main.txt")
	gitRun(t, repo, "commit", "-m", "main moves")
	a := &AppService{}
	s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main")
	if s.State != "blocked" {
		t.Errorf("state = %s, want blocked (not rebased)", s.State)
	}
}

func TestFinishStatus_Blocked_DirtyTracked_ButUntrackedOK(t *testing.T) {
	_, wt := finishFixture(t)
	a := &AppService{}
	// Untracked file must NOT block (spec 5.3/3), only list:
	if err := os.WriteFile(filepath.Join(wt, "scratch.log"), []byte("tmp"), 0644); err != nil {
		t.Fatal(err)
	}
	s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main")
	if s.State != "ready" || len(s.Untracked) != 1 {
		t.Fatalf("untracked handling wrong: %+v", s)
	}
	// Modified tracked file MUST block:
	if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main"); s.State != "blocked" {
		t.Errorf("dirty tracked file not blocking: %+v", s)
	}
}

func TestFinishStatus_Blocked_CleanupOnlyButDirtyTracked(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	info, err := a.CreatePaneWorktree(repo, "dirtyempty", "alpha-main")
	if err != nil {
		t.Fatal(err)
	}
	// 0 commits on the branch, but a TRACKED file is modified:
	if err := os.WriteFile(filepath.Join(info.Path, "README.md"), []byte("uncommitted\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s := a.GetWorktreeFinishStatus(info.Path, "terminal/dirtyempty", "alpha-main")
	if s.State != "blocked" {
		t.Fatalf("state = %s, want blocked (tracked changes must never be force-removed)", s.State)
	}
}

func TestFinishStatus_Blocked_MainDirtyOrWrongBranch(t *testing.T) {
	repo, wt := finishFixture(t)
	a := &AppService{}
	// Main worktree dirty (tracked change) blocks:
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main"); s.State != "blocked" {
		t.Error("dirty main worktree not blocking")
	}
	gitRun(t, repo, "checkout", "--", "README.md")
	// Wrong branch checked out in main blocks:
	gitRun(t, repo, "checkout", "-b", "other")
	if s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main"); s.State != "blocked" {
		t.Error("wrong main branch not blocking")
	}
}
