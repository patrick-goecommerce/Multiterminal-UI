// Worktree-mandatory policy: resolves the global config.ForceWorktrees setting
// against the per-project override in .mtui/config.json, and exposes both to
// the frontend. The resolved value is turned into the
// MULTITERMINAL_FORCE_WORKTREE_ROOT env var at session launch (see
// CreateSession) — the mtui-hook binary reads only that env var and never the
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
// the project containing dir. The per-project override wins over the global
// setting; absent an override the global setting applies.
//
// The override is keyed on the MAIN repo root, so a session running in a
// linked worktree or a subdirectory resolves to the same project as one
// running in the checkout root.
func (a *AppService) EffectiveForceWorktrees(dir string) bool {
	global := a.cfg.ShouldForceWorktrees()
	if dir == "" {
		return global
	}
	root, err := mainRepoRoot(dir)
	if err != nil {
		return global
	}
	if override := loadProjectConfig(root).ForceWorktrees; override != nil {
		return *override
	}
	return global
}

// forceWorktreeRoot returns the main repo root to hand the PreToolUse firewall
// via MULTITERMINAL_FORCE_WORKTREE_ROOT, or "" when the policy is off for this
// project or dir is not inside a git repo (nothing to protect then).
func (a *AppService) forceWorktreeRoot(dir string) string {
	if dir == "" {
		return ""
	}
	root, err := mainRepoRoot(dir)
	if err != nil {
		return ""
	}
	if override := loadProjectConfig(root).ForceWorktrees; override != nil {
		if !*override {
			return ""
		}
		return root
	}
	if !a.cfg.ShouldForceWorktrees() {
		return ""
	}
	return root
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
