//go:build windows

package procs

import (
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// waitGone reports whether pid exits within d.
func waitGone(t *testing.T, pid uint32, d time.Duration) bool {
	t.Helper()
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return true // already gone
	}
	defer windows.CloseHandle(h)
	ev, _ := windows.WaitForSingleObject(h, uint32(d.Milliseconds()))
	return ev == windows.WAIT_OBJECT_0
}

// startInJob starts cmd.exe with the given command line and puts it in a new
// job before it gets to start anything.
func startInJob(t *testing.T, line string) (*Job, *exec.Cmd) {
	t.Helper()
	j := NewJob()
	if j == nil {
		t.Fatal("NewJob returned nil")
	}
	cmd := exec.Command("cmd.exe", "/c", line)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	j.Assign(cmd.Process.Pid)
	return j, cmd
}

// A process whose parent has exited is out of reach for taskkill /T. The job
// still holds it, and Release must end it, since it has no window.
func TestJobReleaseEndsOrphanedConsoleChild(t *testing.T) {
	// The ping in front delays the orphan's start past Assign; start /b then
	// launches it and cmd.exe exits, cutting the parent link.
	j, cmd := startInJob(t, "ping -n 2 127.0.0.1 >nul & start /b ping -n 60 127.0.0.1 >nul")
	root := uint32(cmd.Process.Pid)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("cmd.exe: %v", err)
	}

	var orphan uint32
	deadline := time.Now().Add(10 * time.Second)
	for orphan == 0 && time.Now().Before(deadline) {
		for _, pid := range j.pids() {
			if pid != root {
				orphan = pid
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if orphan == 0 {
		j.Release()
		t.Fatal("the orphaned ping never showed up in the job")
	}

	j.Release()
	if !waitGone(t, orphan, 5*time.Second) {
		terminate(orphan)
		t.Fatal("Release left the orphaned console child running")
	}
}

// If the owner dies, the kernel closes the job handle, and every process in
// the job must end with it. Closing the handle directly is the same event.
func TestJobHandleCloseEndsTree(t *testing.T) {
	j, cmd := startInJob(t, "ping -n 60 127.0.0.1 >nul")
	pid := uint32(cmd.Process.Pid)
	go func() { _ = cmd.Wait() }()

	_ = windows.CloseHandle(j.h)
	if !waitGone(t, pid, 5*time.Second) {
		terminate(pid)
		t.Fatal("closing the armed job left its process running")
	}
}

func TestNilJobIsSafe(t *testing.T) {
	var j *Job
	j.Assign(123)
	j.Release()
}
