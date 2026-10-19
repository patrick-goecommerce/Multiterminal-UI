# Leitstand, Teil 2: Review-Briefing — Design

**Status:** Entwurf, von Patrick abschnittsweise freigegeben (2026-10-15 bis 2026-10-19), Spec-Review offen
**Worktree:** C:\Users\Patrick.Hennig\repos\Multiterminal-UI\.claude\worktrees\leitstand-review-briefing
**Baut auf:** `docs/superpowers/specs/2026-10-07-leitstand-aufmerksamkeit-design.md` (Teil 1, gemergt mit #214)
**Betrifft:** `cmd/mtui-hook`, neu `internal/hub/turn*.go`, `briefing*.go`, `ciwatch*.go`, `internal/hub/attention.go`,
Wire (`server_briefing.go`, `client_*.go`), `internal/backend` (Bindings, Events, Policy), `internal/config`,
`frontend/src/lib/attention.ts`, neu `frontend/src/lib/briefing.ts` und `BriefingCard.svelte`, `WaitingPanel.svelte`,
`SettingsDialog.svelte`, `models.ts`, `stores/config.ts`

## Wozu

Teil 1 sagt, **welches** Pane wartet. Bei rund zehn Projekten und vierzig Panes melden sich viele
Panes als „fertig“, und jedes davon heißt heute: hingehen, scrollen, lesen. Teil 2 gibt jedem
fertigen Eintrag ein Briefing, aus dem Patrick in fünf Sekunden entscheidet, **ob er hingehen muss**
(Triage), und das ihm sagt, **was der Agent getan hat**, ohne das Terminal zu lesen.

Was Patrick dafür festgelegt hat:

| Frage | Entscheidung |
|---|---|
| Wofür ist die Karte? | Triage „hingehen oder liegen lassen“, dazu „was hat er gemacht“ |
| Welche Warnzeichen? | CI rot / PR blockiert, großer Diff, Agent meldet ein Problem, ungesicherte Arbeit |
| Woher „was getan“? | Fakten plus Schlusswort: Auftrag, Git-Zahlen, Prüfbefehle mit Ergebnis, letzter Absatz. Kein LLM-Aufruf |
| CI wird später rot | Das Pane kommt als Fehler zurück (Rang 2, mit Toast), auch wenn es schon gesehen war |
| Diff-Zahlen | Beides: dieser Turn und der Branch gegen sein Ziel |
| Wo die Karte erscheint | Links neben dem Kasten beim Drüberfahren; Zeile 2 des Eintrags trägt schon das Urteil |
| Was „ungesichert“ ist | Nur Uncommittetes. Ungepushte Commits und „kein PR“ sind Info, weil Push bei Patrick erst nach Zustimmung kommt |
| Wo es läuft | Auf dem Host, wie die Aufmerksamkeit aus Teil 1 |

## Ziele und Erfolgskriterien

1. Für jeden fertigen Eintrag kann Patrick allein aus dem Kasten entscheiden, ob er hingeht oder ihn
   als gesehen abhakt. Zeile 2 reicht für das Urteil, die Karte für den Inhalt.
2. Kein Warnzeichen fehlt, das seine Entscheidung geändert hätte; ein ✓ steht nur, wo es belegt ist.
3. Ein später rot gewordenes CI oder ein blockierter PR holt das Pane zurück, auch bei geschlossenem
   Fenster im Daemon-Modus.
4. Kein zusätzlicher Modellaufruf, kein Schreibzugriff auf GitHub, kein Eingriff in Index oder
   Arbeitsbaum des Repos.

## Gemessen

Vor dieser Spec am echten Endpunkt gemessen, nicht angenommen.

**Claude Code 2.1.301, Bash-Hooks** (Probe mit eigener `--settings`-Datei, 2026-10-15):

- Exit-Code ≠ 0 kommt als `PostToolUseFailure`:
  `{"hook_event_name":"PostToolUseFailure","tool_name":"Bash","tool_input":{"command":"echo vorher; exit 3",…},"error":"Exit code 3\nvorher","is_interrupt":false,"prompt_id":…,"transcript_path":…,"duration_ms":…}`
- Erfolg kommt als `PostToolUse` mit `tool_response: {"stdout":"ok","stderr":"","interrupted":false,…}`,
  ohne Exit-Code.
- Beide tragen `prompt_id` (Kennung des Turns) und `transcript_path`. `mtui-hook` liest heute
  keines von beiden.
- Zweite Probe (2026-10-19): `UserPromptSubmit`, `PostToolUseFailure`, `PostToolUse` und `Stop`
  eines Turns tragen **dieselbe** `prompt_id`. `Stop` bringt außerdem `stop_hook_active`,
  `background_tasks` und `session_crons` mit.

**`gh pr list --head <branch> --state all --json …`** (gh, gegen #214, 2026-10-16): ein Aufruf
liefert `number, state, isDraft, baseRefName, headRefName, headRefOid, mergeable, mergeStateStatus,
reviewDecision, statusCheckRollup, url`. `statusCheckRollup` ist eine Liste von
`{"__typename":"CheckRun","name","status","conclusion","workflowName",…}`; für einen gemergten PR
stehen `mergeable` und `mergeStateStatus` auf `UNKNOWN`, `reviewDecision` ist leer, wenn kein
Review verlangt ist. Verbrauch zur Messung: 14 von 5000 GraphQL-Punkten pro Stunde.

## Teil A: Was der Host pro Turn sammelt

### A1 `mtui-hook`

Bei `PostToolUse` und `PostToolUseFailure` mit `tool_name == "Bash"` schreibt der Hook zusätzlich:

- `detail`: den Befehl (`tool_input.command`, eine Zeile, gekürzt wie in Teil 1),
- `prompt_id`, `is_interrupt`,
- `output_tail`: die letzten 300 Zeichen aus `stdout`+`stderr` bzw. aus `error`.

Bei `UserPromptSubmit` und `Stop` kommt `prompt_id` dazu. Die übrigen Ereignisse bleiben, wie sie sind. Es wird
kein neues Hook-Ereignis registriert; alle nötigen (`UserPromptSubmit`, `PostToolUse`,
`PostToolUseFailure`, `Stop`) stehen schon in `hookEvents`.

### A2 `turnBook`

Neu in `internal/hub`, neben dem `attentionBook`, eigener Mutex, je Session ein laufender Turn:

| Ereignis | Wirkung |
|---|---|
| `UserPromptSubmit` | Neuer Turn: Auftrag (≤ 200 Zeichen, `clipDetail`), `prompt_id`, Startzeit, Git-Schnappschuss (B1), leere Prüfliste. Ist der Turn ein Keep-alive-Turn (Markierung aus Teil 1), wird er als solcher geführt und baut kein Briefing. |
| Bash-Befehl fertig | Ist der Befehl eine Prüfung (A3), gilt für ihr Label das Ergebnis dieses Laufs; der letzte Lauf zählt. `is_interrupt` zählt nicht. Ereignisse mit fremder `prompt_id` oder nach `Stop` werden ignoriert (Hintergrund-Subagenten). |
| `Stop`, Exit, „hängt“ | Briefing bauen (Teil B, D), ablegen, `EventSessionBriefing` senden. Der Turn ist geschlossen. |
| Session endet | Turn und Briefing vergessen. |

### A3 Prüfbefehle

Ein Befehl wird in Segmente an `&&`, `;` und `||` zerlegt; vor jedem Segment fallen `cd …`,
Umgebungszuweisungen (`FOO=1`) und `timeout N` weg. Ein Segment ist eine Prüfung, wenn es mit einem
dieser Präfixe beginnt:

| Präfix | Label |
|---|---|
| `go test`, `go vet`, `go build` | wie Präfix |
| `npm test`, `npm run test`, `npx vitest`, `vitest`, `pnpm test`, `yarn test` | `test` |
| `npm run build`, `pnpm build`, `yarn build`, `wails build`, `wails3 build` | `build` |
| `npm run lint`, `npx eslint`, `eslint` | `lint` |
| `npx tsc`, `tsc` | `tsc` |
| `pytest`, `python -m pytest` | `pytest` |
| `cargo test`, `cargo build`, `cargo clippy` | wie Präfix |
| `dotnet build`, `dotnet test` | wie Präfix |

- Mehrere Prüfungen in einer Befehlszeile zählen als **eine** Prüfung mit verbundenem Label
  (`go vet + go test`), weil der Exit-Code nicht einem Segment zuzuordnen ist.
- **Pipe:** Steht hinter einer Prüfung ein `|`, gilt der Exit-Code der Pipe, nicht der Prüfung.
  Ergebnis dann „?“ (unsicher), es sei denn `output_tail` trägt ein eindeutiges Fehlerzeichen
  (`--- FAIL`, `FAIL\t`, `FAIL `, `error TS`, `npm ERR!`, `Tests … failed`, `error[E`): dann ✗.
  Ein „?“ wird nie zu ✓ und ist kein Warnzeichen.
- Alle anderen Befehle zählen nicht. Ein `grep` ohne Treffer endet auch mit Exit-Code 1.

## Teil B: Git-Signale

Gerechnet wird im letzten `cwd`, den die Hooks melden (nach `EnterWorktree` also im Worktree), über
`gitx.Cmd` (keine optionalen Locks, kein Credential-Prompt, kein Konsolenfenster). Ist der `cwd` in
keinem Repo, hat das Briefing keine Git-Zahlen.

### B1 Turn-Schnappschuss

Am Turn-Anfang und bei `Stop` wird der vollständige Arbeitsstand als Tree festgehalten:

1. `git rev-parse --git-path index` → Kopie in eine temporäre Datei,
2. `GIT_INDEX_FILE=<kopie> git add -A`, dann `git write-tree` → Tree-ID,
3. Kopie löschen.

Der echte Index und der Arbeitsbaum werden nicht berührt. Die Kopie bringt die Zeitstempel des echten
Index mit, Git liest deshalb nur geänderte Dateien neu ein. „Turn: 3 Dateien +40 −5“ ist
`git diff --numstat <tree_start> <tree_stop>`. Damit zählen auch Änderungen per `sed` oder Skript
und neue, nicht ignorierte Dateien.

Nebenwirkung: Inhalte geänderter Dateien landen als lose Objekte ohne Verweis im Repo; `git gc`
räumt sie nach der Standardfrist (14 Tage) weg.

### B2 Branch gegen sein Ziel

Ziel ist, in dieser Reihenfolge:

1. die Basis des PRs, sobald der Wächter (Teil C) einen kennt,
2. das Ziel, das das Fenster für MTUI-Worktrees kennt (`Pane.targetBranch`); neu über
   `SetSessionTarget(ref, branch)` an den Host gegeben, wann immer das Fenster es setzt,
3. der Standard-Branch des Remotes (`origin/HEAD`).

Verglichen wird mit `origin/<ziel>`, falls vorhanden, sonst mit `<ziel>`. Es wird nie gefetcht.

| Zahl | Herkunft |
|---|---|
| Branch: Dateien, +, − | `diff --numstat $(merge-base <ziel> HEAD) <tree_stop>`, also Commits und Uncommittetes |
| Commits vor dem Ziel | `rev-list --count <ziel>..HEAD` |
| Ungepusht | `rev-list --count @{upstream}..HEAD`; ohne Upstream „nie gepusht“ |
| Uncommittet | Einträge aus `status --porcelain` |

Steht HEAD auf dem Ziel selbst, gibt es keine Branch-Zahlen, die Karte sagt „auf alpha-main“. Bei
abgekoppeltem HEAD sagt sie „abgekoppelt“, und es gibt keine PR-Suche.

### B3 Warnzeichen aus Git

- `dirty`: Uncommittet > 0 bei `Stop`.
- `big_diff`: Branch über `big_diff_files` Dateien (Standard 15) oder über `big_diff_lines`
  geänderte Zeilen (+ und −, Standard 500). Bei den Zeilen zählen `package-lock.json`, `go.sum`,
  `pnpm-lock.yaml`, `yarn.lock`, `Cargo.lock` und `composer.lock` nicht mit. Ohne Branch-Zahlen
  (auf dem Ziel) misst die Schwelle den Turn.

### B4 Kosten

Alle Git-Aufrufe eines Briefings laufen außerhalb jedes Mutex in einer Goroutine mit 5 s
Gesamtlimit. Pro Session läuft höchstens ein Aufbau; ein neuerer `Stop` bricht den laufenden ab.
Insgesamt laufen höchstens vier Aufbauten gleichzeitig, der Rest wartet. Reißt das Limit, kommt das
Briefing ohne die fehlenden Zahlen und mit `note: "diff_slow"` („Diff zu langsam“); die übrigen
Signale gelten weiter. Der Schnappschuss am Turn-Anfang läuft genauso, mit eigenem Limit von 5 s;
fehlt er, fehlt nur der Turn-Diff.

## Teil C: CI- und PR-Wächter

`ciWatch` auf dem Host. Er **liest nur** und braucht kein Fenster.

### C1 Was beobachtet wird

Branches, die eine Session gerade hat (aus ihrem letzten Briefing) und deren Remote auf GitHub liegt
(`owner/repo` aus der Remote-URL). Mehrere Sessions auf demselben Branch teilen sich eine Abfrage.
Ohne `gh` oder ohne Login fehlt die PR-Zeile; das ist kein Fehler. Der Login wird einmal je
Hoststart geprüft (`gh auth status`) und das Ergebnis gemerkt; ein fehlgeschlagener Aufruf später
setzt ihn auf „unbekannt“ und prüft beim nächsten Briefing erneut.

Abfrage: `gh pr list --repo <owner/repo> --head <branch> --state all --limit 5 --json
number,state,isDraft,baseRefName,headRefOid,headRepositoryOwner,mergeable,reviewDecision,statusCheckRollup,url`.
Genommen wird der PR, dessen `headRepositoryOwner` dem Owner des Remotes entspricht, der offene vor
geschlossenen, sonst der jüngste. `gh` läuft mit `HideConsole`, `GH_PROMPT_DISABLED=1` und 15 s
Zeitlimit.

### C2 Wann

| Lage | Takt |
|---|---|
| Nach jedem Briefing und nach einem erkannten `git push`, `gh pr create`, `gh pr merge` | sofort |
| Checks laufen | jede Minute; nach 30 min alle 5 min; nach 3 h nicht mehr |
| PR offen, Checks fertig | alle 15 min (Konflikte, angeforderte Änderungen) |
| PR gemergt oder geschlossen, Session beendet, `ci_watch` aus | Schluss |
| Rate-Limit oder `gh`-Fehler | 15 min Pause, `note: "ci_paused"` |

### C3 Was abgeleitet wird

- Checks des PR-Heads: `none`, `pending` (ein `CheckRun` nicht `COMPLETED` oder ein `StatusContext`
  `PENDING`/`EXPECTED`), `red`, sonst `green`. Dazu Anzahl, Anzahl grün, Name des ersten roten.
- `ci_red`: ein `CheckRun` mit `conclusion` `FAILURE`, `TIMED_OUT`, `CANCELLED`, `ACTION_REQUIRED`
  oder `STARTUP_FAILURE`, oder ein `StatusContext` mit `state` `ERROR`/`FAILURE`.
- `pr_blocked`: `mergeable == "CONFLICTING"` oder `reviewDecision == "CHANGES_REQUESTED"`.
- Nur Info: Draft, „CI läuft“, „gemergt“, und „CI gilt für älteren Stand“, wenn der lokale HEAD
  nicht `headRefOid` ist.

### C4 Zurückholen

Auslöser ist der **Wechsel** nach `ci_red` oder `pr_blocked`, einmal je Head-Commit und Signal. Ein
gleichbleibend rotes CI holt nicht erneut zurück; ein neuer Push, der wieder rot wird, schon.

Dann bekommt jede Session auf dem Branch über `attentionBook.raise` einen Datensatz `State =
error`, `Source = "ci"`, `Seen = false`, `Since = jetzt`, mit Detail „CI rot: backend“,
„PR #12: Konflikte“ oder „PR #12: Änderungen angefordert“. Das ist Rang 2: Er steht vor allen
fertigen Einträgen und darf toasten (Regel aus Teil 1).

Ausnahme: Ist die Session gerade `active`, `waitingPermission` oder `waitingAnswer`, wird nur das
Briefing aktualisiert. Der Agent ist dann schon dran, oder das Pane steht ohnehin oben. Der nächste
bestätigte Zustand der Session ersetzt den zurückgeholten Datensatz wie jeden anderen.

## Teil D: Der Briefing-Datensatz

Eigener Datensatz neben `Attention`, nicht darin: Attention ist klein und geht bei jedem Wechsel
raus, das Briefing ist größer und entsteht einmal pro Turn. Alle Felder mit `json`- und `yaml`-Tags.

```go
type Briefing struct {
    PromptID string        // Turn, zu dem es gehört
    BuiltAt  time.Time
    Prompt   string        // Auftrag, ≤ 200 Zeichen
    Closing  string        // letzter Absatz der Agent-Antwort, ≤ 600 Zeichen
    Checks   []CheckResult // in der Reihenfolge des ersten Laufs
    Git      *GitFacts     // nil außerhalb eines Repos
    PR       *PRFacts      // nil: kein PR bekannt, kein gh, kein GitHub-Remote
    Warnings []string      // Codes, nach Vorrang sortiert
    Notes    []string      // "diff_slow", "ci_paused", "ci_stale"
}

type CheckResult struct {
    Label  string
    Result string // "ok", "fail", "unsure"
}

type GitFacts struct {
    Branch, Target                        string
    OnTarget, Detached, NoUpstream        bool
    TurnKnown                             bool
    TurnFiles, TurnAdded, TurnDeleted     int
    BranchFiles, BranchAdded, BranchDeleted int
    Commits, Unpushed, Uncommitted        int
}

type PRFacts struct {
    Number                      int
    URL, State, Base, HeadOID   string
    Draft, Conflicting, ChangesRequested bool
    Checks                      string // "none", "pending", "green", "red"
    ChecksTotal, ChecksPassed   int
    FailedCheck                 string
}
```

Warnzeichen und ihr Vorrang: `ci_red`, `pr_blocked`, `check_failed` (letzter Lauf einer Prüfung
`fail`), `ended_error` (Turn endet als Fehler: Exit ≠ 0, „hängt“), `dirty`, `big_diff`.

**Leitung:** `EventSessionBriefing` (Payload `{session, briefing}`) über denselben Weg wie
`EventSessionAttention`; `GET /v1/sessions/{id}/briefing` (404 ohne Briefing) und
`POST /v1/sessions/{id}/target`, beide in neuer Datei `server_briefing.go`. `Host` bekommt
`Briefing(id int) (Briefing, bool, error)` und `SetTarget(id int, branch string) error`; `Remote`
implementiert beide über die Leitung.

**Fenster:** Event `terminal:briefing` mit `hub.Ref`; Bindings `GetSessionBriefing(ref)` und
`SetSessionTarget(ref, branch)` von Hand in `App.js`/`App.d.ts` mit FNV-1a-ID. Das Briefing liegt
im Fenster in einem eigenen Store, nicht im `Pane`.

**Lebensdauer:** nur im Speicher des Hosts. Im Daemon-Modus überlebt es das Fenster, im eingebetteten
Modus nicht; ein wiederhergestelltes Pane hat erst nach seinem nächsten `Stop` wieder eins.

## Teil E: Oberfläche

### E1 Zeile 2 trägt das Urteil

Für Einträge im Zustand „fertig“ oder „Fehler“ mit Briefing ersetzt das Urteil die bisherige
Detailzeile:

| Lage | Zeile 2 |
|---|---|
| Warnzeichen | `⚠ vet ✗ · ungesichert · großer Diff`, höchstens drei, nach Vorrang |
| keine, CI läuft | `◷ CI läuft · „<Schlusswort>“` |
| keine | `✓ sauber · „<Schlusswort>“` |
| zurückgeholter Fehler (`Source = "ci"`) | sein Detail, z. B. `CI rot: backend` |
| kein Briefing | wie Teil 1: das Detail |

Kurzformen: `CI rot`, `PR blockiert`, `<label> ✗`, `Fehlerende`, `ungesichert`, `großer Diff`.

### E2 Sortierung

Innerhalb eines Ranges stehen Einträge mit Warnzeichen vor denen ohne, dann nach Alter wie bisher.
Der Rang kommt weiter nur vom Host; die Warnzeichen sind ein zweiter Sortierschlüssel. Das
Zusammenklappen ab drei fertigen Einträgen (Teil 1) klappt damit zuerst die sauberen weg.

### E3 Die Karte

- Erscheint, wenn die Maus 300 ms auf einem fertigen oder fehlerhaften Eintrag liegt oder der Eintrag
  den Tastaturfokus hat; verschwindet beim Verlassen. Für Fragen und Freigaben gibt es keine Karte.
- Steht links neben dem Kasten auf Höhe des Eintrags, an den Fensterrand geklemmt; in der
  Toolbar-Liste (Grid-Modus) links neben der Liste. Breite etwa 340 px.
- Inhalt, Zeilen ohne Daten fallen weg:
  1. Projekt · Pane · „gegen alpha-main“ (bzw. „auf alpha-main“, „abgekoppelt“)
  2. `Auftrag: „…“`
  3. `Turn:   3 Dateien +40 −5`
  4. `Branch: 7 Dateien +212 −48 · 3 Commits · 2 ungepusht` (bzw. „nie gepusht“)
  5. Prüfungen: `go test ✓   go vet ✗   test ?`
  6. PR: `PR #12 · CI 2/2 ✓ · Review offen`, `PR #12 · CI läuft 1/3`, `PR #12 gemergt`, `kein PR`,
     `CI-Abfrage pausiert`; „für älteren Stand“ als Zusatz
  7. Schlusswort, höchstens drei Zeilen
  8. Warnzeichen ausgeschrieben, Hinweise (`Diff zu langsam`)
- Die PR-Nummer öffnet den PR im Browser. Weitere Knöpfe gibt es nicht; Klick, Alt+Klick und
  Rechtsklick auf den Eintrag bleiben, wie Teil 1 sie festlegt.

### E4 Toasts

Zurückgeholte CI- und PR-Probleme toasten als Rang 2: „goki · hook-refactor: CI rot (backend)“.
Ein fertiges Pane mit Warnzeichen toastet nicht; Rang 3 bleibt still. Der Auftrag steht nie in einem
Toast.

## Teil F: Einstellungen

Neuer Config-Abschnitt, Felder mit `yaml`- und `json`-Tags, gespiegelt in `models.ts` und
`stores/config.ts` (`go test ./internal/tsmodels/`):

```yaml
briefing:
  big_diff_files: 15
  big_diff_lines: 500
  ci_watch: true      # *bool, nil = an
```

Der Host bekommt die Werte über eine Policy-Funktion wie die Keep-alive-Politik aus Teil 1
(`keepAliveFn`); eine Änderung gilt ab dem nächsten Briefing bzw. dem nächsten Takt des Wächters.
SettingsDialog: Abschnitt „Leitstand“ mit den drei Feldern; Initialisierung über eine Funktion,
nie Zuweisungen im `$:`-Block.

## Fehlerfälle und Grenzen

- **Älterer Daemon** ohne Briefing: `GET …/briefing` → 404, das Event kommt nie. Keine Karte, Zeile 2
  wie Teil 1. `SetSessionTarget` → 404 wird geloggt und ignoriert.
- **Keine Hooks** (Shell-Panes, Codex): kein Turn, kein Briefing, keine Karte.
- **Keep-alive-Turn:** baut kein Briefing und lässt das letzte stehen.
- **Hintergrund-Subagent nach `Stop`:** ändert den geschlossenen Turn nicht.
- **Fork-PRs, gleichnamige Branches:** nur ein PR, dessen Head-Repo dem Owner des Remotes gehört.
- **Nicht GitHub** oder kein Remote: keine PR-Zeile.
- **Riesiges Repo:** Zeitlimit, `diff_slow`, siehe B4.
- **Prüfung mit Pipe:** „?“, siehe A3. Ein ✓ steht nur, wo der Exit-Code der Prüfung selbst ihn
  belegt.
- **Datenschutz:** Der Auftrag (≤ 200 Zeichen) steht nur auf der Karte im Fenster, nie im Toast.
- **Projektregeln:** `server.go` wächst nicht (neue Routen in `server_briefing.go`); jede neue
  Produktionsdatei ≤ 300 Zeilen; `internal/backend` importiert weiterhin nicht `internal/terminal`.

## Nicht im Umfang

- Mergen, Freigeben, Review anstoßen aus der Karte (Teilprojekt 3)
- MCP-Zugriff auf das Briefing (Teilprojekt 4, wenn der Chef-Agent es braucht)
- Karte im Pane-Titel, Warnzeichen in `mt ls`
- KI-Kurzfassung, Lesen des Transkripts (`transcript_path` wird weitergegeben, aber nicht gelesen)
- Automatisches Fetchen

## Tests

**Go**
- Prüfbefehl-Erkennung als Tabelle: `cd x && go test ./...`, `FOO=1 npm run build`,
  `go vet ./... && go test ./...` (verbundenes Label), `go test ./... | tail -20` mit und ohne
  `--- FAIL` im Tail, `grep` zählt nicht, `is_interrupt` zählt nicht.
- `turnBook`: erst rot, dann grün → ✓; Keep-alive-Turn ohne Briefing, letztes bleibt; Ereignis nach
  `Stop` und mit fremder `prompt_id` ignoriert; Session-Ende vergisst.
- Git in einem Test-Repo (`t.TempDir()`, echtes `git`): Turn-Diff zählt eine Änderung ohne
  Edit-Werkzeug und eine neue Datei; **der echte Index ist danach Byte für Byte gleich**; Branch-Diff
  enthält Uncommittetes; Lockfile-Zeilen zählen nicht; beide Schwellen; ohne Upstream „nie gepusht“;
  auf dem Ziel keine Branch-Zahlen; ohne Repo leer; Zeitlimit → `diff_slow`.
- Zielwahl in der Reihenfolge PR-Basis, Fensterziel, `origin/HEAD`.
- `ciWatch` mit austauschbarem `gh`-Runner. Fixture ist **die gemessene Antwort von #214**; die
  übrigen Fälle entstehen daraus durch Ändern einzelner Felder (`conclusion`, `status`,
  `mergeable`, `reviewDecision`, `state`). Fälle: laufend → rot holt einmal zurück; rot bleibt rot
  ohne zweites Zurückholen; neuer Head wieder rot → erneut; Session `active` → nur Briefing;
  gemergt beendet; Rate-Limit pausiert; fremder `headRepositoryOwner` wird übergangen; Takt nach
  30 min und 3 h.
- `attentionBook.raise`: Rang 2, ungesehen, Quelle `ci`; nicht über `active`/wartend.
- Leitung: Briefing über `Remote` hin und zurück, `SetTarget`, 404-Fälle.
- `mtui-hook`: Detail, `prompt_id`, `is_interrupt`, `output_tail` an der gemessenen Payload-Form.
- `go test ./internal/tsmodels/` (Struct, Config-Felder, Binding-IDs).

**Vitest**
- Zeile 2: Vorrang, höchstens drei, „sauber“, „CI läuft“, zurückgeholter Fehler, kein Briefing.
- Sortierung innerhalb eines Ranges.
- Position der Karte am Fensterrand.
- PR-Zeile in allen Varianten; Karte ohne Git, ohne PR, mit `diff_slow`.

## Abnahme

E2E mit echtem Claude Code, echtem Repo, echtem Fenster (`needs-e2e-testing`), sinnvoll zusammen
mit der offenen Abnahme von Teil 1:

1. Ein Agent ändert zwei Dateien, `go vet` scheitert: Karte zeigt „Turn: 2 Dateien“, `go vet ✗`;
   Zeile 2 trägt ⚠, der Eintrag steht vor den sauberen.
2. Er repariert und committet: `✓ sauber`.
3. Er hört mit Uncommittetem auf: `⚠ ungesichert`.
4. Branch über 15 Dateien: `⚠ großer Diff`; eine Änderung nur an `package-lock.json` nicht.
5. `go test ./... | tail` mit rotem Test: `test ✗` bzw. `go test ✗`, nie ✓.
6. Push und PR in einem Test-Repo mit absichtlich roter CI; Eintrag als gesehen markieren und
   warten: Er kommt mit Toast „CI rot“ zurück.
7. PR-Konflikt: „PR blockiert“.
8. Daemon-Modus: Fenster zu, CI wird rot, Fenster auf: Eintrag ist da.
9. `gh auth logout`: keine PR-Zeile, keine Fehlermeldung.
10. Keep-alive-Turn: die Karte bleibt unverändert.

## Umsetzung in Stufen

Jede Stufe ist für sich nutzbar und endet grün:

- **a** Turn-Fakten (A), Git-Signale (B), Datensatz und Leitung (D), Zeile 2 und Karte (E1–E3)
- **b** CI- und PR-Wächter (C), Zurückholen, Toasts (E4)
- **c** Einstellungen (F)

## Offene Punkte

1. Die Fehlerzeichen für Prüfungen mit Pipe (A3) sind eine Startliste; sie wird an echten Ausgaben
   von `go test`, `vitest`, `tsc` und `pytest` geprüft, bevor sie in die Tests kommt.
