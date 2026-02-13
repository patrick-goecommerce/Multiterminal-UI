// Package terminal provides VT100 terminal emulation and PTY session management.
//
// The Session type is cross-platform: it uses github.com/aymanbagabas/go-pty
// which wraps Unix PTYs and Windows ConPTY behind a single interface.
// This means the same binary works on Linux, macOS, AND Windows.
package terminal

import (
	"io"
	"os"
	"runtime"
	"sync"

	gopty "github.com/aymanbagabas/go-pty"
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
