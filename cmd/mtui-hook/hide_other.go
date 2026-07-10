//go:build !windows

package main

import "os/exec"

// hideConsole is a no-op on non-Windows platforms.
func hideConsole(_ *exec.Cmd) {}
