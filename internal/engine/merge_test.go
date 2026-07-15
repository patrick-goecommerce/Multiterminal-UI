package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupMergeRepo creates a repo with a card branch and step branches.
// Returns the repo dir and card branch name.
func setupMergeRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "t@t.com")
	gitRun(t, dir, "config", "user.name", "test")

	// Initial commit on default branch.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "init")
	gitRun(t, dir, "branch", "-M", "main")

	// Create card branch from main.
	gitRun(t, dir, "checkout", "-b", "card-1")
	gitRun(t, dir, "checkout", "main")
	return dir, "card-1"
}

// createStepBranch creates a branch from cardBranch with a file change.
func createStepBranch(t *testing.T, dir, cardBranch, stepBranch, file, content string) {
	t.Helper()
	gitRun(t, dir, "checkout", cardBranch)
	gitRun(t, dir, "checkout", "-b", stepBranch)
	os.WriteFile(filepath.Join(dir, file), []byte(content), 0644)
	gitRun(t, dir, "add", file)
	gitRun(t, dir, "commit", "-m", "step: "+stepBranch)
	gitRun(t, dir, "checkout", "main")
}

func TestMergeWave_CleanMerge(t *testing.T) {
	dir, cardBranch := setupMergeRepo(t)
	createStepBranch(t, dir, cardBranch, "step-a", "a.go", "package a\n")
	createStepBranch(t, dir, cardBranch, "step-b", "b.go", "package b\n")

	result := MergeWave(context.Background(), dir, cardBranch,
		[]string{"step-a", "step-b"})

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Reason)
	}
	if len(result.MergedBranches) != 2 {
		t.Fatalf("expected 2 merged branches, got %d", len(result.MergedBranches))
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %d", len(result.Conflicts))
	}
}

func TestMergeWave_ConflictDetected(t *testing.T) {
	dir, cardBranch := setupMergeRepo(t)

	// Both branches modify the same file on the same line.
	createStepBranch(t, dir, cardBranch, "step-a",
		"shared.go", "package shared\nvar X = 1\n")
	createStepBranch(t, dir, cardBranch, "step-b",
		"shared.go", "package shared\nvar X = 2\n")

	result := MergeWave(context.Background(), dir, cardBranch,
		[]string{"step-a", "step-b"})

	if result.Success {
		t.Fatal("expected failure due to conflict")
	}
	if !result.NeedsHumanReview {
		t.Fatal("expected NeedsHumanReview")
	}
	if len(result.Conflicts) == 0 {
		t.Fatal("expected at least one conflict")
	}
}

func TestMergeWave_ConflictAbortedCleanly(t *testing.T) {
	dir, cardBranch := setupMergeRepo(t)
	createStepBranch(t, dir, cardBranch, "step-a",
		"shared.go", "package shared\nvar X = 1\n")
	createStepBranch(t, dir, cardBranch, "step-b",
		"shared.go", "package shared\nvar X = 2\n")

	_ = MergeWave(context.Background(), dir, cardBranch,
		[]string{"step-a", "step-b"})

	// After conflict + abort, repo should be in a clean state.
	gitRun(t, dir, "checkout", cardBranch)
	out := gitOutput(t, dir, "status", "--porcelain")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected clean repo after abort, got:\n%s", out)
	}
}

func TestCanAIMerge_CriticalFile(t *testing.T) {
	conflicts := []MergeConflict{
		{File: "go.mod", IsCritical: true, HunkCount: 1, LinesTotal: 5},
	}
	if canAIMerge(conflicts) {
		t.Fatal("expected canAIMerge=false for critical file")
	}
}

func TestCanAIMerge_SmallConflict(t *testing.T) {
	conflicts := []MergeConflict{
		{File: "util.go", IsCritical: false, HunkCount: 1, LinesTotal: 10},
	}
	if !canAIMerge(conflicts) {
		t.Fatal("expected canAIMerge=true for small non-critical conflict")
	}
}

func TestCanAIMerge_TooManyFiles(t *testing.T) {
	conflicts := []MergeConflict{
		{File: "a.go", HunkCount: 1, LinesTotal: 5},
		{File: "b.go", HunkCount: 1, LinesTotal: 5},
		{File: "c.go", HunkCount: 1, LinesTotal: 5},
		{File: "d.go", HunkCount: 1, LinesTotal: 5},
	}
	if canAIMerge(conflicts) {
		t.Fatal("expected canAIMerge=false for >3 files")
	}
}

func TestCanAIMerge_TooManyHunks(t *testing.T) {
	conflicts := []MergeConflict{
		{File: "a.go", HunkCount: 3, LinesTotal: 10},
	}
	if canAIMerge(conflicts) {
		t.Fatal("expected canAIMerge=false for >2 hunks per file")
	}
}

func TestCanAIMerge_TooManyLines(t *testing.T) {
	conflicts := []MergeConflict{
		{File: "a.go", HunkCount: 1, LinesTotal: 25},
	}
	if canAIMerge(conflicts) {
		t.Fatal("expected canAIMerge=false when lines exceed limit")
	}
}

func TestMergeWave_NoManifestFiles(t *testing.T) {
	dir, cardBranch := setupMergeRepo(t)
	createStepBranch(t, dir, cardBranch, "step-a", "a.go", "package a\n")

	result := MergeWave(context.Background(), dir, cardBranch,
		[]string{"step-a"})

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Reason)
	}
	if result.Reconciliation != nil {
		t.Fatal("expected no reconciliation when no manifest files exist")
	}
}

func TestMergeWave_SequentialOrder(t *testing.T) {
	dir, cardBranch := setupMergeRepo(t)
	createStepBranch(t, dir, cardBranch, "step-1", "first.go", "package first\n")
	createStepBranch(t, dir, cardBranch, "step-2", "second.go", "package second\n")

	result := MergeWave(context.Background(), dir, cardBranch,
		[]string{"step-1", "step-2"})

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Reason)
	}
	if len(result.MergedBranches) != 2 {
		t.Fatalf("expected 2 merged, got %d", len(result.MergedBranches))
	}
	if result.MergedBranches[0] != "step-1" {
		t.Fatalf("expected step-1 first, got %s", result.MergedBranches[0])
	}
	if result.MergedBranches[1] != "step-2" {
		t.Fatalf("expected step-2 second, got %s", result.MergedBranches[1])
	}

	// Verify both files exist on card branch.
	gitRun(t, dir, "checkout", cardBranch)
	for _, f := range []string{"first.go", "second.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("file %s missing after merge", f)
		}
	}
}

func TestMergeWave_FirstBranchMergedBeforeConflict(t *testing.T) {
	dir, cardBranch := setupMergeRepo(t)

	// step-1 is clean, step-2 conflicts with step-1.
	createStepBranch(t, dir, cardBranch, "step-1",
		"shared.go", "package shared\nvar X = 1\n")
	createStepBranch(t, dir, cardBranch, "step-2",
		"shared.go", "package shared\nvar X = 2\n")

	result := MergeWave(context.Background(), dir, cardBranch,
		[]string{"step-1", "step-2"})

	// step-1 should have merged before step-2 failed.
	if len(result.MergedBranches) != 1 {
		t.Fatalf("expected 1 merged branch, got %d", len(result.MergedBranches))
	}
	if result.MergedBranches[0] != "step-1" {
		t.Fatalf("expected step-1 merged, got %s", result.MergedBranches[0])
	}
}

// gitOutput runs a git command and returns stdout.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"git"}, args...)
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}
