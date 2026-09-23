package procs

import (
	"os/exec"
	"time"
)

// TreeWaitDelay is how long Wait keeps waiting for a command's output pipes
// once its process has exited or its context is done.
//
// Without a delay Wait blocks until every process holding the pipe has
// exited. On Windows a command started as cmd.exe /c claude hands its pipe
// handles down to claude and to whatever claude starts, so one orphaned
// grandchild keeps Wait, the goroutine calling it and the whole tree alive.
const TreeWaitDelay = 2 * time.Second

// KillTreeOnCancel makes a cancelled or timed-out command end its whole
// process tree instead of only its direct child, and bounds how long Wait
// waits afterwards.
//
// exec.CommandContext on its own kills cmd.Process when the context is done.
// For a cmd.exe /c wrapper that is only the wrapper: claude, node and the MCP
// servers under it keep running, and the timeout that was meant to stop them
// does nothing. The tree is killed while the root is still alive, because
// taskkill /T walks parent links and cannot reach children whose parent is
// already gone.
//
// The command must have been created with exec.CommandContext.
func KillTreeOnCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		KillProcessTree(cmd.Process.Pid)
		// taskkill can fail (it is a process too); the root must die anyway.
		_ = cmd.Process.Kill()
		return nil
	}
	cmd.WaitDelay = TreeWaitDelay
}
