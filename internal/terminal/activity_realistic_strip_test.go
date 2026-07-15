package terminal

import (
	"strings"
	"testing"
)

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
