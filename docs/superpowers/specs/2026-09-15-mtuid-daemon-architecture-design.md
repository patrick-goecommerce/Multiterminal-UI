# mtuid: Sessions überleben die GUI (Daemon-Architektur) — Design

**Status:** Entwurf
**Branch:** `claude/mtui-herdr-dev-migration-lz2rze`
**Vorbild:** [herdr](https://herdr.dev/) 0.9, Rust, Apache-2.0
**Betrifft:** `internal/backend/app.go`, `internal/terminal`, `internal/discovery`, neu `internal/hub` und `cmd/mtuid`

## Problem

Wer die MTUI-Fenster schließt, tötet seine Agents. `AppService.sessions` ist eine Map im
Wails-Prozess (`app.go:33`), `ServiceShutdown` läuft über alle Sessions und ruft `Close()`
(`app.go:168-182`). Was der Nutzer beim nächsten Start sieht, ist kein Wiederanknüpfen,
sondern ein Neustart: `SavedPane` merkt sich Modus, Modell, Verzeichnis und Worktree
(`internal/config/session.go:34`), und daraus wird ein neuer Prozess gestartet. Der
Bildschirminhalt ist weg, der Agent fängt von vorn an.

Das trifft vier Fälle, die im Alltag ständig vorkommen:

1. **Update.** Der In-App-Updater beendet den Prozess mit `os.Exit(0)` (#184). Vier laufende
   Claude-Panes sind danach vier abgebrochene Läufe.
2. **Absturz der GUI.** Ein Fehler im Frontend oder in der WebView reißt die Agents mit.
   Die Crash-Erkennung (`app_health.go`) kann nur melden, nicht retten.
3. **Langläufer.** Ein Kanban-Wave über mehrere Stunden bindet das Fenster an den Rechner.
   Bildschirm sperren geht, ausloggen nicht.
4. **Zweiter Client.** Multi-Window (Wails v3) sind Fenster eines Prozesses. Ein CLI-Client
   oder ein zweiter Rechner sind damit grundsätzlich nicht möglich.

herdr löst das an genau einer Stelle: dort läuft ein Hintergrunddienst, dem die Terminals
gehören, und der Client hängt sich dran. Alles weitere, was herdr kann und MTUI nicht
(Reattach mit Verlauf, Remote-Maschinen über SSH, CLI als gleichwertiger Client, mehrere
Clients gleichzeitig), fällt aus dieser einen Entscheidung ab.

Umgekehrt hat MTUI bereits, was bei herdr die Agent-Schicht ausmacht: Statuserkennung über
Hooks und VT100-Scan (`internal/terminal/activity.go`), eine Control-API für Agents
(`app_mcp_server.go`: `open_session`, `send_input`, `read_output`, `close_session`,
`list_sessions`), den tmux-Shim, Suspend/Resume mit `claude --resume`
(`session_suspend.go`). Diese Spec baut davon nichts um. Sie verschiebt nur, in welchem
Prozess es läuft.

## Ziel

Nach Phase 1 gilt:

- MTUI schließen und neu öffnen lässt die Agents laufen. Die Panes sind nach dem Start
  wieder da, mit Bildschirminhalt und laufendem Prozess.
- Der Daemon (`mtuid`) besitzt PTY, Screen-Buffer, Aktivitäts-Scan und die Hook-Auswertung.
- Die Wails-App ist ein Client. Ihr Svelte-Frontend bleibt unverändert.
- Alles, was nicht sessiongebunden ist (Git, Issues, Kanban, Chat, Worktrees, Dateien),
  bleibt in der GUI.

Ausdrücklich **nicht** Ziel dieser Phase: Remote-Maschinen, CLI-Client, Plugin-System.
Das Protokoll wird aber so geschnitten, dass Remote später ohne Bruch draufpasst
(siehe „Vorleistung für Remote").

## Schnitt: was gehört dem Daemon

Der Schnitt läuft entlang der Frage „stirbt das sinnvollerweise mit dem Fenster?".

**Daemon (`mtuid`):**

| heute | Datei |
|---|---|
| `sessions`, `launches`, `nextID`, `sessionMode` | `app.go:33-51` |
| PTY-Leseschleife, Screen-Buffer | `internal/terminal/session.go` |
| `collectOutput`, `outputBatcher`, `batchLoop` | `app_stream.go` |
| `scanLoop` (Aktivität, Tokens, Kosten) | `app_scan.go` |
| Hook-Auswertung (tailt die JSONL-Dateien) | `app_hooks.go` |
| Statusline-Empfang | `session_statusline.go` |
| tmux-Shim-API, Agent-Control-MCP-Server | `app_tmux_api.go`, `app_mcp_server.go` |
| Suspend/Wake, Keepalive, Queue (Phase 2) | `app_suspend.go`, `app_keepalive.go`, `app_queue.go` |

**GUI-Client (Wails):** Git, Issues, Kanban/Orchestrator, Chat, Worktrees, Dateibrowser,
Dashboard, Einstellungen, Fensterverwaltung, Rendering.

Der Orchestrator steuert Sessions, wandert aber nicht mit: er ruft dieselben Methoden auf,
die jetzt über den Client gehen. Dass ein Kanban-Board auch ohne offene GUI weiterarbeitet,
ist ein eigener Schritt (Phase 5) und braucht mehr als einen Prozesswechsel, nämlich eine
Entscheidung darüber, wer bei einer Eskalation gefragt wird.

**tmux-API und MCP-Server gehören in den Daemon, nicht in die GUI.** Sie sind heute an
den Fensterprozess gebunden, und genau das ist der Fehler: ein Agent, der über MCP eine
Session öffnet, verliert sie beim nächsten GUI-Neustart. Nach dem Umzug überleben sie.

Die Hooks brauchen keine Umleitung. `mtui-hook` schreibt JSONL-Dateien nach
`%APPDATA%\Multiterminal\hooks` (`cmd/mtui-hook/main.go:96`), niemand postet an einen Port.
Es zieht nur der Leser um.

## Der Seam: `sessionHost`

Der Umbau ist zu groß für einen Sprung. Deshalb wird er als **Modus** gebaut, nicht als
Fork: dieselbe Codebasis fährt beides.

```go
// internal/hub
type Host interface {
    Info() Info
    Create(spec CreateSpec) (int, error)
    Write(id int, data []byte) error
    Resize(id, rows, cols int) error
    Close(id int) error
    List() []SessionSummary
    Get(id int) (SessionSummary, error)
    Repaint(id int) ([]byte, error)          // ANSI-Repaint, heute ResyncSession
    Attach(id int, from int64) (*Subscription, error)
    Shutdown()
}
```

IDs sind lokal zu einem Host. Die Hub-Kennung kommt erst an der Leitungsgrenze dazu
(`Ref{Hub, ID}`), damit ein Client, der später mehrere Hubs sieht, zwei Sessions mit der
Nummer 3 auseinanderhalten kann. Ereignisse laufen nicht über einen Kanal am Interface,
sondern über einen `EventSink`, den der Host bei der Konstruktion bekommt: der Daemon
fächert damit an alle Clients auf, die GUI schiebt sie in Wails weiter, und kein Aufrufer
muss einen Kanal leerlesen, den er nicht braucht.

Zwei Implementierungen:

- **`hub.Embedded`** hält die heutige Map im eigenen Prozess. Verhalten identisch zu heute,
  Ereignisse gehen direkt an `a.app.Event.Emit`. Das ist der Standard, bis Phase 1 stabil
  ist, und bleibt danach als Rückfallebene erhalten.
- **`hub.Remote`** spricht den Daemon über Loopback an.

`AppService` behält alle heutigen Methoden (`CreateSession`, `WriteToSession`, …) als dünne
Weiterleitung an `Host`. Damit ändert sich für das Frontend und für die 326 Go-Dateien
darüber nichts, und die bestehenden Tests konstruieren weiterhin ein `AppService` mit
eingebettetem Host.

Konfiguration: `session_host: embedded | daemon` in `~/.multiterminal.yaml`. Wie bei jedem
neuen Config-Feld muss das Feld zusätzlich von Hand in
`frontend/wailsjs/go/models.ts` nachgezogen werden, sonst verschluckt Wails es beim
Deserialisieren (siehe CLAUDE.md, wiederkehrender Fehler).

## Transport

**Loopback-TCP mit HTTP/1.1 und WebSocket, Port 0, veröffentlicht über `internal/discovery`
als neuer Service `hub`, Token in jedem Request.**

Begründung, warum nicht Named Pipe oder Unix-Socket: `discovery` löst bereits das Problem,
an dem feste Ports scheitern (Ports sind maschinenweit, nicht pro Anmeldesitzung; auf einem
RDP-Host gewinnt die erste Instanz, #183). Focus-Listener und MCP-Server folgen diesem
Muster schon. Ein zweites Transportmuster daneben wäre eine zweite Fehlerquelle mit
eigener Windows-Semantik. Und für Remote ist ein TCP-Port das, was ein SSH-Tunnel ohnehin
weiterreicht.

Das Token ist Pflicht, nicht Zierde: Erreichbarkeit ist keine Identität (CLAUDE.md). Jeder
Request trägt `Authorization: Bearer <token>` aus dem Discovery-Record, der Daemon prüft
es mit `subtle.ConstantTimeCompare` und antwortet sonst mit 401.

### Kontrollpfad (HTTP, JSON)

```
GET    /v1/hub                      → {hub_id, version, pid, started_at, protocol}
GET    /v1/sessions                 → [SessionSummary]
POST   /v1/sessions                 {argv, dir, rows, cols, mode} → {id}
DELETE /v1/sessions/{id}
POST   /v1/sessions/{id}/input      {data}        (base64)
POST   /v1/sessions/{id}/resize     {rows, cols}
POST   /v1/sessions/{id}/suspend
POST   /v1/sessions/{id}/wake
GET    /v1/sessions/{id}/screen     → ANSI-Repaint
```

### Datenpfad (WebSocket `/v1/stream`)

Eine Verbindung pro Client, alle Sessions gemultiplext.

- **Textframes** tragen JSON-Steuernachrichten in beide Richtungen: `attach`, `detach`,
  `session.created`, `session.exited`, `session.activity`, `session.tokens`,
  `session.title`, `hub.shutdown`.
- **Binärframes** tragen Terminal-Output. Header: 1 Byte Typ, 4 Byte Session-ID
  (Big Endian), 8 Byte Offset, danach die Rohbytes.

Kein Base64 im Datenpfad. Heute kostet der Weg zum Frontend eine
Base64-Kodierung pro Batch (`app_stream.go`, `TerminalOutputEvent.Data`); über einen
Binärframe entfällt sie auf der Daemon-Seite und wird erst im Client für den bestehenden
Wails-Event wieder aufgebaut. Der 16-ms-Batcher zieht mit in den Daemon um: er ist genau
dafür da, den teuren Übergang zu bündeln, und der teure Übergang ist jetzt der Socket.

### Offsets und lückenloses Reattach

Jede Session zählt ihre ausgegebenen Bytes monoton hoch. Der Daemon hält pro Session einen
Ringpuffer roher PTY-Bytes (Standard 2 MiB, `daemon_scrollback_bytes`).

`attach` trägt einen Offset:

- `from: -1` (Neuanhängen): Der Daemon schickt den Ringpuffer als Replay und danach live.
  Das ist herdrs „pane history replay" und der Grund, warum sich Reattach richtig anfühlt:
  xterm.js bekommt echten Verlauf, nicht nur das aktuelle Bild.
- `from: <offset>` (Reconnect nach Verbindungsabbruch): Ab genau dort weiter. Liegt der
  Offset außerhalb des Ringpuffers, antwortet der Daemon mit `truncated: true` und
  schickt Repaint plus Ringpuffer. Der Client leert dann das xterm-Buffer, statt ein Loch
  zu übermalen. Das ist dieselbe Überlegung wie bei `outputBatcher.replaceWith`
  (`app_stream.go:68-81`): eine Lücke im Strom ist schlimmer als doppelte Bytes.

Mehrere Clients dürfen gleichzeitig an derselben Session hängen, jeder mit eigenem Offset.
Eingaben aller Clients gehen in dieselbe PTY, ungefiltert. Wer zwei Fenster auf dasselbe
Pane richtet, tippt in dasselbe Terminal. Das ist bei tmux so und soll hier nicht klüger
sein.

## Lebenszyklus des Daemons

**Start.** Die GUI löst beim Hochfahren `discovery.Resolve(ServiceHub)` auf. Kein Record
oder Record veraltet (PID weg): sie startet `mtuid` abgelöst vom eigenen Prozess und wartet
bis zu 5 Sekunden auf dessen Record. Auf Windows heißt „abgelöst" `DETACHED_PROCESS` und
kein gemeinsames Job-Objekt, sonst reißt das Ende der GUI den Daemon mit.

**Kein Konsolenfenster.** `mtuid` wird wie `mtui-hook` und `mtui-statusline` als
GUI-Subsystem gelinkt (`-H=windowsgui`) und bekommt denselben Subsystem-Test. Zusätzlich
gilt die Regel aus CLAUDE.md unverändert weiter: jeder Nicht-PTY-Kindprozess im Daemon ruft
vorher `hideConsole(cmd)`. Die PTY-Sessions selbst sind davon ausgenommen, ConPTY hat kein
Fenster.

**Wettlauf beim Start.** Zwei GUIs, die gleichzeitig hochfahren, dürfen nicht zwei Daemons
starten. Der Daemon nimmt vor dem Binden eine exklusive Dateisperre im Runtime-Verzeichnis
(`os.UserCacheDir()/mtui/hub.lock`, Muster wie `internal/board/lock.go`). Wer die Sperre
nicht bekommt, beendet sich sofort mit 0; der Aufrufer wartet auf den Record des Gewinners.

**Veralteter Record ist der Normalfall, nicht die Ausnahme** (CLAUDE.md): Absturz und
`os.Exit(0)` des Updaters überspringen das Aufräumen. `discovery.Resolve` prüft deshalb die
PID, und der Client behandelt einen 401 oder einen fehlschlagenden `GET /v1/hub` wie „kein
Daemon da".

**Ende.** Der Daemon beendet sich nicht, wenn der letzte Client geht. Das ist der Zweck.
Er beendet sich, wenn er keine Sessions mehr hat und seit `daemon_idle_shutdown`
(Standard 10 Minuten) kein Client verbunden war, sowie auf `POST /v1/hub/shutdown`.
Sessions laufen also nur, solange es Sessions gibt, und ein leerer Daemon bleibt nicht
für immer im Speicher stehen.

**Neustart des Rechners** beendet die PTYs ohnehin. Der Daemon persistiert daher keine
Prozesse, sondern nur seinen ID-Zähler und die Startspezifikation je Session, damit ein
neu gestarteter Daemon für Claude-Panes `--resume` anbieten kann. Das ist genau die
Information, die heute in `SavedPane` steht; sie wandert in Phase 2 vom GUI-Client in den
Daemon.

**Version.** Client und Daemon können nach einem Update auseinanderlaufen. `GET /v1/hub`
liefert `protocol` als Ganzzahl. Passt sie nicht zur Erwartung des Clients, zeigt die GUI
den Hinweis an, dass der Daemon neu gestartet werden muss, und bietet den Neustart an.
Sie verbindet sich nicht mit einem Protokoll, das sie nicht kennt, und sie startet den
Daemon auch nicht eigenmächtig neu: dabei stürben die laufenden Agents, und das ist die
eine Sache, die diese Architektur verhindern soll.

## Ereignisse: `EventSink`

Der Daemon hat kein Wails. Heute steht `a.app.Event.Emit(...)` verstreut im Backend.
Zwischen beide kommt ein Interface:

```go
type EventSink interface{ Emit(name string, payload any) }
```

- Im Daemon fächert der Sink an alle verbundenen Clients auf (Textframe auf der
  WebSocket-Verbindung).
- Im GUI-Client nimmt der Empfänger den Frame entgegen und ruft `a.app.Event.Emit` mit
  demselben Namen und derselben Nutzlast auf.

Dadurch ändert sich im Svelte-Code nichts: `terminal:output-batch`, `terminal:activity`
und der Rest kommen weiter als Wails-Events an, nur aus einem anderen Prozess. Das ist der
Trick, der den Frontend-Diff dieser Phase auf nahe null hält.

## Vorleistung für Remote

Remote ist nicht Teil dieser Phase, aber zwei Entscheidungen fallen jetzt, weil sie später
teuer wären:

1. **Jeder Daemon hat eine `hub_id`** (zufällig, beim ersten Start erzeugt, persistiert).
   Das Protokoll adressiert eine Session als Paar `{hub, id}`, nicht als nackte Zahl.
2. **Die GUI bleibt in Phase 1 bei `int`-IDs.** Frontend, `SavedPane`, xterm-Verwaltung und
   die MCP-Werkzeuge rechnen durchgehend mit Ganzzahlen; das auf zusammengesetzte Referenzen
   umzustellen wäre ein eigener, großer Umbau ohne Nutzen, solange es nur einen Hub gibt.
   Der Client bildet deshalb `{hub, id}` auf eine lokale Ganzzahl ab. Kommt ein zweiter Hub
   dazu, wächst die Tabelle, das Protokoll bleibt.

Der Weg zu Remote ist danach: `mtuid` auf dem entfernten Rechner, SSH-Tunnel auf dessen
Loopback-Port, Discovery-Record vom entfernten Rechner über denselben Kanal holen. Kein
zweites Protokoll.

## Risiken

**Durchsatz.** Heute geht Output über einen Go-Channel im selben Prozess. Künftig über
einen Loopback-Socket. Bei vollem Bildschirmneuaufbau in mehreren Panes gleichzeitig ist
das messbar. Gegenmaßnahmen sind eingebaut (Batcher im Daemon, Binärframes statt Base64),
aber die Annahme muss geprüft werden, bevor `daemon` Standard wird. Messpunkt: acht Panes,
`yes` in allen, Bytes pro Sekunde und CPU-Last der GUI gegen den heutigen Stand.

**Windows.** Der riskanteste Teil ist nicht das Protokoll, sondern die Prozesshygiene:
abgelöster Start, kein gemeinsames Job-Objekt, kein Konsolenfenster, ConPTY in einem
Prozess ohne Fenster. Der Statusline-Shim und die Chat-Spawns sind an genau dieser Stelle
schon zweimal mit einem aufblitzenden Fenster ausgeliefert worden.

**Verwaiste Sessions.** Ein Daemon ohne GUI, an den sich nie wieder jemand hängt, hält
Agents am Leben, die niemand ansieht. Dagegen steht der Idle-Shutdown (leer und ohne
Client) und, ab Phase 3, `mtui ls` und `mtui kill` als Sichtbarkeit von außen.

**Doppelte Wahrheit.** Solange beide Hosts existieren, gibt es zwei Codepfade für
Sessionbesitz. Sie dürfen nicht auseinanderlaufen. Deshalb liegt die Logik in
`hub.Embedded`, und `hub.Remote` ist reiner Transport ohne eigene Zustandsführung.

## Phasen

| Phase | Inhalt | Ergebnis für den Nutzer |
|---|---|---|
| 1a | `internal/hub`: Protokolltypen, `Host`-Interface, `Embedded` | nichts sichtbar, Verhalten identisch |
| 1b | `cmd/mtuid`: Daemon mit HTTP und WebSocket, Discovery, Sperre | Daemon läuft, noch ohne Nutzung |
| 1c | `hub.Remote`, GUI startet und verbindet den Daemon, `session_host: daemon` | GUI schließen und öffnen, Agents laufen weiter |
| 1d | Reattach mit Replay, Offsets, Reconnect nach Abbruch | Panes kommen mit Verlauf zurück |
| 2 | Scan, Hooks, Statusline, Queue, Keepalive, Suspend und Sessionstatus in den Daemon | Status und Queue laufen ohne GUI weiter |
| 3 | `mtui` als CLI-Client (`ls`, `attach`, `send`, `kill`) | Sessions ohne GUI bedienbar |
| 4 | Remote-Hubs über SSH | Laptop und Server in einer Oberfläche |
| 5 | Kanban-Orchestrator headless, weitere Agent-CLIs, Plugin-Hooks | Boards laufen ohne offenes Fenster |

Umgestellt wird erst, wenn 1d steht und die Durchsatzmessung passt. Bis dahin bleibt
`session_host: embedded` der Standard, und `daemon` ist ein Schalter für den, der es
ausprobieren will.
