package launch

import (
	"fmt"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/gitx"
)

// Policy is the part of the configuration a session launch depends on.
//
// It is a value rather than a reference to the config so that a caller in
// another process can be handed one over the wire: the daemon needs the same
// two facts the window has, and nothing else.
type Policy struct {
	// ShimPort is where the hook and statusline helpers post. It comes from
	// the session host, because the value is baked into the session's
	// environment at launch and has to outlive any one window.
	ShimPort int
	// ForceWorktrees is the GLOBAL worktree-mandatory setting. The
	// per-project override in .mtui/config.json is read here, per directory.
	ForceWorktrees bool
}

// IsClaudeMode reports whether the mode is backed by the claude CLI.
//
// Only those understand --resume, and only those have the hook wiring the
// other variables serve; codex and gemini have no verified resume path.
func IsClaudeMode(mode string) bool {
	return mode == "claude" || mode == "claude-auto" || mode == "claude-yolo"
}

// Env builds the PTY environment for a session.
//
// Creating a session and resuming one MUST both go through it. A woken pane
// that loses MULTITERMINAL_SESSION_ID drops its hook and statusline wiring,
// and its activity detection silently falls back to screen scraping; one that
// loses MULTITERMINAL_FORCE_WORKTREE_ROOT loses the worktree firewall in
// cmd/mtui-hook. Both failures are invisible until something goes wrong,
// which is exactly why they need one code path and not two.
func (p Policy) Env(sessionID int, dir, mode string) []string {
	var env []string
	if p.ShimPort > 0 {
		env = append(env, fmt.Sprintf("MTUI_PORT=%d", p.ShimPort))
	}
	if !IsClaudeMode(mode) {
		return env
	}
	env = append(env, fmt.Sprintf("MULTITERMINAL_SESSION_ID=%d", sessionID))
	env = append(env, WorktreeEnvVars(dir)...)
	// The worktree-mandatory policy is resolved once here (global setting plus
	// per-project override) so that mtui-hook only ever reads one variable and
	// never the config. Empty when the policy is off or dir is not a repo.
	if root := p.ForceWorktreeRoot(dir); root != "" {
		env = append(env, "MULTITERMINAL_FORCE_WORKTREE_ROOT="+root)
	}
	return env
}

// WorktreeEnvVars returns the worktree variables for a pane whose dir is a
// linked worktree rather than the main checkout.
//
// It returns nil for the main checkout (including subdirectories of it), for
// non-git directories, and for any lookup failure: the session then launches
// without the restriction, exactly as it did before the feature existed.
//
// Accepted cost: one or two synchronous git subprocesses per Claude-mode
// launch, the same order of magnitude as the other one-time git calls a launch
// already makes. Not measured; revisit if launch latency becomes a complaint.
func WorktreeEnvVars(dir string) []string {
	worktree, mainRoot, ok := gitx.IsLinkedWorktree(dir)
	if !ok {
		return nil
	}
	return []string{
		"MULTITERMINAL_WORKTREE_PATH=" + worktree,
		"MULTITERMINAL_MAIN_REPO_ROOT=" + mainRoot,
	}
}

// ForceWorktreeRoot returns the main repo root to hand the PreToolUse firewall
// via MULTITERMINAL_FORCE_WORKTREE_ROOT, or "" when the policy is off for this
// project or dir is not inside a git repo, since there is nothing to protect
// then.
func (p Policy) ForceWorktreeRoot(dir string) string {
	if dir == "" {
		return ""
	}
	root, err := gitx.MainRepoRoot(dir)
	if err != nil {
		return ""
	}
	if override := LoadProjectConfig(root).ForceWorktrees; override != nil {
		if !*override {
			return ""
		}
		return root
	}
	if !p.ForceWorktrees {
		return ""
	}
	return root
}

// EffectiveForceWorktrees reports whether worktree isolation is mandatory for
// the project containing dir.
//
// The per-project override wins over the global setting; absent an override
// the global setting applies. The override is keyed on the MAIN repo root, so
// a session in a linked worktree or a subdirectory resolves to the same
// project as one in the checkout root.
func (p Policy) EffectiveForceWorktrees(dir string) bool {
	if dir == "" {
		return p.ForceWorktrees
	}
	root, err := gitx.MainRepoRoot(dir)
	if err != nil {
		return p.ForceWorktrees
	}
	if override := LoadProjectConfig(root).ForceWorktrees; override != nil {
		return *override
	}
	return p.ForceWorktrees
}
