package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// The lifecycle-hook reader lives on the session host (internal/hub), because
// an agent keeps reporting while no window is open. What arrives here is the
// event after the host has already recorded what it said about the session's
// state; everything below is what the window does about it.

// onHookReport dispatches one hook event to the parts of the UI that care.
//
// The three handlers are looked up through seams so a test can watch what a
// given event triggers without running the real thing, which probes git and
// spawns a CLI. Nil means "the real one", which is what production always is.
func (a *AppService) onHookReport(r hub.HookReport) {
	if r.Session == 0 {
		return
	}

	// UserPromptSubmit carries the user's prompt text (see cmd/mtui-hook).
	// It is what an automatic pane name is derived from.
	if r.Event == "UserPromptSubmit" && r.Message != "" {
		call2(a.hookPrompt, a.maybeGeneratePaneName)(r.Session, r.Message)
	}

	// Every event carries the session's cwd, which is how a pane that left a
	// worktree is noticed (spec 2026-07-03 section 4).
	call4(a.hookWorktree, a.onWorktreeChange)(r.Session, r.WorktreePath, r.WorktreeBranch, r.Cwd)

	if r.BlockedPath != "" {
		call3(a.hookPathBlocked, a.onWorktreePathBlocked)(r.Session, r.BlockedPath, r.BlockReason)
	}

	if r.Activity != "" {
		a.onHookActivity(r.Session, string(r.Activity), "")
	}
	log.Printf("[hooks] session %d: %s", r.Session, r.Event)
}

// call2, call3 and call4 pick the seam when one is installed and the real
// handler otherwise.
func call2(seam, real func(int, string)) func(int, string) {
	if seam != nil {
		return seam
	}
	return real
}

func call3(seam, real func(int, string, string)) func(int, string, string) {
	if seam != nil {
		return seam
	}
	return real
}

func call4(seam, real func(int, string, string, string)) func(int, string, string, string) {
	if seam != nil {
		return seam
	}
	return real
}
