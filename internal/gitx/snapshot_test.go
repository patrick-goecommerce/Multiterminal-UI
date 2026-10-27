package gitx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// newSnapshotRepo makes a repo whose path contains a space, with one commit
// holding a.txt (two lines).
func newSnapshotRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := filepath.Join(t.TempDir(), "repo mit leer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-q")
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := Cmd(dir, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotLeavesIndexUntouched(t *testing.T) {
	dir := newSnapshotRepo(t)
	writeFile(t, dir, "a.txt", "one\ntwo\nthree\n")
	writeFile(t, dir, "b.txt", "new\n")

	indexPath := filepath.Join(dir, ".git", "index")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	stBefore, _ := os.Stat(indexPath)
	statusBefore := runGit(t, dir, "status", "--porcelain")

	tree, err := SnapshotTree(context.Background(), dir)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if len(tree) < 40 {
		t.Fatalf("tree id looks wrong: %q", tree)
	}

	after, _ := os.ReadFile(indexPath)
	stAfter, _ := os.Stat(indexPath)
	if !bytes.Equal(before, after) {
		t.Error("index bytes changed")
	}
	if !stBefore.ModTime().Equal(stAfter.ModTime()) {
		t.Error("index mtime changed")
	}
	if got := runGit(t, dir, "status", "--porcelain"); got != statusBefore {
		t.Errorf("status changed:\nbefore %q\nafter  %q", statusBefore, got)
	}
	// The snapshot must hold the uncommitted and the untracked file.
	files := runGit(t, dir, "ls-tree", "-r", "--name-only", tree)
	if !strings.Contains(files, "b.txt") {
		t.Errorf("untracked b.txt missing from snapshot: %q", files)
	}
}

func TestSnapshotFreshRepoWithoutIndex(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := filepath.Join(t.TempDir(), "repo mit leer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-q")
	writeFile(t, dir, "x.txt", "x\n")
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); err == nil {
		t.Fatal("test premise: fresh repo should have no index")
	}
	tree, err := SnapshotTree(context.Background(), dir)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if files := runGit(t, dir, "ls-tree", "-r", "--name-only", tree); !strings.Contains(files, "x.txt") {
		t.Errorf("x.txt missing: %q", files)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); err == nil {
		t.Error("snapshot created the real index")
	}
}

func TestSnapshotInLinkedWorktree(t *testing.T) {
	dir := newSnapshotRepo(t)
	wt := filepath.Join(filepath.Dir(dir), "linked mit leer")
	runGit(t, dir, "worktree", "add", "-q", "-b", "side", wt)
	writeFile(t, wt, "w.txt", "w\n")
	tree, err := SnapshotTree(context.Background(), wt)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if files := runGit(t, wt, "ls-tree", "-r", "--name-only", tree); !strings.Contains(files, "w.txt") {
		t.Errorf("w.txt missing: %q", files)
	}
}

func TestNumStatBetweenSnapshots(t *testing.T) {
	dir := newSnapshotRepo(t)
	tree1, err := SnapshotTree(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	// a.txt: one\ntwo\n -> one\nx\ny\n  = 2 added, 1 deleted
	writeFile(t, dir, "a.txt", "one\nx\ny\n")
	writeFile(t, dir, "c.txt", "1\n2\n3\n")
	tree2, err := SnapshotTree(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := NumStat(context.Background(), dir, tree1, tree2)
	if err != nil {
		t.Fatal(err)
	}
	want := []FileStat{{Path: "a.txt", Added: 2, Deleted: 1}, {Path: "c.txt", Added: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestNumStatBinary(t *testing.T) {
	dir := newSnapshotRepo(t)
	tree1, _ := SnapshotTree(context.Background(), dir)
	writeFile(t, dir, "bin.dat", "a\x00b\x00c")
	tree2, _ := SnapshotTree(context.Background(), dir)
	got, err := NumStat(context.Background(), dir, tree1, tree2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "bin.dat" || !got[0].Binary {
		t.Errorf("got %+v", got)
	}
}

func TestSnapshotHonoursGitignore(t *testing.T) {
	dir := newSnapshotRepo(t)
	writeFile(t, dir, ".gitignore", "node_modules/\n")
	runGit(t, dir, "add", ".gitignore")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "ignore")
	writeFile(t, dir, "node_modules/pkg/index.js", "x\n")
	writeFile(t, dir, "src/main.go", "package main\n")
	tree, err := SnapshotTree(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	got, err := NumStat(context.Background(), dir, head, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "src/main.go" {
		t.Errorf("want only src/main.go, got %+v", got)
	}
}

func TestSnapshotCancelled(t *testing.T) {
	dir := newSnapshotRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SnapshotTree(ctx, dir); err == nil {
		t.Error("want an error for a cancelled context")
	}
	// No temp index may be left behind.
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "mtui-index-*"))
	for _, m := range matches {
		t.Errorf("leftover temp index %s", m)
	}
}
