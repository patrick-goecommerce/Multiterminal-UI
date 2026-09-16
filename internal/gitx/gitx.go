// Package gitx is the git plumbing MTUI shares between processes.
//
// It exists because the GUI and the session daemon both have to answer the
// same two questions before a session can start ("which repo is this" and "is
// this a linked worktree"), and because every git call MTUI makes needs the
// same treatment: no optional locks, no credential prompt, and no console
// window on Windows. A second copy of that setup in the daemon would drift,
// and the drift would only show as a hang on a repo waiting for a password.
package gitx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/procs"
)

// Cmd builds a git command for dir.
//
// The flags are not decoration. --no-optional-locks and GIT_OPTIONAL_LOCKS=0
// keep a status call from fighting the user's own git for the index lock;
// core.fsmonitor=false avoids starting a watcher process per call;
// GIT_TERMINAL_PROMPT and GCM_INTERACTIVE turn a credential prompt into an
// error instead of a process that waits forever on a terminal nobody is
// looking at. HideConsole is what keeps a GUI app from flashing a console
// window on every call.
func Cmd(dir string, args ...string) *exec.Cmd {
	full := append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	procs.HideConsole(cmd)
	return cmd
}

// Worktree is one entry of `git worktree list --porcelain`.
type Worktree struct {
	Path   string
	Branch string
}

// MainRepoRoot returns the absolute path of the MAIN worktree for any dir
// inside the repo, whether dir is in the main checkout or in a linked worktree.
//
// It relies on the git guarantee that `git worktree list --porcelain` always
// lists the main worktree first. --show-toplevel gives the containing
// worktree, not the main one, and --git-common-dir gives a path relative to
// the wrong directory; neither answers this question.
func MainRepoRoot(dir string) (string, error) {
	out, err := Cmd(dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	entries := ParseWorktreePorcelain(string(out))
	if len(entries) == 0 || entries[0].Path == "" {
		return "", fmt.Errorf("no worktrees found in %s", dir)
	}
	return filepath.FromSlash(entries[0].Path), nil
}

// Toplevel returns the root of the worktree containing dir: the containing
// worktree's own root, whether dir is that root, a subdirectory of it, a
// linked worktree's root, or a subdirectory of a linked worktree.
//
// It is what tells "dir is somewhere inside the main checkout" from "dir is
// inside a genuinely different linked worktree". Comparing dir against
// MainRepoRoot cannot: a subdirectory of the main checkout is never equal to
// the checkout's root, and is not a linked worktree either.
func Toplevel(dir string) (string, error) {
	out, err := Cmd(dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return filepath.FromSlash(strings.TrimSpace(string(out))), nil
}

// ParseWorktreePorcelain reads `git worktree list --porcelain` output.
func ParseWorktreePorcelain(output string) []Worktree {
	var result []Worktree
	var current Worktree
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				result = append(result, current)
			}
			current = Worktree{}
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

// IsLinkedWorktree reports whether dir sits in a linked worktree rather than
// in the main checkout, and returns both roots.
//
// It is the question a session launch actually asks, and having it here means
// the two git calls it takes are ordered the same way everywhere: MainRepoRoot
// first, because it fails fast when dir is not a git directory at all.
func IsLinkedWorktree(dir string) (worktree, mainRoot string, ok bool) {
	root, err := MainRepoRoot(dir)
	if err != nil {
		return "", "", false
	}
	top, err := Toplevel(dir)
	if err != nil {
		return "", "", false
	}
	if strings.EqualFold(filepath.Clean(top), filepath.Clean(root)) {
		return "", root, false
	}
	return top, root, true
}
