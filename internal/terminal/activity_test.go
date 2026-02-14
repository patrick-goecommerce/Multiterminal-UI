package terminal

import (
	"testing"
)

// ---------------------------------------------------------------------------
// parseTokenCount
// ---------------------------------------------------------------------------

func TestParseTokenCount_Plain(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"3800", 3800},
		{"0", 0},
		{"1", 1},
		{"999999", 999999},
	}
	for _, tt := range tests {
		got := parseTokenCount(tt.input)
		if got != tt.want {
			t.Errorf("parseTokenCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseTokenCount_KSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"15k", 15000},
		{"15K", 15000},
		{"15.2k", 15200},
		{"0.5k", 500},
		{"1.0k", 1000},
		{"100k", 100000},
	}
	for _, tt := range tests {
		got := parseTokenCount(tt.input)
		if got != tt.want {
			t.Errorf("parseTokenCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseTokenCount_Whitespace(t *testing.T) {
	got := parseTokenCount("  15k  ")
	if got != 15000 {
		t.Errorf("parseTokenCount with whitespace = %d, want 15000", got)
	}
}

func TestParseTokenCount_Invalid(t *testing.T) {
	tests := []string{"", "abc", "k", "K"}
	for _, input := range tests {
		got := parseTokenCount(input)
		if got != 0 {
			t.Errorf("parseTokenCount(%q) = %d, want 0", input, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Regex patterns: costPattern
// ---------------------------------------------------------------------------

func TestCostPattern_Match(t *testing.T) {
	tests := []struct {
		input string
		want  string // expected captured group
	}{
		{"Total cost: $0.12", "0.12"},
		{"$1.50 spent", "1.50"},
		{"Cost $0.00 so far", "0.00"},
		{"$123.45", "123.45"},
		{"prefix $99.99 suffix", "99.99"},
	}
	for _, tt := range tests {
		matches := costPattern.FindStringSubmatch(tt.input)
		if len(matches) < 2 {
			t.Errorf("costPattern did not match %q", tt.input)
			continue
		}
		if matches[1] != tt.want {
			t.Errorf("costPattern(%q) captured %q, want %q", tt.input, matches[1], tt.want)
		}
	}
}

func TestCostPattern_NoMatch(t *testing.T) {
	tests := []string{
		"no cost here",
		"$abc",
		"$ 1.00",   // space after $
		"$100",     // no decimal
		"100.00",   // no $
	}
	for _, input := range tests {
		if costPattern.MatchString(input) {
			t.Errorf("costPattern should NOT match %q", input)
		}
	}
}

// ---------------------------------------------------------------------------
// Regex patterns: inputTokenPattern
// ---------------------------------------------------------------------------

func TestInputTokenPattern_Match(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"15.2k input tokens", "15.2"},
		{"3800 input", "3800"},
		{"10k in total", "10"},
		{"5.5K input", "5.5"},
	}
	for _, tt := range tests {
		matches := inputTokenPattern.FindStringSubmatch(tt.input)
		if len(matches) < 2 {
			t.Errorf("inputTokenPattern did not match %q", tt.input)
			continue
		}
		if matches[1] != tt.want {
			t.Errorf("inputTokenPattern(%q) captured %q, want %q", tt.input, matches[1], tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Regex patterns: outputTokenPattern
// ---------------------------------------------------------------------------

func TestOutputTokenPattern_Match(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"3.8k output tokens", "3.8"},
		{"500 output", "500"},
		{"12k out", "12"},
	}
	for _, tt := range tests {
		matches := outputTokenPattern.FindStringSubmatch(tt.input)
		if len(matches) < 2 {
			t.Errorf("outputTokenPattern did not match %q", tt.input)
			continue
		}
		if matches[1] != tt.want {
			t.Errorf("outputTokenPattern(%q) captured %q, want %q", tt.input, matches[1], tt.want)
		}
	}
}

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
// ScanTokens – integration with screen buffer
// ---------------------------------------------------------------------------

func TestScanTokens_ExtractsCost(t *testing.T) {
	// Use a small screen (5 rows) so that text is within the last-10-rows scan window
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("Total: $3.45 | 15.2k input | 3.8k output\r\n"))

	sess.ScanTokens()

	tokens := sess.GetTokens()
	if tokens.TotalCost != 3.45 {
		t.Errorf("TotalCost = %f, want 3.45", tokens.TotalCost)
	}
	// BUG: inputTokenPattern/outputTokenPattern capture groups exclude [kK],
	// so parseTokenCount receives "15.2" instead of "15.2k" → returns 15 not 15200.
	// The [kK]? should be inside the capture group to fix this.
	if tokens.InputTokens != 15200 {
		t.Errorf("InputTokens = %d, want 15200 (known bug: [kK] outside capture group)", tokens.InputTokens)
	}
	if tokens.OutputTokens != 3800 {
		t.Errorf("OutputTokens = %d, want 3800 (known bug: [kK] outside capture group)", tokens.OutputTokens)
	}
}

func TestScanTokens_NoMatch(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("Hello world, no tokens here\r\n"))

	sess.ScanTokens()

	tokens := sess.GetTokens()
	if tokens.TotalCost != 0 {
		t.Errorf("TotalCost = %f, want 0", tokens.TotalCost)
	}
	if tokens.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", tokens.InputTokens)
	}
}

func TestScanTokens_CostOnLastRow(t *testing.T) {
	sess := NewSession(1, 5, 80) // only 5 rows
	// Fill screen so cost ends up on last visible row
	for i := 0; i < 4; i++ {
		sess.Screen.Write([]byte("filler line\r\n"))
	}
	sess.Screen.Write([]byte("$9.99"))

	sess.ScanTokens()

	tokens := sess.GetTokens()
	if tokens.TotalCost != 9.99 {
		t.Errorf("TotalCost = %f, want 9.99", tokens.TotalCost)
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
	if state != ActivityNeedsInput {
		t.Errorf("classifyScreenState = %d, want ActivityNeedsInput (%d)", state, ActivityNeedsInput)
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
	if state != ActivityNeedsInput {
		t.Errorf("classifyScreenState = %d, want ActivityNeedsInput (%d)", state, ActivityNeedsInput)
	}
}
