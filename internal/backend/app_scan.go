package backend

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// ActivityInfo is sent to the frontend when a session's activity state changes.
type ActivityInfo struct {
	ID         hub.Ref `json:"id"`
	Activity   string  `json:"activity"` // "idle", "active", "done", "waitingPermission", "waitingAnswer", "error", plus "sleeping"/"resuming" from emitLifecycleActivity
	Cost       string  `json:"cost"`
	Title      string  `json:"title"`      // OSC-derived window title (fallback pane name)
	ContextPct int     `json:"contextPct"` // % of context window used (statusline); 0 if unknown
	Model      string  `json:"model"`      // model display name (statusline); "" if unknown
	// SessionName is the agent's own name for the session (Claude Code's
	// session_name), "" until it reports one.
	SessionName string `json:"sessionName"`
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
	prevName   = make(map[int]string)
)

// cleanupActivityTracking removes stale tracking data for a closed session.
// The host forgets its own half when the session goes.
func cleanupActivityTracking(id int) {
	prevEmitMu.Lock()
	delete(prevCost, id)
	delete(prevTitle, id)
	delete(prevName, id)
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
		nameChanged := prevName[id] != r.Name
		changed := activityChanged || costChanged || titleChanged || nameChanged
		if costChanged {
			prevCost[id] = costStr
		}
		if titleChanged {
			prevTitle[id] = title
		}
		if nameChanged {
			prevName[id] = r.Name
		}
		prevEmitMu.Unlock()

		if changed && a.app != nil {
			log.Printf("[scan] session %d: activity=%s cost=%s title=%q", id, confirmedActivity, costStr, title)
			a.app.Event.Emit("terminal:activity", scanActivityInfo(a.ref(id), r, costStr))
		}

		// Everything below is a side effect of the *confirmed* change, and
		// this is the only place any of it runs — the hook emit path
		// (onHookActivity) repaints the badge early but deliberately triggers
		// nothing, so a completion reports progress exactly once (#188).
		//
		// None of it is gated on a.app: the event emitter is display, and its
		// absence says nothing about whether the queue must advance.

		// The queue advances on the host, which also runs with no window open.
		// What is left here is telling the orchestrator.
		if activityChanged && confirmedActivity == "done" {
			a.notifyOrchestratorDone(id)
		}

		// A settled "idle" pane (output stopped, no recognisable prompt) with an
		// active finish prep still has to advance: the "done" trigger never
		// fires when the agent finishes without drawing a visible prompt, which
		// would strand the prep item as pending forever. The host deliberately
		// does not do this for every session, because "idle" also means the
		// classifier did not recognise the screen and typing into a pager is
		// worse than waiting. Only this flow knows better, so only it nudges.
		if activityChanged && confirmedActivity == "idle" {
			if st := a.getFinishState(id); st != nil && st.Phase == "preparing" {
				a.host.QueueAdvance(id)
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

// scanActivityInfo is what one scan result tells the frontend.
//
// The state rides along only on its own change. A tick that emits because the
// cost or the title moved used to repeat the confirmed state too, and while a
// hook's fresher state was still inside the debounce window that repeat
// painted the old one back over it: "läuft", "fertig", "läuft" within a
// second. The frontend leaves the state alone when the field is empty.
func scanActivityInfo(ref hub.Ref, r hub.ScanResult, cost string) ActivityInfo {
	info := ActivityInfo{
		ID:         ref,
		Cost:       cost,
		Title:      r.Title,
		ContextPct: r.ContextPct,
		Model:      r.Model,

		SessionName: r.Name,
	}
	if r.Changed {
		info.Activity = string(r.Activity)
		info.ActivitySince = unixOrZero(r.Since)
	}
	return info
}
