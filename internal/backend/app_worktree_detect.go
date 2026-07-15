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

// handleWorktreeCwdUpdate clears the tracked worktree once cwd is observed
// outside it. Ordinary events for a session with no known worktree are a
// silent no-op — only a real transition emits worktree:cleared.
func (a *AppService) handleWorktreeCwdUpdate(mtID int, cwd string) {
	a.worktreeStateMu.Lock()
	st, known := a.worktreeState[mtID]
	if !known {
		a.worktreeStateMu.Unlock()
		return
	}
	stillInside := cwd == "" || strings.EqualFold(cwd, st.Path) || strings.HasPrefix(strings.ToLower(cwd), strings.ToLower(st.Path)+`\`) || strings.HasPrefix(strings.ToLower(cwd), strings.ToLower(st.Path)+"/")
	if stillInside {
		a.worktreeStateMu.Unlock()
		return
	}
	delete(a.worktreeState, mtID)
	a.worktreeStateMu.Unlock()

	log.Printf("[worktree-detect] session %d left %s", mtID, st.Path)
	a.emitWorktreeEventSafe("worktree:cleared", WorktreeClearedEvent{ID: mtID})
}

// onWorktreePathBlocked is the HookManager callback for a blocked write
// attempt (spec 2026-07-09). Purely informational — no state change, MTUI
// does not intervene beyond surfacing it to the user.
func (a *AppService) onWorktreePathBlocked(mtID int, path, reason string) {
	log.Printf("[worktree-detect] session %d blocked write to %s: %s", mtID, path, reason)
	a.emitWorktreeEventSafe("worktree:path-blocked", WorktreePathBlockedEvent{ID: mtID, Path: path, Reason: reason})
}
