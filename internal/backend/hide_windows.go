//go:build windows

package backend

import (
	"os/exec"
	"syscall"
)

// hideConsole sets CREATE_NO_WINDOW and HideWindow on the process so that
// neither the process itself nor any console window it allocates is visible.
// CREATE_NO_WINDOW prevents a new console from being created.
// HideWindow ensures the STARTUPINFO wShowWindow hint is HIDE, which stops
// git child helpers (fsmonitor, GCM credential helpers) from flashing a window
// even when they are launched as console-subsystem programs.
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
	cmd.SysProcAttr.HideWindow = true
}
