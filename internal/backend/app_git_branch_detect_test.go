package backend

import "testing"

func TestIsOnIssueBranch_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp()
	info := a.IsOnIssueBranch(dir, 42)
	if info.OnIssueBranch {
		t.Fatal("expected on_issue_branch=false for non-git dir")
	}
}

func TestIsOnIssueBranch_MainBranch(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "f.txt", "hello", "init")

	a := newTestApp()
	info := a.IsOnIssueBranch(dir, 42)
	if info.OnIssueBranch {
		t.Fatal("expected on_issue_branch=false for default branch")
	}
}

func TestIsOnIssueBranch_DifferentIssue(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "f.txt", "hello", "init")
	gitRun(t, dir, "checkout", "-b", "issue/26-context-menu")

	a := newTestApp()
	info := a.IsOnIssueBranch(dir, 42)
	if !info.OnIssueBranch {
		t.Fatal("expected on_issue_branch=true")
	}
	if info.IssueNumber != 26 {
		t.Fatalf("expected issue_number=26, got %d", info.IssueNumber)
	}
	if info.IsSameIssue {
		t.Fatal("expected is_same_issue=false")
	}
	if info.BranchName != "issue/26-context-menu" {
		t.Fatalf("expected branch 'issue/26-context-menu', got %q", info.BranchName)
	}
}

func TestIsOnIssueBranch_SameIssue(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "f.txt", "hello", "init")
	gitRun(t, dir, "checkout", "-b", "issue/42-fix-login")

	a := newTestApp()
	info := a.IsOnIssueBranch(dir, 42)
	if !info.OnIssueBranch {
		t.Fatal("expected on_issue_branch=true")
	}
	if !info.IsSameIssue {
		t.Fatal("expected is_same_issue=true")
	}
	if info.IssueNumber != 42 {
		t.Fatalf("expected issue_number=42, got %d", info.IssueNumber)
	}
}

func TestIsOnIssueBranch_FeatureBranch(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "f.txt", "hello", "init")
	gitRun(t, dir, "checkout", "-b", "feature/some-work")

	a := newTestApp()
	info := a.IsOnIssueBranch(dir, 42)
	if info.OnIssueBranch {
		t.Fatal("expected on_issue_branch=false for feature/ branch")
	}
}
