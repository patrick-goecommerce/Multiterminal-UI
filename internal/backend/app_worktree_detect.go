// Package backend – detection of Claude Code's native EnterWorktree tool via
// the existing hook pipeline. MTUI does not create worktrees itself here; it
// only observes what Claude decided to do (spec 2026-07-03).
package backend

import (
	"log"
	"strings"
)

// worktreeState is the currently known worktree for one MTUI session.
type worktreeState struct {
	Path   string
	Branch string
}

// emitWorktreeEventSafe calls the emit seam if set (nil in unit tests unless
// explicitly wired) and if the real app is present.
func (a *AppService) emitWorktreeEventSafe(name string, payload any) {
	if a.emitWorktreeEvent != nil {
		a.emitWorktreeEvent(name, payload)
		return
	}
	if a.app != nil {
		a.app.Event.Emit(name, payload)
	}
}

// currentWorktree returns the worktree currently tracked for a session.
func (a *AppService) currentWorktree(sessionID int) (path, branch string, ok bool) {
	a.worktreeStateMu.Lock()
	defer a.worktreeStateMu.Unlock()
	st, exists := a.worktreeState[sessionID]
	return st.Path, st.Branch, exists
}

// onWorktreeChange is the HookManager callback (wired in setupHooks). It is
// invoked on EVERY hook event for a session: worktreePath/worktreeBranch are
// non-empty only on a fresh EnterWorktree detection; cwd is always populated
// and used to notice when a session has left a previously known worktree.
func (a *AppService) onWorktreeChange(mtID int, worktreePath, worktreeBranch, cwd string) {
	if worktreePath != "" {
		a.handleWorktreeDetected(mtID, worktreePath, worktreeBranch)
		return
	}
	a.handleWorktreeCwdUpdate(mtID, cwd)
}

func (a *AppService) handleWorktreeDetected(mtID int, worktreePath, worktreeBranch string) {
	a.worktreeStateMu.Lock()
	a.worktreeState[mtID] = worktreeState{Path: worktreePath, Branch: worktreeBranch}
	a.worktreeStateMu.Unlock()

	target := ""
	if root, err := mainRepoRoot(worktreePath); err == nil {
		target = checkedOutBranch(root)
	} else {
		log.Printf("[worktree-detect] mainRepoRoot(%s): %v", worktreePath, err)
	}

	log.Printf("[worktree-detect] session %d entered %s on %s (target %s)", mtID, worktreePath, worktreeBranch, target)
	a.emitWorktreeEventSafe("worktree:detected", WorktreeDetectedEvent{
		ID: mtID, WorktreePath: worktreePath, WorktreeBranch: worktreeBranch, TargetBranch: target,
	})
}

// handleWorktreeCwdUpdate reconciles the tracked worktree against the cwd that
// rides along on every hook event.
//
// cwd is the only continuously observable truth about where a pane actually
// works. PostToolUse:EnterWorktree fires at most once per session and misses
// every pane that was restored into a worktree, resumed there with --continue,
// or moved with a plain `cd` — 11 of 48 worktree sessions on a real
// installation. Those panes used to keep an empty worktreePath forever and the
// titlebar fell back to the main repo's branch, which is the base branch the
// footer already shows.
//
// So cwd both sets and clears: entering a linked worktree emits detected,
// moving between two of them emits detected for the new one, and leaving for
// the main checkout emits cleared.
func (a *AppService) handleWorktreeCwdUpdate(mtID int, cwd string) {
	if cwd == "" {
		return // event carries no location claim
	}

	a.worktreeStateMu.Lock()
	st, known := a.worktreeState[mtID]
	// Two guards, both keeping git off the 100ms hook path (issue #192): the
	// cheap containment check covers working down inside a known worktree, and
	// the probe cache covers everything else, including the ordinary pane that
	// never sits in a worktree at all.
	if known && pathWithin(cwd, st.Path) {
		a.worktreeStateMu.Unlock()
		return
	}
	if strings.EqualFold(a.lastProbedCwd[mtID], cwd) {
		a.worktreeStateMu.Unlock()
		return
	}
	a.worktreeStateMu.Unlock()

	path, branch, isWorktree := a.probeWorktree(cwd)

	a.worktreeStateMu.Lock()
	a.lastProbedCwd[mtID] = cwd
	a.worktreeStateMu.Unlock()

	if isWorktree {
		if known && strings.EqualFold(path, st.Path) {
			return // same worktree, reached via a path spelling we had not seen
		}
		a.handleWorktreeDetected(mtID, path, branch)
		return
	}
	if !known {
		return // main checkout and nothing tracked — the ordinary case
	}

	a.worktreeStateMu.Lock()
	delete(a.worktreeState, mtID)
	a.worktreeStateMu.Unlock()

	log.Printf("[worktree-detect] session %d left %s", mtID, st.Path)
	a.emitWorktreeEventSafe("worktree:cleared", WorktreeClearedEvent{ID: mtID})
}

// pathWithin reports whether cwd is dir itself or below it. Case-insensitive:
// Windows hands back both C:\Repo and c:\repo for the same directory.
func pathWithin(cwd, dir string) bool {
	if dir == "" {
		return false
	}
	if strings.EqualFold(cwd, dir) {
		return true
	}
	lower, prefix := strings.ToLower(cwd), strings.ToLower(dir)
	return strings.HasPrefix(lower, prefix+`\`) || strings.HasPrefix(lower, prefix+"/")
}

// probeWorktree runs the configured probe, defaulting to defaultWorktreeProbe.
func (a *AppService) probeWorktree(dir string) (path, branch string, ok bool) {
	if a.worktreeProbe != nil {
		return a.worktreeProbe(dir)
	}
	return defaultWorktreeProbe(dir)
}

// defaultWorktreeProbe reports whether dir sits inside a LINKED worktree, and
// on which branch.
//
// The comparison is toplevel-vs-main-worktree, not dir-vs-main-worktree: a
// subdirectory of the main checkout is never equal to its root but is not a
// linked worktree either (see gitToplevel).
func defaultWorktreeProbe(dir string) (path, branch string, ok bool) {
	top, err := gitToplevel(dir)
	if err != nil {
		return "", "", false // not a git repo, or dir is gone
	}
	main, err := mainRepoRoot(dir)
	if err != nil || strings.EqualFold(top, main) {
		return "", "", false
	}
	b := checkedOutBranch(top)
	if b == "" {
		return "", "", false // detached HEAD — nothing meaningful to show
	}
	return top, b, true
}

// onWorktreePathBlocked is the HookManager callback for a blocked write
// attempt (spec 2026-07-09). Purely informational — no state change, MTUI
// does not intervene beyond surfacing it to the user.
func (a *AppService) onWorktreePathBlocked(mtID int, path, reason string) {
	log.Printf("[worktree-detect] session %d blocked write to %s: %s", mtID, path, reason)
	a.emitWorktreeEventSafe("worktree:path-blocked", WorktreePathBlockedEvent{ID: mtID, Path: path, Reason: reason})
}
