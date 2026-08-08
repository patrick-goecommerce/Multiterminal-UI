// The per-project CLAUDE.local.md that MTUI maintains: two variants (advisory
// and mandatory, chosen by the worktree-mandatory policy) plus the logic that
// decides whether an existing file is MTUI's own output and may be rewritten.
package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const projectWorktreeMemoryFile = "CLAUDE.local.md"

// worktreeMemoryMarker is the first line of every file MTUI generates. It
// replaces whole-body comparison as the "is this ours?" test: the text may
// then change freely between versions without stranding existing projects,
// and switching the policy on/off rewrites the file in either direction.
// A file without the marker is only recognized via
// projectWorktreeMemoryPriorVersions (pre-marker installations).
const worktreeMemoryMarker = "<!-- mtui-worktree-memory — automatisch von Multiterminal gepflegt -->"

// projectWorktreeMemoryContent is the advisory variant, written when the
// worktree-mandatory policy is OFF. Isolation is recommended, not enforced.
const projectWorktreeMemoryContent = worktreeMemoryMarker + `
# MTUI-Projekt: Worktree-Isolation

Nutze für Aufgaben mit eigenem, klar abgegrenztem Arbeitsbereich das
` + "`EnterWorktree`" + `-Tool, um deine Änderungen zu isolieren. Der Haupt-Branch
dieses Projekts darf dabei nie direkt gewechselt werden.

` + worktreeMemorySharedRules

// projectWorktreeMemoryForcedContent is the mandatory variant, written when
// the policy is ON. The rule is backed by the mtui-hook PreToolUse firewall
// (cmd/mtui-hook/firewall.go), which denies the write outright — this text
// exists so the model isolates up front instead of learning it from a denial.
const projectWorktreeMemoryForcedContent = worktreeMemoryMarker + `
# MTUI-Projekt: Worktree-Pflicht

Worktree-Isolation ist in diesem Projekt **verpflichtend**. Wechsle mit dem
` + "`EnterWorktree`" + `-Tool in ein eigenes Worktree, BEVOR du Code änderst —
Schreibzugriffe direkt im Hauptrepo werden technisch abgelehnt. Der
Haupt-Branch dieses Projekts darf nie direkt gewechselt werden.

Arbeitest du bereits in einem Worktree, ist keine weitere Isolation nötig —
lege dann keinesfalls ein zweites Worktree an.

Ausgenommen von der Pflicht sind Dokumentation und Planung: ` + "`.md`" + `-Dateien
sowie alles unterhalb von ` + "`docs/`" + `, ` + "`.mtui/`" + ` und ` + "`.claude/`" + ` darfst du
weiterhin direkt im Projektverzeichnis bearbeiten.

` + worktreeMemorySharedRules

// worktreeMemorySharedRules are the paragraphs both variants end with — they
// hold regardless of whether isolation is advisory or mandatory.
const worktreeMemorySharedRules = `Wenn eine Aufgabe erledigt ist: committe deine Arbeit in nachvollziehbaren
Commits. Push, das Erstellen eines Pull Requests (` + "`gh pr create`" + `) und jeder
Merge oder Fast-Forward durch dich selbst erfordern IMMER vorherige Zustimmung
des Nutzers — frage vorher nach, statt eigenständig zu pushen, zu mergen oder
einen PR zu öffnen.

Nutze ` + "`ExitWorktree`" + ` mit ` + "`discard_changes: true`" + ` oder erzwungenem Entfernen
NIEMALS eigenständig — nur nach ausdrücklicher Rückfrage beim Nutzer und
dessen Bestätigung.

Wenn du innerhalb dieses Worktrees eine Spec (` + "`docs/superpowers/specs/...`" + `)
oder einen Plan (` + "`docs/superpowers/plans/...`" + `) erstellst, ergänze den
Dokument-Header um eine Zeile ` + "`**Worktree:** <absoluter Pfad zu diesem Worktree-Checkout>`" + `
(direkt nach ` + "`**Status:**`" + `, bzw. als letzte Header-Zeile ohne Status-Feld).
Im Hauptrepo (kein aktiver Worktree) entfällt die Zeile. So bleibt auch nach
einem Merge erkennbar, in welchem Worktree ein Dokument entstanden ist.
`

// projectWorktreeMemoryPriorVersions lists exact text this function itself
// wrote out in earlier, PRE-MARKER revisions. A CLAUDE.local.md matching one
// of these is entirely MTUI-owned and safe to migrate to the current text;
// anything else without the marker (custom user content) is left untouched.
// New versions no longer need to be added here — the marker covers them.
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
	`# MTUI-Projekt: Worktree-Isolation

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
`,
	// Last pre-marker version: identical to today's advisory text minus the
	// marker line. Without this entry every existing install would look
	// user-customized the moment the marker was introduced.
	`# MTUI-Projekt: Worktree-Isolation

Nutze für Aufgaben mit eigenem, klar abgegrenztem Arbeitsbereich das
` + "`EnterWorktree`" + `-Tool, um deine Änderungen zu isolieren. Der Haupt-Branch
dieses Projekts darf dabei nie direkt gewechselt werden.

` + worktreeMemorySharedRules,
}

// normalizeMemory makes the "is this ours?" comparison robust against the
// round-trip a file takes through git and editors on Windows: CRLF line
// endings (core.autocrlf) and trailing whitespace would otherwise make an
// untouched MTUI file look user-customized, silently freezing it forever.
func normalizeMemory(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimRight(s, " \t\n")
}

// isMTUIAuthoredMemory reports whether content was generated by MTUI (and may
// therefore be rewritten) rather than customized by the user.
func isMTUIAuthoredMemory(content string) bool {
	normalized := normalizeMemory(content)
	if strings.HasPrefix(normalized, worktreeMemoryMarker) {
		return true
	}
	for _, prior := range projectWorktreeMemoryPriorVersions {
		if normalized == normalizeMemory(prior) {
			return true
		}
	}
	return false
}

// ensureProjectWorktreeMemory writes the variant matching the current policy,
// migrating between variants in BOTH directions when the file is MTUI's own.
// User-customized content is never touched — that is also the (undocumented,
// deliberate) escape hatch for a project that wants its own instructions.
func ensureProjectWorktreeMemory(root string, forced bool) error {
	desired := projectWorktreeMemoryContent
	if forced {
		desired = projectWorktreeMemoryForcedContent
	}

	memPath := filepath.Join(root, projectWorktreeMemoryFile)
	existing, err := os.ReadFile(memPath)
	if os.IsNotExist(err) {
		if err := os.WriteFile(memPath, []byte(desired), 0644); err != nil {
			return fmt.Errorf("memory file: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("memory file: %w", err)
	}

	content := string(existing)
	if normalizeMemory(content) == normalizeMemory(desired) {
		return nil
	}
	if !isMTUIAuthoredMemory(content) {
		return nil
	}
	if err := os.WriteFile(memPath, []byte(desired), 0644); err != nil {
		return fmt.Errorf("memory file: %w", err)
	}
	return nil
}
