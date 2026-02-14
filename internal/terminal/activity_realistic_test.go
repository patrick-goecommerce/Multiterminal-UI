package terminal

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Realistic activity detection tests
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
		if strings.Contains(row, "❯") {
			t.Logf("Row %d contains ❯: %q", r, row)
		}
	}

	state := sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Claude Code prompt after output: state = %d, want ActivityDone (%d)", state, ActivityDone)
	}
}

// ---------------------------------------------------------------------------
// Claude Code permission prompt → should detect ActivityNeedsInput
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
	if state != ActivityNeedsInput {
		t.Errorf("Claude permission prompt: state = %d, want ActivityNeedsInput (%d)", state, ActivityNeedsInput)
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
	if state != ActivityNeedsInput {
		t.Errorf("Claude y/n prompt: state = %d, want ActivityNeedsInput (%d)\nPlainTextRow = %q",
			state, ActivityNeedsInput, plain)
	}
}

// ---------------------------------------------------------------------------
// Rich TUI with status bar + prompt → should still detect through noise
// ---------------------------------------------------------------------------

func TestRealistic_StatusBarWithPromptBelow(t *testing.T) {
	sess := newStaleSession(10, 80)
	// Simulate a TUI with status bar at top and shell prompt at bottom.
	// This is what many terminal applications look like.
	sess.Screen.Write([]byte(
		"\x1b[7m Status: OK | Branch: main | $1.23 \x1b[0m\r\n" + // reverse video status bar
			"some build output\r\n" +
			"another line\r\n" +
			"\x1b[32muser@host\x1b[0m:\x1b[34m~\x1b[0m$ ",
	))

	state := sess.DetectActivity()
	if state != ActivityDone {
		// Log all rows to help debug
		for r := 0; r < 10; r++ {
			row := sess.Screen.PlainTextRow(r)
			if row != "" {
				t.Logf("Row %d: %q", r, row)
			}
		}
		t.Errorf("Status bar + prompt: state = %d, want ActivityDone (%d)", state, ActivityDone)
	}
}

// ---------------------------------------------------------------------------
// Active output (not stale) — should NOT trigger any classification
// ---------------------------------------------------------------------------

func TestRealistic_ActiveOutput_NoClassification(t *testing.T) {
	sess := NewSession(1, 10, 80)
	// Even with a prompt on screen, if output was recent, stay Active
	sess.Screen.Write([]byte("\x1b[32m$\x1b[0m "))

	sess.mu.Lock()
	sess.LastOutputAt = time.Now() // just now
	sess.Activity = ActivityActive
	sess.mu.Unlock()

	state := sess.DetectActivity()
	if state != ActivityActive {
		t.Errorf("Active output with prompt on screen: state = %d, want ActivityActive (%d)",
			state, ActivityActive)
	}
}

// ---------------------------------------------------------------------------
// Cursor positioning + erase sequences (common in TUIs) → test prompt detection
// ---------------------------------------------------------------------------

func TestRealistic_CursorPositioning_ThenPrompt(t *testing.T) {
	sess := newStaleSession(10, 80)
	// Simulate a program that uses cursor positioning (like a TUI)
	// then leaves a prompt at a specific position
	sess.Screen.Write([]byte(
		"\x1b[2J" +       // clear screen
			"\x1b[1;1H" + // cursor to (1,1)
			"Welcome to the app\r\n" +
			"\x1b[10;1H" + // jump to row 10
			"$ ",
	))

	state := sess.DetectActivity()
	if state != ActivityDone {
		for r := 0; r < 10; r++ {
			row := sess.Screen.PlainTextRow(r)
			if row != "" {
				t.Logf("Row %d: %q", r, row)
			}
		}
		t.Errorf("Cursor-positioned prompt: state = %d, want ActivityDone (%d)", state, ActivityDone)
	}
}

// ---------------------------------------------------------------------------
// Multiline compile output with no prompt → should be ActivityIdle
// ---------------------------------------------------------------------------

func TestRealistic_CompileOutput_NoPrompt(t *testing.T) {
	sess := newStaleSession(10, 80)
	sess.Screen.Write([]byte(
		"\x1b[1;36mCompiling\x1b[0m main.go...\r\n" +
			"\x1b[1;36mCompiling\x1b[0m utils.go...\r\n" +
			"\x1b[1;32m✓\x1b[0m Build successful (2.3s)\r\n",
	))

	state := sess.DetectActivity()
	if state != ActivityIdle {
		for r := 0; r < 10; r++ {
			row := sess.Screen.PlainTextRow(r)
			if row != "" {
				t.Logf("Row %d: %q", r, row)
			}
		}
		t.Errorf("Compile output with no prompt: state = %d, want ActivityIdle (%d)", state, ActivityIdle)
	}
}

// ---------------------------------------------------------------------------
// Windows cmd.exe prompt with ANSI → should detect ActivityDone
// ---------------------------------------------------------------------------

func TestRealistic_WindowsCmdPrompt(t *testing.T) {
	sess := newStaleSession(5, 80)
	sess.Screen.Write([]byte(
		"Microsoft Windows [Version 10.0.22621]\r\n" +
			"(c) Microsoft Corporation.\r\n" +
			"\r\n" +
			"C:\\Users\\test>",
	))

	state := sess.DetectActivity()
	if state != ActivityDone {
		for r := 0; r < 5; r++ {
			t.Logf("Row %d: %q", r, sess.Screen.PlainTextRow(r))
		}
		t.Errorf("Windows cmd prompt: state = %d, want ActivityDone (%d)", state, ActivityDone)
	}
}

// ---------------------------------------------------------------------------
// Full end-to-end: raw PTY bytes → Screen → DetectActivity → correct state
// This simulates the exact path that app_scan.go takes.
// ---------------------------------------------------------------------------

func TestRealistic_EndToEnd_DoneGreenGlow(t *testing.T) {
	sess := NewSession(1, 10, 80)

	// Phase 1: "Claude is working" — active output arriving
	sess.mu.Lock()
	sess.Activity = ActivityActive
	sess.LastOutputAt = time.Now()
	sess.mu.Unlock()

	// Feed some ANSI output as if from PTY
	sess.Screen.Write([]byte("\x1b[1;36mAnalyzing code...\x1b[0m\r\n"))
	sess.Screen.Write([]byte("\x1b[1;36mWriting fix...\x1b[0m\r\n"))

	// Still recent → should stay Active
	state := sess.DetectActivity()
	if state != ActivityActive {
		t.Errorf("Phase 1: state = %d, want ActivityActive", state)
	}

	// Phase 2: Output stops, prompt appears
	sess.Screen.Write([]byte("\x1b[32m✓ Done.\x1b[0m\r\n"))
	sess.Screen.Write([]byte("\x1b[1;35m❯\x1b[0m "))

	// Make output stale
	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.mu.Unlock()

	state = sess.DetectActivity()
	if state != ActivityDone {
		for r := 0; r < 10; r++ {
			row := sess.Screen.PlainTextRow(r)
			if row != "" {
				t.Logf("Row %d: %q", r, row)
			}
		}
		t.Errorf("Phase 2: state = %d, want ActivityDone (green glow)", state)
	}
}

func TestRealistic_EndToEnd_NeedsInputYellowPulse(t *testing.T) {
	sess := NewSession(1, 10, 80)

	// Phase 1: Claude is working
	sess.mu.Lock()
	sess.Activity = ActivityActive
	sess.LastOutputAt = time.Now()
	sess.mu.Unlock()

	sess.Screen.Write([]byte("\x1b[1mReading file...\x1b[0m\r\n"))

	// Phase 2: Claude asks for permission with ANSI-styled prompt
	sess.Screen.Write([]byte(
		"\x1b[33m⚠ Allow\x1b[0m access to \x1b[1m/etc/hosts\x1b[0m? [Y/n] ",
	))

	// Make output stale
	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.mu.Unlock()

	state := sess.DetectActivity()
	if state != ActivityNeedsInput {
		for r := 0; r < 10; r++ {
			row := sess.Screen.PlainTextRow(r)
			if row != "" {
				t.Logf("Row %d: %q", r, row)
			}
		}
		t.Errorf("Phase 2: state = %d, want ActivityNeedsInput (yellow pulse)", state)
	}
}

// ---------------------------------------------------------------------------
// PlainTextRow verification: make sure ANSI is properly stripped
// ---------------------------------------------------------------------------

func TestRealistic_PlainTextRow_StripsSGR(t *testing.T) {
	s := NewScreen(3, 80)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"bold+color+reset",
			"\x1b[1;31mERROR\x1b[0m: something failed",
			"ERROR: something failed",
		},
		{
			"256-color",
			"\x1b[38;5;196mRed text\x1b[0m normal",
			"Red text normal",
		},
		{
			"truecolor RGB",
			"\x1b[38;2;255;128;0mOrange\x1b[0m end",
			"Orange end",
		},
		{
			"reverse video status bar",
			"\x1b[7m STATUS BAR \x1b[0m",
			" STATUS BAR",
		},
		{
			"nested styles",
			"\x1b[1m\x1b[4m\x1b[32mBold Underline Green\x1b[0m",
			"Bold Underline Green",
		},
	}

	for _, tt := range tests {
		s.Write([]byte("\x1b[2J\x1b[H")) // clear + home
		s.Write([]byte(tt.input))
		got := s.PlainTextRow(0)
		if got != tt.want {
			t.Errorf("%s: PlainTextRow = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Prompt pattern after heavy ANSI: the key question is whether ❯ survives
// ---------------------------------------------------------------------------

func TestRealistic_PromptPattern_AfterANSIStrip(t *testing.T) {
	s := NewScreen(3, 80)

	prompts := []struct {
		name  string
		ansi  string
		plain string
	}{
		{
			"colored ❯",
			"\x1b[1;35m❯\x1b[0m ",
			"❯",
		},
		{
			"colored $ with user@host",
			"\x1b[01;32muser@host\x1b[00m:\x1b[01;34m~\x1b[00m$ ",
			"user@host:~$",
		},
		{
			"bare > with dim color",
			"\x1b[2m>\x1b[0m ",
			">",
		},
	}

	for _, tt := range prompts {
		s.Write([]byte("\x1b[2J\x1b[H")) // clear + home
		s.Write([]byte(tt.ansi))
		got := s.PlainTextRow(0)
		t.Logf("%s: ANSI %q → PlainText %q", tt.name, tt.ansi, got)

		// Check that the prompt character survives ANSI stripping
		if !strings.Contains(got, tt.plain) {
			t.Errorf("%s: PlainTextRow = %q, does not contain %q", tt.name, got, tt.plain)
		}

		// Check that promptPattern matches the stripped text
		trimmed := strings.TrimSpace(got)
		if !promptPattern.MatchString(trimmed) {
			t.Errorf("%s: promptPattern does NOT match %q (after ANSI strip)", tt.name, trimmed)
		}
	}
}

// ---------------------------------------------------------------------------
// Key finding: "user@host:~$" — the $ has no preceding whitespace!
// The promptPattern requires (?:^|\s) before [>$%#], but in "host:~$"
// the ~ directly precedes $. This tests whether real prompts are detected.
// ---------------------------------------------------------------------------

func TestRealistic_BashPrompt_NoSpaceBeforeDollar(t *testing.T) {
	sess := newStaleSession(5, 80)
	// This is the most common bash prompt format and it DOESN'T have
	// a space before the $. The regex (?:^|\s)[>$%#]\s*$ won't match "~$".
	sess.Screen.Write([]byte(
		"\x1b[01;32muser@host\x1b[00m:\x1b[01;34m~/project\x1b[00m$ ",
	))

	plain := sess.Screen.PlainTextRow(0)
	trimmed := strings.TrimSpace(plain)
	t.Logf("Plain prompt: %q", trimmed)

	// This is the critical test: does the prompt regex match "user@host:~/project$"?
	if !promptPattern.MatchString(trimmed) {
		t.Errorf("REAL BUG: promptPattern does NOT match %q — "+
			"the regex requires whitespace or ^ before $, but 'host:~/project$' has '~' before '$'. "+
			"Common bash prompts like 'user@host:~/dir$ ' will never trigger the green glow!",
			trimmed)
	}

	state := sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Realistic bash prompt: state = %d, want ActivityDone (%d)\n"+
			"This means the green glow NEVER fires for standard bash prompts!",
			state, ActivityDone)
	}
}
