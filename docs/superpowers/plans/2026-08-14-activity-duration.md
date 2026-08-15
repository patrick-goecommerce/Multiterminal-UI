# Zustandsdauer pro Pane — Implementierungsplan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Der Status-Badge jedes Panes zeigt, wie lange der aktuelle Zustand schon andauert (`fertig · 3 Std 20`) — auf Basis einer Zustandsermittlung, die nicht mehr flackert.

**Architecture:** Ein Zustandswechsel wird an genau einer Stelle festgestellt: in der Entprell-Auswertung des Scan-Loops. Der Lese-Pfad hört auf, `Activity` bedingungslos zu setzen; ein abweichender Rohzustand muss über ein Zeitfenster stabil sein, bevor er als Wechsel gilt. Anzeige, Dauer, Queue-Vorschub und Issue-Meldung leiten sich alle von diesem einen bestätigten Wechsel ab.

**Tech Stack:** Go 1.21+ (Backend), TypeScript/Svelte 4 (Frontend), Wails v3 alpha, vitest, `go test`.

**Spec:** `docs/superpowers/specs/2026-08-14-activity-duration-design.md`
**Issues:** #188 (Flackern), #189 (Dauer-Anzeige)
**Worktree:** C:\Users\Patrick.Hennig\repos\Multiterminal-UI\.claude\worktrees\activity-duration

## Global Constraints

- **Max 300 Zeilen pro Go-Datei.** `app_scan.go` liegt bereits bei ~215 Zeilen; neue Backend-Logik kommt deshalb in `app_scan_debounce.go`.
- **UI-Text ist Deutsch**, Code, Kommentare und Commit-Messages sind Englisch.
- **`frontend/wailsjs/go/models.ts` wird von Hand gepflegt.** Wails v3 regeneriert es nicht. Jedes neue Feld an einem Struct, das über eine **Binding-Rückgabe** ans Frontend geht, braucht dort Deklaration *und* Zuweisung im Konstruktor. Events (`terminal:activity`) sind davon **nicht** betroffen — sie gehen als JSON.
- **Lock-Reihenfolge `Screen.mu` → `Session.mu` → `App.mu`**, nie verschachtelt, nie rückwärts. Keine Prozess-I/O unter Lock.
- **Entprell-Fenster:** `1200 * time.Millisecond`, als benannte Konstante.
- **Ticker-Auflösung Frontend:** 30 Sekunden.
- Commit-Messages enden mit `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.
- **Nicht pushen, keinen PR anlegen, nichts mergen** ohne ausdrückliche Zustimmung des Nutzers.

## Abweichung von der Spec (bewusst)

Die Spec beschreibt in Abschnitt 7 ein „Umhängen" der Nebenwirkungen auf den bestätigten Wechsel. Beim Lesen des Codes zeigt sich: `processQueue`, `notifyOrchestratorDone`, `notifyFinishOnActivity` und `onActivityChangeForIssue` hängen bereits alle an derselben lokalen Variable `activityChanged` (`app_scan.go:184-209`). Es genügt, deren **Berechnung** zu ändern; ein Umhängen entfällt. Task 3 setzt genau dort an. Die Spec wird in Task 3 entsprechend korrigiert, damit Code und Spec nicht auseinanderlaufen.

Eine Einschränkung dazu, die beim Umsetzen sichtbar wurde: `processQueue` und `onActivityChangeForIssue` hängen zwar im Scan-Loop an `activityChanged`, wurden aber **zusätzlich** direkt aus dem Hook-Callback aufgerufen. Der Hook-Pfad muss diese Aufrufe verlieren, sonst läuft jeder hook-getriebene Abschluss beide Sätze im Abstand von rund zwei Sekunden. Siehe die Notiz am Ende dieses Dokuments.

Zweite bewusste Abweichung, Abschnitt 6: Der Restore-Rückweg ist eine eigene Binding `SeedActivitySince` statt eines zusätzlichen `CreateSession`-Parameters. Begründung und die daraus folgende Zustands-Bindung des Seeds stehen in Task 6, Step 8; die Spec ist entsprechend korrigiert.

## File Structure

**Neu:**
- `internal/backend/app_scan_debounce.go` — Entprell-Zustand, `confirmActivity`, `activitySinceFor`, Aufräumen. Einzige Stelle, die `activitySince` schreibt.
- `internal/backend/app_scan_debounce_test.go` — Tests dazu.
- `frontend/src/lib/duration.ts` — Formatierung der Dauer, reine Funktion ohne Svelte-Abhängigkeit.
- `frontend/src/lib/duration.test.ts`
- `frontend/src/stores/clock.ts` — ein einzelner Ticker-Store, den alle Badges teilen.
- `frontend/src/stores/clock.test.ts`

**Geändert:**
- `internal/terminal/session_spawn.go` — `s.Activity = ActivityActive` aus `noteOutput` entfernen.
- `internal/terminal/session_suspend_test.go` bzw. neue Testdatei — Nachweis, dass ein kosmetischer Chunk den Zustand nicht mehr ändert.
- `internal/backend/app_hooks.go` — `hookEventToActivity` wird zweiwertig; Aufrufer respektiert „keine Aussage".
- `internal/backend/app_hooks_test.go` — Tabelle an die neue Signatur anpassen.
- `internal/backend/app_scan.go` — `activityChanged` kommt aus `confirmActivity`; `ActivityInfo` bekommt `ActivitySince`; `cleanupActivityCache` räumt die neuen Maps mit ab.
- `internal/config/session.go` — `SavedPane.ActivitySince`.
- `frontend/wailsjs/go/models.ts` — `activity_since` an `SavedPane`.
- `frontend/src/stores/tabs.ts` — `Pane.activitySince`, `updateActivity` nimmt den Zeitstempel, `CALM_DELAY_MS` und die Debounce-Logik entfallen.
- `frontend/src/components/PaneTitlebar.svelte` — Badge zeigt die Dauer.
- `frontend/src/lib/session.ts` — `activitySince` speichern und wiederherstellen.
- `frontend/src/App.svelte` — Event-Handler reicht das neue Feld durch.

---

### Task 1: Lese-Pfad schreibt `Activity` nicht mehr

Der Kern von #188. `noteOutput` setzt heute bei **jedem** PTY-Byte `ActivityActive` — auch bei einem Cursor-Hide/Show ohne sichtbare Wirkung, und auch dann, wenn ein Hook den Zustand autoritativ gesetzt hat.

**Files:**
- Modify: `internal/terminal/session_spawn.go` (Funktion `noteOutput`)
- Test: `internal/terminal/session_activity_write_test.go` (neu)

**Interfaces:**
- Consumes: nichts aus vorherigen Tasks.
- Produces: `noteOutput(gen int) bool` behält Signatur und Verhalten für `Title`, `LastOutputAt` und die Suspend-Abort-Markierung; es schreibt `Session.Activity` nicht mehr.

- [ ] **Step 1: Write the failing test**

Neue Datei `internal/terminal/session_activity_write_test.go`:

```go
package terminal

import "testing"

// A cosmetic PTY chunk must not change the activity state. The read path used
// to set ActivityActive on every byte, which made a repaint of Claude's idle
// TUI look like fresh work (issue #188).
func TestNoteOutputDoesNotTouchActivity(t *testing.T) {
	s := NewSession(1, 24, 80)
	s.Activity = ActivityDone

	if ok := s.noteOutput(s.sus.gen); !ok {
		t.Fatal("noteOutput reported a stale generation for a fresh session")
	}

	if got := s.GetActivity(); got != ActivityDone {
		t.Errorf("Activity = %v after a cosmetic chunk, want %v (unchanged)", got, ActivityDone)
	}
}

// A hook-set state is authoritative and must survive later PTY bytes.
func TestNoteOutputKeepsHookState(t *testing.T) {
	s := NewSession(2, 24, 80)
	s.SetHookActivity(ActivityDone)

	s.noteOutput(s.sus.gen)

	if got := s.GetActivity(); got != ActivityDone {
		t.Errorf("hook state was overwritten: Activity = %v, want %v", got, ActivityDone)
	}
	if !s.HasHookData() {
		t.Error("HasHookData() = false, want true")
	}
}

// The bookkeeping noteOutput is actually responsible for must keep working.
func TestNoteOutputStillRecordsOutput(t *testing.T) {
	s := NewSession(3, 24, 80)
	s.Screen.Write([]byte("\x1b]0;my-title\x07"))

	s.noteOutput(s.sus.gen)

	if s.GetLastOutputAt().IsZero() {
		t.Error("LastOutputAt was not updated")
	}
	if s.Name() != "my-title" {
		t.Errorf("Title = %q, want %q", s.Name(), "my-title")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/terminal/ -run 'TestNoteOutput' -v`
Expected: `TestNoteOutputDoesNotTouchActivity` und `TestNoteOutputKeepsHookState` FAIL mit `Activity = 1 ... want 2`. `TestNoteOutputStillRecordsOutput` besteht bereits.

- [ ] **Step 3: Remove the unconditional write**

In `internal/terminal/session_spawn.go`, Funktion `noteOutput`, diese Zeile ersatzlos streichen:

```go
	s.Activity = ActivityActive
```

Der Rest der Funktion bleibt unverändert. Ergänze über der Funktion einen Hinweis, warum dort nichts mehr geschrieben wird:

```go
// noteOutput records a fresh PTY chunk under s.mu. It reports false when the
// chunk belongs to a stale generation, i.e. the caller must stop.
//
// It deliberately does NOT touch s.Activity. Doing so on every byte made a
// cosmetic repaint of Claude's idle TUI — even a bare cursor hide/show —
// indistinguishable from real work, and it silently overwrote authoritative
// hook state (issue #188). Activity is decided in one place: DetectActivity /
// the hook path, evaluated by the scan loop.
//
// This is also the abort half of the two-phase suspend commit: a chunk arriving
// after a suspend was armed marks it aborted in the very same critical section
// that TrySuspend used, so the kill goroutine cannot miss it.
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/terminal/ -run 'TestNoteOutput' -v`
Expected: alle drei PASS.

- [ ] **Step 5: Run the whole package to catch regressions**

Run: `go test ./internal/terminal/ -count=1`
Expected: `ok`.

Falls ein Suspend-Test fehlschlägt: `TrySuspend` verlangt `ActivityDone`, und Tests, die vorher über einen Output-Chunk implizit auf `ActivityActive` kamen, müssen den Zustand jetzt explizit setzen. Das ist eine Testanpassung, keine Regression — aber prüfe jeden Fall einzeln, statt pauschal anzupassen.

- [ ] **Step 6: Commit**

```bash
git add internal/terminal/session_spawn.go internal/terminal/session_activity_write_test.go
git commit -m "fix(terminal): stop the read path from setting Activity on every byte

A cosmetic repaint — even a bare cursor hide/show — set ActivityActive, so an
idle Claude TUI looked like fresh work. It also overwrote hook-set state, which
is supposed to be authoritative, because the hook guard only sits on the read
side and this path never asked.

DetectActivity produces the same Active transition while output is fresh, so
nothing is lost except the bypass.

Refs #188

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `hookEventToActivity` darf „keine Aussage" zurückgeben

Ursache 3 aus #188: `Notification` ohne Fragezeichen mappt auf `Done`. Claudes Texte wie „Claude needs your permission to use Bash" enthalten keins — das reißt eine laufende Session auf „fertig". Der `default`-Zweig hat dasselbe Problem: Ein unbekanntes Event als `Idle` zu werten ist eine Behauptung ohne Grundlage.

Die Funktion kann das heute nicht ausdrücken, weil jeder `ActivityState` eine Aussage ist.

**Files:**
- Modify: `internal/backend/app_hooks.go:32-53` (Funktion) und `:238-243` (Aufrufer)
- Test: `internal/backend/app_hooks_test.go:45-62`

**Interfaces:**
- Consumes: nichts aus Task 1.
- Produces: `hookEventToActivity(event, message string) (terminal.ActivityState, bool)` — `false` heißt: dieses Event trägt keine Zustandsinformation, der Aufrufer lässt den Zustand unangetastet.

- [ ] **Step 1: Write the failing test**

Ersetze die bestehende Tabelle in `internal/backend/app_hooks_test.go` (die Funktion `TestHookEventToActivity`) durch:

```go
func TestHookEventToActivity(t *testing.T) {
	tests := []struct {
		event   string
		message string
		want    terminal.ActivityState
		wantOK  bool
	}{
		{"PreToolUse", "", terminal.ActivityActive, true},
		{"PostToolUse", "", terminal.ActivityActive, true},
		{"UserPromptSubmit", "", terminal.ActivityActive, true},
		{"PostToolUseFailure", "", terminal.ActivityError, true},
		{"PermissionRequest", "", terminal.ActivityWaitingPermission, true},
		{"Stop", "", terminal.ActivityDone, true},
		{"Notification", "Weiter so?", terminal.ActivityWaitingAnswer, true},
		// A notification without a question mark says nothing about whether the
		// turn ended. Claude's own wording ("Claude needs your permission to use
		// Bash") has no "?", and mapping it to Done tore a running session to
		// "finished" (issue #188).
		{"Notification", "Claude needs your permission to use Bash", terminal.ActivityIdle, false},
		// An unknown event is not evidence of idleness either.
		{"unknown", "", terminal.ActivityIdle, false},
	}
	for _, tt := range tests {
		got, ok := hookEventToActivity(tt.event, tt.message)
		if ok != tt.wantOK {
			t.Errorf("hookEventToActivity(%q, %q) ok = %v, want %v", tt.event, tt.message, ok, tt.wantOK)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("hookEventToActivity(%q, %q) = %d, want %d", tt.event, tt.message, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/backend/ -run TestHookEventToActivity -v`
Expected: Compile-Fehler `assignment mismatch: 2 variables but hookEventToActivity returns 1 value`.

- [ ] **Step 3: Make the function two-valued**

In `internal/backend/app_hooks.go` die Funktion ersetzen:

```go
// hookEventToActivity maps a Claude Code event name to an ActivityState.
//
// The bool reports whether the event carries any state information at all.
// Not every event does: a Notification without a question mark says nothing
// about whether the turn ended, and an unknown event says nothing at all.
// Returning a state anyway meant inventing one, which tore running sessions to
// "done" and idle ones to "idle" (issue #188). Callers must leave the current
// state untouched when this is false.
func hookEventToActivity(event, message string) (terminal.ActivityState, bool) {
	switch event {
	case "PreToolUse", "PostToolUse", "UserPromptSubmit":
		return terminal.ActivityActive, true
	case "PostToolUseFailure":
		return terminal.ActivityError, true
	case "PermissionRequest":
		return terminal.ActivityWaitingPermission, true
	case "Notification":
		if strings.Contains(message, "?") {
			return terminal.ActivityWaitingAnswer, true
		}
		return terminal.ActivityIdle, false
	case "Stop":
		return terminal.ActivityDone, true
	default:
		return terminal.ActivityIdle, false
	}
}
```

- [ ] **Step 4: Update the call site**

In `internal/backend/app_hooks.go`, im Block ab Zeile ~238:

```go
	newState, ok := hookEventToActivity(ev.Event, ev.Message)
	if !ok {
		// The event carries no state claim — leave the session as it is.
		return
	}
	sess.SetHookActivity(newState)

	if hm.onActivity != nil {
		hm.onActivity(ev.MtID, activityString(newState), "")
	}
```

Prüfe, ob nach diesem Block noch Code in der Funktion folgt. Falls ja, ersetze das frühe `return` durch eine Verzweigung, die nur `SetHookActivity` und den `onActivity`-Aufruf überspringt — der restliche Code muss weiterlaufen.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/backend/ -run 'TestHook' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/backend/app_hooks.go internal/backend/app_hooks_test.go
git commit -m "fix(hooks): let a hook event report that it makes no state claim

A Notification without a question mark was mapped to Done, so Claude's own
\"needs your permission\" wording tore a running session to \"finished\" without
a single PTY byte. The default branch had the same flaw, calling every unknown
event idle.

hookEventToActivity now returns (state, ok); on false the caller leaves the
session untouched instead of inventing a state.

Refs #188

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Entprellung im Scan-Loop — damit ist #188 erledigt

Nach Task 1 flackert der Rohzustand seltener, aber Ursache 2 bleibt: `classifyScreenState` ist ein Snapshot ohne Hysterese und kippt von `Done` nach `Idle`, wenn ein Tick zwischen `ESC[2K` und das Neuzeichnen der Eingabebox fällt.

Diese Task führt die Bestätigung ein. Weil `processQueue`, `notifyOrchestratorDone`, `notifyFinishOnActivity` und `onActivityChangeForIssue` bereits alle an `activityChanged` hängen, ziehen sie automatisch mit.

**Files:**
- Create: `internal/backend/app_scan_debounce.go`
- Create: `internal/backend/app_scan_debounce_test.go`
- Modify: `internal/backend/app_scan.go:158-169` (Vergleichsblock) und `:85-93` (`cleanupActivityCache`)
- Modify: `docs/superpowers/specs/2026-08-14-activity-duration-design.md` (Abschnitt 7)

**Interfaces:**
- Consumes: nichts aus Task 1/2.
- Produces:
  - `confirmActivity(id int, raw string, now time.Time) bool` — Caller muss `prevActivityMu` halten. Liefert `true` genau dann, wenn `raw` als neuer Zustand bestätigt wurde.
  - `activitySinceFor(id int) time.Time` — nimmt `prevActivityMu` selbst; Nullwert bedeutet unbekannt.
  - `setActivitySinceFor(id int, t time.Time, state string)` — für den Restore-Rückweg in Task 6; nimmt `prevActivityMu` selbst. Der Zustand gehört dazu, siehe Task 6 Step 8.
  - `cleanupActivityDebounce(id int)` — Caller muss `prevActivityMu` halten.
  - `resetActivityDebounceForTest()` — Testhilfe.

- [ ] **Step 1: Write the failing test**

Neue Datei `internal/backend/app_scan_debounce_test.go`:

```go
package backend

import (
	"testing"
	"time"
)

// A single differing tick is not a state change: Claude's TUI can blank the
// prompt line mid-repaint, which classifies as "idle" for one tick (issue #188).
func TestConfirmActivityIgnoresSingleTick(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(1000, 0)
	prevActivity[1] = "done"

	if confirmActivity(1, "idle", base) {
		t.Fatal("a first differing tick must not confirm")
	}
	if confirmActivity(1, "done", base.Add(600*time.Millisecond)) {
		t.Fatal("falling back to the previous state must not confirm")
	}
	if got := prevActivity[1]; got != "done" {
		t.Errorf("prevActivity = %q, want %q", got, "done")
	}
}

// A state that holds past the window is a real change.
func TestConfirmActivityAcceptsStableState(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(2000, 0)
	prevActivity[1] = "active"

	if confirmActivity(1, "done", base) {
		t.Fatal("the first observation must only arm, not confirm")
	}
	if confirmActivity(1, "done", base.Add(debounceWindow-time.Millisecond)) {
		t.Fatal("confirmed one millisecond early")
	}
	if !confirmActivity(1, "done", base.Add(debounceWindow)) {
		t.Fatal("a state stable for the full window must confirm")
	}
	if got := prevActivity[1]; got != "done" {
		t.Errorf("prevActivity = %q, want %q", got, "done")
	}
}

// The timestamp marks when the state actually began — the first observation —
// not when it happened to be confirmed one window later.
func TestConfirmActivityStampsFirstObservation(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	base := time.Unix(3000, 0)
	prevActivity[1] = "active"

	confirmActivity(1, "done", base)
	confirmActivity(1, "done", base.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceFor(1); !got.Equal(base) {
		t.Errorf("activitySince = %v, want %v (first observation)", got, base)
	}
}

// Re-confirming the same state must not restart the clock, otherwise the
// duration would reset on every tick.
func TestConfirmActivityKeepsTimestampOnSameState(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	base := time.Unix(4000, 0)
	prevActivity[1] = "active"
	confirmActivity(1, "done", base)
	confirmActivity(1, "done", base.Add(debounceWindow))

	if confirmActivity(1, "done", base.Add(time.Hour)) {
		t.Fatal("an unchanged state must not report a change")
	}
	prevActivityMu.Unlock()

	if got := activitySinceFor(1); !got.Equal(base) {
		t.Errorf("activitySince = %v, want %v (unchanged)", got, base)
	}
}

// Two panes must not share debounce state.
func TestConfirmActivityIsPerSession(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(5000, 0)
	prevActivity[1] = "active"
	prevActivity[2] = "active"

	confirmActivity(1, "done", base)
	if confirmActivity(2, "done", base.Add(debounceWindow)) {
		t.Fatal("session 2 confirmed using session 1's pending timer")
	}
}

// Closing a pane must not leak map entries.
func TestCleanupActivityDebounce(t *testing.T) {
	resetActivityDebounceForTest()
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	base := time.Unix(6000, 0)
	prevActivity[7] = "active"
	confirmActivity(7, "done", base)
	confirmActivity(7, "done", base.Add(debounceWindow))

	cleanupActivityDebounce(7)

	if _, ok := pendingActivity[7]; ok {
		t.Error("pendingActivity still holds the session")
	}
	if _, ok := pendingSince[7]; ok {
		t.Error("pendingSince still holds the session")
	}
	if _, ok := activitySince[7]; ok {
		t.Error("activitySince still holds the session")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/backend/ -run 'TestConfirmActivity|TestCleanupActivityDebounce' -v`
Expected: Compile-Fehler `undefined: confirmActivity`.

- [ ] **Step 3: Write the debounce**

Neue Datei `internal/backend/app_scan_debounce.go`:

```go
package backend

import "time"

// debounceWindow is how long a differing raw activity state must hold before it
// counts as a real change.
//
// The scan tick is 500-750 ms depending on session count (see scanInterval), so
// a confirmation costs two to three matching observations and up to ~1.75 s of
// latency. That buys immunity against two independent flicker sources: Claude's
// TUI repainting while idle, and classifyScreenState being a snapshot without
// hysteresis — a tick landing between ESC[2K and the redraw of the input box
// classifies the very same screen as "idle" (issue #188).
//
// The frontend's own 900 ms smoothing is removed in exchange, so this is not a
// net slowdown.
const debounceWindow = 1200 * time.Millisecond

// Debounce bookkeeping, guarded by prevActivityMu together with prevActivity.
var (
	// pendingActivity is the candidate state observed but not yet confirmed.
	pendingActivity = make(map[int]string)
	// pendingSince is when that candidate was first observed.
	pendingSince = make(map[int]time.Time)
	// activitySince is when the currently confirmed state began. This is the
	// single place it is written; the read path and SetHookActivity must never
	// touch it, or the flicker this package just fixed returns in the duration.
	activitySince = make(map[int]time.Time)
)

// confirmActivity applies the debounce for one session and reports whether raw
// became the confirmed state. The caller must hold prevActivityMu.
//
// On confirmation, activitySince is stamped with the *first* observation, not
// with now: the state began when it was first seen, not when it survived the
// window. Otherwise every duration would be short by up to one window.
func confirmActivity(id int, raw string, now time.Time) bool {
	if prevActivity[id] == raw {
		// Back to (or still on) the confirmed state — drop any candidate.
		delete(pendingActivity, id)
		delete(pendingSince, id)
		return false
	}
	if pending, ok := pendingActivity[id]; !ok || pending != raw {
		// A new candidate: start its clock.
		pendingActivity[id] = raw
		pendingSince[id] = now
		return false
	}
	if now.Sub(pendingSince[id]) < debounceWindow {
		return false
	}
	prevActivity[id] = raw
	activitySince[id] = pendingSince[id]
	delete(pendingActivity, id)
	delete(pendingSince, id)
	return true
}

// activitySinceFor returns when the confirmed state of a session began. The
// zero time means unknown — the pane has not had a confirmed change yet.
func activitySinceFor(id int) time.Time {
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()
	return activitySince[id]
}

// setActivitySinceFor seeds the timestamp of a restored pane together with the
// state it belongs to (see Task 6, Step 8), so a duration
// survives an MTUI restart instead of starting over.
func setActivitySinceFor(id int, t time.Time, state string) {
	if t.IsZero() {
		return
	}
	prevActivityMu.Lock()
	activitySince[id] = t
	prevActivityMu.Unlock()
}

// cleanupActivityDebounce drops a closed session's debounce state. The caller
// must hold prevActivityMu.
func cleanupActivityDebounce(id int) {
	delete(pendingActivity, id)
	delete(pendingSince, id)
	delete(activitySince, id)
}

// resetActivityDebounceForTest clears all debounce state between tests.
func resetActivityDebounceForTest() {
	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()
	pendingActivity = make(map[int]string)
	pendingSince = make(map[int]time.Time)
	activitySince = make(map[int]time.Time)
	prevActivity = make(map[int]string)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/backend/ -run 'TestConfirmActivity|TestCleanupActivityDebounce' -v`
Expected: alle PASS.

- [ ] **Step 5: Wire it into the scan loop**

In `internal/backend/app_scan.go` den Vergleichsblock (ab „Only emit when state, cost, or title actually changed") ersetzen:

```go
		// Only emit when state, cost, or title actually changed. The activity
		// half runs through confirmActivity, so a one-tick flicker never
		// reaches the UI — nor the queue, orchestrator and issue reporting
		// below, which all key off activityChanged.
		now := time.Now()
		prevActivityMu.Lock()
		activityChanged := confirmActivity(id, actStr, now)
		costChanged := prevCost[id] != costStr
		titleChanged := prevTitle[id] != title
		changed := activityChanged || costChanged || titleChanged
		if costChanged {
			prevCost[id] = costStr
		}
		if titleChanged {
			prevTitle[id] = title
		}
		confirmedActivity := prevActivity[id]
		prevActivityMu.Unlock()
```

Wichtig: `actStr` darf ab hier **nicht** mehr für Emit oder Nebenwirkungen verwendet werden — es ist der unbestätigte Rohwert. Ersetze im restlichen Schleifenkörper jedes `actStr` durch `confirmedActivity`, also im `log.Printf`, im `ActivityInfo{Activity: ...}` und in allen vier Vergleichen `actStr == "done"` / `actStr == "idle"` sowie in den Aufrufen `notifyFinishOnActivity(id, ...)` und `onActivityChangeForIssue(id, ..., costStr)`.

- [ ] **Step 6: Extend the cleanup**

In `internal/backend/app_scan.go`, Funktion `cleanupActivityCache`, innerhalb des bestehenden `prevActivityMu`-Blocks ergänzen:

```go
	cleanupActivityDebounce(id)
```

- [ ] **Step 7: Correct the spec**

In `docs/superpowers/specs/2026-08-14-activity-duration-design.md` den Abschnitt „### 7. Nebenwirkungen umhängen" ersetzen:

```markdown
### 7. Nebenwirkungen

`processQueue`, `notifyOrchestratorDone`, `notifyFinishOnActivity` und
`onActivityChangeForIssue` (`app_scan.go:184-209`) hängen bereits alle an
derselben lokalen Variable `activityChanged`. Sie müssen deshalb nicht
umgehängt werden — es genügt, deren Berechnung auf `confirmActivity`
umzustellen, dann ziehen alle vier automatisch mit.

Das ist der Teil mit der größten Wirkung jenseits der Optik:
`reportIssueProgress` hat keinerlei Dedup (`app_issue_progress.go:20-70`). Mit
aktivem `auto_comment_on_done` bedeutet jeder Scheinabschluss heute einen
GitHub-Kommentar, mit `auto_close_issue` einen erneuten Close-Versuch.
```

- [ ] **Step 8: Run the full backend package**

Run: `go test ./internal/backend/ -count=1` und `go vet ./...`
Expected: `ok` bzw. keine Ausgabe.

Schlägt ein Queue- oder Orchestrator-Test fehl, liegt es fast sicher daran, dass er einen Zustandswechsel in einem einzigen Tick erwartet. Solche Tests müssen `confirmActivity` zweimal über das Fenster hinweg füttern oder `prevActivity` direkt vorbelegen — passe sie einzeln an und halte fest, warum.

- [ ] **Step 9: Commit**

```bash
git add internal/backend/app_scan.go internal/backend/app_scan_debounce.go internal/backend/app_scan_debounce_test.go docs/superpowers/specs/2026-08-14-activity-duration-design.md
git commit -m "fix(scan): confirm an activity state before acting on it

classifyScreenState is a snapshot without hysteresis: a tick landing between
ESC[2K and the redraw of the input box reads the very same screen as idle. A
single such tick used to count as a state change — and every consumer keys off
that one flag, so a flicker advanced the pipeline queue, notified the
orchestrator and posted an issue comment, the last of which has no dedup at all.

A differing raw state now has to hold for debounceWindow before it counts.
Emit, queue, orchestrator and issue reporting all read the confirmed value.

Closes #188

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Zeitstempel ans Frontend übertragen

**Files:**
- Modify: `internal/backend/app_scan.go:13-21` (`ActivityInfo`) und der Emit-Block
- Modify: `internal/backend/app_hooks_setup.go:64-77` (zweiter Emit-Pfad)
- Test: `internal/backend/app_scan_debounce_test.go` (erweitern)

**Interfaces:**
- Consumes: `activitySinceFor(id int) time.Time` aus Task 3.
- Produces: `ActivityInfo.ActivitySince int64` — Unix-Sekunden, `0` heißt unbekannt. JSON-Feld `activitySince`.

- [ ] **Step 1: Write the failing test**

An `internal/backend/app_scan_debounce_test.go` anhängen:

```go
// The wire format carries seconds since epoch; 0 means "unknown" so the badge
// can omit the duration instead of rendering 1970.
func TestActivitySinceUnix(t *testing.T) {
	resetActivityDebounceForTest()

	if got := activitySinceUnix(42); got != 0 {
		t.Errorf("unknown session yielded %d, want 0", got)
	}

	base := time.Unix(7000, 0)
	prevActivityMu.Lock()
	prevActivity[42] = "active"
	confirmActivity(42, "done", base)
	confirmActivity(42, "done", base.Add(debounceWindow))
	prevActivityMu.Unlock()

	if got := activitySinceUnix(42); got != 7000 {
		t.Errorf("activitySinceUnix = %d, want 7000", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/backend/ -run TestActivitySinceUnix -v`
Expected: Compile-Fehler `undefined: activitySinceUnix`.

- [ ] **Step 3: Add the helper**

An `internal/backend/app_scan_debounce.go` anhängen:

```go
// activitySinceUnix returns the confirmed state's start as seconds since epoch,
// or 0 when unknown. The frontend treats 0 as "show the state without a
// duration" rather than rendering an epoch date.
func activitySinceUnix(id int) int64 {
	t := activitySinceFor(id)
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/backend/ -run TestActivitySinceUnix -v`
Expected: PASS.

- [ ] **Step 5: Extend ActivityInfo and both emit sites**

In `internal/backend/app_scan.go` das Struct erweitern:

```go
// ActivityInfo is sent to the frontend when a session's activity state changes.
type ActivityInfo struct {
	ID         int    `json:"id"`
	Activity   string `json:"activity"` // "idle", "active", "done", "waitingPermission", "waitingAnswer", "error"
	Cost       string `json:"cost"`
	Title      string `json:"title"`      // OSC-derived window title (fallback pane name)
	ContextPct int    `json:"contextPct"` // % of context window used (statusline); 0 if unknown
	Model      string `json:"model"`      // model display name (statusline); "" if unknown
	// ActivitySince is when the confirmed state began, as seconds since epoch;
	// 0 when unknown. Travels on the event only — events are plain JSON and do
	// not need models.ts, unlike binding returns.
	ActivitySince int64 `json:"activitySince"`
}
```

Im Emit-Block von `scanAllSessions` das Feld ergänzen:

```go
				ActivitySince: activitySinceUnix(id),
```

Ebenso im zweiten Emit-Pfad in `internal/backend/app_hooks_setup.go` (Block ab Zeile ~64), damit beide Quellen dasselbe Feld liefern und die Anzeige nicht je nach Auslöser springt.

Dort aber **nicht** `activitySinceUnix`, sondern `activitySinceUnixIfState(id, activity)`: Dieser Pfad ist dem Scan-Loop um ein Entprell-Fenster voraus, der gespeicherte Zeitstempel gehört also noch dem Zustand, den das Pane gerade verlässt. Mit dem neuen Label gepaart stünde für rund zwei Sekunden „fertig · 3 Std 20" auf einem eben fertig gewordenen Pane, danach „gerade eben". `0` (= unbekannt, Badge ohne Dauer) ist bis zur Bestätigung die einzige ehrliche Antwort.

- [ ] **Step 6: Run the package**

Run: `go test ./internal/backend/ -count=1` und `go vet ./...`
Expected: `ok` bzw. keine Ausgabe.

- [ ] **Step 7: Commit**

```bash
git add internal/backend/app_scan.go internal/backend/app_scan_debounce.go internal/backend/app_scan_debounce_test.go internal/backend/app_hooks_setup.go
git commit -m "feat(scan): report when the confirmed activity state began

Carries the start of the current state to the frontend so a pane can show how
long it has been finished or waiting. Zero means unknown, which the badge
renders as no duration rather than an epoch date.

Refs #189

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Anzeige im Status-Badge

**Files:**
- Create: `frontend/src/lib/duration.ts`, `frontend/src/lib/duration.test.ts`
- Create: `frontend/src/stores/clock.ts`, `frontend/src/stores/clock.test.ts`
- Modify: `frontend/src/stores/tabs.ts` (`Pane`, `updateActivity`, Entfernen von `CALM_DELAY_MS`)
- Modify: `frontend/src/components/PaneTitlebar.svelte:105-124`
- Modify: `frontend/src/App.svelte` (Event-Handler)

**Interfaces:**
- Consumes: `ActivityInfo.activitySince` (Unix-Sekunden) aus Task 4.
- Produces:
  - `formatDuration(sinceUnix: number, nowMs: number): string` — `''` wenn `sinceUnix` `0`/negativ ist.
  - `now` — ein `Readable<number>` Store mit Millisekunden, 30-Sekunden-Takt.
  - `Pane.activitySince: number` (Unix-Sekunden, `0` = unbekannt).
  - `tabStore.updateActivity(sessionId, activity, cost, activitySince)`.

- [ ] **Step 1: Write the failing tests**

Neue Datei `frontend/src/lib/duration.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { formatDuration } from './duration';

const NOW = 1_800_000_000_000; // ms
const sec = (agoSeconds: number) => Math.floor(NOW / 1000) - agoSeconds;

describe('formatDuration', () => {
  it('returns empty string for an unknown timestamp', () => {
    expect(formatDuration(0, NOW)).toBe('');
  });

  it('collapses anything under a minute', () => {
    expect(formatDuration(sec(0), NOW)).toBe('gerade eben');
    expect(formatDuration(sec(59), NOW)).toBe('gerade eben');
  });

  it('shows whole minutes below an hour', () => {
    expect(formatDuration(sec(60), NOW)).toBe('1 Min');
    expect(formatDuration(sec(12 * 60), NOW)).toBe('12 Min');
    expect(formatDuration(sec(59 * 60 + 59), NOW)).toBe('59 Min');
  });

  it('shows hours and minutes below a day', () => {
    expect(formatDuration(sec(60 * 60), NOW)).toBe('1 Std 0');
    expect(formatDuration(sec(3 * 60 * 60 + 20 * 60), NOW)).toBe('3 Std 20');
    expect(formatDuration(sec(23 * 60 * 60 + 59 * 60), NOW)).toBe('23 Std 59');
  });

  it('switches to whole days after 24 hours', () => {
    expect(formatDuration(sec(24 * 60 * 60), NOW)).toBe('1 Tag');
    expect(formatDuration(sec(3 * 24 * 60 * 60), NOW)).toBe('3 Tage');
  });

  it('never renders a negative duration when clocks disagree', () => {
    expect(formatDuration(sec(-30), NOW)).toBe('gerade eben');
  });
});
```

Neue Datei `frontend/src/stores/clock.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import { now, TICK_MS } from './clock';

describe('now store', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('advances on each tick while subscribed', () => {
    const seen: number[] = [];
    const stop = now.subscribe((v) => seen.push(v));
    const first = seen.length;

    vi.advanceTimersByTime(TICK_MS);
    expect(seen.length).toBe(first + 1);

    vi.advanceTimersByTime(TICK_MS);
    expect(seen.length).toBe(first + 2);
    stop();
  });

  it('runs a single interval no matter how many subscribers', () => {
    const spy = vi.spyOn(globalThis, 'setInterval');
    const a = now.subscribe(() => {});
    const b = now.subscribe(() => {});
    const c = now.subscribe(() => {});
    expect(spy).toHaveBeenCalledTimes(1);
    a(); b(); c();
    spy.mockRestore();
  });

  it('stops ticking once the last subscriber leaves', () => {
    const spy = vi.spyOn(globalThis, 'clearInterval');
    const stop = now.subscribe(() => {});
    stop();
    expect(spy).toHaveBeenCalled();
    spy.mockRestore();
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/duration.test.ts src/stores/clock.test.ts`
Expected: FAIL, `Failed to resolve import "./duration"` bzw. `"./clock"`.

- [ ] **Step 3: Write the implementations**

Neue Datei `frontend/src/lib/duration.ts`:

```ts
/**
 * Formats how long a pane has been in its current state.
 *
 * The output carries no "vor"/"seit" prefix: the state in front of it supplies
 * the tense already ("fertig · 3 Std 20" reads as "finished for 3h20"), and a
 * prefix would have to differ per state to stay correct.
 *
 * @param sinceUnix seconds since epoch; 0 means unknown
 * @param nowMs current time in milliseconds
 * @returns the formatted duration, or '' when unknown
 */
export function formatDuration(sinceUnix: number, nowMs: number): string {
  if (!sinceUnix || sinceUnix <= 0) return '';

  // Clamp: a restored timestamp from a machine whose clock ran ahead would
  // otherwise render as a negative duration.
  const seconds = Math.max(0, Math.floor(nowMs / 1000) - sinceUnix);

  if (seconds < 60) return 'gerade eben';

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} Min`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} Std ${minutes % 60}`;

  const days = Math.floor(hours / 24);
  return days === 1 ? '1 Tag' : `${days} Tage`;
}
```

Neue Datei `frontend/src/stores/clock.ts`:

```ts
import { readable } from 'svelte/store';

/**
 * How often the shared clock advances. Durations are read in minutes and
 * hours, so a coarse tick is enough — and it keeps one interval from waking
 * every pane badge every second.
 */
export const TICK_MS = 30_000;

/**
 * A single shared clock for every duration label in the UI.
 *
 * readable's start function runs on the first subscriber and its teardown on
 * the last, so exactly one interval exists no matter how many panes are open,
 * and none runs while nothing is listening.
 */
export const now = readable(Date.now(), (set) => {
  const timer = setInterval(() => set(Date.now()), TICK_MS);
  return () => clearInterval(timer);
});
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/duration.test.ts src/stores/clock.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit the building blocks**

```bash
git add frontend/src/lib/duration.ts frontend/src/lib/duration.test.ts frontend/src/stores/clock.ts frontend/src/stores/clock.test.ts
git commit -m "feat(ui): add duration formatting and a shared clock store

One interval feeds every duration label; readable's teardown stops it when the
last badge unsubscribes, so 30 open panes still cost a single timer.

Refs #189

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 6: Carry the timestamp through the store**

In `frontend/src/stores/tabs.ts`:

1. Am `Pane`-Typ ergänzen:

```ts
  /** Unix seconds when the current activity state began; 0 = unknown. */
  activitySince: number;
```

2. In `addPane` (und jeder anderen Stelle, die ein `Pane`-Objekt literal anlegt) `activitySince: 0` setzen. Finde sie mit `grep -n "activity:" frontend/src/stores/tabs.ts`.

3. `updateActivity` ersetzen — die gesamte Debounce-Maschinerie entfällt:

```ts
    updateActivity(sessionId: number, activity: string, cost: string, activitySince: number) {
      // No smoothing here anymore: the backend confirms a state over a debounce
      // window before emitting it (see confirmActivity in app_scan_debounce.go),
      // so what arrives has already held still. The old frontend debounce made
      // things worse — it let the spurious "active" through at once and only
      // delayed the recovery.
      update((state) => {
        for (const tab of state.tabs) {
          for (const pane of tab.panes) {
            if (pane.sessionId !== sessionId) continue;
            pane.activity = activity as Pane['activity'];
            if (cost) pane.cost = cost;
            pane.activitySince = activitySince;
          }
        }
        return state;
      });
    },
```

4. `CALM_DELAY_MS`, die `calmDebounce`-Map und den erklärenden Kommentarblock darüber entfernen. Ersetze den Kommentar durch:

```ts
  // Activity states arrive pre-confirmed: the backend holds a differing state
  // for debounceWindow before emitting it, so no smoothing is needed here.
```

Prüfe mit `grep -n "calmDebounce\|CALM_DELAY_MS" frontend/src/stores/tabs.ts`, dass keine Reste bleiben — inklusive eines etwaigen Aufräumens beim Schließen eines Panes.

- [ ] **Step 7: Pass the field from the event handler**

In `frontend/src/App.svelte` den `terminal:activity`-Handler suchen (`grep -n "terminal:activity" frontend/src/App.svelte`) und den vierten Parameter durchreichen:

```ts
        tabStore.updateActivity(data.id, data.activity, data.cost, data.activitySince ?? 0);
```

Das `?? 0` fängt einen Emit-Pfad ab, der das Feld noch nicht setzt.

- [ ] **Step 8: Render it in the badge**

In `frontend/src/components/PaneTitlebar.svelte` im `<script>`-Block ergänzen:

```ts
  import { now } from '../stores/clock';
  import { formatDuration } from '../lib/duration';

  $: durationLabel = formatDuration(pane.activitySince, $now);
  $: badgeText = statusLabel && durationLabel
    ? `${statusLabel} · ${durationLabel}`
    : statusLabel;
  $: badgeTitle = pane.activitySince
    ? `${statusLabel} seit ${new Date(pane.activitySince * 1000).toLocaleString('de-DE')}`
    : '';
```

Im Markup das bestehende Badge-Element ersetzen:

```svelte
    {#if statusLabel}
      <span class="status-badge {statusClass}" title={badgeTitle}>{badgeText}</span>
    {/if}
```

- [ ] **Step 9: Run the frontend suite and the build**

Run: `cd frontend && npx vitest run` und `npm run build`
Expected: alle Tests PASS, Build ohne Fehler.

Schlagen Tests in `tabs.test.ts` fehl, weil `updateActivity` jetzt vier Parameter nimmt: Aufrufe um das vierte Argument ergänzen. Ein Test, der ausdrücklich das alte Debounce-Verhalten prüfte, wird gelöscht — nicht angepasst; er beschreibt Verhalten, das absichtlich verschwunden ist. Halte das im Commit fest.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/stores/tabs.ts frontend/src/components/PaneTitlebar.svelte frontend/src/App.svelte frontend/src/stores/tabs.test.ts
git commit -m "feat(ui): show how long a pane has been in its state

The status badge now reads \"fertig · 3 Std 20\" instead of just \"fertig\", so a
project with several open panes can be triaged without re-reading each one.

Drops the frontend calm-debounce: states arrive confirmed from the backend now,
and the old smoothing let the spurious transition through at once while delaying
the recovery — it lengthened the disturbance instead of hiding it.

Refs #189

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Dauer überlebt den Neustart

**Files:**
- Modify: `internal/config/session.go:36-51` (`SavedPane`)
- Modify: `frontend/wailsjs/go/models.ts` (Klasse `SavedPane`)
- Modify: `frontend/src/lib/session.ts` (`paneToSaved` und Restore)
- Modify: `internal/backend/app_scan_debounce.go` (Binding `SeedActivitySince`)
- Modify: `frontend/wailsjs/go/backend/App.js`, `App.d.ts` (Binding-Signatur)
- Test: `internal/config/session_test.go`, `internal/backend/app_scan_debounce_test.go`, `frontend/src/lib/session.test.ts`

**Interfaces:**
- Consumes: `setActivitySinceFor(id int, t time.Time, state string)` aus Task 3, `Pane.activitySince` und `Pane.activity` aus Task 5.
- Produces: `SavedPane.ActivitySince int64` (JSON `activity_since`) und `SavedPane.ActivityState string` (JSON `activity_state`), sowie die Binding `SeedActivitySince(sessionID int, unix int64, state string)`.

- [ ] **Step 1: Write the failing Go test**

An `internal/config/session_test.go` anhängen:

```go
func TestSavedPaneRoundTripsActivitySince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	in := SessionState{Tabs: []SavedTab{{
		Name:  "t1",
		Panes: []SavedPane{{Name: "p1", ActivitySince: 1700000000, ActivityState: "done"}},
	}}}
	if err := saveSessionTo(path, in); err != nil {
		t.Fatalf("saveSessionTo: %v", err)
	}

	out := loadSessionFrom(path)
	if out == nil {
		t.Fatal("loadSessionFrom returned nil")
	}
	if got := out.Tabs[0].Panes[0].ActivitySince; got != 1700000000 {
		t.Errorf("ActivitySince = %d, want 1700000000", got)
	}
	if got := out.Tabs[0].Panes[0].ActivityState; got != "done" {
		t.Errorf("ActivityState = %q, want %q", got, "done")
	}
}
```

Die verwendeten Namen sind verifiziert: `SessionState` (`session.go:16`), `SavedTab` (`:22`), `saveSessionTo` (`:133`) und `loadSessionFrom` (`:142`). Prüfe nur, ob `path/filepath` in der Testdatei schon importiert ist — die bestehenden `TestRemoveTab_*`-Tests arbeiten ebenfalls mit Pfaden, es sollte da sein.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/config/ -run TestSavedPaneRoundTripsActivitySince -v`
Expected: Compile-Fehler `unknown field ActivitySince`.

- [ ] **Step 3: Add the field**

In `internal/config/session.go` an `SavedPane` anhängen:

```go
	ActivitySince   int64  `json:"activity_since,omitempty"`   // unix seconds when the current activity state began; survives a restart so the pane keeps its duration
	ActivityState   string `json:"activity_state,omitempty"`   // the activity ActivitySince belongs to; the seed is honoured on restore only if the pane confirms this same state first
```

Der Zustand gehört zwingend dazu — warum, steht in Step 8.

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/config/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Update models.ts by hand**

In `frontend/wailsjs/go/models.ts` die Klasse `SavedPane` suchen. Feld deklarieren:

```ts
    activity_since?: number;
    activity_state?: string;
```

Und im Konstruktor zuweisen, neben den anderen primitiven Feldern:

```ts
        this.activity_since = source["activity_since"];
        this.activity_state = source["activity_state"];
```

Wails v3 regeneriert diese Datei nicht — ohne beide Änderungen wird das Feld beim Deserialisieren still verworfen und ist im Frontend immer `undefined`.

- [ ] **Step 6: Write the failing frontend test**

An `frontend/src/lib/session.test.ts` anhängen:

```ts
it('round-trips activitySince through save and restore', () => {
  const pane = makePane({ activitySince: 1700000000, activity: 'done' });
  const saved = paneToSaved(pane);
  expect(saved.activity_since).toBe(1700000000);
  expect(saved.activity_state).toBe('done');
});

it('defaults activitySince to 0 when the saved pane predates the field', () => {
  const saved = { name: 'p1', mode: 0, model: '' } as any;
  expect(savedToPaneActivitySince(saved)).toBe(0);
});
```

`paneToSaved` ist exportiert (`frontend/src/lib/session.ts:127`) und direkt testbar. Eine Helferfunktion `makePane` existiert möglicherweise noch nicht — schau in `session.test.ts` nach, wie die dortigen Tests ein Pane-Objekt aufbauen, und folge dem Muster, statt eine zweite Konvention einzuführen.

`savedToPaneActivitySince` ist **keine** existierende Funktion und soll auch keine werden. Der zweite Test prüft stattdessen den Restore-Pfad direkt: Eine Session-Datei ohne `activity_since` (aus einer älteren Version) darf nicht `undefined` in den Store tragen, sondern muss zu `0` werden — sonst rendert der Badge `NaN`. Formuliere ihn gegen die Restore-Funktion, die du in Step 7 anfasst.

- [ ] **Step 7: Wire save and restore**

In `frontend/src/lib/session.ts`:
- `paneToSaved` schreibt `activity_since: pane.activitySince || 0` und `activity_state: pane.activity || ''`.
- Der Restore-Pfad liest `activitySince: (savedPane as any).activity_since ?? 0` und `activityState: (savedPane as any).activity_state ?? ''` und gibt beides beim Anlegen des Panes mit.

- [ ] **Step 8: Seed the backend on restore**

Der Wert muss zurück ins Backend, sonst steht er zwar im Store, wird aber vom
ersten bestätigten Wechsel überschrieben.

Der Rückweg ist eine **eigene Binding** neben den anderen Debounce-Helfern in
`internal/backend/app_scan_debounce.go`, kein zusätzlicher `CreateSession`-Parameter:

```go
// SeedActivitySince restores a pane's state-start timestamp after a restart.
// state is the activity the timestamp belongs to; the seed is honoured only if
// the pane confirms that same state first. Zero or an empty state is ignored.
func (a *AppService) SeedActivitySince(sessionID int, unix int64, state string) {
	if unix <= 0 {
		return
	}
	setActivitySinceFor(sessionID, time.Unix(unix, 0), state)
}
```

Der Restore-Pfad ruft sie direkt nach `CreateSession` auf und fängt die
Rejection ab (`.catch`), sonst steht eine unbehandelte Promise im Log:

```ts
if (activitySince && activityState) {
  App.SeedActivitySince(sessionId, activitySince, activityState)
    .catch((err) => console.error('[restoreSession] SeedActivitySince failed:', err));
}
```

`frontend/wailsjs/go/backend/App.js` und `App.d.ts` müssen die Signatur
mitführen (die Method-ID bleibt, sie hängt am Namen).

**Warum keine Erweiterung von `CreateSession`:** die Funktion hat über ein
Dutzend Aufrufer in Go und TypeScript, von denen genau dieser eine je einen
Zeitstempel kennt. Alle anderen müssten `0` durchreichen, ohne dass irgendwer
etwas davon hat.

**Warum der Zustand mitmuss** — das ist der Punkt, an dem eine getrennte Binding
sonst kippt: Sie kommt an, *nachdem* `CreateSession` zurückgekehrt ist, also
wenn der Scan-Loop die Session bereits sieht. Ein wiederhergestelltes Pane
startet dabei seine CLI neu, und deren Hochlauf erzeugt durchgehend Output;
`DetectActivity` meldet noch 1,5 s nach dem letzten Byte `Active`
(`activity.go:104-111`) — deutlich mehr als das 1,2-s-Entprell-Fenster. Der
erste nach einem Neustart bestätigte Zustand ist deshalb fast immer ein
flüchtiges `active`. Ein zustandsloser Seed würde genau davon aufgebraucht: das
Pane zeigte „läuft · 3 Std 20" auf einer zwei Sekunden alten Session, und sobald
sie sich auf `done` legt, ist der Seed verbraucht und die Dauer beginnt bei
„gerade eben". Beide Hälften falsch, und zwar im Langsteh-Fall, für den #189
überhaupt existiert.

`confirmActivity` behält den Seed deshalb nur, wenn der erste bestätigte Zustand
zum gespeicherten passt, und verwirft ihn sonst — er wartet **nicht** auf ein
späteres Vorkommen desselben Zustands, das dann Stunden zu früh datiert wäre.
`setActivitySinceFor` nimmt den Seed außerdem nur an, solange `prevActivity[id]`
leer ist: trifft er nach der ersten Bestätigung ein, hat er nichts mehr zu
korrigieren.

Go-Tests dazu in `internal/backend/app_scan_debounce_test.go`:
- Seed überlebt die erste Bestätigung, wenn deren Zustand passt — und
  `confirmActivity` meldet dabei `true` (sonst bestünde der Test auch, wenn gar
  nichts mehr bestätigt).
- Die *zweite* Transition bekommt einen frischen Zeitstempel.
- Ein Seed für `done` wird vom `active` des Hochlaufs verworfen und taucht auch
  beim späteren echten `done` nicht wieder auf.
- Ein Seed nach der ersten Bestätigung wird abgelehnt.
- Ein Seed ohne Zustand wird abgelehnt.

- [ ] **Step 9: Run everything**

Run:
```bash
go test ./... -count=1
go vet ./...
cd frontend && npx vitest run && npm run build
```
Expected: alle Go-Pakete `ok`, `vet` ohne Ausgabe, alle Frontend-Tests PASS, Build sauber.

- [ ] **Step 10: Commit**

```bash
git add internal/config/session.go internal/backend/app_scan_debounce.go internal/backend/app_scan_debounce_test.go frontend/wailsjs/go/models.ts frontend/wailsjs/go/backend/App.js frontend/wailsjs/go/backend/App.d.ts frontend/src/lib/session.ts frontend/src/lib/session.test.ts internal/config/session_test.go
git commit -m "feat: keep a pane's state duration across a restart

A pane open for hours is exactly the case the duration is for, so losing it on
every restart would defeat the point. The timestamp is persisted with the
session and seeded back into the scan loop's bookkeeping on restore.

Closes #189

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Nach dem letzten Task

- [ ] `needs-e2e-testing` an #189 setzen: `gh issue edit 189 --add-label "needs-e2e-testing"`
- [ ] Die e2e-Punkte aus der Spec sind offen und brauchen die laufende App mit echtem Claude CLI: bleibt die Anzeige über eine längere Sitzung ruhig, wächst die Dauer monoton ohne Rücksprünge bei Repaints, überlebt sie einen Neustart, und fühlt sich die Reaktion auf einen echten Wechsel nicht träger an.
- [ ] **Nicht pushen, keinen PR anlegen** — dafür ist die Zustimmung des Nutzers einzuholen.

## Self-Review-Notizen

Gegen die Spec geprüft:

| Spec-Abschnitt | Task |
|---|---|
| 1. Schreibseite bereinigen | Task 1 (Lese-Pfad), Task 2 (`Notification`) |
| 2. Entprellung im Scan-Loop | Task 3 |
| 3. Zeitstempel | Task 3 (`activitySince`) |
| 4. Transport | Task 4 |
| 5. Anzeige | Task 5 |
| 6. Persistenz | Task 6 |
| 7. Nebenwirkungen | Task 3 — entfällt als eigene Arbeit, Spec dort korrigiert |
| Aufräumen der Maps | Task 3, Step 6 |
| Zweiter Emit-Pfad | Task 4, Step 5 |

Der zweite Emit-Pfad (`onHookActivity`, `app_hooks_setup.go`) wird in Task 4 nur um das neue Feld ergänzt, nicht auf `confirmActivity` umgestellt. Er emittiert weiterhin ohne Dedup direkt aus dem Hook-Callback. Das ist bewusst: Ihn vollständig auf die Entprellung zu ziehen hieße, die Hook-Latenz aufzugeben, die ihn überhaupt rechtfertigt. Nach Task 2 ist seine schlimmste Fehlerquelle (`Notification` → `Done`) beseitigt. Bleibt im Betrieb ein Zucken sichtbar, ist das der nächste Verdächtige — dann als eigene Änderung mit eigener Begründung.

Der Emit ist dann aber auch **alles**, was er tun darf. `processQueue` und `onActivityChangeForIssue` hingen ursprünglich ebenfalls am Hook-Callback und liefen damit zweimal je Abschluss — einmal sofort, einmal rund zwei Sekunden später am bestätigten Wechsel. `reportIssueProgress` hat keine Dedup, das waren also zwei GitHub-Kommentare pro Abschluss. Nebenwirkungen gehören ausschließlich an den bestätigten Wechsel im Scan-Loop; dort sind sie auch nicht an `a.app != nil` gebunden, denn der Event-Emitter ist Anzeige und sagt nichts darüber, ob die Queue weiterlaufen muss.
