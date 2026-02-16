package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ---------------------------------------------------------------------------
// activityString — maps ActivityState to frontend event strings
// These strings drive the CSS classes for pane border colors:
//   "done"       → green glow (Claude finished)
//   "needsInput" → yellow pulse (needs user confirmation)
//   "active"     → normal active state
//   "idle"       → no special styling
// ---------------------------------------------------------------------------

func TestActivityString_AllStates(t *testing.T) {
	tests := []struct {
		state terminal.ActivityState
		want  string
	}{
		{terminal.ActivityIdle, "idle"},
		{terminal.ActivityActive, "active"},
		{terminal.ActivityDone, "done"},
		{terminal.ActivityNeedsInput, "needsInput"},
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
