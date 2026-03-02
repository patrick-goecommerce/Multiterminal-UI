package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// classifyGitStatus – pure function tests
// ---------------------------------------------------------------------------

func TestClassifyGitStatus_Untracked(t *testing.T) {
	if got := classifyGitStatus("??"); got != "?" {
		t.Fatalf("expected '?', got %q", got)
	}
}

func TestClassifyGitStatus_Modified(t *testing.T) {
	cases := []string{"M ", " M", "MM"}
	for _, xy := range cases {
		if got := classifyGitStatus(xy); got != "M" {
			t.Fatalf("classifyGitStatus(%q) = %q, want 'M'", xy, got)
		}
	}
}

func TestClassifyGitStatus_Added(t *testing.T) {
	cases := []string{"A ", " A", "AM"}
	for _, xy := range cases {
		if got := classifyGitStatus(xy); got != "A" {
			t.Fatalf("classifyGitStatus(%q) = %q, want 'A'", xy, got)
		}
	}
}

func TestClassifyGitStatus_Deleted(t *testing.T) {
	cases := []string{"D ", " D"}
	for _, xy := range cases {
		if got := classifyGitStatus(xy); got != "D" {
			t.Fatalf("classifyGitStatus(%q) = %q, want 'D'", xy, got)
		}
	}
}

func TestClassifyGitStatus_Renamed(t *testing.T) {
	if got := classifyGitStatus("R "); got != "R" {
		t.Fatalf("expected 'R', got %q", got)
	}
}

func TestClassifyGitStatus_Unchanged(t *testing.T) {
	if got := classifyGitStatus("  "); got != "" {
		t.Fatalf("expected empty string for unchanged, got %q", got)
	}
}

// Test priority: '?' before 'A' before 'D' before 'R' before 'M'
func TestClassifyGitStatus_PriorityQuestionOverAdded(t *testing.T) {
	// '?A' → should be '?' (? takes priority)
	if got := classifyGitStatus("?A"); got != "?" {
		t.Fatalf("expected '?' (priority), got %q", got)
	}
}

// ---------------------------------------------------------------------------
// markParentDirs – pure function tests
// ---------------------------------------------------------------------------

func TestMarkParentDirs_MarksAll(t *testing.T) {
	statuses := make(map[string]string)
	root := filepath.FromSlash("/projects/myapp")
	filePath := filepath.FromSlash("/projects/myapp/src/components/Button.tsx")

	markParentDirs(statuses, filePath, root)

	expected := []string{
		filepath.FromSlash("/projects/myapp/src/components"),
		filepath.FromSlash("/projects/myapp/src"),
	}
	for _, dir := range expected {
		if statuses[dir] != "M" {
			t.Errorf("expected %q to be marked as 'M', got %q", dir, statuses[dir])
		}
	}
}

func TestMarkParentDirs_DoesNotOverwrite(t *testing.T) {
	srcDir := filepath.FromSlash("/projects/myapp/src")
	statuses := map[string]string{
		srcDir: "A",
	}
	root := filepath.FromSlash("/projects/myapp")
	filePath := filepath.FromSlash("/projects/myapp/src/index.ts")

	markParentDirs(statuses, filePath, root)

	// Existing "A" status should not be overwritten
	if statuses[srcDir] != "A" {
		t.Fatalf("existing status should not be overwritten, got %q", statuses[srcDir])
	}
}

func TestMarkParentDirs_FileDirectlyInRoot(t *testing.T) {
	statuses := make(map[string]string)
	root := filepath.FromSlash("/projects/myapp")
	filePath := filepath.FromSlash("/projects/myapp/README.md")

	markParentDirs(statuses, filePath, root)

	// The parent of README.md is the root itself — nothing to mark
	if len(statuses) != 0 {
		t.Fatalf("expected no parent dirs marked, got %d entries", len(statuses))
	}
}

// ---------------------------------------------------------------------------
// GetGitBranch, GetLastCommitTime – integration tests (require git)
// ---------------------------------------------------------------------------

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func gitTestEnv() []string {
	return []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_NOSYSTEM=1", "HOME=/tmp",
	}
}

func TestGetGitBranch_ValidRepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("checkout", "-b", "test-branch")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "initial")

	a := newTestApp()
	branch := a.GetGitBranch(dir)
	if branch != "test-branch" {
		t.Fatalf("expected 'test-branch', got %q", branch)
	}
}

func TestGetGitBranch_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp()
	branch := a.GetGitBranch(dir)
	if branch != "" {
		t.Fatalf("expected empty branch for non-git dir, got %q", branch)
	}
}

func TestGetLastCommitTime_ValidRepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "initial")

	a := newTestApp()
	ts := a.GetLastCommitTime(dir)
	if ts <= 0 {
		t.Fatalf("expected positive timestamp, got %d", ts)
	}
}

func TestGetLastCommitTime_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp()
	ts := a.GetLastCommitTime(dir)
	if ts != 0 {
		t.Fatalf("expected 0 for non-git dir, got %d", ts)
	}
}

func TestGetGitFileStatuses_ValidRepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	// Resolve short names (e.g. PATRIC~1) to canonical long paths on Windows
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "committed.txt"), []byte("committed"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "initial")

	// Create untracked file
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new"), 0644)

	// Modify committed file
	os.WriteFile(filepath.Join(dir, "committed.txt"), []byte("modified"), 0644)

	a := newTestApp()
	statuses := a.GetGitFileStatuses(dir)

	untrackedPath := filepath.Join(dir, "untracked.txt")
	if statuses[untrackedPath] != "?" {
		t.Errorf("expected untracked.txt status '?', got %q", statuses[untrackedPath])
	}

	committedPath := filepath.Join(dir, "committed.txt")
	if statuses[committedPath] != "M" {
		t.Errorf("expected committed.txt status 'M', got %q", statuses[committedPath])
	}
}

func TestGetGitFileStatuses_EmptyDir(t *testing.T) {
	a := newTestApp()
	statuses := a.GetGitFileStatuses("")
	if len(statuses) != 0 {
		t.Fatalf("expected empty map for empty dir, got %d entries", len(statuses))
	}
}

// ---------------------------------------------------------------------------
// AddToGitignore – integration tests
// ---------------------------------------------------------------------------

func setupGitRepo(t *testing.T) (dir string, run func(...string)) {
	t.Helper()
	dir = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	run = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "initial")
	return
}

func TestAddToGitignore_CreatesFile(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	err := a.AddToGitignore(dir, "secret.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitignorePath := filepath.Join(dir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
	if !strings.Contains(string(content), "secret.env") {
		t.Fatalf(".gitignore does not contain 'secret.env', got: %q", string(content))
	}
}

func TestAddToGitignore_AppendsToExisting(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644)

	err := a.AddToGitignore(dir, "dist/output.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(content), "node_modules/") {
		t.Fatalf("existing entry was removed")
	}
	if !strings.Contains(string(content), "dist/output.js") {
		t.Fatalf("new entry not appended")
	}
}

func TestAddToGitignore_NoDuplicates(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("secret.env\n"), 0644)

	err := a.AddToGitignore(dir, "secret.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	count := strings.Count(string(content), "secret.env")
	if count != 1 {
		t.Fatalf("expected exactly 1 occurrence of 'secret.env', got %d", count)
	}
}

func TestAddToGitignore_NormalizesBackslashes(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	err := a.AddToGitignore(dir, `config\local.yaml`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitignorePath := filepath.Join(dir, ".gitignore")
	content, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(content), "config/local.yaml") {
		t.Fatalf("expected forward slashes in .gitignore, got: %q", string(content))
	}
}

func TestAddToGitignore_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp()

	err := a.AddToGitignore(dir, "secret.env")
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
}

func TestAddToGitignore_ExistingNoTrailingNewline(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}
	dir, _ := setupGitRepo(t)
	a := newTestApp()

	gitignorePath := filepath.Join(dir, ".gitignore")
	// Write existing content WITHOUT trailing newline
	os.WriteFile(gitignorePath, []byte("node_modules/"), 0644)

	err := a.AddToGitignore(dir, "dist/output.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(gitignorePath)
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	// Should have two entries on separate lines
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(content))
	}
	if lines[0] != "node_modules/" {
		t.Fatalf("first line should be 'node_modules/', got %q", lines[0])
	}
	if lines[1] != "dist/output.js" {
		t.Fatalf("second line should be 'dist/output.js', got %q", lines[1])
	}
}
