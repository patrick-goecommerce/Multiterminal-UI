package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initTestRepo creates a real git repo with one commit. Returned path is
// EvalSymlinks-resolved (t.TempDir may be a symlink on Windows/macOS).
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	gitRun(t, dir, "init", "-b", "main")
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

func TestIsUnderDir(t *testing.T) {
	cases := []struct {
		path, dir string
		want      bool
	}{
		{`D:\repo\.claude\worktrees\a\file.go`, `D:\repo\.claude\worktrees\a`, true},
		{`D:\repo\.claude\worktrees\a`, `D:\repo\.claude\worktrees\a`, true},
		{`D:\REPO\.claude\WORKTREES\a\file.go`, `D:\repo\.claude\worktrees\a`, true},
		{`D:\repo\.claude\worktrees\b\file.go`, `D:\repo\.claude\worktrees\a`, false},
		{`D:\repo\file.go`, "", false},
		{"", `D:\repo`, false},
	}
	for _, c := range cases {
		if got := isUnderDir(c.path, c.dir); got != c.want {
			t.Errorf("isUnderDir(%q, %q) = %v, want %v", c.path, c.dir, got, c.want)
		}
	}
}

func TestGitMainRepoRoot_FromMainRepo(t *testing.T) {
	repo := initTestRepo(t)
	got, err := gitMainRepoRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	gotClean, _ := filepath.EvalSymlinks(got)
	if gotClean != repo {
		t.Errorf("gitMainRepoRoot = %q, want %q", gotClean, repo)
	}
}

func TestGitMainRepoRoot_FromLinkedWorktree(t *testing.T) {
	repo := initTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(filepath.Dir(wt), 0755)
	gitRun(t, repo, "worktree", "add", "-b", "feature-a", wt)

	got, err := gitMainRepoRoot(wt)
	if err != nil {
		t.Fatal(err)
	}
	gotClean, _ := filepath.EvalSymlinks(got)
	if gotClean != repo {
		t.Errorf("gitMainRepoRoot(worktree) = %q, want main repo %q", gotClean, repo)
	}
}

func TestGitMainRepoRoot_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := gitMainRepoRoot(dir); err == nil {
		t.Fatal("expected an error for a non-git directory")
	}
}

func TestSidecarWriteReadRemove(t *testing.T) {
	hooksDir := t.TempDir()
	writeWorktreeSidecar(hooksDir, "sess1", `D:\repo\.claude\worktrees\a`, `D:\repo`)

	wt, root := resolveWorktreeContext(hooksDir, "sess1")
	if wt != `D:\repo\.claude\worktrees\a` || root != `D:\repo` {
		t.Fatalf("got wt=%q root=%q", wt, root)
	}

	removeWorktreeSidecar(hooksDir, "sess1")
	wt, root = resolveWorktreeContext(hooksDir, "sess1")
	if wt != "" || root != "" {
		t.Fatalf("expected empty context after remove, got wt=%q root=%q", wt, root)
	}
}

func TestResolveWorktreeContext_NoSidecarNoEnv(t *testing.T) {
	hooksDir := t.TempDir()
	wt, root := resolveWorktreeContext(hooksDir, "unknown-session")
	if wt != "" || root != "" {
		t.Fatalf("expected empty context, got wt=%q root=%q", wt, root)
	}
}

func TestResolveWorktreeContext_CorruptSidecarFailsOpen(t *testing.T) {
	hooksDir := t.TempDir()
	if err := os.WriteFile(sidecarPath(hooksDir, "sess-corrupt"), []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}
	wt, root := resolveWorktreeContext(hooksDir, "sess-corrupt")
	if wt != "" || root != "" {
		t.Fatalf("expected fail-open (empty context) for corrupt sidecar JSON, got wt=%q root=%q", wt, root)
	}
}

func TestResolveWorktreeContext_EnvVarTakesPriorityOverSidecar(t *testing.T) {
	hooksDir := t.TempDir()
	writeWorktreeSidecar(hooksDir, "sess1", `D:\stale\worktree`, `D:\stale`)
	t.Setenv("MULTITERMINAL_WORKTREE_PATH", `D:\fresh\worktree`)
	t.Setenv("MULTITERMINAL_MAIN_REPO_ROOT", `D:\fresh`)

	wt, root := resolveWorktreeContext(hooksDir, "sess1")
	if wt != `D:\fresh\worktree` || root != `D:\fresh` {
		t.Fatalf("env var should win, got wt=%q root=%q", wt, root)
	}
}
