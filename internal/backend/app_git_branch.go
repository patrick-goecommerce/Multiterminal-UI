// Package backend provides automatic branch creation for GitHub issues.
package backend

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// slugifyTitle converts an issue title to a URL-safe branch slug.
// "Fix login bug!" → "fix-login-bug"
func slugifyTitle(title string) string {
	// Normalize unicode and lowercase
	s := strings.ToLower(norm.NFKD.String(title))

	// Remove non-ASCII characters (accents etc.)
	var buf strings.Builder
	for _, r := range s {
		if r < 128 {
			buf.WriteRune(r)
		}
	}
	s = buf.String()

	// Replace non-alphanumeric with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	// Truncate to reasonable length for branch names
	if len(s) > 40 {
		s = s[:40]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// issueBranchName generates a branch name for an issue.
// Format: issue/<number>-<slug>
func issueBranchName(number int, title string) string {
	slug := slugifyTitle(title)
	if slug == "" {
		return fmt.Sprintf("issue/%d", number)
	}
	return fmt.Sprintf("issue/%d-%s", number, slug)
}

// isGitRepo checks whether dir is inside a git repository.
func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// hasCleanWorkingTree returns true if there are no uncommitted changes.
func hasCleanWorkingTree(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == ""
}

// branchExists checks if a git branch with the given name exists locally.
func branchExists(dir string, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = dir
	hideConsole(cmd)
	return cmd.Run() == nil
}

// GetOrCreateIssueBranch checks if an issue branch exists, creates it if not,
// and switches to it. Returns the branch name or an error.
// If the working tree is dirty, it returns an error rather than risking data loss.
func (a *App) GetOrCreateIssueBranch(dir string, number int, title string) (string, error) {
	if dir == "" || !isGitRepo(dir) {
		return "", fmt.Errorf("not a git repository")
	}

	branch := issueBranchName(number, title)

	// Check if already on this branch
	currentBranch := a.GetGitBranch(dir)
	if currentBranch == branch {
		return branch, nil
	}

	// Check for dirty working tree
	if !hasCleanWorkingTree(dir) {
		return "", fmt.Errorf("uncommitted changes — bitte zuerst committen oder stashen")
	}

	if branchExists(dir, branch) {
		// Switch to existing branch
		cmd := exec.Command("git", "checkout", branch)
		cmd.Dir = dir
		hideConsole(cmd)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("checkout failed: %w", err)
		}
		log.Printf("[GetOrCreateIssueBranch] switched to existing branch %s", branch)
	} else {
		// Create and switch to new branch
		cmd := exec.Command("git", "checkout", "-b", branch)
		cmd.Dir = dir
		hideConsole(cmd)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("branch create failed: %w", err)
		}
		log.Printf("[GetOrCreateIssueBranch] created new branch %s", branch)
	}

	return branch, nil
}

// IsGitRepo checks if the given directory is a git repository (exported for frontend).
func (a *App) IsGitRepo(dir string) bool {
	return isGitRepo(dir)
}

// HasCleanWorkingTree checks for uncommitted changes (exported for frontend).
func (a *App) HasCleanWorkingTree(dir string) bool {
	return hasCleanWorkingTree(dir)
}
