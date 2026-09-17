package backend

import (
	"os/exec"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/procs"
)

// hideConsole and killProcessTree delegate to internal/procs, which the daemon
// (cmd/mtuid) uses as well. The short names stay because every spawn in this
// package calls them, and one implementation is the point: a second copy is
// how the console-window flash came back twice already (see CLAUDE.md).

func hideConsole(cmd *exec.Cmd) { procs.HideConsole(cmd) }

func killProcessTree(pid int) { procs.KillProcessTree(pid) }
