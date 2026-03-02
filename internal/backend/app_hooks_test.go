package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

type testHookEvent struct {
	Ts        int64  `json:"ts"`
	Event     string `json:"event"`
	SessionID string `json:"session_id"`
	MtID      int    `json:"mt_id"`
	Tool      string `json:"tool"`
	Message   string `json:"message"`
}

func writeTestHookEvent(t *testing.T, dir, sessionID string, ev testHookEvent) {
	t.Helper()
	data, _ := json.Marshal(ev)
	line := string(data) + "\n"
	path := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString(line)
}

func TestHookEventToActivity(t *testing.T) {
	tests := []struct {
		event   string
		message string
		want    terminal.ActivityState
	}{
		{"PreToolUse", "", terminal.ActivityActive},
		{"PostToolUse", "", terminal.ActivityActive},
		{"PostToolUseFailure", "", terminal.ActivityError},
		{"PermissionRequest", "", terminal.ActivityWaitingPermission},
		// Notification with a question → user must respond
		{"Notification", "Möchtest du fortfahren?", terminal.ActivityWaitingAnswer},
		{"Notification", "Should I proceed?", terminal.ActivityWaitingAnswer},
		// Notification without a question → informational, show as done
		{"Notification", "Task completed successfully.", terminal.ActivityDone},
		{"Notification", "", terminal.ActivityDone},
		{"Stop", "", terminal.ActivityDone},
		{"UserPromptSubmit", "", terminal.ActivityActive},
		{"SessionEnd", "", terminal.ActivityIdle},
		{"unknown", "", terminal.ActivityIdle},
	}
	for _, tt := range tests {
		got := hookEventToActivity(tt.event, tt.message)
		if got != tt.want {
			t.Errorf("hookEventToActivity(%q, %q) = %d, want %d", tt.event, tt.message, got, tt.want)
		}
	}
}

func TestHookManager_ProcessesNewEvents(t *testing.T) {
	dir := t.TempDir()
	sess := terminal.NewSession(42, 24, 80)

	hm := newHookManager(dir, func(mtID int) *terminal.Session {
		if mtID == 42 {
			return sess
		}
		return nil
	}, nil)

	writeTestHookEvent(t, dir, "claude-abc", testHookEvent{
		Ts: time.Now().Unix(), Event: "PermissionRequest",
		SessionID: "claude-abc", MtID: 42, Tool: "Bash",
	})

	hm.processDirectory()

	if !sess.HasHookData() {
		t.Fatal("session should have hook data after processing")
	}
	if sess.HookSessionID() != "claude-abc" {
		t.Errorf("HookSessionID = %q, want %q", sess.HookSessionID(), "claude-abc")
	}
	if got := sess.GetActivity(); got != terminal.ActivityWaitingPermission {
		t.Errorf("Activity = %d, want ActivityWaitingPermission", got)
	}
}

func TestHookManager_IncrementalRead(t *testing.T) {
	dir := t.TempDir()
	sess := terminal.NewSession(10, 24, 80)

	hm := newHookManager(dir, func(mtID int) *terminal.Session {
		if mtID == 10 {
			return sess
		}
		return nil
	}, nil)

	// First event
	writeTestHookEvent(t, dir, "s1", testHookEvent{
		Ts: time.Now().Unix(), Event: "PreToolUse", SessionID: "s1", MtID: 10,
	})
	hm.processDirectory()

	if got := sess.GetActivity(); got != terminal.ActivityActive {
		t.Errorf("after first event: Activity = %d, want ActivityActive", got)
	}

	// Second event (appended to same file)
	writeTestHookEvent(t, dir, "s1", testHookEvent{
		Ts: time.Now().Unix(), Event: "PermissionRequest", SessionID: "s1", MtID: 10,
	})
	hm.processDirectory()

	if got := sess.GetActivity(); got != terminal.ActivityWaitingPermission {
		t.Errorf("after second event: Activity = %d, want ActivityWaitingPermission", got)
	}
}

func TestHookManager_SessionEnd_ClearsHookData(t *testing.T) {
	dir := t.TempDir()
	sess := terminal.NewSession(7, 24, 80)
	sess.SetHookActivity(terminal.ActivityActive)

	hm := newHookManager(dir, func(mtID int) *terminal.Session {
		if mtID == 7 {
			return sess
		}
		return nil
	}, nil)

	writeTestHookEvent(t, dir, "sess7", testHookEvent{
		Ts: time.Now().Unix(), Event: "SessionEnd", SessionID: "sess7", MtID: 7,
	})
	hm.processDirectory()

	if sess.HasHookData() {
		t.Error("hook data should be cleared after SessionEnd")
	}
	// JSONL file should be deleted
	path := filepath.Join(dir, "sess7.jsonl")
	if _, err := os.Stat(path); err == nil {
		t.Error("JSONL file should be deleted after SessionEnd")
	}
}

func TestHookManager_IgnoresZeroMtID(t *testing.T) {
	dir := t.TempDir()
	called := false

	hm := newHookManager(dir, func(mtID int) *terminal.Session {
		called = true
		return nil
	}, nil)

	writeTestHookEvent(t, dir, "no-mt", testHookEvent{
		Ts: time.Now().Unix(), Event: "PreToolUse", SessionID: "no-mt", MtID: 0,
	})
	hm.processDirectory()

	if called {
		t.Error("lookup should not be called when mt_id = 0")
	}
}
