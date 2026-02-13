package terminal

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

// SessionStatus represents the current state of a terminal session.
type SessionStatus int

const (
	StatusRunning    SessionStatus = iota // process is alive
	StatusExited                          // process has exited
	StatusError                           // an error occurred
)

// Session wraps a PTY-backed shell process and its virtual screen.
// It manages the full lifecycle: start → read loop → resize → close.
type Session struct {
	mu sync.Mutex

	ID     int           // unique session identifier
	Screen *Screen       // VT100 virtual screen buffer
	Status SessionStatus // current lifecycle status
	Title  string        // derived from OSC or user-set

	ptmx *os.File   // PTY master file descriptor
	cmd  *exec.Cmd  // the spawned process
	done chan struct{}

	// OutputCh receives a signal each time new data is written to Screen.
	// The main Bubbletea loop can select on this to know when to re-render.
	OutputCh chan struct{}

	// ExitCode is set when the process terminates.
	ExitCode int
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

// Start launches the given command (e.g. []string{"bash"} or
// []string{"claude", "--dangerously-skip-permissions"}) inside a new PTY.
// The working directory is set to dir.
// If enableKittyKbd is true, the kitty keyboard protocol is enabled after
// start so that Shift+Enter and other modified keys are reported correctly.
func (s *Session) Start(argv []string, dir string, env []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(argv) == 0 {
		argv = []string{os.Getenv("SHELL")}
		if argv[0] == "" {
			argv = []string{"/bin/bash"}
		}
	}

	s.cmd = exec.Command(argv[0], argv[1:]...)
	s.cmd.Dir = dir

	// Always set TERM and COLORTERM so child processes see a capable terminal.
	baseEnv := append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)
	s.cmd.Env = append(baseEnv, env...)

	rows := s.Screen.Rows()
	cols := s.Screen.Cols()

	ptmx, err := pty.StartWithSize(s.cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		s.Status = StatusError
		return err
	}
	s.ptmx = ptmx

	// Read loop: PTY output → Screen
	go s.readLoop()

	// Wait for process exit
	go s.waitLoop()

	return nil
}

// EnableKittyKeyboard sends the kitty keyboard protocol enable sequence
// (CSI > 1 u) to the PTY. This tells applications inside the terminal
// (like Claude Code) that Shift+Enter and other modified keys will be
// reported as distinct CSI u escape sequences.
//
// Call this after Start for Claude Code sessions so Shift+Enter (for
// multiline input) works out of the box.
func (s *Session) EnableKittyKeyboard() {
	s.mu.Lock()
	ptmx := s.ptmx
	s.mu.Unlock()
	if ptmx != nil {
		// CSI > 1 u  — push "disambiguate escape codes" flag onto the stack.
		// This makes Shift+Enter report as \x1b[13;2u instead of plain \r.
		ptmx.Write([]byte("\x1b[>1u"))
	}
}

// DisableKittyKeyboard pops the kitty keyboard protocol flags (CSI < 1 u).
// Called during cleanup / Close.
func (s *Session) DisableKittyKeyboard() {
	s.mu.Lock()
	ptmx := s.ptmx
	s.mu.Unlock()
	if ptmx != nil {
		ptmx.Write([]byte("\x1b[<1u"))
	}
}

// readLoop continuously reads from the PTY and writes to the Screen.
func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.Screen.Write(buf[:n])
			// Update title from OSC if changed
			s.mu.Lock()
			if s.Screen.Title != "" {
				s.Title = s.Screen.Title
			}
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

// waitLoop waits for the process to exit and updates the session status.
func (s *Session) waitLoop() {
	err := s.cmd.Wait()
	s.mu.Lock()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			s.ExitCode = exitErr.ExitCode()
		}
		s.Status = StatusExited
	} else {
		s.ExitCode = 0
		s.Status = StatusExited
	}
	s.mu.Unlock()
	close(s.done)
}

// Write sends raw bytes to the PTY (i.e. keyboard input from the user).
func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	ptmx := s.ptmx
	s.mu.Unlock()
	if ptmx == nil {
		return 0, io.ErrClosedPipe
	}
	return ptmx.Write(p)
}

// Resize updates the PTY and Screen dimensions.
func (s *Session) Resize(rows, cols int) {
	s.mu.Lock()
	ptmx := s.ptmx
	s.mu.Unlock()

	s.Screen.Resize(rows, cols)
	if ptmx != nil {
		_ = pty.Setsize(ptmx, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
	}
}

// Close terminates the session: kills the process and closes the PTY.
func (s *Session) Close() {
	s.mu.Lock()
	ptmx := s.ptmx
	cmd := s.cmd
	s.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		// Send SIGHUP first, then SIGKILL as fallback
		_ = cmd.Process.Signal(syscall.SIGHUP)
	}
	if ptmx != nil {
		ptmx.Close()
	}

	// Wait for the process to actually finish (with timeout via done channel)
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
