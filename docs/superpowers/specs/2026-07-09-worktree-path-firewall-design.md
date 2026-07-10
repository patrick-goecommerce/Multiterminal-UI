# Design: PreToolUse-Pfad-Firewall gegen Schreiben außerhalb des aktiven Worktrees

**Datum:** 2026-07-09
**Status:** Entwurf (Brainstorming abgeschlossen, wartet auf Freigabe der Spec)
**Baut auf:** `docs/superpowers/specs/2026-07-03-worktree-enterworktree-design.md` (Erkennung von Claude-Codes nativem `EnterWorktree` per Hook) und der seither ergänzten `worktreeDenyRules` in `internal/backend/app_worktree_setup.go` (Bash-Deny für `git checkout`/`git switch`).

## 1. Anlass

In einer separaten Claude-Code-Session (nicht MTUI selbst, sondern eine Session, die AN Multiterminal arbeitete) trat folgendes Muster auf: Nach `EnterWorktree` wechselte nur der Bash-Arbeitsverzeichnis-Kontext in den neuen Worktree. Die Datei-Tools (`Read`/`Write`/`Edit`, die laut ihrer eigenen Spezifikation immer absolute Pfade verlangen) wurden weiterhin mit bereits im Kontext vorhandenen, auf das Hauptrepo zeigenden Pfaden aufgerufen. Ergebnis: mehrere Datei-Änderungen landeten im Hauptrepo-Checkout (Branch `alpha-main`) statt im isolierten Worktree — unbemerkt, bis ein Test-Lauf im Worktree leere/falsche Ergebnisse zeigte.

Die bestehende `worktreeDenyRules`-Absicherung (`Bash(git checkout *)`, `Bash(git switch *)`) griff in der konkreten Situation zufällig beim Versuch, das Hauptrepo per `git checkout --` zurückzusetzen — verhinderte aber nicht das eigentliche Problem: die fehlgeleiteten Schreibzugriffe selbst. Diese Lücke besteht unabhängig davon und beträfe jede MTUI-Nutzerin/jeden MTUI-Nutzer, die/der Claude Code mit `EnterWorktree` in einer Pane einsetzt.

## 2. Ziel

Ein Schreibversuch (`Edit`/`Write`/`NotebookEdit`) auf einen Pfad, der innerhalb des Haupt-Repo-Checkouts liegt, während für die Session ein anderer Worktree aktiv/erwartet ist, wird **aktiv blockiert** (nicht nur geloggt) — für alle MTUI-Nutzer, ohne Konfiguration, sofort nach Rollout wirksam (Nutzerentscheidung aus dem Brainstorming: hartes Blocken von Anfang an, kein Opt-in).

`Read`-Zugriffe auf das Hauptrepo bleiben ausdrücklich erlaubt — Recherche/Kontext-Lesen außerhalb des Worktrees ist normales, notwendiges Verhalten (das war z. B. beim Auslöser-Vorfall selbst der Fall: die Session hatte vor `EnterWorktree` legitim im Hauptrepo recherchiert).

## 3. Warum die Entscheidung nicht in der Wails-App fallen kann

`cmd/mtui-hook` ist ein kurzlebiger Subprozess: Claude Code startet für **jedes** Hook-Event einen frischen Prozess (`main.go`, kein Daemon). Er hat keinen Zugriff auf den In-Memory-State (`worktreeState`) der laufenden Wails-App. Die App selbst erfährt Events nur asynchron per Directory-Polling (`HookManager.processDirectory`, `app_hooks.go`) — für eine Blockier-Entscheidung *während* der Tool-Call noch läuft, ist das zu langsam und zu lose gekoppelt.

Die Entscheidung muss deshalb **vollständig innerhalb des `mtui-hook`-Prozesses** fallen, mit eigenem, dateibasiertem State — ohne Umweg über die Wails-App.

## 4. Mechanismus

### 4.1 Wie der „erwartete Worktree" pro Session bekannt wird

Zwei unabhängige Quellen, die beide in dieselbe Sidecar-Datei schreiben:

**a) MTUI-eigene Issue-Worktree-Panes** (`internal/backend/app_worktree_pane.go`): Der `mainRepoRoot(dir)`-Helfer wird beim Pane-Start bereits aufgerufen (bestehender Code). Zusätzlich zur bereits vorhandenen `MULTITERMINAL_SESSION_ID`-Env-Var (`app.go:218`) wird beim PTY-Start einer Worktree-Pane eine neue Env-Var gesetzt:
```
MULTITERMINAL_WORKTREE_PATH=<worktree-pfad>
MULTITERMINAL_MAIN_REPO_ROOT=<hauptrepo-root>
```
`mtui-hook` liest diese beiden Variablen synchron aus der eigenen Prozessumgebung (kein Datei-I/O nötig) — ab dem ersten Hook-Event der Pane aktiv.

**b) Claude Codes natives `EnterWorktree` (der Auslöser-Fall):** Bei `PostToolUse:EnterWorktree` liefert `tool_response.worktreePath` (bereits heute geparst, `cmd/mtui-hook/main.go:82-88`). `mtui-hook` ruft **einmalig, nur bei diesem Event** `git worktree list --porcelain` im neuen Worktree-Pfad auf (Wiederverwendung derselben Logik wie `mainRepoRoot()`, aber eigenständig implementiert in `cmd/mtui-hook`, da dieses Binary nicht von `internal/backend` importiert — separates `main`-Package) und schreibt beides in eine neue Sidecar-Datei:
```
%APPDATA%\Multiterminal\hooks\<session_id>.worktree.json
{"worktreePath": "...", "mainRepoRoot": "..."}
```
Bei `PostToolUse:ExitWorktree` (heute schon als Fall „`worktree:cleared`" erkannt) wird die Sidecar-Datei gelöscht — die Beschränkung endet damit für die Session.

Beide Quellen laufen in **derselben** Prüf-Logik zusammen: Die Sidecar-Datei hat Vorrang (falls vorhanden und lesbar) — sie spiegelt eine explizite, mid-session ausgelöste `EnterWorktree`-Änderung wider und ist damit das frischere Signal; ist sie nicht vorhanden/lesbar/parsbar, wird die Env-Var als Fallback verwendet. **Korrektur (2026-07-10, finales Review):** Eine frühere Fassung sah die Env-Var als vorrangig vor — das erzeugte einen False-Positive-Block, wenn eine bereits in Worktree A isolierte Pane per verschachteltem `EnterWorktree` in einen Geschwister-Worktree B wechselte (die Env-Var zeigte weiter auf A, obwohl die Sidecar korrekt B meldete). **Fail-open:** Fehlt die Sidecar-Datei, ist sie nicht lesbar oder nicht parsbar, und ist auch keine Env-Var gesetzt, gilt das wie „kein Kontext aktiv" — keine Prüfung, Tool läuft normal weiter. Konsistent mit der bestehenden Hook-Philosophie (`main.go:12-13`: „All failures are silent").

### 4.2 Die eigentliche Prüfung (`PreToolUse`)

`PreToolUse` ist bereits heute ohne Tool-Matcher für alle Tools registriert (`app_hooks_installer.go:16-20` — keine neue Hook-Registrierung nötig). `cmd/mtui-hook/main.go` wird erweitert:

1. Neues Feld `ToolInput json.RawMessage` in `claudeEvent` (`tool_input` aus dem Claude-Code-Hook-Payload).
2. Nur bei `eventType == "PreToolUse"` und `ev.ToolName` in `{"Edit", "Write", "NotebookEdit"}`: den Pfad aus `ToolInput` parsen — Feldname ist **nicht** einheitlich: `Edit`/`Write` nutzen `file_path`, `NotebookEdit` nutzt `notebook_path`. Beide Feldnamen müssen im Hook geprüft werden (erst `file_path`, dann `notebook_path` als Fallback).
3. Erwarteten Worktree-Kontext ermitteln (4.1). Kein Kontext aktiv → keine Prüfung, Tool läuft normal weiter (Standardverhalten für Panes ohne Worktree-Bezug bleibt exakt wie heute).
4. Ist ein Kontext aktiv: Pfad-Präfix-Vergleich (`filepath.Clean` + case-insensitive auf Windows).
   - Pfad liegt unter `worktreePath` → erlaubt.
   - Pfad liegt unter `mainRepoRoot`, aber NICHT unter `worktreePath` → **blockieren**.
   - Pfad liegt unter keinem von beiden (z. B. Scratchpad-Verzeichnis, ein anderes Projekt) → erlaubt (das ist nicht der gefährliche Fall, den dieses Design abdeckt).
5. Blockieren = **Exit-Code 0 + JSON auf stdout** (verifiziert über den `claude-code-guide`-Agenten gegen die aktuelle Hooks-Referenz — nicht Exit-Code 2/stderr, wie in einer früheren Fassung dieser Spec angenommen):
   ```json
   {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Pfad liegt im Hauptrepo (<mainRepoRoot>), nicht im aktiven Worktree (<worktreePath>). Bitte den Pfad korrigieren."}}
   ```
   Claude bekommt `permissionDecisionReason` als Kontext und kann den Tool-Call mit korrigiertem Pfad wiederholen. Wird nichts auf stdout geschrieben (der Normalfall, kein Block), gilt das als Erlauben — unverändert zum bisherigen Verhalten.

**Keine Abweichung von der bisherigen Hook-Philosophie nötig:** Der Kommentar in `main.go:12-13` („a hook must never block or break Claude Code") bleibt technisch korrekt — der Prozess selbst crasht/blockiert nie, er gibt nur strukturiert eine Entscheidung zurück, die Teil des offiziell unterstützten Hook-Protokolls ist. Keine Exit-Code-Änderung an `run()`/`main()` nötig.

### 4.3 Sichtbarkeit in der UI

Jeder Block wird zusätzlich als normale `hookLine`-JSONL-Zeile geschrieben (neues Feld `"blocked_path"`), die der bestehende `HookManager` ohnehin schon pollt. Ein neuer Event-Typ `worktree:path-blocked` (Pane-spezifisch, wie die bestehenden `worktree:detected`/`worktree:cleared`) löst eine Benachrichtigung im Frontend aus. Rein informativ, kein weiterer Nutzer-Eingriff nötig.

**Korrektur zum Implementierungsplan (2026-07-10):** Diese Sektion sprach ursprünglich von einem „Toast". Die Codebase hat aber **keine** Toast-/Snackbar-Komponente (verifiziert: keine Treffer für „toast"/„snackbar" in `frontend/src/components/`) — der einzige bestehende Mechanismus für nicht-blockierende Hinweise ist `sendNotification()` (`frontend/src/lib/notifications.ts`, native Desktop-Benachrichtigung über `App.SendNotification`, bereits genutzt für `app.agentDone`/`app.mergeConflicts`). Der Plan nutzt deshalb bewusst diesen bestehenden Mechanismus statt eine neue Toast-Komponente zu bauen (Scope-Disziplin, kein neuer UI-Baustein für einen einzelnen Anwendungsfall) — Text: „Schreibversuch blockiert" / „Claude wollte außerhalb des aktiven Worktrees schreiben ({path}) — blockiert."

## 5. Bewusste Grenzen

- **Codex-/Gemini-Panes bleiben außen vor.** Das Claude-Code-Hook-Protokoll (`PreToolUse` mit Blockier-Fähigkeit über Exit-Code) ist eine Claude-Code-CLI-Eigenschaft. Ob Codex/Gemini ein äquivalentes Konzept anbieten, ist nicht geprüft und nicht Teil dieses Designs.
- **Race Conditions:** Stirbt der `mtui-hook`-Prozess zwischen `EnterWorktree`-Erkennung und Sidecar-Schreiben (z. B. durch Prozess-Kill), bleibt die Beschränkung für die Session inaktiv, bis das nächste `EnterWorktree` sie neu auslöst. Best-Effort, kein hundertprozentiger Schutz — schließt aber exakt die Lücke aus Abschnitt 1.
- **Kein Schutz vor manuellem `cd` im Terminal außerhalb von Claude-Code-Tool-Aufrufen** (z. B. ein Shell-Befehl, der direkt im Hauptrepo committet) — das ist weiterhin nur durch die bestehenden `worktreeDenyRules` (Bash-Ebene) abgedeckt, nicht Gegenstand dieses Designs.
- **Kein Ersatz für Disziplin bei der Pfad-Konstruktion selbst** — die Firewall verhindert das *Ausführen* eines fehlgeleiteten Schreibzugriffs, nicht, dass ein Agent überhaupt einen falschen Pfad vorschlägt. Sie ist ein Sicherheitsnetz, keine Verhaltensänderung.
- **Fehlende `session_id` im Hook-Payload:** Sollte Claude Codes Hook-Payload `session_id` jemals auslassen (in der Praxis nicht beobachtet — der Wert ist immer eine UUID), fällt `cmd/mtui-hook` auf den literalen String `"unknown"` als Sidecar-/JSONL-Schlüssel zurück (`firstNonEmpty(ev.SessionID, "unknown")`, `main.go`). Zwei solche Sessions würden sich dann denselben Worktree-Kontext teilen. Rein theoretisch, kein Code-Änderungsbedarf — hier nur der Vollständigkeit halber als bewusste Grenze festgehalten.

## 6. Betroffene Dateien

- `cmd/mtui-hook/main.go` — Hook-Entry-Point, Event-Parsing, `EnterWorktree`/`ExitWorktree`-Erkennung, ruft die Firewall-Prüfung auf.
- `cmd/mtui-hook/firewall.go` — Kernlogik der Pfad-Firewall (Sidecar-Lese/Schreiben, `resolveWorktreeContext`, `isUnderDir`, `gitMainRepoRoot`, `checkPathFirewall`), ausgelagert aus `main.go` (Code-Rule: max. 300 Zeilen pro Go-Datei).
- `cmd/mtui-hook/firewall_test.go` — Testfälle für die Firewall-Kernlogik (siehe Abschnitt 7).
- `cmd/mtui-hook/main_test.go` — Testfälle für Event-Parsing, Sidecar-Lebenszyklus und die End-to-End-`run()`-Blockierung.
- `cmd/mtui-hook/hide_windows.go` / `hide_other.go` — Plattform-Split für `hideConsole` (Windows-Console-Unterdrückung beim `git`-Subprozess-Spawn in `gitMainRepoRoot`; no-op auf anderen Plattformen).
- `internal/backend/app_worktree_pane.go` / `app.go` — neue Env-Vars beim PTY-Start für MTUI-eigene Worktree-Panes.
- `internal/backend/app_hooks.go` — neuer Event-Typ `worktree:path-blocked` im `HookManager`-Dispatch.
- Frontend: neuer Listener (z. B. in `App.svelte`, analog zu bestehenden `worktree:*`-Listenern) — kein neuer Dialog, keine neue Komponente nötig; nutzt das bestehende native Desktop-Benachrichtigungssystem (`lib/notifications.ts` → `sendNotification()`/`App.SendNotification`), **nicht** ein Toast/Snackbar-System (siehe Korrektur in Abschnitt 4.3 — die Codebase hat keine Toast-Komponente).

## 7. Testing

- **`cmd/mtui-hook` Unit-Tests** (reine Pfad-Klassifizierung, kein echter Claude-Prozess nötig): Pfad innerhalb Worktree → erlaubt; Pfad innerhalb mainRepoRoot außerhalb Worktree → blockiert (stdout enthält `hookSpecificOutput.permissionDecision: deny`); Pfad außerhalb beider → erlaubt; kein aktiver Kontext (keine Env-Var, keine Sidecar-Datei) → erlaubt (Regressionstest: bestehende Panes ohne Worktree-Bezug dürfen sich nicht ändern); `Read`-Tool wird nie geprüft, auch nicht mit einem Hauptrepo-Pfad.
- **Sidecar-Lebenszyklus:** `EnterWorktree` schreibt Sidecar-Datei mit korrektem Inhalt → `ExitWorktree` löscht sie wieder → nach dem Löschen ist wieder alles erlaubt.
- **Sidecar-Vorrang:** Ist sowohl die (aktuelle) Sidecar-Datei als auch die Env-Var vorhanden, gewinnt die Sidecar-Datei.
- **`app_hooks_installer_test.go`:** bestehende Idempotenz-Tests müssen weiter grün bleiben (keine Änderung an der Hook-Registrierung selbst nötig, nur an der Payload-Verarbeitung).
- **E2E (`needs-e2e-testing`):** mit echtem `claude`-Prozess in einer MTUI-Pane: (1) `EnterWorktree` aufrufen, dann bewusst versuchen, eine Datei im Hauptrepo-Pfad zu editieren → Block + Toast erscheint. (2) Im selben Zustand eine Datei im Worktree editieren → funktioniert normal. (3) Im selben Zustand eine Datei im Hauptrepo lesen (`Read`) → funktioniert normal, kein Block. (4) `ExitWorktree` aufrufen → anschließend ist Editieren im Hauptrepo wieder uneingeschränkt möglich.

## 8. Nicht im Scope

- Codex-/Gemini-Pane-Absicherung (Abschnitt 5).
- Schutz vor manuellen Shell-`cd`/Git-Befehlen außerhalb von Claude-Code-Tool-Aufrufen.
- Rückwirkende Aktualisierung von Abschnitt 9 in `docs/superpowers/specs/2026-07-03-worktree-enterworktree-design.md` (der dortige Stand „Deny-Pattern NICHT umgesetzt" ist inzwischen durch `worktreeDenyRules` überholt) — sollte separat nachgezogen werden, ist aber nicht Teil dieser Änderung.
