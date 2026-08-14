# Zustandsdauer pro Pane — Design

**Status:** Entwurf, zur Umsetzung freigegeben
**Worktree:** C:\Users\Patrick.Hennig\repos\Multiterminal-UI\.claude\worktrees\activity-duration
**Issues:** #188 (Flackern), #189 (Dauer-Anzeige)
**Basiert auf:** PR #187 (`integration/process-optimization`)

## Problem

Wer drei oder vier Panes eines Projekts offen hat, verliert den Überblick, welches davon schon abgearbeitet ist. Ohne Zeitangabe bleibt nur, jedes Pane erneut zu lesen. Ein sichtbares „vor 3 Std" beantwortet das sofort: das habe ich schon durch. „vor 2 Minuten" heißt: da muss ich noch rein.

Die naheliegende Umsetzung — Zeitstempel bei jedem Wechsel des `Activity`-Werts — funktioniert nicht, weil dieser Wert flackert. Gemessen wurden **vier `done`-Transitionen bei einem einzigen echten Abschluss**. Ein daran gehängter Zeitstempel stünde dauerhaft bei „vor 2 Sek", und der Fehler wäre schwer zu bemerken, weil die Zahl plausibel aussieht.

Das Feature setzt deshalb voraus, dass die Zustandsermittlung zuerst verlässlich wird. Beides gehört in eine Änderung: der Fix trägt das Feature, das Feature begründet die Entprellung.

## Ausgangslage

Vollständige Ursachenanalyse in #188. Zusammengefasst:

1. **Der Lese-Pfad schreibt `Activity` bedingungslos.** Jeder PTY-Chunk setzt `Active` — auch ein Cursor-Hide/Show ohne sichtbare Wirkung, auch wenn ein Hook den Zustand autoritativ gesetzt hat. Die Schwelle ist ein beliebiges Byte, nicht „relevanter Output".
2. **`classifyScreenState` hat keine Hysterese.** Fällt ein Scan-Tick zwischen `ESC[2K` und das Neuzeichnen der Eingabebox, kippt derselbe Bildschirm von `Done` nach `Idle`. Unabhängig von Ursache 1 und von deren Behebung nicht berührt.
3. **`Notification` ohne Fragezeichen mappt auf `Done`.** Texte wie „Claude needs your permission to use Bash" reißen eine laufende Session auf „fertig", ganz ohne PTY-Byte.
4. **Zwei Emit-Pfade ohne gemeinsame Dedup.** Der Hook-Callback emittiert an `prevActivity` vorbei.
5. **Die Frontend-Glättung verlängert die Störung.** `active` wird sofort angewendet, die Rückkehr um 900 ms verzögert — der störende Übergang geht ungebremst durch, die Erholung wird gebremst.

Es gibt keine Rangordnung der Schreiber; der letzte gewinnt. Der Hook-Vorrang existiert nur als Guard auf der Leseseite, den der Byte-Pfad umgeht.

## Entwurf

### Leitgedanke

Ein Zustandswechsel wird an **einer** Stelle festgestellt. Anzeige, Dauer, Queue-Vorschub und Issue-Meldung leiten sich alle davon ab, statt jeweils eigene Vorstellungen davon zu haben, wann sich etwas geändert hat.

### 1. Schreibseite bereinigen

Im Lese-Pfad (`noteOutput` in `internal/terminal/session_spawn.go` nach PR #187; davor `session.go:199`) entfällt `s.Activity = ActivityActive`. `LastOutputAt`, `Title` und die Suspend-Abort-Markierung bleiben unberührt.

Das ist kein Funktionsverlust: `DetectActivity` erzeugt dieselbe `Active`-Transition, solange der Output frisch ist (`activity.go:104-111`), einschließlich des dort dokumentierten Queue-Falls. Der Unterschied ist, dass sie durch den Hook-Guard läuft statt daran vorbei.

**Bewusst nicht gewählt:** eine Bedingung `if !s.hasHookData` — sie löst nur den Hook-Fall, nicht den Fall ohne Hooks. Ebenso wenig eine Erkennung, ob ein Chunk „kosmetisch" ist: gegen eine fremde TUI ist das grundsätzlich unzuverlässig.

**Kosten:** „läuft" erscheint erst beim nächsten Scan-Tick statt beim ersten Byte, also bis zu 750 ms später. Für Anzeige und Queue-Logik unkritisch — letztere reagiert ohnehin nur auf Wechsel im Scan-Loop.

Zusätzlich: `hookEventToActivity` (`app_hooks.go:43-47`) bildet `Notification` ohne Fragezeichen nicht mehr auf `Done` ab — eine Notification sagt nichts darüber aus, ob der Turn zu Ende ist.

„Keine Aussage" braucht dafür einen eigenen Rückgabeweg, den die Funktion heute nicht hat: Ihr Ergebnis ist ein einzelner `ActivityState`, und jeder Wert davon ist eine Behauptung. Die Signatur wird deshalb zu `(terminal.ActivityState, bool)`, wobei `false` heißt: dieses Event trägt keine Zustandsinformation. Der Aufrufer in `app_hooks.go` überspringt dann `SetHookActivity` und den Emit vollständig, statt einen Ersatzwert zu erfinden. `Notification` ohne Fragezeichen ist der erste Fall, der das nutzt; `default` (unbekanntes Event) wird im selben Zug von `ActivityIdle` auf „keine Aussage" umgestellt — ein unbekanntes Event als „idle" zu werten war schon vorher eine Behauptung ohne Grundlage.

### 2. Entprellung im Scan-Loop

Neben `prevActivity` (unter `prevActivityMu`) kommen zwei Maps: `pendingActivity` und `pendingSince`.

Ablauf je Session und Tick:

- Rohzustand gleich dem zuletzt bestätigten → nichts zu tun, `pending` verwerfen.
- Rohzustand weicht ab und unterscheidet sich vom bisherigen `pending` → `pending` neu setzen, `pendingSince = jetzt`.
- Rohzustand weicht ab und ist gleich dem `pending`, und `jetzt - pendingSince >= debounceWindow` → **bestätigter Wechsel**: `prevActivity` aktualisieren, Zeitstempel setzen, emittieren, Nebenwirkungen auslösen.

`debounceWindow` = 1,2 s, als benannte Konstante, nicht als Literal. Der Scan-Tick liegt je nach Sessionzahl bei 500–750 ms; bis ein Wechsel bestätigt ist, vergehen damit zwei bis drei übereinstimmende Beobachtungen und real 1,2–1,75 s.

Das deckt Ursache 2 mit ab: ein einzelner Klassifikations-Aussetzer erreicht die Bestätigungsschwelle nie.

**Gegenrechnung zur Reaktionszeit:** Der 900-ms-Debounce im Frontend entfällt dafür, und der ist heute nicht der einzige Aufschlag — vor der Beruhigung liegt das Zucken selbst, das die Anzeige über mehrere Sekunden unbrauchbar macht. Künftig steht dort nach spätestens 1,75 s ein Wert, der stimmt und stehen bleibt. Sollte sich das im Betrieb träge anfühlen, ist das Fenster eine einzelne Konstante.

Der zweite Emit-Pfad (`app_hooks_setup.go:64-77`) wird auf dieselbe Auswertung geführt, damit nicht weiterhin zwei Quellen unabhängig voneinander emittieren.

### 3. Zeitstempel

`activitySince` (Map `sessionID → time.Time`) liegt neben `prevActivity` unter demselben Mutex und wird **ausschließlich** in der Bestätigungslogik aus Schritt 2 gesetzt — und nur, wenn sich der bestätigte Zustand tatsächlich ändert.

Nie im Lese-Pfad, nie in `SetHookActivity`. Andernfalls reproduziert sich der behobene Bug eins zu eins für den Zeitstempel.

Bewusst im Backend statt in `Session`: die Entprellung wohnt dort, und der ganze Zweck ist, dass es genau eine Stelle gibt. `Session` bleibt unberührt.

### 4. Transport

`ActivityInfo` (`app_scan.go:14-21`) bekommt `ActivitySince int64` — Unix-Sekunden, `0` heißt unbekannt.

Der Transport läuft über das `terminal:activity`-Event. Events werden als JSON serialisiert und brauchen **kein** `models.ts`; diese Pflicht betrifft nur Binding-Rückgaben (siehe Persistenz).

Der Emit erfolgt weiterhin nur bei bestätigtem Wechsel — der Zeitstempel ändert sich exakt dann, also entsteht kein zusätzlicher Traffic.

### 5. Anzeige

`Pane` (`stores/tabs.ts`) bekommt `activitySince: number | null`.

Der bestehende Status-Badge wird erweitert, kein neues Element:

| Zustand | Label |
|---|---|
| `done` | `fertig · 3 Std 20` |
| `waitingPermission` / `waitingAnswer` | `wartet auf dich · 12 Min` |
| `active` | `läuft · 4 Min` |
| `error` | `Fehler · 8 Min` |
| sonst | wie heute (kein Badge) |

Staffelung der Dauer, kompakt gehalten:

| Alter | Ausgabe |
|---|---|
| < 60 s | `gerade eben` |
| < 60 min | `12 Min` |
| < 24 h | `3 Std 20` |
| ab 24 h | `3 Tage` |

Die Dauer trägt durchgehend **kein** Präfix wie „vor" oder „seit". Der Zustand davor liefert die Zeitform bereits mit (`fertig · 3 Std 20` liest sich als „seit 3 Std 20 fertig"), und ein Präfix, das mal „vor" und mal „seit" heißen müsste, wäre je nach Zustand falsch. Ausnahme ist `gerade eben`, das für sich steht.

Der genaue Zeitpunkt steht im `title`-Tooltip des Badges (`Fertig seit 14:32 Uhr`).

**Ticker:** Ein einziger `setInterval` auf Modulebene speist einen Svelte-Store `now` (30-Sekunden-Takt); alle Labels leiten sich daraus ab. Nicht ein Intervall pro Pane — bei 31 Panes wären das 31 Timer und permanente Store-Updates. Der Ticker läuft nur, solange mindestens ein Abonnent existiert.

**Fehlt `activitySince`** (Wert `0`/`null`, etwa direkt nach dem Start), zeigt der Badge nur den Zustand ohne Dauer. Kein Platzhalter, kein `—`.

`CALM_DELAY_MS` und die zugehörige Debounce-Logik in `tabs.ts:328-351` entfallen; die Glättung liegt jetzt im Backend. Der erklärende Kommentar `tabs.ts:111-115` wird durch einen Verweis auf die Backend-Entprellung ersetzt.

### 6. Persistenz

`SavedPane` (`internal/config/session.go`) bekommt `ActivitySince int64` (`activity_since`, beide Tags). `paneToSaved` schreibt ihn, der Restore-Pfad gibt ihn zurück.

Damit der Wert nach dem Restore im Backend landet, nimmt `CreateSession` ihn als optionalen Parameter entgegen und initialisiert `activitySince` und `prevActivity` entsprechend vorbelegt. Ohne diesen Rückweg stünde der Wert zwar im Store, würde aber vom ersten bestätigten Wechsel überschrieben.

**`models.ts` muss von Hand nachgezogen werden** — `SavedPane` geht über eine Binding-Rückgabe. Feld deklarieren und im Konstruktor zuweisen (CLAUDE.md, wiederkehrender Fehler).

### 7. Nebenwirkungen umhängen

`processQueue`, `notifyOrchestratorDone` und `reportIssueProgress` (`app_scan.go:176-201`) hängen künftig am bestätigten Wechsel statt am rohen.

Das ist der Teil mit der größten Wirkung jenseits der Optik: `reportIssueProgress` hat keinerlei Dedup (`app_issue_progress.go:20-70`). Mit aktivem `auto_comment_on_done` bedeutet jeder Scheinabschluss heute einen GitHub-Kommentar, mit `auto_close_issue` einen erneuten Close-Versuch.

## Nebenläufigkeit

Lock-Reihenfolge `Screen.mu` → `Session.mu` → `App.mu` bleibt unberührt:

- Schritt 1 **entfernt** eine Zuweisung aus einem bestehenden `Session.mu`-Block. `Screen.Write` ist zu dem Zeitpunkt bereits abgeschlossen.
- Schritt 2 und 3 leben in `scanAllSessions`, das `App.mu` nur zum Kopieren der Session-Map hält (`app_scan.go:97-104`) und danach freigibt. Der Entprell-Zustand liegt unter `prevActivityMu`, ohne dass `Session.mu` oder `Screen.mu` gehalten werden.

Keine neue Verschachtelung, keine Prozess-I/O unter Lock.

**Aufräumen:** Beim Schließen einer Session müssen `pendingActivity`, `pendingSince` und `activitySince` mit `prevActivity` zusammen entfernt werden, sonst wachsen die Maps über die Laufzeit.

## Tests

**Go — Entprellung** (`internal/backend`)
- Ein einzelner abweichender Tick bestätigt nichts.
- Zwei aufeinanderfolgende übereinstimmende Ticks jenseits des Fensters bestätigen.
- Ein Rückfall auf den alten Zustand vor Ablauf des Fensters verwirft `pending`.
- Ein bestätigter Wechsel setzt `activitySince`; ein bestätigter *gleicher* Zustand lässt ihn unverändert.
- Ein echter Abschluss löst genau **einen** `reportIssueProgress`-Aufruf aus (Regression zu #188).
- Session-Schließen räumt alle vier Maps ab.

**Go — Lese-Pfad** (`internal/terminal`)
- Ein kosmetischer Chunk (Cursor hide/show) ändert `Activity` nicht mehr.
- Ein per `SetHookActivity` gesetzter Zustand überlebt nachfolgende PTY-Bytes.
- `LastOutputAt` und `Title` werden weiterhin aktualisiert.

**Go — Hook-Mapping**
- `Notification` ohne Fragezeichen ergibt nicht `Done`.

**Frontend** (`vitest`)
- Formatierung über alle vier Staffelstufen inklusive der Grenzen.
- Fehlender Zeitstempel → Badge ohne Dauer.
- Ein einzelner Ticker unabhängig von der Pane-Anzahl; er stoppt ohne Abonnenten.
- `paneToSaved` / Restore mit `activitySince`.

**Nur e2e mit echtem Claude CLI** (`needs-e2e-testing`)
- Die Anzeige bleibt über eine längere reale Sitzung ruhig — kein Zucken mehr.
- Die Dauer wächst monoton und springt nicht bei TUI-Repaints zurück.
- Ein Neustart erhält die Dauer.
- Reaktionszeit auf einen echten Zustandswechsel fühlt sich nicht träger an als vorher.

## Umsetzungsreihenfolge

1. Schreibseite bereinigen (Lese-Pfad, `Notification`-Mapping) — mit Tests
2. Entprellung samt Umzug der Nebenwirkungen und des zweiten Emit-Pfads — **danach ist #188 für sich erledigt und überprüfbar**
3. Zeitstempel und `ActivityInfo`-Feld
4. Anzeige im Badge, Ticker, Entfernen von `CALM_DELAY_MS`
5. Persistenz inklusive `models.ts` und Restore-Rückweg

Schritte 1–2 sind eigenständig wertvoll und sollten vor dem Rest verifiziert werden.

## Abhängigkeit

PR #187 verschiebt den betroffenen Lese-Pfad nach `noteOutput()` in `internal/terminal/session_spawn.go`. Diese Arbeit setzt darauf auf; andernfalls kollidieren die Änderungen an derselben Funktion.

## Bewusst nicht Teil dieser Arbeit

- **Keine Hysterese in `classifyScreenState` selbst.** Die Entprellung im Scan-Loop deckt das Symptom ab, ohne die Klassifikationslogik anzufassen. Erweist sich das als unzureichend, ist es eine eigene Änderung mit eigener Begründung.
- **Kein Dringlichkeits- oder Alarmkonzept.** Keine Einfärbung nach Alter, kein Sortieren, keine Benachrichtigung ab einer Schwelle. Die Dauer ist Orientierung, nicht Aufforderung.
- **Kein Countdown und kein Sekundentakt.** 30-Sekunden-Auflösung genügt für eine Angabe, die in Minuten und Stunden gelesen wird.
- **Kein Dedup in `reportIssueProgress`.** Nach dem Umzug auf den bestätigten Wechsel entfällt der Anlass. Eine zusätzliche Absicherung dort wäre eine eigene Entscheidung.
- **Keine Dauer für Shell-Panes.** Sie haben schon heute keinen Status-Badge; ein Sonderfall nur für die Zeit wäre inkonsistent.
