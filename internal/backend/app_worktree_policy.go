// Worktree-mandatory policy: resolves the global config.ForceWorktrees setting
// against the per-project override in .mtui/config.json, and exposes both to
// the frontend. The resolved value is turned into the
// MULTITERMINAL_FORCE_WORKTREE_ROOT env var at session launch (see
// createSession) — the mtui-hook binary reads only that env var and never the
// config, so the policy is evaluated exactly once per pane.
package backend

import (
	"fmt"
	"log"
)

// Project override modes exchanged with the frontend. A plain string is used
// instead of a nullable bool because Wails does not round-trip *bool reliably.
const (
	forceWorktreesInherit = "inherit"
	forceWorktreesOn      = "on"
	forceWorktreesOff     = "off"
)

// EffectiveForceWorktrees reports whether worktree isolation is mandatory for
// the project containing dir. The resolution (global setting plus per-project
// override, keyed on the main repo root) is internal/launch, so the daemon
// resolves it identically.
func (a *AppService) EffectiveForceWorktrees(dir string) bool {
	return a.launchPolicy().EffectiveForceWorktrees(dir)
}

// forceWorktreeRoot returns the main repo root to hand the PreToolUse firewall
// via MULTITERMINAL_FORCE_WORKTREE_ROOT, or "" when the policy is off here.
func (a *AppService) forceWorktreeRoot(dir string) string {
	return a.launchPolicy().ForceWorktreeRoot(dir)
}

// GetProjectForceWorktrees returns the project's override mode for dir:
// "inherit", "on", or "off".
func (a *AppService) GetProjectForceWorktrees(dir string) string {
	if dir == "" {
		return forceWorktreesInherit
	}
	root, err := mainRepoRoot(dir)
	if err != nil {
		root = dir
	}
	switch override := loadProjectConfig(root).ForceWorktrees; {
	case override == nil:
		return forceWorktreesInherit
	case *override:
		return forceWorktreesOn
	default:
		return forceWorktreesOff
	}
}

// SetProjectForceWorktrees stores the override mode for dir's project.
// mode must be "inherit", "on", or "off"; "inherit" clears the override.
func (a *AppService) SetProjectForceWorktrees(dir string, mode string) error {
	if dir == "" {
		return fmt.Errorf("no directory specified")
	}
	root, err := mainRepoRoot(dir)
	if err != nil {
		root = dir
	}

	cfg := loadProjectConfig(root)
	switch mode {
	case forceWorktreesInherit:
		cfg.ForceWorktrees = nil
	case forceWorktreesOn:
		cfg.ForceWorktrees = boolPtr(true)
	case forceWorktreesOff:
		cfg.ForceWorktrees = boolPtr(false)
	default:
		return fmt.Errorf("invalid mode %q (want inherit, on, or off)", mode)
	}

	if err := saveProjectConfig(root, cfg); err != nil {
		return fmt.Errorf("saving project config: %w", err)
	}
	log.Printf("[force-worktrees] %s: override=%s", root, mode)
	return nil
}

// boolPtr mirrors config.boolPtr, which is unexported in that package.
func boolPtr(b bool) *bool { return &b }
