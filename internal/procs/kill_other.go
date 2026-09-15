//go:build !windows

package procs

import "syscall"

// KillProcessTree best-effort kill on Unix. ConPTY-style orphan handles are a
// Windows-only failure mode; a plain SIGKILL suffices here.
func KillProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
