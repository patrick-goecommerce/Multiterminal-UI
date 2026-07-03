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

Wenn eine Aufgabe erledigt ist: committe, pushe bei Bedarf und öffne einen
Pull Request (` + "`gh pr create`" + `), oder frage den Nutzer nach dem gewünschten
Vorgehen.

Nutze ` + "`ExitWorktree`" + ` mit ` + "`discard_changes: true`" + ` oder erzwungenem Entfernen
NIEMALS eigenständig — nur nach ausdrücklicher Rückfrage beim Nutzer und
dessen Bestätigung.
`

const projectWorktreeSettingsContent = `{
  "worktree": {
    "baseRef": "head"
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
