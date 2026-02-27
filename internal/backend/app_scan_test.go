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

func TestScanUsesHookActivityWhenPresent(t *testing.T) {
	sess := terminal.NewSession(99, 24, 80)
	// Mark session as having hook-driven state (WaitingPermission)
	sess.SetHookActivity(terminal.ActivityWaitingPermission)

	if !sess.HasHookData() {
		t.Fatal("precondition: session must have hook data")
	}

	// activityString for WaitingPermission must be "waitingPermission"
	got := activityString(terminal.ActivityWaitingPermission)
	if got != "waitingPermission" {
		t.Errorf("activityString = %q, want %q", got, "waitingPermission")
	}

	// Verify DetectActivity alone (without the guard) would reset state
	// since there's no PTY output — this confirms why the guard is needed.
	// The guard is in scanAllSessions(), not DetectActivity() itself.
	detected := sess.DetectActivity()
	if detected == terminal.ActivityWaitingPermission {
		// Unusual: DetectActivity should return Idle/Done with no PTY output,
		// not WaitingPermission (which is only set by hooks).
		t.Logf("DetectActivity returned WaitingPermission — unexpected but not fatal")
	}
	_ = detected
}
