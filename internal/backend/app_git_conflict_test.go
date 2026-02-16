package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClassifyGitStatus_Unmerged(t *testing.T) {
	cases := []string{"UU", "AU", "UA", "DU", "UD", "AA", "DD"}
	for _, xy := range cases {
		if got := classifyGitStatus(xy); got != "U" {
			t.Fatalf("classifyGitStatus(%q) = %q, want 'U'", xy, got)
		}
	}
}

func TestDetectMergeOperation(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)

	// No sentinel → empty
	if op := detectMergeOperation(dir); op != "" {
		t.Fatalf("expected empty, got %q", op)
	}
}

func TestDetectMergeOperation_Merge(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("init"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "init")

	// Create MERGE_HEAD sentinel manually
	gitDir := filepath.Join(dir, ".git")
	os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("abc"), 0644)

	if op := detectMergeOperation(dir); op != "merge" {
		t.Fatalf("expected 'merge', got %q", op)
	}
}

func TestDetectMergeOperation_CherryPick(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("init"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "init")

	gitDir := filepath.Join(dir, ".git")
	os.WriteFile(filepath.Join(gitDir, "CHERRY_PICK_HEAD"), []byte("abc"), 0644)

	if op := detectMergeOperation(dir); op != "cherry-pick" {
		t.Fatalf("expected 'cherry-pick', got %q", op)
	}
}

func TestDetectMergeOperation_Rebase(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("init"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "init")

	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(filepath.Join(gitDir, "rebase-merge"), 0755)

	if op := detectMergeOperation(dir); op != "rebase" {
		t.Fatalf("expected 'rebase', got %q", op)
	}
}

func TestGetMergeConflicts_EmptyDir(t *testing.T) {
	a := newTestApp()
	info := a.GetMergeConflicts("")
	if info.Count != 0 {
		t.Fatalf("expected count 0, got %d", info.Count)
	}
}

func TestGetMergeConflicts_NoConflicts(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "init")

	a := newTestApp()
	info := a.GetMergeConflicts(dir)
	if info.Count != 0 {
		t.Fatalf("expected 0 conflicts, got %d", info.Count)
	}
}

func TestGetMergeConflicts_WithConflicts(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), gitTestEnv()...)
		out, err := cmd.CombinedOutput()
		// Allow merge to fail (expected conflict)
		if err != nil && args[0] != "merge" {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("base"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "base")

	run("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("feature-change"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "feature")

	run("checkout", "master")
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("master-change"), 0644)
	run("add", ".")
	run("commit", "--no-gpg-sign", "-m", "master")

	run("merge", "feature") // should fail with conflict

	a := newTestApp()
	info := a.GetMergeConflicts(dir)
	if info.Count != 1 {
		t.Fatalf("expected 1 conflict, got %d", info.Count)
	}
	if info.Operation != "merge" {
		t.Fatalf("expected operation 'merge', got %q", info.Operation)
	}
	if len(info.Files) != 1 || info.Files[0] != "test.txt" {
		t.Fatalf("expected files [test.txt], got %v", info.Files)
	}
}
