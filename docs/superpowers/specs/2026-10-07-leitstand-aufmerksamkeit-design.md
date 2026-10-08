# Leitstand, Teil 1: Was wartet auf mich, über alle Projekte — Design

**Status:** Umgesetzt auf `worktree-leitstand-aufmerksamkeit`, E2E-Abnahme und Sichtprüfung offen
**Worktree:** C:\Users\Patrick.Hennig\repos\Multiterminal-UI\.claude\worktrees\leitstand-aufmerksamkeit
**Betrifft:** neu `internal/hub/attention*.go`, `internal/hub` (Scan, Hooks, Summary, Wire), `cmd/mtui-hook`,
`internal/backend` (Scan, Notify, Session-Spawn, Dashboard), `frontend/src/lib/focusLayout.ts`,
neu `frontend/src/lib/attention.ts` und `WaitingPanel.svelte`, `PaneGrid.svelte`, `TabBar.svelte`,
`Toolbar.svelte`, `stores/tabs.ts`, `stores/theme.ts`
**Vorbilder:** herdr (fertig heißt ungesehen), Claude Code Agent View, Codex, cmux

## Problem

Patrick arbeitet in **einem** MTUI-Fenster mit rund zehn Projekten als Tabs, in jedem vier bis
sechs Panes, in fast allen laufen Agents. Er sieht nicht auf einen Blick, in welchem Projekt
welches Pane auf ihn wartet. Der Kasten „Wartet auf dich · andere Projekte“ unten rechts im
Fokus-Layout gefällt ihm vom Platz her, aber **es fehlt etwas darin** und er ist zu klein,
um ihn nicht zu übersehen.

Ein Screenshot des laufenden Fensters (alpha.30, Theme `konzept`, 1920×1080) zeigt das Bild:

- Zehn Tabs, die Namen auf rund zehn Zeichen gekürzt („go…“, „gof…“, „goki-i…“).
- Alle zehn Tabs tragen denselben grünen Punkt. „Läuft“ und „fertig“ sehen gleich aus.
- Im aktiven Tab stehen zwei Panes auf „fertig“, der Kasten unten rechts belegt ein Fünftel
  der Spalte mit „Kein anderes Projekt wartet auf dich“.

Ein zweiter Screenshot zeigt eine Frage, die nie im Kasten auftauchte: „Soll ich den Stack als
zwei PRs submitten? Danach starte ich den Review …“. Das Pane war nicht das aktive.

## Gesamtbild: MTUI wird zum Leitstand

Leitsatz: **Du suchst nicht mehr nach den Agents, sie melden sich bei dir, mit allem, was du
für die Entscheidung brauchst.** Vier Teilprojekte, jedes mit eigener Spec:

| # | Teilprojekt | Kern |
|---|---|---|
| 1 | **Aufmerksamkeit** (diese Spec) | Ein Zustandsmodell auf dem Host, der Kasten als Leitstand, die Lecks gestopft |
| 2 | Review-Briefing | Karte beim Fertigwerden: was getan, Diff-Stats, Branch, PR und CI; Zustände abgeleitet statt gepflegt |
| 3 | Weniger Unterbrechungen | Freigabe-Politik je Projekt über `PermissionRequest`-Entscheidungen, Freigabe aus dem Leitstand, Kosten- und Schleifenalarm |
| 4 | Autonom: Issue → PR, Chef-Agent je Projekt | Läufe in echten Panes statt `claude -p`, der Chef-Agent fragt den Leitstand über MCP und eskaliert nur, was er nicht selbst klären kann |

Die Reihenfolge ist Absicht: Mehr Autonomie heißt mehr parallele Agents und damit mehr
Aufmerksamkeitslast. Ohne verlässliches Teilprojekt 1 verschärft Teilprojekt 4 das Problem.
Das Kanban v3 (Issues #109–#146) wird bis dahin nicht weitergebaut; das Board kommt später als
abgeleitete Ansicht aus 2 und 4 zurück.

## Befund: warum heute etwas fehlt

| # | Leck | Stelle |
|---|---|---|
| 1 | Die Liste nimmt nur Freigabe und Frage, **fertig und Fehler sind ausgeschlossen**, ebenso der eigene Tab. | `frontend/src/lib/focusLayout.ts:61-81` |
| 2 | **Agent-Panes** (MCP, `mt new`) landen im gerade aktiven Tab, egal zu welchem Projekt sie gehören. Damit erscheinen sie nie in der Liste. | `frontend/src/App.svelte:441-461` |
| 3 | Das **Tab-Badge hängt von der Reihenfolge ab**: [Frage, läuft] ergibt „läuft“. Der Test prüft nur die umgekehrte Reihenfolge. | `frontend/src/stores/tabs.ts:107-117`, `tabs.test.ts:525-532` |
| 4 | **„Gesehen“ gibt es nur je Tab**, und es wird bei jedem Kosten- oder Titel-Tick neu berechnet; ein gesehenes Badge kommt wieder. | `tabs.ts:152`, `tabs.ts:200` |
| 5 | **Auto-Suspend** macht aus „fertig“ nach dem Timeout „schläft“; die Information „fertig, ungesehen“ ist weg. | `internal/backend/app_suspend_auto.go:82-91` |
| 6 | Eine Freigabe, die mit Esc abgelehnt wurde, bleibt stehen, bis der nächste Hook kommt. Ein Rückfall bei Stillstand existiert nur für „läuft“. | `internal/hub/embedded_scan.go:100-149` |
| 7 | `PostToolUseFailure` zählt als Fehler, obwohl der Agent danach selbst weitermacht. | `internal/hub/hook_activity.go:35` |
| 8 | Die Liste verschwindet bei maximiertem Pane; ein frischer Hook-Zustand hat `since=0` und rutscht ganz nach unten. | `PaneGrid.svelte:452`, `focusLayout.ts:78-80` |
| 9 | Das Dashboard liest den rohen statt des bestätigten Zustands; zwei Anzeigen, zwei Logiken. | `internal/hub/embedded_summary.go:32` |
| 10 | Was der Agent will, erreicht das Frontend nie: `tool_input` liest `mtui-hook`, gibt es aber nicht weiter; der Stop-Text dient nur der Fragenerkennung. | `cmd/mtui-hook/main.go` (`claudeEvent`, `hookLine`), `internal/backend/app_hooks.go:26-28` |
| 11 | Die theme-übergreifende Statussprache kennt grün = läuft, bernstein = wartet, rot = Gefahr, grau = ruht, aber **keine Farbe für „fertig“**. Im Standard-Theme `konzept` sind dazu Akzent und Erfolg dasselbe Grün wie „läuft“. | `frontend/src/stores/theme.ts:34-55`, `:170-194` |
| 12 | Eine beim Update nicht ersetzte `mtui-hook.exe` schickte bei Stop keinen Text; jede Schlussfrage wurde „fertig“. **Auf `alpha-main` behoben** (#213, `acb7f6b`), in alpha.30 noch nicht. Das erklärt den Screenshot-Fall. | `internal/backend/app_hooks_setup.go` |

Der Fragenerkenner selbst trifft den Screenshot-Fall: `sentenceQuestion` zählt ein „?“ mitten im
letzten Absatz, `embedded_hooks_test.go:64` prüft genau dieses Muster.

## Ziele und Erfolgskriterien

1. Kein Pane, das auf Patrick wartet, fehlt im Kasten, egal in welchem Tab es liegt, ob ein
   Agent es geöffnet hat und ob die Frage als Frage erkannt wurde.
2. Jeder Eintrag sagt ohne Klick: welches Projekt, welches Pane, was es will, seit wann.
3. Die Tab-Leiste zeigt für jedes Projekt den Zustand jedes Panes.
4. Kasten, Tab-Leiste, Zähler, Ctrl+Shift+J, Toast und Taskleiste rechnen mit **einer** Logik.
5. Fertig und Fehler verschwinden, wenn Patrick hingesehen hat, und nicht vorher.

## Teil A: das Aufmerksamkeitsmodell

### Datensatz je Session (auf dem Host)

| Feld | Bedeutung |
|---|---|
| `State` | der bestätigte Zustand (`hub.Activity`) |
| `Since` | seit wann er gilt; immer gesetzt, nie 0 |
| `Seen` | ob Patrick den Zustand gesehen hat; jeder Zustandswechsel setzt es auf `false` |
| `Detail` | Kurztext, siehe unten |
| `Source` | woher der Zustand stammt, z. B. „Hook: Stop, Frage erkannt“, „Bildschirm-Scan“, „Prozess beendet (Exit 1)“, „keine Ausgabe seit 10 min“ |
| `Rank` | aus `State` und `Seen` berechnet, siehe Tabelle |

### Rangfolge

Die Zeichen sind Platzhalter; die endgültigen liefert der Prototyp (Teil D).

| Rang | Zustand | Zeichen | Im Kasten | Wer räumt ab |
|---|---|---|---|---|
| 0 | Freigabe (`waitingPermission`) | ▲ | ja | der Agent: nächster Hook, oder der Dialog ist vom Bildschirm verschwunden |
| 1 | Frage (`waitingAnswer`) | ? | ja | der Agent: `UserPromptSubmit` oder nächster Hook |
| 2 | Fehler oder hängt (`error`) | ✕ | ja, bis gesehen | Patrick (gesehen) oder neue Aktivität |
| 3 | fertig, ungesehen (`done`, `Seen=false`) | ✓ | ja, bis gesehen | Patrick (gesehen) |
| 4 | läuft (`active`) | ◌ | nein | — |
| 5 | fertig gesehen, idle, schläft | · | nein | — |

Innerhalb eines Rangs gilt: der älteste zuerst (`Since` aufsteigend).

**Grundregel: Blockierende Zustände räumt der Agent ab, informative räumt Patrick ab.** Hinsehen
allein beantwortet keine Frage, deshalb bleiben Rang 0 und 1 stehen, bis der Agent weitermacht.

### Kurztext (`Detail`)

| Zustand | Kurztext |
|---|---|
| Freigabe | Tool und Ziel: „Bash: npm install …“, „Edit: src/app.ts“ |
| Frage | bei `AskUserQuestion` die erste Frage, sonst der Satz mit dem „?“ aus dem letzten Absatz |
| fertig | der letzte Absatz der letzten Antwort |
| Fehler | Exit-Code bzw. „keine Ausgabe seit 10 min“ |

Höchstens 200 Zeichen. Ist die Frage nicht als Frage erkannt worden, steht das Pane trotzdem als
„fertig“ im Kasten, mit dem letzten Absatz als Kurztext. Die Frage ist dann so oder so sichtbar.

### Gesehen

Ein Zustand gilt als gesehen, wenn **das Fenster den Fokus hat und das Pane mindestens eine
Sekunde lang den Tastaturfokus hatte**, oder sobald Patrick in das Pane tippt. Von Hand:
„als ungelesen markieren“ und „alle fertigen als gesehen markieren“.

Jeder Zustand eines Panes wird nur **einmal** als gesehen gemeldet (Schlüssel: Pane, Rang,
Startzeit). Tippen und Hinsehen laufen über **einen** gemeinsamen Melder.

### Fehler und „hängt“

- **Fehler:** Der Prozess endet mit Exit-Code ≠ 0.
- **Sauberes Ende:** Endet der Prozess mit Exit 0 und hat Patrick das Pane nicht selbst
  geschlossen, wird der Zustand „fertig“ mit `Source` „Prozess beendet (Exit 0)“. Ein Pane, das
  Patrick selbst schließt, erzeugt nie einen Fehler.
- **Hängt:** Nur für Agent-Panes (Claude, Codex), nicht für Shell-Panes. Der Zustand ist
  „läuft“, aber seit 10 Minuten gab es weder Ausgabe noch ein Hook-Ereignis. Claude Code
  zeichnet während der Arbeit laufend seinen Spinner; zehn Minuten Stille heißt deshalb wirklich
  Stillstand.
- `PostToolUseFailure` ist **kein** Fehler mehr, sondern „läuft“: Ein roter Test gehört zur
  Arbeit des Agents.

### Schlafen

Auto-Suspend lässt den Datensatz stehen. Ein fertiges, ungesehenes Pane bleibt Rang 3, auch
wenn sein Prozess schläft.

## Teil B: die Oberfläche

Das Fokus-Layout bleibt: zwei große Panes in der Mitte, kleine Panes rechts, der Kasten
„Wartet auf dich“ unten rechts. Die linke Seite bleibt für Explorer und Issues.

```
 lizenzserver ✓✓ │ polaris-ful ▲◌ │ goki-imp ?◌ │▸Arpatools ◌◌✓│ jera2lex ◌ │…
┌─────────────────────────┬─────────────────────────┬────────────────────┐
│ Zugriffsschutz Styling  │ CMD-Interface           │ ✓ Lagerbewertung   │
│                         │                         │   (klein)          │
│                         │                         ├────────────────────┤
│                         │                         │ WARTET AUF DICH  4 │
│                         │                         │ ▲ polaris·P2    2m │
│                         │                         │   Bash: npm i …    │
│                         │                         │ ? goki·Planung  7m │
│                         │                         │   Soll ich die …?  │
│                         │                         │ ✓ hier·Lager    1m │
│                         │                         │   +3 weitere fertig│
└─────────────────────────┴─────────────────────────┴────────────────────┘
```

### B1 Der Kasten „Wartet auf dich“

- Er zeigt alle Einträge mit Rang 0–3 aus **allen** Tabs, auch aus dem aktuellen; die dann mit
  „hier“ statt des Projektnamens.
- Jeder Eintrag hat zwei Zeilen. Zeile 1: Zeichen, Projekt · Pane, Wartezeit. Zeile 2: Kurztext.
- Fertige Einträge über drei hinaus klappen zu „+N weitere fertig“ zusammen; ein Klick klappt auf.
- Die Höhe richtet sich nach dem Inhalt. Leer: eine leise Zeile „Nichts wartet“, die kleinen
  Panes bekommen den Platz zurück. Voll: höchstens 40 % der Spalte, dann scrollt der Kasten.
- Beim Überfahren eines Eintrags: `Source` und die genaue Uhrzeit von `Since`.
- Ein neuer Eintrag mit Rang 0 oder 1 leuchtet einmal kurz auf, ohne Dauerpulsieren.

### B2 Bedienung

| Aktion | Wirkung |
|---|---|
| Klick auf einen Eintrag | Tab wechseln, Pane fokussieren |
| Alt+Klick | Vorschau: das vorhandene schwebende Fenster über dem aktuellen Tab |
| Ctrl+Shift+J | nächster Eintrag in Rangfolge, im Fokus-Layout als Vorschau, im Grid als Sprung; wiederholt geht es weiter, am Ende von vorn |
| Ctrl+Shift+K | zurück zu dem Pane, in dem Patrick vor dem ersten Ctrl+Shift+J bzw. Sprung war; gilt nur, solange Patrick noch dort ist, wohin ihn der Lauf gebracht hat (Vorschau offen oder auf dem angesprungenen Pane). Nach jeder anderen Navigation ist der Lauf beendet. Sprünge tauschen die zwei großen Panes nie. |
| Ctrl+W in der Vorschau | schließt sie, wie heute |
| Rechtsklick auf einen Eintrag | „Hingehen“, „Vorschau öffnen“, „Als gesehen markieren“, „Alle fertigen als gesehen markieren“ |
| Rechtsklick auf die Titelleiste eines Panes | „Als ungelesen markieren“ (bei fertig oder Fehler, schon gesehen), „Als gesehen markieren“ |

Die Vorschau gibt es nur im Fokus-Layout, wie heute. Im Grid springt der Eintrag hin. (Im
Gespräch war die Vorschau auch fürs Grid genannt; Patrick nutzt das Fokus-Layout, deshalb
bewusst weggelassen.)

### B3 Die Tab-Leiste als Projektübersicht

- Statt „2 ●“ zeigt jeder Tab **ein Zeichen je Pane** in der Pane-Reihenfolge.
- Farbe und Zeichen des Tabs selbst kommen vom **kleinsten Rang** seiner Panes.
- Die Pane-Zahl entfällt (die Zeichen ersetzen sie); das × erscheint erst beim Überfahren.
- Tooltip: voller Projektname und die Panes mit Zustand.
- Lange Namen kürzt Patrick über das vorhandene Umbenennen (`TabBar.svelte:56`).

### B4 Zähler in der Werkzeugzeile

Ist der Kasten nicht zu sehen (Grid-Modus, maximiertes Pane), zeigt die Werkzeugzeile
„N warten“ (Rang 0–3). Ein Klick öffnet dieselbe Liste als Aufklapper. Bei 0 ist der Zähler leer
und nimmt keinen Platz ein.

### B5 Außerhalb des Fensters

- **Taskleiste:** die Zahl der blockierenden Einträge (Rang 0–1) als Overlay am Symbol.
  Rückfall, falls die API unter Wails v3 nicht erreichbar ist: „(3) Multiterminal“ im Fenstertitel.
  Umgesetzt über das Wails-Paket `pkg/w32` (`ITaskbarList3::SetOverlayIcon`); nur das
  Hauptfenster setzt das Overlay, Fehler werden protokolliert. Sichtprüfung offen (offener
  Punkt 2).
- **Toast:** nur für Rang 0–2, nie für das Pane mit Tastaturfokus in einem fokussierten Fenster.
  Ein Klick holt das Fenster nach vorn **und springt zum Pane**. Der Toast feuert nur für
  Zustände, die nach dem Öffnen des Fensters begannen, wenn es keinen früheren Datensatz gab
  (kein Schwall nach einem Neustart). Der Text wird gegen PowerShell abgesichert, weil go-toast
  ihn in einen erweiterbaren Here-String einbettet: Leerraum zusammengezogen, Backtick und `$`
  maskiert, `]]>` entschärft; das Startargument ist XML-maskiert und wird nur aus einem gültigen
  Pane-Ref gebaut.
- **Fertig:** nur der vorhandene Ton.

### B6 Was wegfällt oder sich ändert

- `waitingElsewhere` und die Kastenlogik in `PaneGrid.svelte` gehen in `attention.ts` und
  `WaitingPanel.svelte` auf.
- `computeTabActivity` und `Tab.unreadActivity` werden durch den kleinsten Rang ersetzt.
- Das Dashboard bleibt, liest aber den bestätigten Zustand.

## Teil C: die Technik

### C1 Der Host besitzt das Modell

Neu `internal/hub/attention.go` (und bei Bedarf `attention_*.go`, unter 300 Zeilen je Datei):

- `type Attention struct { State Activity; Since time.Time; Seen bool; Detail, Source string; Rank int }`
  mit `yaml`- und `json`-Tags.
- `AttentionRank(state Activity, seen bool) int` ist die **einzige** Rangfunktion im System.
- Der Datensatz wird bei jedem bestätigten Zustandswechsel neu gesetzt (`Seen=false`, `Since=jetzt`)
  und bei neuem `Detail` ergänzt.
- `Host` bekommt `MarkSeen(id int)` und `MarkUnseen(id int)`; `Embedded` setzt das Flag,
  `Remote` reicht es über das Wire-Protokoll durch, `mtuid` bedient es.
- Sperrdisziplin wie bisher: kurz halten, nie über einen PTY-Aufruf, nicht unter Sperre allokieren.

Warum auf dem Host und nicht im Fenster: Im Daemon-Modus überlebt der Zustand das Fenster, und
`mt ls` sowie der MCP-Server (Teilprojekt 4: „wer wartet?“) brauchen dieselbe Antwort.

### C2 Transport

- Der Datensatz reist **nicht** im Scan-Ereignis mit, sondern als eigenes `EventSessionAttention`
  → `terminal:attention` (`rank`, `seen`, `detail`, `source`, `since`). Der Grund: „gesehen“
  ändert sich ohne Zustandswechsel und braucht ohnehin ein eigenes Ereignis. `since` ist im
  Datensatz immer gesetzt; der Hook-Pfad von `terminal:activity` (`onHookActivity`) schickt
  weiter `since=0`, die Sortierung liest aber den Datensatz.
- `SessionSummary` bekommt die `Attention`; `ResendActivity` schickt sie beim Wiederanhängen mit.
- Neue Bindings: `MarkSessionSeen(ref)`, `MarkSessionUnseen(ref)`, beide nehmen einen `hub.Ref`.
- Beide Spiegel (`frontend/wailsjs/go/models.ts`, bei Config-Feldern `stores/config.ts`) werden
  nachgezogen; `go test ./internal/tsmodels/` prüft Felder und Binding-IDs.
- `mt ls` zeigt eine Spalte mit Zeichen und Wartezeit.

### C3 `mtui-hook` liefert den Kurztext

- `hookLine` bekommt `detail`, gefüllt aus `tool_input`: Bash `command`; Edit, Write, MultiEdit,
  NotebookEdit `file_path`; AskUserQuestion `questions[0].question`; WebFetch `url`; sonst leer.
  Gekürzt auf 200 Zeichen.
- Bei Stop bleibt es bei den letzten 600 Zeichen der Antwort; den letzten Absatz bzw. den
  Fragesatz schneidet der Host heraus (neben `endsWithQuestion`, gleiche Absatzlogik).
- Die Umsetzung basiert auf `origin/alpha-main` mit #213, damit die neue Hook-Binary sicher
  ausgerollt wird.

### C4 Zustandskorrekturen

- **Esc bei Freigabe:** `classifyForScan` setzt `waitingPermission` nur bei **positivem Beleg**
  auf `done`: „Interrupted“ steht in den untersten 8 Zeilen, kein Freigabedialog ist zu sehen
  und der letzte Hook ist älter als 2 Sekunden. Eine Ruhezeit wird nicht verlangt. Ein nicht
  erkannter Dialog bleibt bis zum nächsten Hook stehen.
- **`PostToolUseFailure`** → `active`.
- **Hängt und Exit ≠ 0** → `error`, mit `Source` wie in Teil A.
- **Hintergrund-Ereignisse:** Pre-/PostToolUse eines Hintergrund-Agents oder -Befehls dürfen einen
  beendeten Turn (`done`, `waitingAnswer`) nicht auf `active` zurücksetzen. Wie sich diese
  Ereignisse erkennen lassen, klärt ein echter Hook-Mitschnitt (offener Punkt 1).
- **Auto-Suspend** lässt `Attention` unverändert.
- **Dashboard:** `ConfirmedActivity` statt `GetActivity`.
- **Agent-Panes ins richtige Projekt:** `mtui:session-spawned` trägt `projectRoot`
  (`gitx.MainRepoRoot` auf dem Host; ein Worktree zählt zu seinem Repo). Das Frontend ermittelt
  für jeden Tab einmal dessen `projectRoot` (Binding, im Tab zwischengespeichert, nicht
  persistiert) und legt das Pane in den passenden Tab. Ohne Treffer entsteht ein neuer Tab, **ohne
  den aktiven Tab zu wechseln**. Dessen Verzeichnis ist der `projectRoot`; das eigene
  Verzeichnis des Agents nur, wenn es keinen `projectRoot` gibt.

### C5 Frontend

- `lib/attention.ts`, reine Funktionen mit Vitest: `attentionList(tabs, currentTabId)`,
  `collapseDone(list, 3)`, `nextEntry(list, current)`, `tabRank(panes)`.
- `components/WaitingPanel.svelte`: der Kasten; dieselbe Komponente im Aufklapper der
  Werkzeugzeile.
- Gesehen-Melder: Fensterfokus plus fokussiertes Pane, Timer 1000 ms, oder Eingabe über `onData`.
  Er ruft `MarkSessionSeen` nur für Rang 2–3 mit `seen=false`, damit kein Aufrufregen entsteht.
- `TabBar.svelte`: Pane-Zeichen, Rangfarbe, × beim Überfahren, Tooltip.
- `Toolbar.svelte`: Zähler und Aufklapper.
- Toast mit Sprung: Der Toast trägt den `hub.Ref`; der Klick läuft über den Fokus-Listener
  (`app_notify.go`), der zusätzlich ein Ereignis zum Pane-Sprung schickt.
- Kein `$:`-Block schreibt Variablen direkt (Regel aus `CLAUDE.md`); `focusOrders` wird nicht aus
  einem `$:` heraus geschrieben.

### C6 Theme

Die Statusfarben kommen aus der vorhandenen, theme-übergreifenden Statussprache
(`SEMANTIC` in `stores/theme.ts:170-194`), nicht aus neuen Farben je Theme:

| Zustand | Variable |
|---|---|
| Freigabe, Frage | `--status-waiting` (bernstein); unterschieden durch die Form |
| Fehler | `--status-danger` |
| läuft | `--status-running` |
| ruht, fertig gesehen | `--status-idle` |
| fertig, ungesehen | **neu** `--status-done` und `--status-done-tint` |

`--status-done` muss sich in allen sechs Themes (`konzept`, `dark`, `light`, `dracula`, `nord`,
`solarized`) deutlich von `--status-running` unterscheiden, auch im hellen Theme. Gewählt nach
dem Prototyp: **Blau, `--status-done: #4f9cf0`, `--status-done-tint: rgba(79,156,240,.16)`**.

### C7 Verträglichkeit

Ein älterer `mtuid` ohne die neuen Felder: Das Frontend rechnet den Rang dann aus `activity` und
behandelt „fertig“ als gesehen. Das entspricht dem heutigen Verhalten und erzeugt keine Flut
alter Einträge.

## Teil D: Design-Briefing und Prototyp

Der erste Schritt der Umsetzung ist ein **klickbarer HTML/CSS-Prototyp**, den Patrick freigibt,
bevor Svelte-Code entsteht. Ein erster Stand liegt unter
`docs/superpowers/specs/assets/2026-10-07-leitstand-prototyp.html`. Das Briefing gilt für den Prototyp, egal ob ihn Claude Code baut oder
Claude Design als zweite Meinung.

**Ergebnis**
- Eine HTML-Datei, 1920×1080, kein Figma.
- Farben ausschließlich als CSS-Variablen nach MTUI-Schema (`--bg`, `--fg-muted`, `--accent` …)
  plus die Statussprache aus C6 mit dem neuen `--status-done`, umschaltbar für alle sechs Themes
  mit den echten Werten aus `stores/theme.ts`.
- Je Ansicht ein PNG als Referenz.

**Ansichten**
1. Fokus-Layout mit dem Kasten: alle vier Zustände, „hier“-Markierung, „+3 weitere fertig“
2. Kasten leer (eine Zeile) und voll (scrollt, höchstens 40 % der Spalte)
3. Auswahl per Tastatur (Ctrl+Shift+J), Hover mit `Source`, Rechtsklickmenü
4. Tab-Leiste mit zehn Tabs: Pane-Zeichen, Rangfarbe, aktiver Tab, × beim Überfahren
5. Zähler „4 warten“ in der Werkzeugzeile mit Aufklapper
6. Taskleisten-Overlay 16×16 px: 1–9 und „9+“

**Vorgaben**
- Zeichen für sieben Zustände (Freigabe, Frage, Fehler, fertig ungesehen, fertig gesehen,
  läuft, ruht), unterscheidbar bei 10 px und **ohne Farbe**. Die Form trägt die Bedeutung.
- Maße: Tab etwa 190 px bei zehn Tabs, rechte Spalte etwa 380 px, Kasteneintrag zwei Zeilen.
- Bewegung: einmaliges Aufleuchten bei neuem Rang 0–1; das bisherige Aufleuchten des
  Pane-Rahmens bleibt.
- Deutsche Texte; `--font-sans` (IBM Plex Sans) für die Oberfläche, `--font-mono`
  (JetBrains Mono) nur im Terminal und für Zahlen wie Kosten und Wartezeit.

## Nicht im Umfang

- Freigabe direkt aus dem Kasten, Freigabe-Politik, Kosten- und Schleifenalarm (Teilprojekt 3)
- Review-Karte, Diff-Stats, PR- und CI-Status (Teilprojekt 2)
- Abgelöste Fenster: Sie bekommen keine Zustands-Events, weil `App.svelte:242` vor der
  Registrierung in `:340` aussteigt. Eigenes Issue.
- Codex-Hooks (Codex hat laut Doku `PermissionRequest` und `Stop`), eigenes Issue
- Projekt-Hotkeys, Stummschalten je Projekt, Push aufs Handy
- Vorschau im Grid-Modus
- Der Toast „Issue fertig“ (`app.agentDone`) bleibt unverändert; er gehört zur Issue-Verknüpfung,
  nicht zum Leitstand.
- Toter Code, nur notiert: `AskUserDialog` (`emitAskUserQuestion` ohne Aufrufer),
  `QueueOverview.svelte`, `lib/dashboard.ts`

## Tests

**Go**
- `AttentionRank` für alle Zustände, gesehen und ungesehen
- Zustandswechsel setzt `Seen=false` und `Since`; `MarkSeen`/`MarkUnseen`; über `Remote` und `mtuid`
- `detail` aus `tool_input` je Tool, Kürzung, kaputtes JSON
- Fragesatz bzw. letzter Absatz aus dem Stop-Text, inklusive des Screenshot-Falls
- Esc-Rückfall von `waitingPermission`; `PostToolUseFailure` → `active`; hängt nach 10 min; Exit ≠ 0
- Hintergrund-Ereignis nach Stop setzt nicht auf `active` zurück
- Auto-Suspend behält `Attention`
- `go test ./internal/tsmodels/`

**Vitest**
- `attentionList`: Sortierung nach Rang und Alter, eigener Tab mit „hier“, Zusammenklappen
- `tabRank` in beiden Reihenfolgen ([Frage, läuft] und [läuft, Frage])
- `nextEntry` mit Umlauf, Ctrl+Shift+K-Rücksprung
- Gesehen-Melder: 1 s Fokus, Eingabe, kein Aufruf bei schon gesehen
- Agent-Pane landet im Tab mit passendem `projectRoot`, ohne Tab-Wechsel

## Abnahme (E2E mit echtem Claude CLI)

Drei Projekte als Tabs, Fokus-Layout:

1. Projekt A fordert eine Bash-Freigabe an → Eintrag ▲ mit Befehl erscheint binnen 3 s, Toast,
   Taskleiste zeigt 1. Esc im Pane → der Eintrag verschwindet binnen 3 s. (Die 3 s: Debounce
   1,2 s plus bis zu einem Scan-Takt (0,75 s) plus Ereignisweg.)
2. Projekt B endet mit einer Frage, auf die noch ein Satz folgt → Eintrag ? mit dem Fragesatz.
3. Projekt C wird fertig → Eintrag ✓; nach einer Sekunde Fokus auf dem Pane ist er weg.
4. Ein Pane wird hart beendet (Exit ≠ 0) → Eintrag ✕.
5. Tab-Leiste zeigt in jedem Tab die richtigen Pane-Zeichen; die Rangfarbe stimmt in beiden
   Pane-Reihenfolgen.
6. Ctrl+Shift+J läuft A → B → C in dieser Reihenfolge, Ctrl+Shift+K führt zurück.
7. Pane maximieren → der Zähler „N warten“ erscheint in der Werkzeugzeile.
8. `mt new` in Projekt B, während Projekt A aktiv ist → das Pane erscheint in Tab B, Tab A
   bleibt aktiv.
9. Klick auf einen Toast bei unfokussiertem Fenster → Fenster vorn, richtiges Pane fokussiert.

Bis diese Abnahme gelaufen ist, trägt das Issue `needs-e2e-testing`.

## Offene Punkte für die Umsetzung

1. **Erledigt.** Subagent-Ereignisse tragen `agent_id` (Claude-Code-Hook-Doku); umgesetzt als
   `subagentWorkAfterTurn`. Ursprüngliche Frage, zur Nachvollziehbarkeit: Woran im Hook-Payload
   ein Ereignis eines Hintergrund-Agents oder -Befehls zu erkennen ist (etwa ein `agent_id`), mit
   einem echten Mitschnitt klären, nicht annehmen. Lässt es sich nicht unterscheiden, setzt nach Stop nur
   `UserPromptSubmit` den Zustand auf `active` zurück. Ein Pane, das danach von selbst
   weiterarbeitet, steht dann bis zum nächsten Stop als „fertig“ da; das ist das kleinere Übel
   als eine verschluckte Frage.
2. **Taskleisten-Overlay:** Umgesetzt über `pkg/w32` (`ITaskbarList3::SetOverlayIcon`), nur im
   Hauptfenster. **Offen bleibt die Sichtprüfung** am echten Fenster; schlägt sie fehl, gilt der
   Titel-Rückfall aus B5.
3. **Freigabedialog am Bildschirm erkennen** für den Esc-Rückfall: welches Muster der aktuellen
   Claude-Code-Version zuverlässig ist.
4. **Gesehen nach Neustart:** Im Embedded-Modus starten Panes nach einem App-Neustart neu; ihr
   Zustand gilt dann als gesehen. Im Daemon-Modus behält der Host das Flag.

## Marktbild (Quellen der Recherche, Stand Oktober 2026)

- herdr: `done` ist ein Aufmerksamkeitszustand und wird beim Ansehen zu `idle`; Zustand vererbt
  sich Pane → Tab → Workspace. https://raw.githubusercontent.com/herdrdev/herdr/v0.9.3/docs/next/website/src/content/docs/agents.mdx
- Claude Code Agent View: Gruppen Ready for review, Needs input, Working, Completed;
  Peek mit direkter Antwort. https://code.claude.com/docs/en/agent-view.md
- Codex: „Next chat needing attention“ (Ctrl+Alt+A), Priorität Needs input > Blocked > Ready >
  Running. https://learn.chatgpt.com/docs/notifications.md, https://learn.chatgpt.com/docs/pets.md
- cmux: gelesen erst bei fokussierter App, aktivem Tab und fokussiertem Pane.
  https://mintlify.wiki/manaflow-ai/cmux/features/notifications
- Cursor: Badge und Sidebar zählten nach verschiedener Logik (Bug).
  https://forum.cursor.com/t/dock-menu-bar-badge-count-does-not-match-agents-sidebar-needs-attention-glass/168982
- Antigravity: Entfernen der zentralen Inbox löste Protest aus.
  https://discuss.ai.google.dev/t/feature-request-restore-the-inbox-tab-add-markdown-preview-in-agent-manager/128740
- Superset: Der Engpass ist das Review, nicht die Orchestrierung.
  https://superset.sh/blog/roadmap-to-100-agents
