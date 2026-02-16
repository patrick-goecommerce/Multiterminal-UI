package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// slugifyTitle – pure function tests
// ---------------------------------------------------------------------------

func TestSlugifyTitle_Simple(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Fix login bug", "fix-login-bug"},
		{"Add user authentication", "add-user-authentication"},
		{"Hello World!", "hello-world"},
		{"", ""},
		{"---", ""},
		{"fix: resolve #42 crash on startup", "fix-resolve-42-crash-on-startup"},
	}
	for _, tc := range cases {
		got := slugifyTitle(tc.input)
		if got != tc.want {
			t.Errorf("slugifyTitle(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSlugifyTitle_Unicode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Ärger mit Umlauten", "arger-mit-umlauten"},
		{"café résumé", "cafe-resume"},
		{"naïve façade", "naive-facade"},
	}
	for _, tc := range cases {
		got := slugifyTitle(tc.input)
		if got != tc.want {
			t.Errorf("slugifyTitle(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSlugifyTitle_Truncation(t *testing.T) {
	long := "this is a very long title that should be truncated at forty characters exactly"
	got := slugifyTitle(long)
	if len(got) > 40 {
		t.Errorf("expected max 40 chars, got %d: %q", len(got), got)
	}
	// Should not end with a hyphen
	if got[len(got)-1] == '-' {
		t.Errorf("slug should not end with hyphen: %q", got)
	}
}

func TestSlugifyTitle_SpecialChars(t *testing.T) {
	got := slugifyTitle("feat: [WIP] add @mentions & #tags")
	if got == "" {
		t.Fatal("expected non-empty slug")
	}
	// Should only contain a-z, 0-9, and hyphens
	for _, r := range got {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			t.Errorf("unexpected character %q in slug %q", string(r), got)
		}
	}
}

// ---------------------------------------------------------------------------
// issueBranchName – pure function tests
// ---------------------------------------------------------------------------

func TestIssueBranchName_Normal(t *testing.T) {
	got := issueBranchName(42, "Fix login bug")
	want := "issue/42-fix-login-bug"
	if got != want {
		t.Errorf("issueBranchName(42, ...) = %q, want %q", got, want)
	}
}

func TestIssueBranchName_EmptyTitle(t *testing.T) {
	got := issueBranchName(7, "")
	want := "issue/7"
	if got != want {
		t.Errorf("issueBranchName(7, \"\") = %q, want %q", got, want)
	}
}

func TestIssueBranchName_SpecialTitle(t *testing.T) {
	got := issueBranchName(100, "feat: @user öffne das Fenster!")
	if got == "" {
		t.Fatal("expected non-empty branch name")
	}
	if got[:8] != "issue/10" {
		t.Errorf("expected branch to start with 'issue/100', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// isGitRepo, hasCleanWorkingTree, branchExists – integration tests
// ---------------------------------------------------------------------------

func TestIsGitRepo_True(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	if !isGitRepo(dir) {
		t.Fatal("expected true for git repo")
	}
}

func TestIsGitRepo_False(t *testing.T) {
	dir := t.TempDir()
	if isGitRepo(dir) {
		t.Fatal("expected false for non-git dir")
	}
}

func TestHasCleanWorkingTree_Clean(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	if !hasCleanWorkingTree(dir) {
		t.Fatal("expected clean working tree")
	}
}

func TestHasCleanWorkingTree_Dirty(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)

	if hasCleanWorkingTree(dir) {
		t.Fatal("expected dirty working tree")
	}
}

func TestBranchExists_True(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	if !branchExists(dir, "master") && !branchExists(dir, "main") {
		t.Fatal("expected default branch to exist")
	}
}

func TestBranchExists_False(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	if branchExists(dir, "nonexistent-branch") {
		t.Fatal("expected false for nonexistent branch")
	}
}

// ---------------------------------------------------------------------------
// GetOrCreateIssueBranch – integration tests
// ---------------------------------------------------------------------------

func TestGetOrCreateIssueBranch_CreatesNew(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	a := newTestApp()
	branch, err := a.GetOrCreateIssueBranch(dir, 42, "Fix login bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "issue/42-fix-login-bug" {
		t.Fatalf("expected 'issue/42-fix-login-bug', got %q", branch)
	}
	if !branchExists(dir, branch) {
		t.Fatal("expected branch to be created")
	}
}

func TestGetOrCreateIssueBranch_SwitchesExisting(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")

	// Create branch first
	gitRun(t, dir, "checkout", "-b", "issue/10-test")
	gitRun(t, dir, "checkout", "-") // switch back

	a := newTestApp()
	branch, err := a.GetOrCreateIssueBranch(dir, 10, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "issue/10-test" {
		t.Fatalf("expected 'issue/10-test', got %q", branch)
	}
}

func TestGetOrCreateIssueBranch_DirtyTree(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	gitCommitFile(t, dir, "file.txt", "content", "initial")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)

	a := newTestApp()
	_, err := a.GetOrCreateIssueBranch(dir, 5, "some issue")
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
}

func TestGetOrCreateIssueBranch_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp()
	_, err := a.GetOrCreateIssueBranch(dir, 1, "test")
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func gitInit(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "init")
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), gitTestEnv()...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func gitCommitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "--no-gpg-sign", "-m", msg)
}
