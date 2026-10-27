package hub

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/gitx"
)

func tgit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func twrite(t *testing.T, dir, name string, lines int) {
	t.Helper()
	var b strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tcommit(t *testing.T, dir, msg string) {
	t.Helper()
	tgit(t, dir, "add", "-A")
	tgit(t, dir, "commit", "-q", "-m", msg)
}

// newRepoWithOrigin makes a work repo (path with a space) on main with one
// commit pushed to a bare origin whose HEAD is main.
func newRepoWithOrigin(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "my repo")
	bare := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work tree")
	for _, d := range []string{bare, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tgit(t, bare, "init", "-q", "--bare", "-b", "main")
	tgit(t, work, "init", "-q", "-b", "main")
	tgit(t, work, "remote", "add", "origin", bare)
	twrite(t, work, "base.txt", 1)
	tcommit(t, work, "base")
	tgit(t, work, "push", "-q", "origin", "main")
	tgit(t, work, "remote", "set-head", "origin", "main")
	return work
}

// runBriefing measures dir; a start tree is taken to come from dir itself.
func runBriefing(t *testing.T, dir, start string) gitResult {
	t.Helper()
	var top string
	if start != "" {
		top = tgit(t, dir, "rev-parse", "--show-toplevel")
	}
	res, ok := gitBriefing(context.Background(), dir, start, top, "", nil, BriefingPolicy{})
	if !ok {
		t.Fatal("not a repo")
	}
	return res
}

func TestBranchAgainstTarget(t *testing.T) {
	w := newRepoWithOrigin(t)
	tgit(t, w, "checkout", "-q", "-b", "feat")
	twrite(t, w, "a.txt", 20)
	tcommit(t, w, "one")
	twrite(t, w, "b.txt", 10)
	twrite(t, w, "c.txt", 10)
	tcommit(t, w, "two")
	twrite(t, w, "d.txt", 2)
	f := runBriefing(t, w, "").Facts
	if f.Target != "main" || f.Commits != 2 || f.BranchFiles != 4 || f.BranchAdded != 42 || f.Uncommitted != 1 || !f.NoUpstream {
		t.Fatalf("facts = %+v", f)
	}
	if f.TurnKnown {
		t.Fatal("turn must be unknown without a start tree")
	}
}

func TestUnpushedAgainstUpstream(t *testing.T) {
	w := newRepoWithOrigin(t)
	tgit(t, w, "checkout", "-q", "-b", "feat")
	twrite(t, w, "a.txt", 3)
	tcommit(t, w, "one")
	tgit(t, w, "push", "-q", "-u", "origin", "feat")
	twrite(t, w, "b.txt", 3)
	tcommit(t, w, "two")
	res := runBriefing(t, w, "")
	if res.Facts.Unpushed != 1 || res.Facts.NoUpstream {
		t.Fatalf("facts = %+v", res.Facts)
	}
	if res.Head == "" {
		t.Fatal("head missing")
	}
}

func TestOnTargetMeasuresTurn(t *testing.T) {
	w := newRepoWithOrigin(t)
	start, err := gitx.SnapshotTree(context.Background(), w)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		twrite(t, w, fmt.Sprintf("n%d.txt", i), 1)
	}
	res := runBriefing(t, w, start)
	f := res.Facts
	if !f.OnTarget || f.BranchFiles != 0 || !res.BigDiff || !f.TurnKnown || f.TurnFiles != 16 || f.TurnAdded != 16 {
		t.Fatalf("facts = %+v big=%v", f, res.BigDiff)
	}
}

func TestLockfileLinesDoNotCount(t *testing.T) {
	w := newRepoWithOrigin(t)
	tgit(t, w, "checkout", "-q", "-b", "feat")
	twrite(t, w, "package-lock.json", 900)
	twrite(t, w, "a.go", 10)
	tcommit(t, w, "x")
	res := runBriefing(t, w, "")
	if res.BigDiff || res.Facts.BranchAdded != 910 {
		t.Fatalf("big=%v facts=%+v", res.BigDiff, res.Facts)
	}
}

func TestBigDiffByLines(t *testing.T) {
	w := newRepoWithOrigin(t)
	tgit(t, w, "checkout", "-q", "-b", "feat")
	twrite(t, w, "a.txt", 300)
	twrite(t, w, "b.txt", 201)
	tcommit(t, w, "x")
	if res := runBriefing(t, w, ""); !res.BigDiff {
		t.Fatalf("expected big diff, facts=%+v", res.Facts)
	}
}

func TestTargetOrder(t *testing.T) {
	w := newRepoWithOrigin(t)
	ctx := context.Background()
	if got := resolveTarget(ctx, w, "release", "alpha-main"); got != "release" {
		t.Fatalf("prBase: %q", got)
	}
	if got := resolveTarget(ctx, w, "", "alpha-main"); got != "alpha-main" {
		t.Fatalf("window: %q", got)
	}
	if got := resolveTarget(ctx, w, "", ""); got != "main" {
		t.Fatalf("origin/HEAD: %q", got)
	}
	plain := t.TempDir()
	tgit(t, plain, "init", "-q")
	if got := resolveTarget(ctx, plain, "", ""); got != "main" {
		t.Fatalf("fallback: %q", got)
	}
}

func TestNoRepo(t *testing.T) {
	if _, ok := gitBriefing(context.Background(), t.TempDir(), "", "", "", nil, BriefingPolicy{}); ok {
		t.Fatal("ok in a plain directory")
	}
}

// M3: without origin/HEAD the target falls back to "main". In a repository
// whose trunk is master that names no ref: the branch numbers would all be
// zeros presented as facts. They stay out, the target too; the rest stays,
// and the big-diff threshold measures the turn as on the target.
func TestUnresolvableTargetLeavesBranchOut(t *testing.T) {
	w := t.TempDir()
	tgit(t, w, "init", "-q", "-b", "master")
	twrite(t, w, "base.txt", 1)
	tcommit(t, w, "base")
	tgit(t, w, "checkout", "-q", "-b", "feat")
	twrite(t, w, "a.txt", 3)
	tcommit(t, w, "one")
	start, err := gitx.SnapshotTree(context.Background(), w)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		twrite(t, w, fmt.Sprintf("n%d.txt", i), 1)
	}
	res := runBriefing(t, w, start)
	f := res.Facts
	if f.Target != "" || f.OnTarget {
		t.Errorf("Target/OnTarget = %q/%v, want empty/false", f.Target, f.OnTarget)
	}
	if f.Commits != 0 || f.BranchFiles != 0 || f.BranchAdded != 0 || f.BranchDeleted != 0 {
		t.Errorf("branch numbers = %+v, want none", f)
	}
	if f.Branch != "feat" || f.Uncommitted != 16 || !f.NoUpstream || !f.TurnKnown || f.TurnFiles != 16 {
		t.Errorf("the rest = %+v, want kept", f)
	}
	if !res.BigDiff {
		t.Error("16 files in the turn without branch numbers: want big diff")
	}
}

func TestDetachedHead(t *testing.T) {
	w := newRepoWithOrigin(t)
	tgit(t, w, "checkout", "-q", "--detach")
	f := runBriefing(t, w, "").Facts
	if !f.Detached || f.Branch != "" {
		t.Fatalf("facts = %+v", f)
	}
}

func TestParseGitHubRemote(t *testing.T) {
	want := ghRepo{Owner: "patrick-goecommerce", Name: "Multiterminal-UI"}
	for _, u := range []string{
		"https://github.com/patrick-goecommerce/Multiterminal-UI.git",
		"https://github.com/patrick-goecommerce/Multiterminal-UI",
		"git@github.com:patrick-goecommerce/Multiterminal-UI.git",
		"ssh://git@github.com/patrick-goecommerce/Multiterminal-UI",
	} {
		if got, ok := parseGitHubRemote(u); !ok || got != want {
			t.Errorf("%s: %+v %v", u, got, ok)
		}
	}
	if _, ok := parseGitHubRemote("https://gitlab.com/o/r.git"); ok {
		t.Error("gitlab accepted")
	}
}

func TestBriefingPolicyNormalized(t *testing.T) {
	if p := (BriefingPolicy{}).normalized(); p.BigDiffFiles != 15 || p.BigDiffLines != 500 {
		t.Fatalf("%+v", p)
	}
	if p := (BriefingPolicy{BigDiffFiles: 3, BigDiffLines: 7}).normalized(); p.BigDiffFiles != 3 || p.BigDiffLines != 7 {
		t.Fatalf("%+v", p)
	}
}
