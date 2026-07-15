//go:build !windows

package backend

import "syscall"

// killProcessTree best-effort kill on Unix. ConPTY-style orphan handles are a
// Windows-only failure mode; a plain SIGKILL suffices here.
func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
