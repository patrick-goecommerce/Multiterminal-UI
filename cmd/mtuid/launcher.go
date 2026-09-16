package main

import (
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
)

// daemonLauncher builds the argv and the environment for a session the daemon
// is asked to start by name.
//
// This is what lets a client with no configuration of its own (the CLI, an
// agent over MCP) create a session at all. Before it, only the window could,
// because only the window knew what "claude" meant and which variables a pane
// needs; a daemon that could hold sessions but never make one is half a
// daemon.
//
// It reads the configuration per launch rather than once at startup. The
// config file is a few kilobytes of YAML and a session launch already spends
// one or two git subprocesses, so the cost is nothing; what it buys is that a
// setting the user changes in the running app applies to the next session the
// daemon starts, without restarting a daemon that is holding live agents.
type daemonLauncher struct {
	// shimPort is read at launch time, not stored: the host serves the shim
	// endpoints, so the port only exists once the host does.
	shimPort func() int
}

func (l daemonLauncher) policy() launch.Policy {
	cfg := config.Load()
	return launch.Policy{
		ShimPort:       l.shimPort(),
		ForceWorktrees: cfg.ShouldForceWorktrees(),
		Commands: launch.Commands{
			"claude": cfg.ClaudeCommand,
			"codex":  cfg.CodexCommand,
			"gemini": cfg.GeminiCommand,
		},
	}
}

func (l daemonLauncher) Argv(tool, model string) ([]string, error) {
	return l.policy().Argv(tool, model)
}

func (l daemonLauncher) Env(sessionID int, dir, mode string) []string {
	return l.policy().Env(sessionID, dir, mode)
}

var _ hub.Launcher = daemonLauncher{}
