//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideChildWindow sets CREATE_NO_WINDOW so the wrapped statusline command
// (powershell) does not flash a console window each time Claude renders its
// status line. Mirrors internal/backend.hideConsole. Safe for stdout-captured
// children — it only suppresses console-window creation, not the stdio pipes.
func hideChildWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
