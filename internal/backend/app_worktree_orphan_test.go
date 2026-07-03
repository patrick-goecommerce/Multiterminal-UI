package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveOrphanedWorktree_RemovesCleanMergedWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "done-feature")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-done-feature", wt)
	// Merge it into main so branch -d succeeds (mirrors "already integrated" case).
	gitRun(t, repo, "merge", "--ff-only", "worktree-done-feature")

	a := &AppService{}
	if err := a.RemoveOrphanedWorktree(wt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree directory still exists")
	}
	if branchExists(repo, "worktree-done-feature") {
		t.Error("branch still exists after removal of a merged worktree")
	}
}

func TestRemoveOrphanedWorktree_RefusesUnmergedBranch(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "wip-feature")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-wip-feature", wt)
	if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "work.txt")
	gitRun(t, wt, "commit", "-m", "unmerged work")

	a := &AppService{}
	// Directory removal succeeds (no uncommitted files), but the branch delete
	// (-d, never -D) must refuse since it is not merged anywhere — the commit
	// must not silently disappear.
	err := a.RemoveOrphanedWorktree(wt)
	if err == nil {
		t.Fatal("expected error: unmerged branch must not be force-deleted")
	}
	if !branchExists(repo, "worktree-wip-feature") {
		t.Fatal("DATA LOSS: unmerged branch was deleted")
	}
}
