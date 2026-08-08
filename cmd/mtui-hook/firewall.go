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
// root for a session, or two empty strings if no restriction is active.
// The sidecar file (written by a mid-session EnterWorktree call) takes
// priority over the env vars (set once at pane launch for MTUI-created
// worktree panes): the sidecar reflects an explicit, in-session worktree
// change and is the fresher signal — without this priority, a pane already
// isolated in worktree A that then calls EnterWorktree again to enter a
// sibling worktree B would still be pinned to A by the launch-time env var,
// wrongly blocking legitimate edits inside B (a real false positive found in
// final review, spec 2026-07-09-worktree-path-firewall-design.md §4.1). Any
// failure (missing/corrupt sidecar) falls through to the env var, then to
// "no context active" — fail open throughout.
func resolveWorktreeContext(hooksDir, sessionID string) (worktreePath, mainRepoRoot string) {
	if data, err := os.ReadFile(sidecarPath(hooksDir, sessionID)); err == nil {
		var sc worktreeSidecar
		if json.Unmarshal(data, &sc) == nil && sc.WorktreePath != "" {
			return sc.WorktreePath, sc.MainRepoRoot
		}
	}
	if wt := os.Getenv("MULTITERMINAL_WORKTREE_PATH"); wt != "" {
		return wt, os.Getenv("MULTITERMINAL_MAIN_REPO_ROOT")
	}
	return "", ""
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

// toolInputPath holds the path fields present in Edit/Write/NotebookEdit
// tool_input payloads. Edit and Write use file_path; NotebookEdit uses
// notebook_path — not the same field name.
type toolInputPath struct {
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
}

// writeTools are the tools this firewall can police. KNOWN GAP: writes made
// through the shell (Bash: `sed -i`, `tee`, `echo > file`) bypass it entirely,
// because a PreToolUse hook cannot tell which files an arbitrary command will
// touch. Accepted deliberately — pattern-denying Bash would never be airtight
// and would block legitimate shell work. Documented in README_TECH.md.
var writeTools = map[string]bool{"Edit": true, "Write": true, "NotebookEdit": true}

// worktreeExemptDirs are project-relative directories that stay writable in
// the main checkout even under the worktree-mandatory policy: documentation,
// specs/plans and MTUI's own project state are not code and are routinely
// edited outside a feature worktree.
var worktreeExemptDirs = []string{"docs", ".mtui", ".claude"}

// isWorktreeExemptPath reports whether path is exempt from the
// worktree-mandatory block: any *.md file, or anything under one of
// worktreeExemptDirs relative to root.
//
// This applies ONLY to the force-mode branch. Once a worktree IS active the
// original rule stands unchanged — splitting one session's work across the
// worktree and the main checkout is what that branch exists to prevent.
func isWorktreeExemptPath(path, root string) bool {
	if strings.EqualFold(filepath.Ext(path), ".md") {
		return true
	}
	for _, dir := range worktreeExemptDirs {
		if isUnderDir(path, filepath.Join(root, dir)) {
			return true
		}
	}
	return false
}

// checkPathFirewall inspects a PreToolUse event for Edit/Write/NotebookEdit
// and reports whether it should be blocked, plus the path it classified.
// hooksDir/ev.SessionID resolve the sidecar file if no env var is set.
func checkPathFirewall(ev claudeEvent, hooksDir string) (blocked bool, path string, reason string) {
	if !writeTools[ev.ToolName] || len(ev.ToolInput) == 0 {
		return false, "", ""
	}
	var input toolInputPath
	if json.Unmarshal(ev.ToolInput, &input) != nil {
		return false, "", ""
	}
	path = input.FilePath
	if path == "" {
		path = input.NotebookPath
	}
	if path == "" {
		return false, "", ""
	}

	worktreePath, mainRoot := resolveWorktreeContext(hooksDir, ev.SessionID)
	if worktreePath == "" || mainRoot == "" {
		// Worktree-mandatory policy: with no worktree active yet, code changes
		// in the main checkout are denied so the model has to isolate first.
		// A deny is not a dead end — permissionDecisionReason goes back to the
		// model, which can call EnterWorktree and retry the write there.
		forced := os.Getenv("MULTITERMINAL_FORCE_WORKTREE_ROOT")
		if forced != "" && isUnderDir(path, forced) && !isWorktreeExemptPath(path, forced) {
			return true, path, "Worktree-Pflicht: In diesem Projekt sind Code-Änderungen nur in " +
				"einem Worktree erlaubt, nicht direkt im Hauptrepo (" + forced + "). Nutze zuerst " +
				"das EnterWorktree-Tool und wiederhole die Änderung dort. (Dokumentation und " +
				"Planung — .md, docs/, .mtui/, .claude/ — sind ausgenommen.)"
		}
		return false, path, ""
	}
	if isUnderDir(path, worktreePath) {
		return false, path, ""
	}
	if isUnderDir(path, mainRoot) {
		return true, path, "Pfad liegt im Hauptrepo (" + mainRoot + "), nicht im aktiven Worktree (" + worktreePath + "). Bitte den Pfad korrigieren."
	}
	return false, path, ""
}
