// One-time per-project setup that lets Claude Code use its own native
// EnterWorktree tool safely: a memory instruction (EnterWorktree only fires
// when "worktree" is explicitly mentioned in a CLAUDE.md/memory file — see
// tool description) plus a settings.local.json pinning worktree.baseRef to
// the currently checked-out branch (spec 2026-07-03 section 3).
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// worktreeAskRules make Claude Code ask before a branch switch instead of
// letting it happen silently (a real dev test caught Claude switching
// branches while a worktree was open; the memory instruction alone did not
// stop it). settings.local.json rules at the project root apply for the whole
// session, even after EnterWorktree moves the cwd (design doc 2026-07-03
// section 2). Ask, not deny: the user wants these actions "nicht einfach so",
// not "gar nicht" — the same correction section 3 of that spec records for
// merge/push. Ask also wins over a broader allow entry such as a
// pre-existing "Bash(git checkout:*)".
var worktreeAskRules = []string{
	"Bash(git checkout *)",
	"Bash(git switch *)",
}

const projectWorktreeSettingsContent = `{
  "worktree": {
    "baseRef": "head"
  },
  "permissions": {
    "ask": [
      "Bash(git checkout *)",
      "Bash(git switch *)"
    ]
  }
}
`

// EnsureProjectWorktreeSetup writes the memory instruction and settings file
// into a project's root. Both are safe to call repeatedly (e.g. once per
// Claude-pane launch and once per restored tab): a CLAUDE.local.md that MTUI
// itself wrote in an earlier version is migrated to the current text, and
// worktree.baseRef is merged into an existing settings.local.json without
// touching unrelated keys. Anything not recognized as MTUI's own prior output
// (custom user content) is left untouched.
//
// Which memory variant is written follows the worktree-mandatory policy for
// this project (EffectiveForceWorktrees). The settings half is written
// identically either way.
func (a *AppService) EnsureProjectWorktreeSetup(dir string) error {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return err
	}

	if err := ensureProjectWorktreeMemory(root, a.EffectiveForceWorktrees(root)); err != nil {
		return err
	}
	if err := ensureProjectWorktreeSettings(root); err != nil {
		return err
	}
	// Keep the generated memory file out of `git status`: it is MTUI's own
	// per-project state, and letting an agent commit it would (a) leak the
	// instruction into the repo and (b) risk a CRLF round-trip that makes the
	// file look user-customized, silently freezing the policy for that project.
	if err := ensureInfoExclude(root, []string{projectWorktreeMemoryFile}); err != nil {
		log.Printf("[worktree-setup] info/exclude: %v", err) // non-fatal
	}
	return nil
}

func ensureProjectWorktreeSettings(root string) error {
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("claude dir: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	existing, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		if err := os.WriteFile(settingsPath, []byte(projectWorktreeSettingsContent), 0644); err != nil {
			return fmt.Errorf("settings file: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("settings file: %w", err)
	}

	merged, changed, err := mergeWorktreeSettings(existing)
	if err != nil {
		log.Printf("[worktree-setup] %s: %v — leaving file untouched", settingsPath, err)
		return nil
	}
	if !changed {
		return nil
	}
	if err := os.WriteFile(settingsPath, merged, 0644); err != nil {
		return fmt.Errorf("settings file: %w", err)
	}
	return nil
}

// mergeWorktreeSettings brings an existing settings.local.json up to date,
// preserving every other key untouched: worktree.baseRef="head" if no baseRef
// is set at all (an existing custom value, e.g. "fresh", is left alone), and
// the one-time move of the branch-switch rules from permissions.deny to
// permissions.ask (migrateWorktreeDenyToAsk). changed is false (and raw is
// returned as-is) when neither applies.
//
// The ask rules are deliberately NOT added to an existing file that lacks
// them: its owner may have removed them, and re-adding on every launch is
// exactly how the old deny rules kept coming back.
func mergeWorktreeSettings(raw []byte) (merged []byte, changed bool, err error) {
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return raw, false, fmt.Errorf("parse settings.local.json: %w", err)
	}

	baseRefChanged := ensureWorktreeBaseRef(settings)
	migrated := migrateWorktreeDenyToAsk(settings)
	if !baseRefChanged && !migrated {
		return raw, false, nil
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return raw, false, fmt.Errorf("marshal settings.local.json: %w", err)
	}
	return append(out, '\n'), true, nil
}

// ensureWorktreeBaseRef sets settings["worktree"]["baseRef"]="head" if no
// baseRef is present yet. Reports whether it changed anything.
func ensureWorktreeBaseRef(settings map[string]any) bool {
	worktree, _ := settings["worktree"].(map[string]any)
	if worktree != nil {
		if _, ok := worktree["baseRef"]; ok {
			return false
		}
	} else {
		worktree = map[string]any{}
	}
	worktree["baseRef"] = "head"
	settings["worktree"] = worktree
	return true
}

// migrateWorktreeDenyToAsk removes the branch-switch rules that earlier MTUI
// versions wrote into permissions.deny (and re-added on every launch) and
// puts them into permissions.ask instead, once. A deny list left empty is
// dropped. Once the rules are out of deny this is a no-op, so a user who then
// removes them from ask keeps it that way. Reports whether it changed anything.
func migrateWorktreeDenyToAsk(settings map[string]any) bool {
	permissions, _ := settings["permissions"].(map[string]any)
	if permissions == nil {
		return false
	}
	deny, _ := permissions["deny"].([]any)
	isRule := make(map[string]bool, len(worktreeAskRules))
	for _, r := range worktreeAskRules {
		isRule[r] = true
	}
	keep := make([]any, 0, len(deny))
	var moved []string
	for _, v := range deny {
		if s, ok := v.(string); ok && isRule[s] {
			moved = append(moved, s)
			continue
		}
		keep = append(keep, v)
	}
	if len(moved) == 0 {
		return false
	}
	if len(keep) == 0 {
		delete(permissions, "deny")
	} else {
		permissions["deny"] = keep
	}

	ask, _ := permissions["ask"].([]any)
	have := make(map[string]bool, len(ask))
	for _, v := range ask {
		if s, ok := v.(string); ok {
			have[s] = true
		}
	}
	for _, s := range moved {
		if !have[s] {
			ask = append(ask, s)
			have[s] = true
		}
	}
	permissions["ask"] = ask
	return true
}
