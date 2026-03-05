package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temp git repo with one committed file (hello.txt).
func initTestRepo(t *testing.T) (string, *AppService) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0644)
	run("add", "hello.txt")
	run("commit", "-m", "initial commit")
	return dir, newTestApp()
}

func TestGetDiffStats_NoChanges(t *testing.T) {
	dir, app := initTestRepo(t)
	stats := app.GetDiffStats(dir)
	if len(stats) != 0 {
		t.Fatalf("expected 0 stats, got %d", len(stats))
	}
}

func TestGetDiffStats_ModifiedFile(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\nline 2\n"), 0644)
	stats := app.GetDiffStats(dir)
	if len(stats) == 0 {
		t.Fatal("expected at least 1 stat")
	}
	found := false
	for _, s := range stats {
		if s.Path == "hello.txt" {
			found = true
			if s.Insertions < 1 {
				t.Errorf("expected insertions >= 1, got %d", s.Insertions)
			}
		}
	}
	if !found {
		t.Error("hello.txt not found in stats")
	}
}

func TestGetDiffStats_NewFile(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("brand new\n"), 0644)
	stats := app.GetDiffStats(dir)
	found := false
	for _, s := range stats {
		if s.Path == "new.txt" {
			found = true
			if s.Status != "?" {
				t.Errorf("expected status '?', got %q", s.Status)
			}
		}
	}
	if !found {
		t.Error("new.txt not found in stats")
	}
}

func TestGetFileDiff_Modified(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("changed content\n"), 0644)
	diff := app.GetFileDiff(dir, "hello.txt")
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "changed content") {
		t.Error("diff should contain new content")
	}
}

func TestGetWorkingDiff(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("modified\n"), 0644)
	diff := app.GetWorkingDiff(dir)
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "modified") {
		t.Error("diff should contain 'modified'")
	}
}

func TestStageAndCommit(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("updated\n"), 0644)

	err := app.StageAndCommit(dir, "test commit")
	if err != nil {
		t.Fatalf("StageAndCommit failed: %v", err)
	}

	// Verify clean working tree
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected clean tree, got: %s", out)
	}
}

func TestStageAndCommit_NoChanges(t *testing.T) {
	dir, app := initTestRepo(t)
	err := app.StageAndCommit(dir, "nothing to commit")
	if err == nil {
		t.Error("expected error for no-changes commit")
	}
}

func TestPushBranch_NoRemote(t *testing.T) {
	dir, app := initTestRepo(t)
	err := app.PushBranch(dir)
	if err == nil {
		t.Error("expected error when no remote configured")
	}
}

func TestStageFiles_Selective(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb\n"), 0644)

	// Stage only a.txt
	err := app.StageFiles(dir, []string{"a.txt"})
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}

	// Commit staged
	err = app.CommitStaged(dir, "add a only")
	if err != nil {
		t.Fatalf("CommitStaged failed: %v", err)
	}

	// b.txt should still be untracked
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, _ := cmd.Output()
	output := strings.TrimSpace(string(out))
	if !strings.Contains(output, "b.txt") {
		t.Errorf("expected b.txt to remain, got: %s", output)
	}
	if strings.Contains(output, "a.txt") {
		t.Errorf("a.txt should be committed, got: %s", output)
	}
}

func TestGenerateCommitSuggestion_NewFile(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n"), 0644)
	suggestion := app.GenerateCommitSuggestion(dir, []string{"feature.go"})
	if suggestion.Type != "feat" {
		t.Errorf("expected type 'feat', got %q", suggestion.Type)
	}
}

func TestGenerateCommitSuggestion_TestFile(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte("package main\n"), 0644)
	suggestion := app.GenerateCommitSuggestion(dir, []string{"foo_test.go"})
	if suggestion.Type != "test" {
		t.Errorf("expected type 'test', got %q", suggestion.Type)
	}
}

func TestGenerateCommitSuggestion_FilteredPaths(t *testing.T) {
	dir, app := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\n"), 0644)
	// Only request suggestion for a.go
	suggestion := app.GenerateCommitSuggestion(dir, []string{"a.go"})
	if !strings.Contains(suggestion.Description, "a.go") {
		t.Errorf("description should mention a.go, got %q", suggestion.Description)
	}
	if strings.Contains(suggestion.Description, "b.go") {
		t.Errorf("description should NOT mention b.go, got %q", suggestion.Description)
	}
}

func TestInferScope(t *testing.T) {
	stats := []DiffFileStat{
		{Path: "internal/backend/app.go", Status: "M"},
		{Path: "internal/backend/app_git.go", Status: "M"},
	}
	scope := inferScope(stats)
	if scope != "backend" {
		t.Errorf("expected scope 'backend', got %q", scope)
	}
}

func TestInferScope_Frontend(t *testing.T) {
	stats := []DiffFileStat{
		{Path: "frontend/src/components/Foo.svelte", Status: "M"},
		{Path: "frontend/src/components/Bar.svelte", Status: "A"},
	}
	scope := inferScope(stats)
	if scope != "ui" {
		t.Errorf("expected scope 'ui', got %q", scope)
	}
}
