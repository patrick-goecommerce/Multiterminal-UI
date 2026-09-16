package backend

import (
	"os/exec"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/gitx"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
)

// The git plumbing and the session launch policy live in internal/gitx and
// internal/launch, so that the daemon can use them too: as long as they sat
// next to the Wails bindings, only a process with a window could work out what
// a session's environment has to contain, and that is what kept the MCP server
// and the CLI from ever creating one.
//
// These are the names the rest of this package already used. Keeping them
// means the move did not touch fifteen call sites, and a reader who follows
// one lands in the package that owns the answer.

// ProjectConfig is the per-project settings file. Aliased rather than
// redeclared: the frontend binding returns this type, and two identical
// structs would be one Wails deserialization away from silently diverging.
type ProjectConfig = launch.ProjectConfig

func gitCmd(dir string, args ...string) *exec.Cmd { return gitx.Cmd(dir, args...) }

func mainRepoRoot(dir string) (string, error) { return gitx.MainRepoRoot(dir) }

func gitToplevel(dir string) (string, error) { return gitx.Toplevel(dir) }

func parseWorktreePorcelain(output string) []WorktreeInfo {
	entries := gitx.ParseWorktreePorcelain(output)
	out := make([]WorktreeInfo, 0, len(entries))
	for _, e := range entries {
		out = append(out, WorktreeInfo{Path: e.Path, Branch: e.Branch})
	}
	return out
}

func loadProjectConfig(dir string) ProjectConfig { return launch.LoadProjectConfig(dir) }

func saveProjectConfig(dir string, cfg ProjectConfig) error {
	return launch.SaveProjectConfig(dir, cfg)
}

func isClaudeMode(mode string) bool { return launch.IsClaudeMode(mode) }

func worktreeEnvVars(dir string) []string { return launch.WorktreeEnvVars(dir) }

func resumeArgv(argv []string, resumeID string) []string {
	return launch.ResumeArgv(argv, resumeID)
}

func claudeSessionIDFromArgv(argv []string) string { return launch.SessionIDFromArgv(argv) }

// launchPolicy is the config this window would launch a session with. The
// shim port comes from the host rather than from this process, because a
// session outliving the window keeps reporting to whatever port it was told.
func (a *AppService) launchPolicy() launch.Policy {
	return launch.Policy{
		ShimPort:       a.GetTmuxAPIPort(),
		ForceWorktrees: a.cfg.ShouldForceWorktrees(),
		Commands:       a.launchCommands(),
	}
}

// windowLauncher lets this window's host start and wake a session by name,
// exactly as the daemon's does.
//
// The policy is read per call rather than captured once: the shim port only
// exists after the host does, and a setting the user changes in the settings
// dialog has to apply to the next session without restarting anything.
type windowLauncher struct{ app *AppService }

func (l windowLauncher) Argv(tool, model string) ([]string, error) {
	return l.app.launchPolicy().Argv(tool, model)
}

func (l windowLauncher) Env(sessionID int, dir, mode string) []string {
	return l.app.launchPolicy().Env(sessionID, dir, mode)
}

func (l windowLauncher) ResumeArgv(argv []string, resumeID string) []string {
	return launch.ResumeArgv(argv, resumeID)
}

var _ hub.Launcher = windowLauncher{}
