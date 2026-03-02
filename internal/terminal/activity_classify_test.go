package terminal

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Regex patterns: needsInputPattern
// ---------------------------------------------------------------------------

func TestNeedsInputPattern(t *testing.T) {
	shouldMatch := []string{
		"Do you want to continue? [Y/n]",
		"[y/N] proceed?",
		"(y/n) Are you sure?",
		"proceed?",
		"continue?",
		"confirm?",
		"approve?",
		"allow?",
		"Do you want to delete this file?",
		"Would you like to continue?",
		"Press Enter to continue",
		"waiting for input",
		"Waiting for response",
	}
	for _, s := range shouldMatch {
		if !needsInputPattern.MatchString(s) {
			t.Errorf("needsInputPattern should match %q", s)
		}
	}

	shouldNotMatch := []string{
		"Hello world",
		"compiling main.go",
		"100% complete",
		"file saved successfully",
		// Claude Code YOLO startup line — must NOT trigger attention
		"bypass permissions on (shift+tab to cycle)",
		"permission required", // handled by PermissionRequest hook, not PTY scan
	}
	for _, s := range shouldNotMatch {
		if needsInputPattern.MatchString(s) {
			t.Errorf("needsInputPattern should NOT match %q", s)
		}
	}
}

// ---------------------------------------------------------------------------
// Regex patterns: promptPattern
// ---------------------------------------------------------------------------

func TestPromptPattern(t *testing.T) {
	shouldMatch := []string{
		"$ ",                      // bare $ prompt
		"% ",                      // zsh prompt
		"# ",                      // root prompt
		"> ",                      // generic prompt
		"❯ ",                      // Claude Code prompt
		"❯",                       // Claude Code prompt (no trailing space)
		`C:\Users\test>`,          // Windows cmd
		`C:\Windows\System32>`,    // Windows cmd
		"user@host ~ $ ",          // prompt with space before $
	}
	for _, s := range shouldMatch {
		if !promptPattern.MatchString(s) {
			t.Errorf("promptPattern should match %q", s)
		}
	}

	shouldNotMatch := []string{
		"compiling...",
		"running tests",
		"error: file not found",
	}
	for _, s := range shouldNotMatch {
		if promptPattern.MatchString(s) {
			t.Errorf("promptPattern should NOT match %q", s)
		}
	}
}

// ---------------------------------------------------------------------------
// classifyScreenState — via screen buffer content
// ---------------------------------------------------------------------------

func TestClassifyScreenState_Prompt(t *testing.T) {
	// Use small screen so text is within the last-15-rows scan window
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("some output\r\n$ "))

	state := sess.classifyScreenState()
	if state != ActivityDone {
		t.Errorf("classifyScreenState = %d, want ActivityDone (%d)", state, ActivityDone)
	}
}

func TestClassifyScreenState_NeedsInput(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("Delete file? [Y/n] "))

	state := sess.classifyScreenState()
	if state != ActivityWaitingAnswer {
		t.Errorf("classifyScreenState = %d, want ActivityWaitingAnswer (%d)", state, ActivityWaitingAnswer)
	}
}

func TestClassifyScreenState_Idle(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("compiling main.go...\r\n"))

	state := sess.classifyScreenState()
	if state != ActivityIdle {
		t.Errorf("classifyScreenState = %d, want ActivityIdle (%d)", state, ActivityIdle)
	}
}

func TestClassifyScreenState_NeedsInputTakesPriority(t *testing.T) {
	sess := NewSession(1, 5, 80)
	// Both a prompt and a needs-input pattern — needs-input should win
	// because it is checked first on the same line
	sess.Screen.Write([]byte("Do you want to proceed? [Y/n] $ "))

	state := sess.classifyScreenState()
	if state != ActivityWaitingAnswer {
		t.Errorf("classifyScreenState = %d, want ActivityWaitingAnswer (%d)", state, ActivityWaitingAnswer)
	}
}

func TestClassifyScreenState_QuestionAbovePrompt(t *testing.T) {
	// Claude asked a question, then the input prompt appeared below it.
	// The scanner must detect WaitingAnswer even though the prompt is present.
	sess := NewSession(1, 10, 80)
	sess.Screen.Write([]byte(
		"Sure! What would you like to work on?\r\n" +
			"Woran möchtest du arbeiten?\r\n" +
			"\r\n" +
			"> "))

	state := sess.classifyScreenState()
	if state != ActivityWaitingAnswer {
		t.Errorf("classifyScreenState = %d, want ActivityWaitingAnswer (%d)", state, ActivityWaitingAnswer)
	}
}

func TestClassifyScreenState_StatementAbovePrompt(t *testing.T) {
	// Claude finished with a normal statement (no question) — should be Done.
	sess := NewSession(1, 10, 80)
	sess.Screen.Write([]byte(
		"Task completed successfully.\r\n" +
			"\r\n" +
			"> "))

	state := sess.classifyScreenState()
	if state != ActivityDone {
		t.Errorf("classifyScreenState = %d, want ActivityDone (%d)", state, ActivityDone)
	}
}

func TestSession_HookData(t *testing.T) {
	s := NewSession(1, 24, 80)

	// Initially no hook data
	if s.HasHookData() {
		t.Fatal("new session should not have hook data")
	}

	// Set hook activity
	s.SetHookActivity(ActivityWaitingPermission)
	if !s.HasHookData() {
		t.Fatal("session should have hook data after SetHookActivity")
	}

	s.mu.Lock()
	got := s.Activity
	s.mu.Unlock()
	if got != ActivityWaitingPermission {
		t.Errorf("Activity = %d, want ActivityWaitingPermission (%d)", got, ActivityWaitingPermission)
	}

	// ClearHookData resets flag
	s.ClearHookData()
	if s.HasHookData() {
		t.Fatal("session should not have hook data after ClearHookData")
	}
}

func TestSession_HookSessionID(t *testing.T) {
	s := NewSession(2, 24, 80)

	if id := s.HookSessionID(); id != "" {
		t.Fatalf("new session HookSessionID should be empty, got %q", id)
	}

	s.SetHookSessionID("claude-uuid-abc123")
	if id := s.HookSessionID(); id != "claude-uuid-abc123" {
		t.Errorf("HookSessionID = %q, want %q", id, "claude-uuid-abc123")
	}
}
