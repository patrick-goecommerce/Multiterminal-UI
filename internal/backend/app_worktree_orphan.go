// Manual cleanup for worktrees Claude created via EnterWorktree but never
// removed itself (pane closed, session died, Claude simply moved on). This is
// a deliberately simple admin action, not the full verified finish-flow of
// the old design (spec 2026-07-03 section 6): remove the worktree directory,
// then delete the branch with -d only — an unmerged branch survives so no
// committed work is silently lost.
package backend

import (
	"fmt"
	"strings"
)

// RemoveOrphanedWorktree removes a worktree directory and, if safe, its
// branch. Never uses --force or -D: an unmerged branch or a worktree with
// uncommitted changes causes an error instead of data loss.
func (a *AppService) RemoveOrphanedWorktree(path string) error {
	root, err := mainRepoRoot(path)
	if err != nil {
		return err
	}
	branch := checkedOutBranch(path)

	if out, err := gitCmd(root, "worktree", "remove", path).CombinedOutput(); err != nil {
		return fmt.Errorf("worktree remove fehlgeschlagen: %s – %w", strings.TrimSpace(string(out)), err)
	}
	_ = gitCmd(root, "worktree", "prune").Run()

	if branch == "" {
		return nil
	}
	if out, err := gitCmd(root, "branch", "-d", branch).CombinedOutput(); err != nil {
		return fmt.Errorf("Worktree entfernt, Branch %q aber nicht gelöscht (nicht gemergt): %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}
