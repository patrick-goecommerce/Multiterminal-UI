package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

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
