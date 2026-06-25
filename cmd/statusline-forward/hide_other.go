//go:build !windows

package main

import "os/exec"

// hideChildWindow is a no-op on non-Windows platforms (no console windows).
func hideChildWindow(_ *exec.Cmd) {}
