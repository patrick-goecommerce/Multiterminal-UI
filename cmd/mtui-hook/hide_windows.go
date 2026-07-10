//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideConsole sets CREATE_NO_WINDOW and HideWindow on the process so that
// neither the process itself nor any console window it allocates is visible.
// Duplicated from internal/backend/hide_windows.go: this binary must not
// import internal/backend (would pull in the Wails dependency tree into a
// binary that is meant to stay tiny and GUI-subsystem/console-free).
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
	cmd.SysProcAttr.HideWindow = true
}
