# Design: Worktree-Isolation über natives Claude-Code-`EnterWorktree`

**Datum:** 2026-07-03
**Status:** Implementiert (Branch `feat/worktree-enterworktree`, 13 Tasks, alle SDD-reviewed). `needs-e2e-testing` bis zur manuellen Verifikation (s. u.).

## 0. Umsetzungs-Notiz (Nachtrag 2026-07-03)

`frontend/src/lib/session.ts` (Save/Restore der Tab-/Pane-Konfiguration) wurde vom Plan bewusst NICHT angefasst — sie stammt unverändert vom Vorgänger-Branch `feat/worktree-pro-pane` und erwies sich als voll kompatibel mit diesem Design: Sie serialisiert/restauriert `worktreePath`/`branch`/`targetBranch` rein über Pfad-Existenz (`App.WorktreeDirExists`), unabhängig davon, ob der Wert einst von MTUI selbst gesetzt wurde oder — wie jetzt — von `tabStore.setWorktree()` aus der Hook-Erkennung stammt. Ergebnis: App-Neustart während ein Pane in einem Claude-erzeugten `.claude/worktrees/`-Verzeichnis arbeitet, restauriert die Session korrekt in diesem Verzeichnis (oder fällt sauber auf das Hauptverzeichnis zurück, falls der Worktree inzwischen weg ist) — ohne dass dieser Plan dafür etwas bauen musste.
**Ersetzt:** `docs/superpowers/specs/2026-07-02-worktree-pro-pane-design.md` als Design-Richtung. Der zugehörige Branch `feat/worktree-pro-pane` (21 Tasks + Fixes, fertig implementiert und reviewed) wird **nicht verworfen** — er bleibt unangetastet liegen, da große Teile seiner Backend-Logik (Verifikations-Gate, Merge, Cleanup, Idempotenz, Prozessbaum-Kill) hier wiederverwendet werden. Siehe Abschnitt 8.

## 1. Ziel & Philosophie-Wechsel

Ziel bleibt unverändert: Ein Projekt hat zu jedem Zeitpunkt genau **einen** lebenden Branch (z. B. `alpha-main`). Mehrere Panes arbeiten parallel an unterschiedlichen, nicht kollidierenden Bereichen desselben Projekts — jedes Pane wie ein eigenständiger Entwickler mit eigenem Arbeitsbereich, der committet, testet und seine Arbeit zurückführt.

**Der Mechanismus ändert sich grundlegend.** Der alte Entwurf ließ MTUI den Worktree aktiv erzeugen (Checkbox im LaunchDialog, `git worktree add` durch MTUIs Go-Backend) und den kompletten Abschluss orchestrieren (✓-Button → Verifikations-Gate → ff-only-Merge → Cleanup, alles von MTUI kontrolliert).

Dieser Entwurf kehrt die Rollen um: **Claude Code entscheidet selbst**, wann ein Worktree sinnvoll ist, über sein eigenes natives Tool `EnterWorktree`. Claude committet, pusht, öffnet bei Bedarf einen PR und räumt seinen Worktree danach selbst auf (`ExitWorktree`) — wie ein echter Entwickler, der eine Aufgabe bekommt und sie selbständig abwickelt. **MTUI wird vom Regisseur zum Beobachter**: es richtet das Projekt einmalig so ein, dass Claude sicher isolieren kann, erkennt per Hook, was passiert, und zeigt den Status an.

Diese Rollenumkehr ist eine bewusste Entscheidung des Projektinhabers, keine technische Notwendigkeit — sie spart erhebliche UI- und Orchestrierungs-Komplexität, verzichtet dafür aber auf die unabhängige Sicherheitsprüfung, die das alte Design vor jedem Merge/Löschen erzwang (siehe Abschnitt 5, „Ehrlich benannte Abschwächung").

## 2. Verifizierte Grundlagen (empirisch getestet, nicht nur Doku)

Alle folgenden Aussagen wurden in einem Wegwerf-Testrepo (`D:\repos\worktree-test-sandbox`) mit einem echten `claude -p --dangerously-skip-permissions`-Prozess verifiziert, nicht nur aus der offiziellen Doku übernommen (die an mehreren Stellen unvollständig bzw. bei einer Recherche-Runde sogar mit unverifizierbaren/vermutlich halluzinierten GitHub-Issue-Verweisen widersprüchlich war):

- **`EnterWorktree(name)`** erzeugt einen echten Verzeichniswechsel für die Session (`pwd` bestätigt es), legt den Worktree unter **`.claude/worktrees/<name>`** an (innerhalb des Projekts, nicht als Sibling) und den Branch als **`worktree-<name>`**. Nur nutzbar, wenn „worktree" explizit in einer User- oder CLAUDE.md-/Memory-Anweisung erwähnt wird (harte Voraussetzung laut Tool-Beschreibung).
- **`.claude/settings.local.json`-Regeln aus dem Haupt-Projektverzeichnis gelten in der GESAMTEN Session weiter**, auch nachdem `EnterWorktree` die Session in den neuen Pfad geschaltet hat — verifiziert: ein `git merge`-Deny-Rule griff im Worktree, obwohl nur im Haupt-Repo hinterlegt.
- **`PostToolUse`-Hooks können spezifisch auf den Tool-Namen `EnterWorktree` matchen.** Der Hook-Payload enthält `tool_response.worktreePath`, `tool_response.worktreeBranch` und `session_id` — strukturiert, direkt nutzbar, kein Terminal-Output-Parsing nötig. (Hook-Kommandos laufen mit dem CWD zum Zeitpunkt des Feuerns — bei `PostToolUse:EnterWorktree` ist das bereits der NEUE Worktree-Pfad, nicht der ursprüngliche. Bei der Implementierung mit absoluten Pfaden arbeiten, nicht mit relativen.)
- **`ExitWorktree(action:"remove")` verweigert das Entfernen standardmäßig**, wenn der Worktree uncommittete Dateien ODER Commits enthält, die auf keinem anderen Branch vorkommen — exakt wie in der Tool-Beschreibung dokumentiert. Nur mit explizitem `discard_changes: true` wird trotzdem gelöscht (verifiziert: ein Commit ohne jede andere Referenz wurde damit unwiederbringlich entfernt). Die Tool-Beschreibung weist das aufrufende Modell selbst an: „confirm with the user, then re-invoke with discard_changes: true".
- **`worktree.baseRef`** ist konfigurierbar: `head` (Default über Setting steuerbar) branched vom aktuellen lokalen HEAD, `fresh` von `origin/<default-branch>`. Für das „ein lebender Branch"-Modell ist `head` die richtige Wahl.

## 3. Einmalige Projekt-Einrichtung

Statt einer Checkbox pro Pane-Start: Beim ersten Öffnen eines Projekts in MTUI (oder beim ersten Claude-Pane-Launch dafür) legt MTUI im Projekt-Root an, falls nicht vorhanden:

1. **Memory-Anweisung** (Datei-Ort/-Name bei Implementierung final festlegen — Kandidat: `CLAUDE.local.md` im Projekt-Root, analog zum alten Design, aber projekt- statt worktree-weit):

```markdown
# MTUI-Projekt: Worktree-Isolation

Nutze für Aufgaben mit eigenem, klar abgegrenztem Arbeitsbereich das `EnterWorktree`-Tool,
um deine Änderungen zu isolieren. Der Haupt-Branch dieses Projekts darf dabei nie direkt
gewechselt werden.

Wenn eine Aufgabe erledigt ist: committe, pushe bei Bedarf und öffne einen Pull Request
(`gh pr create`), oder frage den Nutzer nach dem gewünschten Vorgehen.

Nutze `ExitWorktree` mit `discard_changes: true` oder erzwungenem Entfernen NIEMALS
eigenständig — nur nach ausdrücklicher Rückfrage beim Nutzer und dessen Bestätigung.
```

2. **`.claude/settings.local.json`** im Projekt-Root — bewusst KEINE Blanko-Verbote gegen Merge/Push/PR/Worktree-Entfernen (der Nutzer hat das explizit korrigiert: diese Aktionen sind erlaubt, nur nicht „einfach so"). Enthält stattdessen `worktree.baseRef: "head"`. Ob überhaupt noch harte Deny-Regeln sinnvoll sind, ist in Abschnitt 5 diskutiert.

3. Diese Einrichtung ist **pro Projekt einmalig**, nicht pro Pane. Ein neuer Claude-Pane-Start für ein bereits eingerichtetes Projekt braucht keinen Dialog, keine Checkbox, keine Eingabe.

## 4. Erkennung (Hook-Erweiterung)

MTUIs bestehende Hook-Infrastruktur (`cmd/mtui-hook`, `internal/backend/app_hooks.go` — aktuell genutzt für `UserPromptSubmit`-basiertes Pane-Auto-Naming) wird erweitert:

- Neuer Matcher **`PostToolUse:EnterWorktree`** wird beim Session-Start registriert (analog zur bestehenden `UserPromptSubmit`-Registrierung in `app_hooks_setup.go`).
- Der Hook schreibt (wie das bestehende Muster) eine JSONL-Zeile mit **absolutem Pfad** in ein Verzeichnis, das MTUI pollt — payload enthält `session_id`, `tool_response.worktreePath`, `tool_response.worktreeBranch`.
- `HookManager` bekommt einen neuen Callback (analog `onPrompt`), der `session_id` auf die MTUI-interne `sessionId` (int, PTY-Ebene) abbildet — diese Korrelation existiert vermutlich bereits für das Auto-Naming und wird wiederverwendet/verifiziert.
- Bei Treffer: Pane-State bekommt `worktreePath`/`worktreeBranch`/`targetBranch` (= aktuell im Haupt-Verzeichnis ausgecheckter Branch zum Erkennungszeitpunkt) gesetzt, Frontend-Event `worktree:detected` (sessionId-getaggt wie die bestehenden Events) → Badge erscheint.
- Analog: **`PostToolUse:ExitWorktree`** erkannt → Badge verschwindet, Pane-State zurückgesetzt.
- **Ein Pane kann über seine Lebensdauer mehrfach nacheinander unterschiedliche Worktrees durchlaufen** (Claude beendet Aufgabe A, arbeitet normal weiter, beginnt Aufgabe B mit neuem `EnterWorktree`-Aufruf) — anders als im alten Design (1 Pane = 1 Worktree für die gesamte Lebensdauer). Der Pane-State trackt nur den **aktuellen** Worktree, keine Historie.
- **PR-Status (optional, zweite Ausbaustufe):** periodisches `gh pr list --head <branch>` statt Bash-Command-Text-Parsing (zuverlässiger, da `gh pr create` auf viele Arten aufgerufen werden kann) — zeigt am Badge an, ob ein PR offen/gemergt ist.

## 5. Sicherheitsmodell — ehrlich benannte Abschwächung

Im alten Design prüfte MTUI selbst (`GetWorktreeFinishStatus`) unabhängig von Claude, ob ein Merge sicher ist, bevor es ihn ausführte — eine zweite, von Claudes eigenem Verhalten unabhängige Instanz. **In diesem Design gibt es keine solche unabhängige Instanz mehr für Merge/PR/Cleanup**, weil MTUI diese Schritte nicht mehr selbst ausführt. Die verbleibenden Sicherungen sind:

1. **Das native Tool-Verhalten** (`ExitWorktree` verweigert Entfernen ungemergter Commits ohne expliziten Override) — technisch verifiziert, wirkt unabhängig davon, was Claude „möchte".
2. **Die Memory-Anweisung** („niemals eigenständig discard_changes/force") — reine Prompt-Ebene, kein Enforcement (wie schon im alten Design für `CLAUDE.local.md` dokumentiert: Kontext, keine erzwungene Konfiguration).
3. **`.claude/settings.local.json`-Deny-Regeln** könnten zusätzlich versuchen, direkte Branch-Wechsel im Haupt-Verzeichnis zu verhindern (`git checkout <branch>`/`git switch <branch>`, NICHT `git checkout -- <datei>`) — offene Implementierungsfrage, siehe Abschnitt 9. Ob sich Berechtigungs-Deny-Regeln auf einzelne Tool-Parameter (z. B. `discard_changes: true` bei `ExitWorktree`) statt nur auf Bash-Befehlstext anwenden lassen, ist **nicht verifiziert** und müsste vor der Implementierung geprüft werden — falls nein, bleibt Punkt 2 die einzige Bremse gegen eigenständiges `discard_changes`.

**Bewusst akzeptiertes Risiko:** Es ist möglich, dass Claude in einer Session `discard_changes: true` nutzt, ohne wirklich beim Nutzer nachgefragt zu haben (Prompt-Befolgung ist nicht garantiert). Der Projektinhaber hat diese Abschwächung explizit gewählt, um MTUIs Rolle auf „Beobachter" zu reduzieren.

## 6. Ausfall-Netz

Da MTUI Cleanup nicht mehr erzwingt, können Worktrees verwaisen (Pane geschlossen, Session beendet, Claude hat nie aufgeräumt):

- **Pane-Close-Dialog** bleibt bestehen (aus dem alten Design übernommen): schließt der Nutzer ein Pane mit aktivem Worktree, Hinweis „Worktree bleibt liegen" statt automatischem Löschen.
- **Verwaiste-Worktrees-Ansicht** (neu, klein): keine neue Seite/Panel — Erweiterung der bestehenden `WorktreeDropdown`-Komponente (zeigt heute schon kategorisierte Worktrees in der Pane-Titelleiste), neue Kategorie „verwaist" für Einträge unter `.claude/worktrees/` ohne zugehöriges aktives Pane. MTUI listet sie (wiederverwendbar aus `ListAllWorktrees`/`app_worktree.go`) mit einer manuellen Aufräum-Möglichkeit — kein automatischer Merge, nur „anzeigen + auf Wunsch entfernen" als bewusst einfache Notlösung, kein Ersatz für den alten Verifikations-Gate-Mechanismus.

## 7. Frontend-Umfang (deutlich kleiner als im alten Design)

- **Kein LaunchDialog-Checkbox/Felder** (ersatzlos, entfällt gegenüber Task 15 des alten Branches).
- **Kein ✓-Finish-Button mit Merge-Logik, kein Bestätigungs-Overlay** (entfällt gegenüber Tasks 17/18 der Merge-Anteile) — stattdessen nur:
  - **⎇-Badge** in der Pane-Titelleiste, erscheint/verschwindet reaktiv auf die Hook-Events, zeigt Branch-Name + optionalen PR-Status.
  - **Verwaiste-Worktrees-Ansicht** (Abschnitt 6).
- Bestehende Pane-Close-Bestätigung wird wiederverwendet (Konzept aus Task 18 übernehmbar).

## 8. Verhältnis zum bestehenden `feat/worktree-pro-pane`-Branch

Der Branch bleibt **unverändert liegen** (nicht gemerged, nicht verworfen), da folgende Bausteine daraus voraussichtlich direkt oder mit kleinen Anpassungen wiederverwendbar sind:

| Baustein (alter Branch) | Wiederverwendbarkeit hier |
|---|---|
| `app_worktree_marker.go` (Idempotenz-Marker) | Ggf. für die Verwaiste-Worktrees-Ansicht, falls Aufräum-Vorgänge Crash-sicher gemacht werden sollen |
| `kill_windows.go`/`kill_other.go` (Prozessbaum-Kill) | Für die manuelle Aufräum-Aktion in Abschnitt 6 (Windows-Handle-Problem bleibt identisch) |
| `app_worktree.go` (`ListAllWorktrees`, Kategorisierung) | Für die Verwaiste-Worktrees-Ansicht, Kategorie-Erkennung um `.claude/worktrees/`-Präfix erweitern |
| `app_worktree_finish_status.go`, `app_worktree_cleanup.go` (Verifikations-Gate, ff-only-Merge) | **Nicht mehr im primären Flow genutzt** (MTUI merged nicht mehr aktiv) — bleiben als Referenz/mögliche spätere Zusatzfunktion („MTUI-seitiger Merge als Alternative anbieten") erhalten, nicht Teil des MVP dieses Designs |
| Tasks 1–4, 15, 16 (Erzeugung/LaunchDialog) | Entfällt vollständig |
| Tasks 17, 18 (Badge/Finish-Dialog) | Badge-Konzept wiederverwendbar, Finish-Dialog-Logik entfällt |

Die endgültige Entscheidung, ob/wie der alte Branch geschlossen wird (verwerfen, als Referenz behalten, Teile per Cherry-Pick übernehmen), wird beim Schreiben des Implementierungsplans getroffen, nicht in diesem Dokument vorweggenommen.

## 9. Offene Implementierungsfragen — Ergebnis nach Umsetzung

- **Deny-Pattern für Branch-Wechsel im Haupt-Verzeichnis:** NICHT umgesetzt — das Sicherheitsmodell (Abschnitt 5) verzichtet bewusst auf harte Deny-Regeln für Merge/Push/PR/Worktree-Entfernen (User-Korrektur während des Brainstormings: „nicht einfach so", nicht „gar nicht"). Es gibt daher auch keine Deny-Regel gegen Branch-Wechsel im Hauptverzeichnis — dieser Schutz besteht nicht.
- **Tool-Parameter-Matching (`discard_changes`) bei Claude-Code-Permissions:** nicht verifiziert, da keine Deny-Regeln mehr Teil des Designs sind — die Frage ist gegenstandslos geworden.
- **Hook-Registrierungssyntax für `PostToolUse:EnterWorktree`:** **erledigt sich von selbst** — `PostToolUse` ist in `app_hooks_installer.go` bereits global (ohne Tool-Matcher) für alle Tools registriert. Es war keine neue Registrierung nötig, nur eine Erweiterung der Payload-Auswertung (Tasks 1–3).
- **`session_id` → MTUI-`sessionId`-Korrelation:** bestätigt vorhanden und wiederverwendet — `MULTITERMINAL_SESSION_ID` wird beim Sessionstart gesetzt (`app.go`) und von `mtui-hook` unverändert durchgereicht (`mt_id`-Feld).
- **PR-Status-Erkennung (`gh pr list`):** nicht umgesetzt (v1-Scope-Reduktion) — die Badge-Anzeige beschränkt sich auf Branch-Name, kein PR-Status. Kandidat für ein Folge-Ticket.
- **Koexistenz mit `.mt-worktrees`-Altlasten:** `categorizeWorktree` unterscheidet die Präfixe eindeutig (`.mt-worktrees/` → Kategorie `terminal`/`issue`, `.claude/worktrees/` → Kategorie `claude`) — keine Kollision, beide Schemata laufen nebeneinander.

## 9a. E2E-Checkliste (`needs-e2e-testing`, mit echtem Claude Code)

1. Neues Projekt in MTUI öffnen, Claude-Pane starten → `CLAUDE.local.md` + `.claude/settings.local.json` (`worktree.baseRef: head`) entstehen im Projekt-Root.
2. Claude eine Aufgabe geben, die isolierte Arbeit nahelegt → beobachten, ob Claude von sich aus `EnterWorktree` aufruft; ⎇-Badge muss innerhalb der 100ms-Hook-Polling-Latenz erscheinen.
3. Claude fertigstellen lassen (committen, `gh pr create`) → beobachten, ob Claude eigenständig sinnvoll handelt.
4. Claude bitten, den Worktree zu entfernen, OHNE dass Arbeit gemergt/gepusht wurde → prüfen, ob Claude gemäß Memory-Anweisung beim Nutzer nachfragt, bevor es `discard_changes: true` nutzt (Beobachtungstest, kein hartes Gate — die Memory-Anweisung ist Kontext, keine Durchsetzung).
5. Pane schließen, während ein Worktree mit offener Arbeit existiert → Bestätigungsdialog erscheint, Worktree bleibt liegen.
6. Verwaisten Worktree über die Dropdown-Sektion „Verwaist" manuell entfernen — Fall „bereits gemergt" (klaglos entfernt) und „nicht gemergt" (Fehlermeldung, Branch bleibt stehen).
7. App-Neustart mit einem Pane, das gerade in einem `.claude/worktrees/`-Verzeichnis arbeitet → Session restauriert korrekt in diesem Verzeichnis (`session.ts`, unverändert vom Vorgänger-Branch — siehe Abschnitt 0); Badge erscheint nach dem nächsten Hook-Event erneut.
8. Zwei parallele Claude-Panes im selben Projekt, beide erzeugen eigene `EnterWorktree`-Worktrees → keine gegenseitige Störung, beide Badges unabhängig korrekt.

## 10. Nicht im Scope

- Shell-Panes (kein natives `EnterWorktree` ohne Claude) — bleiben unverändert, wie heute.
- MTUI-seitiger, aktiv orchestrierter Merge als PRIMÄRER Pfad (das war das alte Design) — bleibt als möglicher Zusatzweg für später denkbar (Abschnitt 8), ist aber nicht Teil dieses Entwurfs.
