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
		"permission required",
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
