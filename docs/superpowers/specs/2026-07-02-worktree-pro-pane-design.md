# Design: Worktree-pro-Pane mit Finish-Flow

**Datum:** 2026-07-02
**Status:** Entwurf, Rev. 2 (zwei Red-Team-Runden: 3 + 4 adversariale Reviews eingearbeitet)
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

**Ehrliche Parallelitäts-Grenze (Red-Team R2):** Der Merge läuft zwingend im
Haupt-Worktree (5.4), und dort kann nur ein Branch ausgecheckt sein. Parallele
Panes können daher nur dann nacheinander durchfinishen, wenn sie denselben
Ziel-Branch haben (der Normalfall: mehrere Features → `alpha-main`). Panes mit
einem anderen Ziel-Branch bleiben `blocked`, bis der Ziel-Branch im
Haupt-Worktree ausgecheckt ist. Das wird in der Blocked-Meldung klar benannt.

Bestehende Infrastruktur, die wiederverwendet wird: `CreateNamedWorktree`-Logik
(`app_worktree.go`), Prompt-Queue (`app_queue.go`), Activity-Detection
(`activity.go`, Zustände `done`/`waitingPermission`/`waitingAnswer`/…),
Session-Persistenz (`config/session.go`), `gitCmd` mit `hideConsole`
(`app_git_cmd.go`).

## 2. User-Flow

1. **Launch:** User öffnet den LaunchDialog, aktiviert die Checkbox
   „⎇ Isolierter Worktree". Zwei Felder erscheinen: **Name** (vorbefüllt mit
   einem garantiert freien Namen, s. 3.3) und **Ziel-Branch** (vorbefüllt mit
   dem aktuell im Haupt-Worktree ausgecheckten Branch). Der Checkbox-Zustand
   wird gemerkt; ist er gemerkt-aktiv, sind die Felder beim Öffnen direkt
   sichtbar (präventiv statt Überraschung, Red-Team R2). Die Checkbox ist
   ausgeblendet (und wird zurückgesetzt), wenn der Darstellungsmodus „Chat"
   gewählt ist — Chat-Panes haben keine PTY/Queue und keinen Finish-Flow (v1).
2. **Arbeit:** Session läuft mit dem Worktree als CWD. Titlebar zeigt ein
   deutlich sichtbares ⎇-Badge mit Branch-Name. Claude kennt via
   `CLAUDE.local.md` dauerhaft Branch, Ziel-Branch und Regeln (überlebt
   `/clear`, Neustart).
3. **Finish:** User klickt ✓ in der Titlebar → MTUI schickt den Prep-Prompt über
   die Queue → Claude committet + rebased → MTUI prüft hart → Bestätigungs-Overlay
   (Ziel-Branch, Commit-Liste, Datei-Statistik) → „Mergen & Aufräumen" → Merge,
   Cleanup, Pane startet neu im Haupt-Repo.

## 3. Worktree-Erstellung

### 3.1 Lage: Sibling-Verzeichnis

Pane-Worktrees liegen **außerhalb** des Repos:

```
<repo-parent>/<repo-name>.mt-worktrees/<name>/
z. B. D:\repos\Multiterminal.mt-worktrees\login-fix\
```

Branch-Schema bleibt `terminal/<name>` (kompatibel zur bestehenden Kategorisierung).

Begründung (Red-Team R1, 2 Reviewer unabhängig): In-Repo-Worktrees unter
`.mt-worktrees/` vergiften `go build ./...`, Vite-/esbuild-Watcher und die
MTUI-Sidebar, und Claude-Sessions im Worktree laden die Haupt-Repo-`CLAUDE.md`
doppelt (Directory-Traversal stoppt nicht an der Repo-Grenze).

Randfälle (Red-Team R2):
- **Repo-Parent nicht schreibbar** (z. B. Repo unter `C:\Program Files\…`):
  `worktree add` scheitert → klare Fehlermeldung an den User; kein stiller
  Fallback in v1.
- **`categorizeWorktree`** erweitert das Sibling-Schema mit Prefix-Vergleich
  **inklusive abschließendem Separator** (wie Bestandscode Z. 183, sonst matcht
  `Foo.mt-worktrees-backup` fälschlich) und bezieht den Prefix auf `mainRoot`
  (3.2), nicht auf das CWD. Bestehende In-Repo-Worktrees (Issue/Dropdown)
  bleiben unterstützt.

### 3.2 Haupt-Root-Bestimmung (Red-Team R1+R2, kritisch)

`git rev-parse --show-toplevel` liefert in einem Worktree den Worktree-Pfad;
`git rev-parse --git-common-dir` liefert **im Haupt-Repo nur das relative
`.git`** (Red-Team R2, im Repo verifiziert) — beide sind als alleinige Quelle
unbrauchbar. Neue Funktion `mainRepoRoot(dir)`:

```
git -C <dir> worktree list --porcelain
→ erster Eintrag ist IMMER der Haupt-Worktree (git-Garantie)
→ mainRoot = Pfad des ersten Eintrags
```

Unabhängig von CWD (Haupt-Repo oder Worktree), relativen Pfaden und
`--separate-git-dir`. Alle Pfade (Sibling-Verzeichnis) und die
Ziel-Branch-Vorbelegung (aktueller Branch des **Haupt-Worktrees**) beziehen
sich auf `mainRoot`.

### 3.3 `CreatePaneWorktree(dir, name, targetBranch)` (neu, Backend)

1. `mainRoot` bestimmen (3.2).
2. **Freien Namen sicherstellen:** Der vorbefüllte Default wird gegen
   existierende Branches (`terminal/<name>`) und Verzeichnisse geprüft und
   automatisch hochgezählt (`login-fix`, `login-fix-2`, …) — sonst produziert
   der Default `pane-<N>` wiederkehrende Kollisionen mit liegengebliebenen
   Alt-Branches (Red-Team R2). Nur ein **manuell eingegebener** kollidierender
   Name ist ein harter Fehler an den User.
3. Edge Cases prüfen (Red-Team R2): `targetBranch` existiert lokal nicht →
   Fehler mit Meldung; Haupt-Worktree in detached HEAD → Ziel-Branch-Feld hat
   keinen Default, User muss wählen.
4. Worktree anlegen: `git worktree add -b terminal/<safeName> <siblingPath> <targetBranch>`
   (Name-Sanitizing wie `sanitizeWorktreeName`).
5. **Beide Steuerdateien** in `<mainRoot>/.git/info/exclude` eintragen:
   `CLAUDE.local.md` **und** `.claude/settings.local.json` (Red-Team R2: die
   Konvention „`.claude/` ist gitignored" gilt nur in diesem Repo — in fremden
   Nutzer-Repos wäre die Datei sonst permanent „dirty" und würde mitgemergt).
   `info/exclude` wird über alle Worktrees geteilt (verifiziert). Idempotenz:
   Eintrag nur, wenn die exakte getrimmte Zeile noch nicht existiert.
6. `CLAUDE.local.md` in den Worktree schreiben (Inhalt: 3.4).
7. `.claude/settings.local.json` in den Worktree schreiben mit
   `permissions.deny`-Regeln (3.5).
8. Rückgabe `PaneWorktreeInfo` (frontend-exponiert, json+yaml-Tags,
   **models.ts-Sync erforderlich** — bekannte Falle):

```go
type PaneWorktreeInfo struct {
    Path         string `json:"path" yaml:"path"`
    Branch       string `json:"branch" yaml:"branch"`
    TargetBranch string `json:"target_branch" yaml:"target_branch"`
}
```

Alle git-Aufrufe laufen über `gitCmd` (enthält `hideConsole` — Pflicht laut
CLAUDE.md, von beiden Red-Team-Runden als korrekt bestätigt).

### 3.4 `CLAUDE.local.md` (Kontext, kein Enforcement)

```markdown
# MTUI-Worktree

Du arbeitest in einem isolierten MTUI-Worktree.
- Branch: `terminal/<name>`
- Ziel-Branch: `<targetBranch>` (lokal — kein fetch nötig, kein Push in v1)

Diese Worktree-Regeln haben Vorrang vor der Git-Workflow-Sektion der
Projekt-CLAUDE.md (kein PR, kein Push, kein Branch-Wechsel aus diesem Worktree).

Regeln:
- Committe abgeschlossene Arbeit in nachvollziehbaren Commits.
  Committe KEINE Secrets, .env-Dateien oder Build-Artefakte — ergänze für
  solche Dateien .gitignore-Einträge oder lass sie untracked stehen.
- Merge NIEMALS selbst in `<targetBranch>`. Lösche NIEMALS diesen Worktree
  oder Branch. Beides macht MTUI über den ✓-Button.
- Bei Rebase-Konflikten: löse sie NICHT eigenmächtig — führe
  `git rebase --abort` aus und nenne die Konfliktdateien. Der User entscheidet.
- Wenn der User die Arbeit abschließen will: committe alle offenen Änderungen,
  rebase auf den lokalen `<targetBranch>` und weise den User dann auf den
  ✓-Button in der Pane-Titelleiste hin.
```

Der Vorrang-Satz ist nötig, weil die im Worktree ausgecheckte Projekt-CLAUDE.md
(z. B. „Feature-Branches → PR") gleichzeitig geladen wird (Red-Team R2).
Keine natürlichsprachlichen Trigger-Wörter — einzige Abschluss-Aktion ist der
✓-Button (Red-Team R1: False Positives, Erwartungslücke).

### 3.5 `permissions.deny` — Best-Effort-Backstop, nicht Primärschutz

`CLAUDE.local.md` ist laut Doku *Kontext, keine erzwungene Konfiguration*.
Zusätzlich schreibt MTUI Deny-Regeln in die `.claude/settings.local.json` des
Worktrees (Deny schlägt Allow jeder Ebene — verifiziert):

- `Bash(git merge *)` — blockt Selbst-Merges; blockt **nicht** `git merge-base`
  (Space erzwingt Wortgrenze, Red-Team R2 verifiziert) und leider auch
  `git merge --abort` (akzeptierter Kollateralschaden, Abort läuft über die
  Konflikt-Regel in 3.4 ohnehin vor dem Deny-Griff — bei Implementierung E2E
  verifizieren).
- `Bash(git worktree remove *)`, `Bash(git branch -D *)`, `Bash(git push *)`
- `Write`/`Edit` auf `CLAUDE.local.md` und `.claude/settings.local.json`
  (sonst kann Claude die eigenen Leitplanken editieren, Red-Team R2).

**Einordnung (Red-Team R2):** Bash-Deny ist Prefix-Matching und über
`sh -c "…"`, Skripte oder Variablen umgehbar. Die Regeln sind ein
Best-Effort-Backstop gegen versehentliche Aktionen. Der **echte Schutz** gegen
Datenverlust ist die Verifikation in 5.4 (ff-Recheck vor Merge, Merge im
Haupt-Worktree, `branch -d` ohne Force) — sie funktioniert auch, wenn alle
Deny-Regeln versagen. Exakte Patterns werden bei der Implementierung gegen das
reale Permission-Matching E2E verifiziert.

## 4. Tracking & Persistenz

### 4.1 Pane-State (Frontend)

`Pane` in `tabs.ts` hat bereits `worktreePath`/`worktreeBranch`; **neu**:
`targetBranch` (auch in `addPane` durchreichen). Titlebar zeigt bei gesetztem
`worktreePath` das ⎇-Badge und den ✓-Button.

### 4.2 Persistenz (Red-Team R1: existiert bisher NICHT; R2: Restore-CWD-Lücke)

- `SavedPane` (`internal/config/session.go`): Felder `worktree_path`,
  `worktree_branch`, `target_branch` ergänzen (yaml+json-Tags).
- `session.ts` `saveSession()`: Felder mitschreiben.
- `restoreSession()`: **Worktree-Panes starten mit `savedPane.worktree_path`
  als CWD**, nicht mit `savedTab.dir` (Red-Team R2, kritisch: der heutige
  Restore-Pfad kennt nur das Tab-Verzeichnis — die Session liefe sonst im
  Haupt-Repo, während Badge und Finish auf den Worktree zeigen). Felder an
  `addPane` durchreichen.
- **Fehlender Worktree beim Restore** (manuell gelöscht): vor dem Start
  `os.Stat` + Abgleich mit `ListAllWorktrees`; fehlt das Verzeichnis →
  `git worktree prune`, Pane ohne Worktree-Tracking im Haupt-Repo starten,
  User informieren (Red-Team R2).
- **`models.ts` manuell syncen** (Klasse + Felddeklaration + Konstruktor) —
  bekannter Wiederholungsbug, ohne Sync werden die Felder still verworfen.

### 4.3 Finish-Zustand lebt im Backend

Backend-State pro Session-ID (unter `App.mu`), das Frontend rendert nur Events:

```go
type finishState struct {
    Phase        string // siehe Übergangstabelle
    TargetBranch string
    WorktreePath string
    PrepItemID   int    // Queue-Item des Prep-Prompts
    BlockReason  string
    Mode         string // "claude" | "shell"
}
```

**Vollständige Übergangstabelle** (Red-Team R2: deklarierte, aber nie gesetzte
Phasen sind verboten):

| Von | Ereignis | Nach |
|---|---|---|
| `""` | `StartWorktreeFinish` (pending Queue leer) | `preparing` |
| `""` | `StartWorktreeFinish` (pending Items vorhanden) | `blocked` („Queue nicht leer" + Anzahl; Overlay bietet „Pending verwerfen & starten") |
| `preparing` | Prep-Item erreicht Queue-Status `done` → Status-Checks (5.3) grün | `ready` |
| `preparing` | Prep-Item `done` → Checks nicht grün | `blocked` (Grund) |
| `preparing` | Activity `waitingPermission`/`waitingAnswer` | bleibt `preparing`, Event `finish-blocked` (informativ: „Claude hat eine Rückfrage") — Korrelation läuft weiter |
| `preparing` | Prep-Item via `RemoveFromQueue`/`ClearQueue` entfernt | `""` + Event (Finish abgebrochen) |
| `preparing` | Timeout (kein Abschluss nach 10 min) | Event (Hinweis + Abbrechen-Angebot), Phase bleibt |
| `ready` | Bestätigung im Overlay | `merging` |
| `ready` / `blocked` / `preparing` | `CancelWorktreeFinish` | `""` (Prep-Item entfernen falls pending) |
| `blocked` | ✓ erneut | `preparing` (neuer Prep-Zyklus) |
| `merging` | ff-Merge erfolgreich | `merged` (**Marker persistiert, 4.4**) |
| `merging` | nicht mehr ff (Ziel bewegt) | `blocked` („Erneut vorbereiten") |
| `merged` | (unmittelbar) | `cleanup` |
| `cleanup` | remove + `branch -d` erfolgreich | `""` (Marker gelöscht) + `finish-done` |
| `cleanup` | remove scheitert nach Retries | bleibt `cleanup` (Overlay: „Cleanup erneut versuchen") |

Lebenszyklus-Hygiene (Red-Team R2): `CloseSession` löscht den finishState der
Session mit (wie `queues`/`prevActivity`); ein Session-Neustart im selben Pane
erzeugt eine neue Session-ID — ein alter finishState in den Phasen
`preparing`–`ready`/`blocked` wird dabei verworfen, `merged`/`cleanup` lebt
unabhängig davon über den Marker (4.4) weiter.

Während `preparing`–`cleanup` ist die Queue der Session für neue Items gesperrt.

### 4.4 Idempotenz-Marker (Red-Team R2, kritisch präzisiert)

Persistiert wird **nicht** im Session-JSON (das schreibt das Frontend bei jedem
`SaveTabs` komplett neu — ein Backend-Marker dort würde überschrieben, und
Session-IDs überleben den Neustart nicht). Stattdessen:

- Eigene Datei `~/.multiterminal-worktree-finish.json`, **rein
  backend-geschrieben**, gekeyed nach **absolutem Worktree-Pfad**:
  `{ "<abs-worktree-path>": { "phase": "merged", "target_branch": "…", "branch": "…" } }`
- Geschrieben unmittelbar nach erfolgreichem ff-Merge, gelöscht nach
  erfolgreichem Cleanup.
- **Beim App-Start:** Abgleich gegen `ListAllWorktrees` — für jeden noch
  existierenden Worktree mit Marker-Phase `merged`/`cleanup` wird der Cleanup
  wieder aufgenommen (nie erneut gemergt).

## 5. Finish-Flow

### 5.1 Ablauf (Claude-Panes)

1. **`StartWorktreeFinish(sessionId)`** (Klick ✓; No-op, wenn bereits eine
   Phase aktiv — Doppelklick-Schutz): prüft pending Queue (Übergangstabelle),
   setzt `preparing`, hängt den Prep-Prompt als markiertes Queue-Item an
   (`PrepItemID`). Der ✓-Button ist während aktiver Phase disabled; die
   Titlebar zeigt einen `preparing`-Indikator (Spinner + „Bereite Merge vor…"),
   mit Abbrechen-Aktion (`CancelWorktreeFinish`).
   Prep-Prompt: *„Committe alle offenen Änderungen in nachvollziehbaren
   Commits. Committe keine Secrets, .env-Dateien oder Build-Artefakte —
   ergänze für solche Dateien .gitignore-Einträge oder lass sie untracked und
   erwähne sie. Rebase dann `terminal/<name>` auf den lokalen `<targetBranch>`.
   Bei Rebase-Konflikten: nicht selbst lösen, `git rebase --abort` ausführen
   und die Konfliktdateien nennen. Merge nicht selbst, pushe nicht, erstelle
   keinen PR."*
2. **Abschluss-Korrelation (neu zu bauen — Red-Team R2: `processQueue` hat
   heute KEINEN Item-Abschluss-Callback):** `processQueue` erhält einen Hook —
   wenn das gerade auf `done` gesetzte Item die `PrepItemID` eines aktiven
   finishState ist, stößt es den Status-Check an. Generische `done`-Übergänge
   (nach jedem Turn, für frühere Items) lösen nichts aus. `RemoveFromQueue`/
   `ClearQueue` erkennen die `PrepItemID` und setzen den Finish zurück
   (Übergangstabelle). Zusätzlich pollt MTUI während `preparing` den
   Rebase-Zustand über das **per-Worktree-Gitdir**
   (`<mainRoot>/.git/worktrees/<name>/rebase-merge`, Red-Team R2 — nicht
   `.git/rebase-merge`), um hängende Rebases unabhängig vom Activity-Signal zu
   erkennen.
3. Checks grün → Event `worktree:finish-ready` mit Commit-Liste + Stat +
   Untracked-Liste → **Overlay** (Ziel-Branch, Commits, Dateien, ggf. Hinweis
   auf untracked Dateien) → „Mergen & Aufräumen" → 5.4.

### 5.2 Prozessbaum beenden (Red-Team R1+R2, kritisch, Windows)

`Session.Close()` killt nur den `cmd.exe /c claude`-Wrapper; Node-/MCP-/Watcher-
Kinder überleben und halten Handles im Worktree → `git worktree remove`
scheitert auf Windows. Präzisierte Anforderungen (R2):

- **`Session.Pid() int`** (neu): liest `cmd.Process.Pid` unter `s.mu` — ohne
  Getter kommt der Finish-Flow nicht an die PID.
- **Reihenfolge zwingend:** `killProcessTree(sess.Pid())` läuft **synchron
  VOR** `Session.Close()` — nach `Process.Kill()` ist der Wrapper tot und
  `taskkill /T` findet die verwaisten Enkel nicht mehr. Das async
  `CloseSession` (Goroutine) wird im Finish-Flow **nicht** verwendet;
  stattdessen synchroner Pfad: kill tree → Close → remove.
- `killProcessTree`: Windows `taskkill /PID <pid> /T /F` via `hideConsole`
  (Muster wie `gitCmd`), lebt neben `hide_windows.go` (Build-Tag) mit
  Unix-Gegenstück (Prozessgruppen-Kill). **Exit-Code ≠ 0 wird toleriert**
  (Baum bereits beendet → taskkill Exit 128).
- `git worktree remove` (zuerst ohne `--force`; nach Fehlschlag Retry mit
  Backoff bis ~3 s, dann `--force` — der Merge ist zu diesem Zeitpunkt
  verifiziert durch), danach `git worktree prune`.

### 5.3 Status-Checks (`GetWorktreeFinishStatus`)

Alle Checks gegen den **lokalen** Ziel-Ref, derselbe Ref für alle Checks und
den Merge:

1. **Schon gemergt?** `git merge-base --is-ancestor <branch> <target>`
   (Richtung branch→target!) und `rev-list --count <target>..<branch>` = 0 →
   Branch ist vollständig im Ziel enthalten → **kein Merge nötig**; Overlay
   bietet „Nur aufräumen" (Worktree + Branch entfernen). Deckt beide Fälle ab
   (Red-Team R2, kritisch): Crash nach Merge aber vor Marker **und**
   „nie Arbeit geleistet" — beide enden sicher im Cleanup statt im Deadlock;
   `branch -d` schlägt bei enthaltenem Branch nie fehl.
2. Sonst: `rev-list --count <target>..<branch>` > 0 **und**
   `git merge-base --is-ancestor <target> <branch>` (rebased?).
3. **Worktree clean für getrackte Dateien:**
   `git status --porcelain --untracked-files=no` leer. Untracked Dateien
   blocken **nicht** (Red-Team R2: der Prep-Prompt verbietet Artefakt-Commits —
   untracked-blockend wäre ein Deadlock), werden aber im Overlay gelistet und
   gehen beim Cleanup mit dem Worktree verloren (Hinweis im Overlay).
4. **Haupt-Worktree:** clean (getrackte Dateien, inkl. staged) **und**
   Ziel-Branch dort ausgecheckt — sonst `blocked` mit klarer Meldung inkl. des
   Parallelitäts-Hinweises aus §1 (ein ff-Merge bewegt Dateien im
   Haupt-Working-Tree; laufende Sessions dort sehen danach den neuen Stand —
   git selbst verweigert das Überschreiben lokal modifizierter Dateien).

### 5.4 Merge & Cleanup (`FinishWorktree`)

Läuft in einer **Backend-Goroutine** (Retry/Backoff darf den Wails-Binding-Call
und den Finish-Mutex nicht minutenlang blockieren — Binding kehrt sofort
zurück, Abschluss kommt per Event), serialisiert über einen globalen
Finish-Mutex (parallele Finishes, `index.lock`, TOCTOU):

1. ff-Check **erneut** (Overlay-Status kann veraltet sein). Nicht mehr ff →
   `blocked` „Erneut vorbereiten".
2. `gitCmd(mainRoot, "merge", "--ff-only", "terminal/<name>")` — Merge läuft
   **im Haupt-Worktree** (`git merge` wirkt nur auf den HEAD des aktuellen
   Worktrees; jede andere Variante hätte den Ziel-Branch nie aktualisiert und
   anschließend die Arbeit gelöscht — Red-Team R1, kritisch).
3. Marker `merged` persistieren (4.4), Phase → `cleanup`.
4. Prozessbaum + Session beenden (5.2), Worktree entfernen (Retry), prune.
5. `gitCmd(mainRoot, "branch", "-d", "terminal/<name>")` — nur `-d`, **kein
   `-D`-Fallback**; erst nach erfolgreichem Remove. Schlägt `-d` trotz Merge
   fehl (Ziel zwischenzeitlich verschoben o. ä.): Branch bleibt stehen,
   Meldung „manuell prüfen" — weiterhin kein Force.
6. Marker löschen, finishState löschen, `worktree:finish-done` → Frontend
   startet neue Session im Haupt-Repo (gleicher Modus wie zuvor: shell/claude;
   ohne Branch-Wechsel — der Haupt-Worktree steht bereits auf dem Ziel-Branch).
   Pending-Queue-Reste der alten Session sind mit dem Overlay-Schritt bereits
   behandelt (Übergangstabelle: Start nur bei leerer pending Queue).

### 5.5 Shell-Panes: mechanischer Finish

Shell-Panes haben keinen Claude — MTUI übernimmt die Prep-Schritte selbst,
aber **ohne blindes `git add -A`** (Red-Team R2, beide Reviewer: staged sonst
genau die Artefakte/Secrets, die der Claude-Pfad ausschließt):

1. ✓ öffnet einen **Staging-Review-Dialog** (Wiederverwendung des
   CommitPushDialog-Musters): Liste der geänderten + untracked Dateien mit
   Abwahl, Commit-Message-Feld. `.gitignore` wird respektiert.
2. Nach Commit führt MTUI `git rebase <target>` im Worktree aus. Konflikt →
   `blocked` mit zwei Aktionen: „Rebase abbrechen" (`git rebase --abort`) oder
   „im Terminal auflösen, dann ✓ erneut" (kein Claude verfügbar — Red-Team R2).
3. Race-Schutz: der mechanische git-Lauf startet nur, wenn die Shell idle ist
   (keine laufende Vordergrund-Kommando-Ausgabe); `index.lock`-Kollisionen
   (User tippt parallel git-Befehle) werden als `blocked` gemeldet, nicht
   retried.
4. Ab Status-Check identisch zu 5.3/5.4.

Der ✓-Button hat damit zwei Einstiege (Prompt vs. Dialog), aber **eine**
Abschluss-Semantik; der Overlay-Text benennt den Modus.

### 5.6 Fehlerfälle

| Fall | Verhalten |
|---|---|
| Rebase-Konflikt (Claude-Pane) | Claude bricht laut Prep-Prompt ab (`rebase --abort`) und nennt Konfliktdateien → `blocked`; Overlay bietet „Claude lösen lassen" (dedizierter Konflikt-Prompt) oder „Abbrechen" |
| Rebase hängt (weder done noch Rückfrage) | `rebase-merge`-Polling im per-Worktree-Gitdir erkennt es; Timeout-Hinweis nach 10 min mit Abbrechen-Angebot |
| Claude stellt Rückfrage während Prep | Event „Claude hat eine Rückfrage" — User antwortet im Pane, Korrelation läuft weiter |
| Haupt-Worktree dirty / falscher Branch | `blocked` mit klarer Meldung (inkl. Parallelitäts-Hinweis §1), kein Merge |
| Ziel bewegt (nicht mehr ff) | `blocked` „Erneut vorbereiten" → neuer Prep-Zyklus |
| Branch schon vollständig im Ziel (0 neue Commits) | Overlay „Nur aufräumen" — kein Merge, sicherer Cleanup (deckt Crash-nach-Merge-vor-Marker ab) |
| `worktree remove` scheitert nach Retries | Merge ist durch; Phase bleibt `cleanup`, Overlay „Cleanup erneut versuchen" |
| Crash zwischen Merge und Cleanup | Marker (4.4, pfad-gekeyed) → App-Start nimmt Cleanup wieder auf, mergt nie erneut |
| User löscht Prep-Item / leert Queue | Finish wird zurückgesetzt (Event), kein hängender Zustand |
| Pane wird mit aktivem Worktree geschlossen | Dialog: „Worktree behalten oder verwerfen?" — kein stilles Verwaisen; finishState wird mit der Session aufgeräumt |
| Worktree-Verzeichnis manuell gelöscht | Restore erkennt es (4.2): prune, Start im Haupt-Repo, Meldung |
| App-Neustart | Pane↔Worktree aus Persistenz (4.2, CWD = worktree_path!); Marker-Abgleich (4.4) |

## 6. UI & Events

- **LaunchDialog:** Checkbox „⎇ Isolierter Worktree" + Felder Name/Ziel-Branch;
  bei gemerkt-aktiver Checkbox direkt aufgeklappt; ausgeblendet bei
  Darstellungsmodus „Chat".
- **PaneTitlebar:** ⎇-Badge (deutlich sichtbar) + ✓-Button (Tooltip „Worktree
  fertigstellen: mergen & aufräumen"; disabled während aktiver Phase;
  `preparing`-Spinner). ✓ erscheint für **jedes** Pane mit getracktem Worktree —
  auch für über das bestehende WorktreeDropdown erzeugte; fehlt dort
  `targetBranch`, schlägt das Overlay einen Ziel-Branch vor (Ableitung via
  `git merge-base --fork-point` bzw. Reflog als **Vorschlag** mit
  Branch-Picker-Override — fork-point ist reflog-abhängig, nie Automatik).
- **WorktreeFinishDialog.svelte** (neu): Zustände ready (Commits + Stat +
  Untracked-Hinweis + Bestätigen), blocked (Grund + Aktionen), cleanup-retry,
  „Nur aufräumen".
- **Events** (Payload-Structs in `app_events.go`, json-Tags, models.ts-Sync;
  Emission ist Broadcast an alle Fenster — **jede Payload trägt `sessionId`**,
  das Frontend filtert per Pane-Ownership wie bei `terminal:activity`,
  Red-Team R2 / Multi-Window):

```go
type WorktreeFinishReadyEvent struct {
    SessionID int      `json:"sessionId"`
    TargetBranch string `json:"targetBranch"`
    Commits   []string `json:"commits"`
    Stat      string   `json:"stat"`
    Untracked []string `json:"untracked"`
    CleanupOnly bool   `json:"cleanupOnly"` // 0 neue Commits
}
type WorktreeFinishBlockedEvent struct {
    SessionID int    `json:"sessionId"`
    Phase     string `json:"phase"`
    Reason    string `json:"reason"`
}
type WorktreeFinishDoneEvent struct {
    SessionID int    `json:"sessionId"`
    MainRoot  string `json:"mainRoot"`
    TargetBranch string `json:"targetBranch"`
    Mode      string `json:"mode"` // Relaunch-Modus: shell|claude
}
```

- **Sidebar/git-Polling:** Bei fokussiertem Worktree-Pane zeigt die Sidebar den
  Worktree-Pfad (Implementierung verifiziert den heutigen Anzeige-Pfad —
  Red-Team R2, UNSICHER-Fund).

## 7. Konfiguration

- Frontend-Config-Store: `worktree_launch_default` (letzter Checkbox-Zustand).
- Keine neuen YAML-Felder in v1. Falls doch Felder in `config.Config` landen:
  yaml+json-Tags **und** `models.ts`-Sync.

## 8. Nicht im Scope (v1)

- Push / PR-Erstellung (Abschluss-Aktion bleibt eine einzelne Code-Stelle —
  PR-Option ist eine spätere Config-Erweiterung).
- `git fetch` / Remote-Sync (rein lokal; ein Ref für alle Checks + Merge).
- Paralleles Finishen auf **unterschiedliche** Ziel-Branches (strukturelle
  Grenze, §1).
- Konfigurierbarer Worktree-Basis-Pfad (Fallback für nicht schreibbare Parents).
- Konfigurierbare Schnellbutton-Leiste (eigenes Issue).
- Finish-Flow für Chat-Panes.
- Automatische Worktrees ohne Opt-in.

## 9. Implementierungsumfang

- **Backend neu:** `app_worktree_pane.go` (`CreatePaneWorktree`, `mainRepoRoot`,
  Steuerdateien, Namens-Findung), `app_worktree_finish.go` (finishState,
  Übergänge, `StartWorktreeFinish`, `CancelWorktreeFinish`, `FinishWorktree`,
  Mutex, Marker) und `app_worktree_finish_status.go` (`GetWorktreeFinishStatus`,
  Checks, Commit-Liste/Stat — Split wegen 300-Zeilen-Regel, Red-Team R2).
  `killProcessTree` neben `hide_windows.go` (Build-Tags, Unix-Gegenstück).
- **Backend erweitert:** `app_queue.go` (Item-Done-Hook, PrepItem-Erkennung in
  Remove/Clear, Queue-Sperre), `app_scan.go` (waiting*-Weiterleitung bei
  aktivem Finish), `app_events.go` (Payload-Structs), `terminal/session.go`
  (`Pid()`), `config/session.go` (`SavedPane`-Felder), `CloseSession`
  (finishState-Cleanup), `app_worktree.go` (Sibling-Kategorisierung).
- **Frontend:** LaunchDialog (Checkbox/Felder/Chat-Ausblendung), PaneTitlebar
  (Badge, ✓, preparing-Indikator), WorktreeFinishDialog, Staging-Review für
  Shell-Finish, `tabs.ts` (`targetBranch`, addPane), `session.ts`
  (save/restore inkl. **CWD = worktree_path**), Event-Filterung per sessionId,
  **models.ts-Sync** (PaneWorktreeInfo, SavedPane-Felder, Event-Payloads).
- **Tests:** `mainRepoRoot` (Haupt-Repo relativ!, Worktree, CWD-Varianten),
  Status-Checks (schon-gemergt-Erkennung, 0 Commits, nicht rebased, dirty
  tracked vs. untracked), Übergangstabelle vollständig (jeder Übergang, inkl.
  Remove/Clear des Prep-Items), **Prep-Item-Korrelation ignoriert frühere
  done-Transitions** (Red-Team R2, sicherheitskritisch), Marker-Idempotenz
  (merged→nur Cleanup), Namens-Hochzählung, Sibling-Kategorisierung
  (Separator-Grenze), gemischte Sibling+In-Repo-Worktrees; Frontend:
  Persistenz-Roundtrip inkl. Restore-CWD. E2E (echtes git, echter Claude CLI,
  Deny-Pattern-Verifikation) → `needs-e2e-testing`-Label bis real getestet.

## 10. Red-Team-Traceability

### Runde 1 (Design-Entwurf, 3 Reviewer)

| # | Fund (Kurzform) | Antwort im Design |
|---|---|---|
| G-K1 | Merge-Ort undefiniert, Arbeit wäre verloren | 5.4 Schritt 2: Merge im Haupt-Worktree |
| G-K2 | `--show-toplevel` in Worktrees falsch | 3.2 (in R2 nochmals korrigiert) |
| G-K3 | `CLAUDE.local.md` bricht Clean-Check / wird committet | 3.3 Schritt 5: `info/exclude` |
| G-H1/H2 | Ziel-Ref lokal vs. origin; Ancestor-Edge-Cases | 5.3 + 8: ein lokaler Ref |
| G-H3 | Parallele Finishes (Lock, TOCTOU) | 5.4: Mutex + ff-Recheck |
| G-M1/M2 | `-D`-Force, `--force`-Remove | 5.4/5, 5.2: `-d` ohne Fallback |
| L-K1 | Finish-State nur im Frontend | 4.3 Backend-State + Events |
| L-K2 | Persistenz existiert nicht | 4.2 |
| L-K3 | Prozessbaum überlebt Close | 5.2 |
| L-K4 | done-Korrelation fehlt | 5.1/2 markiertes Prep-Item |
| L-H1 / U-H2 | In-Repo-Worktrees | 3.1 Sibling |
| L-M1 | Rückfrage lässt Flow hängen | Übergangstabelle + 5.6 |
| L-M2 | Pane-Close verwaist Worktree | 5.6 Dialog |
| L-M3 | Crash Merge↔Remove | 4.4 Marker |
| U-K1 | `CLAUDE.local.md` kein Enforcement | 3.5 deny (als Backstop eingeordnet) |
| U-K3 | „committe alles" gefährlich | 5.1/1 Prep-Prompt + Overlay |
| U-H1 | generisches done verfrüht | 5.1/2 |
| U-H3 | zwei Worktree-Wege | 6: ✓ für jedes getrackte Pane |
| U-H4 | Shell-Prompt ins Leere | 5.5 mechanischer Finish |
| U-M1 | Trigger-Wörter | 3.4 gestrichen |
| U-M2 | vergessene Checkbox | 2 + 6: aufgeklappte Felder + Badge |
| U-M3 | Rebase-Konflikt-UX | 5.6 |
| U-N2 | 0 Commits | 5.3/1 „Nur aufräumen" |

### Runde 2 (Spec-Review, 4 frische Reviewer)

| # | Fund (Kurzform) | Antwort in Rev. 2 |
|---|---|---|
| G2-K1 | `--git-common-dir` im Haupt-Repo relativ → mainRoot falsch | 3.2: erster Eintrag von `worktree list --porcelain` |
| G2-K2 | Merge-ohne-Marker-Crash → 0-Commit-Deadlock | 5.3/1: Schon-gemergt-Erkennung → „Nur aufräumen" |
| G2-H1 / C2-M5 | Parallelität nur bei gemeinsamem Ziel-Branch | §1 ehrlich dokumentiert + 5.3/4 Meldung + §8 |
| G2-H2 / U2-H2 | Shell-Finish blindes `git add -A` | 5.5: Staging-Review-Dialog |
| G2-H3 | Shell-Rebase-Konflikt ohne Löser; falscher rebase-merge-Pfad | 5.5/2 + 5.1/2 per-Worktree-Gitdir |
| G2-M1 | info/exclude-Idempotenz unpräzise | 3.3/5 exakte Zeile |
| G2-M2 / U2-K2 | settings.local.json fehlt in exclude | 3.3/5 beide Steuerdateien |
| G2-M3 | Sibling: Parent nicht schreibbar; Kategorisierungs-Prefix | 3.1 Randfälle |
| G2-M4 | worktree add Edge Cases (fehlender Ziel-Branch, detached HEAD) | 3.3/3 |
| G2-N1/N2 | ff unter laufender Session; `branch -d`-Restfall | 5.3/4 + 5.4/5 |
| L2-K1 / C2-H3 | Kill-Reihenfolge, fehlende PID, async CloseSession | 5.2 |
| L2-K2 / C2-M6 | Queue hat keinen Done-Callback; Remove/Clear; Timeout | 5.1/2 + Übergangstabelle + Test |
| L2-K3 | Restore-CWD falsch | 4.2 CWD = worktree_path |
| L2-K4 / C2-K1 | Marker im Session-JSON überschrieben; Keying | 4.4 pfad-gekeyte Backend-Datei |
| L2-H1 | finishState-Leak (Close/Neustart) | 4.3 Lebenszyklus-Hygiene |
| L2-H2 / C2-M1 | „input" existiert nicht; waiting* nicht verdrahtet | Übergangstabelle + 9 (app_scan.go) |
| L2-H3 | Shell-git-Races | 5.5/3 |
| L2-M1 | Events broadcasten an alle Fenster | 6: sessionId in jeder Payload |
| L2-M2 | Worktree manuell gelöscht | 4.2 |
| L2-M3 | taskkill-Exit bei totem Baum | 5.2 |
| L2-M4 | Backoff blockiert Binding | 5.4 Goroutine + Events |
| L2-M5 | Sidebar im Worktree | 6 (Verifikations-Auftrag) |
| C2-H1 | tote Phasen merging/cleanup | 4.3 Übergangstabelle |
| C2-H2 | kein Timeout/Cancel | Übergangstabelle + `CancelWorktreeFinish` |
| C2-H4 | Event-Payloads unspezifiziert | 6 Structs + app_events.go |
| C2-M2 | Queue-Rest nach Finish | Übergangstabelle (Start nur bei leerer Queue) + Sperre |
| C2-M3 | PaneWorktreeInfo undefiniert | 3.3/8 Struct + models.ts |
| C2-M4 | Traceability-Referenz falsch | korrigiert (G-K1 → 5.4/2) |
| C2-M7 | 300-Zeilen-Regel | 9: Status-Datei-Split |
| U2-K1 | Untracked-Artefakt-Deadlock | 5.3/3 tracked-only Clean-Check |
| U2-H1 | Checkbox vs. Chat-Display-Modus | 2 + 6 |
| U2-H3 | pane-N-Namenskollision | 3.3/2 Hochzählen |
| U2-H4 | ✓ bei voller Queue | Übergangstabelle (blocked + „Pending verwerfen") |
| U2-M1 | CLAUDE.md-Widerspruch | 3.4 Vorrang-Satz |
| U2-M2 | deny bypassbar / editierbar | 3.5 Backstop-Einordnung + Write/Edit-Deny |
| U2-M3 | preparing ohne UI / Doppelklick | 5.1/1 |
| U2-M4 | Konflikt-Verhalten unspezifiziert | 3.4 + 5.1/1 abort-Anweisung + Polling |
| U2-M5 | targetBranch-Picker für Alt-Worktrees | 6 fork-point-Vorschlag |
| U2-N2 | Checkbox reaktiv statt präventiv | 2 aufgeklappte Felder |
