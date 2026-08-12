//go:build windows

package terminal

import (
	"encoding/csv"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// imageName returns the executable name of pid via tasklist, or "" when the
// process is gone.
func imageName(t *testing.T, pid int) string {
	t.Helper()
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		t.Fatalf("tasklist: %v", err)
	}
	text := strings.TrimSpace(string(out))
	// When nothing matches, tasklist prints a *localized* info line ("INFO:" /
	// "INFORMATION:" / …). A real record is always a quoted CSV row, so key off
	// that instead of the message text.
	if !strings.HasPrefix(text, `"`) {
		return ""
	}
	rec, err := csv.NewReader(strings.NewReader(text)).Read()
	if err != nil || len(rec) == 0 {
		return ""
	}
	return rec[0]
}

// waitForScreen polls the session screen until it contains want.
func waitForScreen(t *testing.T, s *Session, want string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.Screen.PlainText(), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("screen never contained %q; got:\n%s", want, s.Screen.PlainText())
}

// TestStart_DirectExeHasNoCmdWrapper is the E2E counterpart to TestWindowsArgv:
// a real .exe on PATH must become the session's own process, not a grandchild
// of a cmd.exe wrapper.
func TestStart_DirectExeHasNoCmdWrapper(t *testing.T) {
	s := NewSession(1, 24, 80)
	// ping keeps running long enough to inspect the process, and prints output.
	if err := s.Start([]string{"ping", "-n", "20", "127.0.0.1"}, t.TempDir(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	pid := s.Pid()
	if pid == 0 {
		t.Fatal("Pid() = 0 after Start")
	}
	if got := imageName(t, pid); !strings.EqualFold(got, "PING.EXE") {
		t.Errorf("session process = %q, want PING.EXE (a cmd.exe here means the wrapper is still applied)", got)
	}
	waitForScreen(t, s, "127.0.0.1")
}

// TestStart_CmdShimStillUsesComspec guards the reason the wrapper exists:
// ConPTY cannot launch a .cmd shim directly, so those must keep going through
// cmd.exe and must still actually run.
func TestStart_CmdShimStillUsesComspec(t *testing.T) {
	dir := t.TempDir()
	shim := filepath.Join(dir, "mtuishim.cmd")
	script := "@echo off\r\necho MTUI_SHIM_OK\r\nping -n 20 127.0.0.1 >NUL\r\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewSession(2, 24, 80)
	// The PATH entry passed here wins over the inherited one (go-pty dedups
	// case-insensitively, last occurrence wins) — the same rule envValue uses.
	env := []string{"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")}
	if err := s.Start([]string{"mtuishim"}, dir, env); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	if got := imageName(t, s.Pid()); !strings.EqualFold(got, "cmd.exe") {
		t.Errorf("session process = %q, want cmd.exe for a .cmd shim", got)
	}
	waitForScreen(t, s, "MTUI_SHIM_OK")
}

// TestStart_DirectExeTreeIsKillable verifies the kill contract for the
// wrapper-less case: Pid() is the root of the tree, so taskkill /T (what
// backend.killProcessTree runs) still reaches the descendants.
func TestStart_DirectExeTreeIsKillable(t *testing.T) {
	s := NewSession(3, 24, 80)
	// cmd.exe is itself a .exe, so it is started directly (no second cmd.exe)
	// and its ping child is the descendant taskkill /T has to reach.
	if err := s.Start([]string{"cmd.exe", "/c", "ping -n 60 127.0.0.1"}, t.TempDir(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForScreen(t, s, "127.0.0.1")

	rootPid := s.Pid()
	childPid := childProcessOf(t, rootPid)
	if childPid == 0 {
		t.Fatal("no child process found below the session process")
	}

	kill := exec.Command("taskkill", "/PID", strconv.Itoa(rootPid), "/T", "/F")
	if out, err := kill.CombinedOutput(); err != nil {
		t.Fatalf("taskkill: %v – %s", err, out)
	}
	s.Close()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if imageName(t, childPid) == "" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("child pid %d survived taskkill /T on the session pid", childPid)
}

// childProcessOf returns the first process whose parent is pid.
func childProcessOf(t *testing.T, pid int) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	query := "ParentProcessId=" + strconv.Itoa(pid)
	for time.Now().Before(deadline) {
		out, err := exec.Command("powershell.exe", "-NoProfile", "-Command",
			"(Get-CimInstance Win32_Process -Filter '"+query+"' | Select-Object -First 1).ProcessId").Output()
		if err == nil {
			if v, convErr := strconv.Atoi(strings.TrimSpace(string(out))); convErr == nil && v > 0 {
				return v
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 0
}
