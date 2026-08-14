package backend

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// TestActivityInfoCarriesStatuslineFields guards that the event payload exposes
// context%/model so the frontend can render them.
func TestActivityInfoCarriesStatuslineFields(t *testing.T) {
	info := ActivityInfo{ID: 1, Activity: "active", Cost: "$1.23", ContextPct: 40, Model: "Opus 4.8"}
	if info.ContextPct != 40 || info.Model != "Opus 4.8" {
		t.Fatalf("ActivityInfo = %+v, want ContextPct=40 Model=Opus 4.8", info)
	}
}

// ---------------------------------------------------------------------------
// activityString — maps ActivityState to frontend event strings
// These strings drive the CSS classes for pane border colors:
//   "done"               → green glow (Claude finished)
//   "waitingPermission"  → yellow pulse (tool approval needed)
//   "waitingAnswer"      → yellow pulse (text input needed)
//   "error"              → red indicator (tool execution failed)
//   "active"             → normal active state
//   "idle"               → no special styling
// ---------------------------------------------------------------------------

func TestActivityString_AllStates(t *testing.T) {
	tests := []struct {
		state terminal.ActivityState
		want  string
	}{
		{terminal.ActivityIdle, "idle"},
		{terminal.ActivityActive, "active"},
		{terminal.ActivityDone, "done"},
		{terminal.ActivityWaitingPermission, "waitingPermission"},
		{terminal.ActivityWaitingAnswer, "waitingAnswer"},
		{terminal.ActivityError, "error"},
	}
	for _, tt := range tests {
		got := activityString(tt.state)
		if got != tt.want {
			t.Errorf("activityString(%d) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestActivityString_UnknownState(t *testing.T) {
	// Any unknown state should default to "idle"
	got := activityString(terminal.ActivityState(99))
	if got != "idle" {
		t.Errorf("activityString(99) = %q, want 'idle'", got)
	}
}

func TestScan_TracksOSCTitleChange(t *testing.T) {
	sess := terminal.NewSession(7, 24, 80)
	// OSC 2 ; <title> BEL — Claude/shell sets the window title
	sess.Screen.Write([]byte("\x1b]2;my-pane\x07"))

	app := &AppService{
		sessions: map[int]*terminal.Session{7: sess},
		queues:   map[int]*sessionQueue{},
	}

	cleanupActivityTracking(7) // start from a clean tracking state
	app.scanAllSessions()

	prevActivityMu.Lock()
	got := prevTitle[7]
	prevActivityMu.Unlock()

	if got != "my-pane" {
		t.Fatalf("after scan, prevTitle[7] = %q, want %q", got, "my-pane")
	}

	cleanupActivityTracking(7)
	prevActivityMu.Lock()
	_, exists := prevTitle[7]
	prevActivityMu.Unlock()
	if exists {
		t.Fatal("cleanupActivityTracking should remove the prevTitle entry")
	}
}

// TestScanGuard_StaleActiveHookFallsBackToScreen reproduces the reported bug:
// once a pane's first UserPromptSubmit hook fires, HasHookData() latches true
// forever (only SessionEnd clears it), so the PTY heuristic is skipped for
// the rest of the session's life. If the terminating Stop hook event is lost
// or delayed, the pane — and the pipeline queue waiting on its "done"
// transition — hung on "active" forever. The scan must fall back to the PTY
// screen once output has gone stale.
func TestScanGuard_StaleActiveHookFallsBackToScreen(t *testing.T) {
	sess := terminal.NewSession(9, 10, 80)
	sess.SetHookActivity(terminal.ActivityActive) // e.g. from UserPromptSubmit/PostToolUse
	sess.Screen.Write([]byte(
		"\x1b[32m✓ Task completed successfully\x1b[0m\r\n" +
			"\x1b[1;35m❯\x1b[0m ",
	))
	// Simulate the Stop hook never arriving: PTY output stopped a while ago.
	sess.SetLastOutputAtForTest(time.Now().Add(-2 * time.Second))

	app := &AppService{
		sessions: map[int]*terminal.Session{9: sess},
		queues:   map[int]*sessionQueue{},
	}

	cleanupActivityTracking(9)
	// A single tick only arms the debounce candidate (confirmActivity, task 3 /
	// issue #188) — it takes debounceWindow of a stable state to confirm. Back-
	// date the pending timestamp instead of sleeping the test, then tick again
	// so the candidate confirms.
	app.scanAllSessions()
	prevActivityMu.Lock()
	if since, ok := pendingSince[9]; ok {
		pendingSince[9] = since.Add(-debounceWindow)
	}
	prevActivityMu.Unlock()
	app.scanAllSessions()

	got := activityString(sess.GetActivity())
	// scanAllSessions doesn't persist the fallback into sess.Activity (same as
	// the existing done→waitingAnswer cross-check), so assert on the emitted
	// state via prevActivity instead of GetActivity().
	prevActivityMu.Lock()
	emitted := prevActivity[9]
	prevActivityMu.Unlock()
	if emitted != "done" {
		t.Fatalf("after scan with stale active hook + completed-prompt screen, emitted activity = %q (raw hook state %q), want %q — Stop-event-lost fallback not working", emitted, got, "done")
	}
}

func TestScanGuard_HookActivityNotOverwrittenByScan(t *testing.T) {
	// Setup: a session with hook-driven WaitingPermission state
	// and NO PTY output (LastOutputAt = zero, no screen content).
	// Without the guard, DetectActivity() would return Idle or Done
	// (since there's no PTY output matching the needsInput pattern).
	// With the guard, the session stays at WaitingPermission.
	sess := terminal.NewSession(42, 24, 80)
	sess.SetHookActivity(terminal.ActivityWaitingPermission)

	// Build a minimal AppService with this session
	app := &AppService{
		sessions: map[int]*terminal.Session{42: sess},
		queues:   map[int]*sessionQueue{},
	}

	// Run one scan cycle
	app.scanAllSessions()

	// After scanning, the activity must still be WaitingPermission
	// (the hook guard must have prevented DetectActivity() from resetting it)
	got := activityString(sess.GetActivity())
	if got != "waitingPermission" {
		t.Errorf("after scan, activity = %q, want %q — hook guard not working", got, "waitingPermission")
	}
}
