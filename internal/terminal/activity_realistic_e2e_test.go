package terminal

import (
	"testing"
	"time"
)

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
	if state != ActivityWaitingAnswer {
		for r := 0; r < 10; r++ {
			row := sess.Screen.PlainTextRow(r)
			if row != "" {
				t.Logf("Row %d: %q", r, row)
			}
		}
		t.Errorf("Phase 2: state = %d, want ActivityWaitingAnswer (yellow pulse)", state)
	}
}
