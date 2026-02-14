package backend

import (
	"os/exec"
	"strconv"
	"strings"
)

// GetGitBranch returns the current git branch for the given directory.
func (a *App) GetGitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetLastCommitTime returns the Unix timestamp (seconds) of the last git commit.
func (a *App) GetLastCommitTime(dir string) int64 {
	cmd := exec.Command("git", "log", "-1", "--format=%ct")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return ts
}
