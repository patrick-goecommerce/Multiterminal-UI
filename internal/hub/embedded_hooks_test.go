package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// The mapping from a Claude Code event to an agent state is what the whole
// hook path turns on, so every case is pinned.
func TestHookActivity_MapsWhatAnEventClaims(t *testing.T) {
	cases := []struct {
		name   string
		ev     hooks.Event
		want   Activity
		wantOK bool
	}{
		{"tool starts", hooks.Event{Event: "PreToolUse", Tool: "Bash"}, ActivityActive, true},
		{"tool done", hooks.Event{Event: "PostToolUse", Tool: "Bash"}, ActivityActive, true},
		{"prompt", hooks.Event{Event: "UserPromptSubmit"}, ActivityActive, true},
		{"tool failed", hooks.Event{Event: "PostToolUseFailure"}, ActivityError, true},
		{"permission", hooks.Event{Event: "PermissionRequest", Tool: "Bash"}, ActivityWaitingPermission, true},
		{"stop", hooks.Event{Event: "Stop"}, ActivityDone, true},
		{"stop with a statement", hooks.Event{Event: "Stop", Message: "Alles erledigt."}, ActivityDone, true},
		{"stop with a question", hooks.Event{Event: "Stop", Message: "Tests sind grün.\n\nSoll ich pushen?"}, ActivityWaitingAnswer, true},
		// AskUserQuestion waits for the user; it is not work.
		{"ask user", hooks.Event{Event: "PreToolUse", Tool: "AskUserQuestion"}, ActivityWaitingAnswer, true},
		{"ask user via permission", hooks.Event{Event: "PermissionRequest", Tool: "AskUserQuestion"}, ActivityWaitingAnswer, true},
		{"ask user answered", hooks.Event{Event: "PostToolUse", Tool: "AskUserQuestion"}, ActivityActive, true},
		// Notifications are read by their type.
		{"permission prompt", hooks.Event{Event: "Notification", NotificationType: "permission_prompt", Message: "Claude needs your permission to use Bash"}, ActivityWaitingPermission, true},
		{"elicitation", hooks.Event{Event: "Notification", NotificationType: "elicitation_dialog"}, ActivityWaitingAnswer, true},
		{"agent needs input", hooks.Event{Event: "Notification", NotificationType: "agent_needs_input"}, ActivityWaitingAnswer, true},
		{"elicitation answered", hooks.Event{Event: "Notification", NotificationType: "elicitation_response"}, ActivityActive, true},
		// An idle reminder says nothing new: the state it reminds of is
		// already set, and "waiting for your input" must not undo a question.
		{"idle reminder", hooks.Event{Event: "Notification", NotificationType: "idle_prompt", Message: "Claude is waiting for your input"}, ActivityIdle, false},
		// Without a type (older Claude Code): the old "?" reading.
		{"untyped question", hooks.Event{Event: "Notification", Message: "Weiter so?"}, ActivityWaitingAnswer, true},
		{"untyped statement", hooks.Event{Event: "Notification", Message: "Claude needs your permission to use Bash"}, ActivityIdle, false},
		// An unknown event is not evidence of idleness either.
		{"unknown", hooks.Event{Event: "unknown"}, ActivityIdle, false},
	}
	for _, c := range cases {
		got, ok := hookActivity(c.ev)
		if ok != c.wantOK {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.wantOK)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%s: activity = %q, want %q", c.name, got, c.want)
		}
	}
}

// The case that started this: Claude ends with a question, then Claude Code
// prints a timing line and a recap below it, so the screen check (two lines
// above the prompt) never sees the "?".
func TestEndsWithQuestion(t *testing.T) {
	yes := []string{
		"Noch offen aus #985: SmartSupply.\n\nSoll ich pushen und den PR gegen main öffnen? Danach startet automatisch das Pflicht-Review.",
		"Fertig.\n\nWie soll ich weitermachen?\n\n1. Alles committen\n2. Erst die Tests",
		"Welche Variante nimmst du? **A** oder **B**?**",
		"Passt das so?",
		"Soll ich das mergen？",
		"Ergebnis:\n```go\nx := a ? b : c\n```\n\nSoll ich das so lassen?",
		// The tail of a long message can begin inside a code block.
		"x := y\n```\n\nSoll ich weitermachen?",
	}
	no := []string{
		"",
		"Alles erledigt, Tests sind grün.",
		"Warum? Weil der Cache leer war.\n\nIch habe ihn neu gefüllt, jetzt läuft es.",
		"Siehe https://example.com/page?tab=1 für Details.",
		"Code:\n```\nif ok? then\n```",
		"- erledigt\n- getestet",
	}
	for _, m := range yes {
		if !endsWithQuestion(m) {
			t.Errorf("endsWithQuestion(%q) = false, want true", m)
		}
	}
	for _, m := range no {
		if endsWithQuestion(m) {
			t.Errorf("endsWithQuestion(%q) = true, want false", m)
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
