# Design: Worktree-pro-Pane mit Finish-Flow

**Datum:** 2026-07-02
**Status:** Entwurf (Red-Team-geprüft, 3 adversariale Reviews eingearbeitet)
**Branch-Ziel:** `alpha-main`

## 1. Ziel & Kontext

Jedes Terminal-Pane kann opt-in in einem eigenen, isolierten git-Worktree starten,
abgeleitet von einem Ziel-Branch. Claude Code arbeitet darin isoliert von anderen
Panes. Nach Abschluss führt MTUI einen deterministischen Finish-Flow aus:
Vorbereitung durch Claude (commit + rebase), Bestätigung durch den User,
ff-only-Merge in den Ziel-Branch, vollständiger Cleanup (Worktree + Branch weg,
Pane zurück ins Haupt-Repo).

**Rollenverteilung (Kernentscheidung):**
- **Claude bereitet vor** (committen, auf Ziel-Branch rebasen) — niemals selbst mergen.
- **MTUI führt aus** (Merge, Worktree-Remove, Branch-Delete) — deterministisch, skriptbar.
- **Der User bestätigt** einmal, mit Sicht auf Commits + Dateiliste.

Bestehende Infrastruktur, die wiederverwendet wird: `CreateNamedWorktree`-Logik
(`app_worktree.go`), Prompt-Queue (`app_queue.go`), Activity-Detection
(`activity.go`, Status `done`/`input`), Session-Persistenz (`config/session.go`),
`gitCmd` mit `hideConsole` (`app_git_cmd.go`).

## 2. User-Flow

1. **Launch:** User öffnet den LaunchDialog, aktiviert die Checkbox
   „⎇ Isolierter Worktree". Zwei Felder erscheinen: **Name** (vorbefüllt, z. B.
   `issue-42` bei Issue-Launch, sonst `pane-<N>`) und **Ziel-Branch** (vorbefüllt
   mit dem aktuellen Branch des Haupt-Worktrees). Der Checkbox-Zustand wird
   gemerkt (Config-Store).
2. **Arbeit:** Session läuft mit dem Worktree als CWD. Titlebar zeigt ein deutlich
   sichtbares ⎇-Badge mit Branch-Name. Claude kennt via `CLAUDE.local.md`
   dauerhaft Branch, Ziel-Branch und Regeln (überlebt `/clear`, Neustart).
3. **Finish:** User klickt ✓ in der Titlebar → MTUI schickt den Prep-Prompt über
   die Queue → Claude committet + rebased → MTUI prüft hart → Bestätigungs-Overlay
   (Ziel-Branch, Commit-Liste, Datei-Statistik) → „Mergen & Aufräumen" → Merge,
   Cleanup, Pane startet neu im Haupt-Repo.

## 3. Worktree-Erstellung

### 3.1 Lage: Sibling-Verzeichnis (Red-Team-Auflage)

Pane-Worktrees liegen **außerhalb** des Repos:

```
<repo-parent>/<repo-name>.mt-worktrees/<name>/
z. B. D:\repos\Multiterminal.mt-worktrees\login-fix\
```

Branch-Schema bleibt `terminal/<name>` (kompatibel zur bestehenden Kategorisierung).

Begründung (Red-Team, 2 Reviewer unabhängig): In-Repo-Worktrees unter
`.mt-worktrees/` vergiften `go build ./...`, Vite-/esbuild-Watcher und die
MTUI-Sidebar, und Claude-Sessions im Worktree laden die Haupt-Repo-`CLAUDE.md`
doppelt (Directory-Traversal stoppt nicht an der Repo-Grenze).
`categorizeWorktree`/`ListAllWorktrees` werden um das Sibling-Schema erweitert;
bestehende In-Repo-Worktrees (Issue/Dropdown) bleiben unterstützt.

### 3.2 Haupt-Root-Bestimmung (Red-Team-Auflage, kritisch)

`repoRoot()` via `git rev-parse --show-toplevel` liefert **in einem Worktree den
Worktree-Pfad** — das hätte verschachtelte Worktrees und falsche
Ziel-Branch-Vorbelegung erzeugt. Neue Funktion `mainRepoRoot(dir)`:

```
git rev-parse --git-common-dir   → <mainRoot>/.git  → parent = mainRoot
```

Alle Pfade (Worktree-Lage, Sibling-Verzeichnis) und die Ziel-Branch-Vorbelegung
(`aktueller Branch des Haupt-Worktrees`, nicht des CWD) beziehen sich auf
`mainRoot`.

### 3.3 `CreatePaneWorktree(dir, name, targetBranch)` (neu, Backend)

1. `mainRoot` bestimmen (3.2).
2. Worktree anlegen: `git worktree add -b terminal/<safeName> <siblingPath> <targetBranch>`
   (Name-Sanitizing wie `sanitizeWorktreeName`; existiert der Branch bereits → Fehler
   an den User, kein stilles Wiederverwenden).
3. `CLAUDE.local.md` in den Worktree schreiben (Inhalt: 3.4).
4. `CLAUDE.local.md` in `<mainRoot>/.git/info/exclude` eintragen (idempotent —
   nur wenn Zeile fehlt). `info/exclude` wird über alle Worktrees geteilt
   (verifiziert); damit ist die Datei weder in `git status` sichtbar noch wird
   sie von `git add -A` erfasst. (Red-Team-Auflage, kritisch: sonst bricht der
   Clean-Check dauerhaft und die Steuerdatei würde in den Ziel-Branch gemergt.)
5. `.claude/settings.local.json` in den Worktree schreiben mit
   `permissions.deny`-Regeln (3.5). `.claude/` ist per Konvention gitignored.
6. Rückgabe `PaneWorktreeInfo{Path, Branch, TargetBranch}`.

Alle git-Aufrufe laufen über `gitCmd` (enthält `hideConsole` — Pflicht laut
CLAUDE.md, vom Red-Team als bereits korrekt bestätigt).

### 3.4 `CLAUDE.local.md` (Kontext, kein Enforcement)

```markdown
# MTUI-Worktree

Du arbeitest in einem isolierten MTUI-Worktree.
- Branch: `terminal/<name>`
- Ziel-Branch: `<targetBranch>` (lokal — kein fetch nötig, kein Push in v1)

Regeln:
- Committe abgeschlossene Arbeit in nachvollziehbaren Commits.
  Committe KEINE Secrets, .env-Dateien oder Build-Artefakte.
- Merge NIEMALS selbst in `<targetBranch>`. Lösche NIEMALS diesen Worktree
  oder Branch. Beides macht MTUI über den ✓-Button.
- Wenn der User die Arbeit abschließen will: committe alle offenen Änderungen,
  rebase auf den lokalen `<targetBranch>` und weise den User dann auf den
  ✓-Button in der Pane-Titelleiste hin.
```

Keine natürlichsprachlichen Trigger-Wörter („merge"/„fertig") — Red-Team-Auflage:
False Positives und Erwartungslücke (User glaubt gemergt, Cleanup lief nie).
Einzige Abschluss-Aktion ist der ✓-Button.

### 3.5 Harte Verbote via `permissions.deny` (Red-Team-Auflage, kritisch)

`CLAUDE.local.md` ist laut offizieller Doku *Kontext, keine erzwungene
Konfiguration* — Claude kann die Regeln ignorieren oder die Datei sogar selbst
editieren. Die sicherheitsrelevanten Verbote werden deshalb zusätzlich als
Deny-Regeln in `.claude/settings.local.json` des Worktrees hinterlegt (vom
Claude-Code-Harness erzwungen), sinngemäß:

- `git merge …` (Selbst-Merge in den Ziel-Branch)
- `git worktree remove …`
- `git branch -D …`
- `git push --force …`

Exakte Patterns werden bei der Implementierung gegen das reale
Permission-Matching von Claude Code verifiziert (E2E-Testpunkt).

## 4. Tracking & Persistenz

### 4.1 Pane-State (Frontend)

`Pane` in `tabs.ts` erhält/nutzt: `worktreePath`, `worktreeBranch`,
`targetBranch` (neu). Titlebar zeigt bei gesetztem `worktreePath` das ⎇-Badge
und den ✓-Button.

### 4.2 Persistenz (Red-Team-Fund: existiert bisher NICHT)

- `SavedPane` (`internal/config/session.go`): Felder `worktree_path`,
  `worktree_branch`, `target_branch` ergänzen (yaml+json-Tags).
- `session.ts` `saveSession()`/`restoreSession()`: Felder mitschreiben/-lesen.
- **`models.ts` manuell syncen** (Klasse + Felddeklaration + Konstruktor) —
  bekannter Wiederholungsbug, ohne Sync werden die Felder still verworfen.

### 4.3 Finish-Zustand lebt im Backend (Red-Team-Auflage, kritisch)

Der Finish-Flow ist **keine** Frontend-Zustandsmaschine (stirbt bei Tab-Detach,
Reload, App-Neustart), sondern Backend-State pro Session (unter `App.mu`):

```go
type finishState struct {
    Phase        string // "", "preparing", "ready", "blocked", "merging", "merged", "cleanup"
    TargetBranch string
    WorktreePath string
    PrepItemID   int    // Queue-Item des Prep-Prompts
    BlockReason  string
}
```

Phasen `merged`/`cleanup` werden zusätzlich persistiert (Marker-Datei im
Config-Verzeichnis oder Session-JSON), damit ein Crash zwischen Merge und
Remove beim Retry **nur den Cleanup** wiederholt statt erneut zu mergen
(Idempotenz, Red-Team-Auflage). Das Frontend rendert ausschließlich Events:
`worktree:finish-ready`, `worktree:finish-blocked`, `worktree:finish-done`.

## 5. Finish-Flow

### 5.1 Ablauf

1. **`StartWorktreeFinish(sessionId)`** (Klick ✓): setzt Phase `preparing`,
   hängt den Prep-Prompt als **markiertes Queue-Item** an (`PrepItemID`).
   Prep-Prompt: *„Committe alle offenen Änderungen in nachvollziehbaren Commits
   (keine Secrets/.env/Build-Artefakte). Rebase dann `terminal/<name>` auf den
   lokalen `<targetBranch>`. Merge nicht selbst und pushe nicht."*
2. **Abschluss-Korrelation** (Red-Team-Auflage, kritisch): Der Status-Check
   läuft **nicht** beim nächsten generischen `done` (feuert nach jedem Turn und
   ggf. für frühere Queue-Prompts), sondern erst, wenn **genau das Prep-Item**
   den Queue-Status `done` erreicht (`processQueue` kennt Item-Abschluss).
   Meldet die Activity-Detection stattdessen `input` (Rückfrage/Permission),
   wird `worktree:finish-blocked` mit Grund „Claude hat eine Rückfrage" emittiert.
3. **`GetWorktreeFinishStatus`** prüft (alle gegen den **lokalen** Ziel-Ref,
   derselbe Ref für alle Checks — Red-Team-Auflage):
   - `git rev-list --count <target>..<branch>` **> 0** (0 Commits blockt hart —
     sonst würden Worktree+Branch trotz leerem Ergebnis gelöscht),
   - `git merge-base --is-ancestor <target> <branch>` (rebased?),
   - Worktree clean (`git status --porcelain` leer; `CLAUDE.local.md` ist via
     `info/exclude` unsichtbar),
   - **Haupt-Worktree clean** und **Ziel-Branch dort ausgecheckt** (sonst
     `blocked` mit klarer Meldung — ein ff-Merge bewegt Dateien im
     Haupt-Working-Tree).
   Ergebnis `ready` → Event mit Commit-Liste (`git log <target>..<branch>
   --oneline`) + `--stat`-Zusammenfassung fürs Overlay.
4. **Overlay** (Frontend): Ziel-Branch, Commits, Dateiliste → „Mergen & Aufräumen".
5. **`FinishWorktree(sessionId)`** (nach Bestätigung), serialisiert über einen
   globalen Finish-Mutex (Red-Team-Auflage: parallele Finishes = Kern-Use-Case,
   `index.lock`-Kollisionen und TOCTOU vermeiden):
   1. ff-Check **erneut** ausführen (Overlay-Status kann veraltet sein). Nicht
      mehr ff (Ziel hat sich bewegt, z. B. durch anderes Pane) → `blocked`
      „Erneut vorbereiten" (Flow springt zu Schritt 1 zurück).
   2. `gitCmd(mainRoot, "merge", "--ff-only", "terminal/<name>")` — Merge läuft
      **im Haupt-Worktree** (Red-Team-Auflage, kritisch: `git merge` wirkt nur
      auf den HEAD des aktuellen Worktrees; jede andere Variante hätte den
      Ziel-Branch nie aktualisiert und anschließend die Arbeit gelöscht).
   3. Phase `merged` persistieren.
   4. Session schließen **inkl. Prozessbaum** (5.2).
   5. `git worktree remove <path>` (ohne `--force` zuerst; bei Fehlschlag
      Retry mit Backoff, dann `--force` — Merge ist zu diesem Zeitpunkt
      verifiziert durch), `git worktree prune`.
   6. `gitCmd(mainRoot, "branch", "-d", "terminal/<name>")` — nur `-d`, **kein
      `-D`-Fallback** (Red-Team-Auflage: `-d` ist die letzte Sicherung gegen
      Datenverlust); erst nach erfolgreichem Remove (ausgecheckter Branch ist
      nicht löschbar).
   7. Finish-State löschen, `worktree:finish-done` emittieren → Frontend startet
      neue Session im Haupt-Repo (ohne Branch-Wechsel — der Haupt-Worktree
      bleibt auf dem Ziel-Branch, der dort ohnehin ausgecheckt ist).

### 5.2 Prozessbaum beenden (Red-Team-Auflage, kritisch, Windows)

`Session.Close()` killt heute nur den `cmd.exe /c claude`-Wrapper; Node-,
MCP-Server- und Watcher-Kindprozesse überleben und halten Handles im
Worktree-Verzeichnis → `git worktree remove` scheitert auf Windows.

Neu: `killProcessTree(pid)` — Windows: `taskkill /PID <pid> /T /F` (via
`hideConsole`!); Unix: Kill der Prozessgruppe. Aufgerufen im Finish-Flow vor
dem Remove; `worktree remove` mit Retry/Backoff (Windows gibt Handles verzögert
frei, ~5 Versuche, exponentiell bis ~3 s).

### 5.3 Fehlerfälle

| Fall | Verhalten |
|---|---|
| Rebase-Konflikt / dirty Worktree | `blocked` + Overlay mit Problem; zusätzlich erkennt MTUI einen laufenden Rebase (`.git/rebase-merge` vorhanden) und bietet „Rebase abbrechen" neben „Claude lösen lassen" an |
| Claude stellt Rückfrage während Prep | `blocked` „Claude hat eine Rückfrage" — User antwortet im Pane, klickt ✓ erneut |
| Haupt-Worktree dirty / falscher Branch | `blocked` mit klarer Meldung, kein Merge |
| Ziel bewegt (nicht mehr ff) | `blocked` „Erneut vorbereiten" → neuer Prep-Zyklus (rebased auf neuen Stand) |
| `worktree remove` scheitert nach Retries | Merge ist durch; Phase bleibt `cleanup`, Overlay bietet „Cleanup erneut versuchen" |
| Crash zwischen Merge und Remove | persistierte Phase `merged` → Retry führt nur Cleanup aus (kein zweiter Merge) |
| Pane wird mit aktivem Worktree geschlossen | Dialog: „Worktree behalten oder verwerfen?" — kein stilles Verwaisen |
| App-Neustart | Pane↔Worktree-Zuordnung kommt aus der Session-Persistenz (4.2); `ListAllWorktrees` verifiziert Existenz |

## 6. UI

- **LaunchDialog:** Checkbox „⎇ Isolierter Worktree" + Felder Name/Ziel-Branch.
  Sichtbar für Shell- und Claude-Launches, nicht für Chat-Panes.
- **PaneTitlebar:** ⎇-Badge (Branch-Name, deutlich sichtbar — Red-Team:
  Verwechslungsgefahr, wenn User die gemerkte Checkbox vergisst) + ✓-Button
  (Tooltip „Worktree fertigstellen: mergen & aufräumen"). ✓ erscheint für
  **jedes** Pane mit getracktem Worktree — auch für über das bestehende
  WorktreeDropdown erzeugte (fehlt `targetBranch`, fragt das Overlay ihn per
  Branch-Auswahl nach). Kein zweites, konkurrierendes Worktree-System.
- **WorktreeFinishDialog.svelte** (neu): Zustände ready (Commits + Stat +
  Bestätigen), blocked (Grund + Aktionen), cleanup-retry.
- **Shell-Panes:** ✓ löst den **mechanischen** Finish aus — MTUI führt selbst
  `git add -A && git commit` (mit Nachfrage-Dialog für die Message) und
  `git rebase <target>` aus statt eines Prompts; Rest des Flows identisch.
- **Chat-Panes:** kein ✓ (keine PTY/Queue).

## 7. Konfiguration

- Frontend-Config-Store: `worktree_launch_default` (letzter Checkbox-Zustand).
- Keine neuen YAML-Felder in v1 (Ziel-Branch kommt pro Launch aus dem Dialog).
  Falls doch Felder in `config.Config` landen: yaml+json-Tags **und**
  `models.ts`-Sync (bekannte Falle).

## 8. Nicht im Scope (v1)

- Push / PR-Erstellung (Abschluss-Aktion ist bewusst eine einzelne Stelle im
  Code — PR-Option ist eine spätere Config-Erweiterung).
- `git fetch` / Remote-Sync (v1 arbeitet rein lokal; Rebase-Ziel ist der lokale
  Ziel-Branch — Red-Team-Auflage: ein Ref für Ancestor-Check UND Merge).
- Konfigurierbare Schnellbutton-Leiste (eigenes Issue).
- Automatische Worktrees ohne Opt-in.

## 9. Implementierungsumfang

- **Backend neu:** `app_worktree_pane.go` (`CreatePaneWorktree`, `mainRepoRoot`,
  Steuerdateien) und `app_worktree_finish.go` (`StartWorktreeFinish`,
  `GetWorktreeFinishStatus`, `FinishWorktree`, finishState, Finish-Mutex,
  `killProcessTree` in `session_helpers`/`hide_windows`-Umfeld). Max 300 Zeilen
  pro Datei.
- **Backend erweitert:** Queue (markiertes Prep-Item + Abschluss-Callback),
  `SavedPane`, Worktree-Kategorisierung (Sibling-Schema).
- **Frontend:** LaunchDialog, PaneTitlebar (Badge + ✓), WorktreeFinishDialog,
  `tabs.ts`/`session.ts`-Felder, Event-Handling, `models.ts`-Sync.
- **Tests:** `go test` für `mainRepoRoot` (Worktree vs. Haupt-Repo),
  Status-Checks (0 Commits, nicht rebased, dirty), Finish-State-Übergänge,
  Idempotenz (merged→cleanup), Sibling-Kategorisierung; Frontend-Tests für
  Persistenz-Roundtrip. E2E-Flow (echtes git, echter Claude CLI) →
  `needs-e2e-testing`-Label bis real getestet.

## 10. Red-Team-Traceability

| # | Fund (Kurzform) | Antwort im Design |
|---|---|---|
| G-K1 | Merge-Ort undefiniert, Arbeit wäre verloren | 5.1 Schritt 2: Merge im Haupt-Worktree, Checks 5.1/3 |
| G-K2 | `--show-toplevel` in Worktrees falsch | 3.2 `mainRepoRoot` via `--git-common-dir` |
| G-K3 | `CLAUDE.local.md` bricht Clean-Check / wird committet | 3.3 Schritt 4: `info/exclude` |
| G-H1/H2 | Ziel-Ref lokal vs. origin; Ancestor-Edge-Cases | 5.1/3 + 8: ein lokaler Ref, 0-Commits blockt |
| G-H3 | Parallele Finishes (Lock, TOCTOU) | 5.1/5: Mutex + ff-Recheck vor Merge |
| G-M1/M2 | `-D`-Force, `--force`-Remove | 5.1/5–6: `-d` ohne Fallback, `--force` nur nach Merge |
| L-K1 | Finish-State nur im Frontend | 4.3 Backend-State + Events |
| L-K2 | Persistenz existiert nicht | 4.2 SavedPane + models.ts |
| L-K3 | Prozessbaum überlebt Close | 5.2 `killProcessTree` + Retry |
| L-K4 | done-Korrelation fehlt | 5.1/2: markiertes Prep-Queue-Item |
| L-H1 / U-H2 | In-Repo-Worktrees (Build/Watcher/doppelte CLAUDE.md) | 3.1 Sibling-Verzeichnis |
| L-M1 | Rückfrage lässt Flow hängen | 5.1/2 + 5.3: blocked bei `input` |
| L-M2 | Pane-Close verwaist Worktree | 5.3: Behalten/Verwerfen-Dialog |
| L-M3 | Crash Merge↔Remove nicht idempotent | 4.3 + 5.3: persistierte Phase `merged` |
| U-K1 | `CLAUDE.local.md` kein Enforcement | 3.5 `permissions.deny` |
| U-K3 | „committe alles" gefährlich | 5.1/1 präzisierter Prep-Prompt + Overlay-Review |
| U-H1 | generisches done verfrüht | 5.1/2 |
| U-H3 | zwei Worktree-Wege | 6: ✓ für jedes getrackte Worktree-Pane |
| U-H4 | Shell-Panes: Prompt ins Leere | 6: mechanischer Finish |
| U-M1 | Trigger-Wörter | 3.4 gestrichen |
| U-M2 | vergessene Checkbox | 6: deutliches Badge (Zustand bleibt gemerkt) |
| U-M3 | Rebase-Konflikt-UX | 5.3: „Rebase abbrechen"-Angebot |
| U-N2 | Overlay bei 0 Commits | 5.1/3: blockt hart |
| U-N3/G-M3 | `info/exclude` falsch adressiert | 3.3/4: adressiert `CLAUDE.local.md` |
