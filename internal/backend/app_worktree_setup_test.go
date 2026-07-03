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
	for _, want := range []string{"EnterWorktree", "discard_changes", "NIEMALS", "erfordern IMMER vorherige Zustimmung"} {
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

// If worktree.baseRef is already present (any value), EnsureProjectWorktreeSetup
// must not touch the settings file at all.
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
	if string(got) != existing {
		t.Error("EnsureProjectWorktreeSetup touched a settings file that already had worktree.baseRef set")
	}
}
