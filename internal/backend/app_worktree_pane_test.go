package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initPaneTestRepo creates a real git repo on branch alpha-main with one commit.
// Returned path is EvalSymlinks-resolved (t.TempDir may be a symlink on Windows/macOS).
func initPaneTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	gitRun(t, dir, "init", "-b", "alpha-main")
	gitRun(t, dir, "config", "user.email", "test@test.local")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "init")
	return dir
}

func TestMainRepoRoot_FromMainRepo(t *testing.T) {
	repo := initPaneTestRepo(t)
	got, err := mainRepoRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got, repo) {
		t.Errorf("mainRepoRoot = %q, want %q", got, repo)
	}
}

func TestMainRepoRoot_FromInsideWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+"-wt")
	gitRun(t, repo, "worktree", "add", "-b", "terminal/x", wt, "alpha-main")
	got, err := mainRepoRoot(wt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got, repo) {
		t.Errorf("mainRepoRoot from worktree = %q, want main repo %q", got, repo)
	}
}

func TestMainRepoRoot_NotARepo(t *testing.T) {
	if _, err := mainRepoRoot(t.TempDir()); err == nil {
		t.Error("expected error for non-repo dir")
	}
}
