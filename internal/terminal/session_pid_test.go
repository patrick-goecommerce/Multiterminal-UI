package terminal

import (
	"runtime"
	"testing"
	"time"
)

func TestPid_ZeroBeforeStart(t *testing.T) {
	s := NewSession(1, 24, 80)
	if got := s.Pid(); got != 0 {
		t.Errorf("Pid before Start = %d, want 0", got)
	}
}

// Once the process has been waited on its PID is free for the OS to reuse.
// A tree kill by the old number (Embedded.Close runs one before closing) would
// then end whatever process got it next.
func TestPid_ZeroOnceTheProcessHasExited(t *testing.T) {
	s := NewSession(1, 24, 80)
	argv := []string{"sh", "-c", "exit 0"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/c", "exit 0"}
	}
	if err := s.Start(argv, t.TempDir(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Close)

	select {
	case <-s.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("process did not exit")
	}
	if got := s.Pid(); got != 0 {
		t.Errorf("Pid after exit = %d, want 0", got)
	}
}

func TestPid_ReportedWhileRunning(t *testing.T) {
	s := NewSession(1, 24, 80)
	argv := []string{"sh", "-c", "sleep 30"}
	if runtime.GOOS == "windows" {
		argv = []string{"ping", "-n", "30", "127.0.0.1"}
	}
	if err := s.Start(argv, t.TempDir(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Close)
	if got := s.Pid(); got <= 0 {
		t.Errorf("Pid while running = %d, want the child's PID", got)
	}
}
