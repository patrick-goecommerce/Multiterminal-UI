// Package terminal provides VT100 terminal emulation and PTY session management.
//
// The Session type is cross-platform: it uses github.com/aymanbagabas/go-pty
// which wraps Unix PTYs and Windows ConPTY behind a single interface.
// This means the same binary works on Linux, macOS, AND Windows.
package terminal

import (
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	gopty "github.com/aymanbagabas/go-pty"
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

// SessionStatus represents the current state of a terminal session.
type SessionStatus int

const (
	StatusRunning SessionStatus = iota // process is alive
	StatusExited                       // process has exited
	StatusError                        // an error occurred
)

// Session wraps a PTY-backed shell process and its virtual screen.
// It manages the full lifecycle: start → read loop → resize → close.
type Session struct {
	mu sync.Mutex

	ID     int           // unique session identifier
	Screen *Screen       // VT100 virtual screen buffer
	Status SessionStatus // current lifecycle status
	Title  string        // derived from OSC or user-set

	p   gopty.Pty  // cross-platform PTY (Unix PTY or Windows ConPTY)
	cmd *gopty.Cmd // the spawned child process

	done chan struct{}

	// OutputCh receives a signal each time new data is written to Screen.
	// The main Bubbletea loop can select on this to know when to re-render.
	OutputCh chan struct{}

	// ExitCode is set when the process terminates.
	ExitCode int

	// LastOutputAt records when the last PTY output was received.
	LastOutputAt time.Time

	// Activity tracks the current activity state for Claude panes.
	Activity ActivityState

	// Tokens holds parsed token usage / cost information.
	Tokens TokenInfo
}

// NewSession creates a Session with the given screen dimensions but does not
// start any process yet. Call Start to spawn the shell.
func NewSession(id, rows, cols int) *Session {
	return &Session{
		ID:       id,
		Screen:   NewScreen(rows, cols),
		Status:   StatusRunning,
		OutputCh: make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
}

// Start launches the given command inside a new PTY.
// argv is the command + arguments (e.g. []string{"bash"} or
// []string{"claude", "--dangerously-skip-permissions"}).
// dir is the working directory; env holds additional environment variables.
func (s *Session) Start(argv []string, dir string, env []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(argv) == 0 {
		argv = defaultShell()
	}

	// Always set TERM and COLORTERM so child processes see a capable terminal.
	fullEnv := append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)
	fullEnv = append(fullEnv, env...)

	rows := s.Screen.Rows()
	cols := s.Screen.Cols()

	// Create the cross-platform PTY
	p, err := gopty.New()
	if err != nil {
		s.Status = StatusError
		return err
	}

	// Set initial size (width=cols, height=rows)
	if err := p.Resize(cols, rows); err != nil {
		p.Close()
		s.Status = StatusError
		return err
	}

	// Create the command to run inside the PTY
	cmd := p.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = fullEnv

	if err := cmd.Start(); err != nil {
		p.Close()
		s.Status = StatusError
		return err
	}

	s.p = p
	s.cmd = cmd

	go s.readLoop()
	go s.waitLoop()

	return nil
}

// readLoop continuously reads from the PTY and writes to the Screen.
func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.p.Read(buf)
		if n > 0 {
			s.Screen.Write(buf[:n])
			// Update title and timestamps
			s.mu.Lock()
			if s.Screen.Title != "" {
				s.Title = s.Screen.Title
			}
			s.LastOutputAt = time.Now()
			s.Activity = ActivityActive
			s.mu.Unlock()
			// Signal that new output is available (non-blocking)
			select {
			case s.OutputCh <- struct{}{}:
			default:
			}
		}
		if err != nil {
			break
		}
	}
}

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

	// If we're not active (no recent output), check for idle timeout
	if currentActivity == ActivityActive && !lastOutput.IsZero() {
		if time.Since(lastOutput) > 2*time.Second {
			// Output stopped — check what's on screen
			newState := s.classifyScreenState()
			s.mu.Lock()
			s.Activity = newState
			s.mu.Unlock()
			return newState
		}
	}

	return currentActivity
}

// classifyScreenState examines the last few rows of the screen to determine
// if Claude is done or waiting for input.
func (s *Session) classifyScreenState() ActivityState {
	rows := s.Screen.Rows()
	// Check last 5 non-empty rows
	for r := rows - 1; r >= 0 && r >= rows-5; r-- {
		line := s.Screen.PlainTextRow(r)
		if line == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)

		// Needs input patterns
		if needsInputPattern.MatchString(trimmed) {
			return ActivityNeedsInput
		}

		// Prompt returned (Claude is done)
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
	inputTokenPattern  = regexp.MustCompile(`(\d+\.?\d*)[kK]?\s*(?:input|in\b)`)
	outputTokenPattern = regexp.MustCompile(`(\d+\.?\d*)[kK]?\s*(?:output|out\b)`)
	needsInputPattern  = regexp.MustCompile(`(?i)\[Y/n\]|\[y/N\]|\(y/n\)|proceed\?|continue\?|confirm|approve|allow|permission`)
	promptPattern      = regexp.MustCompile(`(?:^|\s)[>$%#]\s*$|^\s*[>]\s*$`)
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

// waitLoop waits for the process to exit and updates the session status.
func (s *Session) waitLoop() {
	err := s.cmd.Wait()
	s.mu.Lock()
	if err != nil {
		if s.cmd.ProcessState != nil {
			s.ExitCode = s.cmd.ProcessState.ExitCode()
		} else {
			s.ExitCode = 1
		}
	} else {
		s.ExitCode = 0
	}
	s.Status = StatusExited
	s.mu.Unlock()
	close(s.done)
}

// Write sends raw bytes to the PTY (i.e. keyboard input from the user).
func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	pty := s.p
	s.mu.Unlock()
	if pty == nil {
		return 0, io.ErrClosedPipe
	}
	return pty.Write(p)
}

// Resize updates the PTY and Screen dimensions.
func (s *Session) Resize(rows, cols int) {
	s.Screen.Resize(rows, cols)
	s.mu.Lock()
	pty := s.p
	s.mu.Unlock()
	if pty != nil {
		// go-pty uses (width, height) = (cols, rows)
		_ = pty.Resize(cols, rows)
	}
}

// Close terminates the session: kills the process and closes the PTY.
func (s *Session) Close() {
	s.mu.Lock()
	cmd := s.cmd
	pty := s.p
	s.mu.Unlock()

	// Kill the process first
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	// Close the PTY (also kills on Windows via ConPTY)
	if pty != nil {
		pty.Close()
	}

	// Wait for the process to actually finish
	<-s.done
}

// Done returns a channel that is closed when the session exits.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// IsRunning reports whether the process is still alive.
func (s *Session) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status == StatusRunning
}

// EnableKittyKeyboard sends the kitty keyboard protocol enable sequence
// (CSI > 1 u) to the PTY. This tells applications inside the terminal
// (like Claude Code) that Shift+Enter and other modified keys will be
// reported as distinct CSI u escape sequences.
func (s *Session) EnableKittyKeyboard() {
	s.mu.Lock()
	pty := s.p
	s.mu.Unlock()
	if pty != nil {
		pty.Write([]byte("\x1b[>1u"))
	}
}

// DisableKittyKeyboard pops the kitty keyboard protocol flags (CSI < 1 u).
func (s *Session) DisableKittyKeyboard() {
	s.mu.Lock()
	pty := s.p
	s.mu.Unlock()
	if pty != nil {
		pty.Write([]byte("\x1b[<1u"))
	}
}

// defaultShell returns the default shell command for the current OS.
func defaultShell() []string {
	if runtime.GOOS == "windows" {
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return []string{comspec}
		}
		return []string{"cmd.exe"}
	}
	// Unix
	if shell := os.Getenv("SHELL"); shell != "" {
		return []string{shell}
	}
	return []string{"/bin/bash"}
}
