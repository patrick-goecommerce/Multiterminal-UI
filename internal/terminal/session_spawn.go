package terminal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	gopty "github.com/aymanbagabas/go-pty"
)

// spawnLocked starts one process generation into this session. The caller must
// hold s.mu and must have armed s.done / s.sus.readExit for this generation.
//
// Start and Resume share this function on purpose: a resumed pane that is
// spawned with a different argv normalisation or a different environment is a
// subtly different pane (see sessionEnv in app_suspend.go).
func (s *Session) spawnLocked(argv []string, dir string, env []string) error {
	if s.sus.spawn != nil {
		// Test seam: lets the suspend/resume state machine be exercised without
		// a real PTY (see session_suspend_test.go).
		return s.sus.spawn(argv, dir, env)
	}

	s.Dir = dir

	// defaultShell already yields a real executable; only a caller-supplied
	// argv may need the Windows COMSPEC wrapper. The decision itself happens
	// further down, once the session's PATH is assembled (see windowsArgv).
	prepareArgv := true
	if len(argv) == 0 {
		argv = defaultShell()
		prepareArgv = false
	}

	fullEnv := buildEnv(env)

	// Decide the Windows wrapper here — after fullEnv is final — so the lookup
	// runs against the same PATH the child process gets. Resume goes through
	// this same path, so a woken pane is normalised exactly like a fresh one.
	if prepareArgv && runtime.GOOS == "windows" {
		argv = windowsArgv(argv, os.Getenv("COMSPEC"), sessionLookPath(fullEnv))
	}

	rows := s.Screen.Rows()
	cols := s.Screen.Cols()

	// Create the cross-platform PTY
	p, err := gopty.New()
	if err != nil {
		return err
	}

	// Set initial size (width=cols, height=rows)
	if err := p.Resize(cols, rows); err != nil {
		p.Close()
		return err
	}

	// Create the command to run inside the PTY
	cmd := p.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = fullEnv
	hidePTYConsole(cmd)

	if err := cmd.Start(); err != nil {
		p.Close()
		return err
	}

	s.p = p
	s.cmd = cmd

	gen := s.sus.gen
	done := s.done
	readExit := s.sus.readExit

	go s.readLoop(p, gen, done, readExit)
	go s.waitLoop(cmd, gen, done)

	return nil
}

// buildEnv assembles the child environment: the parent's, minus variables that
// would keep Claude Code from starting, plus our own.
func buildEnv(env []string) []string {
	parentEnv := os.Environ()
	fullEnv := make([]string, 0, len(parentEnv)+len(env)+2)
	for _, e := range parentEnv {
		// CLAUDECODE is set by Claude Code sessions; remove it so nested
		// Claude instances don't refuse to start.
		if strings.HasPrefix(e, "CLAUDECODE=") {
			continue
		}
		fullEnv = append(fullEnv, e)
	}
	fullEnv = append(fullEnv, "TERM=xterm-256color", "COLORTERM=truecolor")
	fullEnv = append(fullEnv, env...)

	// Prepend executable directory to PATH so the tmux shim is found first.
	// On Windows the env var is often "Path" not "PATH", so check case-insensitively.
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		found := false
		for i, e := range fullEnv {
			if len(e) > 5 && strings.EqualFold(e[:5], "PATH=") {
				fullEnv[i] = e[:5] + exeDir + string(os.PathListSeparator) + e[5:]
				found = true
				break
			}
		}
		if !found {
			fullEnv = append(fullEnv, "PATH="+exeDir)
		}
	}
	return fullEnv
}

// readLoop continuously reads from one generation's PTY and writes to the
// Screen. The PTY handle, the generation number and the generation's channels
// are passed in: reading them off the Session would race with Resume swapping
// them for the next generation.
//
// It never closes RawOutputCh — only Close() does (see Session.Close).
func (s *Session) readLoop(p gopty.Pty, gen int, done, exit chan struct{}) {
	defer close(exit)
	buf := make([]byte, 65536)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			s.Screen.Write(chunk)

			if !s.noteOutput(gen) {
				return // a newer generation owns the session
			}

			// Send raw bytes to GUI frontend (blocking with done-guard)
			select {
			case s.RawOutputCh <- chunk:
			case <-done:
			}

			// Signal for legacy TUI consumers (non-blocking)
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

// noteOutput records a fresh PTY chunk under s.mu. It reports false when the
// chunk belongs to a stale generation, i.e. the caller must stop.
//
// This is the abort half of the two-phase suspend commit: a chunk arriving
// after a suspend was armed marks it aborted in the very same critical section
// that TrySuspend used, so the kill goroutine cannot miss it.
func (s *Session) noteOutput(gen int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sus.gen != gen {
		return false
	}
	if s.Screen.Title != "" {
		s.Title = s.Screen.Title
	}
	s.LastOutputAt = time.Now()
	if s.Status == StatusSuspending {
		s.sus.aborted = true
	}
	s.Activity = ActivityActive
	return true
}

// waitLoop waits for one generation's process to exit and updates the session
// status. A suspend kills the process deliberately, so an armed or completed
// suspend must not be overwritten with StatusExited — that would drop the
// "Prozess beendet" overlay over a pane that is only sleeping.
func (s *Session) waitLoop(cmd *gopty.Cmd, gen int, done chan struct{}) {
	err := cmd.Wait()
	s.mu.Lock()
	if s.sus.gen == gen && s.Status == StatusRunning {
		if err != nil {
			if cmd.ProcessState != nil {
				s.ExitCode = cmd.ProcessState.ExitCode()
			} else {
				s.ExitCode = 1
			}
		} else {
			s.ExitCode = 0
		}
		s.Status = StatusExited
	}
	s.mu.Unlock()
	close(done)
}
