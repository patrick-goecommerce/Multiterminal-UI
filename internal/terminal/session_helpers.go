package terminal

import (
	"os"
	"runtime"
	"time"
)

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

// GetLastOutputAt returns when the last PTY output was received, under lock.
func (s *Session) GetLastOutputAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LastOutputAt
}

// SetLastOutputAtForTest backdates LastOutputAt so tests outside this package
// can simulate PTY output having gone stale, without a real Write() + sleep.
func (s *Session) SetLastOutputAtForTest(t time.Time) {
	s.mu.Lock()
	s.LastOutputAt = t
	s.mu.Unlock()
}

// SetHookActivityAtForTest backdates when a hook last set the activity, so
// tests outside this package can get past a settle time without sleeping.
func (s *Session) SetHookActivityAtForTest(t time.Time) {
	s.mu.Lock()
	s.hookActivityAt = t
	s.mu.Unlock()
}

// GetStatus returns the session's lifecycle status under lock. Reading the
// Status field directly races with the read and wait loops that write it.
func (s *Session) GetStatus() SessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status
}

// GetDir returns the working directory the session was started in.
func (s *Session) GetDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Dir
}

// GetExitCode returns the process exit code. It is only meaningful once the
// status is StatusExited.
func (s *Session) GetExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ExitCode
}
