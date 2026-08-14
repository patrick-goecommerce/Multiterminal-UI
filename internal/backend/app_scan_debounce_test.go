package backend

import (
	"testing"
	"time"
)

// A single differing tick is not a state change: Claude's TUI can blank the
// prompt line mid-repaint, which classifies as "idle" for one tick (issue #188).
func TestConfirmActivityIgnoresSingleTick(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(1000, 0)
	prevActivity[1] = "done"

	if confirmActivity(1, "idle", base) {
		t.Fatal("a first differing tick must not confirm")
	}
	if confirmActivity(1, "done", base.Add(600*time.Millisecond)) {
		t.Fatal("falling back to the previous state must not confirm")
	}
	if got := prevActivity[1]; got != "done" {
		t.Errorf("prevActivity = %q, want %q", got, "done")
	}
}

// A state that holds past the window is a real change.
func TestConfirmActivityAcceptsStableState(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(2000, 0)
	prevActivity[1] = "active"

	if confirmActivity(1, "done", base) {
		t.Fatal("the first observation must only arm, not confirm")
	}
	if confirmActivity(1, "done", base.Add(debounceWindow-time.Millisecond)) {
		t.Fatal("confirmed one millisecond early")
	}
	if !confirmActivity(1, "done", base.Add(debounceWindow)) {
		t.Fatal("a state stable for the full window must confirm")
	}
	if got := prevActivity[1]; got != "done" {
		t.Errorf("prevActivity = %q, want %q", got, "done")
	}
}

// The timestamp marks when the state actually began — the first observation —
// not when it happened to be confirmed one window later.
func TestConfirmActivityStampsFirstObservation(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	base := time.Unix(3000, 0)
	prevActivity[1] = "active"

	confirmActivity(1, "done", base)
	confirmActivity(1, "done", base.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceFor(1); !got.Equal(base) {
		t.Errorf("activitySince = %v, want %v (first observation)", got, base)
	}
}

// Re-confirming the same state must not restart the clock, otherwise the
// duration would reset on every tick.
func TestConfirmActivityKeepsTimestampOnSameState(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	base := time.Unix(4000, 0)
	prevActivity[1] = "active"
	confirmActivity(1, "done", base)
	confirmActivity(1, "done", base.Add(debounceWindow))

	if confirmActivity(1, "done", base.Add(time.Hour)) {
		t.Fatal("an unchanged state must not report a change")
	}
	prevActivityMu.Unlock()

	if got := activitySinceFor(1); !got.Equal(base) {
		t.Errorf("activitySince = %v, want %v (unchanged)", got, base)
	}
}

// Two panes must not share debounce state.
func TestConfirmActivityIsPerSession(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(5000, 0)
	prevActivity[1] = "active"
	prevActivity[2] = "active"

	confirmActivity(1, "done", base)
	if confirmActivity(2, "done", base.Add(debounceWindow)) {
		t.Fatal("session 2 confirmed using session 1's pending timer")
	}
}

// Closing a pane must not leak map entries.
func TestCleanupActivityDebounce(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(6000, 0)
	prevActivity[7] = "active"
	confirmActivity(7, "done", base)
	confirmActivity(7, "done", base.Add(debounceWindow))

	cleanupActivityDebounce(7)

	if _, ok := pendingActivity[7]; ok {
		t.Error("pendingActivity still holds the session")
	}
	if _, ok := pendingSince[7]; ok {
		t.Error("pendingSince still holds the session")
	}
	if _, ok := activitySince[7]; ok {
		t.Error("activitySince still holds the session")
	}
}

// The wire format carries seconds since epoch; 0 means "unknown" so the badge
// can omit the duration instead of rendering 1970.
func TestActivitySinceUnix(t *testing.T) {
	resetActivityDebounceForTest()

	if got := activitySinceUnix(42); got != 0 {
		t.Errorf("unknown session yielded %d, want 0", got)
	}

	base := time.Unix(7000, 0)
	prevActivityMu.Lock()
	prevActivity[42] = "active"
	confirmActivity(42, "done", base)
	confirmActivity(42, "done", base.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceUnix(42); got != 7000 {
		t.Errorf("activitySinceUnix = %d, want 7000", got)
	}
}

// The hook emit path announces a state before confirmActivity has seen it, so
// the recorded timestamp still belongs to the previous state. Handing it out
// with the new label would show "fertig · 3 Std 20" for a pane that just
// finished, then snap back to "gerade eben" ~2 s later. 0 ("unknown") is the
// only honest answer until the state is confirmed.
func TestActivitySinceUnixIfState(t *testing.T) {
	resetActivityDebounceForTest()

	base := time.Unix(11000, 0)
	prevActivityMu.Lock()
	prevActivity[60] = "active"
	activitySince[60] = base
	prevActivityMu.Unlock()

	if got := activitySinceUnixIfState(60, "done"); got != 0 {
		t.Errorf("a state the session has not confirmed yielded %d, want 0 (unknown)", got)
	}
	if got := activitySinceUnixIfState(60, "active"); got != 11000 {
		t.Errorf("the confirmed state yielded %d, want 11000", got)
	}
	if got := activitySinceUnixIfState(61, "done"); got != 0 {
		t.Errorf("an unknown session yielded %d, want 0", got)
	}
}

// forceActivity is used by writers that bypass confirmActivity (queue reset,
// suspend/resume). The forced state's timestamp must win, and any pending
// candidate armed before the force must not survive to confirm on a later
// unrelated tick.
func TestForceActivitySetsTimestampAndClearsPending(t *testing.T) {
	resetActivityDebounceForTest()

	base := time.Unix(8000, 0)
	prevActivityMu.Lock()
	prevActivity[9] = "active"
	// Arm a pending candidate for "done" that has not yet confirmed.
	confirmActivity(9, "done", base)
	prevActivityMu.Unlock()

	forced := base.Add(300 * time.Millisecond)
	prevActivityMu.Lock()
	forceActivity(9, "idle", forced)
	if got := prevActivity[9]; got != "idle" {
		t.Errorf("prevActivity = %q, want %q", got, "idle")
	}
	if _, ok := pendingActivity[9]; ok {
		t.Error("pendingActivity still holds a candidate after force")
	}
	if _, ok := pendingSince[9]; ok {
		t.Error("pendingSince still holds a candidate after force")
	}
	prevActivityMu.Unlock()

	if got := activitySinceFor(9); !got.Equal(forced) {
		t.Errorf("activitySince = %v, want %v", got, forced)
	}

	// The candidate armed before the force must not confirm later just
	// because enough time has passed since it was originally armed.
	prevActivityMu.Lock()
	confirmed := confirmActivity(9, "done", base.Add(debounceWindow))
	prevActivityMu.Unlock()
	if confirmed {
		t.Fatal("a candidate armed before forceActivity confirmed after the force")
	}
}

// A restored session has no prevActivity entry yet (it is a brand-new
// session ID from CreateSession's counter), so its very first observation
// always looks like a fresh transition to confirmActivity — same as a pane
// that was never restored at all. Without special-casing this, the seed
// set by SeedActivitySince would be overwritten by that first confirmation
// within one debounce window (~1.2-2.25s after restart), which defeats the
// entire point of persisting the timestamp across a restart (#189).
func TestSeedActivitySince_SurvivesFirstConfirmationAfterRestart(t *testing.T) {
	resetActivityDebounceForTest()

	seeded := time.Unix(1700000000, 0) // long before "now"
	setActivitySinceFor(50, seeded, "idle")

	restartTick := time.Now()
	prevActivityMu.Lock()
	armed := confirmActivity(50, "idle", restartTick)
	confirmed := confirmActivity(50, "idle", restartTick.Add(debounceWindow))
	prevActivityMu.Unlock()

	if armed {
		t.Error("the first observation confirmed immediately, without the debounce window")
	}
	if !confirmed {
		// Without this the test would still pass if confirmation stopped
		// happening at all — an untouched seed proves nothing on its own.
		t.Fatal("a seeded session's stable state never confirmed")
	}
	if got := activitySinceFor(50); !got.Equal(seeded) {
		t.Errorf("activitySince = %v, want seeded %v (seed was overwritten by the first post-restart confirmation)", got, seeded)
	}

	// The seed is consumed by that first confirmation: the *next* transition
	// is a state that genuinely began now and must be stamped accordingly.
	next := restartTick.Add(time.Hour)
	prevActivityMu.Lock()
	confirmActivity(50, "done", next)
	confirmActivity(50, "done", next.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceFor(50); !got.Equal(next) {
		t.Errorf("activitySince after the second transition = %v, want %v — the seed must not outlive its first confirmation", got, next)
	}
}

// A restored pane re-launches its CLI, and that boot keeps DetectActivity on
// "active" well past the debounce window. The first state confirmed after a
// restart is therefore usually a transient "active", not the state the badge
// showed before the restart. A state-blind seed would attach to it and claim
// the pane had been running for hours; then, once it settled on "done", the
// seed would be spent and the duration would restart at "gerade eben".
func TestSeedActivitySince_DroppedWhenAnotherStateConfirmsFirst(t *testing.T) {
	resetActivityDebounceForTest()

	seeded := time.Unix(1700000000, 0)
	setActivitySinceFor(52, seeded, "done")

	boot := time.Unix(1800000000, 0) // the CLI booting after the restart
	prevActivityMu.Lock()
	confirmActivity(52, "active", boot)
	confirmActivity(52, "active", boot.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceFor(52); !got.Equal(boot) {
		t.Errorf("activitySince = %v, want %v — a seed for 'done' must not be consumed by the boot's 'active'", got, boot)
	}

	// And it must be gone, not lying in wait for the real "done".
	settle := boot.Add(20 * time.Second)
	prevActivityMu.Lock()
	confirmActivity(52, "done", settle)
	confirmActivity(52, "done", settle.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceFor(52); !got.Equal(settle) {
		t.Errorf("activitySince = %v, want %v — a dropped seed must not resurface on a later matching state", got, settle)
	}
}

// The seed arrives through a binding called after CreateSession returns, so it
// races the scan loop. Landing after the session already has a confirmed state
// it has nothing left to correct, and applying it would back-date a state that
// demonstrably began after the restart.
func TestSeedActivitySince_IgnoredOnceAStateIsConfirmed(t *testing.T) {
	resetActivityDebounceForTest()

	base := time.Unix(9500, 0)
	prevActivityMu.Lock()
	confirmActivity(53, "active", base)
	confirmActivity(53, "active", base.Add(debounceWindow))
	prevActivityMu.Unlock()

	setActivitySinceFor(53, time.Unix(1700000000, 0), "active")

	if got := activitySinceFor(53); !got.Equal(base) {
		t.Errorf("activitySince = %v, want %v — a late seed must not overwrite a confirmed state's start", got, base)
	}
	prevActivityMu.Lock()
	_, stillSeeded := seededActivity[53]
	prevActivityMu.Unlock()
	if stillSeeded {
		t.Error("a refused seed was still recorded and would hijack the next confirmation")
	}
}

// A seed without a state cannot be matched against anything, so it must be
// refused outright rather than attaching to whatever confirms first.
func TestSeedActivitySince_RequiresAState(t *testing.T) {
	resetActivityDebounceForTest()

	setActivitySinceFor(54, time.Unix(1700000000, 0), "")

	base := time.Unix(9600, 0)
	prevActivityMu.Lock()
	confirmActivity(54, "done", base)
	confirmActivity(54, "done", base.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceFor(54); !got.Equal(base) {
		t.Errorf("activitySince = %v, want %v (stateless seed must be ignored)", got, base)
	}
}

// A genuinely new (never-restored) pane must keep its existing behaviour:
// no seed means the first confirmation stamps the observation time, same
// as before this map existed.
func TestConfirmActivity_UnseededFreshSessionStampsObservationTime(t *testing.T) {
	resetActivityDebounceForTest()

	base := time.Unix(9000, 0)
	prevActivityMu.Lock()
	confirmActivity(51, "idle", base)
	confirmActivity(51, "idle", base.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceFor(51); !got.Equal(base) {
		t.Errorf("activitySince = %v, want %v (first observation, unseeded)", got, base)
	}
}
