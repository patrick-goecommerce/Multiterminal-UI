package hub

import (
	"strings"
	"testing"
	"time"
)

// These came over from internal/backend with the debouncer. They are the
// reason #188 and #189 stayed fixed, so they are worth keeping exactly as
// sharp as they were.

// A single differing tick is not a state change: an agent's TUI can blank the
// prompt line mid-repaint, and that classifies as "idle" for one tick (#188).
func TestDebounce_IgnoresASingleTick(t *testing.T) {
	d := newActivityDebouncer()
	base := time.Unix(1000, 0)
	d.force(1, ActivityDone, base)

	if changed, _, _ := d.confirm(1, ActivityIdle, base); changed {
		t.Fatal("a first differing tick must not confirm")
	}
	if changed, _, _ := d.confirm(1, ActivityDone, base.Add(600*time.Millisecond)); changed {
		t.Fatal("falling back to the previous state must not confirm")
	}
	if state, _ := d.state(1); state != ActivityDone {
		t.Errorf("confirmed = %q, want %q", state, ActivityDone)
	}
}

// A state that holds past the window is a real change.
func TestDebounce_AcceptsAStableState(t *testing.T) {
	d := newActivityDebouncer()
	base := time.Unix(2000, 0)
	d.force(1, ActivityActive, base)

	if changed, _, _ := d.confirm(1, ActivityDone, base); changed {
		t.Fatal("the first observation must only arm, not confirm")
	}
	if changed, _, _ := d.confirm(1, ActivityDone, base.Add(debounceWindow-time.Millisecond)); changed {
		t.Fatal("confirmed one millisecond early")
	}
	changed, state, _ := d.confirm(1, ActivityDone, base.Add(debounceWindow))
	if !changed {
		t.Fatal("a state stable for the full window must confirm")
	}
	if state != ActivityDone {
		t.Errorf("confirmed = %q, want %q", state, ActivityDone)
	}
}

// The timestamp marks when the state began, which is the first observation and
// not the moment it survived the window. Otherwise every duration shown in the
// UI is short by up to a window.
func TestDebounce_StampsTheFirstObservation(t *testing.T) {
	d := newActivityDebouncer()
	first := time.Unix(3000, 0)
	d.force(1, ActivityActive, first.Add(-time.Hour))

	d.confirm(1, ActivityDone, first)
	_, _, began := d.confirm(1, ActivityDone, first.Add(debounceWindow))
	if !began.Equal(first) {
		t.Errorf("began = %s, want the first observation %s", began, first)
	}
}

// Re-observing the confirmed state must not restart its clock, or a pane that
// has been done for an hour would keep reading "gerade eben".
func TestDebounce_KeepsTheTimestampOnTheSameState(t *testing.T) {
	d := newActivityDebouncer()
	began := time.Unix(4000, 0)
	d.force(1, ActivityDone, began)

	d.confirm(1, ActivityDone, began.Add(time.Hour))
	if _, got := d.state(1); !got.Equal(began) {
		t.Errorf("began = %s, want it unchanged at %s", got, began)
	}
}

func TestDebounce_IsPerSession(t *testing.T) {
	d := newActivityDebouncer()
	base := time.Unix(5000, 0)
	d.force(1, ActivityActive, base)
	d.force(2, ActivityActive, base)

	// Session 1 holds a new state; session 2 flickers back and forth.
	d.confirm(1, ActivityDone, base)
	d.confirm(2, ActivityDone, base)
	d.confirm(2, ActivityActive, base.Add(500*time.Millisecond))

	if changed, _, _ := d.confirm(1, ActivityDone, base.Add(debounceWindow)); !changed {
		t.Error("session 1 should have confirmed")
	}
	if state, _ := d.state(2); state != ActivityActive {
		t.Errorf("session 2 confirmed = %q, want it still %q", state, ActivityActive)
	}
}

func TestDebounce_ForgetDropsEverything(t *testing.T) {
	d := newActivityDebouncer()
	base := time.Unix(6000, 0)
	d.force(1, ActivityDone, base)
	d.confirm(1, ActivityIdle, base) // arm a candidate too
	d.seed(2, ActivityDone, base)

	d.forget(1)

	if state, began := d.state(1); state != "" || !began.IsZero() {
		t.Errorf("after forget: state = %q, began = %s, want both empty", state, began)
	}
	if _, armed := d.pending[1]; armed {
		t.Error("forget left an armed candidate behind")
	}
	if state, _ := d.state(2); state != "" {
		t.Error("forget touched another session")
	}
}

// force is for a state applied from outside the screen: a queue reset, a
// suspend. Setting the state alone would leave the timestamp on the previous
// one and leave an armed candidate that can confirm on a single unrelated tick
// (#188 follow-up).
func TestDebounce_ForceStampsAndClearsTheCandidate(t *testing.T) {
	d := newActivityDebouncer()
	base := time.Unix(7000, 0)
	d.force(1, ActivityActive, base.Add(-time.Hour))
	d.confirm(1, ActivityDone, base) // arms a candidate

	forced := base.Add(time.Minute)
	d.force(1, ActivityIdle, forced)

	state, began := d.state(1)
	if state != ActivityIdle {
		t.Errorf("state = %q, want %q", state, ActivityIdle)
	}
	if !began.Equal(forced) {
		t.Errorf("began = %s, want the forced time %s", began, forced)
	}
	// The armed "done" candidate must be gone, or one more tick of "done"
	// would confirm it straight through the window.
	if changed, _, _ := d.confirm(1, ActivityDone, forced.Add(time.Millisecond)); changed {
		t.Error("a stale candidate survived force and confirmed on one tick")
	}
}

// #189: a restored pane keeps the duration it had before the restart.
func TestDebounce_SeedSurvivesTheFirstConfirmation(t *testing.T) {
	d := newActivityDebouncer()
	seeded := time.Unix(8000, 0)
	d.seed(1, ActivityDone, seeded)

	base := seeded.Add(3 * time.Hour)
	d.confirm(1, ActivityDone, base)
	_, _, began := d.confirm(1, ActivityDone, base.Add(debounceWindow))

	if !began.Equal(seeded) {
		t.Errorf("began = %s, want the seeded %s", began, seeded)
	}
}

// A restored pane re-launches its CLI, and that boot reads as "active" for
// well over a window. A state-blind seed would be swallowed by it and show a
// three-hour-old timestamp on a two-second-old state.
func TestDebounce_SeedIsDroppedWhenAnotherStateConfirmsFirst(t *testing.T) {
	d := newActivityDebouncer()
	seeded := time.Unix(9000, 0)
	d.seed(1, ActivityDone, seeded)

	base := seeded.Add(3 * time.Hour)
	d.confirm(1, ActivityActive, base)
	_, _, began := d.confirm(1, ActivityActive, base.Add(debounceWindow))

	if began.Equal(seeded) {
		t.Error("the seed was applied to a state it does not describe")
	}
	if !began.Equal(base) {
		t.Errorf("began = %s, want the observation time %s", began, base)
	}
}

// The seed arrives from a client after the session was created, so it races
// the scan. Landing after a confirmation it could only mis-stamp a later one.
func TestDebounce_SeedIsRefusedOnceAStateIsConfirmed(t *testing.T) {
	d := newActivityDebouncer()
	now := time.Unix(10000, 0)
	d.force(1, ActivityDone, now)

	d.seed(1, ActivityDone, now.Add(-5*time.Hour))

	if _, began := d.state(1); !began.Equal(now) {
		t.Errorf("began = %s, want the confirmed %s", began, now)
	}
}

// Without a state the seed could only attach to whatever confirms first, which
// is the bug the pairing exists to prevent.
func TestDebounce_SeedNeedsBothHalves(t *testing.T) {
	d := newActivityDebouncer()
	at := time.Unix(11000, 0)

	d.seed(1, "", at)
	d.seed(2, ActivityDone, time.Time{})

	for _, id := range []int{1, 2} {
		if _, began := d.state(id); !began.IsZero() {
			t.Errorf("session %d took an incomplete seed: began = %s", id, began)
		}
	}
}

// A session nobody seeded gets the honest answer: when the state was observed.
func TestDebounce_UnseededSessionStampsTheObservation(t *testing.T) {
	d := newActivityDebouncer()
	base := time.Unix(12000, 0)

	d.confirm(1, ActivityDone, base)
	_, _, began := d.confirm(1, ActivityDone, base.Add(debounceWindow))

	if !began.Equal(base) {
		t.Errorf("began = %s, want %s", began, base)
	}
}

// A closed session must take its debounce state with it. Without this every
// closed pane leaves five map entries behind for the life of the process, and
// a reused ID would inherit a stranger's confirmed state.
func TestEmbedded_CloseForgetsTheDebounceState(t *testing.T) {
	host := newTestHost(t, nil)
	id, err := host.Create(CreateSpec{Argv: shellArgv(), Dir: sessionDir(t), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := host.ForceActivity(id, ActivityDone, time.Now()); err != nil {
		t.Fatalf("ForceActivity: %v", err)
	}
	if state, _ := host.ConfirmedActivity(id); state != ActivityDone {
		t.Fatalf("confirmed = %q before close, want %q", state, ActivityDone)
	}

	if err := host.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if state, began := host.ConfirmedActivity(id); state != "" || !began.IsZero() {
		t.Errorf("after close: state = %q, began = %s, want both empty", state, began)
	}
}

// Setting a state for a session the host does not have is refused rather than
// kept in a map forever: the confirmed state belongs to a session.
func TestEmbedded_ActivityWritesNeedASession(t *testing.T) {
	host := newTestHost(t, nil)
	if err := host.ForceActivity(4242, ActivityDone, time.Now()); err == nil {
		t.Error("ForceActivity on an unknown session succeeded")
	}
	if err := host.SeedActivity(4242, ActivityDone, time.Now()); err == nil {
		t.Error("SeedActivity on an unknown session succeeded")
	}
}

// Waking is the counterpart to Suspend, and the reason a daemon can pick up a
// pane nobody is watching: Resume needs a caller that remembers how the
// session was launched, and with no window open there is no such caller. The
// host has the CreateSpec and a Launcher, so it can work both out itself.
func TestEmbedded_WakeResumesASleepingSession(t *testing.T) {
	h := NewEmbedded(Options{Version: "test", Launcher: stubLauncher{}})
	t.Cleanup(h.Release)
	id := suspendedSession(t, h, "conversation-uuid")

	if err := h.Wake(id); err != nil {
		t.Fatalf("Wake: %v", err)
	}
	waitFor(t, func() bool {
		s, err := h.Get(id)
		return err == nil && !s.Asleep()
	}, "the session never woke up")

	if s, _ := h.Get(id); s.Status != StatusRunning {
		t.Errorf("status after Wake = %q, want %q", s.Status, StatusRunning)
	}
}

// suspendedSession returns a session that is asleep, with the given
// conversation ID recorded.
func suspendedSession(t *testing.T, h *Embedded, resumeID string) int {
	t.Helper()
	id, err := h.Create(CreateSpec{Argv: shellArgv(), Dir: sessionDir(t), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := h.SetHookActivity(id, ActivityDone); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}
	if err := h.SetHookSessionID(id, resumeID); err != nil {
		t.Fatalf("SetHookSessionID: %v", err)
	}
	// Arm again if the first attempt was cancelled. A shell prints its prompt
	// shortly after starting, and output arriving after the suspend is armed
	// aborts it on purpose, so that work in flight is never killed. Under load
	// that prompt lands after the arming, and waiting out a suspend that was
	// already cancelled is a 45 second failure for no reason.
	suspended := false
	for attempt := 0; attempt < 10 && !suspended; attempt++ {
		if err := h.Suspend(id); err != nil {
			t.Fatalf("Suspend: %v", err)
		}
		deadline := time.Now().Add(2 * time.Second)
		for !suspended && time.Now().Before(deadline) {
			if s, err := h.Get(id); err == nil && s.Status == StatusSuspended {
				suspended = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if !suspended {
		t.Fatal("the session never reached suspended")
	}
	return id
}

// A host with no launcher cannot rebuild the environment, and a pane woken
// without one has no hook wiring at all. Saying so beats waking it broken.
func TestEmbedded_WakeRefusesWithoutALauncher(t *testing.T) {
	h := newTestHost(t, nil)
	id := suspendedSession(t, h, "conversation-uuid")

	err := h.Wake(id)
	if err == nil {
		t.Fatal("Wake without a launcher succeeded")
	}
	if !strings.Contains(err.Error(), "launcher") {
		t.Errorf("error = %q, want it to say why", err)
	}
}

// An awake session is not an error to wake: two callers racing a wake should
// not both have to work out which one won.
func TestEmbedded_WakeIsANoopWhenAwake(t *testing.T) {
	h := NewEmbedded(Options{Version: "test", Launcher: stubLauncher{}})
	t.Cleanup(h.Release)
	id, err := h.Create(CreateSpec{Argv: shellArgv(), Dir: sessionDir(t), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := h.Wake(id); err != nil {
		t.Errorf("Wake on an awake session = %v, want nil", err)
	}
}

// stubLauncher is a Launcher that answers without reading any configuration.
type stubLauncher struct{}

func (stubLauncher) Argv(tool, model string) ([]string, error) { return []string{tool}, nil }
func (stubLauncher) Env(int, string, string) []string          { return nil }
func (stubLauncher) ResumeArgv(argv []string, resumeID string) []string {
	return append(append([]string{}, argv...), "--resume", resumeID)
}

// waitFor polls until cond holds, or fails. A suspend is armed asynchronously
// (the kill takes long enough that no caller should block on it), so a test
// that asserts straight after Suspend would be asserting on a race.
//
// The deadline is generous because it has to cover the worst case, not the
// usual one: a suspend kills a process tree, and under -race with the whole
// suite running it took past ten seconds. A tight deadline here does not catch
// a slow suspend, it just fails on a loaded machine.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
