package hub

import (
	"testing"
	"time"
)

// The keep-alive picks a session and decides whether to nudge it. Both halves
// are easy to get subtly wrong in ways nobody notices for hours, so they are
// tested through keepAliveTarget rather than through the loop.

// policyFor builds a host whose keep-alive policy is fixed.
func policyFor(t *testing.T, k KeepAlive) *Embedded {
	t.Helper()
	h := NewEmbedded(Options{Version: "test"})
	t.Cleanup(h.Release)
	h.keepAliveFn = func() KeepAlive { return k }
	return h
}

func claudePolicy() KeepAlive {
	return KeepAlive{Every: time.Hour, Message: "Hi!", Modes: []string{"claude"}}
}

// A policy with nothing in it does nothing. An empty message in particular:
// sending a bare Return into an agent is a stray keystroke, not a keep-alive.
func TestKeepAlive_OffPolicies(t *testing.T) {
	cases := map[string]KeepAlive{
		"no interval": {Message: "Hi!", Modes: []string{"claude"}},
		"no message":  {Every: time.Hour, Modes: []string{"claude"}},
		"no modes":    {Every: time.Hour, Message: "Hi!"},
		"nothing":     {},
	}
	for name, policy := range cases {
		t.Run(name, func(t *testing.T) {
			if !policy.off() {
				t.Error("policy should be off")
			}
			h := policyFor(t, policy)
			if _, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
				t.Error("an off policy picked a target")
			}
		})
	}
}

func TestKeepAlive_NoSessionsMeansNoTarget(t *testing.T) {
	h := policyFor(t, claudePolicy())
	if _, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
		t.Error("picked a target with no sessions")
	}
}

// The oldest eligible pane wins. IDs are handed out in order, so the lowest is
// the one open longest, and nudging a different pane each time would scatter
// the message across a workspace.
func TestKeepAlive_PicksTheOldestEligibleSession(t *testing.T) {
	h := policyFor(t, claudePolicy())
	long := time.Now().Add(-2 * time.Hour)
	adoptForKeepAlive(t, h, 3, "claude", long)
	adoptForKeepAlive(t, h, 7, "claude", long)

	id, ok := h.keepAliveTarget(time.Now(), time.Time{})
	if !ok {
		t.Fatal("no target picked")
	}
	if id != 3 {
		t.Errorf("target = %d, want the oldest (3)", id)
	}
}

// A shell pane is not an agent and must never be typed into.
func TestKeepAlive_SkipsIneligibleModes(t *testing.T) {
	h := policyFor(t, claudePolicy())
	adoptForKeepAlive(t, h, 1, "shell", time.Now().Add(-2*time.Hour))

	if _, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
		t.Error("a shell pane was picked")
	}
}

// Silence is measured across EVERY session, not just the target's. Somebody
// working in the next pane is not idle, and a message typed into a workspace
// its owner never left is noise.
func TestKeepAlive_AnotherBusySessionCountsAsActivity(t *testing.T) {
	h := policyFor(t, claudePolicy())
	adoptForKeepAlive(t, h, 1, "claude", time.Now().Add(-2*time.Hour))
	adoptForKeepAlive(t, h, 2, "shell", time.Now()) // somebody is typing here

	if _, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
		t.Error("nudged while another pane was active")
	}
}

func TestKeepAlive_WaitsOutTheInterval(t *testing.T) {
	h := policyFor(t, claudePolicy())
	adoptForKeepAlive(t, h, 1, "claude", time.Now().Add(-30*time.Minute))

	if _, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
		t.Error("nudged after 30 minutes of a one-hour interval")
	}
}

// Two nudges inside one interval would double up on a pane that stays quiet.
func TestKeepAlive_DoesNotNudgeTwicePerInterval(t *testing.T) {
	h := policyFor(t, claudePolicy())
	now := time.Now()
	adoptForKeepAlive(t, h, 1, "claude", now.Add(-2*time.Hour))

	if _, ok := h.keepAliveTarget(now, now.Add(-10*time.Minute)); ok {
		t.Error("nudged again ten minutes after the last one")
	}
	if _, ok := h.keepAliveTarget(now, now.Add(-90*time.Minute)); !ok {
		t.Error("did not nudge ninety minutes after the last one")
	}
}

// A pane whose process is gone cannot be nudged.
func TestKeepAlive_SkipsSessionsThatAreNotRunning(t *testing.T) {
	h := policyFor(t, claudePolicy())
	id := adoptForKeepAlive(t, h, 1, "claude", time.Now().Add(-2*time.Hour))
	if err := h.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
		t.Error("picked a session that is gone")
	}
}

// adoptForKeepAlive starts a session with a known mode and last-output time.
func adoptForKeepAlive(t *testing.T, h *Embedded, id int, mode string, lastOutput time.Time) int {
	t.Helper()
	got, err := h.Create(CreateSpec{
		ID: id, Argv: sleepArgv(), Dir: sessionDir(t), Rows: 24, Cols: 80, Mode: mode,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	h.setLastOutputForTest(got, lastOutput)
	return got
}
