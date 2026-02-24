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

// WorktreeInfo describes an active git worktree.
type WorktreeInfo struct {
	Path     string `json:"path" yaml:"path"`
	Branch   string `json:"branch" yaml:"branch"`
	Issue    int    `json:"issue" yaml:"issue"`
	Category string `json:"category" yaml:"category"`
	Name     string `json:"name" yaml:"name"`
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
	return filepath.FromSlash(strings.TrimSpace(string(out))), nil
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

// parseWorktreePorcelain returns raw WorktreeInfo entries from git --porcelain output.
// Only Path and Branch are populated.
func parseWorktreePorcelain(output string) []WorktreeInfo {
	var result []WorktreeInfo
	var current WorktreeInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				result = append(result, current)
			}
			current = WorktreeInfo{}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.Path = filepath.FromSlash(strings.TrimPrefix(line, "worktree "))
		}
		if strings.HasPrefix(line, "branch refs/heads/") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
		if line == "detached" {
			current.Branch = "(detached)"
		}
	}
	if current.Path != "" {
		result = append(result, current)
	}
	return result
}

// parseWorktreeList extracts Multiterminal issue worktrees from git worktree list output.
func parseWorktreeList(output string, root string) []WorktreeInfo {
	var result []WorktreeInfo
	wtPrefix := filepath.Join(root, worktreeDir, "issue-")
	for _, wt := range parseWorktreePorcelain(output) {
		if strings.HasPrefix(strings.ToLower(wt.Path), strings.ToLower(wtPrefix)) {
			base := filepath.Base(wt.Path)
			num, _ := strconv.Atoi(strings.TrimPrefix(base, "issue-"))
			wt.Issue = num
			result = append(result, wt)
		}
	}
	return result
}

// ListAllWorktrees returns ALL git worktrees categorized as "main", "terminal", or "issue".
func (a *App) ListAllWorktrees(dir string) []WorktreeInfo {
	root, err := repoRoot(dir)
	if err != nil {
		return nil
	}
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = root
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[ListAllWorktrees] error: %v", err)
		return nil
	}
	return parseAllWorktreeList(string(out), root)
}

// parseAllWorktreeList parses ALL worktrees without filtering.
// root is expected in OS-native path format (as returned by repoRoot).
func parseAllWorktreeList(output string, root string) []WorktreeInfo {
	root = filepath.FromSlash(root)
	mtPrefix := filepath.Join(root, worktreeDir) + string(filepath.Separator)
	entries := parseWorktreePorcelain(output)
	result := make([]WorktreeInfo, 0, len(entries))
	for i := range entries {
		categorizeWorktree(&entries[i], root, mtPrefix)
		result = append(result, entries[i])
	}
	return result
}

// categorizeWorktree fills Category, Name, Issue based on path.
// Uses case-insensitive path comparison for Windows compatibility.
func categorizeWorktree(wt *WorktreeInfo, root, mtPrefix string) {
	// Normalize for case-insensitive comparison on Windows
	wtPathNorm := strings.ToLower(filepath.Clean(wt.Path))
	rootNorm := strings.ToLower(filepath.Clean(root))
	mtPrefixNorm := strings.ToLower(mtPrefix)

	if wtPathNorm == rootNorm {
		wt.Category = "main"
		wt.Name = "main"
		return
	}
	if strings.HasPrefix(wtPathNorm, mtPrefixNorm) {
		base := filepath.Base(wt.Path)
		if strings.HasPrefix(base, "issue-") {
			wt.Category = "issue"
			num, _ := strconv.Atoi(strings.TrimPrefix(base, "issue-"))
			wt.Issue = num
			wt.Name = base
		} else {
			wt.Category = "terminal"
			wt.Name = base
		}
		return
	}
	wt.Category = "terminal"
	wt.Name = filepath.Base(wt.Path)
}
