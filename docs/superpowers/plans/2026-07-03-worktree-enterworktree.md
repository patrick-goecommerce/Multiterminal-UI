# Worktree-Isolation über EnterWorktree — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** MTUI erkennt per Hook, wenn Claude Code selbständig sein natives `EnterWorktree`-Tool nutzt, zeigt den Status in der Pane-Titelleiste an, und bietet ein manuelles Aufräum-Netz für verwaiste Worktrees — ohne selbst Worktrees zu erzeugen oder Merges zu orchestrieren.

**Architecture:** Erweiterung der bestehenden Hook-Pipeline (`mtui-hook` → JSONL → `HookManager`) um `PostToolUse`-Events mit `tool_name=="EnterWorktree"` auszuwerten und `cwd` generell mitzuführen. Ein neuer, kleiner Zustand pro Session trackt den aktuell erkannten Worktree; Events an das Frontend spiegeln das in ein reines Status-Badge. Ein einmaliger Projekt-Setup-Schritt hinterlegt Memory-Anweisung + Settings. Die komplette alte Erzeugungs-/Merge-Orchestrierung (LaunchDialog-Checkbox, ✓-Finish-Button, WorktreeFinishDialog) wird aus dem UI entfernt; die zugehörigen Backend-Dateien bleiben unangetastet als Referenz liegen.

**Tech Stack:** Go 1.21+ (Backend), Svelte 4 + TypeScript (Frontend), Claude Code Hooks (`PostToolUse`), git CLI via `gitCmd`.

## Global Constraints

- **Alle git-Aufrufe über die bestehende `gitCmd`-Funktion** (`internal/backend/app_git_cmd.go`) — enthält `hideConsole`, Pflicht für jeden neuen Prozess-Spawn.
- **Kein `git branch -D`** — nur `-d` (verweigert bei ungemergten Branches). Kein `--force` als Erststrategie bei `git worktree remove`.
- **UI-Texte Deutsch, Code/Kommentare Englisch.**
- **Frontend-exponierte Go-Structs brauchen `json`+`yaml`-Tags UND manuellen Sync in `frontend/wailsjs/go/models.ts` + `frontend/wailsjs/go/backend/App.d.ts`/`App.js`** (Wails v3 regeneriert das NICHT automatisch — bekannter Wiederholungsbug). Für `App.js`-Bindings: FNV-1a-32-Hash über `github.com/patrick-goecommerce/Multiterminal-UI/internal/backend.AppService.<Method>`, verifizierbar mit dem vorhandenen Skript `.superpowers/sdd/verify-hashes.js` (aus einer früheren Session; falls nicht mehr vorhanden, nach demselben Muster neu schreiben: `Math.imul(hash, 0x01000193)` FNV-1a über den vollqualifizierten Methodennamen, kalibriert gegen 3 bekannte IDs im bestehenden `App.js`).
- **SVELTE-RECURRING-BUG: Niemals Variablen-Zuweisungen direkt in `$:`-Blöcke schreiben** — Svelte trackt jede referenzierte Variable (Lesen UND Schreiben) als Dependency; das reißt bei jeder Änderung alle Felder auf Default zurück. Init-Logik gehört in Funktionen, aufgerufen aus einem `$: if (bedingung) fn();`.
- **Max 300 Zeilen pro neue Go-Datei.**
- **Bestehende Backend-Dateien der alten Worktree-Erzeugung/-Merge-Logik (`app_worktree_pane.go`, `app_worktree_pane_files.go`, `app_worktree_finish.go`, `app_worktree_finish_status.go`, `app_worktree_cleanup.go`, `app_worktree_marker.go`, `app_worktree_shell.go`, `kill_windows.go`/`kill_other.go`) werden NICHT gelöscht** — sie bleiben als mögliche Referenz/Zusatzfunktion liegen (Spec Abschnitt 8). Dieser Plan entfernt nur ihre FRONTEND-Verdrahtung.
- Tests: `go test ./internal/... ./cmd/...`, `go vet ./...`, `cd frontend && npm run build && npx vitest run`.

## Datei-Landkarte

| Datei | Verantwortung |
|---|---|
| `cmd/mtui-hook/main.go` (mod) | `cwd` + `tool_response` (nur für `EnterWorktree`) aus dem Claude-Code-Hook-JSON erfassen und in die JSONL-Zeile schreiben |
| `internal/backend/app_hooks.go` (mod) | `rawHookEvent` um `Cwd`/`WorktreePath`/`WorktreeBranch` erweitern, neuer `onWorktreeChange`-Callback |
| `internal/backend/app_hooks_setup.go` (mod) | Callback verdrahten |
| `internal/backend/app_worktree_detect.go` (neu) | Pro-Session-Zustand (aktueller Worktree), Event-Emission `worktree:detected`/`worktree:cleared` |
| `internal/backend/app_worktree.go` (mod) | `categorizeWorktree` um `.claude/worktrees/`-Präfix → Kategorie `"claude"` erweitern |
| `internal/backend/app_worktree_orphan.go` (neu) | `RemoveOrphanedWorktree(path string) error` — manuelle, sichere Aufräum-Primitive |
| `internal/backend/app_worktree_setup.go` (neu) | `EnsureProjectWorktreeSetup(dir string) error` — einmalige Memory-Datei + `.claude/settings.local.json` |
| `internal/backend/app_events.go` (mod) | `WorktreeDetectedEvent`/`WorktreeClearedEvent`-Structs |
| `frontend/wailsjs/go/models.ts`, `App.d.ts`, `App.js` (mod) | Sync der neuen Backend-Methoden |
| `frontend/src/stores/tabs.ts` (mod) | `finishPhase` entfernen, `setWorktree`/`clearWorktree`-Methoden |
| `frontend/src/App.svelte` (mod) | Alte Finish-Flow-Verdrahtung entfernen, neue Detection-Events verdrahten, Projekt-Setup beim Claude-Launch anstoßen |
| `frontend/src/components/PaneTitlebar.svelte` (mod) | ✓-Button/Spinner entfernen, Badge bleibt |
| `frontend/src/components/PaneGrid.svelte`, `TerminalPane.svelte` (mod) | `finishWorktree`/`cancelFinish`-Passthrough entfernen |
| `frontend/src/components/LaunchDialog.svelte` (mod) | Worktree-Checkbox/-Felder entfernen |
| `frontend/src/components/WorktreeFinishDialog.svelte` (löschen) | Nicht mehr genutzt |
| `frontend/src/components/WorktreeDropdown.svelte` (mod) | Neue Kategorie „verwaist" mit Aufräum-Aktion |

---

### Task 1: `mtui-hook` — `cwd` + `EnterWorktree`-Tool-Response erfassen

**Files:**
- Modify: `cmd/mtui-hook/main.go`
- Test: `cmd/mtui-hook/main_test.go`

**Interfaces:**
- Consumes: nichts Neues (stdlib `encoding/json`)
- Produces: `hookLine` bekommt neue Felder `Cwd string`, `WorktreePath string`, `WorktreeBranch string` (json-Tags `cwd`, `worktree_path`, `worktree_branch`, alle `omitempty`) — von Task 2 gelesen.

- [ ] **Step 1: Failing Test ergänzen** (an `cmd/mtui-hook/main_test.go` anhängen)

```go
func TestRunCapturesCwdAndEnterWorktreeResponse(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "9")
	os.Args = []string{"mtui-hook", "PostToolUse"}

	stdin := `{"session_id":"xyz","cwd":"D:\\repos\\proj\\.claude\\worktrees\\feature-a","tool_name":"EnterWorktree","tool_input":{"name":"feature-a"},"tool_response":{"worktreePath":"D:\\repos\\proj\\.claude\\worktrees\\feature-a","worktreeBranch":"worktree-feature-a","message":"Created worktree..."}}`
	r, w, _ := os.Pipe()
	w.Write([]byte(stdin))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	data, err := os.ReadFile(filepath.Join(appData, "Multiterminal", "hooks", "xyz.jsonl"))
	if err != nil {
		t.Fatalf("jsonl not written: %v", err)
	}
	var line struct {
		Event          string `json:"event"`
		Tool           string `json:"tool"`
		Cwd            string `json:"cwd"`
		WorktreePath   string `json:"worktree_path"`
		WorktreeBranch string `json:"worktree_branch"`
	}
	if err := json.Unmarshal(data[:len(data)-1], &line); err != nil {
		t.Fatalf("bad jsonl: %v (%q)", err, data)
	}
	if line.Cwd != `D:\repos\proj\.claude\worktrees\feature-a` {
		t.Errorf("cwd = %q", line.Cwd)
	}
	if line.WorktreePath != `D:\repos\proj\.claude\worktrees\feature-a` {
		t.Errorf("worktree_path = %q", line.WorktreePath)
	}
	if line.WorktreeBranch != "worktree-feature-a" {
		t.Errorf("worktree_branch = %q", line.WorktreeBranch)
	}
}

func TestRunIgnoresToolResponseForOtherTools(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "9")
	os.Args = []string{"mtui-hook", "PostToolUse"}

	stdin := `{"session_id":"abc2","cwd":"D:\\repos\\proj","tool_name":"Bash","tool_response":{"worktreePath":"should-not-leak"}}`
	r, w, _ := os.Pipe()
	w.Write([]byte(stdin))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	data, err := os.ReadFile(filepath.Join(appData, "Multiterminal", "hooks", "abc2.jsonl"))
	if err != nil {
		t.Fatalf("jsonl not written: %v", err)
	}
	var line struct {
		WorktreePath string `json:"worktree_path"`
	}
	json.Unmarshal(data[:len(data)-1], &line)
	if line.WorktreePath != "" {
		t.Errorf("worktree_path leaked for non-EnterWorktree tool: %q", line.WorktreePath)
	}
}
```

- [ ] **Step 2: Testlauf — muss fehlschlagen**

Run: `go test ./cmd/mtui-hook/ -run 'TestRunCapturesCwd|TestRunIgnoresToolResponse' -v`
Expected: FAIL (Felder `cwd`/`worktree_path`/`worktree_branch` existieren noch nicht in `hookLine`)

- [ ] **Step 3: Implementierung** (`cmd/mtui-hook/main.go` ändern)

```go
type claudeEvent struct {
	SessionID    string          `json:"session_id"`
	ToolName     string          `json:"tool_name"`
	Message      string          `json:"message"`
	Prompt       string          `json:"prompt"`
	Cwd          string          `json:"cwd"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

type enterWorktreeResponse struct {
	WorktreePath   string `json:"worktreePath"`
	WorktreeBranch string `json:"worktreeBranch"`
}

// hookLine mirrors internal/backend.rawHookEvent — keep the json tags in sync.
type hookLine struct {
	Ts             int64  `json:"ts"`
	Event          string `json:"event"`
	SessionID      string `json:"session_id"`
	MtID           int    `json:"mt_id"`
	Tool           string `json:"tool"`
	Message        string `json:"message"`
	Cwd            string `json:"cwd,omitempty"`
	WorktreePath   string `json:"worktree_path,omitempty"`
	WorktreeBranch string `json:"worktree_branch,omitempty"`
}
```

In `run()`, nach dem bestehenden `message`-Block (vor dem `line, err := json.Marshal(...)`-Aufruf) ergänzen:

```go
	var worktreePath, worktreeBranch string
	if ev.ToolName == "EnterWorktree" && len(ev.ToolResponse) > 0 {
		var wt enterWorktreeResponse
		if json.Unmarshal(ev.ToolResponse, &wt) == nil {
			worktreePath = wt.WorktreePath
			worktreeBranch = wt.WorktreeBranch
		}
	}
```

Und den `json.Marshal(hookLine{...})`-Aufruf um die drei Felder erweitern:

```go
	line, err := json.Marshal(hookLine{
		Ts:             time.Now().Unix(),
		Event:          eventType,
		SessionID:      sessionID,
		MtID:           mtID,
		Tool:           ev.ToolName,
		Message:        message,
		Cwd:            ev.Cwd,
		WorktreePath:   worktreePath,
		WorktreeBranch: worktreeBranch,
	})
```

- [ ] **Step 4: Testlauf — muss grün sein**

Run: `go test ./cmd/mtui-hook/... -v`
Expected: PASS (alle Tests inkl. der bestehenden `TestRunWritesJSONLLine`)

- [ ] **Step 5: Commit**

```bash
git add cmd/mtui-hook/main.go cmd/mtui-hook/main_test.go
git commit -m "feat(hooks): capture cwd + EnterWorktree tool_response in mtui-hook"
```

---

### Task 2: `HookManager` — Worktree-Detection-Callback

**Files:**
- Modify: `internal/backend/app_hooks.go`
- Test: `internal/backend/app_hooks_test.go`

**Interfaces:**
- Consumes: `rawHookEvent` (erweitert um Cwd/WorktreePath/WorktreeBranch aus Task 1)
- Produces: `HookManager.onWorktreeChange func(mtID int, worktreePath, worktreeBranch, cwd string)` — optionaler Callback, aufgerufen bei jedem Event mit `ev.WorktreePath != ""` (EnterWorktree erkannt) ODER bei jedem Event überhaupt (damit der Aufrufer selbst per `cwd` erkennen kann, ob eine Session einen zuvor bekannten Worktree wieder verlassen hat — von Task 3 genutzt).

- [ ] **Step 1: Failing Test ergänzen** (an `internal/backend/app_hooks_test.go` anhängen — Datei existiert bereits, Muster der bestehenden Tests dort übernehmen für Session-Stub/Verzeichnis-Setup)

```go
func TestHandleEvent_CallsOnWorktreeChangeForEnterWorktree(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session {
		if mtID == 1 {
			return sess
		}
		return nil
	}, nil)

	var gotPath, gotBranch, gotCwd string
	var calls int
	hm.onWorktreeChange = func(mtID int, worktreePath, worktreeBranch, cwd string) {
		calls++
		gotPath, gotBranch, gotCwd = worktreePath, worktreeBranch, cwd
	}

	hm.handleEvent(rawHookEvent{
		Event: "PostToolUse", MtID: 1, SessionID: "s1", Tool: "EnterWorktree",
		Cwd: `D:\repos\proj\.claude\worktrees\feature-a`,
		WorktreePath: `D:\repos\proj\.claude\worktrees\feature-a`, WorktreeBranch: "worktree-feature-a",
	})

	if calls != 1 {
		t.Fatalf("onWorktreeChange called %d times, want 1", calls)
	}
	if gotPath != `D:\repos\proj\.claude\worktrees\feature-a` || gotBranch != "worktree-feature-a" {
		t.Errorf("got path=%q branch=%q", gotPath, gotBranch)
	}
	if gotCwd != `D:\repos\proj\.claude\worktrees\feature-a` {
		t.Errorf("got cwd=%q", gotCwd)
	}
}

func TestHandleEvent_CallsOnWorktreeChangeWithEmptyPathForOrdinaryEvents(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session { return sess }, nil)

	var gotPath string
	var calls int
	hm.onWorktreeChange = func(mtID int, worktreePath, worktreeBranch, cwd string) {
		calls++
		gotPath = worktreePath
	}

	hm.handleEvent(rawHookEvent{Event: "PostToolUse", MtID: 1, SessionID: "s1", Tool: "Bash", Cwd: `D:\repos\proj`})

	if calls != 1 {
		t.Fatalf("onWorktreeChange called %d times, want 1 (ordinary event must still report cwd)", calls)
	}
	if gotPath != "" {
		t.Errorf("worktreePath = %q, want empty for non-EnterWorktree event", gotPath)
	}
}
```

- [ ] **Step 2: Testlauf — muss fehlschlagen**

Run: `go test ./internal/backend/ -run 'TestHandleEvent_CallsOnWorktreeChange' -v`
Expected: FAIL (`onWorktreeChange`-Feld existiert nicht)

- [ ] **Step 3: Implementierung** (`internal/backend/app_hooks.go`)

`rawHookEvent`-Struct erweitern:

```go
type rawHookEvent struct {
	Ts             int64  `json:"ts"`
	Event          string `json:"event"`
	SessionID      string `json:"session_id"`
	MtID           int    `json:"mt_id"`
	Tool           string `json:"tool"`
	Message        string `json:"message"`
	Cwd            string `json:"cwd"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
}
```

`HookManager`-Struct um das Feld ergänzen (nach `onPrompt`):

```go
	// onWorktreeChange, if set, is called on EVERY hook event with the
	// session's current cwd, plus worktreePath/worktreeBranch when the event
	// is a PostToolUse:EnterWorktree detection (empty strings otherwise — the
	// caller uses cwd to notice when a session has left a previously known
	// worktree, spec 2026-07-03 section 4).
	onWorktreeChange func(mtID int, worktreePath, worktreeBranch, cwd string)
```

In `handleEvent`, nach dem bestehenden `UserPromptSubmit`-Block (vor `newState := hookEventToActivity(...)`) ergänzen:

```go
	if hm.onWorktreeChange != nil {
		hm.onWorktreeChange(ev.MtID, ev.WorktreePath, ev.WorktreeBranch, ev.Cwd)
	}
```

- [ ] **Step 4: Testlauf — muss grün sein**

Run: `go test ./internal/backend/... -v 2>&1 | tail -30`
Expected: PASS, keine Regression bestehender Hook-Tests

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_hooks.go internal/backend/app_hooks_test.go
git commit -m "feat(hooks): dispatch onWorktreeChange callback with cwd tracking"
```

---

### Task 3: Pro-Session-Worktree-Zustand + Detection-Events

**Files:**
- Create: `internal/backend/app_worktree_detect.go`
- Modify: `internal/backend/app_hooks_setup.go` (Callback verdrahten)
- Modify: `internal/backend/app_events.go` (Event-Structs)
- Modify: `internal/backend/app.go` (Cleanup in `CloseSession`)
- Test: `internal/backend/app_worktree_detect_test.go`

**Interfaces:**
- Consumes: `HookManager.onWorktreeChange` (Task 2), `mainRepoRoot(dir) (string, error)` und `checkedOutBranch(root) string` (bestehend, `app_worktree_pane.go`/`app_worktree_finish_status.go` — unverändert wiederverwendet)
- Produces:

```go
func (a *AppService) onWorktreeChange(mtID int, worktreePath, worktreeBranch, cwd string) // HookManager-Callback
func (a *AppService) currentWorktree(sessionID int) (path, branch string, ok bool)        // für Task 12 (Orphan-Filter) und Tests
```

Events (`app_events.go`):

```go
// WorktreeDetectedEvent is emitted when a Claude session enters a native
// EnterWorktree-created worktree.
type WorktreeDetectedEvent struct {
	ID           int    `json:"id"`
	WorktreePath string `json:"worktreePath"`
	WorktreeBranch string `json:"worktreeBranch"`
	TargetBranch string `json:"targetBranch"`
}

// WorktreeClearedEvent is emitted when a session's cwd is no longer inside a
// previously detected worktree (Claude called ExitWorktree, or otherwise left).
type WorktreeClearedEvent struct {
	ID int `json:"id"`
}
```

Event-Namen: `worktree:detected`, `worktree:cleared`.

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_detect_test.go
package backend

import "testing"

func newDetectTestApp() *AppService {
	return &AppService{
		worktreeState: map[int]worktreeState{},
	}
}

func TestOnWorktreeChange_DetectsNewWorktree(t *testing.T) {
	repo := initPaneTestRepo(t) // existing helper from app_worktree_pane_test.go, still present
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	var emitted *WorktreeDetectedEvent
	a.emitWorktreeEvent = func(name string, payload any) {
		if ev, ok := payload.(WorktreeDetectedEvent); ok {
			emitted = &ev
		}
	}

	a.onWorktreeChange(1, wt, "worktree-feature-a", wt)

	if emitted == nil {
		t.Fatal("expected WorktreeDetectedEvent to be emitted")
	}
	if emitted.WorktreePath != wt || emitted.WorktreeBranch != "worktree-feature-a" {
		t.Errorf("unexpected event: %+v", emitted)
	}
	if emitted.TargetBranch != "alpha-main" {
		t.Errorf("targetBranch = %q, want alpha-main (checked out in main worktree)", emitted.TargetBranch)
	}
	path, branch, ok := a.currentWorktree(1)
	if !ok || path != wt || branch != "worktree-feature-a" {
		t.Errorf("currentWorktree = %q/%q/%v", path, branch, ok)
	}
}

func TestOnWorktreeChange_ClearsWhenCwdLeavesWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(wt, 0755)
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	a.emitWorktreeEvent = func(string, any) {}
	a.onWorktreeChange(1, wt, "worktree-feature-a", wt) // enter

	var cleared *WorktreeClearedEvent
	a.emitWorktreeEvent = func(name string, payload any) {
		if ev, ok := payload.(WorktreeClearedEvent); ok {
			cleared = &ev
		}
	}
	a.onWorktreeChange(1, "", "", repo) // ordinary event, cwd back at main repo

	if cleared == nil || cleared.ID != 1 {
		t.Fatalf("expected WorktreeClearedEvent for session 1, got %+v", cleared)
	}
	if _, _, ok := a.currentWorktree(1); ok {
		t.Error("currentWorktree still reports a worktree after clear")
	}
}

func TestOnWorktreeChange_NoOpWhenCwdStaysInsideKnownWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(wt, 0755)
	gitRun(t, repo, "worktree", "add", "-b", "worktree-feature-a", wt)

	a := newDetectTestApp()
	events := 0
	a.emitWorktreeEvent = func(string, any) { events++ }
	a.onWorktreeChange(1, wt, "worktree-feature-a", wt) // enter: 1 event
	a.onWorktreeChange(1, "", "", filepath.Join(wt, "sub")) // still inside: no new event

	if events != 1 {
		t.Errorf("events = %d, want 1 (no re-emit while still inside the same worktree)", events)
	}
}
```

- [ ] **Step 2: Testlauf — muss fehlschlagen**

Run: `go test ./internal/backend/ -run TestOnWorktreeChange -v`
Expected: FAIL/compile error (`worktreeState`, `emitWorktreeEvent`, `onWorktreeChange`, `currentWorktree` existieren nicht)

- [ ] **Step 3: Implementierung**

3a. `app.go`: `AppService`-Struct um zwei Felder ergänzen (neben der bestehenden `sessions map[int]*terminal.Session`-Deklaration):

```go
	worktreeStateMu sync.Mutex
	worktreeState   map[int]worktreeState
	// emitWorktreeEvent is a seam for testing; production wiring assigns it to
	// a.app.Event.Emit in setupHooks. Never nil-checked directly — callers use
	// the emitWorktreeEventSafe helper below.
	emitWorktreeEvent func(name string, payload any)
```

Im Konstruktor (neben `sessions: map[int]*terminal.Session{}` oder äquivalenter Init-Stelle): `worktreeState: map[int]worktreeState{},`.

In `CloseSession` (neben `delete(a.sessions, id)`, Zeile ~282): `a.worktreeStateMu.Lock(); delete(a.worktreeState, id); a.worktreeStateMu.Unlock()`.

3b. `app_events.go`: die zwei Structs aus dem Interfaces-Block anhängen.

3c. Neue Datei `internal/backend/app_worktree_detect.go`:

```go
// Package backend – detection of Claude Code's native EnterWorktree tool via
// the existing hook pipeline. MTUI does not create worktrees itself here; it
// only observes what Claude decided to do (spec 2026-07-03).
package backend

import (
	"log"
	"strings"
)

// worktreeState is the currently known worktree for one MTUI session.
type worktreeState struct {
	Path   string
	Branch string
}

// emitWorktreeEventSafe calls the emit seam if set (nil in unit tests unless
// explicitly wired) and if the real app is present.
func (a *AppService) emitWorktreeEventSafe(name string, payload any) {
	if a.emitWorktreeEvent != nil {
		a.emitWorktreeEvent(name, payload)
		return
	}
	if a.app != nil {
		a.app.Event.Emit(name, payload)
	}
}

// currentWorktree returns the worktree currently tracked for a session.
func (a *AppService) currentWorktree(sessionID int) (path, branch string, ok bool) {
	a.worktreeStateMu.Lock()
	defer a.worktreeStateMu.Unlock()
	st, exists := a.worktreeState[sessionID]
	return st.Path, st.Branch, exists
}

// onWorktreeChange is the HookManager callback (wired in setupHooks). It is
// invoked on EVERY hook event for a session: worktreePath/worktreeBranch are
// non-empty only on a fresh EnterWorktree detection; cwd is always populated
// and used to notice when a session has left a previously known worktree.
func (a *AppService) onWorktreeChange(mtID int, worktreePath, worktreeBranch, cwd string) {
	if worktreePath != "" {
		a.handleWorktreeDetected(mtID, worktreePath, worktreeBranch)
		return
	}
	a.handleWorktreeCwdUpdate(mtID, cwd)
}

func (a *AppService) handleWorktreeDetected(mtID int, worktreePath, worktreeBranch string) {
	a.worktreeStateMu.Lock()
	a.worktreeState[mtID] = worktreeState{Path: worktreePath, Branch: worktreeBranch}
	a.worktreeStateMu.Unlock()

	target := ""
	if root, err := mainRepoRoot(worktreePath); err == nil {
		target = checkedOutBranch(root)
	} else {
		log.Printf("[worktree-detect] mainRepoRoot(%s): %v", worktreePath, err)
	}

	log.Printf("[worktree-detect] session %d entered %s on %s (target %s)", mtID, worktreePath, worktreeBranch, target)
	a.emitWorktreeEventSafe("worktree:detected", WorktreeDetectedEvent{
		ID: mtID, WorktreePath: worktreePath, WorktreeBranch: worktreeBranch, TargetBranch: target,
	})
}

// handleWorktreeCwdUpdate clears the tracked worktree once cwd is observed
// outside it. Ordinary events for a session with no known worktree are a
// silent no-op — only a real transition emits worktree:cleared.
func (a *AppService) handleWorktreeCwdUpdate(mtID int, cwd string) {
	a.worktreeStateMu.Lock()
	st, known := a.worktreeState[mtID]
	if !known {
		a.worktreeStateMu.Unlock()
		return
	}
	stillInside := cwd == "" || strings.EqualFold(cwd, st.Path) || strings.HasPrefix(strings.ToLower(cwd), strings.ToLower(st.Path)+`\`) || strings.HasPrefix(strings.ToLower(cwd), strings.ToLower(st.Path)+"/")
	if stillInside {
		a.worktreeStateMu.Unlock()
		return
	}
	delete(a.worktreeState, mtID)
	a.worktreeStateMu.Unlock()

	log.Printf("[worktree-detect] session %d left %s", mtID, st.Path)
	a.emitWorktreeEventSafe("worktree:cleared", WorktreeClearedEvent{ID: mtID})
}
```

3d. `app_hooks_setup.go`: nach `a.hookMgr.onPrompt = a.maybeGeneratePaneName` ergänzen:

```go
	a.hookMgr.onWorktreeChange = a.onWorktreeChange
```

- [ ] **Step 4: Testlauf — muss grün sein**

Run: `go test ./internal/backend/... -v 2>&1 | tail -40` und `go vet ./...` und `go build ./...`
Expected: alle PASS, kein Vet-/Build-Fehler

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_detect.go internal/backend/app_worktree_detect_test.go internal/backend/app_hooks_setup.go internal/backend/app_events.go internal/backend/app.go
git commit -m "feat(worktree): detect native EnterWorktree via hook cwd tracking"
```

---

### Task 4: Kategorisierung `.claude/worktrees/` als eigene Kategorie

**Files:**
- Modify: `internal/backend/app_worktree.go`
- Test: `internal/backend/app_worktree_test.go` (Datei existiert bereits — an bestehende `categorizeWorktree`-Tests anhängen; falls kein passender Testfile-Name gefunden wird, `grep -rl categorizeWorktree internal/backend/*_test.go` prüfen und dort ergänzen)

**Interfaces:**
- Consumes: nichts Neues
- Produces: `categorizeWorktree` setzt `Category: "claude"` für Pfade unter `<root>/.claude/worktrees/` — von Task 12 (Frontend-Orphan-Filter über `ListAllWorktrees`) konsumiert.

- [ ] **Step 1: Failing Test schreiben**

```go
func TestCategorizeWorktree_ClaudeNativePrefix(t *testing.T) {
	root := filepath.Join("D:", "repos", "proj")
	mtPrefix := filepath.Join(root, worktreeDir) + string(filepath.Separator)
	wt := &WorktreeInfo{Path: filepath.Join(root, ".claude", "worktrees", "feature-a")}
	categorizeWorktree(wt, root, mtPrefix)
	if wt.Category != "claude" {
		t.Errorf("category = %q, want claude", wt.Category)
	}
	if wt.Name != "feature-a" {
		t.Errorf("name = %q, want feature-a", wt.Name)
	}
}

func TestCategorizeWorktree_ClaudeNativeNestedName(t *testing.T) {
	// EnterWorktree names may contain "/"-separated segments.
	root := filepath.Join("D:", "repos", "proj")
	mtPrefix := filepath.Join(root, worktreeDir) + string(filepath.Separator)
	wt := &WorktreeInfo{Path: filepath.Join(root, ".claude", "worktrees", "area", "feature-a")}
	categorizeWorktree(wt, root, mtPrefix)
	if wt.Category != "claude" {
		t.Errorf("category = %q, want claude", wt.Category)
	}
}
```

- [ ] **Step 2: Testlauf — muss fehlschlagen**

Run: `go test ./internal/backend/ -run TestCategorizeWorktree_ClaudeNative -v`
Expected: FAIL (Category ist aktuell `"terminal"`, nicht `"claude"`)

- [ ] **Step 3: Implementierung** — in `categorizeWorktree` (`app_worktree.go`), vor dem finalen `wt.Category = "terminal"`-Fallback am Funktionsende einen neuen Zweig einfügen (nach dem bestehenden `mtPrefix`-Block, gleiche Einrückungsebene):

```go
	claudeWtPrefix := strings.ToLower(filepath.Join(root, ".claude", "worktrees")) + string(filepath.Separator)
	if strings.HasPrefix(wtPathNorm, claudeWtPrefix) {
		rel := wt.Path[len(root)+len(string(filepath.Separator))+len(".claude")+len(string(filepath.Separator))+len("worktrees")+len(string(filepath.Separator)):]
		wt.Category = "claude"
		wt.Name = filepath.ToSlash(rel)
		return
	}
```

Hinweis: `rel` wird über eine Index-Berechnung statt `strings.TrimPrefix` gebildet, weil `claudeWtPrefix` klein geschrieben ist (case-insensitive-Vergleich) und direkt auf `wt.Path` angewendet zu falscher Groß-/Kleinschreibung im Ergebnis führen würde — die Länge ist bei beiden identisch, daher ist die Index-Berechnung sicher. Bei Unsicherheit beim Implementieren: alternativ `filepath.Rel(filepath.Join(root, ".claude", "worktrees"), wt.Path)` verwenden (stdlib, robuster) — beide Varianten müssen den zweiten Test (verschachtelter Name mit „/") bestehen.

- [ ] **Step 4: Testlauf — muss grün sein**

Run: `go test ./internal/backend/... -v 2>&1 | tail -20`
Expected: PASS, keine Regression der bestehenden `categorizeWorktree`-Tests (main/terminal/issue-Kategorien unverändert)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree.go internal/backend/app_worktree_test.go
git commit -m "feat(worktree): categorize .claude/worktrees/ paths as native EnterWorktree worktrees"
```

---

### Task 5: Manuelle Aufräum-Primitive für verwaiste Worktrees

**Files:**
- Create: `internal/backend/app_worktree_orphan.go`
- Test: `internal/backend/app_worktree_orphan_test.go`

**Interfaces:**
- Consumes: `mainRepoRoot(dir) (string, error)` (bestehend), `gitCmd` (bestehend)
- Produces (frontend-exponiert, models.ts-Sync in Task 7): `func (a *AppService) RemoveOrphanedWorktree(path string) error`

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_orphan_test.go
package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveOrphanedWorktree_RemovesCleanMergedWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "done-feature")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-done-feature", wt)
	// Merge it into main so branch -d succeeds (mirrors "already integrated" case).
	gitRun(t, repo, "merge", "--ff-only", "worktree-done-feature")

	a := &AppService{}
	if err := a.RemoveOrphanedWorktree(wt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree directory still exists")
	}
	if branchExists(repo, "worktree-done-feature") {
		t.Error("branch still exists after removal of a merged worktree")
	}
}

func TestRemoveOrphanedWorktree_RefusesUnmergedBranch(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "wip-feature")
	gitRun(t, repo, "worktree", "add", "-b", "worktree-wip-feature", wt)
	if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "work.txt")
	gitRun(t, wt, "commit", "-m", "unmerged work")

	a := &AppService{}
	// Directory removal succeeds (no uncommitted files), but the branch delete
	// (-d, never -D) must refuse since it is not merged anywhere — the commit
	// must not silently disappear.
	err := a.RemoveOrphanedWorktree(wt)
	if err == nil {
		t.Fatal("expected error: unmerged branch must not be force-deleted")
	}
	if !branchExists(repo, "worktree-wip-feature") {
		t.Fatal("DATA LOSS: unmerged branch was deleted")
	}
}
```

- [ ] **Step 2: Testlauf — muss fehlschlagen**

Run: `go test ./internal/backend/ -run TestRemoveOrphanedWorktree -v`
Expected: FAIL/compile error (`RemoveOrphanedWorktree` existiert nicht)

- [ ] **Step 3: Implementierung**

```go
// internal/backend/app_worktree_orphan.go
// Manual cleanup for worktrees Claude created via EnterWorktree but never
// removed itself (pane closed, session died, Claude simply moved on). This is
// a deliberately simple admin action, not the full verified finish-flow of
// the old design (spec 2026-07-03 section 6): remove the worktree directory,
// then delete the branch with -d only — an unmerged branch survives so no
// committed work is silently lost.
package backend

import (
	"fmt"
	"strings"
)

// RemoveOrphanedWorktree removes a worktree directory and, if safe, its
// branch. Never uses --force or -D: an unmerged branch or a worktree with
// uncommitted changes causes an error instead of data loss.
func (a *AppService) RemoveOrphanedWorktree(path string) error {
	root, err := mainRepoRoot(path)
	if err != nil {
		return err
	}
	branch := checkedOutBranch(path)

	if out, err := gitCmd(root, "worktree", "remove", path).CombinedOutput(); err != nil {
		return fmt.Errorf("worktree remove fehlgeschlagen: %s – %w", strings.TrimSpace(string(out)), err)
	}
	_ = gitCmd(root, "worktree", "prune").Run()

	if branch == "" {
		return nil
	}
	if out, err := gitCmd(root, "branch", "-d", branch).CombinedOutput(); err != nil {
		return fmt.Errorf("Worktree entfernt, Branch %q aber nicht gelöscht (nicht gemergt): %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}
```

- [ ] **Step 4: Testlauf — muss grün sein**

Run: `go test ./internal/backend/... -v 2>&1 | tail -20` und `go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_orphan.go internal/backend/app_worktree_orphan_test.go
git commit -m "feat(worktree): safe manual cleanup primitive for orphaned worktrees"
```

---

### Task 6: Einmalige Projekt-Einrichtung

**Files:**
- Create: `internal/backend/app_worktree_setup.go`
- Test: `internal/backend/app_worktree_setup_test.go`

**Interfaces:**
- Consumes: `mainRepoRoot(dir) (string, error)` (bestehend)
- Produces (frontend-exponiert, models.ts-Sync in Task 7): `func (a *AppService) EnsureProjectWorktreeSetup(dir string) error`

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_setup_test.go
package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureProjectWorktreeSetup_CreatesFilesOnce(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	memPath := filepath.Join(repo, projectWorktreeMemoryFile)
	mem, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("memory file not created: %v", err)
	}
	for _, want := range []string{"EnterWorktree", "discard_changes", "NIEMALS"} {
		if !strings.Contains(string(mem), want) {
			t.Errorf("memory file missing %q", want)
		}
	}

	settingsPath := filepath.Join(repo, ".claude", "settings.local.json")
	settings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
	if !strings.Contains(string(settings), `"baseRef"`) || !strings.Contains(string(settings), `"head"`) {
		t.Errorf("settings missing worktree.baseRef=head: %s", settings)
	}
}

func TestEnsureProjectWorktreeSetup_DoesNotOverwriteExisting(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}
	memPath := filepath.Join(repo, projectWorktreeMemoryFile)
	custom := []byte("# custom edits by the user\n")
	if err := os.WriteFile(memPath, custom, 0644); err != nil {
		t.Fatal(err)
	}

	if err := a.EnsureProjectWorktreeSetup(repo); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(memPath)
	if string(got) != string(custom) {
		t.Error("second call overwrote the user's edited memory file")
	}
}
```

- [ ] **Step 2: Testlauf — muss fehlschlagen**

Run: `go test ./internal/backend/ -run TestEnsureProjectWorktreeSetup -v`
Expected: FAIL/compile error

- [ ] **Step 3: Implementierung**

```go
// internal/backend/app_worktree_setup.go
// One-time per-project setup that lets Claude Code use its own native
// EnterWorktree tool safely: a memory instruction (EnterWorktree only fires
// when "worktree" is explicitly mentioned in a CLAUDE.md/memory file — see
// tool description) plus a settings.local.json pinning worktree.baseRef to
// the currently checked-out branch (spec 2026-07-03 section 3).
package backend

import (
	"fmt"
	"os"
	"path/filepath"
)

const projectWorktreeMemoryFile = "CLAUDE.local.md"

const projectWorktreeMemoryContent = `# MTUI-Projekt: Worktree-Isolation

Nutze für Aufgaben mit eigenem, klar abgegrenztem Arbeitsbereich das
` + "`EnterWorktree`" + `-Tool, um deine Änderungen zu isolieren. Der Haupt-Branch
dieses Projekts darf dabei nie direkt gewechselt werden.

Wenn eine Aufgabe erledigt ist: committe, pushe bei Bedarf und öffne einen
Pull Request (` + "`gh pr create`" + `), oder frage den Nutzer nach dem gewünschten
Vorgehen.

Nutze ` + "`ExitWorktree`" + ` mit ` + "`discard_changes: true`" + ` oder erzwungenem Entfernen
NIEMALS eigenständig — nur nach ausdrücklicher Rückfrage beim Nutzer und
dessen Bestätigung.
`

const projectWorktreeSettingsContent = `{
  "worktree": {
    "baseRef": "head"
  }
}
`

// EnsureProjectWorktreeSetup writes the memory instruction and settings file
// into a project's root, once. Existing files are left untouched so manual
// edits by the user survive repeated calls (e.g. one per Claude-pane launch).
func (a *AppService) EnsureProjectWorktreeSetup(dir string) error {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return err
	}

	memPath := filepath.Join(root, projectWorktreeMemoryFile)
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		if err := os.WriteFile(memPath, []byte(projectWorktreeMemoryContent), 0644); err != nil {
			return fmt.Errorf("memory file: %w", err)
		}
	}

	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("claude dir: %w", err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		if err := os.WriteFile(settingsPath, []byte(projectWorktreeSettingsContent), 0644); err != nil {
			return fmt.Errorf("settings file: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Testlauf — muss grün sein**

Run: `go test ./internal/backend/... -v 2>&1 | tail -20` und `go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_setup.go internal/backend/app_worktree_setup_test.go
git commit -m "feat(worktree): one-time project setup for native EnterWorktree usage"
```

---

### Task 7: Bindings-Sync (models.ts / App.d.ts / App.js)

**Files:**
- Modify: `frontend/wailsjs/go/backend/App.d.ts`, `frontend/wailsjs/go/backend/App.js`

**Interfaces:**
- Consumes: `RemoveOrphanedWorktree(path string) error` (Task 5), `EnsureProjectWorktreeSetup(dir string) error` (Task 6). Kein neuer Struct-Typ nötig (beide Methoden nehmen/liefern nur Primitives + Error) — `WorktreeInfo` in `models.ts` existiert bereits und trägt schon ein `category`-Feld (aus altem Branch), keine Änderung nötig für Task 4's neue Kategorie „claude" (reiner String-Wert).
- Produces: aufrufbare Bindings `App.RemoveOrphanedWorktree(path)` / `App.EnsureProjectWorktreeSetup(dir)` für Task 9/12.

- [ ] **Step 1: Bindings ergänzen** — in `App.d.ts` nach dem Muster einer bestehenden `(path: string) => Promise<void>`-artigen Methode (z. B. `AbortWorktreeRebase`) zwei Deklarationen ergänzen:

```typescript
export function RemoveOrphanedWorktree(path: string): Promise<void>;
export function EnsureProjectWorktreeSetup(dir: string): Promise<void>;
```

In `App.js`: FNV-1a-32-Hashes über `github.com/patrick-goecommerce/Multiterminal-UI/internal/backend.AppService.RemoveOrphanedWorktree` bzw. `...AppService.EnsureProjectWorktreeSetup` berechnen (Skript aus einer früheren Session: `.superpowers/sdd/verify-hashes.js` — falls noch vorhanden, Methodennamen ergänzen und `node .superpowers/sdd/verify-hashes.js` ausführen; falls nicht vorhanden, nach dem im Global-Constraints-Abschnitt beschriebenen Algorithmus neu schreiben und gegen 3 bekannte Bestands-IDs in `App.js` kalibrieren). Beide Methoden analog zu `AbortWorktreeRebase` als `$Call.ByID(<hash>, path)` ergänzen.

- [ ] **Step 2: Verifikation**

Run: `cd frontend && npm run build`
Expected: Exit 0

- [ ] **Step 3: Commit**

```bash
git add frontend/wailsjs/go/backend/App.d.ts frontend/wailsjs/go/backend/App.js
git commit -m "feat(bindings): sync RemoveOrphanedWorktree + EnsureProjectWorktreeSetup"
```

---

### Task 8: `tabs.ts` — `finishPhase` entfernen, `setWorktree`/`clearWorktree` ergänzen

**Files:**
- Modify: `frontend/src/stores/tabs.ts`
- Test: `frontend/src/stores/tabs.test.ts` (Datei existiert bereits — an bestehende Tests anhängen)

**Interfaces:**
- Consumes: nichts Neues
- Produces: `tabStore.setWorktree(sessionId: number, path: string, branch: string, targetBranch: string): void`, `tabStore.clearWorktree(sessionId: number): void` — von Task 9 genutzt. `Pane.finishPhase` **entfällt** (Feld + alle Referenzen).

- [ ] **Step 1: Vor der Änderung — Referenzen finden**

Run: `grep -rn "finishPhase" frontend/src` — Ergebnis notieren (mindestens `tabs.ts` Interface-Deklaration, Default in `addPane`, `setFinishPhase`-Methode, `PaneTitlebar.svelte`). Jede Fundstelle wird in diesem bzw. Task 10 behandelt — an dieser Stelle nur `tabs.ts` selbst.

- [ ] **Step 2: Failing Test schreiben** (an `frontend/src/stores/tabs.test.ts` anhängen; Import-Stil und Store-Reset-Muster aus den Nachbartests dieser Datei übernehmen)

```typescript
describe('worktree detection state', () => {
  it('setWorktree populates worktreePath/branch/targetBranch on the matching pane', () => {
    const tabId = tabStore.addTab('Test', '/tmp/proj');
    const paneId = tabStore.addPane(tabId, 42, 'pane', 'claude', '');
    tabStore.setWorktree(42, '/tmp/proj/.claude/worktrees/feature-a', 'worktree-feature-a', 'alpha-main');
    const state = tabStore.getState();
    const pane = state.tabs.find((t) => t.id === tabId)!.panes.find((p) => p.id === paneId)!;
    expect(pane.worktreePath).toBe('/tmp/proj/.claude/worktrees/feature-a');
    expect(pane.branch).toBe('worktree-feature-a');
    expect(pane.targetBranch).toBe('alpha-main');
  });

  it('clearWorktree resets worktree fields to empty', () => {
    const tabId = tabStore.addTab('Test2', '/tmp/proj2');
    const paneId = tabStore.addPane(tabId, 43, 'pane', 'claude', '');
    tabStore.setWorktree(43, '/tmp/proj2/.claude/worktrees/x', 'worktree-x', 'main');
    tabStore.clearWorktree(43);
    const state = tabStore.getState();
    const pane = state.tabs.find((t) => t.id === tabId)!.panes.find((p) => p.id === paneId)!;
    expect(pane.worktreePath).toBe('');
    expect(pane.branch).toBe('');
    expect(pane.targetBranch).toBe('');
  });
});
```

- [ ] **Step 3: Testlauf — muss fehlschlagen**

Run: `cd frontend && npx vitest run src/stores/tabs.test.ts`
Expected: FAIL (`setWorktree`/`clearWorktree` existieren nicht)

- [ ] **Step 4: Implementierung**

Im `Pane`-Interface die Zeile `finishPhase: '' | 'preparing' | 'ready' | 'blocked' | 'merging' | 'cleanup';` löschen (Zeile 38 laut aktuellem Stand — Position vor der Änderung mit `grep -n "finishPhase" frontend/src/stores/tabs.ts` verifizieren, da sich Zeilennummern durch vorherige Tasks verschoben haben können).

Im `addPane`-Push-Objekt die Zeile `finishPhase: '',` löschen.

Die bestehende `setFinishPhase`-Methode (Store-Objekt) komplett durch zwei neue Methoden ersetzen:

```typescript
    setWorktree(sessionId: number, path: string, branch: string, targetBranch: string) {
      update((state) => {
        for (const tab of state.tabs) {
          const pane = tab.panes.find((p) => p.sessionId === sessionId);
          if (pane) {
            pane.worktreePath = path;
            pane.branch = branch;
            pane.targetBranch = targetBranch;
          }
        }
        return state;
      });
    },

    clearWorktree(sessionId: number) {
      update((state) => {
        for (const tab of state.tabs) {
          const pane = tab.panes.find((p) => p.sessionId === sessionId);
          if (pane) {
            pane.worktreePath = '';
            pane.branch = '';
            pane.targetBranch = '';
          }
        }
        return state;
      });
    },
```

- [ ] **Step 5: Testlauf — muss grün sein**

Run: `cd frontend && npx vitest run src/stores/tabs.test.ts`
Expected: PASS (neue Tests + keine Regression bestehender Tests in derselben Datei)

- [ ] **Step 6: Commit**

```bash
git add frontend/src/stores/tabs.ts frontend/src/stores/tabs.test.ts
git commit -m "refactor(tabs): replace finishPhase machinery with setWorktree/clearWorktree"
```

---

### Task 9: `App.svelte` — alte Finish-Flow-Verdrahtung entfernen, neue Detection-Events verdrahten

**Files:**
- Modify: `frontend/src/App.svelte`
- Delete: `frontend/src/components/WorktreeFinishDialog.svelte`

**Interfaces:**
- Consumes: `tabStore.setWorktree`/`clearWorktree` (Task 8), Events `worktree:detected`/`worktree:cleared` (Task 3), `App.EnsureProjectWorktreeSetup` (Task 6/7)
- Produces: keine neuen Interfaces — reine Verdrahtung.

- [ ] **Step 1: Import + State entfernen**

Zeile 22 löschen: `import WorktreeFinishDialog from './components/WorktreeFinishDialog.svelte';`

Den `finishDialog`-State-Block (aktuell Zeilen 78–108, exakte Grenzen mit `grep -n "let finishDialog" frontend/src/App.svelte` und der schließenden `};`-Zeile davor bestätigen, da sich Zeilennummern durch vorherige Tasks in dieser Datei verschieben können) vollständig löschen — das ist genau dieser Block:

```typescript
  let finishDialog: {
    visible: boolean;
    sessionId: number;
    state: 'ready' | 'blocked' | 'staging';
    worktreePath: string;
    targetBranch: string;
    commits: string[];
    stat: string;
    untracked: string[];
    cleanupOnly: boolean;
    reason: string;
    files: { path: string; status: string; selected: boolean }[];
    commitMessage: string;
    rebaseConflict: boolean;
    cleanupFailed: boolean;
  } = {
    visible: false,
    sessionId: 0,
    state: 'ready',
    worktreePath: '',
    targetBranch: '',
    commits: [],
    stat: '',
    untracked: [],
    cleanupOnly: false,
    reason: '',
    files: [],
    commitMessage: '',
    rebaseConflict: false,
    cleanupFailed: false,
  };
```

- [ ] **Step 2: Alte Event-Listener durch neue ersetzen**

Den Block der drei `EventsOn('worktree:finish-*', ...)`-Listener (siehe Interfaces-Referenz oben, aktuell zwischen dem Kommentar `// Worktree finish flow: ...` und dem `// Auto-generated pane name`-Kommentar) vollständig ersetzen durch:

```typescript
    // Native EnterWorktree detection: Claude decided on its own to isolate
    // work; MTUI only tracks and displays it (spec 2026-07-03).
    EventsOn('worktree:detected', (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.id)) return;
      tabStore.setWorktree(p.id, p.worktreePath, p.worktreeBranch, p.targetBranch || '');
    });
    EventsOn('worktree:cleared', (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.id)) return;
      tabStore.clearWorktree(p.id);
    });
```

- [ ] **Step 3: Alte Finish-Flow-Funktionen entfernen**

Den kompletten Block von `function startFinish(sessionId: number) {` bis zum Ende von `async function relaunchPaneAfterFinish(...)` (siehe Interfaces-Referenz — umfasst `startFinish`, `stagingDeselect`, `openShellStaging`, `runShellStage`, `handleFinishWorktree`, `handleRetryFinish`, `handleCancelFinish`, `relaunchPaneAfterFinish`) vollständig löschen. `findPaneBySession`/`findPaneLocation` (unmittelbar davor) **bleiben erhalten**, falls sie von anderem Code genutzt werden — vor dem Löschen mit `grep -n "findPaneBySession\|findPaneLocation" frontend/src/App.svelte` prüfen, ob außerhalb des gelöschten Blocks noch Aufrufer existieren; falls nein, ebenfalls löschen.

- [ ] **Step 4: `handleLaunch` — Worktree-Erzeugung entfernen**

Den Block

```typescript
      // Per-pane worktree (opt-in via LaunchDialog checkbox). Chat launches never
      // carry a worktree detail; issue worktrees keep their existing flow above.
      let paneWt: { path: string; branch: string; target_branch: string } | null = null;
      if (e.detail.worktree) {
        try {
          paneWt = await App.CreatePaneWorktree(tab.dir || '', e.detail.worktree.name, e.detail.worktree.targetBranch);
          if (paneWt) sessionDir = paneWt.path;
        } catch (err: any) {
          alert(`Worktree-Erstellung fehlgeschlagen:\n${err?.message || err}`);
```

vollständig löschen (inkl. der zugehörigen schließenden `return;`/`}`/`}`-Zeilen dieses `if`-Blocks — mit `grep -n -A 8 "Per-pane worktree" frontend/src/App.svelte` die exakten Grenzen bestätigen).

Den `tabStore.addPane(...)`-Aufruf, der `paneWt?.path ?? worktreePath, paneWt?.branch ?? paneBranch, paneWt?.target_branch ?? ''` übergibt, auf die Vor-Worktree-Signatur zurücksetzen:

```typescript
        tabStore.addPane(tab.id, sessionId, name, type, model,
          issueCtx?.number, issueCtx?.title, issueBranch, worktreePath, paneBranch,
          '', false, 'terminal', '', claudeSessionId);
```

Das `worktree`-Feld aus dem `CustomEvent<{...}>`-Typ von `handleLaunch` (Funktionssignatur) entfernen.

- [ ] **Step 5: Projekt-Setup beim Claude-Launch anstoßen**

Im `handleLaunch`, direkt vor dem `const sessionId = await App.CreateSession(argv, sessionDir, 24, 80, type);`-Aufruf, ergänzen (nur für Claude-Modi, nicht `shell`):

```typescript
      if (type !== 'shell' && tab.dir) {
        App.EnsureProjectWorktreeSetup(tab.dir).catch((err) => console.error('[EnsureProjectWorktreeSetup]', err));
      }
```

- [ ] **Step 6: Dialog-Einbindung im Markup entfernen**

Den kompletten `<WorktreeFinishDialog ... />`-Block (Zeilen siehe Interfaces-Referenz, zwischen `<AskUserDialog ... />` und dem schließenden `</div>` des Root-Elements) löschen.

- [ ] **Step 7: `WorktreeFinishDialog.svelte` löschen**

```bash
rm frontend/src/components/WorktreeFinishDialog.svelte
```

- [ ] **Step 8: Verifikation**

Run: `grep -n "finishDialog\|WorktreeFinishDialog\|CreatePaneWorktree\|StartWorktreeFinish\|FinishWorktree\|CancelWorktreeFinish\|CheckWorktreeFinish\|AbortWorktreeRebase\|GetWorktreeChangedFiles\|CommitWorktreeFiles\|RebaseWorktreeOntoTarget" frontend/src/App.svelte`
Expected: keine Treffer mehr

Run: `cd frontend && npm run build`
Expected: Exit 0 (Fehler an dieser Stelle bedeuten meist einen noch verbliebenen Aufrufer aus Step 3/4 — mit dem Compiler-Fehler die Fundstelle lokalisieren und entfernen)

Run: `npx vitest run`
Expected: alle bestehenden Tests weiterhin grün

- [ ] **Step 9: Commit**

```bash
git add frontend/src/App.svelte
git rm frontend/src/components/WorktreeFinishDialog.svelte
git commit -m "refactor(app): remove old finish-flow orchestration, wire EnterWorktree detection"
```

---

### Task 10: `PaneTitlebar`/`PaneGrid`/`TerminalPane` — ✓-Button entfernen, Badge bleibt

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte`
- Modify: `frontend/src/components/PaneGrid.svelte`
- Modify: `frontend/src/components/TerminalPane.svelte`

**Interfaces:**
- Consumes: nichts Neues
- Produces: keine — reine Entfernung. Das ⎇-Badge (zeigt `pane.worktreePath`/`pane.branch`) bleibt unverändert bestehen.

- [ ] **Step 1: `PaneTitlebar.svelte` — Button/Spinner-Markup entfernen**

Den Block

```svelte
      {#if pane.finishPhase === 'preparing' || pane.finishPhase === 'merging' || pane.finishPhase === 'cleanup'}
        <button class="pane-btn finish-btn spinning" title="Fertigstellen läuft – klicken zum Abbrechen"
          on:click|stopPropagation={() => dispatch('cancelFinish', { sessionId: pane.sessionId })}>◌</button>
      {:else}
        <button class="pane-btn finish-btn" title="Worktree fertigstellen: mergen & aufräumen"
          on:click|stopPropagation={() => dispatch('finishWorktree', { paneId: pane.id, sessionId: pane.sessionId })}>✓</button>
      {/if}
```

löschen. Das umschließende `{#if pane.worktreePath}`/`{/if}` (Badge) **bleibt erhalten**, nur dieser innere Zweig entfällt.

Die Styles `.finish-btn { color: #4ade80; font-weight: 700; }` und `.finish-btn.spinning { animation: wt-spin 1s linear infinite; }` sowie die zugehörige `@keyframes wt-spin`-Regel löschen.

- [ ] **Step 2: `PaneGrid.svelte` — Passthrough entfernen**

Löschen:

```typescript
  function handleFinishWorktree(e: CustomEvent) {
    dispatch('finishWorktree', e.detail);
  }

  function handleCancelFinish(e: CustomEvent) {
    dispatch('cancelFinish', e.detail);
  }
```

Und im Template die Zeilen `on:finishWorktree={handleFinishWorktree}` sowie `on:cancelFinish={handleCancelFinish}` an der `<TerminalPane ...>`-Einbindung löschen.

- [ ] **Step 3: `TerminalPane.svelte` — Passthrough entfernen**

An der `<PaneTitlebar ...>`-Einbindung die Zeilen `on:finishWorktree` und `on:cancelFinish` löschen.

- [ ] **Step 4: Verifikation**

Run: `grep -rn "finishWorktree\|cancelFinish\|finish-btn" frontend/src/components/`
Expected: keine Treffer mehr

Run: `cd frontend && npm run build && npx vitest run`
Expected: Exit 0 / alle Tests grün

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/PaneTitlebar.svelte frontend/src/components/PaneGrid.svelte frontend/src/components/TerminalPane.svelte
git commit -m "refactor(titlebar): remove finish button, keep worktree badge as pure status"
```

---

### Task 11: `LaunchDialog.svelte` — Worktree-Checkbox entfernen

**Files:**
- Modify: `frontend/src/components/LaunchDialog.svelte`

**Interfaces:**
- Consumes: nichts Neues
- Produces: `launch`-Event-Detail verliert das `worktree`-Feld dauerhaft (Task 9 hat den Konsumenten bereits entfernt — dieser Task entfernt den Producer).

- [ ] **Step 1: State entfernen**

Löschen:

```typescript
  let useWorktree = localStorage.getItem('mtui.worktreeLaunchDefault') === '1';
  let wtName = '';
  let wtTarget = '';
```

sowie die zugehörige `loadWorktreeDefaults`/`toggleWorktree`-Funktionspaar und die reaktive Zeile `$: if (visible && useWorktree) loadWorktreeDefaults();` (mit `grep -n "loadWorktreeDefaults\|toggleWorktree\|wtDefaultsLoaded" frontend/src/components/LaunchDialog.svelte` die vollständigen Funktionskörper lokalisieren und löschen).

- [ ] **Step 2: `launch()`-Funktion bereinigen**

Die Zeilen

```typescript
    const worktree = useWorktree && display !== 'chat' && !issueContext && wtTarget && wtName.trim()
      ? { name: wtName, targetBranch: wtTarget } : null;
    dispatch('launch', { type, model: selectedModel, issue: issueContext, display, permissionMode, worktree });
```

ersetzen durch:

```typescript
    dispatch('launch', { type, model: selectedModel, issue: issueContext, display, permissionMode });
```

- [ ] **Step 3: Markup entfernen**

Den kompletten Block

```svelte
      {#if selectedDisplay !== 'chat' && !issueContext}
        <div class="worktree-opt">
          ...
        </div>
      {/if}
```

(Zeilen 227–ca. 240, exakte Endzeile mit dem schließenden `{/if}` über `grep -n -A 15 "worktree-opt" frontend/src/components/LaunchDialog.svelte` bestätigen) vollständig löschen.

Die Style-Regel `.worktree-opt { margin: 10px 0; }` sowie alle weiteren `.wt-*`-Style-Regeln (`.wt-check`, `.wt-field`, `.wt-hint` — mit `grep -n "\.wt-" frontend/src/components/LaunchDialog.svelte` auflisten) löschen.

- [ ] **Step 4: `dir`-Prop prüfen**

Die Prop `export let dir: string = '';` **bleibt erhalten**, falls sie anderweitig genutzt wird (z. B. für Issue-Kontext) — mit `grep -n "\bdir\b" frontend/src/components/LaunchDialog.svelte` prüfen, ob nach dieser Änderung noch eine Verwendung existiert; falls nicht, Prop und die entsprechende Übergabe in `App.svelte` (`dir={$activeTab?.dir ?? ''}` an der `<LaunchDialog ...>`-Einbindung) ebenfalls entfernen.

- [ ] **Step 5: Verifikation**

Run: `grep -n "useWorktree\|wtName\|wtTarget\|worktree" frontend/src/components/LaunchDialog.svelte`
Expected: keine Treffer

Run: `cd frontend && npm run build && npx vitest run`
Expected: Exit 0 / alle Tests grün

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/LaunchDialog.svelte frontend/src/App.svelte
git commit -m "refactor(launch): remove worktree opt-in checkbox, no longer user-driven"
```

---

### Task 12: `WorktreeDropdown` — Kategorie „verwaist" mit Aufräum-Aktion

**Files:**
- Modify: `frontend/src/components/WorktreeDropdown.svelte`
- Modify: `frontend/src/App.svelte` (Prop-Übergabe: welche Worktrees sind „aktiv")

**Interfaces:**
- Consumes: `App.RemoveOrphanedWorktree(path)` (Task 7), `allWorktrees` (bestehend, jetzt mit `category === "claude"`-Einträgen aus Task 4), `tab.panes[].worktreePath` (bestehend)
- Produces: keine neuen Backend-Interfaces — reine Frontend-Ableitung + Aktion.

- [ ] **Step 1: Orphan-Ableitung in `App.svelte`**

Nach der bestehenden `loadWorktrees()`-Funktion eine reine Ableitungsfunktion ergänzen (Funktion statt `$:`-Inline-Logik zur Vermeidung des Svelte-Reactivity-Footguns bei komplexeren Ausdrücken):

```typescript
  function orphanedClaudeWorktrees(): { path: string; branch: string; name: string }[] {
    const tab = $activeTab;
    if (!tab) return [];
    const activePaths = new Set(tab.panes.map((p) => p.worktreePath).filter(Boolean));
    return allWorktrees
      .filter((w: any) => w.category === 'claude' && !activePaths.has(w.path))
      .map((w: any) => ({ path: w.path, branch: w.branch, name: w.name }));
  }
```

An der `<PaneGrid ...>`-Einbindung eine neue Prop ergänzen: `orphanedWorktrees={orphanedClaudeWorktrees()}` (neu berechnet bei jedem Re-Render der Elternkomponente — ausreichend, da `WorktreeDropdown` nur bei Bedarf geöffnet wird und `allWorktrees` bereits reaktiv über `loadWorktrees()` aktuell gehalten wird).

Neuen Event-Handler ergänzen:

```typescript
  async function handleRemoveOrphanedWorktree(e: CustomEvent<{ path: string }>) {
    try {
      await App.RemoveOrphanedWorktree(e.detail.path);
      await loadWorktrees();
    } catch (err: any) {
      alert(`Aufräumen fehlgeschlagen:\n${err?.message || err}`);
    }
  }
```

An der `<PaneGrid ...>`-Einbindung: `on:removeOrphanedWorktree={handleRemoveOrphanedWorktree}`.

- [ ] **Step 2: `PaneGrid.svelte` — Prop/Event durchreichen**

Neue Prop `export let orphanedWorktrees: { path: string; branch: string; name: string }[] = [];` ergänzen (Muster der bestehenden `worktrees`-Prop). An der `<TerminalPane ...>`-Einbindung: `{orphanedWorktrees}` und `on:removeOrphanedWorktree` durchreichen (identisches Muster wie `worktrees`/`on:worktreeListChanged`, die bereits vorhanden sind — mit `grep -n "worktrees=" frontend/src/components/PaneGrid.svelte` die exakte Stelle finden).

`TerminalPane.svelte` und `PaneTitlebar.svelte`: dieselbe Prop/Event-Durchreichung nach demselben bestehenden Muster (`worktrees`-Prop dient als Vorlage — mit `grep -n "worktrees" frontend/src/components/TerminalPane.svelte frontend/src/components/PaneTitlebar.svelte` die exakten Stellen finden).

- [ ] **Step 3: `WorktreeDropdown.svelte` — neue Sektion**

Neue Prop ergänzen: `export let orphanedWorktrees: { path: string; branch: string; name: string }[] = [];`

Nach dem bestehenden `{#if issueWorktrees.length > 0}`-Block (letzter Kategorie-Block vor `{#if worktrees.length === 0}`) ergänzen:

```svelte
  {#if orphanedWorktrees.length > 0}
    <div class="section-header">Verwaist (Claude EnterWorktree)</div>
    {#each orphanedWorktrees as wt}
      <div class="menu-item orphan-item" title={wt.path}>
        <span class="branch-icon">⎇</span>
        <span class="branch-name">{wt.name}</span>
        <button class="orphan-remove" title="Entfernen" on:click|stopPropagation={() => dispatch('removeOrphanedWorktree', { path: wt.path })}>×</button>
      </div>
    {/each}
  {/if}
```

Style ergänzen (im bestehenden `<style>`-Block, Muster der `.menu-item`-Regeln übernehmen):

```css
  .orphan-item { display: flex; align-items: center; gap: 6px; padding: 6px 12px; }
  .orphan-remove { margin-left: auto; background: none; border: none; color: var(--fg-muted); cursor: pointer; font-size: 14px; padding: 0 4px; }
  .orphan-remove:hover { color: #f87171; }
```

- [ ] **Step 4: Verifikation**

Run: `cd frontend && npm run build && npx vitest run`
Expected: Exit 0 / alle Tests grün

Manuell (sofern ein Dev-Build zur Hand ist): Sandbox-Repo mit einem manuell per `git worktree add -b worktree-orphan <repo>/.claude/worktrees/orphan` angelegten Worktree ohne zugehöriges Pane öffnen → Dropdown zeigt „Verwaist"-Sektion, „×" entfernt ihn.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.svelte frontend/src/components/PaneGrid.svelte frontend/src/components/TerminalPane.svelte frontend/src/components/PaneTitlebar.svelte frontend/src/components/WorktreeDropdown.svelte
git commit -m "feat(worktree): orphaned-worktree section with manual cleanup in dropdown"
```

---

### Task 13: Abschluss-Verifikation + E2E-Checkliste

**Files:** keine neuen — Verifikation + Dokumentation.

- [ ] **Step 1: Volle Test-Suite**

Run: `go test ./internal/... ./cmd/... && go vet ./...`
Expected: alle PASS, keine Vet-Findings

Run: `cd frontend && npm run build && npx vitest run`
Expected: Exit 0 / alle Tests grün

- [ ] **Step 2: 300-Zeilen-Check**

Run (PowerShell): `Get-ChildItem internal/backend/app_worktree_detect.go, internal/backend/app_worktree_orphan.go, internal/backend/app_worktree_setup.go | ForEach-Object { "$($_.Name): $((Get-Content $_ | Measure-Object -Line).Lines)" }`
Expected: alle < 300 Zeilen

- [ ] **Step 3: Vollständigkeits-Grep gegen die alte Finish-Flow-UI**

Run: `grep -rln "WorktreeFinishDialog\|finishWorktree\|cancelFinish\|useWorktree\|finishPhase" frontend/src/`
Expected: keine Treffer (alle Fundstellen aus Tasks 9–11 vollständig entfernt)

- [ ] **Step 4: E2E-Checkliste dokumentieren** (als Kommentar am Tracking-Issue bzw. `needs-e2e-testing`-Label; zu testen mit echtem Claude Code, nicht nur `claude -p`-Simulation wie in der Brainstorming-Phase):

1. Neues Projekt in MTUI öffnen, Claude-Pane starten → `CLAUDE.local.md` + `.claude/settings.local.json` (`worktree.baseRef: head`) entstehen im Projekt-Root.
2. Claude eine Aufgabe geben, die isolierte Arbeit nahelegt → beobachten, ob Claude von sich aus `EnterWorktree` aufruft; ⎇-Badge muss innerhalb der 100ms-Hook-Polling-Latenz erscheinen.
3. Claude fertigstellen lassen (committen, `gh pr create`) → beobachten, ob Claude eigenständig sinnvoll handelt (kein MTUI-Eingriff nötig).
4. Claude bitten, den Worktree zu entfernen, OHNE dass Arbeit gemergt/gepusht wurde → prüfen, ob Claude gemäß Memory-Anweisung beim Nutzer nachfragt, bevor es `discard_changes: true` nutzt (Prompt-Befolgung ist nicht erzwungen — dies ist ein Beobachtungstest, kein Gate).
5. Pane schließen, während ein Worktree mit offener Arbeit existiert → Bestätigungsdialog erscheint, Worktree bleibt liegen.
6. Verwaisten Worktree über die Dropdown-Sektion „Verwaist" manuell entfernen — sowohl den Fall „bereits gemergt" (sollte klaglos entfernt werden) als auch „nicht gemergt" (sollte mit Fehlermeldung verweigert werden) durchspielen.
7. App-Neustart mit einem Pane, das gerade in einem Worktree arbeitet → Badge muss nach Neustart über den nächsten Hook-Event wieder korrekt erscheinen (kein expliziter State-Restore nötig, da rein Hook-getrieben).

- [ ] **Step 5: Commit**

```bash
git add docs/
git commit -m "docs(worktree): e2e checklist for native EnterWorktree detection"
```

## Self-Review-Ergebnis (beim Schreiben geprüft)

- **Spec-Coverage:** Abschnitt 2 (verifizierte Grundlagen) → direkt in Task 1–3 kodiert; Abschnitt 3 (Projekt-Setup) → Task 6; Abschnitt 4 (Hook-Erweiterung) → Tasks 1–3; Abschnitt 5 (Sicherheitsmodell) → Task 6's Memory-Text + Task 13 Punkt 4 als Beobachtungstest (die im Spec-Text explizit benannte Abschwächung wird NICHT durch zusätzlichen Code kompensiert, sondern bewusst so belassen — kein Task nötig, nur Dokumentation); Abschnitt 6 (Ausfall-Netz) → Task 5 + Task 12; Abschnitt 7 (Frontend-Umfang) → Tasks 9–11 (Entfernung) + Task 10 (Badge bleibt); Abschnitt 8 (Verhältnis zum alten Branch) → Global Constraints (keine Löschung der alten Backend-Dateien) + Datei-Landkarte; Abschnitt 9 (offene Implementierungsfragen) → in den jeweiligen Tasks als konkrete Entscheidungen aufgelöst (Deny-Pattern-Frage entfällt, da Abschnitt 5 keine harten Denies mehr vorsieht; Hook-Registrierung erwies sich als bereits vorhanden statt neu nötig; Session-Korrelation verifiziert bestehend); Abschnitt 10 (Shell-Panes out of scope) → kein Task, unverändert belassen.
- **Platzhalter:** keine — jeder Schritt enthält vollständigen Code oder exakte, bereits verifizierte Grep-/Kommando-Anweisungen.
- **Typkonsistenz:** `WorktreeDetectedEvent`/`WorktreeClearedEvent` (Task 3) ↔ Frontend-Handler-Zugriffe `p.id`/`p.worktreePath`/`p.worktreeBranch`/`p.targetBranch` (Task 9) — Feldnamen stimmen überein (Go-JSON-Tags `id`/`worktreePath`/`worktreeBranch`/`targetBranch`). `tabStore.setWorktree(sessionId, path, branch, targetBranch)`-Signatur (Task 8) identisch an der Aufrufstelle in Task 9 verwendet.
