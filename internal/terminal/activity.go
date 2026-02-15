package terminal

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TokenInfo holds parsed token usage and cost data from Claude Code output.
type TokenInfo struct {
	TotalCost    float64 // accumulated cost in dollars
	InputTokens  int     // total input tokens
	OutputTokens int     // total output tokens
}

// ActivityState describes what a Claude session is currently doing.
type ActivityState int

const (
	ActivityIdle       ActivityState = iota // no recent output
	ActivityActive                          // currently producing output
	ActivityDone                            // just finished (prompt returned)
	ActivityNeedsInput                      // waiting for user confirmation
)

// ScanTokens scans the screen buffer for token/cost patterns and updates
// the Tokens field. Call this periodically (e.g. from the tick handler).
func (s *Session) ScanTokens() {
	rows := s.Screen.Rows()
	// Scan last 10 rows of the screen for cost/token patterns
	var text strings.Builder
	scanStart := rows - 10
	if scanStart < 0 {
		scanStart = 0
	}
	for r := scanStart; r < rows; r++ {
		text.WriteString(s.Screen.PlainTextRow(r))
		text.WriteByte('\n')
	}
	content := text.String()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Look for cost patterns like $0.12 or $1.50
	if matches := costPattern.FindStringSubmatch(content); len(matches) >= 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			s.Tokens.TotalCost = v
		}
	}

	// Look for token patterns like "15.2k input" or "3.8k output"
	if matches := inputTokenPattern.FindStringSubmatch(content); len(matches) >= 2 {
		s.Tokens.InputTokens = parseTokenCount(matches[1])
	}
	if matches := outputTokenPattern.FindStringSubmatch(content); len(matches) >= 2 {
		s.Tokens.OutputTokens = parseTokenCount(matches[1])
	}
}

// DetectActivity checks screen content for prompt/input patterns and
// updates the Activity state. Call this periodically.
func (s *Session) DetectActivity() ActivityState {
	s.mu.Lock()
	lastOutput := s.LastOutputAt
	currentActivity := s.Activity
	s.mu.Unlock()

	// Need output to have happened at all
	if lastOutput.IsZero() {
		return currentActivity
	}

	elapsed := time.Since(lastOutput)

	// While actively producing output, stay in Active state
	if elapsed < 1500*time.Millisecond {
		return currentActivity
	}

	// Output stopped for >1.5s — classify what's on screen
	newState := s.classifyScreenState()
	s.mu.Lock()
	s.Activity = newState
	s.mu.Unlock()
	return newState
}

// classifyScreenState examines the last rows of the screen to determine
// if Claude is done or waiting for input.
func (s *Session) classifyScreenState() ActivityState {
	rows := s.Screen.Rows()
	// Check last 15 non-empty rows (Claude Code uses a rich TUI with status bars)
	scanFrom := rows - 15
	if scanFrom < 0 {
		scanFrom = 0
	}
	for r := rows - 1; r >= scanFrom; r-- {
		line := s.Screen.PlainTextRow(r)
		if line == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)

		// Needs input patterns (check first — takes priority)
		if needsInputPattern.MatchString(trimmed) {
			return ActivityNeedsInput
		}

		// Prompt returned (Claude/shell is done)
		if promptPattern.MatchString(trimmed) {
			return ActivityDone
		}
	}
	return ActivityIdle
}

// ResetActivity sets the activity state back to Idle.
func (s *Session) ResetActivity() {
	s.mu.Lock()
	s.Activity = ActivityIdle
	s.mu.Unlock()
}

// Token/cost regex patterns
var (
	costPattern        = regexp.MustCompile(`\$(\d+\.\d+)`)
	inputTokenPattern  = regexp.MustCompile(`(\d+\.?\d*[kK]?)\s*(?:input|in\b)`)
	outputTokenPattern = regexp.MustCompile(`(\d+\.?\d*[kK]?)\s*(?:output|out\b)`)

	// Needs user input: permission prompts, Y/n confirmations, etc.
	needsInputPattern = regexp.MustCompile(`(?i)` +
		`\[Y/n\]|\[y/N\]|\(y/n\)|` + // Classic Y/n prompts
		`(?:proceed|continue|confirm|approve|allow)\s*\?|` + // Question prompts
		`permission|Do you want to|Would you like to|` + // Permission phrases
		`Press Enter to|waiting for|Waiting for`)

	// Prompt returned — Claude or shell is done and waiting for new input.
	// Matches: ❯, >, $, %, # at end of line (with optional whitespace)
	// Also matches Windows cmd.exe prompt like C:\path>
	// Note: no whitespace requirement before $/%/# because real prompts like
	// "user@host:~/project$" have path chars directly before the prompt char.
	promptPattern = regexp.MustCompile(
		`[❯›»]\s*$|` + // Claude Code prompt characters (U+276F, U+203A, U+00BB)
		`[>$%#]\s*$|` + // Unix shell prompts (at end of line)
		`^[A-Za-z]:\\[^>]*>\s*$`) // Windows cmd.exe prompt (C:\Users\x>)
)

// parseTokenCount converts strings like "15.2k" or "3800" to an integer.
func parseTokenCount(s string) int {
	s = strings.TrimSpace(s)
	multiplier := 1.0
	if strings.HasSuffix(strings.ToLower(s), "k") {
		multiplier = 1000
		s = s[:len(s)-1]
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(v * multiplier)
}
