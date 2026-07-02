// Package backend – per-pane git worktrees (sibling directory) with a
// deterministic finish flow (ff-only merge into the target branch + cleanup).
package backend

import (
	"fmt"
	"path/filepath"
)

// mainRepoRoot returns the absolute path of the MAIN worktree for any dir
// inside the repo (main checkout or linked worktree).
// It relies on the git guarantee that `git worktree list --porcelain` always
// lists the main worktree first. --show-toplevel (worktree path) and
// --git-common-dir (relative ".git" in the main repo) are both unsuitable.
func mainRepoRoot(dir string) (string, error) {
	out, err := gitCmd(dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	entries := parseWorktreePorcelain(string(out))
	if len(entries) == 0 || entries[0].Path == "" {
		return "", fmt.Errorf("no worktrees found in %s", dir)
	}
	return filepath.FromSlash(entries[0].Path), nil
}
