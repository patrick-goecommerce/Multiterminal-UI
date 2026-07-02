// Package backend – per-pane git worktrees (sibling directory) with a
// deterministic finish flow (ff-only merge into the target branch + cleanup).
package backend

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// mainRepoRoot returns the absolute path of the MAIN worktree for any dir
// inside the repo (main checkout or linked worktree).
// It relies on the git guarantee that `git worktree list --porcelain` always
// lists the main worktree first. --show-toplevel (worktree path) and
// --git-common-dir (relative ".git" in the main repo) are both unsuitable.
func mainRepoRoot(dir string) (string, error) {
	out, err := gitCmd(dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	entries := parseWorktreePorcelain(string(out))
	if len(entries) == 0 || entries[0].Path == "" {
		return "", fmt.Errorf("no worktrees found in %s", dir)
	}
	return filepath.FromSlash(entries[0].Path), nil
}

// paneWorktreeBase returns the sibling directory that holds all pane
// worktrees for a repo: <parent>/<repo-name>.mt-worktrees
// Sibling (not in-repo) so builds, watchers and CLAUDE.md discovery in the
// main repo never see worktree contents (spec 3.1).
func paneWorktreeBase(mainRoot string) string {
	return filepath.Join(filepath.Dir(mainRoot), filepath.Base(mainRoot)+".mt-worktrees")
}

// findFreePaneName sanitizes base and appends -2, -3, … until neither the
// branch terminal/<name> nor the sibling directory exists. Default names like
// pane-3 would otherwise collide with leftover branches every launch (spec 3.3/2).
func findFreePaneName(mainRoot, base string) string {
	name := sanitizeWorktreeName(base)
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", name, i)
		}
		if branchExists(mainRoot, "terminal/"+candidate) {
			continue
		}
		if _, err := os.Stat(filepath.Join(paneWorktreeBase(mainRoot), candidate)); err == nil {
			continue
		}
		return candidate
	}
}

// PaneWorktreeInfo describes a pane worktree returned to the frontend.
type PaneWorktreeInfo struct {
	Path         string `json:"path" yaml:"path"`
	Branch       string `json:"branch" yaml:"branch"`
	TargetBranch string `json:"target_branch" yaml:"target_branch"`
}

// PaneWorktreeDefaults prefills the launch dialog fields.
type PaneWorktreeDefaults struct {
	Name         string `json:"name" yaml:"name"`
	TargetBranch string `json:"target_branch" yaml:"target_branch"`
}

// GetPaneWorktreeDefaults returns a collision-free name and the branch
// currently checked out in the MAIN worktree ("" on detached HEAD — the
// dialog then forces an explicit choice, spec 3.3/3).
func (a *AppService) GetPaneWorktreeDefaults(dir, base string) PaneWorktreeDefaults {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return PaneWorktreeDefaults{Name: sanitizeWorktreeName(base)}
	}
	target := ""
	if out, err := gitCmd(root, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		if b := strings.TrimSpace(string(out)); b != "HEAD" {
			target = b
		}
	}
	return PaneWorktreeDefaults{Name: findFreePaneName(root, base), TargetBranch: target}
}

// CreatePaneWorktree creates the sibling worktree with branch terminal/<name>
// forked from targetBranch and writes the control files.
func (a *AppService) CreatePaneWorktree(dir, name, targetBranch string) (*PaneWorktreeInfo, error) {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return nil, err
	}
	if targetBranch == "" {
		return nil, fmt.Errorf("Ziel-Branch fehlt")
	}
	if !branchExists(root, targetBranch) {
		return nil, fmt.Errorf("Ziel-Branch %q existiert lokal nicht", targetBranch)
	}
	safe := sanitizeWorktreeName(name)
	branch := "terminal/" + safe
	wtPath := filepath.Join(paneWorktreeBase(root), safe)
	if branchExists(root, branch) {
		return nil, fmt.Errorf("Branch %q existiert bereits – bitte anderen Namen wählen", branch)
	}
	if _, err := os.Stat(wtPath); err == nil {
		return nil, fmt.Errorf("Verzeichnis %q existiert bereits – bitte anderen Namen wählen", wtPath)
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return nil, fmt.Errorf("Worktree-Basisverzeichnis nicht anlegbar (Repo-Parent schreibgeschützt?): %w", err)
	}
	out, err := gitCmd(root, "worktree", "add", "-b", branch, wtPath, targetBranch).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("worktree add failed: %s – %w", strings.TrimSpace(string(out)), err)
	}
	if err := writeWorktreeControlFiles(wtPath, root, branch, targetBranch); err != nil {
		log.Printf("[CreatePaneWorktree] control files: %v", err) // non-fatal
	}
	log.Printf("[CreatePaneWorktree] %s on %s (target %s)", wtPath, branch, targetBranch)
	return &PaneWorktreeInfo{Path: wtPath, Branch: branch, TargetBranch: targetBranch}, nil
}
