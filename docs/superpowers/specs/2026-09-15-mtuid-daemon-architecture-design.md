# mtuid: Sessions überleben die GUI (Daemon-Architektur) — Design

**Status:** Phase 1, 2a, 2b und 3 umgesetzt, Standard bleibt `embedded` bis zur Durchsatzmessung
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
| tmux-Shim-API, Agent-Control-MCP-Server | `app_tmux_api.go`, `app_mcp_server.go` (beide inzwischen beim Host: `internal/hub/embedded_shim.go`, `internal/mcpsrv`) |
| Suspend/Wake, Keepalive, Queue (Phase 2) | `app_suspend.go`, `app_keepalive.go`, `app_queue.go` |

**GUI-Client (Wails):** Git, Issues, Kanban/Orchestrator, Chat, Worktrees, Dateibrowser,
Dashboard, Einstellungen, Fensterverwaltung, Rendering.

Der Orchestrator steuert Sessions, wandert aber nicht mit: er ruft dieselben Methoden auf,
die jetzt über den Client gehen. Dass ein Kanban-Board auch ohne offene GUI weiterarbeitet,
ist ein eigener Schritt (Phase 5) und braucht mehr als einen Prozesswechsel, nämlich eine
Entscheidung darüber, wer bei einer Eskalation gefragt wird.

**tmux-API und MCP-Server gehören in den Daemon, nicht in die GUI.** Sie waren an den
Fensterprozess gebunden, und genau das war der Fehler: ein Agent, der über MCP eine Session
öffnet, verlor sie beim nächsten GUI-Neustart. Seit Phase 2a beziehungsweise 2c überleben
sie ihn.

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

Die erste Fassung dieser Spec hatte die Reihenfolge falsch. Sie sah vor, die GUI in
Phase 1c auf den Daemon umzuschalten und Scan, Hooks, Queue, Keepalive und Suspend erst
danach umzuziehen. Das geht nicht: 44 Stellen in 20 Backend-Dateien greifen direkt auf
`*terminal.Session` zu. Sobald die Sessions im Daemon liegen, hat der Scan-Loop in der GUI
keinen Screen mehr zu lesen, der Suspend keinen Prozess zu töten und die Queue keinen
Zustand, an dem sie sich ausrichtet. Zwischen "alles lokal" und "alles im Daemon" gibt es
bei diesen Schleifen keinen halben Schritt.

Die Reihenfolge ist deshalb umgedreht: **erst umziehen, solange alles noch im selben
Prozess läuft, dann den Transport umschalten.** Jeder Zwischenstand bleibt lauffähig, und
der riskante Schritt (Prozesswechsel) trifft am Ende auf Code, der schon nicht mehr weiß,
wo seine Sessions liegen.

| Phase | Inhalt | Ergebnis | Stand |
|---|---|---|---|
| 1a | `internal/hub`: Protokolltypen, `Host`, `Embedded`, Ringpuffer mit Offsets | nichts sichtbar | **fertig** |
| 1b | `cmd/mtuid`: Daemon mit HTTP und WebSocket, Discovery, Sperre, Idle-Shutdown; `hub.Remote` mit Reconnect | Daemon läuft, noch ungenutzt | **fertig** |
| 1c | Sessionbesitz im Backend auf `hub.Embedded`; Output über Ring und Subscription statt `collectOutput` | unverändertes Verhalten, ein Besitzer | **fertig** |
| 1d | Scan, Hooks, Statusline, Suspend, Keepalive, Queue über den Host; `internal/backend` importiert `internal/terminal` nicht mehr | unverändertes Verhalten, GUI ohne Sessionwissen | **fertig** |
| 1e | `session_host: daemon`: GUI startet den Daemon, verbindet sich, Restore hängt wieder an statt neu zu starten | **App schließen und öffnen, Agents laufen weiter** | **fertig** |
| 2a | Scan, Hook-Leser und die Shim-Endpunkte (Statusline, tmux) laufen auf dem Host | Zustand wird auch ohne offenes Fenster fortgeschrieben; `MTUI_PORT` bleibt gültig | **fertig** |
| 2b | Sessions im Daemon anlegen (`CreateSpec.Launch`, `hub.Launcher`, `internal/launch`) | der Daemon startet Agents selbst, `mt new` | **fertig** |
| 2c | Debounce, Queue, Wecken, Keepalive und der MCP-Server beim Host | eine eingereihte Aufgabe und eine delegierte Session laufen ohne offenes Fenster weiter | **fertig** |
| 3 | `mt` als CLI-Client (`ls`, `read`, `send`, `keys`, `wait`, `kill`, `hub`) | Sessions ohne GUI bedienbar | **fertig** |
| 4 | Remote-Hubs über SSH | Laptop und Server in einer Oberfläche | offen |
| 5 | Kanban-Orchestrator headless, weitere Agent-CLIs, Plugin-Hooks | Boards laufen ohne offenes Fenster | offen |

`session_host: embedded` bleibt der Standard, bis die Durchsatzmessung aus dem
Risiko-Abschnitt auf einer echten Windows-Maschine gemacht ist. Die Messung selbst gibt es
jetzt als Benchmark:

```
go test ./internal/hub/ -run '^$' -bench Throughput -benchtime 5x
```

Acht Panes, jedes 4 MiB Ausgabe, gemessen wird, was beim Leser ankommt. Auf Linux:

```
BenchmarkThroughput_Embedded   24.73 MB/s
BenchmarkThroughput_Remote     23.64 MB/s
```

Also **4,4 % Kosten für den Socket**. Das ist wenig genug, dass der Standardwechsel eine
Frage der Windows-Zahl und der GUI-CPU ist, nicht mehr eine des Protokolls. Was der
Benchmark nicht beantwortet: die CPU der Oberfläche. Der Verbraucher ist dort eine
Goroutine, nicht ein WebView, der in xterm.js schreibt, und das Coalescing dazwischen liegt
in `internal/backend`. Dafür braucht es die App mit acht beschäftigten Panes. `daemon` ist der Schalter für
den, der es ausprobieren will; schlägt irgendetwas daran fehl (kein `mtuid` neben der
App, kein Record, falsche Protokollversion), fällt die App auf den eingebetteten Host
zurück und schreibt eine Warnung, die über `CheckHealth` in der Oberfläche landet.

**Was im Daemon-Modus heute noch an der offenen GUI hängt:** nichts mehr, was eine Session
am Leben hält. Scan, Hook-Leser, Shim-Endpunkte, Anlegen, Wecken, Queue, Keepalive und der
Agent-Control-MCP-Server liegen beim Host. Im Fenster bleibt, was einen Tab braucht: der
Worktree-Finish-Flow, das Anlegen eines Panes, und das Zeichnen eines Panes für eine
Session, die ein Agent delegiert hat.

Dieses Zeichnen läuft über das Ereignis des Hosts, nicht über einen Aufruf im Fenster. Eine
Session trägt seit 2c, wer sie angefragt hat (`CreateSpec.Origin`, zurück in
`SessionSummary`); `EventSessionCreated` mit `OriginAgent` ist für das Fenster das Signal,
einen Pane dafür anzulegen. Eine Map im Fenster konnte das nicht beantworten: im
Daemon-Modus kann die Delegation passiert sein, bevor dieses Fenster überhaupt lief. An
derselben Markierung hängt die Idle-Suspend-Sperre, die eine delegierte Session in Ruhe
lässt.

Wer den MCP-Server serviert, entscheidet sich danach, wem die Sessions gehören: im
Daemon-Modus `mtuid`, sonst das Fenster. Beide veröffentlichen den Port über
`internal/discovery`, und die Registrierung bei der Claude-CLI liest ihn von dort. Das
Fenster entfernt beim Beenden nur den Record, den es selbst geschrieben hat: der Daemon
veröffentlicht einmal beim Start, und ein gelöschter Record wäre für den Rest seines Lebens
nicht mehr auffindbar.

Beim Keepalive ist eine Hälfte bewusst im Fenster geblieben. Der periodische Anstoß gehört
zum Host, weil er im Fenster genau dann aufhört, wenn er gebraucht wird. Das Anlegen eines
Panes, falls gar keine Claude-Session existiert, bleibt dort: es hängt einen Pane in einen
Tab, und einen Tab hat nur ein Fenster. Scan, Hook-Leser und die
Shim-Endpunkte sind seit Phase 2a beim Host und laufen durch; Sessions anlegen kann der
Daemon seit Phase 2b selbst.

**Korrektur zu einer früheren Fassung dieser Spec.** Hier stand, Phase 2b hänge daran,
dass der Daemon die Umgebung einer Session nicht bauen könne, das sei "GUI-Politik". Das
war falsch, und die Messung hat es widerlegt. `sessionEnv` waren zwanzig Zeilen, und was
sie brauchten, war: der Shim-Port (kam schon vom Host), ein Stringvergleich auf den Modus,
zwei Git-Aufrufe und ein Config-Feld. Kein Fensterzustand, nirgends. Blockiert hat nicht
die Architektur, sondern dass der Code in einem Paket lag, das der Daemon nicht
importieren kann. Der Schnitt war entsprechend klein: `internal/gitx` für die
Git-Plumbing, `internal/launch` für die Policy, und im Backend bleiben Delegate-Namen
stehen, damit fünfzehn Aufrufstellen unberührt bleiben.

Die Lehre daraus ist keine über Daemons: bevor man einen Blocker in eine Spec schreibt,
zählt man nach, woran er wirklich hängt.

Beim Shim war es keine Bequemlichkeit, sondern ein Muss: `MTUI_PORT` wird beim Start in
die Umgebung der Session gebacken und kann ihr nie wieder anders gesagt werden. Aus dem
Fenster bedient hätte jede Session, die das Fenster überlebt, für den Rest ihres Lebens
in einen toten Port geschrieben.

## Stand nach Phase 1

Phase 1 ist umgesetzt: mit `session_host: daemon` überleben die Agents das Schließen der
App, und der Restore hängt sich wieder an sie an, statt daneben neue zu starten.

Was steht:

- `internal/hub` mit `Host`, `Embedded`, `Ring`, `Server` und `Remote`. Daemon und GUI
  benutzen dieselbe `Embedded`-Implementierung; `Remote` ist reiner Transport.
- `cmd/mtuid`: Dateisperre als Einzelinstanz-Garantie, Discovery-Record mit Token, HTTP
  für Steuerung, WebSocket für Terminalbytes, Idle-Abschaltung, Logdatei, GUI-Subsystem
  auf Windows. Wird vom Installer und vom Alpha-Release neben die App gelegt.
- `internal/backend` importiert `internal/terminal` nicht mehr. Alles, was es über eine
  Session weiß, fragt es über den Host, und jede dieser Fragen ist auch über einen Socket
  beantwortbar.
- `internal/procs` mit `HideConsole`, `KillProcessTree` und `Detach`, die vorher drei- bis
  viermal im Baum standen oder gar nicht existierten.

Drei Fehler, die beim Bauen aufgefallen sind und mitbehoben wurden:

1. **`terminal.readLoop` hat Ausgabe verworfen.** Das `select` über den Sendevorgang und
   den Prozessende-Guard stand in einer Anweisung. Ein Kind, das mehr schreibt als der
   Leser abholt, hält die Schleife über sein Ende hinaus am Lesen, und ab da sind beide
   Fälle bereit: Go wählt zufällig, also ging ungefähr die Hälfte der restlichen Chunks
   verloren. Betroffen war das Ende der Ausgabe, also genau der Teil, den man liest.
2. **`CheckAskUser` ist beim Aufruf abgestürzt.** Es fragt `PlainTextRows(0, -1)` für „den
   ganzen Bildschirm" ab, und daraus wurde `make([]string, 0, -1)`, was paniced. Ein
   negatives Ende heißt jetzt „bis unten".
3. **`hub.Embedded` konnte das Ende einer Session vor ihrer Entstehung melden**, bei einem
   Prozess, der sofort endet.

Offen und bewusst nicht entschieden: ob der Kanban-Orchestrator langfristig in den Daemon
gehört. Er steuert Sessions, aber er stellt auch Rückfragen. Wer die beantwortet, wenn
kein Fenster offen ist, ist eine Produktfrage und keine Architekturfrage.

## Was von herdr noch fehlt, nach Nutzen sortiert

1. **Remote über SSH** (Phase 4). Ein Tunnel auf den Loopback-Port des entfernten
   Daemons, plus eine Hub-Auswahl im Client. Das Protokoll trägt die Hub-Kennung schon,
   und `mt --hub <name>` ist die Stelle, an der es sichtbar würde. Vorher fällig: die
   Session-Identität von `int` auf `hub:id` umstellen, 166 Fundstellen im Frontend und
   120 in Go. Danach wird es teurer, vorher ist es reine Kosten.
2. **Mehr Agent-CLIs.** herdr startet 22, wir kennen drei. Seit Phase 2b ist das eine
   Tabelle in `internal/launch/agents.go` und ein Kommando in der Config, sonst nichts.
3. **Plugins.** herdr lädt Verzeichnisse mit `herdr-plugin.toml`, Actions und
   Event-Hooks, aus einem Marketplace, der GitHub-Repos mit einem Topic indiziert, ohne
   Sandbox. Reizvoll, aber es ist auch das Stück, das ein Werkzeug von "tut eine Sache"
   zu "ist eine Plattform" macht, mit allem, was daran hängt.

Nicht übernommen, bewusst: herdrs Socket hat **keine Authentifizierung** und verlässt
sich auf Dateirechte. Auf Windows löst das #183 nicht, deshalb bleibt es bei Port 0 plus
Token im Discovery-Record.

## Stand nach Phase 3: die CLI

herdrs eigentliches Alleinstellungsmerkmal ist nicht der Daemon, sondern dass derselbe
Daemon drei Gesichter hat: ein Agent-Skill, eine CLI und der rohe Socket. MTUI hatte das
Fenster und den MCP-Server. `cmd/mt` ist das dritte, und seit Phase 2c hängt keines der
drei an einem offenen Fenster.

```
mt ls                     was der Daemon hält, mit Agent-Zustand
mt read <id> [--follow]   Bildschirm als Text oder als laufender Strom
mt send <id> <text...>    Prompt tippen und abschicken ("-" liest stdin)
mt keys <id> <taste...>   ctrl-c, enter, down, yes, …
mt wait <id>              blockiert bis done oder blocked
mt kill <id>              Sessions beenden
mt hub [--stop]           welcher Daemon läuft, und ihn beenden
```

Zwei Entscheidungen, die nicht offensichtlich sind:

**Die CLI heißt `mt`, nicht `mtui`.** Der Installer legt die GUI als `mtui.exe` ab und
setzt `{app}` auf den PATH. Ein `mtui ls` in der Shell hätte ein Fenster geöffnet. `mt`
passt außerdem zum `.mt-worktrees`-Präfix, das es im Repo schon gibt.

**Die CLI startet keinen Daemon.** Die GUI tut das, weil sie Sessions anlegt; ein Daemon,
den ein vertipptes `mt ls` hochfährt, hätte nichts zu zeigen und bliebe trotzdem stehen.
Stattdessen sagt sie es und beendet sich mit Code 3, den ein Skript abfragen kann.

Die Exit-Codes sind Teil der Schnittstelle: 3 heißt "kein Daemon", 4 heißt "der Agent
arbeitet noch". Ein Skript muss "läuft weiter" von "kaputt" unterscheiden können, und mit
Code 1 für beides ginge das nicht.

Das Warte-Vokabular liegt seit Phase 3 in `internal/hub` statt in `internal/backend`. Die
CLI und das MCP-Tool stellen dieselbe Frage an denselben Host; zwei Implementierungen
wären zwei Definitionen von "fertig".

Drei Fehler, die beim Bauen aufgefallen sind:

1. **`flag.Parse` hört beim ersten Positional auf.** `mt wait 3 --timeout 30s` hat die ID
   gelesen und den Timeout danach kommentarlos ignoriert: gefragt waren 30 Sekunden,
   gewartet wurden fünf Minuten. `parseArgs` schält pro Runde ein Positional ab, damit
   Optionen auf beiden Seiten stehen dürfen.
2. **`.gitignore` Zeile 3 war ein blankes `mtui`.** Das trifft nicht nur die gebaute
   Binary, sondern auch das Quellverzeichnis. Das ganze CLI-Paket war für git unsichtbar.
   Die Root-Binaries sind jetzt mit führendem Slash verankert.
3. **`--help` eines Unterkommandos hat erst den Daemon gewählt.** Es ist damit genau auf
   der Maschine gescheitert, deren Besitzer die Hilfe liest, weil er noch nichts
   eingerichtet hat.

Was die CLI **nicht** kann: Sessions anlegen. Dafür müsste der Daemon die Umgebung einer
Session selbst bauen können (Hook-Verdrahtung, Worktree-Firewall, Session-ID), und das
ist heute GUI-Politik. Es ist derselbe Block, an dem Phase 2b hängt, und deshalb löst man
beides zusammen oder gar nicht.

## Stand nach Phase 2b: der Daemon startet selbst

Bis hierher konnte eine Session nur ein Prozess anlegen, der schon wusste, was "claude"
auf dieser Maschine ist und welche Variablen ein Pane braucht, und das war das Fenster.
Ein Daemon, der Agents hält, aber keinen machen kann, ist ein halber Daemon: delegieren
ging nur bei offenem Fenster, und `mt` konnte jede Session bedienen, aber keine starten.

```
CreateSpec.Launch   nennt ein Tool statt einer Kommandozeile
hub.Launcher        Argv(tool, model) und Env(id, dir, mode)
launch.Policy       implementiert das, aus Config und Git
mt new <tool>       startet eine und gibt die ID aus
```

`hub.Launcher` ist eine Schnittstelle und kein Import von `internal/launch`, weil dieses
Paket Sessions besitzt und jenes Policy; der Pfeil zeigt in eine Richtung. Ein Host ohne
Launcher lehnt eine Launch-Anfrage ab, statt etwas halb konfiguriert zu starten. Das
klingt pedantisch und ist es nicht: ein Pane ohne `MULTITERMINAL_SESSION_ID` startet
tadellos und hat überhaupt keine Hook-Verdrahtung.

Die ID wird vor der Umgebung reserviert, weil ein Teil dieser Umgebung die Session
benennt. Genau deshalb trägt `CreateSpec` beides, `ID` und `Launch`, statt einen Callback
zu nehmen, der den Socket nicht überlebt hätte.

Der Daemon liest die Config pro Start, nicht einmal beim Hochfahren. Eine YAML-Datei ist
nichts neben den zwei Git-Subprozessen, die ein Start ohnehin kostet, und dafür gilt eine
Einstellung, die jemand in der laufenden App ändert, für die nächste Session, ohne einen
Daemon neu zu starten, der lebende Agents hält.

Gegengeprüft gegen einen echten `mtuid`, mit Stub-`claude` und `force_worktrees: true`:

```
$ mt new claude --dir ~/Multiterminal-UI
1
$ mt read 1
session=1 port=44727
worktree_root=/home/user/Multiterminal-UI
```

Session-ID, der eigene Shim-Port des Daemons und die Worktree-Firewall, aufgelöst aus
Config und Repo. Ohne Fenster.
