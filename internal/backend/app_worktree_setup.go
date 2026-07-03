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

const projectWorktreeMemoryFile = "CLAUDE.local.md"

const projectWorktreeMemoryContent = `# MTUI-Projekt: Worktree-Isolation

Nutze für Aufgaben mit eigenem, klar abgegrenztem Arbeitsbereich das
` + "`EnterWorktree`" + `-Tool, um deine Änderungen zu isolieren. Der Haupt-Branch
dieses Projekts darf dabei nie direkt gewechselt werden.

Wenn eine Aufgabe erledigt ist: committe deine Arbeit in nachvollziehbaren
Commits. Push, das Erstellen eines Pull Requests (` + "`gh pr create`" + `) und jeder
Merge oder Fast-Forward durch dich selbst erfordern IMMER vorherige Zustimmung
des Nutzers — frage vorher nach, statt eigenständig zu pushen, zu mergen oder
einen PR zu öffnen.

Nutze ` + "`ExitWorktree`" + ` mit ` + "`discard_changes: true`" + ` oder erzwungenem Entfernen
NIEMALS eigenständig — nur nach ausdrücklicher Rückfrage beim Nutzer und
dessen Bestätigung.
`

// projectWorktreeMemoryPriorVersions lists exact text this function itself
// wrote out in earlier revisions. A CLAUDE.local.md matching one of these
// verbatim is entirely MTUI-owned and safe to migrate to the current text;
// anything else (custom user content, or the current version already) is
// left untouched.
var projectWorktreeMemoryPriorVersions = []string{
	`# MTUI-Projekt: Worktree-Isolation

Nutze für Aufgaben mit eigenem, klar abgegrenztem Arbeitsbereich das
` + "`EnterWorktree`" + `-Tool, um deine Änderungen zu isolieren. Der Haupt-Branch
dieses Projekts darf dabei nie direkt gewechselt werden.

Wenn eine Aufgabe erledigt ist: committe, pushe bei Bedarf und öffne einen
Pull Request (` + "`gh pr create`" + `), oder frage den Nutzer nach dem gewünschten
Vorgehen.

Nutze ` + "`ExitWorktree`" + ` mit ` + "`discard_changes: true`" + ` oder erzwungenem Entfernen
NIEMALS eigenständig — nur nach ausdrücklicher Rückfrage beim Nutzer und
dessen Bestätigung.
`,
}

// worktreeDenyRules are the hard-enforced Bash denies (user-mandated after a
// real dev test caught Claude switching branches while a worktree was open —
// the memory-only instruction "Haupt-Branch darf nie direkt gewechselt
// werden" is prompt context, not enforcement). settings.local.json rules
// written at the project root apply for the whole session, even after
// EnterWorktree moves the cwd into a worktree (design doc 2026-07-03 section
// 2), so these block a self-switch both inside an active worktree and in the
// main checkout. Accepted collateral: also blocks the file-restore form
// (`git checkout -- <file>`), same trade-off already made for the
// merge-abort collateral in the pre-EnterWorktree design (spec 2026-07-02
// section 3.5). Deny always wins over any Allow entry a project already has
// (e.g. a pre-existing blanket "Bash(git checkout:*)" allow) — verified.
var worktreeDenyRules = []string{
	"Bash(git checkout *)",
	"Bash(git switch *)",
}

const projectWorktreeSettingsContent = `{
  "worktree": {
    "baseRef": "head"
  },
  "permissions": {
    "deny": [
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
func (a *AppService) EnsureProjectWorktreeSetup(dir string) error {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return err
	}

	if err := ensureProjectWorktreeMemory(root); err != nil {
		return err
	}
	if err := ensureProjectWorktreeSettings(root); err != nil {
		return err
	}
	return nil
}

func ensureProjectWorktreeMemory(root string) error {
	memPath := filepath.Join(root, projectWorktreeMemoryFile)
	existing, err := os.ReadFile(memPath)
	if os.IsNotExist(err) {
		if err := os.WriteFile(memPath, []byte(projectWorktreeMemoryContent), 0644); err != nil {
			return fmt.Errorf("memory file: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("memory file: %w", err)
	}

	content := string(existing)
	if content == projectWorktreeMemoryContent {
		return nil
	}
	for _, prior := range projectWorktreeMemoryPriorVersions {
		if content == prior {
			if err := os.WriteFile(memPath, []byte(projectWorktreeMemoryContent), 0644); err != nil {
				return fmt.Errorf("memory file: %w", err)
			}
			return nil
		}
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

// mergeWorktreeSettings brings an existing settings.local.json up to date
// with two independent, additive changes, preserving every other key
// untouched: worktree.baseRef="head" if no baseRef is set at all (an
// existing custom value, e.g. "fresh", is left alone), and each rule in
// worktreeDenyRules appended to permissions.deny if not already present
// (order-preserving, no duplicates). Either half can apply on its own —
// e.g. a project with baseRef already set from an older MTUI version still
// gets the deny rules added when this fix rolls out. changed is false (and
// raw is returned as-is) when both are already satisfied.
func mergeWorktreeSettings(raw []byte) (merged []byte, changed bool, err error) {
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return raw, false, fmt.Errorf("parse settings.local.json: %w", err)
	}

	baseRefChanged := ensureWorktreeBaseRef(settings)
	denyChanged := ensureWorktreeDenyRules(settings)
	if !baseRefChanged && !denyChanged {
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

// ensureWorktreeDenyRules appends any of worktreeDenyRules missing from
// settings["permissions"]["deny"], preserving existing entries and order.
// Reports whether it changed anything.
func ensureWorktreeDenyRules(settings map[string]any) bool {
	permissions, _ := settings["permissions"].(map[string]any)
	if permissions == nil {
		permissions = map[string]any{}
	}

	existingDeny, _ := permissions["deny"].([]any)
	have := make(map[string]bool, len(existingDeny))
	for _, v := range existingDeny {
		if s, ok := v.(string); ok {
			have[s] = true
		}
	}

	changed := false
	for _, rule := range worktreeDenyRules {
		if !have[rule] {
			existingDeny = append(existingDeny, rule)
			changed = true
		}
	}
	if !changed {
		return false
	}

	permissions["deny"] = existingDeny
	settings["permissions"] = permissions
	return true
}
