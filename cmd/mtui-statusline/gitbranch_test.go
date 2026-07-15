package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitBranchReadsHEAD(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, ".git"), 0755)
	os.WriteFile(filepath.Join(d, ".git", "HEAD"), []byte("ref: refs/heads/feature/x\n"), 0644)
	sub := filepath.Join(d, "a", "b")
	os.MkdirAll(sub, 0755)
	if got := gitBranch(sub); got != "feature/x" {
		t.Fatalf("got %q want feature/x", got)
	}
}

func TestGitBranchDetachedReturnsEmpty(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, ".git"), 0755)
	os.WriteFile(filepath.Join(d, ".git", "HEAD"), []byte("a1b2c3d4e5\n"), 0644)
	if got := gitBranch(d); got != "" {
		t.Fatalf("got %q want empty (detached)", got)
	}
}

func TestGitBranchWorktreeFile(t *testing.T) {
	d := t.TempDir()
	real := filepath.Join(d, "realgit")
	os.MkdirAll(real, 0755)
	os.WriteFile(filepath.Join(real, "HEAD"), []byte("ref: refs/heads/wt\n"), 0644)
	wt := filepath.Join(d, "wt")
	os.MkdirAll(wt, 0755)
	os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+real+"\n"), 0644)
	if got := gitBranch(wt); got != "wt" {
		t.Fatalf("got %q want wt", got)
	}
}

func TestGitBranchNoRepo(t *testing.T) {
	if got := gitBranch(t.TempDir()); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}
