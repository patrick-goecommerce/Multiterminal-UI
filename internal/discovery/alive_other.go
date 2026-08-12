//go:build !windows

package discovery

import (
	"os"
	"syscall"
)

// processAlive reports whether pid still identifies a running process.
//
// os.FindProcess never fails on Unix, so the liveness question is answered by
// the null signal: it performs the permission and existence checks without
// delivering anything.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
