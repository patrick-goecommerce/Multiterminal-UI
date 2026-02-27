# CLI Session Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add CLI flags (`--list-tabs`, `--remove-tab`, `--clean`, `--safe-mode`, `--help`) to mtui for session management without starting the GUI.

**Architecture:** Standard `flag` package in `main.go` dispatches CLI-only commands before GUI init and exits. `--safe-mode` passes a boolean into `AppService`, which skips loading/saving sessions and restores the original session file on shutdown.

**Tech Stack:** Go stdlib `flag` package, existing `config.SessionState` / `config.SaveSession` / `config.LoadSession`.

---

### Task 1: Add `RemoveTab` to `internal/config/session.go`

**Files:**
- Modify: `internal/config/session.go`
- Test: `internal/config/session_test.go` (create new file — tests for `RemoveTab`)

**Step 1: Write the failing test**

Create `internal/config/session_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveTab_RemovesMatchingTab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	state := SessionState{
		ActiveTab: 0,
		Tabs: []SavedTab{
			{Name: "Alpha", Dir: "/a"},
			{Name: "Beta", Dir: "/b"},
			{Name: "Gamma", Dir: "/c"},
		},
	}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}

	found, err := removeTabFrom(path, "Beta")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected tab to be found")
	}

	result := loadSessionFrom(path)
	if result == nil {
		t.Fatal("expected session to exist after removal")
	}
	if len(result.Tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(result.Tabs))
	}
	for _, tab := range result.Tabs {
		if tab.Name == "Beta" {
			t.Fatal("Beta should have been removed")
		}
	}
}

func TestRemoveTab_ReturnsFalseWhenNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	state := SessionState{
		Tabs: []SavedTab{{Name: "Alpha", Dir: "/a"}},
	}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}

	found, err := removeTabFrom(path, "DoesNotExist")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected tab not to be found")
	}
}

func TestRemoveTab_NoSessionFile(t *testing.T) {
	found, err := removeTabFrom(filepath.Join(t.TempDir(), "no-such-file.json"), "X")
	if err != nil {
		t.Fatal("expected no error when file missing")
	}
	if found {
		t.Fatal("expected false when file missing")
	}
}

func TestRemoveTab_ActiveTabClamped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	state := SessionState{
		ActiveTab: 2, // points to Gamma (index 2)
		Tabs: []SavedTab{
			{Name: "Alpha", Dir: "/a"},
			{Name: "Beta", Dir: "/b"},
			{Name: "Gamma", Dir: "/c"},
		},
	}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}

	// Remove Gamma — ActiveTab=2 would be out of bounds
	found, err := removeTabFrom(path, "Gamma")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected tab to be found")
	}

	result := loadSessionFrom(path)
	if result.ActiveTab >= len(result.Tabs) {
		t.Fatalf("ActiveTab %d out of bounds for %d tabs", result.ActiveTab, len(result.Tabs))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestRemoveTab -v
```

Expected: compile error — `removeTabFrom`, `saveSessionTo`, `loadSessionFrom` not defined.

**Step 3: Add internal helpers and public `RemoveTab` to `internal/config/session.go`**

Add at the end of `internal/config/session.go`:

```go
// RemoveTab removes the first tab matching name from the session file.
// Returns (true, nil) if found and removed, (false, nil) if not found,
// or (false, err) on read/write failure.
func RemoveTab(name string) (bool, error) {
	return removeTabFrom(sessionPath(), name)
}

// removeTabFrom is the testable core, operating on an explicit path.
func removeTabFrom(path string, name string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return false, err
	}

	idx := -1
	for i, t := range state.Tabs {
		if t.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, nil
	}

	state.Tabs = append(state.Tabs[:idx], state.Tabs[idx+1:]...)
	if state.ActiveTab >= len(state.Tabs) && len(state.Tabs) > 0 {
		state.ActiveTab = len(state.Tabs) - 1
	}

	return true, saveSessionTo(path, state)
}

// saveSessionTo is the testable core for SaveSession.
func saveSessionTo(path string, state SessionState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// loadSessionFrom is the testable core for LoadSession.
func loadSessionFrom(path string) *SessionState {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	if len(state.Tabs) == 0 {
		return nil
	}
	return &state
}
```

Also refactor `SaveSession` and `LoadSession` to delegate to the new helpers:

```go
func SaveSession(state SessionState) error {
	p := sessionPath()
	if p == "" {
		return nil
	}
	return saveSessionTo(p, state)
}

func LoadSession() *SessionState {
	p := sessionPath()
	if p == "" {
		return nil
	}
	return loadSessionFrom(p)
}
```

Remove the duplicate implementation bodies from the original `SaveSession` / `LoadSession` (json.MarshalIndent and os.ReadFile code) since the helpers now own that logic.

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/config/session.go internal/config/session_test.go
git commit -m "feat(config): add RemoveTab with testable helpers"
```

---

### Task 2: Add `safeMode` support to `AppService`

**Files:**
- Modify: `internal/backend/app.go:28-57` (struct + constructor)
- Modify: `internal/backend/app.go:91-117` (ServiceShutdown)
- Modify: `internal/backend/app.go:244-266` (SaveTabs + LoadTabs)

No new tests needed here — safe-mode behaviour is integration-level and tested via manual verification. The existing `LoadTabs`/`SaveTabs` call paths are already tested elsewhere.

**Step 1: Add fields to `AppService` struct (lines 28-45)**

Add two fields at the end of the struct:

```go
safeMode      bool
sessionBackup *config.SessionState // populated in safe-mode; restored on shutdown
```

**Step 2: Update `NewAppService` signature and body (line 48)**

Change:
```go
func NewAppService(app *application.App, cfg config.Config) *AppService {
	return &AppService{
		app:           app,
		cfg:           cfg,
		sessions:      make(map[int]*terminal.Session),
		queues:        make(map[int]*sessionQueue),
		sessionIssues: make(map[int]*sessionIssue),
		winMgr:        newWindowManager(app),
	}
}
```

To:
```go
func NewAppService(app *application.App, cfg config.Config, safeMode bool) *AppService {
	svc := &AppService{
		app:           app,
		cfg:           cfg,
		sessions:      make(map[int]*terminal.Session),
		queues:        make(map[int]*sessionQueue),
		sessionIssues: make(map[int]*sessionIssue),
		winMgr:        newWindowManager(app),
		safeMode:      safeMode,
	}
	if safeMode {
		svc.sessionBackup = config.LoadSession() // may be nil — that's fine
		log.Println("[SafeMode] active: sessions will not be loaded or saved")
	}
	return svc
}
```

**Step 3: Guard `SaveTabs` (line 246)**

Change:
```go
func (a *AppService) SaveTabs(state config.SessionState) {
	log.Printf("[SaveTabs] saving %d tabs", len(state.Tabs))
	if err := config.SaveSession(state); err != nil {
		log.Printf("[SaveTabs] error: %v", err)
	}
}
```

To:
```go
func (a *AppService) SaveTabs(state config.SessionState) {
	if a.safeMode {
		log.Printf("[SaveTabs] skipped (safe-mode)")
		return
	}
	log.Printf("[SaveTabs] saving %d tabs", len(state.Tabs))
	if err := config.SaveSession(state); err != nil {
		log.Printf("[SaveTabs] error: %v", err)
	}
}
```

**Step 4: Guard `LoadTabs` (line 254)**

Change:
```go
func (a *AppService) LoadTabs() *config.SessionState {
	if !a.cfg.ShouldRestoreSession() {
		log.Printf("[LoadTabs] restore_session disabled")
		return nil
	}
	state := config.LoadSession()
	...
```

To:
```go
func (a *AppService) LoadTabs() *config.SessionState {
	if a.safeMode {
		log.Printf("[LoadTabs] skipped (safe-mode)")
		return nil
	}
	if !a.cfg.ShouldRestoreSession() {
		log.Printf("[LoadTabs] restore_session disabled")
		return nil
	}
	state := config.LoadSession()
	...
```

**Step 5: Restore backup in `ServiceShutdown` (line 91)**

Add at the end of `ServiceShutdown`, before the final `return nil`:

```go
// In safe-mode, restore the original session file so the next normal
// start resumes where the user left off before this safe-mode run.
if a.safeMode {
	if a.sessionBackup != nil {
		if err := config.SaveSession(*a.sessionBackup); err != nil {
			log.Printf("[SafeMode] failed to restore session backup: %v", err)
		} else {
			log.Printf("[SafeMode] session backup restored (%d tabs)", len(a.sessionBackup.Tabs))
		}
	} else {
		// No session existed before — remove whatever the frontend may have written
		config.ClearSession()
		log.Printf("[SafeMode] no backup to restore; session cleared")
	}
}
```

**Step 6: Fix the call site in `main.go` (next task handles this)**

(No build yet — `main.go` still passes the old 2-arg form.)

**Step 7: Commit**

```bash
git add internal/backend/app.go
git commit -m "feat(backend): add safe-mode flag to AppService"
```

---

### Task 3: Add CLI flag parsing to `main.go`

**Files:**
- Modify: `main.go`

No tests — CLI dispatch is verified manually.

**Step 1: Replace `main.go` with the updated version**

Full updated `main.go`:

```go
// Multiterminal UI (mtui) – A GUI terminal multiplexer optimised for Claude Code.
//
// Stack: Go · Wails · Svelte · xterm.js · go-pty
package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/backend"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// If launched via multiterminal: protocol (notification click),
	// signal the running instance to focus and exit immediately.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "multiterminal:") {
			signalFocus()
			return
		}
	}

	// --- CLI flags ---
	var (
		flagListTabs  = flag.Bool("list-tabs", false, "List saved tab names and exit")
		flagRemoveTab = flag.String("remove-tab", "", "Remove a tab by name and exit")
		flagClean     = flag.Bool("clean", false, "Delete the session file and exit")
		flagSafeMode  = flag.Bool("safe-mode", false, "Start without loading sessions; restore session on close")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mtui [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// CLI-only commands — execute and exit without starting the GUI.
	if *flagListTabs {
		runListTabs()
		return
	}
	if *flagRemoveTab != "" {
		runRemoveTab(*flagRemoveTab)
		return
	}
	if *flagClean {
		runClean()
		return
	}

	// --- GUI start ---
	log.Println("Starting Multiterminal UI...")

	cfg := config.Load()
	log.Println("Config loaded, theme:", cfg.Theme)

	backend.InitLoggingFromConfig(cfg)

	app := application.New(application.Options{
		Name: "Multiterminal",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	svc := backend.NewAppService(app, cfg, *flagSafeMode)
	log.Println("AppService created, starting Wails...")

	app.RegisterService(application.NewService(svc))

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            backend.VersionTitle(),
		Width:            1400,
		Height:           900,
		MinWidth:         800,
		MinHeight:        600,
		URL:              "/?windowId=main",
		BackgroundColour: application.NewRGBA(30, 30, 30, 255),
	})
	mainWindow.Center()
	mainWindow.Maximise()

	svc.SetMainWindow(mainWindow)

	if err := app.Run(); err != nil {
		log.Println("Wails error:", err)
	}
	log.Println("Multiterminal UI exited")
}

// runListTabs prints tab names from the saved session, one per line.
func runListTabs() {
	state := config.LoadSession()
	if state == nil {
		fmt.Fprintln(os.Stderr, "No session file found.")
		return
	}
	for _, tab := range state.Tabs {
		fmt.Println(tab.Name)
	}
}

// runRemoveTab removes a single tab by name from the session file.
func runRemoveTab(name string) {
	found, err := config.RemoveTab(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Tab %q not found in session.\n", name)
		os.Exit(1)
	}
	fmt.Printf("Tab %q removed.\n", name)
}

// runClean deletes the session file entirely.
func runClean() {
	config.ClearSession()
	fmt.Println("Session file cleared.")
}

// signalFocus connects to the running instance's focus listener
// to bring the window to the foreground.
func signalFocus() {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:41987", 2*time.Second)
	if err != nil {
		return
	}
	conn.Close()
}
```

**Step 2: Build to verify it compiles**

```bash
cd /d/repos/Multiterminal && go build ./...
```

Expected: no errors.

**Step 3: Smoke-test CLI flags manually**

```bash
# list tabs (with a real session file):
./build/bin/mtui-portable.exe --list-tabs

# clean:
./build/bin/mtui-portable.exe --clean

# help:
./build/bin/mtui-portable.exe --help
```

Expected: each command prints output and exits without opening a window.

**Step 4: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add main.go
git commit -m "feat: add CLI flags --list-tabs, --remove-tab, --clean, --safe-mode"
```

---

### Task 4: Verify safe-mode end-to-end (manual)

No code changes. This is a manual verification checklist.

**Before starting:**
- Make sure `~/.multiterminal-session.json` exists with at least one tab (just run mtui normally and close it).

**Test 1 — safe-mode starts clean:**
1. Run `mtui --safe-mode`
2. Verify: no tabs are opened on startup
3. Open a couple of new panes/tabs manually
4. Close the window

**Test 2 — session restored after safe-mode:**
1. Open `~/.multiterminal-session.json` in a text editor
2. Verify it matches the state **before** the safe-mode run (not the panes opened during safe-mode)

**Test 3 — normal start after safe-mode:**
1. Run `mtui` (no flags)
2. Verify: original tabs are restored as expected

**Step 1: Commit verification notes (optional)**

If any bugs are found, fix them and commit. If all good:

```bash
git log --oneline -5
```

All three feature commits should be visible.

---

## Summary of Changed Files

| File | What changed |
|------|-------------|
| `internal/config/session.go` | Added `RemoveTab`, `removeTabFrom`, `saveSessionTo`, `loadSessionFrom` |
| `internal/config/session_test.go` | New — unit tests for `RemoveTab` |
| `internal/backend/app.go` | `safeMode`/`sessionBackup` fields, `NewAppService` sig, guards in `SaveTabs`/`LoadTabs`, restore in `ServiceShutdown` |
| `main.go` | `flag` parsing, CLI dispatch functions `runListTabs`, `runRemoveTab`, `runClean` |
