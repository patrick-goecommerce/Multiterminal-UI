package backend

import (
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// worktreePath – pure function tests
// ---------------------------------------------------------------------------

func TestWorktreePath(t *testing.T) {
	cases := []struct {
		repoDir string
		issue   int
		want    string
	}{
		{"/home/user/project", 42, "/home/user/project/.mt-worktrees/issue-42"},
		{"/tmp/repo", 1, "/tmp/repo/.mt-worktrees/issue-1"},
		{"/repo", 100, "/repo/.mt-worktrees/issue-100"},
	}
	for _, tc := range cases {
		got := worktreePath(tc.repoDir, tc.issue)
		if got != tc.want {
			t.Errorf("worktreePath(%q, %d) = %q, want %q", tc.repoDir, tc.issue, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// parseWorktreeList – pure function tests
// ---------------------------------------------------------------------------

func TestParseWorktreeList_Empty(t *testing.T) {
	result := parseWorktreeList("", "/repo")
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d entries", len(result))
	}
}

func TestParseWorktreeList_NoMultiterminalWorktrees(t *testing.T) {
	output := `worktree /repo
HEAD abc123
branch refs/heads/main

worktree /other/worktree
HEAD def456
branch refs/heads/feature

`
	result := parseWorktreeList(output, "/repo")
	if len(result) != 0 {
		t.Fatalf("expected no MT worktrees, got %d", len(result))
	}
}

func TestParseWorktreeList_WithMultiterminalWorktrees(t *testing.T) {
	root := "/home/user/project"
	mtWt := filepath.Join(root, ".mt-worktrees", "issue-42")
	output := "worktree " + root + "\nHEAD abc123\nbranch refs/heads/main\n\nworktree " + mtWt + "\nHEAD def456\nbranch refs/heads/issue/42-fix-bug\n\n"

	result := parseWorktreeList(output, root)
	if len(result) != 1 {
		t.Fatalf("expected 1 MT worktree, got %d", len(result))
	}
	if result[0].Path != mtWt {
		t.Errorf("expected path %q, got %q", mtWt, result[0].Path)
	}
	if result[0].Branch != "issue/42-fix-bug" {
		t.Errorf("expected branch 'issue/42-fix-bug', got %q", result[0].Branch)
	}
	if result[0].Issue != 42 {
		t.Errorf("expected issue 42, got %d", result[0].Issue)
	}
}

func TestParseWorktreeList_MultipleIssues(t *testing.T) {
	root := "/repo"
	wt1 := filepath.Join(root, ".mt-worktrees", "issue-1")
	wt2 := filepath.Join(root, ".mt-worktrees", "issue-99")
	output := "worktree " + root + "\nHEAD aaa\nbranch refs/heads/main\n\nworktree " + wt1 + "\nHEAD bbb\nbranch refs/heads/issue/1-first\n\nworktree " + wt2 + "\nHEAD ccc\nbranch refs/heads/issue/99-last\n\n"

	result := parseWorktreeList(output, root)
	if len(result) != 2 {
		t.Fatalf("expected 2 MT worktrees, got %d", len(result))
	}
	if result[0].Issue != 1 {
		t.Errorf("expected issue 1, got %d", result[0].Issue)
	}
	if result[1].Issue != 99 {
		t.Errorf("expected issue 99, got %d", result[1].Issue)
	}
}

func TestParseWorktreeList_LastEntryNoTrailingNewline(t *testing.T) {
	root := "/repo"
	wt := filepath.Join(root, ".mt-worktrees", "issue-5")
	output := "worktree " + wt + "\nHEAD abc\nbranch refs/heads/issue/5-test"

	result := parseWorktreeList(output, root)
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
	if result[0].Issue != 5 {
		t.Errorf("expected issue 5, got %d", result[0].Issue)
	}
}

// ---------------------------------------------------------------------------
// CreateWorktree / RemoveWorktree – integration tests
// ---------------------------------------------------------------------------

func TestCreateWorktree_Integration(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	a := newTestApp()
	wt, err := a.CreateWorktree(dir, 42, "Fix login bug")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if wt == nil {
		t.Fatal("expected non-nil WorktreeInfo")
	}
	if wt.Issue != 42 {
		t.Errorf("expected issue 42, got %d", wt.Issue)
	}
	if wt.Branch != "issue/42-fix-login-bug" {
		t.Errorf("expected branch 'issue/42-fix-login-bug', got %q", wt.Branch)
	}

	// Should be idempotent
	wt2, err := a.CreateWorktree(dir, 42, "Fix login bug")
	if err != nil {
		t.Fatalf("second CreateWorktree failed: %v", err)
	}
	if wt2.Path != wt.Path {
		t.Errorf("expected same path on second call")
	}
}

func TestRemoveWorktree_Integration(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	a := newTestApp()
	_, err := a.CreateWorktree(dir, 10, "test worktree")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	err = a.RemoveWorktree(dir, 10)
	if err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Removing again should be a no-op
	err = a.RemoveWorktree(dir, 10)
	if err != nil {
		t.Fatalf("second RemoveWorktree should be no-op, got: %v", err)
	}
}

func TestListWorktrees_Integration(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	a := newTestApp()
	// No worktrees initially
	list := a.ListWorktrees(dir)
	if len(list) != 0 {
		t.Fatalf("expected 0 worktrees initially, got %d", len(list))
	}

	// Create one
	_, err := a.CreateWorktree(dir, 7, "test")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	list = a.ListWorktrees(dir)
	if len(list) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(list))
	}
	if list[0].Issue != 7 {
		t.Errorf("expected issue 7, got %d", list[0].Issue)
	}
}

func TestRemoveWorktree_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp()
	err := a.RemoveWorktree(dir, 1)
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}
