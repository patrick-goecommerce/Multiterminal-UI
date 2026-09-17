package hub

import (
	"sync"
	"time"
)

// Deciding whether a session's state actually changed.
//
// This sits with the detection rather than with the caller, and the reason is
// not tidiness: classifyForScan is a snapshot without hysteresis, so a tick
// landing between an ESC[2K and the redraw of the input box classifies the
// very same screen as "idle" (issue #188). A consumer that reacts to the raw
// per-tick answer is reacting to a repaint. That matters far beyond the UI: a
// prompt queue advancing on a flickered "done" sends the next prompt while the
// agent is still working, which is a correctness bug, not a cosmetic one.
//
// What stays with the caller is what to DO about a confirmed change: emit an
// event, report issue progress, advance a queue. Whether there was a change at
// all belongs to whoever is looking at the screen.

// debounceWindow is how long a differing raw activity state must hold before
// it counts as a real change.
//
// The scan tick is 500-750 ms depending on session count (see scanTick). The
// observation count and the worst-case latency are maximised by different
// ticks: at the fastest tick (500 ms) a confirmation costs four observations
// (arm at t0, hold at t0+500ms and t0+1000ms, confirm at t0+1500ms), but the
// coarsest tick (750 ms, 7+ sessions) needs only three observations while
// producing the higher overall latency, roughly 2.25 s once the up-to-one-tick
// delay before the state is first observed is counted. That buys immunity
// against two independent flicker sources: the agent's TUI repainting while
// idle, and the screen classifier being a snapshot without hysteresis (#188).
const debounceWindow = 1200 * time.Millisecond

// activityDebouncer holds the per-session debounce state.
//
// It is a type rather than the package-level maps it grew up as, so that two
// hosts in one process (a test, a window that also talks to a daemon) cannot
// write over each other's sessions.
type activityDebouncer struct {
	mu sync.Mutex
	// confirmed is the state that survived the window.
	confirmed map[int]Activity
	// pending is the candidate observed but not yet confirmed, and since is
	// when it was first seen.
	pending map[int]Activity
	since   map[int]time.Time
	// began is when the currently confirmed state started. This is the single
	// place it is written; a read path that stamps it would bring the flicker
	// back in the duration instead of in the badge.
	began map[int]time.Time
	// seeded maps a session whose began was restored from a session file to
	// the state that timestamp belongs to. The entry lives until that
	// session's first confirmation.
	//
	// A restored pane gets a brand-new session ID, so confirmed[id] starts
	// empty exactly like a pane that was never restored, and there is no other
	// way to tell the two apart. Without this the first post-restart
	// confirmation would stamp began with the observation time and silently
	// defeat the whole point of persisting it (#189).
	//
	// The state is part of the entry, not an afterthought: a restored pane
	// re-launches its CLI, and that boot produces output for well over a
	// debounce window, so the first state confirmed after a restart is almost
	// always a transient "active". A state-blind seed would be swallowed by
	// it, showing "läuft · 3 Std 20" on a session two seconds old and then
	// "fertig · gerade eben" once it settled. Both halves wrong, in exactly
	// the long-idle case the feature exists for.
	seeded map[int]Activity
}

func newActivityDebouncer() *activityDebouncer {
	return &activityDebouncer{
		confirmed: map[int]Activity{},
		pending:   map[int]Activity{},
		since:     map[int]time.Time{},
		began:     map[int]time.Time{},
		seeded:    map[int]Activity{},
	}
}

// confirm applies the debounce for one session and reports whether raw became
// the confirmed state, along with when that state began.
//
// On confirmation, began is stamped with the *first* observation, not with
// now: the state began when it was first seen, not when it survived the
// window. Otherwise every duration would be short by up to one window.
//
// Exception: a seeded session keeps its seeded value through its first
// confirmation, but only when that confirmation lands on the state the seed
// was taken from. Either way the seed is consumed by the first confirmation
// and never applies to a later one.
func (d *activityDebouncer) confirm(id int, raw Activity, now time.Time) (changed bool, state Activity, began time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.confirmed[id] == raw {
		// Back to (or still on) the confirmed state: drop any candidate.
		delete(d.pending, id)
		delete(d.since, id)
		return false, d.confirmed[id], d.began[id]
	}
	if pending, ok := d.pending[id]; !ok || pending != raw {
		// A new candidate: start its clock.
		d.pending[id] = raw
		d.since[id] = now
		return false, d.confirmed[id], d.began[id]
	}
	if now.Sub(d.since[id]) < debounceWindow {
		return false, d.confirmed[id], d.began[id]
	}

	d.confirmed[id] = raw
	seedState, seeded := d.seeded[id]
	delete(d.seeded, id)
	if !seeded || seedState != raw {
		// No seed, or the pane settled on a different state than the one the
		// seed describes (the CLI-boot "active" above), so that state began
		// now and the observation time is the honest answer.
		d.began[id] = d.since[id]
	}
	delete(d.pending, id)
	delete(d.since, id)
	return true, raw, d.began[id]
}

// state returns the confirmed activity and when it began. An empty activity
// means the session has not had a confirmed change yet.
func (d *activityDebouncer) state(id int) (Activity, time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.confirmed[id], d.began[id]
}

// force sets the confirmed state and its start together, and drops any pending
// candidate.
//
// Use it whenever a state is applied outside confirm (a queue reset forcing
// idle, a suspend or resume). Writing the confirmed state alone would leave
// began pointing at the *previous* state and leave an armed candidate in
// place, which can then confirm on a single unrelated tick (#188 follow-up).
//
// It also consumes any pending seed: the forced timestamp already wins here,
// so a later confirm must not treat the session as still seeded and keep
// stamping began with this stale forced value forever.
func (d *activityDebouncer) force(id int, state Activity, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.confirmed[id] = state
	d.began[id] = now
	delete(d.pending, id)
	delete(d.since, id)
	delete(d.seeded, id)
}

// seed restores a pane's state-start timestamp after a restart, together with
// the state it belongs to, so a duration survives a restart instead of
// starting over.
//
// A zero time or an empty state is ignored: without a state the seed could
// only attach to whatever confirms first, which is the bug the pairing exists
// to prevent. It is also refused once the session has a confirmed state, since
// it arrives from a client after the session was created and therefore races
// the scan loop; landing after a confirmation it could only mis-stamp a much
// later change.
func (d *activityDebouncer) seed(id int, state Activity, at time.Time) {
	if at.IsZero() || state == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.confirmed[id] != "" {
		return
	}
	d.began[id] = at
	d.seeded[id] = state
}

// forget drops a closed session's debounce state.
func (d *activityDebouncer) forget(id int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.confirmed, id)
	delete(d.pending, id)
	delete(d.since, id)
	delete(d.began, id)
	delete(d.seeded, id)
}

// backdateCandidate moves an armed candidate's clock back by d and reports
// whether there was one. It exists so a test can reach a confirmation without
// sleeping a whole debounce window, and so that a test which expected an armed
// candidate and found none fails loudly instead of passing on nothing.
func (d *activityDebouncer) backdateCandidate(id int, by time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	since, armed := d.since[id]
	if !armed {
		return false
	}
	d.since[id] = since.Add(-by)
	return true
}
