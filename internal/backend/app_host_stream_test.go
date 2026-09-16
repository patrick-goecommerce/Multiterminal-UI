package backend

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// printArgv is a command that writes a marker to the terminal and exits.
func printArgv(text string) []string {
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		return []string{comspec, "/c", "echo " + text}
	}
	return []string{"/bin/sh", "-c", "printf '%s' " + text}
}

// sleepArgvForTest is a command that stays alive without producing output.
func sleepArgvForTest() []string {
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		return []string{comspec, "/c", "ping -n 60 127.0.0.1 > nul"}
	}
	return []string{"/bin/sh", "-c", "sleep 60"}
}

// drainBatcher swaps the batcher until it has seen want, or the deadline
// passes. The batch loop that normally does this needs a Wails app.
func drainBatcher(t *testing.T, a *AppService, id int, want string) string {
	t.Helper()
	var got strings.Builder
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for sid, data := range a.outputBatch().swap() {
			if sid == id {
				got.Write(data)
			}
		}
		if strings.Contains(got.String(), want) {
			return got.String()
		}
		time.Sleep(10 * time.Millisecond)
	}
	return got.String()
}

// CreateSession has to end with the session's output in the batcher, which is
// what the frontend reads. The path changed from a channel the backend drained
// itself to a subscription on the host, so this is the one that must not
// regress.
func TestCreateSession_OutputReachesTheBatcher(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	id := a.CreateSession(printArgv("batcher-marker"), t.TempDir(), 24, 80, "shell")
	if id <= 0 {
		t.Fatalf("CreateSession returned %d", id)
	}

	if got := drainBatcher(t, a, id, "batcher-marker"); !strings.Contains(got, "batcher-marker") {
		t.Errorf("batched output = %q, want the marker", got)
	}
}

// The ID the caller gets back must be the one the environment was built
// around, or the hook and the statusline shim report against a session nobody
// knows.
func TestCreateSession_UsesTheReservedID(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	id := a.CreateSession(printArgv("x"), t.TempDir(), 24, 80, "shell")
	if id <= 0 {
		t.Fatalf("CreateSession returned %d", id)
	}
	summary, err := a.host.Get(id)
	if err != nil {
		t.Fatalf("the host does not know session %d: %v", id, err)
	}
	if summary.ID != id {
		t.Errorf("host session id = %d, want %d", summary.ID, id)
	}
}

// A failed launch must not leave a half-registered session behind.
func TestCreateSession_FailedLaunchIsNotRegistered(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	id := a.CreateSession([]string{"definitely-not-a-real-binary-mtui"}, t.TempDir(), 24, 80, "shell")
	if id != -1 {
		t.Fatalf("CreateSession returned %d for a command that cannot start, want -1", id)
	}
	if n := len(a.host.List()); n != 0 {
		t.Errorf("host holds %d sessions after a failed launch, want 0", n)
	}
	a.mu.Lock()
	launches := len(a.launches)
	a.mu.Unlock()
	if launches != 0 {
		t.Errorf("%d launch specs remembered after a failed launch, want 0", launches)
	}
}

// CloseSession must end the process and forget it, so a later lookup does not
// hand out a dead session.
func TestCloseSession_ForgetsTheSession(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	id := a.CreateSession(sleepArgvForTest(), t.TempDir(), 24, 80, "shell")
	if id <= 0 {
		t.Fatalf("CreateSession returned %d", id)
	}

	a.CloseSession(id)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if !a.hasSession(id) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("the session is still there after CloseSession")
}

// A truncated chunk means the ring dropped bytes. Appending across that gap
// leaves the pane garbled, so the pump has to repaint first.
func TestPumpToBatcher_TruncationTriggersARepaint(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	sess := newResyncTestSession(t, 1, "mirror line")
	adopt(t, a, 1, sess)

	ch := make(chan hub.Chunk, 2)
	ch <- hub.Chunk{Offset: 100, Data: []byte("after the gap"), Truncated: true}
	close(ch)
	a.pumpToBatcher(1, hub.NewSubscriptionForTest(ch))

	got := string(a.outputBatch().swap()[1])
	if !strings.Contains(got, "mirror line") {
		t.Errorf("batched output = %q, want it to start from a repaint of the mirror", got)
	}
	if !strings.Contains(got, "after the gap") {
		t.Errorf("batched output = %q, want the chunk itself as well", got)
	}
}

// An exit or a suspend reported by the daemon arrives as JSON, not as a
// struct. The handler used to type-assert, so in daemon mode none of these
// events reached the UI at all.
func TestOnHostEvent_HandlesAJSONPayloadFromTheDaemon(t *testing.T) {
	const id = 5
	cleanupActivityTracking(id)
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, id, terminal.NewSession(id, 24, 80))

	raw, err := json.Marshal(hub.SessionSuspended{ID: id, ResumeID: "uuid"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	a.onHostEvent(hub.EventSessionSuspended, json.RawMessage(raw))

	state, _ := confirmedOf(a, id)
	if state != "sleeping" {
		t.Errorf("activity after a JSON suspend event = %q, want %q", state, "sleeping")
	}
}

// The in-process host hands over the struct itself; both shapes must work.
func TestOnHostEvent_HandlesAStructPayloadFromTheEmbeddedHost(t *testing.T) {
	const id = 6
	cleanupActivityTracking(id)
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, id, terminal.NewSession(id, 24, 80))

	a.onHostEvent(hub.EventSessionResumed, hub.SessionResumed{ID: id, ResumeID: "uuid"})

	state, _ := confirmedOf(a, id)
	if state != "resuming" {
		t.Errorf("activity after a struct resume event = %q, want %q", state, "resuming")
	}
}
