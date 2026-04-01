package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupRepoWithCommits creates a temp git repo with the given commit history.
// messages[i] is the commit message, files[i] lists which files to touch for that commit.
func setupRepoWithCommits(t *testing.T, messages []string, files [][]string) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.name", "test")
	run("git", "config", "user.email", "test@test.com")

	for i, msg := range messages {
		var touchFiles []string
		if i < len(files) {
			touchFiles = files[i]
		}
		if len(touchFiles) == 0 {
			touchFiles = []string{fmt.Sprintf("file%d.txt", i)}
		}
		for _, f := range touchFiles {
			fpath := filepath.Join(dir, f)
			os.MkdirAll(filepath.Dir(fpath), 0o755)
			content := fmt.Sprintf("commit %d: %s\n", i, msg)
			os.WriteFile(fpath, []byte(content), 0o644)
		}
		run("git", "add", "-A")
		run("git", "commit", "-m", msg)
	}

	return dir
}

func TestFixChainDetected(t *testing.T) {
	dir := setupRepoWithCommits(t,
		[]string{"fix: typo", "fix: import", "fix: build error"},
		nil,
	)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	found := false
	for _, s := range signals {
		if s.Type == "fix_chain" {
			found = true
			if s.Source != "repo" {
				t.Errorf("expected source=repo, got %s", s.Source)
			}
		}
	}
	if !found {
		t.Error("expected fix_chain signal, got none")
	}
}

func TestFixChainNotTriggeredBelow3(t *testing.T) {
	dir := setupRepoWithCommits(t,
		[]string{"fix: typo", "fix: import"},
		nil,
	)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	for _, s := range signals {
		if s.Type == "fix_chain" {
			t.Error("expected no fix_chain signal for only 2 fix commits")
		}
	}
}

func TestFixChainBrokenByNonFix(t *testing.T) {
	dir := setupRepoWithCommits(t,
		[]string{"fix: a", "fix: b", "feat: new feature", "fix: c"},
		nil,
	)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	for _, s := range signals {
		if s.Type == "fix_chain" {
			t.Error("expected no fix_chain when chain is broken by non-fix commit")
		}
	}
}

func TestRevertDetected(t *testing.T) {
	dir := setupRepoWithCommits(t,
		[]string{"feat: add feature", "Revert \"feat: add feature\""},
		nil,
	)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	found := false
	for _, s := range signals {
		if s.Type == "revert" {
			found = true
		}
	}
	if !found {
		t.Error("expected revert signal, got none")
	}
}

func TestFileChurnDetected(t *testing.T) {
	msgs := make([]string, 6)
	filesets := make([][]string, 6)
	for i := 0; i < 6; i++ {
		msgs[i] = fmt.Sprintf("change %d", i)
		filesets[i] = []string{"hot_file.go"}
	}

	dir := setupRepoWithCommits(t, msgs, filesets)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	found := false
	for _, s := range signals {
		if s.Type == "file_churn" {
			found = true
			if s.Source != "repo" {
				t.Errorf("expected source=repo, got %s", s.Source)
			}
		}
	}
	if !found {
		t.Error("expected file_churn signal, got none")
	}
}

func TestFileChurnNotTriggeredBelow5(t *testing.T) {
	msgs := make([]string, 4)
	filesets := make([][]string, 4)
	for i := 0; i < 4; i++ {
		msgs[i] = fmt.Sprintf("change %d", i)
		filesets[i] = []string{"warm_file.go"}
	}

	dir := setupRepoWithCommits(t, msgs, filesets)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	for _, s := range signals {
		if s.Type == "file_churn" {
			t.Error("expected no file_churn signal for only 4 commits touching same file")
		}
	}
}

func TestPendulumDetected(t *testing.T) {
	dir := setupRepoWithCommits(t,
		[]string{"fix the build errors", "update the readme", "fix the build warnings"},
		nil,
	)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	found := false
	for _, s := range signals {
		if s.Type == "pendulum" {
			found = true
		}
	}
	if !found {
		t.Error("expected pendulum signal, got none")
	}
}

func TestCleanHistory(t *testing.T) {
	dir := setupRepoWithCommits(t,
		[]string{"feat: add auth", "refactor: extract utils", "docs: update readme", "test: add unit tests"},
		nil,
	)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	if len(signals) != 0 {
		t.Errorf("expected no signals for clean history, got %v", signals)
	}
}

func TestEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Run()

	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	if signals != nil {
		t.Errorf("expected nil signals for empty repo, got %v", signals)
	}
}

func TestMixedSignals(t *testing.T) {
	// 5 commits all touching same file + 3 consecutive fix messages = both signals.
	msgs := []string{"fix: a", "fix: b", "fix: c", "fix: d", "fix: e"}
	filesets := [][]string{
		{"shared.go"}, {"shared.go"}, {"shared.go"}, {"shared.go"}, {"shared.go"},
	}

	dir := setupRepoWithCommits(t, msgs, filesets)
	d := NewRepoLoopDetector()
	signals := d.Detect(context.Background(), dir)

	foundFixChain := false
	foundFileChurn := false
	for _, s := range signals {
		if s.Type == "fix_chain" {
			foundFixChain = true
		}
		if s.Type == "file_churn" {
			foundFileChurn = true
		}
	}
	if !foundFixChain {
		t.Error("expected fix_chain signal in mixed scenario")
	}
	if !foundFileChurn {
		t.Error("expected file_churn signal in mixed scenario")
	}
}
