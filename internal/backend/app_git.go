package backend

import (
	"os"
	"os/exec"
	"path/filepath"
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

// GitFileStatus represents the git status of a single file.
// Status values: "M" = modified, "A" = added/staged, "?" = untracked/new,
// "D" = deleted, "R" = renamed, "" = unchanged.
type GitFileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// GetGitFileStatuses returns a map of relative file paths to their git status
// for the given directory. Uses `git status --porcelain` for parsing.
func (a *App) GetGitFileStatuses(dir string) map[string]string {
	result := make(map[string]string)
	if dir == "" {
		return result
	}

	// Get git repo root to compute relative paths correctly
	rootCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	rootCmd.Dir = dir
	hideConsole(rootCmd)
	rootOut, err := rootCmd.Output()
	if err != nil {
		return result
	}
	repoRoot := filepath.FromSlash(strings.TrimSpace(string(rootOut)))
	if resolved, err := filepath.EvalSymlinks(repoRoot); err == nil {
		repoRoot = resolved
	}

	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return result
	}

	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		xy := line[:2]
		relPath := strings.TrimSpace(line[3:])

		// Handle renames: "R  old -> new"
		if idx := strings.Index(relPath, " -> "); idx >= 0 {
			relPath = relPath[idx+4:]
		}

		// Convert to absolute path then make relative to browsed dir
		absPath := filepath.Join(repoRoot, relPath)
		status := classifyGitStatus(xy)
		if status != "" {
			result[absPath] = status
			// Also mark parent directories as modified
			markParentDirs(result, absPath, dir)
		}
	}

	return result
}

// classifyGitStatus converts porcelain XY codes to a simple status string.
func classifyGitStatus(xy string) string {
	x, y := xy[0], xy[1]
	if x == '?' || y == '?' {
		return "?"
	}
	if x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D') {
		return "U"
	}
	if x == 'A' || y == 'A' {
		return "A"
	}
	if x == 'D' || y == 'D' {
		return "D"
	}
	if x == 'R' || y == 'R' {
		return "R"
	}
	if x == 'M' || y == 'M' {
		return "M"
	}
	return ""
}

// MergeConflictInfo holds information about current merge conflicts.
type MergeConflictInfo struct {
	Files     []string `json:"files"`
	Operation string   `json:"operation"` // "merge", "rebase", "cherry-pick", ""
	Count     int      `json:"count"`
}

// GetMergeConflicts returns conflict information for the given directory.
func (a *App) GetMergeConflicts(dir string) MergeConflictInfo {
	info := MergeConflictInfo{Files: []string{}}
	if dir == "" {
		return info
	}

	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return info
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			info.Files = append(info.Files, line)
		}
	}
	info.Count = len(info.Files)

	if info.Count > 0 {
		info.Operation = detectMergeOperation(dir)
	}
	return info
}

// detectMergeOperation checks git sentinel files to determine the active operation.
func detectMergeOperation(dir string) string {
	// Find .git directory
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}

	if fileExists(filepath.Join(gitDir, "CHERRY_PICK_HEAD")) {
		return "cherry-pick"
	}
	if fileExists(filepath.Join(gitDir, "REBASE_HEAD")) || dirExists(filepath.Join(gitDir, "rebase-merge")) || dirExists(filepath.Join(gitDir, "rebase-apply")) {
		return "rebase"
	}
	if fileExists(filepath.Join(gitDir, "MERGE_HEAD")) {
		return "merge"
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// markParentDirs marks all parent directories up to root as modified.
func markParentDirs(statuses map[string]string, filePath string, rootDir string) {
	dir := filepath.Dir(filePath)
	absRoot := filepath.Clean(rootDir)
	for dir != absRoot && len(dir) > len(absRoot) {
		if _, exists := statuses[dir]; !exists {
			statuses[dir] = "M"
		}
		dir = filepath.Dir(dir)
	}
}
