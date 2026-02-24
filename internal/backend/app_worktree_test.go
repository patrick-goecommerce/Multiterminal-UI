package backend

import (
	"strings"
	"testing"
)

func TestParseAllWorktrees(t *testing.T) {
	root := "/repo"
	output := "worktree /repo\nHEAD abc1234\nbranch refs/heads/main\n\nworktree /repo/.mt-worktrees/issue-42\nHEAD def5678\nbranch refs/heads/fix/bug-42\n\nworktree /repo/.mt-worktrees/login\nHEAD aaa9999\nbranch refs/heads/terminal/login\n\n"
	result := parseAllWorktreeList(output, root)
	if len(result) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(result))
	}
	if result[0].Category != "main" {
		t.Errorf("expected main, got %s", result[0].Category)
	}
	if result[1].Category != "issue" || result[1].Issue != 42 {
		t.Errorf("expected issue 42, got %+v", result[1])
	}
	if result[2].Category != "terminal" || result[2].Name != "login" {
		t.Errorf("expected terminal/login, got %+v", result[2])
	}
	if !strings.HasSuffix(result[2].Branch, "terminal/login") {
		t.Errorf("unexpected branch: %s", result[2].Branch)
	}
}
