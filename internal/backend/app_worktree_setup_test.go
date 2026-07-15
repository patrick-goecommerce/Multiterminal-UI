package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureProjectWorktreeSetup_CreatesFilesOnce(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	memPath := filepath.Join(repo, projectWorktreeMemoryFile)
	mem, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("memory file not created: %v", err)
	}
	for _, want := range []string{"EnterWorktree", "discard_changes", "NIEMALS", "erfordern IMMER vorherige Zustimmung", "**Worktree:**", "docs/superpowers/specs", "docs/superpowers/plans"} {
		if !strings.Contains(string(mem), want) {
			t.Errorf("memory file missing %q", want)
		}
	}

	settingsPath := filepath.Join(repo, ".claude", "settings.local.json")
	settings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
	if !strings.Contains(string(settings), `"baseRef"`) || !strings.Contains(string(settings), `"head"`) {
		t.Errorf("settings missing worktree.baseRef=head: %s", settings)
	}
}

// Hard rule: as long as a pane has a worktree open, its branch (and the main
// project's branch, since these deny rules apply session-wide) must never be
// switched — a real dev test showed Claude switching branches inside an
// active worktree, which the memory-only instruction failed to prevent.
// Session-wide Bash denies are the only verified enforcement mechanism
// (settings.local.json rules from the project root apply for the whole
// session, even after EnterWorktree changes the cwd — see design doc
// 2026-07-03 section 2).
func TestEnsureProjectWorktreeSetup_DeniesBranchSwitchViaBash(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(repo, ".claude", "settings.local.json")
	settings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
	for _, want := range []string{`"Bash(git checkout *)"`, `"Bash(git switch *)"`} {
		if !strings.Contains(string(settings), want) {
			t.Errorf("settings missing branch-switch deny rule %s: %s", want, settings)
		}
	}
}

func TestEnsureProjectWorktreeSetup_DoesNotOverwriteExisting(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	memPath := filepath.Join(repo, projectWorktreeMemoryFile)
	custom := []byte("# custom edits by the user\n")
	if err := os.WriteFile(memPath, custom, 0644); err != nil {
		t.Fatal(err)
	}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(memPath)
	if string(got) != string(custom) {
		t.Error("second call overwrote the user's edited memory file")
	}
}

// A project whose CLAUDE.local.md still holds a memory instruction that MTUI
// itself wrote out in an earlier version must be migrated to the current
// text, since the file is entirely MTUI-owned there — no user content to lose.
func TestEnsureProjectWorktreeSetup_MigratesPriorMtuiVersion(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	memPath := filepath.Join(repo, projectWorktreeMemoryFile)
	stale := projectWorktreeMemoryPriorVersions[0]
	if err := os.WriteFile(memPath, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != projectWorktreeMemoryContent {
		t.Errorf("stale MTUI-generated memory file was not migrated to the current version:\n%s", got)
	}
}

// The prior MTUI-generated text (before the spec/plan worktree-header
// paragraph was added) must also be migrated, not just the oldest one.
func TestEnsureProjectWorktreeSetup_MigratesPreviousMtuiVersion(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	memPath := filepath.Join(repo, projectWorktreeMemoryFile)
	stale := projectWorktreeMemoryPriorVersions[len(projectWorktreeMemoryPriorVersions)-1]
	if err := os.WriteFile(memPath, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != projectWorktreeMemoryContent {
		t.Errorf("stale MTUI-generated memory file was not migrated to the current version:\n%s", got)
	}
}

// EnsureProjectWorktreeSetup must add worktree.baseRef to an existing
// settings.local.json without touching unrelated keys already in it (e.g. a
// user's Bash permission allowlist).
func TestEnsureProjectWorktreeSetup_MergesBaseRefIntoExistingSettings(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	claudeDir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	existing := `{
  "permissions": {
    "allow": [
      "Bash(go build:*)"
    ]
  }
}
`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"Bash(go build:*)"`) {
		t.Errorf("merge dropped existing settings content: %s", got)
	}
	if !strings.Contains(string(got), `"baseRef"`) || !strings.Contains(string(got), `"head"`) {
		t.Errorf("merge did not add worktree.baseRef=head: %s", got)
	}
}

// Real-world case (found live in the D:\repos\gotime project): a
// settings.local.json written long before the worktree feature existed can
// already have a broad "Bash(git checkout:*)" ALLOW entry and a large,
// unrelated allow-list. EnsureProjectWorktreeSetup must still add the
// branch-switch deny rules without touching any of that — Deny always wins
// over Allow, but only if the deny rule actually gets written.
func TestEnsureProjectWorktreeSetup_AddsDenyRulesToExistingSettingsWithAllowlist(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	claudeDir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	existing := `{
  "permissions": {
    "allow": [
      "Bash(git checkout:*)",
      "Bash(npm run build:*)"
    ]
  }
}
`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"Bash(git checkout:*)"`, `"Bash(npm run build:*)"`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("merge dropped existing allow entry %s: %s", want, got)
		}
	}
	for _, want := range worktreeDenyRules {
		if !strings.Contains(string(got), `"`+want+`"`) {
			t.Errorf("merge did not add deny rule %q: %s", want, got)
		}
	}
}

// Calling EnsureProjectWorktreeSetup twice on a settings file that already
// has the deny rules must not duplicate them.
func TestEnsureProjectWorktreeSetup_DoesNotDuplicateDenyRules(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(repo, ".claude", "settings.local.json")
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(got), `"Bash(git checkout *)"`); n != 1 {
		t.Errorf("expected exactly 1 occurrence of the checkout deny rule, got %d: %s", n, got)
	}
}

// If worktree.baseRef is already present (any value), EnsureProjectWorktreeSetup
// must leave it as-is (but still add the separately-tracked deny rules).
func TestEnsureProjectWorktreeSetup_DoesNotOverwriteExistingBaseRef(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	claudeDir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	existing := `{
  "worktree": {
    "baseRef": "fresh"
  }
}
`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(got), `"baseRef": "fresh"`) {
		t.Error("EnsureProjectWorktreeSetup overwrote an existing custom worktree.baseRef value")
	}
}
