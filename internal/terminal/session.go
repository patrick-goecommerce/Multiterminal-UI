// Package terminal provides VT100 terminal emulation and PTY session management.
//
// The Session type is cross-platform: it uses github.com/aymanbagabas/go-pty
// which wraps Unix PTYs and Windows ConPTY behind a single interface.
// This means the same binary works on Linux, macOS, AND Windows.
package terminal

import (
	"io"
	"sync"
	"time"

	gopty "github.com/aymanbagabas/go-pty"
)

// SessionStatus represents the current state of a terminal session.
type SessionStatus int

const (
	StatusRunning    SessionStatus = iota // process is alive
	StatusExited                          // process has exited
	StatusError                           // an error occurred
	StatusSuspending                      // suspend armed: process still alive, kill pending
	StatusSuspended                       // process tree killed on purpose, session object alive
)

// Session wraps a PTY-backed shell process and its virtual screen.
// It manages the full lifecycle: start → read loop → resize → close.
type Session struct {
	mu sync.Mutex

	ID     int           // unique session identifier
	Screen *Screen       // VT100 virtual screen buffer
	Status SessionStatus // current lifecycle status
	Title  string        // derived from OSC or user-set
	Dir    string        // working directory the session was started in

	p   gopty.Pty  // cross-platform PTY (Unix PTY or Windows ConPTY)
	cmd *gopty.Cmd // the spawned child process

	// done is re-armed per process generation: it is closed when the current
	// generation's process exits, and replaced by Resume. Read it via Done().
	done chan struct{}

	// sus holds the process-generation bookkeeping (see session_suspend.go).
	sus suspendState

	// OutputCh receives a signal each time new data is written to Screen.
	OutputCh chan struct{}

	// RawOutputCh carries raw PTY output bytes for the GUI frontend (xterm.js).
	// Each message is a copy of the bytes read from the PTY.
	RawOutputCh chan []byte

	// ExitCode is set when the process terminates.
	ExitCode int

	// LastOutputAt records when the last PTY output was received.
	LastOutputAt time.Time

	// Activity tracks the current activity state for Claude panes.
	Activity ActivityState

	// Tokens holds parsed token usage / cost information.
	Tokens TokenInfo

	// Statusline-sourced telemetry (Claude's statusLine JSON via the forwarder shim).
	// When costSource == CostSourceStatusline, the screen scrape must not overwrite cost.
	contextPct int
	model      string
	costSource CostSource

	// hookSessionID / hasHookData: set by Claude Code hook events.
	// hasHookData=true means the scan loop skips DetectActivity() for this session.
	hookSessionID string
	hasHookData   bool
}

// NewSession creates a Session with the given screen dimensions but does not
// start any process yet. Call Start to spawn the shell.
func NewSession(id, rows, cols int) *Session {
	return &Session{
		ID:          id,
		Screen:      NewScreen(rows, cols),
		Status:      StatusRunning,
		OutputCh:    make(chan struct{}, 1),
		RawOutputCh: make(chan []byte, 256),
		done:        make(chan struct{}),
	}
}

// Start launches the given command inside a new PTY.
// argv is the command + arguments (e.g. []string{"bash"} or
// []string{"claude", "--dangerously-skip-permissions"}).
// dir is the working directory; env holds additional environment variables.
//
// The actual spawn lives in spawnLocked (session_spawn.go) so Resume can
// re-run it verbatim for the next process generation.
func (s *Session) Start(argv []string, dir string, env []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sus.readExit = make(chan struct{})
	if err := s.spawnLocked(argv, dir, env); err != nil {
		s.Status = StatusError
		close(s.sus.readExit)
		s.sus.readExit = nil
		return err
	}
	return nil
}

// Write sends raw bytes to the PTY (i.e. keyboard input or pasted text).
//
// Flow control relies on natural pipe backpressure rather than a fixed timed
// throttle. go-pty's Write forwards to a blocking OS pipe (conPty.inPipe):
// it delivers every byte and blocks when the pipe buffer is full until the
// console host drains it. That paces large pastes to the consumer's actual
// read speed, instead of guessing at a per-chunk sleep that is either too
// short (risking loss) or needlessly slow.
//
// We still bound each write to whole UTF-8 runes: the Windows console host
// decodes pipe input to UTF-16, and a multi-byte sequence split across a
// write boundary can be corrupted. utf8SafeChunkLen guarantees no rune
// straddles a write.
func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	pty := s.p
	suspended := s.Status == StatusSuspended
	s.mu.Unlock()
	// A suspended session has no PTY on purpose. Report that explicitly instead
	// of the generic closed-pipe error so callers can wake it (or refuse) rather
	// than dropping the input silently.
	if suspended {
		return 0, ErrSuspended
	}
	if pty == nil {
		return 0, io.ErrClosedPipe
	}

	const chunkSize = 4096
	total := 0
	for len(p) > 0 {
		end := utf8SafeChunkLen(p, chunkSize)
		n, err := pty.Write(p[:end])
		total += n
		if err != nil {
			return total, err
		}
		p = p[n:]
	}
	return total, nil
}

// utf8SafeChunkLen returns a chunk length (1..=max) that does not split a
// multi-byte UTF-8 sequence in p. If the byte at index max is a UTF-8
// continuation byte (10xxxxxx), a rune straddles the boundary, so the length
// is moved back to the start of that rune, keeping the whole sequence in one
// write.
//
// This matters on Windows ConPTY: it decodes each write independently, so a
// UTF-8 sequence split across two writes (e.g. box-drawing characters in a
// pasted table landing on a 512-byte boundary) gets corrupted or dropped.
// Pure-ASCII input is never affected, which is why plain text always pastes
// cleanly while text with special characters fails intermittently.
func utf8SafeChunkLen(p []byte, max int) int {
	if len(p) <= max {
		return len(p)
	}
	end := max
	// A continuation byte has the form 10xxxxxx. Walk back until end points at
	// the lead byte of the straddling rune (or the start of the buffer).
	for end > 0 && p[end]&0xC0 == 0x80 {
		end--
	}
	// Defensive: never return 0 (would stall the write loop). This only
	// happens for malformed input lacking a lead byte within a full chunk.
	if end == 0 {
		return max
	}
	return end
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

// Close terminates the session for good: kills the current process, closes the
// PTY and — as the very last step — closes RawOutputCh. It is idempotent: a
// second call returns immediately instead of panicking on a double close.
//
// RawOutputCh is closed here and nowhere else. readLoop must never close it,
// because a suspended session keeps the same channel across process
// generations while collectOutput (app_stream.go) reads it without a lock.
func (s *Session) Close() {
	s.mu.Lock()
	if s.sus.closed {
		s.mu.Unlock()
		return
	}
	s.sus.closed = true
	cmd, pty := s.cmd, s.p
	done, readExit := s.done, s.sus.readExit
	s.closeWakeLocked() // release anyone waiting for a resume that will never come
	s.mu.Unlock()

	// Kill the process first
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	// Close the PTY (also kills on Windows via ConPTY)
	if pty != nil {
		pty.Close()
	}

	// Only a live generation has loops to wait for. A suspended session (its
	// process is already gone, done closed) and a session that never started
	// (done never closed) must not block here.
	if cmd != nil {
		<-done
		if !waitClosed(readExit, readLoopDrainTimeout) {
			// The PTY read did not return (a ConPTY handle can outlive its
			// process). readLoop may still be sending, so closing RawOutputCh
			// now would panic it. Leave the channel open — collectOutput then
			// ends with its context instead. Leaking one channel beats
			// crashing the app.
			return
		}
	}
	s.closeRawOutput()
}

// readLoopDrainTimeout bounds how long Close waits for a generation's readLoop
// to return before giving up on closing RawOutputCh.
const readLoopDrainTimeout = 2 * time.Second

// Done returns a channel that is closed when the current process generation
// exits. Resume replaces it, so callers that outlive a suspend must re-read it
// (see watchExit in app_suspend.go) instead of caching the value.
func (s *Session) Done() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

// IsRunning reports whether the process is still alive.
func (s *Session) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status == StatusRunning
}

// GetTokens returns a snapshot of the token/cost info.
func (s *Session) GetTokens() TokenInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Tokens
}

// GetTitle returns the current OSC-derived window title, read thread-safe
// straight from the screen buffer (the source of truth for OSC 0/2 titles).
func (s *Session) GetTitle() string {
	return s.Screen.GetTitle()
}

// Name returns the session's display title (the sticky last-non-empty OSC title
// cached on the session). Read thread-safe under s.mu — readLoop writes Title
// under the same lock.
func (s *Session) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Title
}

// Pid returns the wrapper process id (cmd.exe on Windows), or 0 before Start.
// The finish flow needs it to kill the whole process tree BEFORE Close():
// after Process.Kill() the grandchildren are orphaned and taskkill /T cannot
// find them anymore.
func (s *Session) Pid() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Pid
	}
	return 0
}
