package terminal

import "testing"

func TestPid_ZeroBeforeStart(t *testing.T) {
	s := NewSession(1, 24, 80)
	if got := s.Pid(); got != 0 {
		t.Errorf("Pid before Start = %d, want 0", got)
	}
}
