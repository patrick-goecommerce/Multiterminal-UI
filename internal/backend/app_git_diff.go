package backend

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DiffFileStat represents diff statistics for a single file.
type DiffFileStat struct {
	Path       string `json:"path" yaml:"path"`
	Status     string `json:"status" yaml:"status"`
	Insertions int    `json:"insertions" yaml:"insertions"`
	Deletions  int    `json:"deletions" yaml:"deletions"`
}

// CommitSuggestion holds an auto-generated conventional commit message.
type CommitSuggestion struct {
	Type        string `json:"type" yaml:"type"`
	Scope       string `json:"scope" yaml:"scope"`
	Description string `json:"description" yaml:"description"`
	Full        string `json:"full" yaml:"full"`
}

// GetDiffStats returns per-file diff statistics for the working tree vs HEAD.
func (a *AppService) GetDiffStats(dir string) []DiffFileStat {
	var result []DiffFileStat

	// Get numstat for tracked files
	cmd := exec.Command("git", "diff", "--numstat", "HEAD")
	cmd.Dir = dir
	hideConsole(cmd)
	out, _ := cmd.Output()

	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		ins, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		path := parts[2]
		result = append(result, DiffFileStat{
			Path:       path,
			Status:     "M",
			Insertions: ins,
			Deletions:  del,
		})
		seen[path] = true
	}

	// Enrich status from porcelain output
	statuses := a.GetGitFileStatuses(dir)
	for i := range result {
		for absPath, st := range statuses {
			if filepath.Base(absPath) == filepath.Base(result[i].Path) &&
				strings.HasSuffix(filepath.ToSlash(absPath), result[i].Path) {
				result[i].Status = st
				break
			}
		}
	}

	// Add untracked files (not in numstat)
	for absPath, st := range statuses {
		if st != "?" {
			continue
		}
		// Convert absolute path to relative using repo root
		rootCmd := exec.Command("git", "rev-parse", "--show-toplevel")
		rootCmd.Dir = dir
		hideConsole(rootCmd)
		rootOut, err := rootCmd.Output()
		if err != nil {
			continue
		}
		root := filepath.FromSlash(strings.TrimSpace(string(rootOut)))
		rel, err := filepath.Rel(root, absPath)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		result = append(result, DiffFileStat{
			Path:   rel,
			Status: "?",
		})
		seen[rel] = true
	}

	return result
}

// GetFileDiff returns the diff output for a single file against HEAD.
func (a *AppService) GetFileDiff(dir, path string) string {
	cmd := exec.Command("git", "diff", "HEAD", "--", path)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		// Fallback for untracked files
		cmd2 := exec.Command("git", "diff", "--no-index", "/dev/null", path)
		cmd2.Dir = dir
		hideConsole(cmd2)
		out2, _ := cmd2.Output()
		return string(out2)
	}
	return string(out)
}

// GetWorkingDiff returns the full diff of the working tree against HEAD.
func (a *AppService) GetWorkingDiff(dir string) string {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = dir
	hideConsole(cmd)
	out, _ := cmd.Output()
	return string(out)
}

// StageAndCommit stages all changes and commits with the given message.
func (a *AppService) StageAndCommit(dir, message string) error {
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = dir
	hideConsole(addCmd)
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = dir
	hideConsole(commitCmd)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// StageFiles stages only the specified paths.
func (a *AppService) StageFiles(dir string, paths []string) error {
	args := append([]string{"add", "--"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	hideConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CommitStaged commits only the currently staged changes.
func (a *AppService) CommitStaged(dir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	hideConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PushBranch pushes the current branch to origin.
func (a *AppService) PushBranch(dir string) error {
	branch := a.GetGitBranch(dir)
	if branch == "" {
		return fmt.Errorf("cannot determine current branch")
	}
	cmd := exec.Command("git", "push", "-u", "origin", branch)
	cmd.Dir = dir
	hideConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// GenerateCommitSuggestion analyzes the given paths and returns a suggestion.
func (a *AppService) GenerateCommitSuggestion(dir string, paths []string) CommitSuggestion {
	// Build stats from the selected paths
	allStats := a.GetDiffStats(dir)
	var filtered []DiffFileStat
	if len(paths) == 0 {
		filtered = allStats
	} else {
		pathSet := make(map[string]bool)
		for _, p := range paths {
			pathSet[p] = true
		}
		for _, s := range allStats {
			if pathSet[s.Path] {
				filtered = append(filtered, s)
			}
		}
	}
	if len(filtered) == 0 {
		return CommitSuggestion{Type: "chore", Description: "update files"}
	}

	typ := inferCommitType(filtered)
	scope := inferScope(filtered)
	desc := inferDescription(filtered)

	full := typ
	if scope != "" {
		full += "(" + scope + ")"
	}
	full += ": " + desc

	return CommitSuggestion{
		Type:        typ,
		Scope:       scope,
		Description: desc,
		Full:        full,
	}
}
