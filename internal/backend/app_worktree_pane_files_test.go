package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitRunOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitCmd(dir, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestEnsureInfoExclude_AddsOnceIdempotent(t *testing.T) {
	repo := initPaneTestRepo(t)
	for i := 0; i < 2; i++ {
		if err := ensureInfoExclude(repo, []string{"CLAUDE.local.md", ".claude/settings.local.json"}); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "CLAUDE.local.md"); got != 1 {
		t.Errorf("CLAUDE.local.md appears %d times, want 1", got)
	}
	if !strings.Contains(string(data), ".claude/settings.local.json") {
		t.Error("settings.local.json pattern missing")
	}
}

func TestWriteWorktreeControlFiles(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".mt-worktrees", "feat")
	gitRun(t, repo, "worktree", "add", "-b", "terminal/feat", wt, "alpha-main")
	if err := writeWorktreeControlFiles(wt, repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(wt, "CLAUDE.local.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"terminal/feat", "alpha-main", "NIEMALS"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("CLAUDE.local.md missing %q", want)
		}
	}
	settings, err := os.ReadFile(filepath.Join(wt, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Ask, not deny: the user wants these actions confirmed, not forbidden.
	for _, want := range []string{`"ask"`, "git merge ", "git worktree remove ", "git branch -D ", "git push ", "Edit(CLAUDE.local.md)"} {
		if !strings.Contains(string(settings), want) {
			t.Errorf("settings.local.json missing %q", want)
		}
	}
	if strings.Contains(string(settings), `"deny"`) {
		t.Errorf("settings.local.json must not carry a deny list: %s", settings)
	}
	// Control files must be invisible to git status (tracked-only AND untracked):
	if out := gitRunOut(t, wt, "status", "--porcelain"); strings.Contains(out, "CLAUDE.local.md") || strings.Contains(out, "settings.local.json") {
		t.Errorf("control files leak into git status: %q", out)
	}
}
