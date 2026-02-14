//go:build windows

package backend

import (
	"os/exec"
	"syscall"
)

// hideConsole sets CREATE_NO_WINDOW on the process so that child processes
// do not flash a visible console window when running as a GUI app.
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
