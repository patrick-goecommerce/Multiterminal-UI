package terminal

import (
	"errors"
	"testing"
	"time"
)

// stubSpawn installs the test seam so the suspend/resume state machine can be
// exercised without spawning a real PTY. It records every spawn call.
type spawnRecorder struct {
	calls int
	argv  []string
	dir   string
	env   []string
	err   error
}

func (r *spawnRecorder) install(s *Session) {
	s.mu.Lock()
	s.sus.spawn = func(argv []string, dir string, env []string) error {
		r.calls++
		r.argv, r.dir, r.env = argv, dir, env
		return r.err
	}
	s.mu.Unlock()
}

// doneSession returns a session that looks like a finished Claude pane:
// running, activity "done", no live process (the PTY is stubbed out).
func doneSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession(42, 24, 80)
	s.SetHookActivity(ActivityDone)
	return s
}

func suspendedSession(t *testing.T) *Session {
	t.Helper()
	s := doneSession(t)
	if !s.TrySuspend() {
		t.Fatal("TrySuspend on a done session must succeed")
	}
	if !s.FinishSuspend() {
		t.Fatal("FinishSuspend must complete an armed suspend")
	}
	return s
}

// ---------------------------------------------------------------------------
// Gate: only ActivityDone may be suspended (design D4).
// ---------------------------------------------------------------------------

func TestTrySuspend_OnlyDoneIsAllowed(t *testing.T) {
	cases := []struct {
		name  string
		state ActivityState
		want  bool
	}{
		{"idle", ActivityIdle, false},
		{"active", ActivityActive, false},
		{"done", ActivityDone, true},
		{"waitingPermission", ActivityWaitingPermission, false},
		{"waitingAnswer", ActivityWaitingAnswer, false},
		{"error", ActivityError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSession(1, 24, 80)
			s.SetHookActivity(tc.state)
			if got := s.TrySuspend(); got != tc.want {
				t.Fatalf("TrySuspend with %s = %v, want %v", tc.name, got, tc.want)
			}
			wantStatus := StatusRunning
			if tc.want {
				wantStatus = StatusSuspending
			}
			s.mu.Lock()
			gotStatus := s.Status
			s.mu.Unlock()
			if gotStatus != wantStatus {
				t.Fatalf("status = %v, want %v", gotStatus, wantStatus)
			}
		})
	}
}

func TestTrySuspend_RejectsNonRunningStatus(t *testing.T) {
	for _, st := range []SessionStatus{StatusExited, StatusError, StatusSuspending, StatusSuspended} {
		s := NewSession(1, 24, 80)
		s.SetHookActivity(ActivityDone)
		s.mu.Lock()
		s.Status = st
		s.mu.Unlock()
		if s.TrySuspend() {
			t.Fatalf("TrySuspend must refuse status %v", st)
		}
	}
}

func TestTrySuspend_RejectsClosedSession(t *testing.T) {
	s := doneSession(t)
	s.Close()
	if s.TrySuspend() {
		t.Fatal("a closed session must never be suspendable")
	}
}

// ---------------------------------------------------------------------------
// Two-phase commit: output after arming aborts the suspend.
// ---------------------------------------------------------------------------

func TestSuspend_AbortedByOutputAfterArming(t *testing.T) {
	s := doneSession(t)
	if !s.TrySuspend() {
		t.Fatal("TrySuspend failed")
	}
	if s.SuspendAborted() {
		t.Fatal("nothing arrived yet — must not be aborted")
	}

	// readLoop's critical section: a chunk lands while the suspend is armed.
	if ok := s.noteOutput(s.Generation()); !ok {
		t.Fatal("noteOutput rejected the current generation")
	}

	if !s.SuspendAborted() {
		t.Fatal("output after arming must abort the suspend")
	}
	s.AbortSuspend()
	s.mu.Lock()
	status, aborted := s.Status, s.sus.aborted
	s.mu.Unlock()
	if status != StatusRunning {
		t.Fatalf("after AbortSuspend status = %v, want StatusRunning", status)
	}
	if aborted {
		t.Fatal("abort flag must be cleared for the next attempt")
	}
	if s.IsSuspended() {
		t.Fatal("an aborted suspend must not report the session as suspended")
	}
}

func TestNoteOutput_DoesNotAbortWhileRunning(t *testing.T) {
	s := doneSession(t)
	s.noteOutput(s.Generation())
	if s.SuspendAborted() {
		t.Fatal("output on a plain running session must not set the abort flag")
	}
}

func TestNoteOutput_StaleGenerationStops(t *testing.T) {
	s := suspendedSession(t)
	rec := &spawnRecorder{}
	rec.install(s)
	if err := s.Resume(nil, "", nil); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	// Generation 0's readLoop is still draining: it must stop, not clobber the
	// state of generation 1.
	if s.noteOutput(0) {
		t.Fatal("noteOutput must reject a stale generation")
	}
}

func TestFinishSuspend_RequiresArmedSuspend(t *testing.T) {
	s := doneSession(t)
	if s.FinishSuspend() {
		t.Fatal("FinishSuspend without TrySuspend must be a no-op")
	}
}

// ---------------------------------------------------------------------------
// Identity survives a suspend.
// ---------------------------------------------------------------------------

func TestSuspend_PreservesIdentity(t *testing.T) {
	s := NewSession(7, 24, 80)
	s.Screen.Write([]byte("hello from the pane"))
	s.SetHookSessionID("362bf457-1111-2222-3333-444455556666")
	s.SetResumeID("362bf457-1111-2222-3333-444455556666")
	s.mu.Lock()
	s.Tokens = TokenInfo{TotalCost: 1.25, InputTokens: 1500, OutputTokens: 900}
	s.Title = "claude — repo"
	s.mu.Unlock()
	s.SetHookActivity(ActivityDone)

	screenBefore := s.Screen.PlainText()
	rawBefore := s.RawOutputCh

	if !s.TrySuspend() || !s.FinishSuspend() {
		t.Fatal("suspend failed")
	}

	if s.ID != 7 {
		t.Fatalf("ID = %d, want 7", s.ID)
	}
	if got := s.Screen.PlainText(); got != screenBefore {
		t.Fatal("screen buffer must survive the suspend")
	}
	if got := s.GetTokens(); got.TotalCost != 1.25 || got.InputTokens != 1500 || got.OutputTokens != 900 {
		t.Fatalf("tokens lost: %+v", got)
	}
	if got := s.HookSessionID(); got != "362bf457-1111-2222-3333-444455556666" {
		t.Fatalf("hook session id = %q", got)
	}
	if got := s.ResumeID(); got != "362bf457-1111-2222-3333-444455556666" {
		t.Fatalf("resume id = %q", got)
	}
	if !s.HasHookData() {
		t.Fatal("hook data flag must survive the suspend")
	}
	if got := s.Name(); got != "claude — repo" {
		t.Fatalf("title = %q", got)
	}
	if !s.IsSuspended() {
		t.Fatal("IsSuspended must report true")
	}
	if s.RawOutputCh != rawBefore {
		t.Fatal("RawOutputCh must never be swapped — collectOutput reads it unlocked")
	}
}

func TestSuspend_DoesNotCloseRawOutputCh(t *testing.T) {
	s := suspendedSession(t)
	select {
	case _, ok := <-s.RawOutputCh:
		if !ok {
			t.Fatal("RawOutputCh was closed by the suspend — collectOutput would stop forever")
		}
		t.Fatal("unexpected data on RawOutputCh")
	default:
		// empty and open — exactly right
	}
}

// ---------------------------------------------------------------------------
// Resume.
// ---------------------------------------------------------------------------

func TestResume_ReArmsDoneAndSpawns(t *testing.T) {
	s := suspendedSession(t)
	// A suspended session's done channel is "exited" so Close() never blocks.
	rec := &spawnRecorder{}
	rec.install(s)

	genBefore := s.Generation()
	if err := s.Resume([]string{"claude", "--resume", "uuid"}, "C:\\repo", []string{"MULTITERMINAL_SESSION_ID=42"}); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if rec.calls != 1 {
		t.Fatalf("spawn calls = %d, want 1", rec.calls)
	}
	if rec.dir != "C:\\repo" || len(rec.argv) != 3 || len(rec.env) != 1 {
		t.Fatalf("Resume passed argv=%v dir=%q env=%v", rec.argv, rec.dir, rec.env)
	}
	if s.Generation() != genBefore+1 {
		t.Fatalf("generation = %d, want %d", s.Generation(), genBefore+1)
	}
	if s.IsSuspended() {
		t.Fatal("session must be running again after Resume")
	}
	if !s.IsRunning() {
		t.Fatal("status must be StatusRunning after Resume")
	}

	// done must be a fresh, OPEN channel — otherwise watchExit would instantly
	// report the resumed pane as exited.
	select {
	case <-s.Done():
		t.Fatal("Resume must arm a fresh done channel, not reuse the closed one")
	default:
	}
}

func TestResume_RejectsSecondConcurrentCall(t *testing.T) {
	s := suspendedSession(t)
	rec := &spawnRecorder{}
	rec.install(s)

	if err := s.Resume(nil, "", nil); err != nil {
		t.Fatalf("first Resume: %v", err)
	}
	if err := s.Resume(nil, "", nil); !errors.Is(err, ErrNotSuspended) {
		t.Fatalf("second Resume error = %v, want ErrNotSuspended", err)
	}
	if rec.calls != 1 {
		t.Fatalf("spawn calls = %d, want exactly 1 (no double process)", rec.calls)
	}
}

func TestResume_NotSuspended(t *testing.T) {
	s := doneSession(t)
	if err := s.Resume(nil, "", nil); !errors.Is(err, ErrNotSuspended) {
		t.Fatalf("Resume on a running session = %v, want ErrNotSuspended", err)
	}
}

func TestResume_SpawnFailureStaysSuspended(t *testing.T) {
	s := suspendedSession(t)
	rec := &spawnRecorder{err: errors.New("boom")}
	rec.install(s)

	if err := s.Resume(nil, "", nil); err == nil {
		t.Fatal("Resume must surface the spawn error")
	}
	if !s.IsSuspended() {
		t.Fatal("a failed Resume must leave the session suspended, not half-started")
	}
	// done must be closed again so Close() cannot block on a generation that
	// never came to life.
	select {
	case <-s.Done():
	default:
		t.Fatal("failed Resume left an open done channel")
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked after a failed Resume")
	}
}

// ---------------------------------------------------------------------------
// Close semantics.
// ---------------------------------------------------------------------------

func TestClose_OnSuspendedSessionDoesNotBlock(t *testing.T) {
	s := suspendedSession(t)
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close on a suspended session must not block")
	}
	select {
	case _, ok := <-s.RawOutputCh:
		if ok {
			t.Fatal("unexpected data")
		}
	default:
		t.Fatal("Close must close RawOutputCh so collectOutput terminates")
	}
}

func TestClose_TwiceDoesNotPanic(t *testing.T) {
	s := suspendedSession(t)
	s.Close()
	s.Close() // must not panic on a double close of RawOutputCh
}

func TestClose_ReleasesWaitAwake(t *testing.T) {
	s := suspendedSession(t)
	result := make(chan bool, 1)
	go func() { result <- s.WaitAwake() }()
	time.Sleep(20 * time.Millisecond)
	s.Close()
	select {
	case awake := <-result:
		if awake {
			t.Fatal("WaitAwake must report false when the session was closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not release WaitAwake — watchExit would leak")
	}
}

func TestWaitAwake_ReturnsTrueOnResume(t *testing.T) {
	s := suspendedSession(t)
	rec := &spawnRecorder{}
	rec.install(s)

	result := make(chan bool, 1)
	go func() { result <- s.WaitAwake() }()
	time.Sleep(20 * time.Millisecond)
	if err := s.Resume(nil, "", nil); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	select {
	case awake := <-result:
		if !awake {
			t.Fatal("WaitAwake must report true after a resume")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Resume did not release WaitAwake")
	}
}

func TestWaitAwake_ReturnsImmediatelyWhenRunning(t *testing.T) {
	s := doneSession(t)
	result := make(chan bool, 1)
	go func() { result <- s.WaitAwake() }()
	select {
	case awake := <-result:
		if !awake {
			t.Fatal("a running session is awake")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitAwake blocked on a running session")
	}
}

// ---------------------------------------------------------------------------
// Write on a suspended session.
// ---------------------------------------------------------------------------

func TestWrite_OnSuspendedSessionReportsErrSuspended(t *testing.T) {
	s := suspendedSession(t)
	n, err := s.Write([]byte("hallo"))
	if n != 0 {
		t.Fatalf("wrote %d bytes into a suspended session", n)
	}
	if !errors.Is(err, ErrSuspended) {
		t.Fatalf("error = %v, want ErrSuspended (silent io.ErrClosedPipe loses input)", err)
	}
}

func TestResumeID_RoundTrip(t *testing.T) {
	s := NewSession(1, 24, 80)
	if got := s.ResumeID(); got != "" {
		t.Fatalf("fresh session resume id = %q, want empty", got)
	}
	s.SetResumeID("abc")
	if got := s.ResumeID(); got != "abc" {
		t.Fatalf("resume id = %q, want abc", got)
	}
}
