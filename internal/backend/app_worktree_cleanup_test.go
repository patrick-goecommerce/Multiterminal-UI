package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMergeWorktreeBranch_FFOnly(t *testing.T) {
	repo, _ := finishFixture(t) // branch terminal/feat mit 1 Commit, rebased
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	// Ziel-Branch muss den Commit jetzt enthalten:
	if out := gitRunOut(t, repo, "log", "--oneline", "-1"); !strings.Contains(out, "feat: work") {
		t.Errorf("target branch head = %q, want feat commit", out)
	}
	// Arbeitskopie im Haupt-Worktree wurde mitbewegt (ff):
	if _, err := os.Stat(filepath.Join(repo, "work.txt")); err != nil {
		t.Error("ff merge did not update main working tree")
	}
}

func TestMergeWorktreeBranch_RefusesNonFF(t *testing.T) {
	repo, _ := finishFixture(t)
	// Ziel bewegt sich → nicht mehr ff:
	if err := os.WriteFile(filepath.Join(repo, "clash.txt"), []byte("z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "clash.txt")
	gitRun(t, repo, "commit", "-m", "target moves")
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err == nil {
		t.Fatal("expected non-ff merge to be refused")
	}
}

func TestCleanupWorktree_RemovesWorktreeAndBranch(t *testing.T) {
	repo, wt := finishFixture(t)
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	if err := cleanupWorktree(repo, wt, "terminal/feat"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree dir still exists")
	}
	if branchExists(repo, "terminal/feat") {
		t.Error("branch still exists")
	}
}

func TestCleanupWorktree_UnmergedBranchSurvives(t *testing.T) {
	repo, wt := finishFixture(t)
	// KEIN Merge — branch -d muss verweigern, kein -D-Fallback (Spec 5.4/5):
	err := cleanupWorktree(repo, wt, "terminal/feat")
	if err == nil {
		t.Fatal("expected error: unmerged branch must not be deleted")
	}
	if !branchExists(repo, "terminal/feat") {
		t.Fatal("DATA LOSS: unmerged branch was deleted")
	}
}

// TestFinishWorktree_BlockedRetryAfterMergeCleansUp covers the cleanup-retry
// path: the merge already went through (count==0) and a marker exists, so a
// FinishWorktree call from the "blocked" phase must resume straight into the
// cleanup and tear everything down.
func TestFinishWorktree_BlockedRetryAfterMergeCleansUp(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	repo, wt := finishFixture(t)
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	if err := saveFinishMarker(finishMarkerPath(), wt, finishMarker{
		Phase: "merged", Branch: "terminal/feat", TargetBranch: "alpha-main",
	}); err != nil {
		t.Fatal(err)
	}
	a := newTestApp()
	a.finishStates[1] = &finishState{
		Phase: "blocked", WorktreePath: wt, Branch: "terminal/feat",
		TargetBranch: "alpha-main", Mode: "shell",
	}
	a.FinishWorktree(1)

	deadline := time.After(5 * time.Second)
	for {
		if a.getFinishState(1) == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("cleanup never completed: %+v", a.getFinishState(1))
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree dir still exists after retry cleanup")
	}
	if branchExists(repo, "terminal/feat") {
		t.Error("branch still exists after retry cleanup")
	}
	if _, ok := loadFinishMarkers(finishMarkerPath())[wt]; ok {
		t.Error("finish marker not deleted after cleanup")
	}
}

// TestFinishWorktree_CleanupPhaseRetryCleansUp proves the in-session recovery
// path for a cleanup failure that happened AFTER the merge went through: the
// flow is parked in phase "cleanup" (not "blocked") with the marker still in
// place, and the retry routes back through FinishWorktree (not Start). The
// call must be accepted, skip the re-merge (count==0) and finish the cleanup.
func TestFinishWorktree_CleanupPhaseRetryCleansUp(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	repo, wt := finishFixture(t)
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	if err := saveFinishMarker(finishMarkerPath(), wt, finishMarker{
		Phase: "merged", Branch: "terminal/feat", TargetBranch: "alpha-main",
	}); err != nil {
		t.Fatal(err)
	}
	a := newTestApp()
	// Parked in "cleanup" — the state the backend leaves behind when the merge
	// succeeded but cleanupWorktree failed (e.g. a held handle on Windows).
	a.finishStates[1] = &finishState{
		Phase: "cleanup", WorktreePath: wt, Branch: "terminal/feat",
		TargetBranch: "alpha-main", Mode: "shell",
	}
	a.FinishWorktree(1)

	deadline := time.After(5 * time.Second)
	for {
		if a.getFinishState(1) == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("cleanup never completed: %+v", a.getFinishState(1))
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree dir still exists after cleanup-phase retry")
	}
	if branchExists(repo, "terminal/feat") {
		t.Error("branch still exists after cleanup-phase retry")
	}
	if _, ok := loadFinishMarkers(finishMarkerPath())[wt]; ok {
		t.Error("finish marker not deleted after cleanup-phase retry")
	}
}

// TestReconcileFinishMarkers_ResumesCleanup covers startup recovery: a marker
// points at a merged, still-existing worktree ⇒ reconcile removes the
// worktree, deletes the branch and drops the marker (no re-merge).
func TestReconcileFinishMarkers_ResumesCleanup(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	repo, wt := finishFixture(t)
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	if err := saveFinishMarker(finishMarkerPath(), wt, finishMarker{
		Phase: "merged", Branch: "terminal/feat", TargetBranch: "alpha-main",
	}); err != nil {
		t.Fatal(err)
	}
	a := newTestApp()
	a.ReconcileFinishMarkers(repo)

	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree dir still exists after reconcile")
	}
	if branchExists(repo, "terminal/feat") {
		t.Error("branch still exists after reconcile")
	}
	if _, ok := loadFinishMarkers(finishMarkerPath())[wt]; ok {
		t.Error("marker not removed after reconcile")
	}
}
