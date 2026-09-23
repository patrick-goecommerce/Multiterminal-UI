package terminal

import "time"

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
	cmd, pty, job := s.cmd, s.p, s.job
	s.job = nil
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
	}
	// After the root is gone: whatever the tree kill missed is still in the job.
	job.Release()
	if cmd != nil {
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
