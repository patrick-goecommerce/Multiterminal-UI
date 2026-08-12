//go:build windows

package backend

import (
	"log"
	"os/exec"
	"strconv"
)

// killProcessTree force-kills pid and all descendants. Session.Close() only
// kills the root process (the cmd.exe wrapper for a .cmd shim, the target
// binary itself for a real .exe — see terminal.windowsArgv). Either way its
// node/MCP/watcher descendants survive and hold handles inside the worktree,
// which makes `git worktree remove` fail on Windows (spec 5.2). taskkill /T
// walks the tree from pid regardless of its depth. Non-zero exit (tree already
// gone, taskkill exit 128) is tolerated.
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
