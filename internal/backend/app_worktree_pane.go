// Package backend – per-pane git worktrees (sibling directory) with a
// deterministic finish flow (ff-only merge into the target branch + cleanup).
package backend

import (
	"fmt"
	"os"
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

// paneWorktreeBase returns the sibling directory that holds all pane
// worktrees for a repo: <parent>/<repo-name>.mt-worktrees
// Sibling (not in-repo) so builds, watchers and CLAUDE.md discovery in the
// main repo never see worktree contents (spec 3.1).
func paneWorktreeBase(mainRoot string) string {
	return filepath.Join(filepath.Dir(mainRoot), filepath.Base(mainRoot)+".mt-worktrees")
}

// findFreePaneName sanitizes base and appends -2, -3, … until neither the
// branch terminal/<name> nor the sibling directory exists. Default names like
// pane-3 would otherwise collide with leftover branches every launch (spec 3.3/2).
func findFreePaneName(mainRoot, base string) string {
	name := sanitizeWorktreeName(base)
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", name, i)
		}
		if branchExists(mainRoot, "terminal/"+candidate) {
			continue
		}
		if _, err := os.Stat(filepath.Join(paneWorktreeBase(mainRoot), candidate)); err == nil {
			continue
		}
		return candidate
	}
}
