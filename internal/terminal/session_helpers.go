package terminal

import (
	"os"
	"runtime"
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
