package main

import (
	"time"

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

func (l daemonLauncher) ResumeArgv(argv []string, resumeID string) []string {
	// No config is involved in rewriting a command line, so this does not pay
	// for a config read the way Argv and Env do.
	return launch.ResumeArgv(argv, resumeID)
}

var _ hub.Launcher = daemonLauncher{}

// keepAlivePolicy reads the keep-alive settings for one tick.
//
// Per tick and not once at startup, for the same reason Argv and Env are: the
// user changes this in the settings dialog of an app that may well be running
// against a daemon holding live agents, and restarting the daemon to pick it
// up would end them.
func keepAlivePolicy() hub.KeepAlive {
	cfg := config.Load()
	if !cfg.ShouldKeepAlive() || cfg.KeepAlive.IntervalMinutes <= 0 {
		return hub.KeepAlive{}
	}
	return hub.KeepAlive{
		Every:   time.Duration(cfg.KeepAlive.IntervalMinutes) * time.Minute,
		Message: cfg.KeepAlive.Message,
		Modes:   launch.KeepAliveModes(),
	}
}
