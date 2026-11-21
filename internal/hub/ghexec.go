package hub

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/procs"
)

// ghLimit bounds one gh call.
const ghLimit = 15 * time.Second

// ExecGH runs the GitHub CLI and returns its stdout. It never prompts (the
// host has no terminal to answer in), shows no console window and gives up
// after 15 s. The CI watcher only ever asks it to read.
func ExecGH(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, ghLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	// A child of gh still holding the pipe must not keep Output waiting past
	// the limit.
	cmd.WaitDelay = time.Second
	procs.HideConsole(cmd)
	return cmd.Output()
}
