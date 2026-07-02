//go:build windows

package backend

import (
	"os/exec"
	"testing"
	"time"
)

func TestKillProcessTree_KillsChildAndToleratesDead(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping -n 30 127.0.0.1 >NUL")
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	killProcessTree(pid)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done: // killed → Wait returns promptly
	case <-time.After(5 * time.Second):
		t.Fatal("process tree not killed within 5s")
	}
	killProcessTree(pid) // second call on dead tree must not panic/hang
}
