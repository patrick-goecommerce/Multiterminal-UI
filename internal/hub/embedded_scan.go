package hub

import (
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ScanActivity implements Host.
//
// This is the half of the old scan loop that has to be next to the screens:
// deciding what each session is doing, and whether that is a real change or a
// repaint. An earlier version of this comment said debouncing belonged to the
// caller. It does not: the classifier is a snapshot without hysteresis, so
// "did it change" is a question about the screen, and a queue that advances on
// a flickered "done" is broken rather than merely jumpy (#188).
//
// What stays with the caller is what to DO about a confirmed change.
func (h *Embedded) ScanActivity() []ScanResult {
	ids, sessions := h.sessionObjects()
	now := time.Now()
	out := make([]ScanResult, 0, len(ids))
	for i, sess := range sessions {
		r := scanOne(ids[i], sess)
		if r.Asleep {
			out = append(out, r)
			continue
		}
		changed, confirmed, began := h.activity.confirm(r.ID, r.Activity, now)
		r.Changed = changed
		r.Since = began
		if confirmed != "" {
			// Report the confirmed state; fall back to the raw reading only
			// while the session has never confirmed one, so a caller never
			// sees an activity outside the documented set.
			r.Activity = confirmed
		}
		out = append(out, r)
	}
	// Advancing the queue belongs here and not in the loop above it: this is
	// where a transition is confirmed, and a caller that drives the scan
	// itself (a client polling /v1/scan, a test) must not silently get a queue
	// that never moves. It fires only on Changed, which is true exactly once
	// per transition, so a second scan right after is a no-op.
	h.advanceQueuesAfterScan(out)
	return out
}

// ConfirmedActivity implements Host.
func (h *Embedded) ConfirmedActivity(id int) (Activity, time.Time) {
	return h.activity.state(id)
}

// ForceActivity implements Host.
func (h *Embedded) ForceActivity(id int, state Activity, at time.Time) error {
	if _, err := h.lookup(id); err != nil {
		return err
	}
	h.activity.force(id, state, at)
	return nil
}

// SeedActivity implements Host.
func (h *Embedded) SeedActivity(id int, state Activity, at time.Time) error {
	if _, err := h.lookup(id); err != nil {
		return err
	}
	h.activity.seed(id, state, at)
	return nil
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

// scanTick returns how long to wait before looking again.
//
// More sessions means a slower tick: the scan reads every screen, and past a
// handful of panes the cost of looking often outweighs the latency it saves.
func scanTick(sessions int) time.Duration {
	switch {
	case sessions <= 3:
		return 500 * time.Millisecond
	case sessions <= 6:
		return 600 * time.Millisecond
	default:
		return 750 * time.Millisecond
	}
}

// scanLoop keeps every session's state current for as long as the host lives.
//
// It runs where the sessions are, which is the point: a daemon whose last
// window closed still has agents working, and their activity, tokens and cost
// have to keep being written down. A scan driven by the client would stop the
// moment the client did.
func (h *Embedded) scanLoop() {
	interval := scanTick(0)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			results := h.ScanActivity()
			if len(results) > 0 {
				// ScanActivity already advanced the queues; an enqueued task
				// keeps moving because this loop runs where the sessions are,
				// not because a client asked.
				h.emit(EventSessionScan, ScanReport{Results: results})
			}
			if next := scanTick(len(results)); next != interval {
				interval = next
				ticker.Reset(interval)
			}
		}
	}
}

// BackdateActivityForTest moves a session's armed debounce candidate back by
// the full debounce window, so a caller can confirm a state change on the next
// scan instead of sleeping. It reports whether a candidate was armed at all:
// a test that back-dates nothing and then asserts a confirmation would
// otherwise pass without having tested the debounce.
func (h *Embedded) BackdateActivityForTest(id int) bool {
	return h.activity.backdateCandidate(id, debounceWindow)
}
