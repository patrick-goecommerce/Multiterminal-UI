// Hard verification gate before a worktree finish: all checks run against the
// LOCAL target ref — the same ref the merge later uses (spec 5.3).
package backend

import (
	"fmt"
	"strconv"
	"strings"
)

// WorktreeFinishStatus is the result of the pre-merge verification.
type WorktreeFinishStatus struct {
	State     string   `json:"state" yaml:"state"` // "ready" | "cleanup_only" | "blocked"
	Reason    string   `json:"reason,omitempty" yaml:"reason,omitempty"`
	Commits   []string `json:"commits,omitempty" yaml:"commits,omitempty"`
	Stat      string   `json:"stat,omitempty" yaml:"stat,omitempty"`
	Untracked []string `json:"untracked,omitempty" yaml:"untracked,omitempty"`
}

func revCount(root, from, to string) (int, error) {
	out, err := gitCmd(root, "rev-list", "--count", from+".."+to).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func isAncestor(root, anc, desc string) bool {
	return gitCmd(root, "merge-base", "--is-ancestor", anc, desc).Run() == nil
}

func checkedOutBranch(root string) string {
	out, err := gitCmd(root, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// trackedDirty reports whether tracked files have modifications (untracked
// files are deliberately ignored — spec 5.3/3, untracked-artifact deadlock).
func trackedDirty(dir string) bool {
	out, err := gitCmd(dir, "status", "--porcelain", "--untracked-files=no").Output()
	return err != nil || len(strings.TrimSpace(string(out))) > 0
}

func untrackedFiles(dir string) []string {
	out, err := gitCmd(dir, "status", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "?? ") {
			files = append(files, strings.TrimPrefix(line, "?? "))
		}
	}
	return files
}

func blocked(reason string) WorktreeFinishStatus {
	return WorktreeFinishStatus{State: "blocked", Reason: reason}
}

// GetWorktreeFinishStatus verifies a pane worktree is mergeable into target.
func (a *AppService) GetWorktreeFinishStatus(worktreePath, branch, target string) WorktreeFinishStatus {
	root, err := mainRepoRoot(worktreePath)
	if err != nil {
		return blocked(err.Error())
	}
	count, err := revCount(root, target, branch)
	if err != nil {
		return blocked(fmt.Sprintf("git rev-list fehlgeschlagen: %v", err))
	}
	if count == 0 {
		// Branch fully contained in target: either never worked, or a crash
		// happened after merge but before the marker — both end in a safe
		// cleanup instead of a deadlock (spec 5.3/1, red-team G2-K2).
		// EXCEPT: tracked uncommitted changes must never reach the --force
		// cleanup path — that would silently destroy work.
		if trackedDirty(worktreePath) {
			return blocked("Uncommittete Änderungen im Worktree — committen oder verwerfen, bevor aufgeräumt wird")
		}
		return WorktreeFinishStatus{State: "cleanup_only", Untracked: untrackedFiles(worktreePath)}
	}
	if !isAncestor(root, target, branch) {
		return blocked(fmt.Sprintf("Branch ist nicht auf %s rebased — erneut vorbereiten", target))
	}
	if trackedDirty(worktreePath) {
		return blocked("Uncommittete Änderungen im Worktree")
	}
	if got := checkedOutBranch(root); got != target {
		return blocked(fmt.Sprintf("Im Haupt-Repo ist %q ausgecheckt, nicht der Ziel-Branch %q — paralleles Finishen geht nur auf denselben Ziel-Branch", got, target))
	}
	if trackedDirty(root) {
		return blocked("Das Haupt-Repo hat uncommittete Änderungen — der Merge würde Dateien dort bewegen")
	}
	logOut, _ := gitCmd(root, "log", "--oneline", target+".."+branch).Output()
	var commits []string
	for _, l := range strings.Split(strings.TrimSpace(string(logOut)), "\n") {
		if l != "" {
			commits = append(commits, l)
		}
	}
	statOut, _ := gitCmd(root, "diff", "--stat", target+"..."+branch).Output()
	return WorktreeFinishStatus{
		State:     "ready",
		Commits:   commits,
		Stat:      strings.TrimSpace(string(statOut)),
		Untracked: untrackedFiles(worktreePath),
	}
}
