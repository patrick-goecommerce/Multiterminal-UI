// Mechanical finish for shell panes: MTUI itself commits (SELECTED files only
// — never add -A, spec 5.5) and rebases; conflicts are reported, never
// auto-resolved.
package backend

import (
	"fmt"
	"strings"
)

// WorktreeFileChange is one changed/untracked file for the staging dialog.
type WorktreeFileChange struct {
	Path   string `json:"path" yaml:"path"`
	Status string `json:"status" yaml:"status"`
}

// GetWorktreeChangedFiles lists modified + untracked files (porcelain).
func (a *AppService) GetWorktreeChangedFiles(path string) []WorktreeFileChange {
	out, err := gitCmd(path, "status", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var changes []WorktreeFileChange
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		changes = append(changes, WorktreeFileChange{
			Status: strings.TrimSpace(line[:2]),
			Path:   strings.TrimSpace(line[3:]),
		})
	}
	return changes
}

// CommitWorktreeFiles stages exactly the given files and commits.
func (a *AppService) CommitWorktreeFiles(path string, files []string, message string) error {
	if len(files) == 0 {
		return fmt.Errorf("keine Dateien ausgewählt")
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("Commit-Message fehlt")
	}
	args := append([]string{"add", "--"}, files...)
	if out, err := gitCmd(path, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s – %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := gitCmd(path, "commit", "-m", message).CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RebaseWorktreeOntoTarget rebases the worktree branch onto the LOCAL target.
// On conflict the rebase stays in progress; the caller offers abort/manual.
func (a *AppService) RebaseWorktreeOntoTarget(path, target string) error {
	if out, err := gitCmd(path, "rebase", target).CombinedOutput(); err != nil {
		return fmt.Errorf("rebase auf %s fehlgeschlagen (Konflikt?): %s", target, strings.TrimSpace(string(out)))
	}
	return nil
}

// AbortWorktreeRebase aborts an in-progress rebase.
func (a *AppService) AbortWorktreeRebase(path string) error {
	if out, err := gitCmd(path, "rebase", "--abort").CombinedOutput(); err != nil {
		return fmt.Errorf("rebase --abort: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
