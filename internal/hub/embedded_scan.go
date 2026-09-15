package hub

import (
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ScanActivity implements Host.
//
// This is the half of the old scan loop that has to be next to the screens:
// deciding what each session is doing. The other half, deciding what the UI
// and the queue do about a change, stays with the caller, because debouncing,
// reporting progress and advancing a queue are not properties of a terminal.
func (h *Embedded) ScanActivity() []ScanResult {
	ids, sessions := h.sessionObjects()
	out := make([]ScanResult, 0, len(ids))
	for i, sess := range sessions {
		out = append(out, scanOne(ids[i], sess))
	}
	return out
}

func scanOne(id int, sess *terminal.Session) ScanResult {
	// A sleeping pane has a frozen screen (#180). Classifying it would re-emit
	// the state it had before falling asleep and overwrite the "schläft" badge
	// on every tick; its tokens cannot change either.
	if sess.IsSuspendedOrSuspending() {
		return ScanResult{ID: id, Asleep: true}
	}

	sess.ScanTokens() // always scan for token/cost data

	activity := classifyForScan(sess)
	ctxPct, model, _ := sess.StatuslineInfo()
	return ScanResult{
		ID:         id,
		Activity:   activityOf(activity),
		Cost:       sess.GetTokens().TotalCost,
		Title:      sess.GetTitle(),
		ContextPct: ctxPct,
		Model:      model,
	}
}

// classifyForScan decides a session's activity for this tick.
//
// Hook events drive the state for agents that report them, with two exceptions
// where the screen knows better, both of which cost a pane its progress if
// they are missed.
func classifyForScan(sess *terminal.Session) terminal.ActivityState {
	if !sess.HasHookData() {
		return sess.DetectActivity()
	}

	activity := sess.GetActivity()

	// When the hook says "done", cross-check the screen for a trailing
	// question (the agent ended with "Was liegt an?"). The Stop hook fires
	// before the screen scanner can see the question.
	if activity == terminal.ActivityDone {
		if screen := sess.ClassifyScreenState(); screen == terminal.ActivityWaitingAnswer {
			return terminal.ActivityWaitingAnswer
		}
		return activity
	}

	// When the hook says "active" but the PTY has been quiet well past the
	// normal detection threshold and the screen already shows a completed
	// prompt, the terminating hook event was lost or delayed. Without this a
	// pane, and any queue waiting on its "done" transition, would hang
	// forever: a hook-driven session never falls back to the screen scan
	// otherwise.
	if activity == terminal.ActivityActive {
		lastOutput := sess.GetLastOutputAt()
		if !lastOutput.IsZero() && time.Since(lastOutput) > terminal.ActivityStaleThreshold {
			if screen := sess.ClassifyScreenState(); screen == terminal.ActivityDone || screen == terminal.ActivityWaitingAnswer {
				return screen
			}
		}
	}
	return activity
}
