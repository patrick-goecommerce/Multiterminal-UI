// One-time per-project setup that lets Claude Code use its own native
// EnterWorktree tool safely: a memory instruction (EnterWorktree only fires
// when "worktree" is explicitly mentioned in a CLAUDE.md/memory file — see
// tool description) plus a settings.local.json pinning worktree.baseRef to
// the currently checked-out branch (spec 2026-07-03 section 3).
package backend

import (
	"fmt"
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

// Hard rule (user-mandated after a real dev test caught Claude switching
// branches while a worktree was open): deny git checkout/switch via Bash for
// the whole session. This is the only verified enforcement mechanism —
// settings.local.json rules written at the project root apply session-wide,
// even after EnterWorktree moves the cwd into the worktree (design doc
// 2026-07-03 section 2) — so it blocks a self-switch both inside an active
// worktree and in the main checkout. Accepted collateral: also blocks the
// file-restore form (`git checkout -- <file>`), same trade-off already made
// for the merge-abort collateral in the pre-EnterWorktree design (spec
// 2026-07-02 section 3.5).
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
// into a project's root, once. Existing files are left untouched so manual
// edits by the user survive repeated calls (e.g. one per Claude-pane launch).
func (a *AppService) EnsureProjectWorktreeSetup(dir string) error {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return err
	}

	memPath := filepath.Join(root, projectWorktreeMemoryFile)
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		if err := os.WriteFile(memPath, []byte(projectWorktreeMemoryContent), 0644); err != nil {
			return fmt.Errorf("memory file: %w", err)
		}
	}

	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("claude dir: %w", err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		if err := os.WriteFile(settingsPath, []byte(projectWorktreeSettingsContent), 0644); err != nil {
			return fmt.Errorf("settings file: %w", err)
		}
	}
	return nil
}
