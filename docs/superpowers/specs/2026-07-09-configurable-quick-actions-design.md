# Konfigurierbare Quick-Actions & Fertigstellen-Prompt

## Problem

Der Fertig-Button (✓) in der Pane-Titelleiste sendet immer denselben fest
einprogrammierten Prompt (`prepPromptTemplate` in `app_worktree_finish.go`):
committen, auf den Ziel-Branch rebasen, nicht selbst mergen/pushen/PR
erstellen. Das ist eine von vielen möglichen Vorgehensweisen — andere Nutzer
wollen stattdessen pushen und einen Pull Request erstellen, einen Loop
einbauen der prüft ob der PR gemerged ist, danach den Worktree aufräumen,
oder einen `git pull` automatisieren. Es gibt nicht die eine richtige
Variante.

Zusätzlich gibt es außer "Commit & Push" (☁) und "Fertigstellen" (✓) keine
Möglichkeit, sich eigene, häufig gebrauchte Prompts als Buttons in die
Pane-Titelleiste zu legen.

## Ziel

Ein einheitliches System:

1. Der Text, der beim Klick auf ✓ als Vorbereitungs-Prompt gesendet wird,
   ist pro Nutzer in den Settings frei editierbar (Default = heutiges
   Verhalten).
2. Nutzer können zusätzlich bis zu 5 eigene Quick-Action-Buttons
   konfigurieren, die beim Klick einen frei definierten Prompt in die
   Prompt-Queue der Pane legen.

Beide sind im Kern dieselbe Mechanik: ein (Platzhalter-)Text wird an eine
laufende Claude-Code-Session gesendet. Was die Session mit diesem Text tut
(inkl. eigener Slash-Commands/Skills wie `/loop`) entscheidet Claude Code
selbst — Multiterminal interpretiert den Text nicht, er geht 1:1 durch.

## Nicht-Ziele

- Kein Schritt-Baukasten / keine Workflow-Engine in Multiterminal. Die
  gesamte Flexibilität kommt aus frei editierbarem Prompt-Text, nicht aus
  einer Sequenz konfigurierbarer Einzelschritte.
- Keine Slash-Command-Erkennung, -Validierung oder -Autocomplete durch
  Multiterminal. Der Text geht unverändert in die PTY; Claude Code kennt
  seine eigenen Skills bereits.
- Quick-Actions erscheinen ausschließlich bei Claude-Panes (`pane.mode ===
  'claude'`), nicht bei Shell-Panes.
- Die bestehende Fertigstellen-State-Machine (`preparing` → `ready` /
  `blocked` → `merging` → `cleanup`, inkl. des Bestätigungsdialogs mit
  Commit-Liste und Diff-Stat) bleibt unverändert. Sie deckt bereits den
  Fall ab, dass ein eigener Prompt Push+PR+Merge selbst erledigt: sobald der
  Ziel-Branch keine neuen Commits mehr gegenüber dem lokalen Branch hat
  (`GetWorktreeFinishStatus`, `count == 0`), springt der Dialog automatisch
  auf `cleanup_only` — es gibt dann nur noch "Nur aufräumen" zu bestätigen.

## Datenmodell

Neue Felder in `config.Config` (`internal/config/config.go`), nach dem
Muster von `ClaudeModels`:

```go
FinishPrepPrompt string        `yaml:"finish_prep_prompt" json:"finish_prep_prompt"`
QuickActions     []QuickAction `yaml:"quick_actions" json:"quick_actions"`

type QuickAction struct {
    Label  string `yaml:"label" json:"label"`   // 1-2 Zeichen oder Emoji, z.B. "🔁"
    Prompt string `yaml:"prompt" json:"prompt"` // Text mit Platzhaltern
}
```

- `FinishPrepPrompt == ""` → heutiges hartcodiertes Template bleibt aktiv
  (Abwärtskompatibilität, kein Migrationsschritt nötig).
- `QuickActions` leer (Default) → Titelleiste zeigt nur ☁ und ✓ wie heute.
- Maximal 5 Einträge in `QuickActions` — durchgesetzt im Settings-UI (kein
  Backend-Limit nötig, das Feld ist rein additiv).

### Platzhalter

In beiden Prompt-Texten per einfacher String-Ersetzung (nicht mehr
`fmt.Sprintf`-positional wie heute) verfügbar:

- `{{branch}}` — Branch der Pane
- `{{targetBranch}}` — Ziel-Branch (Merge-Ziel)
- `{{worktreePath}}` — Pfad des Worktrees

Bei Panes ohne Worktree (z.B. Hauptrepo-Claude-Pane) bleiben
`{{targetBranch}}`/`{{worktreePath}}` leer; `{{branch}}` kann trotzdem den
aktuellen Branch des Hauptrepos widerspiegeln.

## Backend-Änderungen

- `app_worktree_finish.go`: `prepPromptTemplate`-Konstante bleibt als
  Fallback-Wert bestehen. Neue kleine Hilfsfunktion `renderFinishPrompt(cfg
  *config.Config, branch, target, worktreePath string) string` ersetzt die
  `fmt.Sprintf`-Verwendung: nutzt `cfg.FinishPrepPrompt`, wenn gesetzt,
  sonst den bisherigen Text; ersetzt danach die drei Platzhalter.
- Keine Änderung an `CheckWorktreeFinish`, `GetWorktreeFinishStatus`,
  `onQueueItemDone` oder der Phasen-Logik — diese bleiben exakt wie heute.

## Frontend-Änderungen

- **`terminal.ts`/Config-Store**: kein neuer Roundtrip nötig — Platzhalter-
  Rendering für Quick-Actions passiert im Frontend, da Branch/Target/
  Worktree-Pfad bereits im Pane-Store liegen (`pane.branch`,
  `pane.targetBranch`, `pane.worktreePath`). Eine kleine reine Funktion
  `renderQuickActionPrompt(template, pane)` in `frontend/src/lib/` ersetzt
  die drei Platzhalter.
- **`SettingsDialog.svelte`**: neuer Abschnitt "Quick Actions" mit
  - einem Textfeld für `finishPrepPrompt` (Platzhalter-Hinweistext),
  - einer Liste von bis zu 5 Zeilen `{ Label, Prompt }` mit Hinzufügen-/
    Entfernen-Buttons.
  Die Init-Logik lebt in `initDialog()`, keine `$:`-Blöcke, die
  Dialog-Variablen sowohl lesen als auch schreiben (bekannter
  Reset-Bug, siehe CLAUDE.md).
- **`PaneTitlebar.svelte`**: rendert nach dem ✓-Button zusätzlich
  `{#each $config.quickActions as qa}` einen Chip-Button (Label = `qa.label`,
  Tooltip = `qa.prompt`, gekürzt) — nur wenn `pane.mode === 'claude'`.
  Klick dispatcht `quickAction` mit `{ sessionId, prompt: qa.prompt }`.
- **Event-Handler** (wo `finishWorktree`/`commitPush` heute verarbeitet
  werden): neuer Handler für `quickAction` — rendert Platzhalter, ruft
  `App.AddToQueue(sessionId, renderedPrompt)`.
- **`wailsjs/go/models.ts`**: `QuickAction`-Klasse ergänzen sowie
  `quickActions`/`finishPrepPrompt`-Felder in der `Config`-Klasse
  (Konstruktor-Konvertierung nicht vergessen — Pflichtschritt laut
  CLAUDE.md, sonst werden die Felder beim Deserialisieren stillschweigend
  zu `undefined`).

## Testing

- `internal/config`: Roundtrip-Test für `finish_prep_prompt`/
  `quick_actions` (YAML + JSON), leerer Default → Fallback-Prompt greift.
- Backend: Unit-Test für `renderFinishPrompt` (alle drei Platzhalter, sowie
  Custom-Prompt ohne jeden Platzhalter).
- Frontend: Unit-Test für `renderQuickActionPrompt` (Platzhalter-Ersetzung
  inkl. leerer Worktree-Felder).
- Frontend: Test, dass der Quick-Action-Button bei `pane.mode !== 'claude'`
  nicht gerendert wird.
- `SettingsDialog`: expliziter Check des bekannten Recurring-Bugs
  (`grep -n '$:' SettingsDialog.svelte` → nur `visible`-Zeile), da hier ein
  neuer Abschnitt mit Liste + Textfeldern reinkommt — höchstes
  Rückfall-Risiko laut CLAUDE.md.
- Kein neuer Test für die Fertigstellen-State-Machine nötig, da an ihr
  nichts geändert wird.
