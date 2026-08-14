package backend

import "time"

// debounceWindow is how long a differing raw activity state must hold before it
// counts as a real change.
//
// The scan tick is 500-750 ms depending on session count (see scanInterval).
// The observation count and the worst-case latency are maximised by different
// ticks: at the fastest tick (500 ms) a confirmation costs four observations
// (arm at t0, hold at t0+500ms and t0+1000ms, confirm at t0+1500ms), but the
// coarsest tick (750 ms, 7+ sessions) needs only three observations while
// producing the higher overall latency — ~2.25 s once the up-to-one-tick
// delay before the state is first observed is counted. That buys immunity
// against two independent flicker sources: Claude's TUI repainting while
// idle, and classifyScreenState being a snapshot without hysteresis — a tick
// landing between ESC[2K and the redraw of the input box classifies the very
// same screen as "idle" (issue #188).
//
// The frontend's own 900 ms smoothing is removed in exchange, so this is not a
// net slowdown.
const debounceWindow = 1200 * time.Millisecond

// Debounce bookkeeping, guarded by prevActivityMu together with prevActivity.
var (
	// pendingActivity is the candidate state observed but not yet confirmed.
	pendingActivity = make(map[int]string)
	// pendingSince is when that candidate was first observed.
	pendingSince = make(map[int]time.Time)
	// activitySince is when the currently confirmed state began. This is the
	// single place it is written; the read path and SetHookActivity must never
	// touch it, or the flicker this package just fixed returns in the duration.
	activitySince = make(map[int]time.Time)
	// seededActivity marks a session whose activitySince was seeded from a
	// restored session file (setActivitySinceFor) but has not yet had its
	// first confirmActivity call. A restored pane gets a brand-new session ID
	// from CreateSession's counter, so prevActivity[id] starts empty exactly
	// like a pane that was never restored — confirmActivity cannot otherwise
	// tell the two apart. Without this flag, the first post-restart
	// confirmation would stamp activitySince with the observation time,
	// silently overwriting the seed within one debounce window and defeating
	// the entire point of persisting it across a restart (#189).
	seededActivity = make(map[int]bool)
)

// confirmActivity applies the debounce for one session and reports whether raw
// became the confirmed state. The caller must hold prevActivityMu.
//
// On confirmation, activitySince is stamped with the *first* observation, not
// with now: the state began when it was first seen, not when it survived the
// window. Otherwise every duration would be short by up to one window.
//
// Exception: a session seeded via setActivitySinceFor keeps its seeded value
// through its first confirmation instead — see seededActivity.
func confirmActivity(id int, raw string, now time.Time) bool {
	if prevActivity[id] == raw {
		// Back to (or still on) the confirmed state — drop any candidate.
		delete(pendingActivity, id)
		delete(pendingSince, id)
		return false
	}
	if pending, ok := pendingActivity[id]; !ok || pending != raw {
		// A new candidate: start its clock.
		pendingActivity[id] = raw
		pendingSince[id] = now
		return false
	}
	if now.Sub(pendingSince[id]) < debounceWindow {
		return false
	}
	prevActivity[id] = raw
	if seededActivity[id] {
		delete(seededActivity, id)
	} else {
		activitySince[id] = pendingSince[id]
	}
	delete(pendingActivity, id)
	delete(pendingSince, id)
	return true
}

// activitySinceFor returns when the confirmed state of a session began. The
// zero time means unknown — the pane has not had a confirmed change yet.
func activitySinceFor(id int) time.Time {
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()
	return activitySince[id]
}

// setActivitySinceFor seeds the timestamp of a restored pane, so a duration
// survives an MTUI restart instead of starting over.
func setActivitySinceFor(id int, t time.Time) {
	if t.IsZero() {
		return
	}
	prevActivityMu.Lock()
	activitySince[id] = t
	seededActivity[id] = true
	prevActivityMu.Unlock()
}

// SeedActivitySince restores a pane's state-start timestamp after a restart, so
// its badge keeps the duration it had instead of starting over. Zero is ignored.
func (a *AppService) SeedActivitySince(sessionID int, unix int64) {
	if unix <= 0 {
		return
	}
	setActivitySinceFor(sessionID, time.Unix(unix, 0))
}

// cleanupActivityDebounce drops a closed session's debounce state. The caller
// must hold prevActivityMu.
func cleanupActivityDebounce(id int) {
	delete(pendingActivity, id)
	delete(pendingSince, id)
	delete(activitySince, id)
	delete(seededActivity, id)
}

// forceActivity sets the confirmed activity state and its start timestamp
// together, and drops any pending debounce candidate. The caller must hold
// prevActivityMu.
//
// Use this instead of writing prevActivity[id] directly whenever a state is
// applied outside confirmActivity (e.g. a queue reset forcing "idle", or a
// suspend/resume transition). A bare prevActivity write leaves activitySince
// pointing at the *previous* state and leaves any armed candidate in place,
// which can then confirm on a single unrelated tick (issue #188 follow-up).
//
// This also consumes any pending seed from setActivitySinceFor: the forced
// timestamp already wins here, so a later confirmActivity call must not
// treat the session as still-seeded and keep stamping activitySince with
// this stale forced value forever.
func forceActivity(id int, state string, now time.Time) {
	prevActivity[id] = state
	activitySince[id] = now
	delete(pendingActivity, id)
	delete(pendingSince, id)
	delete(seededActivity, id)
}

// activitySinceUnix returns the confirmed state's start as seconds since epoch,
// or 0 when unknown. The frontend treats 0 as "show the state without a
// duration" rather than rendering an epoch date.
//
// This takes prevActivityMu itself (via activitySinceFor), so the caller must
// NOT hold it — unlike confirmActivity/forceActivity/cleanupActivityDebounce,
// which require it. Calling this from inside a critical section would deadlock
// the scan loop.
func activitySinceUnix(id int) int64 {
	t := activitySinceFor(id)
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// activitySinceUnixIfState is activitySinceUnix restricted to a state: it
// returns the start timestamp only when the session's *confirmed* state is
// already `state`, and 0 ("unknown") otherwise. It takes prevActivityMu
// itself, so the caller must not hold it.
//
// The hook emit path (app_hooks_setup.go) announces a state the moment the
// hook event lands — one debounce window before confirmActivity has seen it.
// At that moment activitySince still belongs to the *previous* state, so
// pairing it with the new label would render e.g. "fertig · 3 Std 20" for
// ~1.5-2.25 s and then snap to "fertig · gerade eben". Emitting 0 instead
// shows the new state without a duration until the confirmed value arrives,
// which is the documented meaning of 0 and never jumps backwards.
func activitySinceUnixIfState(id int, state string) int64 {
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()
	if prevActivity[id] != state {
		return 0
	}
	t := activitySince[id]
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// resetActivityDebounceForTest clears all debounce state between tests.
func resetActivityDebounceForTest() {
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()
	pendingActivity = make(map[int]string)
	pendingSince = make(map[int]time.Time)
	activitySince = make(map[int]time.Time)
	prevActivity = make(map[int]string)
	seededActivity = make(map[int]bool)
}
