//go:build windows

package procs

import (
	"os/exec"
	"syscall"
)

// Windows creation flags, from processthreadsapi.h.
const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// Detach makes a child outlive its parent.
//
// DETACHED_PROCESS gives it no console at all, which is also what keeps it from
// inheriting the parent's, and a new process group means a Ctrl-Break aimed at
// the parent's group does not reach it. Neither is combined with
// CREATE_NO_WINDOW: that flag asks for a new console that is merely hidden,
// which is the opposite of what a detached background service wants, and mtuid
// is linked as a GUI-subsystem binary anyway.
func Detach(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= detachedProcess | createNewProcessGroup
	cmd.SysProcAttr.HideWindow = true
}
