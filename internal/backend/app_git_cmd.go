package backend

import (
	"os"
	"os/exec"
)

// gitCmd builds an *exec.Cmd for git that never flashes a console window:
// HideWindow+CREATE_NO_WINDOW via hideConsole, plus global flags/env that stop
// git from spawning visible child helper processes (fsmonitor, optional locks).
//
// The suppression flags --no-optional-locks and -c core.fsmonitor=false are
// inserted as global git options before the subcommand. They do not affect
// command output.
func gitCmd(dir string, args ...string) *exec.Cmd {
	full := append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	hideConsole(cmd)
	return cmd
}
