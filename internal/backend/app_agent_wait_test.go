package backend

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// waitTestApp gives an app one adopted session whose state a test can set.
func waitTestApp(t *testing.T, id int) (*AppService, *terminal.Session) {
	t.Helper()
	a := newTestApp()
	t.Cleanup(a.host.Release)
	sess := terminal.NewSession(id, 24, 80)
	adopt(t, a, id, sess)
	return a, sess
}

// A session that is already finished returns at once: an agent asking about
// work that is over should not wait out a poll interval, let alone a timeout.
func TestWaitForAgent_ReturnsImmediatelyWhenAlreadyDone(t *testing.T) {
	a, sess := waitTestApp(t, 1)
	sess.SetHookActivity(terminal.ActivityDone)

	start := time.Now()
	state, err := a.waitForAgent(context.Background(), 1, nil, time.Minute)
	if err != nil {
		t.Fatalf("WaitForAgent: %v", err)
	}
	if state != "done" {
		t.Errorf("state = %q, want %q", state, "done")
	}
	if elapsed := time.Since(start); elapsed > agentWaitPoll {
		t.Errorf("waited %s on an already finished session", elapsed)
	}
}

// The default question is "finished or needs me", which is what makes the call
// worth having: a permission prompt is a reason to come back, same as a result.
func TestWaitForAgent_BlockedCountsByDefault(t *testing.T) {
	a, sess := waitTestApp(t, 2)
	sess.SetHookActivity(terminal.ActivityWaitingPermission)

	state, err := a.waitForAgent(context.Background(), 2, nil, time.Minute)
	if err != nil {
		t.Fatalf("WaitForAgent: %v", err)
	}
	if state != "blocked" {
		t.Errorf("state = %q, want %q", state, "blocked")
	}
}

// A question waiting for an answer blocks the same way a permission prompt
// does: both mean a human is needed.
func TestWaitForAgent_AQuestionIsAlsoBlocked(t *testing.T) {
	a, sess := waitTestApp(t, 3)
	sess.SetHookActivity(terminal.ActivityWaitingAnswer)

	state, err := a.waitForAgent(context.Background(), 3, []string{"blocked"}, time.Minute)
	if err != nil {
		t.Fatalf("WaitForAgent: %v", err)
	}
	if state != "blocked" {
		t.Errorf("state = %q, want %q", state, "blocked")
	}
}

// The state the caller is waiting for arrives while it waits.
func TestWaitForAgent_ReturnsWhenTheStateArrives(t *testing.T) {
	a, sess := waitTestApp(t, 4)
	sess.SetHookActivity(terminal.ActivityActive)

	go func() {
		time.Sleep(2 * agentWaitPoll)
		sess.SetHookActivity(terminal.ActivityDone)
	}()

	state, err := a.waitForAgent(context.Background(), 4, []string{"done"}, 10*time.Second)
	if err != nil {
		t.Fatalf("WaitForAgent: %v", err)
	}
	if state != "done" {
		t.Errorf("state = %q, want %q", state, "done")
	}
}

// A working session that never finishes has to give the caller its turn back,
// and say what it was doing rather than just "timeout".
func TestWaitForAgent_TimesOutWithTheStateItSaw(t *testing.T) {
	a, sess := waitTestApp(t, 5)
	sess.SetHookActivity(terminal.ActivityActive)

	_, err := a.waitForAgent(context.Background(), 5, []string{"done"}, 300*time.Millisecond)
	if err == nil {
		t.Fatal("waiting on a session that never finishes returned no error")
	}
	if !strings.Contains(err.Error(), "idle") && !strings.Contains(err.Error(), "still") {
		t.Errorf("timeout error = %q, want it to name what the session was doing", err)
	}
}

// A session whose process is gone ends the wait even when the caller asked for
// "done": nothing about it is going to change.
func TestWaitForAgent_AnExitEndsTheWait(t *testing.T) {
	a, _ := waitTestApp(t, 6)

	// Forget the session entirely, as a close would.
	if err := a.host.Close(6); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The session is gone, so the wait cannot even start.
	if _, err := a.waitForAgent(context.Background(), 6, []string{"done"}, time.Second); err == nil {
		t.Error("waiting on a session the host does not have succeeded")
	}
}

func TestWaitForAgent_RejectsAnUnknownState(t *testing.T) {
	a, _ := waitTestApp(t, 7)

	_, err := a.waitForAgent(context.Background(), 7, []string{"finished"}, time.Second)
	if err == nil {
		t.Fatal("an unknown state was accepted")
	}
	if !strings.Contains(err.Error(), "finished") || !strings.Contains(err.Error(), "done") {
		t.Errorf("error = %q, want it to name the bad state and the valid ones", err)
	}
}

// A cancelled call returns rather than holding the caller to the timeout.
func TestWaitForAgent_HonoursCancellation(t *testing.T) {
	a, sess := waitTestApp(t, 8)
	sess.SetHookActivity(terminal.ActivityActive)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(2 * agentWaitPoll)
		cancel()
	}()

	start := time.Now()
	if _, err := a.waitForAgent(ctx, 8, []string{"done"}, time.Minute); err == nil {
		t.Fatal("a cancelled wait returned no error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("cancellation took %s", elapsed)
	}
}

// A sleeping pane is finished work whose process was released on purpose. For
// a caller waiting on the outcome that is done, not gone.
func TestAgentWaitState_ASleepingPaneIsDone(t *testing.T) {
	a, sess := waitTestApp(t, 9)
	sess.SetHookActivity(terminal.ActivityDone)
	if !sess.TrySuspend() {
		t.Fatal("TrySuspend refused a done session")
	}

	if got := a.agentWaitState(9); got != "done" {
		t.Errorf("state of a sleeping pane = %q, want %q", got, "done")
	}
}
