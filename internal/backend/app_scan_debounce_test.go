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
