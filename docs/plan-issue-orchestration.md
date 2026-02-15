# Integrationsplan: Issue-Orchestrierung für Multiterminal

> Inspiriert von ccpm-Konzepten, nativ in Go/Svelte gebaut.
> Ziel: GitHub Issue → Branch → Claude Agent → Progress → Close

---

## Status Quo

### Was schon da ist
| Feature | Wo | Status |
|---|---|---|
| GitHub Issues CRUD | `app_issues.go` (250 LOC) | List, Detail, Create, Update, Comment, Labels |
| Issues-Parsing | `app_issues_parse.go` (141 LOC) | JSON-Parsing für `gh` CLI Output |
| Issues-UI (Sidebar) | `IssuesView.svelte` | Liste, Detail, Filter, Suche, Drag-to-Pane |
| Issue-Dialog | `IssueDialog.svelte` | Create/Edit Dialog mit Labels |
| Drag & Drop | `IssuesView.svelte:117-135` | Generiert `Closes #N: Title\n...Body` Text |
| Git Branch/Status | `app_git.go` | Branch-Name, Commit-Age, File-Statuses |
| Activity Detection | `app_scan.go` + `activity.go` | Idle/Active/Done/NeedsInput pro Session |
| Pipeline Queue | `app_queue.go` | Prompt-Batching pro Session |

### Was fehlt (= dieses Projekt)
1. **Issue ↔ Pane Binding** — Kein Pane weiß, an welchem Issue es arbeitet
2. **Auto-Branch pro Issue** — Kein automatisches `git checkout -b issue/42-fix-bug`
3. **Git Worktree Support** — Kein isolierter Workspace pro Agent
4. **Progress Tracking** — Keine automatischen Status-Updates an GitHub
5. **Issue-aware Launch** — Kein "Starte Claude für dieses Issue" Workflow

---

## Architektur-Übersicht

```
┌─ Sidebar: Issues ─────────┐     ┌─ PaneGrid ──────────────────────┐
│                            │     │                                  │
│  #42 Fix login bug    [▶]──┼────→│  Pane: "Claude – #42"           │
│  #43 Add dark mode    [▶]  │     │  Branch: issue/42-fix-login-bug │
│  #44 Refactor API     [▶]  │     │  Worktree: ~/.mt-worktrees/42/  │
│                            │     │  ┌──────────────────────────┐   │
│  ● = Pane aktiv            │     │  │  Claude arbeitet...      │   │
│  ✓ = Erledigt              │     │  │  Activity: active ████   │   │
│                            │     │  └──────────────────────────┘   │
└────────────────────────────┘     └──────────────────────────────────┘
         │                                        │
         │  Activity: done ──────────────────────→ │
         │                                        ↓
         │                           ┌─ Go Backend ──────────────┐
         │                           │  Auto-Comment auf #42:    │
         │                           │  "Agent fertig. Branch:   │
         │                           │   issue/42-fix-login-bug" │
         │                           │  Optional: Auto-Close     │
         └───────────────────────────┘                           │
                                     └───────────────────────────┘
```

---

## Phase 1: Issue ↔ Pane Binding

> Kernfeature: Ein Issue einem Pane zuordnen, im UI sichtbar machen.

### 1.1 Pane-Datenmodell erweitern

**`frontend/src/stores/tabs.ts`** — `Pane` Interface:
```typescript
export interface Pane {
  // ... bestehende Felder ...
  issueNumber: number | null;   // NEU: Verknüpftes GitHub Issue
  issueTitle: string;           // NEU: Titel für Anzeige
  issueBranch: string;          // NEU: Zugehöriger Branch-Name
}
```

**`internal/config/session.go`** — `SavedPane` Struct:
```go
type SavedPane struct {
    Name        string `json:"name"`
    Mode        int    `json:"mode"`
    Model       string `json:"model"`
    IssueNumber int    `json:"issue_number,omitempty"` // NEU
    IssueBranch string `json:"issue_branch,omitempty"` // NEU
}
```

### 1.2 Issue-Launch-Flow

**Neuer Button in `IssuesView.svelte`**: Neben jedem Issue ein Play-Button `[▶]`
→ Klick öffnet den bestehenden `LaunchDialog`, aber **vorbelegt** mit Issue-Kontext.

**`LaunchDialog.svelte`** erweitern:
- Neuer optionaler Prop: `issueContext: { number, title, body, labels } | null`
- Wenn gesetzt: Zeigt Issue-Titel an, Launch-Button sagt "Claude für #42 starten"
- Nach Launch: Pane bekommt `issueNumber`, Pane-Name wird `"Claude – #42"`

**Neuer Flow:**
```
IssuesView [▶] → dispatch('launchForIssue', issue)
  → App.svelte: showLaunchDialog = true, issueContext = issue
    → LaunchDialog: "Claude für #42 starten"
      → handleLaunch() mit issueContext
        → tabStore.addPane(..., issueNumber: 42, issueTitle: "Fix login bug")
```

### 1.3 Auto-Prompt bei Issue-Launch

Nach dem Pane-Start wird automatisch ein initialer Prompt an den Claude-Agent gesendet:

```
Closes #42: Fix login bug
Labels: bug, high-priority

Login fails when password contains special characters.
Steps to reproduce: ...

Ref: #42
```

Das ist exakt der Text, den `buildDragText()` schon generiert — Wiederverwendung.
Timing: 500ms nach Session-Start (warten bis Claude bereit ist), dann `WriteToSession()`.

### 1.4 UI: Pane-Titlebar mit Issue-Badge

**`PaneTitlebar.svelte`** erweitern:
- Wenn `pane.issueNumber`: Zeige `#42` Badge neben dem Pane-Namen
- Badge ist klickbar → öffnet Issue-Detail in Sidebar
- Farbe: Grün (open), Lila (closed)

### 1.5 UI: Issues-Sidebar zeigt verknüpfte Panes

**`IssuesView.svelte`** erweitern:
- Issues die mit einem aktiven Pane verknüpft sind, zeigen einen Indikator:
  - `●` (gelb pulsierend) = Agent arbeitet (activity: active)
  - `●` (grün) = Agent fertig (activity: done)
  - `●` (orange) = Braucht Input (activity: needsInput)
- Info kommt via neuen Svelte Store oder Event

**Dateien:**
| Datei | Änderung | Geschätzt |
|---|---|---|
| `stores/tabs.ts` | Pane-Interface + addPane erweitern | ~20 LOC |
| `IssuesView.svelte` | Play-Button, Activity-Indikator | ~40 LOC |
| `LaunchDialog.svelte` | Issue-Kontext Prop + UI | ~30 LOC |
| `App.svelte` | `handleLaunchForIssue()`, Auto-Prompt | ~25 LOC |
| `PaneTitlebar.svelte` | Issue-Badge | ~15 LOC |
| `config/session.go` | SavedPane erweitern | ~5 LOC |

---

## Phase 2: Auto-Branch pro Issue

> Automatisches Branch-Erstellen und -Wechseln beim Issue-Launch.

### 2.1 Backend: Branch-Management

**Neue Datei `internal/backend/app_git_branch.go`** (~80 LOC):

```go
// CreateIssueBranch erstellt einen Branch für ein Issue und wechselt dorthin.
// Format: issue/<number>-<slugified-title>
func (a *App) CreateIssueBranch(dir string, number int, title string) (string, error)

// GetOrCreateIssueBranch prüft ob ein Branch existiert, erstellt ihn ggf.
func (a *App) GetOrCreateIssueBranch(dir string, number int, title string) (string, error)

// slugifyTitle konvertiert "Fix login bug" → "fix-login-bug"
func slugifyTitle(title string) string
```

**Logik:**
1. Branch-Name generieren: `issue/<number>-<slug>` (max 50 Zeichen)
2. Prüfen ob Branch existiert: `git branch --list issue/42-*`
3. Wenn nicht: `git checkout -b issue/42-fix-login-bug`
4. Wenn ja: `git checkout issue/42-fix-login-bug`
5. Branch-Name zurückgeben

### 2.2 Integration in Issue-Launch-Flow

In `App.svelte` → `handleLaunchForIssue()`:
```typescript
// Nach CreateSession, vor Auto-Prompt:
const branchName = await App.GetOrCreateIssueBranch(tab.dir, issue.number, issue.title);
tabStore.addPane(tabId, sessionId, name, mode, model, issue.number, issue.title, branchName);
```

### 2.3 Branch-Schutz

- Nur Branch erstellen wenn `dir` ein Git-Repo ist
- Dirty Working Tree? → User fragen: "Uncommitted changes. Trotzdem Branch wechseln?"
- Konfigurierbar: `auto_branch_on_issue: true/false` in `~/.multiterminal.yaml`

**Dateien:**
| Datei | Änderung | Geschätzt |
|---|---|---|
| `app_git_branch.go` | Neues File: Branch-CRUD | ~80 LOC |
| `App.svelte` | Branch-Aufruf im Launch-Flow | ~10 LOC |
| `config/config.go` | `AutoBranchOnIssue` Option | ~5 LOC |

---

## Phase 3: Git Worktree Support (Optional/Advanced)

> Isolierte Arbeitsverzeichnisse pro Issue-Agent. Ermöglicht parallele Arbeit
> an mehreren Issues ohne Branch-Konflikte.

### 3.1 Warum Worktrees?

Ohne Worktrees: Alle Panes teilen sich dasselbe Working Directory und denselben Branch.
→ Zwei Claude-Agents können nicht gleichzeitig an #42 und #43 arbeiten.

Mit Worktrees: Jedes Issue bekommt ein eigenes Verzeichnis mit eigenem Branch.
→ Vollständig parallele Arbeit.

### 3.2 Backend: Worktree-Management

**Neue Datei `internal/backend/app_worktree.go`** (~120 LOC):

```go
// WorktreeInfo beschreibt einen Git Worktree.
type WorktreeInfo struct {
    Path   string `json:"path"`
    Branch string `json:"branch"`
    Issue  int    `json:"issue"`
}

// CreateWorktree erstellt einen Worktree für ein Issue.
// Speicherort: <repo>/.multiterminal-worktrees/issue-42/
func (a *App) CreateWorktree(dir string, number int, title string) (*WorktreeInfo, error)

// RemoveWorktree entfernt einen Worktree (nach Issue-Close).
func (a *App) RemoveWorktree(dir string, number int) error

// ListWorktrees zeigt alle aktiven Worktrees.
func (a *App) ListWorktrees(dir string) []WorktreeInfo
```

**Logik:**
1. Worktree-Pfad: `<repo>/.mt-worktrees/issue-<number>/`
2. `git worktree add .mt-worktrees/issue-42 -b issue/42-fix-login-bug`
3. Pane-Session startet im Worktree-Verzeichnis statt im Repo-Root
4. Bei Cleanup: `git worktree remove .mt-worktrees/issue-42`

### 3.3 Integration

- `.mt-worktrees/` zu `.gitignore` hinzufügen
- `CreateSession()` bekommt Worktree-Pfad statt `tab.dir`
- Pane-Titlebar zeigt Worktree-Info: `#42 (worktree)`
- Config: `use_worktrees: true/false` (default: false — opt-in)

### 3.4 Risiken & Einschränkungen

- Worktrees teilen `.git` — gleichzeitige Operationen können kollidieren
- Disk-Space: Jeder Worktree ist eine volle Kopie des Working Trees
- IDE-Kompatibilität: manche IDEs verstehen Worktrees nicht
- **Empfehlung:** Phase 3 nur für Power-User, default off

**Dateien:**
| Datei | Änderung | Geschätzt |
|---|---|---|
| `app_worktree.go` | Neues File: Worktree CRUD | ~120 LOC |
| `App.svelte` | Worktree-Pfad im Launch-Flow | ~15 LOC |
| `config/config.go` | `UseWorktrees` Option | ~5 LOC |
| `PaneTitlebar.svelte` | Worktree-Indikator | ~5 LOC |

---

## Phase 4: Automatisches Progress-Tracking

> Agent-Aktivität zurück an GitHub melden: Kommentare, Status-Updates, Auto-Close.

### 4.1 Event-basiertes Tracking

Bestehende Activity Detection (`app_scan.go`) erkennt schon:
- `ActivityDone` → Agent ist fertig (Prompt zurück)
- `ActivityNeedsInput` → Wartet auf User-Bestätigung

**Neues Verhalten bei Issue-verknüpften Panes:**

| Event | Aktion |
|---|---|
| Pane erstellt + Issue verknüpft | Kommentar auf Issue: "Agent gestartet auf Branch `issue/42-...`" |
| `ActivityDone` (erstes Mal) | Kommentar: "Agent hat Aufgabe abgeschlossen." |
| Pane geschlossen (manuell) | Kommentar: "Session beendet. Kosten: $0.45" |
| User wählt "Issue schließen" | `gh issue close` + finaler Kommentar |

### 4.2 Backend: Progress-Reporter

**Neue Datei `internal/backend/app_issue_progress.go`** (~100 LOC):

```go
// ReportIssueProgress postet einen Status-Kommentar auf ein Issue.
func (a *App) ReportIssueProgress(dir string, number int, event string, details string) error

// Wird aus scanLoop aufgerufen wenn sich Activity ändert:
func (a *App) onActivityChange(sessionID int, oldActivity, newActivity ActivityState) {
    // Prüfen ob Session ein Issue hat
    // Wenn ja: ReportIssueProgress() aufrufen
}
```

**Kommentar-Format:**
```markdown
🤖 **Multiterminal Agent Update**

Status: ✅ Aufgabe abgeschlossen
Branch: `issue/42-fix-login-bug`
Kosten: $0.45 (15.2k input, 3.8k output)
```

### 4.3 Konfigurierbar

```yaml
# ~/.multiterminal.yaml
issue_tracking:
  auto_comment_on_start: true    # Kommentar wenn Agent startet
  auto_comment_on_done: true     # Kommentar wenn Agent fertig
  auto_comment_on_close: true    # Kommentar wenn Pane geschlossen
  auto_close_issue: false        # Issue automatisch schließen (default: nein)
  include_cost_in_comment: true  # Kosten im Kommentar anzeigen
```

**Dateien:**
| Datei | Änderung | Geschätzt |
|---|---|---|
| `app_issue_progress.go` | Neues File: Progress-Reporting | ~100 LOC |
| `app_scan.go` | Activity-Change Hook einbauen | ~20 LOC |
| `app.go` | Issue-Number pro Session tracken | ~10 LOC |
| `config/config.go` | `IssueTracking` Config-Struct | ~15 LOC |

---

## Phase 5: UI-Polish & Workflow-Verbesserungen

### 5.1 Issue-Pane-Toolbar

Quick-Actions in der Pane-Titlebar für Issue-verknüpfte Panes:
- **"Commit & Push"** — staged changes committen mit `Closes #42` Message
- **"Issue schließen"** — Issue auf GitHub schließen
- **"Branch löschen"** — Cleanup nach Issue-Close
- **"PR erstellen"** — `gh pr create` mit Issue-Referenz

### 5.2 Sidebar: Issue-Board-Ansicht

Alternative zur Listenansicht: Kanban-ähnliches Mini-Board:
```
┌─────────┬──────────┬─────────┐
│  Open   │ In Work  │  Done   │
│  #44    │  #42 ●   │  #41 ✓  │
│  #45    │  #43 ●   │  #40 ✓  │
└─────────┴──────────┴─────────┘
```
"In Work" = Hat ein aktives Pane. Visuell sofort erkennbar.

### 5.3 Keyboard Shortcuts

| Shortcut | Aktion |
|---|---|
| Ctrl+I | Issues-Sidebar öffnen/fokussieren |
| Ctrl+Shift+I | Neues Issue erstellen |
| Enter (auf Issue in Sidebar) | Claude für Issue starten |

### 5.4 Notifications

Bestehende Notification-Infrastruktur (`app_notify.go`) nutzen:
- "Agent für #42 ist fertig!" → Desktop-Notification
- Klick auf Notification → Pane fokussieren

---

## Implementierungs-Reihenfolge

```
Phase 1: Issue ↔ Pane Binding          ← Kernfeature, Basis für alles
  ↓
Phase 2: Auto-Branch                    ← Natürliche Erweiterung
  ↓
Phase 4: Progress-Tracking              ← Macht Issue-Binding richtig nützlich
  ↓
Phase 5: UI-Polish                      ← Workflow-Optimierung
  ↓
Phase 3: Worktrees (optional)           ← Nur wenn Bedarf für parallele Issues
```

**Phase 3 (Worktrees) bewusst nach hinten:** Hohe Komplexität, Edge-Cases,
und viele User brauchen es nicht. Kann jederzeit nachgerüstet werden.

---

## Neue Dateien (Zusammenfassung)

| Datei | Phase | LOC | Verantwortung |
|---|---|---|---|
| `internal/backend/app_git_branch.go` | 2 | ~80 | Branch-Erstellen, Slugify |
| `internal/backend/app_worktree.go` | 3 | ~120 | Worktree-CRUD |
| `internal/backend/app_issue_progress.go` | 4 | ~100 | Auto-Comments auf Issues |

## Bestehende Dateien (Änderungen)

| Datei | Phase | Umfang |
|---|---|---|
| `frontend/src/stores/tabs.ts` | 1 | Pane-Interface + addPane |
| `frontend/src/components/IssuesView.svelte` | 1, 5 | Play-Button, Activity-Dots, Board |
| `frontend/src/components/LaunchDialog.svelte` | 1 | Issue-Kontext |
| `frontend/src/components/PaneTitlebar.svelte` | 1, 5 | Issue-Badge, Quick-Actions |
| `frontend/src/App.svelte` | 1, 2 | Launch-Flow, Branch-Integration |
| `internal/backend/app.go` | 1, 4 | Issue-Number pro Session |
| `internal/backend/app_scan.go` | 4 | Activity-Change Hook |
| `internal/config/config.go` | 2, 3, 4 | Neue Config-Optionen |
| `internal/config/session.go` | 1 | SavedPane erweitern |

---

## Abgrenzung zu ccpm

| ccpm | Multiterminal (unser Ansatz) |
|---|---|
| Prompt-Dateien in `.claude/commands/` | Native Go-Backend + Svelte-UI |
| PRD → Epic → Task Workflow | Direkt: GitHub Issue → Agent |
| Nur in Claude Code Slash-Commands | Eigene GUI mit visueller Orchestrierung |
| Kein UI, rein textbasiert | Kanban-Board, Activity-Dots, Notifications |
| Manueller Status-Report via Prompts | Automatisches Progress-Tracking |
| Git Worktrees als Kernkonzept | Worktrees optional, Branch-per-Issue default |

**Unser Vorteil:** Wir sehen Activity, Kosten, und Status live — und können
automatisch reagieren. ccpm muss den Agent bitten, seinen Status zu reporten.
