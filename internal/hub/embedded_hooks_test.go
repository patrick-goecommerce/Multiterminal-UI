package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// The mapping from a Claude Code event name to an agent state is what the
// whole hook path turns on, so every case is pinned.
func TestHookActivity_MapsWhatAnEventClaims(t *testing.T) {
	cases := []struct {
		event   string
		message string
		want    Activity
		wantOK  bool
	}{
		{"PreToolUse", "", ActivityActive, true},
		{"PostToolUse", "", ActivityActive, true},
		{"UserPromptSubmit", "", ActivityActive, true},
		{"PostToolUseFailure", "", ActivityError, true},
		{"PermissionRequest", "", ActivityWaitingPermission, true},
		{"Stop", "", ActivityDone, true},
		{"Notification", "Weiter so?", ActivityWaitingAnswer, true},
		// A notification without a question mark says nothing about whether
		// the turn ended. Claude's own wording ("Claude needs your permission
		// to use Bash") has no "?", and mapping it to done tore a running
		// session to "finished" (#188).
		{"Notification", "Claude needs your permission to use Bash", ActivityIdle, false},
		// An unknown event is not evidence of idleness either.
		{"unknown", "", ActivityIdle, false},
	}
	for _, c := range cases {
		got, ok := hookActivity(c.event, c.message)
		if ok != c.wantOK {
			t.Errorf("hookActivity(%q, %q) ok = %v, want %v", c.event, c.message, ok, c.wantOK)
			continue
		}
		if ok && got != c.want {
			t.Errorf("hookActivity(%q, %q) = %q, want %q", c.event, c.message, got, c.want)
		}
	}
}

func hookTestHost(t *testing.T, id int) (*Embedded, *terminal.Session, *recordingSink) {
	t.Helper()
	sink := newSink()
	h := NewEmbedded(Options{Sink: sink})
	t.Cleanup(h.Release)
	sess := terminal.NewSession(id, 24, 80)
	h.AdoptForTest(id, sess)
	return h, sess, sink
}

func TestApplyHookEvent_RecordsStateAndReportsIt(t *testing.T) {
	h, sess, sink := hookTestHost(t, 42)

	h.applyHookEvent(nil, hooks.Event{
		Event: "PermissionRequest", SessionID: "claude-abc", MtID: 42, Tool: "Bash",
	})

	if !sess.HasHookData() {
		t.Error("the session has no hook data after an event")
	}
	if got := sess.HookSessionID(); got != "claude-abc" {
		t.Errorf("HookSessionID = %q, want %q", got, "claude-abc")
	}
	if got := sess.GetActivity(); got != terminal.ActivityWaitingPermission {
		t.Errorf("activity = %d, want ActivityWaitingPermission", got)
	}
	sink.await(t, EventSessionHook)
}

// An event that says nothing about the state must leave it alone.
func TestApplyHookEvent_AStatelessEventChangesNothing(t *testing.T) {
	h, sess, _ := hookTestHost(t, 1)
	sess.SetHookActivity(terminal.ActivityDone)

	h.applyHookEvent(nil, hooks.Event{
		Event: "Notification", MtID: 1, SessionID: "s1",
		Message: "Claude needs your permission to use Bash",
	})

	if got := sess.GetActivity(); got != terminal.ActivityDone {
		t.Errorf("activity = %d, want it untouched at ActivityDone", got)
	}
}

// SessionEnd means the agent is gone: the hook data goes with it, so the pane
// falls back to screen-pattern detection instead of being frozen at whatever
// the last hook said.
func TestApplyHookEvent_SessionEndClearsHookData(t *testing.T) {
	h, sess, _ := hookTestHost(t, 7)
	h.applyHookEvent(nil, hooks.Event{Event: "Stop", MtID: 7, SessionID: "s7"})
	if !sess.HasHookData() {
		t.Fatal("setup: the session should have hook data")
	}

	h.applyHookEvent(nil, hooks.Event{Event: "SessionEnd", MtID: 7, SessionID: "s7"})

	if sess.HasHookData() {
		t.Error("hook data survived SessionEnd")
	}
}

// An event for a session this host does not have is not an error, just
// nothing: with several hosts around, most events belong to someone else.
func TestApplyHookEvent_IgnoresAnUnknownSession(t *testing.T) {
	h, _, sink := hookTestHost(t, 1)

	h.applyHookEvent(nil, hooks.Event{Event: "Stop", MtID: 999, SessionID: "x"})

	select {
	case name := <-sink.got:
		t.Errorf("reported %q for a session the host does not have", name)
	default:
	}
}

// A pane that is closed rather than exited never writes a SessionEnd, which
// was the only thing that removed its hook file.
func TestClose_RemovesTheSessionsHookFile(t *testing.T) {
	h, sess, _ := hookTestHost(t, 42)
	dir := t.TempDir()
	h.hookWatcher = hooks.NewWatcher(dir, nil)
	file := filepath.Join(dir, "claude-abc.jsonl")
	if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sess.SetHookSessionID("claude-abc")

	if err := h.Close(42); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("hook file survived the close (stat err = %v)", err)
	}
}
