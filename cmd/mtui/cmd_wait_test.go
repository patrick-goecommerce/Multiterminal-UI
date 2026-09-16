package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// `mtui wait` is the command that makes the CLI more than a remote control:
// it is what lets a shell script chain two agents. These tests are about the
// contract a script depends on, which is the exit code and the one word on
// stdout, not the prose.

func TestWait_ReturnsAtOnceOnAFinishedSession(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)
	if err := host.SetHookActivity(id, hub.ActivityDone); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}

	started := time.Now()
	code, stdout, stderr := cli(t, "wait", strconv.Itoa(id))
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "done" {
		t.Errorf("stdout = %q, want %q", stdout, "done")
	}
	if elapsed := time.Since(started); elapsed > hub.AgentWaitPoll {
		t.Errorf("waited %s on an already finished session", elapsed)
	}
}

// The default question is "finished or needs me". A permission prompt is a
// reason to come back, same as a result.
func TestWait_BlockedCountsByDefault(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)
	if err := host.SetHookActivity(id, hub.ActivityWaitingPermission); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}

	code, stdout, _ := cli(t, "wait", strconv.Itoa(id))
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if strings.TrimSpace(stdout) != "blocked" {
		t.Errorf("stdout = %q, want %q", stdout, "blocked")
	}
}

// A timeout gets its own exit code: "the agent is still working" and "the call
// broke" need different handling in a script, and exit 1 for both would make
// them indistinguishable.
func TestWait_TimeoutHasItsOwnExitCode(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)
	if err := host.SetHookActivity(id, hub.ActivityActive); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}

	code, _, stderr := cli(t, "wait", strconv.Itoa(id), "--timeout", "400ms")
	if code != exitTimeout {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitTimeout, stderr)
	}
	// The message has to say what the session was doing, not just "timeout".
	if !strings.Contains(stderr, "idle") && !strings.Contains(stderr, "still") {
		t.Errorf("stderr = %q, want it to name the state", stderr)
	}
}

func TestWait_JSONCarriesTheTimeoutAsAField(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)
	if err := host.SetHookActivity(id, hub.ActivityActive); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}

	code, stdout, _ := cli(t, "wait", strconv.Itoa(id), "--timeout", "400ms", "--json")
	if code != exitTimeout {
		t.Fatalf("exit = %d, want %d", code, exitTimeout)
	}
	var got waitResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON (%v): %s", err, stdout)
	}
	if !got.TimedOut {
		t.Error("timed_out is not set")
	}
	if got.ID != id {
		t.Errorf("id = %d, want %d", got.ID, id)
	}
	if got.State == "" {
		t.Error("state is empty; a timeout should still say what it saw")
	}
}

// The state arrives while the wait is running, which is the whole point.
func TestWait_ReturnsWhenTheStateArrives(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)
	if err := host.SetHookActivity(id, hub.ActivityActive); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}
	go func() {
		time.Sleep(2 * hub.AgentWaitPoll)
		_ = host.SetHookActivity(id, hub.ActivityDone)
	}()

	code, stdout, stderr := cli(t, "wait", strconv.Itoa(id), "--until", "done", "--timeout", "20s")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "done" {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestWait_RejectsAnUnknownState(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)

	code, _, stderr := cli(t, "wait", strconv.Itoa(id), "--until", "finished")
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "finished") || !strings.Contains(stderr, "done") {
		t.Errorf("stderr = %q, want the bad state and the valid ones", stderr)
	}
}

// The listing and the wait share one vocabulary, with one deliberate
// difference: a suspended pane reads "asleep" in a list and "done" to a
// waiter. Somebody looking at a list wants to see the process is gone; a
// caller waiting on the outcome does not care.
func TestDisplayState_NamesASleepingPaneAsleep(t *testing.T) {
	asleep := hub.SessionSummary{Status: hub.StatusSuspended, Activity: hub.ActivityIdle}
	if got := displayState(asleep); got != "asleep" {
		t.Errorf("displayState = %q, want %q", got, "asleep")
	}
	if got := hub.AgentState(asleep); got != "done" {
		t.Errorf("AgentState = %q, want %q", got, "done")
	}
}

func TestLs_CanFilterByState(t *testing.T) {
	host := testDaemon(t)
	done := startSession(t, host)
	busy := startSession(t, host)
	if err := host.SetHookActivity(done, hub.ActivityDone); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}
	if err := host.SetHookActivity(busy, hub.ActivityActive); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}

	code, stdout, stderr := cli(t, "ls", "--state", "done", "--json")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	var got []hub.SessionSummary
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON (%v): %s", err, stdout)
	}
	if len(got) != 1 || got[0].ID != done {
		t.Fatalf("filtered listing = %+v, want only session %d", got, done)
	}
}
