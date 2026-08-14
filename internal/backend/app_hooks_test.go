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
		wantOK  bool
	}{
		{"PreToolUse", "", terminal.ActivityActive, true},
		{"PostToolUse", "", terminal.ActivityActive, true},
		{"UserPromptSubmit", "", terminal.ActivityActive, true},
		{"PostToolUseFailure", "", terminal.ActivityError, true},
		{"PermissionRequest", "", terminal.ActivityWaitingPermission, true},
		{"Stop", "", terminal.ActivityDone, true},
		{"Notification", "Weiter so?", terminal.ActivityWaitingAnswer, true},
		// A notification without a question mark says nothing about whether the
		// turn ended. Claude's own wording ("Claude needs your permission to use
		// Bash") has no "?", and mapping it to Done tore a running session to
		// "finished" (issue #188).
		{"Notification", "Claude needs your permission to use Bash", terminal.ActivityIdle, false},
		// An unknown event is not evidence of idleness either.
		{"unknown", "", terminal.ActivityIdle, false},
	}
	for _, tt := range tests {
		got, ok := hookEventToActivity(tt.event, tt.message)
		if ok != tt.wantOK {
			t.Errorf("hookEventToActivity(%q, %q) ok = %v, want %v", tt.event, tt.message, ok, tt.wantOK)
			continue
		}
		if ok && got != tt.want {
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

func TestHookManager_UserPromptSubmitTriggersOnPrompt(t *testing.T) {
	dir := t.TempDir()
	sess := terminal.NewSession(8, 24, 80)

	hm := newHookManager(dir, func(mtID int) *terminal.Session {
		if mtID == 8 {
			return sess
		}
		return nil
	}, nil)

	var gotID int
	var gotPrompt string
	hm.onPrompt = func(mtID int, prompt string) {
		gotID = mtID
		gotPrompt = prompt
	}

	writeTestHookEvent(t, dir, "s8", testHookEvent{
		Ts: time.Now().Unix(), Event: "UserPromptSubmit",
		SessionID: "s8", MtID: 8, Message: "Refactor the auth module",
	})
	hm.processDirectory()

	if gotID != 8 {
		t.Errorf("onPrompt mtID = %d, want 8", gotID)
	}
	if gotPrompt != "Refactor the auth module" {
		t.Errorf("onPrompt prompt = %q, want 'Refactor the auth module'", gotPrompt)
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

func TestHandleEvent_CallsOnWorktreeChangeForEnterWorktree(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session {
		if mtID == 1 {
			return sess
		}
		return nil
	}, nil)

	var gotPath, gotBranch, gotCwd string
	var calls int
	hm.onWorktreeChange = func(mtID int, worktreePath, worktreeBranch, cwd string) {
		calls++
		gotPath, gotBranch, gotCwd = worktreePath, worktreeBranch, cwd
	}

	hm.handleEvent(rawHookEvent{
		Event: "PostToolUse", MtID: 1, SessionID: "s1", Tool: "EnterWorktree",
		Cwd:            `D:\repos\proj\.claude\worktrees\feature-a`,
		WorktreePath:   `D:\repos\proj\.claude\worktrees\feature-a`,
		WorktreeBranch: "worktree-feature-a",
	})

	if calls != 1 {
		t.Fatalf("onWorktreeChange called %d times, want 1", calls)
	}
	if gotPath != `D:\repos\proj\.claude\worktrees\feature-a` || gotBranch != "worktree-feature-a" {
		t.Errorf("got path=%q branch=%q", gotPath, gotBranch)
	}
	if gotCwd != `D:\repos\proj\.claude\worktrees\feature-a` {
		t.Errorf("got cwd=%q", gotCwd)
	}
}

func TestHandleEvent_CallsOnWorktreeChangeWithEmptyPathForOrdinaryEvents(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session { return sess }, nil)

	var gotPath string
	var calls int
	hm.onWorktreeChange = func(mtID int, worktreePath, worktreeBranch, cwd string) {
		calls++
		gotPath = worktreePath
	}

	hm.handleEvent(rawHookEvent{Event: "PostToolUse", MtID: 1, SessionID: "s1", Tool: "Bash", Cwd: `D:\repos\proj`})

	if calls != 1 {
		t.Fatalf("onWorktreeChange called %d times, want 1 (ordinary event must still report cwd)", calls)
	}
	if gotPath != "" {
		t.Errorf("worktreePath = %q, want empty for non-EnterWorktree event", gotPath)
	}
}

func TestHandleEvent_CallsOnPathBlocked(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session { return sess }, nil)

	var gotPath, gotReason string
	var calls int
	hm.onPathBlocked = func(mtID int, path, reason string) {
		calls++
		gotPath, gotReason = path, reason
	}

	hm.handleEvent(rawHookEvent{
		Event: "PreToolUse", MtID: 1, SessionID: "s1", Tool: "Edit",
		BlockedPath: `D:\repo\internal\backend\app.go`,
		BlockReason: "Pfad liegt im Hauptrepo...",
	})

	if calls != 1 {
		t.Fatalf("onPathBlocked called %d times, want 1", calls)
	}
	if gotPath != `D:\repo\internal\backend\app.go` || gotReason != "Pfad liegt im Hauptrepo..." {
		t.Errorf("got path=%q reason=%q", gotPath, gotReason)
	}
}

func TestHandleEvent_DoesNotCallOnPathBlockedWhenEmpty(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session { return sess }, nil)

	calls := 0
	hm.onPathBlocked = func(int, string, string) { calls++ }

	hm.handleEvent(rawHookEvent{Event: "PreToolUse", MtID: 1, SessionID: "s1", Tool: "Edit"})

	if calls != 0 {
		t.Errorf("onPathBlocked should not fire without a BlockedPath, called %d times", calls)
	}
}
