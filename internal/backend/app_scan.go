package backend

import (
	"fmt"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"log"
	"sync"
	"time"
)

// ActivityInfo is sent to the frontend when a session's activity state changes.
type ActivityInfo struct {
	ID         int    `json:"id"`
	Activity   string `json:"activity"` // "idle", "active", "done", "waitingPermission", "waitingAnswer", "error", plus "sleeping"/"resuming" from emitLifecycleActivity
	Cost       string `json:"cost"`
	Title      string `json:"title"`      // OSC-derived window title (fallback pane name)
	ContextPct int    `json:"contextPct"` // % of context window used (statusline); 0 if unknown
	Model      string `json:"model"`      // model display name (statusline); "" if unknown
	// ActivitySince is when the confirmed state began, as seconds since epoch;
	// 0 when unknown. Travels on the event only — events are plain JSON and do
	// not need models.ts, unlike binding returns.
	ActivitySince int64 `json:"activitySince"`
}

// The last values emitted per session, so an unchanged tick stays silent.
//
// Only cost and title live here now. Whether the ACTIVITY changed is decided
// by the host (hub.ScanResult.Changed): the debounce that answers it belongs
// next to the screen classifier it corrects for, and a queue that has to
// advance with no window open cannot ask a window whether the state moved.
var (
	prevEmitMu sync.Mutex
	prevCost   = make(map[int]string)
	prevTitle  = make(map[int]string)
)

// cleanupActivityTracking removes stale tracking data for a closed session.
// The host forgets its own half when the session goes.
func cleanupActivityTracking(id int) {
	prevEmitMu.Lock()
	delete(prevCost, id)
	delete(prevTitle, id)
	prevEmitMu.Unlock()
}

// applyScanResults turns one scan tick into what the UI, the queue and the
// issue reporting do about it.
//
// The host decides what each session is doing and ticks on its own; this is
// the other half, and it runs wherever the window is. See
// hub.Embedded.scanLoop.
func (a *AppService) applyScanResults(results []hub.ScanResult) {
	for _, r := range results {
		id := r.ID
		if r.Asleep {
			continue
		}

		actStr := string(r.Activity)
		costStr := ""
		if r.Cost > 0 {
			costStr = fmt.Sprintf("$%.2f", r.Cost)
		}
		ctxPct, model := r.ContextPct, r.Model
		title := r.Title

		// Only emit when state, cost, or title actually changed. The activity
		// half runs through confirmActivity, so a one-tick flicker never
		// reaches the UI — nor the queue, orchestrator and issue reporting
		// below, which all key off activityChanged.
		// The host already applied the debounce: r.Activity is the confirmed
		// state and r.Changed says whether this tick is the transition. Every
		// side effect below keys off r.Changed and never off comparing the
		// activity, which would react to a repaint (#188).
		activityChanged := r.Changed
		confirmedActivity := actStr
		prevEmitMu.Lock()
		costChanged := prevCost[id] != costStr
		titleChanged := prevTitle[id] != title
		changed := activityChanged || costChanged || titleChanged
		if costChanged {
			prevCost[id] = costStr
		}
		if titleChanged {
			prevTitle[id] = title
		}
		prevEmitMu.Unlock()

		if changed && a.app != nil {
			log.Printf("[scan] session %d: activity=%s cost=%s title=%q", id, confirmedActivity, costStr, title)
			a.app.Event.Emit("terminal:activity", ActivityInfo{
				ID:            id,
				Activity:      confirmedActivity,
				Cost:          costStr,
				Title:         title,
				ContextPct:    ctxPct,
				Model:         model,
				ActivitySince: unixOrZero(r.Since),
			})
		}

		// Everything below is a side effect of the *confirmed* change, and
		// this is the only place any of it runs — the hook emit path
		// (onHookActivity) repaints the badge early but deliberately triggers
		// nothing, so a completion reports progress exactly once (#188).
		//
		// None of it is gated on a.app: the event emitter is display, and its
		// absence says nothing about whether the queue must advance.

		// Trigger pipeline queue on fresh "done" transition
		if activityChanged && confirmedActivity == "done" {
			a.processQueue(id)
			// Notify orchestrator that this agent finished
			a.notifyOrchestratorDone(id)
		}

		// A settled-"idle" pane (output stopped, no recognizable prompt) with an
		// active finish prep must still advance the queue: the "done" trigger
		// above never fires when Claude finishes without a visible ❯ prompt, which
		// would otherwise strand the finish prep as "pending" forever. Scoped to a
		// preparing finish flow so general pipeline timing is unaffected.
		if activityChanged && confirmedActivity == "idle" {
			if st := a.getFinishState(id); st != nil && st.Phase == "preparing" {
				a.processQueue(id)
			}
		}

		// Surface waiting states to an active finish flow (spec 5.1/2)
		if activityChanged {
			a.notifyFinishOnActivity(id, confirmedActivity)
		}

		// Report issue progress on activity transitions
		if activityChanged {
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

// unixOrZero renders a timestamp for the frontend, which reads 0 as "show the
// state without a duration" rather than rendering an epoch date.
func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
