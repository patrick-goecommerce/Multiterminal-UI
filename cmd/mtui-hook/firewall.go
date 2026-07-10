// PreToolUse path firewall: blocks Edit/Write/NotebookEdit calls that target
// a path inside the main repo checkout while a different worktree is the
// expected working area for this session. See
// docs/superpowers/specs/2026-07-09-worktree-path-firewall-design.md.
package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// worktreeSidecar is the per-session state this binary persists to disk so
// that a later, freshly-spawned PreToolUse invocation for the same session
// can recover the worktree context an earlier PostToolUse:EnterWorktree call
// discovered. mtui-hook has no long-lived process to hold this in memory.
type worktreeSidecar struct {
	WorktreePath string `json:"worktreePath"`
	MainRepoRoot string `json:"mainRepoRoot"`
}

func sidecarPath(hooksDir, sessionID string) string {
	return filepath.Join(hooksDir, sessionID+".worktree.json")
}

func writeWorktreeSidecar(hooksDir, sessionID, worktreePath, mainRepoRoot string) {
	data, err := json.Marshal(worktreeSidecar{WorktreePath: worktreePath, MainRepoRoot: mainRepoRoot})
	if err != nil {
		return
	}
	_ = os.WriteFile(sidecarPath(hooksDir, sessionID), data, 0644)
}

func removeWorktreeSidecar(hooksDir, sessionID string) {
	_ = os.Remove(sidecarPath(hooksDir, sessionID))
}

// resolveWorktreeContext returns the expected worktree path and main-repo
// root for a session, or two empty strings if no restriction is active. The
// env vars (set once at pane launch for MTUI-created worktree panes) take
// priority over the sidecar file (written by a mid-session EnterWorktree
// call) since they require no disk I/O. Any failure (missing/corrupt
// sidecar) is treated as "no context active" — fail open.
func resolveWorktreeContext(hooksDir, sessionID string) (worktreePath, mainRepoRoot string) {
	if wt := os.Getenv("MULTITERMINAL_WORKTREE_PATH"); wt != "" {
		return wt, os.Getenv("MULTITERMINAL_MAIN_REPO_ROOT")
	}
	data, err := os.ReadFile(sidecarPath(hooksDir, sessionID))
	if err != nil {
		return "", ""
	}
	var sc worktreeSidecar
	if json.Unmarshal(data, &sc) != nil {
		return "", ""
	}
	return sc.WorktreePath, sc.MainRepoRoot
}

// isUnderDir reports whether path is dir itself or nested inside it,
// comparing case-insensitively (Windows paths).
func isUnderDir(path, dir string) bool {
	if dir == "" || path == "" {
		return false
	}
	p := strings.ToLower(filepath.Clean(path))
	d := strings.ToLower(filepath.Clean(dir))
	return p == d || strings.HasPrefix(p, d+string(filepath.Separator))
}

// gitMainRepoRoot shells out to `git worktree list --porcelain` from dir and
// returns the first entry's path — git guarantees the main worktree is
// always listed first. Mirrors internal/backend.mainRepoRoot, duplicated
// here (not imported, see Global Constraints).
func gitMainRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "--no-optional-locks", "-c", "core.fsmonitor=false", "worktree", "list", "--porcelain")
	cmd.Dir = dir
	// Same suppression env as internal/backend.gitCmd (duplicated, not imported —
	// see Global Constraints): stops git from prompting or spawning helpers.
	cmd.Env = append(os.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line) // porcelain lines can carry a trailing \r
		if strings.HasPrefix(line, "worktree ") {
			return filepath.FromSlash(strings.TrimPrefix(line, "worktree ")), nil
		}
	}
	return "", errors.New("no worktree entries found")
}
