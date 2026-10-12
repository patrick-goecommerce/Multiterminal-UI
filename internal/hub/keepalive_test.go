package hub

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
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

// The nudge ends in a Return, and a Return answers whatever the pane is asking.
// A prompt left open for hours is exactly what the keep-alive would find.
func TestKeepAlive_NeverTypesIntoAPaneThatIsAsking(t *testing.T) {
	for _, activity := range []Activity{ActivityWaitingPermission, ActivityWaitingAnswer, ActivityActive, ActivityError} {
		t.Run(string(activity), func(t *testing.T) {
			h := policyFor(t, claudePolicy())
			id := adoptForKeepAlive(t, h, 1, "claude", time.Now().Add(-2*time.Hour))
			if err := h.SetHookActivity(id, activity); err != nil {
				t.Fatal(err)
			}
			if got, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
				t.Errorf("picked session %d while it was %s", got, activity)
			}
		})
	}
}

// A pane that is asking is skipped, not a reason to give up: the next one at
// rest still gets the nudge.
func TestKeepAlive_SkipsTheAskingPaneForTheNextOne(t *testing.T) {
	h := policyFor(t, claudePolicy())
	long := time.Now().Add(-2 * time.Hour)
	asking := adoptForKeepAlive(t, h, 1, "claude", long)
	adoptForKeepAlive(t, h, 2, "claude", long)
	if err := h.SetHookActivity(asking, ActivityWaitingPermission); err != nil {
		t.Fatal(err)
	}
	if err := h.SetHookActivity(2, ActivityDone); err != nil {
		t.Fatal(err)
	}

	if id, ok := h.keepAliveTarget(time.Now(), time.Time{}); !ok || id != 2 {
		t.Errorf("target = (%d, %v), want (2, true)", id, ok)
	}
}

// A pane that finished and nobody has seen is left alone: the greeting's reply
// would settle as done and seen and erase its entry in the waiting list.
func TestKeepAlive_LeavesAnUnseenFinishedPaneAlone(t *testing.T) {
	h := policyFor(t, claudePolicy())
	long := time.Now().Add(-2 * time.Hour)
	adoptForKeepAlive(t, h, 1, "claude", long)
	adoptForKeepAlive(t, h, 2, "claude", long)
	for _, id := range []int{1, 2} {
		if err := h.SetHookActivity(id, ActivityDone); err != nil {
			t.Fatal(err)
		}
	}
	h.attention.confirmed(1, ActivityDone, long)

	if id, ok := h.keepAliveTarget(time.Now(), time.Time{}); !ok || id != 2 {
		t.Errorf("target = (%d, %v), want (2, true)", id, ok)
	}

	h.attention.markSeen(1, true)
	if id, ok := h.keepAliveTarget(time.Now(), time.Time{}); !ok || id != 1 {
		t.Errorf("after seeing it, target = (%d, %v), want (1, true)", id, ok)
	}
}

// A session another agent opened is that agent's conversation.
func TestKeepAlive_LeavesAgentSessionsAlone(t *testing.T) {
	h := policyFor(t, claudePolicy())
	id := adoptForKeepAlive(t, h, 1, "claude", time.Now().Add(-2*time.Hour))
	h.SetOriginForTest(id, OriginAgent)

	if _, ok := h.keepAliveTarget(time.Now(), time.Time{}); ok {
		t.Error("nudged a session an agent opened")
	}
}

// A greeting that reached the session marks it, so the reply settles as done
// and seen (attentionBook.settleNudge) instead of an hourly question.
func TestKeepAlive_SendingMarksTheSession(t *testing.T) {
	h := policyFor(t, claudePolicy())
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: sessionDir(t)})
	if err != nil {
		t.Fatal(err)
	}
	h.sendKeepAlive(id)
	h.attention.mu.Lock()
	marked := h.attention.nudged[id]
	h.attention.mu.Unlock()
	if !marked {
		t.Error("the keep-alive typed into the session without marking it")
	}
}

func nudged(h *Embedded, id int) bool {
	h.attention.mu.Lock()
	defer h.attention.mu.Unlock()
	return h.attention.nudged[id]
}

// Nudge is the same greeting on demand (the app's startup ping): it marks the
// turn like the loop does, and refuses where it has nothing to type.
func TestNudge_MarksTheSession(t *testing.T) {
	h := policyFor(t, claudePolicy())
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: sessionDir(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Nudge(id); err != nil {
		t.Fatalf("Nudge: %v", err)
	}
	if !nudged(h, id) {
		t.Error("Nudge typed into the session without marking it")
	}
}

func TestNudge_UnknownSession(t *testing.T) {
	h := policyFor(t, claudePolicy())
	if err := h.Nudge(99); err == nil {
		t.Error("Nudge on an unknown session: want an error")
	}
}

func TestNudge_WithoutAMessageTypesNothing(t *testing.T) {
	h := policyFor(t, KeepAlive{Every: time.Hour, Modes: []string{"claude"}})
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: sessionDir(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Nudge(id); err == nil {
		t.Error("Nudge without a message: want an error")
	}
	if nudged(h, id) {
		t.Error("a nudge that typed nothing marked the session")
	}
}

// adoptForKeepAlive registers a session with a known mode and last-output
// time.
//
// No process is started. With one, the silence the test sets up was a race:
// a pseudoconsole writes its startup sequences a moment after the process
// starts, which moved LastOutputAt back to "now" after the test had dated it
// two hours back, and the keep-alive then rightly saw no silence. That turned
// the Windows run red on a slow runner. The keep-alive only reads the
// summary, so a session object in the right state is all it needs.
func adoptForKeepAlive(t *testing.T, h *Embedded, id int, mode string, lastOutput time.Time) int {
	t.Helper()
	h.AdoptForTest(id, terminal.NewSession(id, 24, 80))
	h.mu.Lock()
	h.sessions[id].spec.Mode = mode
	h.mu.Unlock()
	h.setLastOutputForTest(id, lastOutput)
	return id
}
