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
