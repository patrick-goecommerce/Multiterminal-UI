//go:build windows

package backend

import (
	"log"
	"os/exec"
	"strconv"
)

// killProcessTree force-kills pid and all descendants. Session.Close() only
// kills the cmd.exe wrapper — node/MCP/watcher grandchildren survive and hold
// handles inside the worktree, which makes `git worktree remove` fail on
// Windows (spec 5.2). Non-zero exit (tree already gone, taskkill exit 128)
// is tolerated.
func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	hideConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[killProcessTree] pid %d: %v – %s (tolerated)", pid, err, out)
	}
}
