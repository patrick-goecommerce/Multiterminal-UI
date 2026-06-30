package main

import (
	"os"
	"path/filepath"
	"strings"
)

// gitBranch walks up from startDir to find the first .git entry, then reads
// HEAD to determine the current branch name. Returns "" for detached HEAD,
// worktree errors, or any directory that is not inside a git repo.
// Never spawns a subprocess.
func gitBranch(startDir string) string {
	headPath, err := findGitHEAD(startDir)
	if err != nil {
		return ""
	}
	return parseHEAD(headPath)
}

// findGitHEAD walks up the directory tree from dir until it locates a .git
// entry. If .git is a directory it returns <dir>/.git/HEAD. If .git is a
// file (worktree / submodule) it parses the gitdir: indirection and returns
// <gitdir>/HEAD.
func findGitHEAD(dir string) (string, error) {
	dir = filepath.Clean(dir)
	for {
		gitEntry := filepath.Join(dir, ".git")
		info, err := os.Lstat(gitEntry)
		if err == nil {
			if info.IsDir() {
				return filepath.Join(gitEntry, "HEAD"), nil
			}
			// .git is a file — worktree / submodule indirection
			return resolveGitFile(gitEntry)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// reached filesystem root without finding .git
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// resolveGitFile reads a .git FILE (not directory) and resolves the
// "gitdir: <path>" directive to return the path of the real HEAD file.
func resolveGitFile(gitFilePath string) (string, error) {
	data, err := os.ReadFile(gitFilePath)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir: "
	if !strings.HasPrefix(line, prefix) {
		return "", os.ErrInvalid
	}
	gitdir := strings.TrimPrefix(line, prefix)
	if !filepath.IsAbs(gitdir) {
		// resolve relative path against the directory containing the .git file
		gitdir = filepath.Join(filepath.Dir(gitFilePath), gitdir)
	}
	return filepath.Join(gitdir, "HEAD"), nil
}

// parseHEAD reads the HEAD file at headPath and extracts the branch name.
// Returns "" for detached HEAD or any read/parse error.
func parseHEAD(headPath string) string {
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	const refPrefix = "ref: refs/heads/"
	if !strings.HasPrefix(line, refPrefix) {
		return ""
	}
	return strings.TrimPrefix(line, refPrefix)
}
