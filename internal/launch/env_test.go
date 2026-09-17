package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// envMap turns the KEY=VALUE slice into something a test can ask questions of.
func envMap(t *testing.T, env []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(env))
	for _, e := range env {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			t.Fatalf("environment entry without a value: %q", e)
		}
		out[k] = v
	}
	return out
}

func TestIsClaudeMode(t *testing.T) {
	for _, mode := range []string{"claude", "claude-auto", "claude-yolo"} {
		if !IsClaudeMode(mode) {
			t.Errorf("IsClaudeMode(%q) = false", mode)
		}
	}
	for _, mode := range []string{"", "shell", "codex", "gemini", "Claude", "claude-x"} {
		if IsClaudeMode(mode) {
			t.Errorf("IsClaudeMode(%q) = true", mode)
		}
	}
}

// A shell pane gets the helper port and nothing else. The hook variables would
// be meaningless to it, and MULTITERMINAL_SESSION_ID in particular would make
// a plain shell look like an agent to anything reading the environment.
func TestEnv_AShellPaneGetsOnlyTheShimPort(t *testing.T) {
	p := Policy{ShimPort: 41234}
	env := envMap(t, p.Env(7, t.TempDir(), "shell"))

	if env["MTUI_PORT"] != "41234" {
		t.Errorf("MTUI_PORT = %q, want 41234", env["MTUI_PORT"])
	}
	if _, ok := env["MULTITERMINAL_SESSION_ID"]; ok {
		t.Error("a shell pane was given a session ID")
	}
}

// No shim port means no variable at all rather than MTUI_PORT=0: a helper that
// reads "0" would try to post to port zero on every call.
func TestEnv_NoShimPortMeansNoVariable(t *testing.T) {
	env := envMap(t, Policy{}.Env(1, t.TempDir(), "shell"))
	if _, ok := env["MTUI_PORT"]; ok {
		t.Errorf("MTUI_PORT was set without a port: %q", env["MTUI_PORT"])
	}
}

// The session ID is what the hook and the statusline shim report back with.
// A Claude pane without it falls back to screen scraping, silently.
func TestEnv_AClaudePaneAnnouncesItsSessionID(t *testing.T) {
	env := envMap(t, Policy{ShimPort: 1}.Env(42, t.TempDir(), "claude"))
	if env["MULTITERMINAL_SESSION_ID"] != "42" {
		t.Errorf("MULTITERMINAL_SESSION_ID = %q, want 42", env["MULTITERMINAL_SESSION_ID"])
	}
}

// initRepo makes a git repository a test can point the policy at.
func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on this machine")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.invalid"},
		{"config", "user.name", "Test"},
		{"commit", "-q", "--allow-empty", "-m", "erster"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// macOS puts TempDir under a symlink, and git reports the resolved path.
	// Comparing an unresolved path against git's answer would fail there for
	// reasons that have nothing to do with the policy.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return resolved
}

// The firewall variable is what cmd/mtui-hook enforces the worktree policy
// with. It is off unless somebody asked for it.
func TestForceWorktreeRoot_OffByDefault(t *testing.T) {
	repo := initRepo(t)
	if got := (Policy{}).ForceWorktreeRoot(repo); got != "" {
		t.Errorf("ForceWorktreeRoot = %q with the policy off, want empty", got)
	}
}

func TestForceWorktreeRoot_OnGivesTheMainRoot(t *testing.T) {
	repo := initRepo(t)
	got := (Policy{ForceWorktrees: true}).ForceWorktreeRoot(repo)
	if got != repo {
		t.Errorf("ForceWorktreeRoot = %q, want %q", got, repo)
	}
}

// A directory that is not a repository has nothing to protect, so the firewall
// must not be armed with a root it invented.
func TestForceWorktreeRoot_NonRepoIsEmptyEvenWhenOn(t *testing.T) {
	if got := (Policy{ForceWorktrees: true}).ForceWorktreeRoot(t.TempDir()); got != "" {
		t.Errorf("ForceWorktreeRoot = %q outside a repo, want empty", got)
	}
}

// The per-project override wins over the global setting in both directions.
// This is the pair that decides whether an agent is confined to a worktree, so
// getting it backwards is not a cosmetic bug.
func TestForceWorktreeRoot_ProjectOverrideWinsBothWays(t *testing.T) {
	t.Run("off exempts a project while the global setting is on", func(t *testing.T) {
		repo := initRepo(t)
		writeOverride(t, repo, false)
		if got := (Policy{ForceWorktrees: true}).ForceWorktreeRoot(repo); got != "" {
			t.Errorf("ForceWorktreeRoot = %q, want empty", got)
		}
	})
	t.Run("on forces a project while the global setting is off", func(t *testing.T) {
		repo := initRepo(t)
		writeOverride(t, repo, true)
		if got := (Policy{}).ForceWorktreeRoot(repo); got != repo {
			t.Errorf("ForceWorktreeRoot = %q, want %q", got, repo)
		}
	})
}

func TestEffectiveForceWorktrees_FollowsTheSameResolution(t *testing.T) {
	repo := initRepo(t)
	writeOverride(t, repo, false)
	if (Policy{ForceWorktrees: true}).EffectiveForceWorktrees(repo) {
		t.Error("the project override was ignored")
	}
	// Outside a repo there is no override to read, so the global answer stands.
	if !(Policy{ForceWorktrees: true}).EffectiveForceWorktrees(t.TempDir()) {
		t.Error("the global setting was dropped outside a repo")
	}
}

// A worktree variable pair only makes sense for a linked worktree. The main
// checkout, and any subdirectory of it, is not one.
func TestWorktreeEnvVars_MainCheckoutGetsNone(t *testing.T) {
	repo := initRepo(t)
	if got := WorktreeEnvVars(repo); got != nil {
		t.Errorf("WorktreeEnvVars(main checkout) = %v, want nil", got)
	}
	sub := filepath.Join(repo, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if got := WorktreeEnvVars(sub); got != nil {
		t.Errorf("WorktreeEnvVars(subdirectory) = %v, want nil", got)
	}
}

func TestWorktreeEnvVars_NonRepoGetsNone(t *testing.T) {
	if got := WorktreeEnvVars(t.TempDir()); got != nil {
		t.Errorf("WorktreeEnvVars(no repo) = %v, want nil", got)
	}
	if got := WorktreeEnvVars(""); got != nil {
		t.Errorf("WorktreeEnvVars(\"\") = %v, want nil", got)
	}
}

// A linked worktree is the case the variables exist for: mtui-hook needs both
// where the pane is and where the repo it belongs to lives.
func TestWorktreeEnvVars_LinkedWorktreeGetsBothPaths(t *testing.T) {
	repo := initRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "linked")
	cmd := exec.Command("git", "worktree", "add", "-q", "-b", "seiten-zweig", wt)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git worktree add: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = os.RemoveAll(wt) })

	env := envMap(t, WorktreeEnvVars(wt))
	if env["MULTITERMINAL_MAIN_REPO_ROOT"] != repo {
		t.Errorf("MULTITERMINAL_MAIN_REPO_ROOT = %q, want %q",
			env["MULTITERMINAL_MAIN_REPO_ROOT"], repo)
	}
	if env["MULTITERMINAL_WORKTREE_PATH"] == "" {
		t.Error("MULTITERMINAL_WORKTREE_PATH is empty for a linked worktree")
	}
}

func writeOverride(t *testing.T, repo string, force bool) {
	t.Helper()
	if err := SaveProjectConfig(repo, ProjectConfig{ForceWorktrees: &force}); err != nil {
		t.Fatalf("SaveProjectConfig: %v", err)
	}
}
