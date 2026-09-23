//go:build !windows

package procs

import "os/exec"

// HideConsole is a no-op on non-Windows platforms.
func HideConsole(_ *exec.Cmd) {}
