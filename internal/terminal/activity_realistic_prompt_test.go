package terminal

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Realistic activity detection tests — prompt detection
//
// Previous tests used plain strings like "$ " and "[Y/n]". But real terminal
// output goes through the VT100 Screen buffer first: raw ANSI bytes are
// processed by Screen.Write(), then classifyScreenState() reads PlainTextRow()
// and matches regex patterns.
//
// These tests feed actual ANSI escape sequences (as a real shell or Claude
// Code would produce) through the full pipeline and verify the activity
// state is detected correctly. If these fail but the plain-string tests
// pass, the problem is in the ANSI stripping / Screen buffer processing.
// ---------------------------------------------------------------------------

// helper: create a session with stale output (>1.5s ago) so DetectActivity
// will actually classify the screen instead of returning "still active".
func newStaleSession(rows, cols int) *Session {
	sess := NewSession(1, rows, cols)
	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.Activity = ActivityActive
	sess.mu.Unlock()
	return sess
}

// ---------------------------------------------------------------------------
// Bash prompt with ANSI colors → should detect ActivityDone
// ---------------------------------------------------------------------------

func TestRealistic_BashPrompt_Colored(t *testing.T) {
	sess := newStaleSession(5, 80)
	// Typical colored bash prompt: \[\033[01;32m\]user@host\[\033[00m\]:\[\033[01;34m\]~/project\[\033[00m\]$
	sess.Screen.Write([]byte(
		"\x1b[01;32muser@host\x1b[00m:\x1b[01;34m~/project\x1b[00m$ ",
	))

	plain := sess.Screen.PlainTextRow(0)
	t.Logf("PlainTextRow after colored bash prompt: %q", plain)

	state := sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Colored bash prompt: state = %d, want ActivityDone (%d)\nPlainTextRow(0) = %q",
			state, ActivityDone, plain)
	}
}

func TestRealistic_BashPrompt_Root(t *testing.T) {
	sess := newStaleSession(5, 80)
	// Root prompt: root@server:/etc#
	sess.Screen.Write([]byte(
		"\x1b[01;31mroot@server\x1b[00m:\x1b[01;34m/etc\x1b[00m# ",
	))

	plain := sess.Screen.PlainTextRow(0)
	t.Logf("PlainTextRow after root prompt: %q", plain)

	state := sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Root prompt: state = %d, want ActivityDone (%d)\nPlainTextRow(0) = %q",
			state, ActivityDone, plain)
	}
}

// ---------------------------------------------------------------------------
// Zsh prompt with special characters → should detect ActivityDone
// ---------------------------------------------------------------------------

func TestRealistic_ZshPrompt(t *testing.T) {
	sess := newStaleSession(5, 80)
	// Zsh prompt with % at end
	sess.Screen.Write([]byte(
		"\x1b[1m\x1b[36m~/project\x1b[0m % ",
	))

	plain := sess.Screen.PlainTextRow(0)
	t.Logf("PlainTextRow after zsh prompt: %q", plain)

	state := sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Zsh prompt: state = %d, want ActivityDone (%d)\nPlainTextRow(0) = %q",
			state, ActivityDone, plain)
	}
}

// ---------------------------------------------------------------------------
// Claude Code prompt (❯) with ANSI styling → should detect ActivityDone
// ---------------------------------------------------------------------------

func TestRealistic_ClaudeCodePrompt(t *testing.T) {
	sess := newStaleSession(10, 80)
	// Simulate Claude Code finishing a task and showing its prompt.
	// Claude Code typically shows: colored output, then a prompt line with ❯
	sess.Screen.Write([]byte(
		"\x1b[32m✓ Task completed successfully\x1b[0m\r\n" +
			"\x1b[1;35m❯\x1b[0m ",
	))

	plain0 := sess.Screen.PlainTextRow(0)
	plain1 := sess.Screen.PlainTextRow(1)
	t.Logf("Row 0: %q", plain0)
	t.Logf("Row 1: %q", plain1)

	state := sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Claude Code prompt (❯): state = %d, want ActivityDone (%d)\nRow0=%q\nRow1=%q",
			state, ActivityDone, plain0, plain1)
	}
}

func TestRealistic_ClaudeCodePrompt_AfterMultilineOutput(t *testing.T) {
	sess := newStaleSession(10, 80)
	// Simulate Claude Code with several lines of output, then prompt
	sess.Screen.Write([]byte(
		"\x1b[2J\x1b[H" + // clear screen, cursor home
			"\x1b[1;36mClaude\x1b[0m is ready.\r\n" +
			"\r\n" +
			"\x1b[90mSession: abc123\x1b[0m\r\n" +
			"\x1b[90mCost: $0.45\x1b[0m\r\n" +
			"\r\n" +
			"\x1b[1;35m❯\x1b[0m ",
	))

	// Check what PlainTextRow produces for the prompt line
	for r := 0; r < 10; r++ {
		row := sess.Screen.PlainTextRow(r)
		if len(row) > 0 && containsRune(row, '❯') {
			t.Logf("Row %d contains ❯: %q", r, row)
		}
	}

	state := sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Claude Code prompt after output: state = %d, want ActivityDone (%d)", state, ActivityDone)
	}
}

// containsRune reports whether s contains the rune r.
func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Claude Code permission prompt → should detect ActivityWaitingAnswer
// ---------------------------------------------------------------------------

func TestRealistic_ClaudeCode_PermissionPrompt(t *testing.T) {
	sess := newStaleSession(10, 80)
	// Claude Code asking for tool approval with ANSI styling
	sess.Screen.Write([]byte(
		"\x1b[1;33m⚠\x1b[0m Claude wants to run:\r\n" +
			"\x1b[1m  rm -rf /tmp/test\x1b[0m\r\n" +
			"\r\n" +
			"\x1b[1;33mDo you want to proceed?\x1b[0m \x1b[90m[Y/n]\x1b[0m ",
	))

	// Log what the screen looks like after ANSI processing
	for r := 0; r < 5; r++ {
		row := sess.Screen.PlainTextRow(r)
		if row != "" {
			t.Logf("Row %d: %q", r, row)
		}
	}

	state := sess.DetectActivity()
	if state != ActivityWaitingAnswer {
		t.Errorf("Claude permission prompt: state = %d, want ActivityWaitingAnswer (%d)", state, ActivityWaitingAnswer)
	}
}

func TestRealistic_ClaudeCode_YesNoPrompt(t *testing.T) {
	sess := newStaleSession(10, 80)
	// Another common Claude Code pattern
	sess.Screen.Write([]byte(
		"\x1b[36mAllow\x1b[0m Claude to edit \x1b[1msrc/main.go\x1b[0m? (y/n) ",
	))

	plain := sess.Screen.PlainTextRow(0)
	t.Logf("PlainTextRow: %q", plain)

	state := sess.DetectActivity()
	if state != ActivityWaitingAnswer {
		t.Errorf("Claude y/n prompt: state = %d, want ActivityWaitingAnswer (%d)\nPlainTextRow = %q",
			state, ActivityWaitingAnswer, plain)
	}
}
