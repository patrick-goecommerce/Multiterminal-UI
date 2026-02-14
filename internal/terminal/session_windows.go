//go:build windows

package terminal

import (
	"syscall"

	gopty "github.com/aymanbagabas/go-pty"
)

// hidePTYConsole sets CREATE_NO_WINDOW on the process creation flags so
// that child processes spawned via ConPTY do not flash a visible console
// window. This is only needed when the host app is a GUI app.
func hidePTYConsole(cmd *gopty.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
