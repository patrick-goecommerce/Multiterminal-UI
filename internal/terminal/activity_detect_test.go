package terminal

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// DetectActivity – time-based activity state transitions
// These tests drive the actual "window blink" behavior:
//   - Green glow  → ActivityDone (prompt returned after >1.5s silence)
//   - Yellow pulse → ActivityWaitingAnswer (Y/n prompt after >1.5s silence)
//   - No change   → while output is recent (<1.5s) or never happened
// ---------------------------------------------------------------------------

func TestDetectActivity_NoOutputYet(t *testing.T) {
	sess := NewSession(1, 5, 80)
	// LastOutputAt is zero — no output has ever been received.
	// DetectActivity should return current activity (default: ActivityIdle).
	state := sess.DetectActivity()
	if state != ActivityIdle {
		t.Errorf("DetectActivity with no output = %d, want ActivityIdle (%d)", state, ActivityIdle)
	}
}

func TestDetectActivity_RecentOutput_StaysActive(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("$ ")) // prompt on screen

	// Simulate recent output (just now)
	sess.mu.Lock()
	sess.LastOutputAt = time.Now()
	sess.Activity = ActivityActive
	sess.mu.Unlock()

	state := sess.DetectActivity()
	// Output was < 1.5s ago → should keep current state (Active), NOT classify screen
	if state != ActivityActive {
		t.Errorf("DetectActivity with recent output = %d, want ActivityActive (%d)", state, ActivityActive)
	}
}

func TestDetectActivity_RecentOutput_TransitionsDoneToActive(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("$ ")) // prompt on screen

	// Simulate: Activity was Done (previous command finished), but new output
	// just arrived (pipeline queue sent next prompt → PTY echo).
	// This is the critical case for queue advancement.
	sess.mu.Lock()
	sess.LastOutputAt = time.Now()
	sess.Activity = ActivityDone
	sess.mu.Unlock()

	state := sess.DetectActivity()
	// Output is flowing → must transition to Active regardless of previous state
	if state != ActivityActive {
		t.Errorf("DetectActivity with Done + recent output = %d, want ActivityActive (%d)", state, ActivityActive)
	}

	// Verify the session's Activity field was also updated
	sess.mu.Lock()
	stored := sess.Activity
	sess.mu.Unlock()
	if stored != ActivityActive {
		t.Errorf("Session.Activity after transition = %d, want ActivityActive (%d)", stored, ActivityActive)
	}
}

func TestDetectActivity_RecentOutput_TransitionsIdleToActive(t *testing.T) {
	sess := NewSession(1, 5, 80)

	// Simulate: Activity was Idle, new output arrives
	sess.mu.Lock()
	sess.LastOutputAt = time.Now()
	sess.Activity = ActivityIdle
	sess.mu.Unlock()

	state := sess.DetectActivity()
	if state != ActivityActive {
		t.Errorf("DetectActivity with Idle + recent output = %d, want ActivityActive (%d)", state, ActivityActive)
	}
}

func TestDetectActivity_StaleOutput_ClassifiesDone(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("$ ")) // prompt on screen → should classify as Done

	// Simulate output that stopped 2 seconds ago
	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.Activity = ActivityActive
	sess.mu.Unlock()

	state := sess.DetectActivity()
	// Output was > 1.5s ago → should classify screen → sees "$ " → Done
	if state != ActivityDone {
		t.Errorf("DetectActivity with stale output + prompt = %d, want ActivityDone (%d)", state, ActivityDone)
	}

	// Verify the session's Activity field was also updated
	sess.mu.Lock()
	stored := sess.Activity
	sess.mu.Unlock()
	if stored != ActivityDone {
		t.Errorf("Session.Activity after DetectActivity = %d, want ActivityDone (%d)", stored, ActivityDone)
	}
}

func TestDetectActivity_StaleOutput_ClassifiesWaitingAnswer(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("Allow access? [Y/n] "))

	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.Activity = ActivityActive
	sess.mu.Unlock()

	state := sess.DetectActivity()
	// Should see "[Y/n]" → WaitingAnswer (yellow pulse)
	if state != ActivityWaitingAnswer {
		t.Errorf("DetectActivity with stale output + Y/n = %d, want ActivityWaitingAnswer (%d)", state, ActivityWaitingAnswer)
	}
}

func TestDetectActivity_StaleOutput_ClassifiesIdle(t *testing.T) {
	sess := NewSession(1, 5, 80)
	sess.Screen.Write([]byte("building project...\r\n"))

	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.Activity = ActivityActive
	sess.mu.Unlock()

	state := sess.DetectActivity()
	// No prompt or Y/n → Idle
	if state != ActivityIdle {
		t.Errorf("DetectActivity with stale output + no prompt = %d, want ActivityIdle (%d)", state, ActivityIdle)
	}
}

// ---------------------------------------------------------------------------
// Full activity lifecycle: Active → Done → Reset → Idle
// This simulates the complete "green blink" cycle.
// ---------------------------------------------------------------------------

func TestActivityLifecycle_ActiveToDoneToReset(t *testing.T) {
	sess := NewSession(1, 5, 80)

	// Step 1: Output starts arriving — session becomes Active
	sess.mu.Lock()
	sess.Activity = ActivityActive
	sess.LastOutputAt = time.Now()
	sess.mu.Unlock()

	// Step 2: While output is recent, DetectActivity preserves Active
	state := sess.DetectActivity()
	if state != ActivityActive {
		t.Errorf("Step 2: state = %d, want ActivityActive (%d)", state, ActivityActive)
	}

	// Step 3: Output stops, prompt appears, 2s passes
	sess.Screen.Write([]byte("$ "))
	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.mu.Unlock()

	state = sess.DetectActivity()
	if state != ActivityDone {
		t.Errorf("Step 3: state = %d, want ActivityDone (%d) — green glow should trigger", state, ActivityDone)
	}

	// Step 4: Frontend acknowledges → ResetActivity back to Idle
	sess.ResetActivity()
	sess.mu.Lock()
	stored := sess.Activity
	sess.mu.Unlock()
	if stored != ActivityIdle {
		t.Errorf("Step 4: after ResetActivity, state = %d, want ActivityIdle (%d)", stored, ActivityIdle)
	}
}

// ---------------------------------------------------------------------------
// Full activity lifecycle: Active → WaitingAnswer → Reset → Idle
// This simulates the "yellow pulse" cycle.
// ---------------------------------------------------------------------------

func TestActivityLifecycle_ActiveToWaitingAnswerToReset(t *testing.T) {
	sess := NewSession(1, 5, 80)

	// Step 1: Claude is generating output
	sess.mu.Lock()
	sess.Activity = ActivityActive
	sess.LastOutputAt = time.Now()
	sess.mu.Unlock()

	// Step 2: Output stops with a confirmation prompt, 2s passes
	sess.Screen.Write([]byte("Delete all files? [Y/n] "))
	sess.mu.Lock()
	sess.LastOutputAt = time.Now().Add(-2 * time.Second)
	sess.mu.Unlock()

	state := sess.DetectActivity()
	if state != ActivityWaitingAnswer {
		t.Errorf("Step 2: state = %d, want ActivityWaitingAnswer (%d) — yellow pulse should trigger", state, ActivityWaitingAnswer)
	}

	// Step 3: User answers → reset
	sess.ResetActivity()
	sess.mu.Lock()
	stored := sess.Activity
	sess.mu.Unlock()
	if stored != ActivityIdle {
		t.Errorf("Step 3: after ResetActivity, state = %d, want ActivityIdle (%d)", stored, ActivityIdle)
	}
}

// ---------------------------------------------------------------------------
// ResetActivity
// ---------------------------------------------------------------------------

func TestResetActivity(t *testing.T) {
	sess := NewSession(1, 5, 80)

	sess.mu.Lock()
	sess.Activity = ActivityDone
	sess.mu.Unlock()

	sess.ResetActivity()

	sess.mu.Lock()
	state := sess.Activity
	sess.mu.Unlock()
	if state != ActivityIdle {
		t.Errorf("After ResetActivity, state = %d, want ActivityIdle (%d)", state, ActivityIdle)
	}
}

func TestResetActivity_FromWaitingAnswer(t *testing.T) {
	sess := NewSession(1, 5, 80)

	sess.mu.Lock()
	sess.Activity = ActivityWaitingAnswer
	sess.mu.Unlock()

	sess.ResetActivity()

	sess.mu.Lock()
	state := sess.Activity
	sess.mu.Unlock()
	if state != ActivityIdle {
		t.Errorf("After ResetActivity from WaitingAnswer, state = %d, want ActivityIdle (%d)", state, ActivityIdle)
	}
}
