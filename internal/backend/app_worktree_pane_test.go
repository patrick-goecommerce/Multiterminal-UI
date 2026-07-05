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
	want := filepath.Join("D:", "repos", "Foo", ".claude", "worktrees")
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

func TestCreatePaneWorktree_HappyPath(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	wt, err := a.CreatePaneWorktree(repo, "login fix", "alpha-main")
	if err != nil {
		t.Fatal(err)
	}
	if wt.Branch != "terminal/login-fix" || wt.TargetBranch != "alpha-main" {
		t.Errorf("unexpected info: %+v", wt)
	}
	if !strings.HasPrefix(strings.ToLower(wt.Path), strings.ToLower(paneWorktreeBase(repo)+string(filepath.Separator))) {
		t.Errorf("worktree not in sibling base: %q", wt.Path)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "CLAUDE.local.md")); err != nil {
		t.Error("CLAUDE.local.md missing")
	}
	if _, err := os.Stat(filepath.Join(wt.Path, ".claude", "settings.local.json")); err != nil {
		t.Error("settings.local.json missing")
	}
}

func TestCreatePaneWorktree_MissingTargetBranch(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	if _, err := a.CreatePaneWorktree(repo, "x", "nope"); err == nil {
		t.Error("expected error for missing target branch")
	}
	if _, err := a.CreatePaneWorktree(repo, "x", ""); err == nil {
		t.Error("expected error for empty target branch")
	}
}

func TestCreatePaneWorktree_ExistingBranchIsHardError(t *testing.T) {
	repo := initPaneTestRepo(t)
	gitRun(t, repo, "branch", "terminal/x")
	a := &AppService{}
	if _, err := a.CreatePaneWorktree(repo, "x", "alpha-main"); err == nil {
		t.Error("expected error for manually chosen colliding name")
	}
}

func TestGetPaneWorktreeDefaults(t *testing.T) {
	repo := initPaneTestRepo(t)
	gitRun(t, repo, "branch", "terminal/fix")
	a := &AppService{}
	d := a.GetPaneWorktreeDefaults(repo, "fix")
	if d.Name != "fix-2" || d.TargetBranch != "alpha-main" {
		t.Errorf("defaults = %+v, want fix-2/alpha-main", d)
	}
}

func TestCreateIssueWorktree_HappyPath(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	wt, err := a.CreateIssueWorktree(repo, 123, "Dashboard Filter!")
	if err != nil {
		t.Fatal(err)
	}
	if wt.Branch != "issue/123-dashboard-filter" || wt.TargetBranch != "alpha-main" {
		t.Errorf("unexpected info: %+v", wt)
	}
	wantPath := filepath.Join(paneWorktreeBase(repo), "issue-123-dashboard-filter")
	if !strings.EqualFold(wt.Path, wantPath) {
		t.Errorf("path = %q, want %q", wt.Path, wantPath)
	}
	// The main repo's own checked-out branch must never change.
	if got := checkedOutBranch(repo); got != "alpha-main" {
		t.Errorf("main repo branch changed to %q", got)
	}
}

func TestCreateIssueWorktree_ReattachesSameIssue(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	first, err := a.CreateIssueWorktree(repo, 42, "Fix login")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.CreateIssueWorktree(repo, 42, "Fix login")
	if err != nil {
		t.Fatal(err)
	}
	if second.Path != first.Path || second.Branch != first.Branch {
		t.Errorf("re-attach mismatch: %+v vs %+v", first, second)
	}
}

func TestCreateIssueWorktree_DetachedHeadRejected(t *testing.T) {
	repo := initPaneTestRepo(t)
	gitRun(t, repo, "checkout", "--detach", "HEAD")
	a := &AppService{}
	if _, err := a.CreateIssueWorktree(repo, 1, "x"); err == nil {
		t.Error("expected error on detached HEAD")
	}
}
