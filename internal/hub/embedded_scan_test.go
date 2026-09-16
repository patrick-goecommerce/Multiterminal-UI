package hub

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// The activity strings drive the pane border colours in the UI:
//
//	"done"              green glow, the agent finished
//	"waitingPermission" yellow pulse, a tool needs approval
//	"waitingAnswer"     yellow pulse, text input needed
//	"error"             red, a tool failed
//	"active"            working
//	"idle"              nothing special
//
// They are a wire format, so the mapping is pinned in both directions.
func TestActivityMapping_RoundTrips(t *testing.T) {
	cases := []struct {
		state terminal.ActivityState
		want  Activity
	}{
		{terminal.ActivityIdle, ActivityIdle},
		{terminal.ActivityActive, ActivityActive},
		{terminal.ActivityDone, ActivityDone},
		{terminal.ActivityWaitingPermission, ActivityWaitingPermission},
		{terminal.ActivityWaitingAnswer, ActivityWaitingAnswer},
		{terminal.ActivityError, ActivityError},
	}
	for _, c := range cases {
		if got := activityOf(c.state); got != c.want {
			t.Errorf("activityOf(%d) = %q, want %q", c.state, got, c.want)
		}
		if got := terminalActivity(c.want); got != c.state {
			t.Errorf("terminalActivity(%q) = %d, want %d", c.want, got, c.state)
		}
	}
}

// An unknown value on either side is idle, not an empty string or a panic: it
// crosses a protocol boundary, so a peer from another version must land
// somewhere harmless.
func TestActivityMapping_UnknownIsIdle(t *testing.T) {
	if got := activityOf(terminal.ActivityState(99)); got != ActivityIdle {
		t.Errorf("activityOf(99) = %q, want %q", got, ActivityIdle)
	}
	if got := terminalActivity(Activity("something-else")); got != terminal.ActivityIdle {
		t.Errorf("terminalActivity(unknown) = %d, want %d", got, terminal.ActivityIdle)
	}
}

// A sleeping pane's screen is frozen, so the scan must skip it rather than
// re-report the state it had before it fell asleep (#180).
func TestScanActivity_SkipsASleepingPane(t *testing.T) {
	h := newTestHost(t, nil)
	sess := terminal.NewSession(1, 24, 80)
	sess.SetHookActivity(terminal.ActivityDone)
	h.AdoptForTest(1, sess)

	if !sess.TrySuspend() {
		t.Fatal("TrySuspend refused a done session")
	}

	results := h.ScanActivity()
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !results[0].Asleep {
		t.Error("a suspending pane was scanned instead of skipped")
	}
}

// A hook that said "done" is trusted, except when the screen shows the agent
// asked something: the Stop hook fires before the question is on screen.
func TestScanActivity_DoneWithATrailingQuestionBecomesWaitingAnswer(t *testing.T) {
	h := newTestHost(t, nil)
	// A 10-row screen: the classifier only looks at the last 15 rows, so a
	// taller screen would leave this content above the window it scans.
	sess := terminal.NewSession(1, 10, 80)
	sess.Screen.Write([]byte("Was liegt an?\r\n\x1b[1;35m❯\x1b[0m "))
	sess.SetHookActivity(terminal.ActivityDone)
	h.AdoptForTest(1, sess)

	results := h.ScanActivity()
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Activity != ActivityWaitingAnswer {
		t.Errorf("activity = %q, want %q", results[0].Activity, ActivityWaitingAnswer)
	}
}

// Hook state is authoritative: a session with no output at all must keep what
// the hook said rather than being reclassified as idle.
func TestScanActivity_HookStateSurvivesAScanWithNoOutput(t *testing.T) {
	h := newTestHost(t, nil)
	sess := terminal.NewSession(1, 24, 80)
	sess.SetHookActivity(terminal.ActivityWaitingPermission)
	h.AdoptForTest(1, sess)

	results := h.ScanActivity()
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Activity != ActivityWaitingPermission {
		t.Errorf("activity = %q, want %q", results[0].Activity, ActivityWaitingPermission)
	}
}

// The host scans on its own so a daemon nobody is watching still knows what
// its agents are doing.
func TestScanLoop_ReportsWithoutBeingAsked(t *testing.T) {
	sink := newSink()
	h := NewEmbedded(Options{Scan: true, Sink: sink})
	t.Cleanup(h.Release)

	if _, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sink.await(t, EventSessionScan)
}

// Without a session there is nothing to report, and a tick that emits an
// empty report every half second would be noise on every client's socket.
func TestScanLoop_SaysNothingWithNoSessions(t *testing.T) {
	sink := newSink()
	h := NewEmbedded(Options{Scan: true, Sink: sink})
	t.Cleanup(h.Release)

	select {
	case name := <-sink.got:
		t.Errorf("an idle host emitted %q", name)
	case <-time.After(1500 * time.Millisecond):
	}
}

// The tick slows down as panes pile up: the scan reads every screen.
func TestScanTick_SlowsDownWithMoreSessions(t *testing.T) {
	if scanTick(1) >= scanTick(5) || scanTick(5) >= scanTick(20) {
		t.Errorf("ticks do not increase: %v, %v, %v", scanTick(1), scanTick(5), scanTick(20))
	}
}

// Release stops the loop; a ticker left running would keep a released host
// alive and keep reading screens nobody owns any more.
func TestScanLoop_StopsOnRelease(t *testing.T) {
	sink := newSink()
	h := NewEmbedded(Options{Scan: true, Sink: sink})
	if _, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sink.await(t, EventSessionScan)

	h.Release()
	// Drain whatever was already queued, then require silence.
	for len(sink.got) > 0 {
		<-sink.got
	}
	select {
	case name := <-sink.got:
		t.Errorf("a released host emitted %q", name)
	case <-time.After(1500 * time.Millisecond):
	}
}
