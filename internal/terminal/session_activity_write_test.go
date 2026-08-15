package terminal

import "testing"

// A cosmetic PTY chunk must not change the activity state. The read path used
// to set ActivityActive on every byte, which made a repaint of Claude's idle
// TUI look like fresh work (issue #188).
func TestNoteOutputDoesNotTouchActivity(t *testing.T) {
	s := NewSession(1, 24, 80)
	s.Activity = ActivityDone

	if ok := s.noteOutput(s.sus.gen); !ok {
		t.Fatal("noteOutput reported a stale generation for a fresh session")
	}

	if got := s.GetActivity(); got != ActivityDone {
		t.Errorf("Activity = %v after a cosmetic chunk, want %v (unchanged)", got, ActivityDone)
	}
}

// A hook-set state is authoritative and must survive later PTY bytes.
func TestNoteOutputKeepsHookState(t *testing.T) {
	s := NewSession(2, 24, 80)
	s.SetHookActivity(ActivityDone)

	s.noteOutput(s.sus.gen)

	if got := s.GetActivity(); got != ActivityDone {
		t.Errorf("hook state was overwritten: Activity = %v, want %v", got, ActivityDone)
	}
	if !s.HasHookData() {
		t.Error("HasHookData() = false, want true")
	}
}

// The bookkeeping noteOutput is actually responsible for must keep working.
func TestNoteOutputStillRecordsOutput(t *testing.T) {
	s := NewSession(3, 24, 80)
	s.Screen.Write([]byte("\x1b]0;my-title\x07"))

	s.noteOutput(s.sus.gen)

	if s.GetLastOutputAt().IsZero() {
		t.Error("LastOutputAt was not updated")
	}
	if s.Name() != "my-title" {
		t.Errorf("Title = %q, want %q", s.Name(), "my-title")
	}
}
