package procs

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestKillTreeOnCancel_EndsCommandOnTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	// sh keeps the pipe; the grandchild sleep inherits it, which is exactly
	// the shape that used to make Wait hang past the timeout.
	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 30 & sleep 30")
	KillTreeOnCancel(cmd)

	start := time.Now()
	_, _ = cmd.Output()
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("Output returned after %v; the timeout did not end the command", took)
	}
}
