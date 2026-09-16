package hub

import (
	"strings"
	"testing"
)

// AgentState is the whole vocabulary in one function, and every caller (the
// MCP tool, the CLI, a future remote client) reads a session through it. The
// table is therefore the contract, not an implementation detail.
func TestAgentState(t *testing.T) {
	cases := []struct {
		name    string
		summary SessionSummary
		want    string
	}{
		{"a working agent is idle for waiting purposes",
			SessionSummary{Status: StatusRunning, Activity: ActivityActive}, "idle"},
		{"a finished agent is done",
			SessionSummary{Status: StatusRunning, Activity: ActivityDone}, "done"},
		{"a permission prompt blocks",
			SessionSummary{Status: StatusRunning, Activity: ActivityWaitingPermission}, "blocked"},
		{"a question blocks the same way",
			SessionSummary{Status: StatusRunning, Activity: ActivityWaitingAnswer}, "blocked"},
		{"an error needs a human, so it blocks",
			SessionSummary{Status: StatusRunning, Activity: ActivityError}, "blocked"},
		{"a gone process has exited",
			SessionSummary{Status: StatusExited, Activity: ActivityDone}, "exited"},
		{"a failed process has exited too",
			SessionSummary{Status: StatusError, Activity: ActivityActive}, "exited"},
		// A sleeping pane is finished work whose process was released on
		// purpose. For a caller waiting on the outcome that is done, not gone.
		{"a sleeping pane is done, not exited",
			SessionSummary{Status: StatusSuspended, Activity: ActivityIdle}, "done"},
		{"a pane on its way to sleep is already done",
			SessionSummary{Status: StatusSuspending, Activity: ActivityIdle}, "done"},
		// The status wins over the activity: a suspended pane keeps whatever
		// activity it had when it went to sleep, and that must not leak out.
		{"the status wins over a stale activity",
			SessionSummary{Status: StatusSuspended, Activity: ActivityWaitingPermission}, "done"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AgentState(tc.summary); got != tc.want {
				t.Errorf("AgentState = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeAgentStates_EmptyMeansDoneOrBlocked(t *testing.T) {
	got, err := NormalizeAgentStates(nil)
	if err != nil {
		t.Fatalf("NormalizeAgentStates: %v", err)
	}
	if !got["done"] || !got["blocked"] {
		t.Errorf("default set = %v, want done and blocked", got)
	}
	if got["idle"] || got["exited"] {
		t.Errorf("default set = %v, want nothing beyond done and blocked", got)
	}
}

func TestNormalizeAgentStates_IsForgivingAboutSpelling(t *testing.T) {
	got, err := NormalizeAgentStates([]string{" DONE ", "Blocked"})
	if err != nil {
		t.Fatalf("NormalizeAgentStates: %v", err)
	}
	if !got["done"] || !got["blocked"] {
		t.Errorf("set = %v, want done and blocked", got)
	}
}

// An unknown state is a typo, and the error has to be usable without the
// source at hand: it names both the bad value and the valid ones.
func TestNormalizeAgentStates_RejectsAnUnknownState(t *testing.T) {
	_, err := NormalizeAgentStates([]string{"finished"})
	if err == nil {
		t.Fatal("an unknown state was accepted")
	}
	if !strings.Contains(err.Error(), "finished") {
		t.Errorf("error = %q, want it to name the bad state", err)
	}
	for _, want := range []string{"blocked", "done", "exited", "idle"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to list %q", err, want)
		}
	}
}
