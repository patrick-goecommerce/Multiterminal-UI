//go:build !windows

package procs

import (
	"os/exec"
	"syscall"
)

// Detach makes a child outlive its parent.
//
// A new session detaches it from the parent's controlling terminal and process
// group, so a signal sent to the parent's group (or the parent going away) does
// not take the child with it. That is the whole point for the session daemon:
// closing the GUI must not end the agents it started.
func Detach(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
