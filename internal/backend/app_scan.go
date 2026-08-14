package backend

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ActivityInfo is sent to the frontend when a session's activity state changes.
type ActivityInfo struct {
	ID         int    `json:"id"`
	Activity   string `json:"activity"` // "idle", "active", "done", "waitingPermission", "waitingAnswer", "error"
	Cost       string `json:"cost"`
	Title      string `json:"title"`      // OSC-derived window title (fallback pane name)
	ContextPct int    `json:"contextPct"` // % of context window used (statusline); 0 if unknown
	Model      string `json:"model"`      // model display name (statusline); "" if unknown
	// ActivitySince is when the confirmed state began, as seconds since epoch;
	// 0 when unknown. Travels on the event only — events are plain JSON and do
	// not need models.ts, unlike binding returns.
	ActivitySince int64 `json:"activitySince"`
}

// prevActivity tracks the last emitted state per session to avoid spamming.
var (
	prevActivityMu sync.Mutex
	prevActivity   = make(map[int]string)
	prevCost       = make(map[int]string)
	prevTitle      = make(map[int]string)
)

// scanInterval returns the scan tick duration based on the number of active sessions.
// More sessions → slower ticks to reduce overhead.
func (a *AppService) scanInterval() time.Duration {
	a.mu.Lock()
	n := len(a.sessions)
	a.mu.Unlock()
	switch {
	case n <= 3:
		return 500 * time.Millisecond
	case n <= 6:
		return 600 * time.Millisecond
	default:
		return 750 * time.Millisecond
	}
}

// scanLoop periodically scans all sessions for activity changes and token info.
// The interval adapts to the number of active sessions.
func (a *AppService) scanLoop(ctx context.Context) {
	interval := a.scanInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.scanAllSessions()
			// Re-check if interval should change
			if newInterval := a.scanInterval(); newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
			}
		}
	}
}

func activityString(a terminal.ActivityState) string {
	switch a {
	case terminal.ActivityActive:
		return "active"
	case terminal.ActivityDone:
		return "done"
	case terminal.ActivityWaitingPermission:
		return "waitingPermission"
	case terminal.ActivityWaitingAnswer:
		return "waitingAnswer"
	case terminal.ActivityError:
		return "error"
	default:
		return "idle"
	}
}

// cleanupActivityTracking removes stale tracking data for a closed session.
func cleanupActivityTracking(id int) {
	prevActivityMu.Lock()
	delete(prevActivity, id)
	delete(prevCost, id)
	delete(prevTitle, id)
	cleanupActivityDebounce(id)
	prevActivityMu.Unlock()
}

// scanAllSessions checks each session for activity and token updates.
func (a *AppService) scanAllSessions() {
	a.mu.Lock()
	ids := make([]int, 0, len(a.sessions))
	sessions := make([]*terminal.Session, 0, len(a.sessions))
	for id, s := range a.sessions {
		ids = append(ids, id)
		sessions = append(sessions, s)
	}
	a.mu.Unlock()

	for i, sess := range sessions {
		id := ids[i]

		// A sleeping pane has a frozen screen (issue #180). Classifying it would
		// re-emit the state it had before falling asleep and overwrite the
		// "schläft" badge on every tick; its tokens cannot change either.
		if sess.IsSuspendedOrSuspending() {
			continue
		}

		sess.ScanTokens() // always scan for token/cost data

		var activity terminal.ActivityState
		if sess.HasHookData() {
			// Hook events drive activity state for Claude panes — skip PTY regex scan
			activity = sess.GetActivity()
			// Exception: when hook says "done", cross-check screen for a trailing
			// question (e.g. Claude ended with "Was liegt an?"). The Stop hook fires
			// before the PTY scanner can see the question, so we do it here.
			if activity == terminal.ActivityDone {
				if screen := sess.ClassifyScreenState(); screen == terminal.ActivityWaitingAnswer {
					activity = terminal.ActivityWaitingAnswer
				}
			}
			// Exception: when hook says "active" but the PTY has been quiet well
			// past the normal detection threshold AND the screen already shows a
			// completed prompt, the terminating hook event (Stop) was lost or
			// delayed. Without this, a pane — and any pipeline queue waiting on
			// it via the "done" transition below — would hang forever, since
			// hook-driven sessions never fall back to the PTY scan otherwise.
			if activity == terminal.ActivityActive {
				if lastOutput := sess.GetLastOutputAt(); !lastOutput.IsZero() && time.Since(lastOutput) > terminal.ActivityStaleThreshold {
					if screen := sess.ClassifyScreenState(); screen == terminal.ActivityDone || screen == terminal.ActivityWaitingAnswer {
						activity = screen
					}
				}
			}
		} else {
			activity = sess.DetectActivity()
		}
		actStr := activityString(activity)

		tokens := sess.GetTokens()
		costStr := ""
		if tokens.TotalCost > 0 {
			costStr = fmt.Sprintf("$%.2f", tokens.TotalCost)
		}

		ctxPct, model, _ := sess.StatuslineInfo()

		title := sess.GetTitle()

		// Only emit when state, cost, or title actually changed. The activity
		// half runs through confirmActivity, so a one-tick flicker never
		// reaches the UI — nor the queue, orchestrator and issue reporting
		// below, which all key off activityChanged.
		now := time.Now()
		prevActivityMu.Lock()
		activityChanged := confirmActivity(id, actStr, now)
		costChanged := prevCost[id] != costStr
		titleChanged := prevTitle[id] != title
		changed := activityChanged || costChanged || titleChanged
		if costChanged {
			prevCost[id] = costStr
		}
		if titleChanged {
			prevTitle[id] = title
		}
		confirmedActivity := prevActivity[id]
		if confirmedActivity == "" {
			// No confirmed state yet (session just started, still on its
			// first candidate). Fall back to the raw observation instead of
			// emitting "" — outside the documented enum — when only cost or
			// title changed on this tick. activityString never returns "",
			// so this is always a valid value; it does not weaken the
			// debounce guarantee because activityChanged is false here, so
			// none of the confirmed-transition side effects below fire.
			confirmedActivity = actStr
		}
		prevActivityMu.Unlock()

		if changed && a.app != nil {
			log.Printf("[scan] session %d: activity=%s cost=%s title=%q", id, confirmedActivity, costStr, title)
			a.app.Event.Emit("terminal:activity", ActivityInfo{
				ID:            id,
				Activity:      confirmedActivity,
				Cost:          costStr,
				Title:         title,
				ContextPct:    ctxPct,
				Model:         model,
				ActivitySince: activitySinceUnix(id),
			})
		}

		// Trigger pipeline queue on fresh "done" transition
		if activityChanged && confirmedActivity == "done" && a.app != nil {
			a.processQueue(id)
			// Notify orchestrator that this agent finished
			a.notifyOrchestratorDone(id)
		}

		// A settled-"idle" pane (output stopped, no recognizable prompt) with an
		// active finish prep must still advance the queue: the "done" trigger
		// above never fires when Claude finishes without a visible ❯ prompt, which
		// would otherwise strand the finish prep as "pending" forever. Scoped to a
		// preparing finish flow so general pipeline timing is unaffected.
		if activityChanged && confirmedActivity == "idle" && a.app != nil {
			if st := a.getFinishState(id); st != nil && st.Phase == "preparing" {
				a.processQueue(id)
			}
		}

		// Surface waiting states to an active finish flow (spec 5.1/2)
		if activityChanged && a.app != nil {
			a.notifyFinishOnActivity(id, confirmedActivity)
		}

		// Report issue progress on activity transitions
		if activityChanged && a.app != nil {
			a.onActivityChangeForIssue(id, confirmedActivity, costStr)
		}
	}
}

// onActivityChangeForIssue triggers issue progress reports when
// a session linked to an issue changes activity state.
func (a *AppService) onActivityChangeForIssue(sessionID int, newActivity string, cost string) {
	if newActivity == "done" {
		a.reportIssueProgress(sessionID, progressDone, cost)
	}
}
