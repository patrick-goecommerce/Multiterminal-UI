package backend

import (
	"encoding/json"
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

// A branch switch must not happen silently — a real dev test showed Claude
// switching branches inside an active worktree, which the memory-only
// instruction failed to prevent. It must not be forbidden outright either:
// the user wants to be asked, so a fresh settings file carries ask rules and
// no deny list at all.
func TestEnsureProjectWorktreeSetup_AsksBeforeBranchSwitch(t *testing.T) {
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
	perms := readPermissions(t, settingsPath)
	if _, ok := perms["deny"]; ok {
		t.Errorf("fresh settings must not carry a deny list: %s", settings)
	}
	assertList(t, perms["ask"], worktreeAskRules...)
}

// readPermissions parses a settings.local.json and returns its permissions
// object (empty map when absent).
func readPermissions(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	p, _ := s["permissions"].(map[string]any)
	if p == nil {
		p = map[string]any{}
	}
	return p
}

// assertList checks that a JSON list holds exactly want, in order.
func assertList(t *testing.T, got any, want ...string) {
	t.Helper()
	list, _ := got.([]any)
	if len(list) != len(want) {
		t.Fatalf("list = %v, want %v", list, want)
	}
	for i, w := range want {
		if list[i] != w {
			t.Errorf("list[%d] = %v, want %q (whole list %v)", i, list[i], w, list)
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

// An existing settings file without the branch-switch rules is left as its
// owner made it: MTUI cannot tell "never had them" from "removed them", and
// re-adding them on every launch is the bug this replaced (the old deny rules
// came back after every manual removal).
func TestEnsureProjectWorktreeSetup_DoesNotAddRulesToExistingSettings(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	claudeDir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	existing := `{
  "worktree": {
    "baseRef": "head"
  },
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
	if string(got) != existing {
		t.Errorf("settings without MTUI rules must stay byte-identical, got:\n%s", got)
	}
}

// One-time migration: the deny rules earlier MTUI versions wrote move to ask.
// A deny entry the user wrote stays in deny, other keys stay, and once moved
// the user can delete the ask rules without MTUI putting anything back.
func TestEnsureProjectWorktreeSetup_MigratesDenyRulesToAsk(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	claudeDir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	existing := `{
  "worktree": {
    "baseRef": "head"
  },
  "permissions": {
    "allow": [
      "Bash(go build:*)"
    ],
    "deny": [
      "Bash(rm -rf *)",
      "Bash(git checkout *)",
      "Bash(git switch *)"
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

	perms := readPermissions(t, settingsPath)
	assertList(t, perms["deny"], "Bash(rm -rf *)")
	assertList(t, perms["ask"], worktreeAskRules...)
	assertList(t, perms["allow"], "Bash(go build:*)")

	// The user now removes the ask rules; the next launch must not restore them.
	cleared := `{
  "worktree": {
    "baseRef": "head"
  },
  "permissions": {
    "deny": [
      "Bash(rm -rf *)"
    ]
  }
}
`
	if err := os.WriteFile(settingsPath, []byte(cleared), 0644); err != nil {
		t.Fatal(err)
	}
	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != cleared {
		t.Errorf("removed ask rules came back:\n%s", got)
	}
}

// A deny list that held nothing but MTUI's rules is dropped entirely, and a
// rule already in ask is not listed twice.
func TestEnsureProjectWorktreeSetup_MigrationDropsEmptyDenyAndDedupes(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	claudeDir := filepath.Join(repo, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	existing := `{
  "worktree": {
    "baseRef": "head"
  },
  "permissions": {
    "ask": [
      "Bash(git checkout *)"
    ],
    "deny": [
      "Bash(git checkout *)",
      "Bash(git switch *)"
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

	perms := readPermissions(t, settingsPath)
	if _, ok := perms["deny"]; ok {
		t.Errorf("empty deny list should be dropped: %v", perms)
	}
	assertList(t, perms["ask"], worktreeAskRules...)
}

// Calling EnsureProjectWorktreeSetup twice on a fresh project must not
// duplicate the ask rules.
func TestEnsureProjectWorktreeSetup_DoesNotDuplicateAskRules(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	perms := readPermissions(t, filepath.Join(repo, ".claude", "settings.local.json"))
	assertList(t, perms["ask"], worktreeAskRules...)
}

// If worktree.baseRef is already present (any value), EnsureProjectWorktreeSetup
// must leave it as-is.
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
