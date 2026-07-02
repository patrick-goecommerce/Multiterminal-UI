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

func TestPaneWorktreeBase(t *testing.T) {
	got := paneWorktreeBase(filepath.Join("D:", "repos", "Foo"))
	want := filepath.Join("D:", "repos", "Foo.mt-worktrees")
	if got != want {
		t.Errorf("paneWorktreeBase = %q, want %q", got, want)
	}
}

func TestFindFreePaneName_NoCollision(t *testing.T) {
	repo := initPaneTestRepo(t)
	if got := findFreePaneName(repo, "My Feature!"); got != "my-feature" {
		t.Errorf("got %q, want my-feature", got)
	}
}

func TestFindFreePaneName_BranchCollisionIncrements(t *testing.T) {
	repo := initPaneTestRepo(t)
	gitRun(t, repo, "branch", "terminal/fix")
	if got := findFreePaneName(repo, "fix"); got != "fix-2" {
		t.Errorf("got %q, want fix-2", got)
	}
}

func TestFindFreePaneName_DirCollisionIncrements(t *testing.T) {
	repo := initPaneTestRepo(t)
	dir := filepath.Join(paneWorktreeBase(repo), "fix")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if got := findFreePaneName(repo, "fix"); got != "fix-2" {
		t.Errorf("got %q, want fix-2", got)
	}
}
