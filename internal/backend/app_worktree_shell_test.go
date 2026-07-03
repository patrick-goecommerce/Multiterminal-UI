package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellFinishPrimitives(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	info, err := a.CreatePaneWorktree(repo, "sh", "alpha-main")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(info.Path, "keep.txt"), []byte("k\n"), 0644)
	os.WriteFile(filepath.Join(info.Path, "secret.env"), []byte("s\n"), 0644)

	changes := a.GetWorktreeChangedFiles(info.Path)
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want 2 untracked", changes)
	}
	// Selektiver Commit: nur keep.txt — secret.env darf NICHT committet werden.
	if err := a.CommitWorktreeFiles(info.Path, []string{"keep.txt"}, "feat: keep"); err != nil {
		t.Fatal(err)
	}
	if out := gitRunOut(t, info.Path, "show", "--stat", "--oneline", "HEAD"); strings.Contains(out, "secret.env") {
		t.Fatal("selective commit staged unrelated file")
	}
	// Rebase auf unbewegtes Ziel ist ein No-op-Erfolg:
	if err := a.RebaseWorktreeOntoTarget(info.Path, "alpha-main"); err != nil {
		t.Fatal(err)
	}
}

func TestRebaseConflict_ReportsAndAborts(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	info, _ := a.CreatePaneWorktree(repo, "conf", "alpha-main")
	// Konflikt bauen: gleiche Datei in Ziel und Branch ändern.
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("target\n"), 0644)
	gitRun(t, repo, "commit", "-am", "target change")
	os.WriteFile(filepath.Join(info.Path, "README.md"), []byte("branch\n"), 0644)
	gitRun(t, info.Path, "commit", "-am", "branch change")

	if err := a.RebaseWorktreeOntoTarget(info.Path, "alpha-main"); err == nil {
		t.Fatal("expected rebase conflict error")
	}
	if err := a.AbortWorktreeRebase(info.Path); err != nil {
		t.Fatal(err)
	}
	// Nach Abort: kein rebase-in-progress, Branch-Commit intakt.
	if out := gitRunOut(t, info.Path, "log", "--oneline", "-1"); !strings.Contains(out, "branch change") {
		t.Errorf("abort lost the branch commit: %q", out)
	}
}
