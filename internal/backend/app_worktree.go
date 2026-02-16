// Package backend provides git worktree management for parallel issue work.
// Each issue gets its own worktree directory with an isolated branch,
// allowing multiple Claude agents to work on different issues simultaneously.
package backend

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const worktreeDir = ".mt-worktrees"

// WorktreeInfo describes an active git worktree for an issue.
type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Issue  int    `json:"issue"`
}

// worktreePath returns the directory for an issue worktree.
func worktreePath(repoDir string, issueNumber int) string {
	return filepath.Join(repoDir, worktreeDir, fmt.Sprintf("issue-%d", issueNumber))
}

// repoRoot returns the git toplevel for a directory.
func repoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateWorktree creates a git worktree for an issue with its own branch.
// Returns the worktree path and branch name.
func (a *App) CreateWorktree(dir string, issueNumber int, title string) (*WorktreeInfo, error) {
	root, err := repoRoot(dir)
	if err != nil {
		return nil, err
	}

	branch := issueBranchName(issueNumber, title)
	wtPath := worktreePath(root, issueNumber)

	// Check if worktree already exists
	if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
		log.Printf("[CreateWorktree] worktree already exists at %s", wtPath)
		return &WorktreeInfo{Path: wtPath, Branch: branch, Issue: issueNumber}, nil
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(wtPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir failed: %w", err)
	}

	// Create worktree with new branch (or existing branch)
	var cmd *exec.Cmd
	if branchExists(root, branch) {
		cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	} else {
		cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	}
	cmd.Dir = root
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("worktree add failed: %s – %w", strings.TrimSpace(string(out)), err)
	}

	log.Printf("[CreateWorktree] created worktree at %s on branch %s", wtPath, branch)
	return &WorktreeInfo{Path: wtPath, Branch: branch, Issue: issueNumber}, nil
}

// RemoveWorktree removes a worktree for an issue and optionally its branch.
func (a *App) RemoveWorktree(dir string, issueNumber int) error {
	root, err := repoRoot(dir)
	if err != nil {
		return err
	}

	wtPath := worktreePath(root, issueNumber)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil // already gone
	}

	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = root
	hideConsole(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("worktree remove failed: %w", err)
	}

	log.Printf("[RemoveWorktree] removed worktree at %s", wtPath)
	return nil
}

// ListWorktrees returns all active worktrees that belong to Multiterminal.
func (a *App) ListWorktrees(dir string) []WorktreeInfo {
	root, err := repoRoot(dir)
	if err != nil {
		return nil
	}

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = root
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[ListWorktrees] error: %v", err)
		return nil
	}

	return parseWorktreeList(string(out), root)
}

// parseWorktreeList extracts Multiterminal worktrees from git worktree list output.
func parseWorktreeList(output string, root string) []WorktreeInfo {
	var result []WorktreeInfo
	var current WorktreeInfo
	wtPrefix := filepath.Join(root, worktreeDir, "issue-")

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" && strings.HasPrefix(current.Path, wtPrefix) {
				result = append(result, current)
			}
			current = WorktreeInfo{}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		}
		if strings.HasPrefix(line, "branch refs/heads/") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	// Don't forget last entry
	if current.Path != "" && strings.HasPrefix(current.Path, wtPrefix) {
		result = append(result, current)
	}

	// Extract issue numbers from paths
	for i := range result {
		base := filepath.Base(result[i].Path)
		if strings.HasPrefix(base, "issue-") {
			num, _ := strconv.Atoi(strings.TrimPrefix(base, "issue-"))
			result[i].Issue = num
		}
	}

	return result
}
