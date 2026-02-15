package terminal

import "testing"

// ---------------------------------------------------------------------------
// NewSession – construction tests (no PTY needed)
// ---------------------------------------------------------------------------

func TestNewSession_CreatesScreen(t *testing.T) {
	sess := NewSession(42, 24, 80)

	if sess.ID != 42 {
		t.Fatalf("expected ID 42, got %d", sess.ID)
	}
	if sess.Screen == nil {
		t.Fatal("Screen should not be nil")
	}
	if sess.Screen.Rows() != 24 || sess.Screen.Cols() != 80 {
		t.Fatalf("expected 24x80 screen, got %dx%d", sess.Screen.Rows(), sess.Screen.Cols())
	}
}

func TestNewSession_StatusRunning(t *testing.T) {
	sess := NewSession(1, 10, 40)

	if sess.Status != StatusRunning {
		t.Fatalf("expected StatusRunning, got %d", sess.Status)
	}
}

func TestNewSession_IsRunning(t *testing.T) {
	sess := NewSession(1, 10, 40)

	if !sess.IsRunning() {
		t.Fatal("new session should be running")
	}
}

func TestNewSession_ChannelsCreated(t *testing.T) {
	sess := NewSession(1, 10, 40)

	if sess.OutputCh == nil {
		t.Fatal("OutputCh should not be nil")
	}
	if sess.RawOutputCh == nil {
		t.Fatal("RawOutputCh should not be nil")
	}
	if sess.done == nil {
		t.Fatal("done channel should not be nil")
	}
}

func TestNewSession_DoneChannelOpen(t *testing.T) {
	sess := NewSession(1, 10, 40)

	select {
	case <-sess.Done():
		t.Fatal("done channel should not be closed on new session")
	default:
		// expected
	}
}

func TestNewSession_TokensZero(t *testing.T) {
	sess := NewSession(1, 10, 40)
	tokens := sess.GetTokens()

	if tokens.TotalCost != 0 {
		t.Fatalf("expected zero cost, got %f", tokens.TotalCost)
	}
	if tokens.InputTokens != 0 || tokens.OutputTokens != 0 {
		t.Fatalf("expected zero tokens, got input=%d output=%d", tokens.InputTokens, tokens.OutputTokens)
	}
}

func TestNewSession_ActivityIdle(t *testing.T) {
	sess := NewSession(1, 10, 40)

	if sess.Activity != ActivityIdle {
		t.Fatalf("expected ActivityIdle, got %d", sess.Activity)
	}
}

// ---------------------------------------------------------------------------
// Write without PTY returns error
// ---------------------------------------------------------------------------

func TestSession_WriteWithoutPTY(t *testing.T) {
	sess := NewSession(1, 10, 40)
	_, err := sess.Write([]byte("hello"))
	if err == nil {
		t.Fatal("Write without PTY should return error")
	}
}

// ---------------------------------------------------------------------------
// Resize without PTY doesn't panic
// ---------------------------------------------------------------------------

func TestSession_ResizeWithoutPTY(t *testing.T) {
	sess := NewSession(1, 10, 40)

	// Should not panic
	sess.Resize(50, 120)

	if sess.Screen.Rows() != 50 || sess.Screen.Cols() != 120 {
		t.Fatalf("expected 50x120, got %dx%d", sess.Screen.Rows(), sess.Screen.Cols())
	}
}

// ---------------------------------------------------------------------------
// EnableKittyKeyboard / DisableKittyKeyboard without PTY don't panic
// ---------------------------------------------------------------------------

func TestSession_KittyKeyboardWithoutPTY(t *testing.T) {
	sess := NewSession(1, 10, 40)

	// Should not panic even without PTY
	sess.EnableKittyKeyboard()
	sess.DisableKittyKeyboard()
}

// ---------------------------------------------------------------------------
// defaultShell returns a valid value
// ---------------------------------------------------------------------------

func TestDefaultShell_ReturnsNonEmpty(t *testing.T) {
	result := defaultShell()
	if len(result) == 0 {
		t.Fatal("defaultShell should return at least one element")
	}
	if result[0] == "" {
		t.Fatal("shell path should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Multiple NewSession with different IDs
// ---------------------------------------------------------------------------

func TestNewSession_UniqueIDs(t *testing.T) {
	s1 := NewSession(1, 10, 40)
	s2 := NewSession(2, 10, 40)
	s3 := NewSession(99, 10, 40)

	if s1.ID == s2.ID || s2.ID == s3.ID {
		t.Fatal("sessions should have unique IDs")
	}
}

// ---------------------------------------------------------------------------
// Session screen dimensions
// ---------------------------------------------------------------------------

func TestNewSession_SmallScreen(t *testing.T) {
	sess := NewSession(1, 1, 1)
	if sess.Screen.Rows() != 1 || sess.Screen.Cols() != 1 {
		t.Fatalf("expected 1x1 screen, got %dx%d", sess.Screen.Rows(), sess.Screen.Cols())
	}
}

func TestNewSession_LargeScreen(t *testing.T) {
	sess := NewSession(1, 100, 300)
	if sess.Screen.Rows() != 100 || sess.Screen.Cols() != 300 {
		t.Fatalf("expected 100x300, got %dx%d", sess.Screen.Rows(), sess.Screen.Cols())
	}
}
