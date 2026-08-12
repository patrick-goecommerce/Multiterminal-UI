package terminal

import (
	"errors"
	"sync"
	"time"
)

// Suspend/resume ("pane sleeping", issue #180).
//
// A suspended session keeps its identity — ID, Screen, Tokens, hook session id,
// title — and only loses its process tree. That matters because the numeric
// session ID is the join key for hooks, queues, issue links, worktree state and
// the statusline forwarder: re-creating the pane instead would scatter all of
// them (and destroy the xterm.js scrollback in the frontend).
//
// Invariants:
//   - RawOutputCh is created once and closed once, by Close(). It is never
//     swapped, because collectOutput (app_stream.go) reads the field without a
//     lock — a swap would be a data race.
//   - done and readExit are re-armed per process generation.
//   - A suspended session has a closed done channel and no live cmd/PTY, so
//     Close() neither blocks nor kills anything.

// ErrSuspended is returned by Write when the session is asleep. Callers should
// wake it (or refuse the write) instead of dropping the input.
var ErrSuspended = errors.New("terminal: session is suspended")

// ErrNotSuspended is returned by Resume when the session is not asleep. Resume
// flips the status under the same lock it checks it in, so a second concurrent
// Resume gets this error rather than spawning a second process.
var ErrNotSuspended = errors.New("terminal: session is not suspended")

// suspendState is the process-generation bookkeeping of a Session. All fields
// are guarded by Session.mu unless noted.
type suspendState struct {
	gen      int           // current process generation; bumped by every Resume
	aborted  bool          // PTY output arrived after a suspend was armed
	resumeID string        // Claude session UUID to hand to --resume
	readExit chan struct{} // closed when the current generation's readLoop returns
	wake     chan struct{} // closed when a suspended session resumes or is closed
	closed   bool          // final Close() has run
	closeRaw sync.Once     // guards close(RawOutputCh): exactly one close, ever

	// spawn replaces the real PTY spawn in tests. Never set in production.
	spawn func(argv []string, dir string, env []string) error
}

// TrySuspend arms a suspend: it re-checks the preconditions under s.mu and, if
// they hold, flips the session to StatusSuspending.
//
// This is phase one of a two-phase commit. Candidates are picked lock-free
// (taskkill must never run under a lock), so between the decision and the kill
// the pane may have woken up. Only ActivityDone qualifies: ActivityIdle means
// "no prompt recognised", which is also what a running dev server, a pager or
// an interactive rebase look like.
func (s *Session) TrySuspend() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sus.closed || s.Status != StatusRunning || s.Activity != ActivityDone {
		return false
	}
	s.Status = StatusSuspending
	s.sus.aborted = false
	if s.sus.wake == nil {
		s.sus.wake = make(chan struct{})
	}
	return true
}

// SuspendAborted reports whether PTY output arrived since TrySuspend armed the
// suspend. The kill goroutine must check this immediately before killing.
func (s *Session) SuspendAborted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sus.aborted
}

// AbortSuspend rolls an armed suspend back to StatusRunning. Safe to call on a
// session that is no longer suspending (no-op).
func (s *Session) AbortSuspend() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status != StatusSuspending {
		return
	}
	s.Status = StatusRunning
	s.sus.aborted = false
	s.closeWakeLocked()
}

// FinishSuspend completes an armed suspend after the process tree was killed:
// it tears down this generation's PTY, waits for its loops and marks the
// session suspended. Identity (ID, Screen, Tokens, hook data, title, resume id)
// is untouched.
//
// It does NOT re-check the abort flag: by the time it runs the tree is already
// dead, so there is nothing left to roll back to. The abort check belongs
// before the kill.
func (s *Session) FinishSuspend() bool {
	s.mu.Lock()
	if s.Status != StatusSuspending {
		s.mu.Unlock()
		return false
	}
	cmd, pty := s.cmd, s.p
	done, readExit := s.done, s.sus.readExit
	s.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if pty != nil {
		pty.Close()
	}
	if cmd != nil {
		<-done
		// Bounded like Close(): a ConPTY read that never returns must not strand
		// the pane in StatusSuspending, where nothing would report its state.
		// A late readLoop is harmless — the generation bump makes it stop.
		waitClosed(readExit, readLoopDrainTimeout)
	}

	s.mu.Lock()
	s.p = nil
	s.cmd = nil
	s.sus.readExit = nil
	s.Status = StatusSuspended
	s.Activity = ActivityIdle
	s.mu.Unlock()
	return true
}

// Resume starts a fresh process generation into the same session object.
// argv/dir/env must be built the same way the original launch was (plus
// --resume), otherwise the woken pane loses its hook wiring or its worktree
// firewall.
func (s *Session) Resume(argv []string, dir string, env []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sus.closed || s.Status != StatusSuspended {
		return ErrNotSuspended
	}

	// Flip the status before spawning: a second concurrent Resume (click and
	// keystroke arriving together) then bails out with ErrNotSuspended instead
	// of starting a second process into the same session.
	s.sus.gen++
	s.done = make(chan struct{})
	s.sus.readExit = make(chan struct{})
	s.Status = StatusRunning
	s.Activity = ActivityIdle
	s.sus.aborted = false

	if err := s.spawnLocked(argv, dir, env); err != nil {
		// Roll back to a consistent suspended state: done stays closed so
		// Close() never blocks on a generation that never existed.
		close(s.done)
		close(s.sus.readExit)
		s.sus.readExit = nil
		s.Status = StatusSuspended
		return err
	}
	s.closeWakeLocked()
	return nil
}

// IsSuspended reports whether the session is asleep (process tree killed,
// object alive).
func (s *Session) IsSuspended() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status == StatusSuspended
}

// IsSuspendedOrSuspending covers the whole "do not treat this as an exit"
// window: from arming the suspend until the session is woken again.
func (s *Session) IsSuspendedOrSuspending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status == StatusSuspended || s.Status == StatusSuspending
}

// WaitAwake blocks while the session is suspended and returns true once it has
// been resumed, or false when it was closed for good. Used by watchExit so a
// deliberate suspend is not reported to the frontend as a process exit.
func (s *Session) WaitAwake() bool {
	s.mu.Lock()
	wake := s.sus.wake
	closed := s.sus.closed
	suspended := s.Status == StatusSuspended || s.Status == StatusSuspending
	s.mu.Unlock()

	if closed {
		return false
	}
	if !suspended || wake == nil {
		return true
	}
	<-wake

	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.sus.closed
}

// ResumeID returns the Claude session UUID to resume this pane with.
func (s *Session) ResumeID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sus.resumeID
}

// SetResumeID stores the Claude session UUID parsed from argv at launch.
// sess.HookSessionID() stays authoritative — it is also correct when Claude
// picks a different UUID internally — and overwrites this value when known.
func (s *Session) SetResumeID(id string) {
	s.mu.Lock()
	s.sus.resumeID = id
	s.mu.Unlock()
}

// SetSpawnForTest replaces the PTY spawn with fn. Tests in other packages use
// it to exercise suspend/resume without starting real processes; production
// code must never call it.
func (s *Session) SetSpawnForTest(fn func(argv []string, dir string, env []string) error) {
	s.mu.Lock()
	s.sus.spawn = fn
	s.mu.Unlock()
}

// Generation returns the current process generation (0 = first launch).
func (s *Session) Generation() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sus.gen
}

// closeWakeLocked releases WaitAwake waiters. Caller must hold s.mu.
func (s *Session) closeWakeLocked() {
	if s.sus.wake != nil {
		close(s.sus.wake)
		s.sus.wake = nil
	}
}

// waitClosed waits for ch to be closed and reports whether it actually was.
// A nil channel counts as closed (no generation was ever armed).
func waitClosed(ch <-chan struct{}, timeout time.Duration) bool {
	if ch == nil {
		return true
	}
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// closeRawOutput closes RawOutputCh exactly once, ever.
func (s *Session) closeRawOutput() {
	s.sus.closeRaw.Do(func() { close(s.RawOutputCh) })
}
