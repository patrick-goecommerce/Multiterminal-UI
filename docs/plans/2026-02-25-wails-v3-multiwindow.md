# Wails v3 Migration + Multi-Window Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate Multiterminal from Wails v2 to Wails v3 on branch `wails-v3-alpha`, then implement browser-like multi-window support (tab drag out → new window, right-click context menu, merge-on-close).

**Architecture:** Shared `AppService` singleton (Wails v3 service pattern) manages all PTY sessions globally. Windows are thin views. PTY events broadcast to all windows, each filters by owned session IDs. Tab state is canonical in Go, synchronized via events.

**Tech Stack:** Go 1.24 · Wails v3 alpha (`github.com/wailsapp/wails/v3`) · Svelte 4 · xterm.js · go-pty

**Branch:** `wails-v3-alpha` (already created). All work happens here.

**Design doc:** `docs/plans/2026-02-25-wails-v3-multiwindow-design.md`

---

## Phase 1: Wails v3 Migration

### Task 1: Update go.mod to Wails v3

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Context:** Currently uses `github.com/wailsapp/wails/v2 v2.11.0`. Wails v3 module path is `github.com/wailsapp/wails/v3`.

**Step 1: Remove v2, add v3**

```bash
go get github.com/wailsapp/wails/v3@latest
go mod tidy
```

Expected: `go.mod` now has `github.com/wailsapp/wails/v3` entry, no more `v2`.

**Step 2: Verify go.sum updated**

```bash
grep "wailsapp/wails" go.mod
```

Expected: Only `v3` line present.

**Step 3: Fix all import paths in Go files**

Every file that imports `"github.com/wailsapp/wails/v2/pkg/runtime"` must change to the v3 equivalent. Find all occurrences:

```bash
grep -rn "wailsapp/wails/v2" internal/ main.go
```

Files to update (replace `v2` imports):
- `main.go`
- `internal/backend/app.go`
- `internal/backend/app_stream.go`
- `internal/backend/app_scan.go`
- `internal/backend/app_queue.go`
- `internal/backend/app_notify.go`

In each file, change:
```go
// OLD
"github.com/wailsapp/wails/v2/pkg/runtime"
"github.com/wailsapp/wails/v2"
"github.com/wailsapp/wails/v2/pkg/options"
"github.com/wailsapp/wails/v2/pkg/options/assetserver"
"github.com/wailsapp/wails/v2/pkg/options/windows"
"github.com/wailsapp/wails/v2/pkg/logger"
// NEW
"github.com/wailsapp/wails/v3/pkg/application"
```

Note: In v3, there is no separate `runtime` package — everything goes through `application.Application` or `application.WebviewWindow`.

**Step 4: Verify it compiles (will fail on API mismatches — expected)**

```bash
go build ./... 2>&1 | head -30
```

Expected: Errors about undefined `wails.Run`, `runtime.EventsEmit`, etc. — normal at this stage.

**Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: update go.mod to wails/v3"
```

---

### Task 2: Migrate main.go to Wails v3

**Files:**
- Modify: `main.go` (83 lines)

**Context:** Current `main.go` uses `wails.Run(&options.App{...})`. In v3, it's `application.New(...)` + `app.RegisterService(...)` + `mainWindow := app.Window.New(...)` + `app.Run()`. The `App` struct is not bound directly — it becomes a Service (done in Task 3). Here we just fix `main.go` to compile with v3.

**Step 1: Rewrite main.go**

Replace the entire `main()` function content (keep `signalFocus()` unchanged):

```go
package main

import (
	"embed"
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
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "multiterminal:") {
			signalFocus()
			return
		}
	}

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

	svc := backend.NewAppService(app, cfg)
	app.RegisterService(application.NewService(svc))

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            backend.VersionTitle(),
		Width:            1400,
		Height:           900,
		MinWidth:         800,
		MinHeight:        600,
		URL:              "/?windowId=main",
		BackgroundColour: application.NewRGB(30, 30, 30),
	})
	mainWindow.Center()
	mainWindow.Maximise()
	svc.SetMainWindow(mainWindow)

	if err := app.Run(); err != nil {
		log.Println("Wails error:", err)
	}
	log.Println("Multiterminal UI exited")
}

func signalFocus() {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:41987", 2*time.Second)
	if err != nil {
		return
	}
	conn.Close()
}
```

**Step 2: Verify it compiles (will still fail on AppService missing — expected)**

```bash
go build ./... 2>&1 | grep "main.go"
```

Expected: No errors from main.go itself. Errors from backend package are OK.

**Step 3: Commit**

```bash
git add main.go
git commit -m "chore(wails-v3): migrate main.go to v3 application.New pattern"
```

---

### Task 3: Migrate App struct → AppService

**Files:**
- Modify: `internal/backend/app.go`

**Context:** In v3, the bound struct becomes a "Service". Key changes:
- Remove `ctx context.Context` field (v3 provides ctx via `ServiceStartup`)
- Remove `Startup(ctx)` / `Shutdown(ctx)` methods — replace with `ServiceStartup` / `ServiceShutdown`
- Add `app *application.Application` and `mainWindow *application.WebviewWindow` fields
- Add `NewAppService(app, cfg)` constructor
- `runtime.EventsEmit` calls → `a.app.Event.Emit` (done in Task 4)
- `runtime.OpenDirectoryDialog` → `a.mainWindow.OpenDirectoryDialog` (done in Task 6)

**Step 1: Update the App struct and constructor**

In `internal/backend/app.go`, make these changes:

```go
// Change import:
// REMOVE: "github.com/wailsapp/wails/v2/pkg/runtime"
// ADD:    "github.com/wailsapp/wails/v3/pkg/application"

// Rename App → AppService everywhere in this file
// Change struct:
type AppService struct {
	app            *application.Application
	mainWindow     *application.WebviewWindow
	cfg            config.Config
	health         config.HealthState
	sessions       map[int]*terminal.Session
	queues         map[int]*sessionQueue
	sessionIssues  map[int]*sessionIssue
	mu             sync.Mutex
	nextID         int
	cancelAll      context.CancelFunc
	resolvedClaudePath string
	claudeDetected     bool
}

func NewAppService(app *application.Application, cfg config.Config) *AppService {
	return &AppService{
		app:           app,
		cfg:           cfg,
		sessions:      make(map[int]*terminal.Session),
		queues:        make(map[int]*sessionQueue),
		sessionIssues: make(map[int]*sessionIssue),
	}
}

func (a *AppService) SetMainWindow(w *application.WebviewWindow) {
	a.mainWindow = w
}
```

**Step 2: Replace Startup/Shutdown with v3 Service lifecycle**

```go
// REMOVE: func (a *App) Startup(ctx context.Context) { ... }
// REMOVE: func (a *App) Shutdown(ctx context.Context) { ... }

// ADD:
func (a *AppService) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	a.health = config.LoadHealth()
	config.MarkStarting(&a.health)
	_ = config.SaveHealth(a.health)

	a.resolveClaudeOnStartup()

	scanCtx, cancel := context.WithCancel(ctx)
	a.cancelAll = cancel
	go a.scanLoop(scanCtx)

	a.startFocusListener()
	registerProtocol()
	return nil
}

func (a *AppService) ServiceShutdown() error {
	if a.cancelAll != nil {
		a.cancelAll()
	}
	a.mu.Lock()
	sessions := make([]*terminal.Session, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()

	for _, s := range sessions {
		s.Close()
	}

	config.MarkCleanShutdown(&a.health)
	if config.ShouldAutoDisableLogging(&a.health) {
		config.DisableAutoLogging(&a.health)
		a.cfg.LoggingEnabled = false
		_ = config.Save(a.cfg)
		log.Println("[Shutdown] Auto-logging disabled after 3 clean shutdowns")
	}
	_ = config.SaveHealth(a.health)
	log.Println("[Shutdown] Clean shutdown recorded")
	return nil
}
```

**Step 3: Update SessionInfo — no changes needed (it's a plain struct)**

**Step 4: Fix CreateSession — remove `runtime.EventsEmit` call (replace with `a.app.Event.Emit` — covered in Task 4)**

**Step 5: Replace `runtime.OpenDirectoryDialog` in `SelectDirectory`**

```go
// REMOVE:
func (a *AppService) SelectDirectory(startDir string) string {
	if startDir == "" {
		startDir = a.GetWorkingDir()
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Arbeitsverzeichnis wählen",
		DefaultDirectory: startDir,
	})
	if err != nil {
		return ""
	}
	return dir
}

// ADD:
func (a *AppService) SelectDirectory(startDir string) string {
	if startDir == "" {
		startDir = a.GetWorkingDir()
	}
	result, err := a.mainWindow.OpenDirectoryDialog(application.OpenDialogOptions{
		Title:            "Arbeitsverzeichnis wählen",
		DefaultDirectory: startDir,
	})
	if err != nil || len(result) == 0 {
		return ""
	}
	return result[0]
}
```

**Step 6: Run go vet to catch remaining issues**

```bash
go vet ./internal/backend/... 2>&1 | head -20
```

**Step 7: Commit**

```bash
git add internal/backend/app.go
git commit -m "chore(wails-v3): migrate App struct to AppService with v3 service pattern"
```

---

### Task 4: Migrate EventsEmit calls

**Files:**
- Modify: `internal/backend/app.go` (1 call)
- Modify: `internal/backend/app_stream.go` (3 calls)
- Modify: `internal/backend/app_scan.go` (1 call)
- Modify: `internal/backend/app_queue.go` (1 call)

**Context:** `runtime.EventsEmit(a.ctx, "event-name", arg1, arg2)` → `a.app.Event.Emit("event-name", payload)`.

In v3, the emit payload is a single value. Frontend receives it as `event.data`. We bundle multiple args into a struct.

**Step 1: Define event payload types in a new file `internal/backend/app_events.go`**

```go
package backend

// TerminalOutputEvent is emitted when PTY output is available.
type TerminalOutputEvent struct {
	ID   int    `json:"id"`
	Data string `json:"data"` // base64-encoded
}

// TerminalExitEvent is emitted when a PTY session exits.
type TerminalExitEvent struct {
	ID       int `json:"id"`
	ExitCode int `json:"exitCode"`
}

// TerminalErrorEvent is emitted when a session fails to start.
type TerminalErrorEvent struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}
```

**Step 2: Update app_stream.go**

```go
// REMOVE import: "github.com/wailsapp/wails/v2/pkg/runtime"

// In streamOutput, replace:
runtime.EventsEmit(a.ctx, "terminal:output", id, b64)
// WITH:
a.app.Event.Emit("terminal:output", TerminalOutputEvent{ID: id, Data: b64})

// In watchExit, replace:
runtime.EventsEmit(a.ctx, "terminal:exit", id, sess.ExitCode)
// WITH:
a.app.Event.Emit("terminal:exit", TerminalExitEvent{ID: id, ExitCode: sess.ExitCode})

// In streamOutput, replace ctx.Done() check — keep as-is
// (ctx comes from ServiceStartup, passed via cancelAll)
// BUT: a.ctx no longer exists. Change cancelAll ctx usage:
// The scanCtx is created in ServiceStartup and passed to scanLoop.
// streamOutput uses a.ctx.Done() — replace with a separate done channel per session.
```

Wait — `streamOutput` and `watchExit` use `a.ctx.Done()` to know when to stop. In v3, `a.ctx` is removed. Solution: pass the service-lifecycle context into the goroutines:

```go
// In ServiceStartup, store the ctx:
a.serviceCtx = ctx  // add field: serviceCtx context.Context

// In CreateSession (app.go):
go a.streamOutput(id, sess, a.serviceCtx)
go a.watchExit(id, sess)

// streamOutput signature:
func (a *AppService) streamOutput(id int, sess *terminal.Session, ctx context.Context) {
    // replace a.ctx.Done() with ctx.Done()
}
```

Full updated `app_stream.go`:

```go
package backend

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func (a *AppService) coalesceDelay() time.Duration {
	a.mu.Lock()
	n := len(a.sessions)
	a.mu.Unlock()
	switch {
	case n <= 2:
		return 6 * time.Millisecond
	case n <= 4:
		return 10 * time.Millisecond
	case n <= 6:
		return 14 * time.Millisecond
	default:
		return 18 * time.Millisecond
	}
}

func (a *AppService) streamOutput(id int, sess *terminal.Session, ctx context.Context) {
	for {
		select {
		case data, ok := <-sess.RawOutputCh:
			if !ok {
				return
			}
			buf := append([]byte(nil), data...)
			deadline := time.After(a.coalesceDelay())
		collect:
			for {
				select {
				case more, ok := <-sess.RawOutputCh:
					if !ok {
						b64 := base64.StdEncoding.EncodeToString(buf)
						a.app.Event.Emit("terminal:output", TerminalOutputEvent{ID: id, Data: b64})
						return
					}
					buf = append(buf, more...)
				case <-deadline:
					break collect
				case <-ctx.Done():
					return
				}
			}
			b64 := base64.StdEncoding.EncodeToString(buf)
			a.app.Event.Emit("terminal:output", TerminalOutputEvent{ID: id, Data: b64})
		case <-ctx.Done():
			return
		}
	}
}

func (a *AppService) watchExit(id int, sess *terminal.Session) {
	<-sess.Done()
	a.app.Event.Emit("terminal:exit", TerminalExitEvent{ID: id, ExitCode: sess.ExitCode})
}
```

**Step 3: Update app_scan.go**

Find `runtime.EventsEmit(a.ctx, "terminal:activity", ActivityInfo{...})` and replace:

```go
a.app.Event.Emit("terminal:activity", activityInfo)
```

Remove `"github.com/wailsapp/wails/v2/pkg/runtime"` import. ActivityInfo struct stays unchanged.

**Step 4: Update app_queue.go**

Find `runtime.EventsEmit(a.ctx, "queue:update", sessionId)` and replace:

```go
a.app.Event.Emit("queue:update", sessionId)
```

**Step 5: Update app.go (terminal:error)**

Find `runtime.EventsEmit(a.ctx, "terminal:error", id, errMsg)` and replace:

```go
a.app.Event.Emit("terminal:error", TerminalErrorEvent{ID: id, Message: errMsg})
```

Also update `go a.streamOutput(id, sess)` → `go a.streamOutput(id, sess, a.serviceCtx)` in `CreateSession`.

**Step 6: Add `serviceCtx` field to AppService struct in app.go**

```go
type AppService struct {
    // ...existing fields...
    serviceCtx context.Context  // ADD THIS
}

// In ServiceStartup, add:
a.serviceCtx = ctx
```

**Step 7: Build check**

```bash
go build ./internal/backend/... 2>&1 | head -30
```

Expected: Errors only from app_notify.go (window calls) — handled in Task 5.

**Step 8: Commit**

```bash
git add internal/backend/app.go internal/backend/app_stream.go internal/backend/app_scan.go \
        internal/backend/app_queue.go internal/backend/app_events.go
git commit -m "chore(wails-v3): migrate EventsEmit to app.Event.Emit"
```

---

### Task 5: Migrate Window runtime calls (app_notify.go)

**Files:**
- Modify: `internal/backend/app_notify.go`

**Context:** `app_notify.go` uses `runtime.WindowIsMinimised`, `runtime.WindowUnminimise`, `runtime.WindowShow`, `runtime.WindowSetAlwaysOnTop`. In v3, these are methods on `*application.WebviewWindow`.

**Step 1: Update app_notify.go**

```go
package backend

import (
	"log"
	"net"

	"github.com/go-toast/toast"
)

const focusAddr = "127.0.0.1:41987"

func (a *AppService) SendNotification(title string, body string) {
	n := toast.Notification{
		AppID:               "Multiterminal",
		Title:               title,
		Message:             body,
		ActivationType:      "protocol",
		ActivationArguments: "multiterminal:focus",
	}
	if err := n.Push(); err != nil {
		log.Printf("[SendNotification] failed: %v", err)
	}
}

func (a *AppService) startFocusListener() {
	ln, err := net.Listen("tcp", focusAddr)
	if err != nil {
		log.Printf("[focusListener] could not listen on %s: %v", focusAddr, err)
		return
	}
	go func() {
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
			if a.mainWindow.IsMinimised() {
				a.mainWindow.UnMinimise()
			}
			a.mainWindow.Show()
			a.mainWindow.SetAlwaysOnTop(true)
			a.mainWindow.SetAlwaysOnTop(false)
		}
	}()
}
```

Note: Verify exact v3 method names (`IsMinimised`, `UnMinimise`, `Show`, `SetAlwaysOnTop`) against v3 docs:
```bash
# Check v3 WebviewWindow methods:
grep -r "func.*WebviewWindow" $(go env GOPATH)/pkg/mod/github.com/wailsapp/wails/v3*/pkg/application/*.go 2>/dev/null | grep -i "minim\|show\|ontop" | head -10
```

**Step 2: Build check — backend should now compile**

```bash
go build ./internal/backend/... 2>&1 | head -20
```

Expected: Clean or only minor issues. Fix any remaining `App` → `AppService` renames in other files.

**Step 3: Run tests**

```bash
go test ./internal/backend/... -v 2>&1 | tail -20
```

**Step 4: Commit**

```bash
git add internal/backend/app_notify.go
git commit -m "chore(wails-v3): migrate window runtime calls to WebviewWindow methods"
```

---

### Task 6: Fix remaining App→AppService renames across all backend files

**Files:**
- All `internal/backend/app_*.go` files that reference `*App` receiver

**Context:** All method receivers `(a *App)` must become `(a *AppService)`. Some files (app_git.go, app_files.go, app_issues.go, etc.) were not touched yet.

**Step 1: Find all remaining `*App` receivers**

```bash
grep -rn "func (a \*App)" internal/backend/ | grep -v "_test.go"
```

**Step 2: Rename all occurrences**

In each listed file, replace `(a *App)` with `(a *AppService)`.

Also check for any type assertions or struct literals:
```bash
grep -rn "\bApp{" internal/backend/ | grep -v "_test.go"
grep -rn "NewApp(" internal/backend/ | grep -v "_test.go"
```

Replace `NewApp(` → `NewAppService(` where found.

**Step 3: Update test files that reference `App` type**

```bash
grep -rn "\*App\b\|NewApp(" internal/backend/ --include="*_test.go"
```

Update test files to use `AppService` / `NewAppService`.

**Step 4: Full build**

```bash
go build ./... 2>&1
```

Expected: Clean build (main.go + all backend). Frontend not built yet.

**Step 5: Run all tests**

```bash
go test ./... 2>&1
```

**Step 6: Commit**

```bash
git add internal/backend/
git commit -m "chore(wails-v3): rename App→AppService across all backend files"
```

---

### Task 7: Regenerate frontend bindings + update frontend event handling

**Files:**
- Modify: `frontend/src/` (event handlers in App.svelte, TerminalPane.svelte)
- Auto-generated: `frontend/wailsjs/` (do not edit manually)

**Context:** Wails v3 generates new bindings. The service is now `AppService` instead of `App`. Event payloads changed structure (bundled into structs instead of variadic args).

**Step 1: Generate v3 bindings**

```bash
wails3 generate bindings -d ./internal/backend -o ./frontend/wailsjs/go/backend
```

Or via build:
```bash
wails build 2>&1 | head -30
```

The generated `wailsjs/go/backend/AppService.js` and `AppService.d.ts` replace the old `App.js`.

**Step 2: Update all frontend imports**

Find all files importing from `backend/App`:
```bash
grep -rn "backend/App" frontend/src/
```

Replace in each file:
```ts
// OLD
import * as App from '../../wailsjs/go/backend/App';
// NEW
import * as App from '../../wailsjs/go/backend/AppService';
```

**Step 3: Update event handler payload structure in frontend**

Event payloads changed from variadic to struct. Find all `Events.On` usages:

```bash
grep -rn "Events.On\|EventsOn" frontend/src/
```

In `TerminalPane.svelte` (or wherever terminal events are handled):

```ts
// OLD (v2 - variadic args)
Events.On('terminal:output', (id: number, data: string) => { ... })
Events.On('terminal:exit',   (id: number, exitCode: number) => { ... })
Events.On('terminal:activity', (info: ActivityInfo) => { ... })

// NEW (v3 - single event object with .data)
Events.On('terminal:output', (event: any) => {
    const { id, data } = event.data;
    // ...
})
Events.On('terminal:exit', (event: any) => {
    const { id, exitCode } = event.data;
    // ...
})
Events.On('terminal:activity', (event: any) => {
    const info: ActivityInfo = event.data;
    // ...
})
```

Also update `terminal:error` and `queue:update` events similarly.

**Step 4: Build frontend**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

Expected: Clean build.

**Step 5: Full wails build**

```bash
wails build -debug 2>&1 | tail -20
```

Expected: `build/bin/mtui-portable.exe` produced.

**Step 6: Smoke test**
- Launch the binary
- Open a terminal pane, verify PTY works
- Launch Claude session, verify output streams
- Verify settings dialog opens

**Step 7: Commit**

```bash
git add frontend/wailsjs/ frontend/src/
git commit -m "chore(wails-v3): regenerate bindings, update frontend imports and event payloads"
```

---

## Phase 2: Multi-Window Backend

### Task 8: WindowManager — app_window.go

**Files:**
- Create: `internal/backend/app_window.go`
- Modify: `internal/backend/app.go` (add `windowManager` field)

**Context:** Manages the lifecycle of secondary windows. Main window is always `"main"`. Each secondary window gets a UUID. The manager tracks which tabIDs each window owns.

**Step 1: Write tests first — `internal/backend/app_window_test.go`**

```go
package backend

import (
	"testing"
)

func TestWindowManagerRegisterUnregister(t *testing.T) {
	wm := newWindowManager(nil)

	wm.register("win1", nil, []string{"tab1", "tab2"})
	if len(wm.windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(wm.windows))
	}

	wm.unregister("win1")
	if len(wm.windows) != 0 {
		t.Fatalf("expected 0 windows after unregister")
	}
}

func TestWindowManagerGetTabWindow(t *testing.T) {
	wm := newWindowManager(nil)
	wm.register("win1", nil, []string{"tab-a", "tab-b"})

	winID := wm.getWindowForTab("tab-a")
	if winID != "win1" {
		t.Errorf("expected win1, got %q", winID)
	}
	winID = wm.getWindowForTab("unknown")
	if winID != "" {
		t.Errorf("expected empty for unknown tab, got %q", winID)
	}
}

func TestWindowManagerMoveTab(t *testing.T) {
	wm := newWindowManager(nil)
	wm.register("win1", nil, []string{"tab-a", "tab-b"})
	wm.register("win2", nil, []string{"tab-c"})

	wm.moveTab("tab-a", "win2")

	if wm.getWindowForTab("tab-a") != "win2" {
		t.Error("tab-a should now belong to win2")
	}
	// win1 should no longer list tab-a
	entry := wm.windows["win1"]
	for _, id := range entry.TabIDs {
		if id == "tab-a" {
			t.Error("win1 should not contain tab-a anymore")
		}
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/backend/... -run TestWindowManager -v
```

Expected: FAIL — `newWindowManager` undefined.

**Step 3: Implement `app_window.go`**

```go
package backend

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// windowEntry tracks one open Wails window and the tab IDs it currently owns.
type windowEntry struct {
	Window *application.WebviewWindow
	TabIDs []string
}

// windowManager tracks all open windows.
type windowManager struct {
	mu      sync.Mutex
	windows map[string]*windowEntry
	app     *application.Application
}

func newWindowManager(app *application.Application) *windowManager {
	return &windowManager{
		windows: make(map[string]*windowEntry),
		app:     app,
	}
}

func (wm *windowManager) register(id string, win *application.WebviewWindow, tabIDs []string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.windows[id] = &windowEntry{Window: win, TabIDs: tabIDs}
}

func (wm *windowManager) unregister(id string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	delete(wm.windows, id)
}

func (wm *windowManager) getWindowForTab(tabID string) string {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for id, entry := range wm.windows {
		for _, t := range entry.TabIDs {
			if t == tabID {
				return id
			}
		}
	}
	return ""
}

func (wm *windowManager) moveTab(tabID, targetWindowID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	// Remove from current window
	for _, entry := range wm.windows {
		for i, t := range entry.TabIDs {
			if t == tabID {
				entry.TabIDs = append(entry.TabIDs[:i], entry.TabIDs[i+1:]...)
				break
			}
		}
	}
	// Add to target
	if target, ok := wm.windows[targetWindowID]; ok {
		target.TabIDs = append(target.TabIDs, tabID)
	}
}

// WindowInfo is returned to the frontend.
type WindowInfo struct {
	ID     string   `json:"id"`
	TabIDs []string `json:"tabIds"`
}

// --- Methods exposed to frontend ---

// DetachTab creates a new window for the given tab and returns the new windowID.
func (a *AppService) DetachTab(tabID string, sourceWindowID string) (string, error) {
	newID := fmt.Sprintf("win-%s", uuid.New().String()[:8])
	url := fmt.Sprintf("/?windowId=%s&tabs=%s", newID, tabID)

	// Get source window position for offset
	var x, y int
	a.winMgr.mu.Lock()
	if src, ok := a.winMgr.windows[sourceWindowID]; ok && src.Window != nil {
		x, y = src.Window.Position()
	}
	a.winMgr.mu.Unlock()

	win := a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Multiterminal",
		Width:  1200,
		Height: 800,
		X:      x + 30,
		Y:      y + 30,
		URL:    url,
	})

	a.winMgr.register(newID, win, []string{tabID})

	win.OnDestroy(func() {
		log.Printf("[WindowManager] window %s destroyed", newID)
		a.winMgr.unregister(newID)
	})

	win.Show()
	log.Printf("[DetachTab] created window %s for tab %s", newID, tabID)
	return newID, nil
}

// MergeWindowToMain is called by a secondary window before it closes.
// tabState is the serialized tab state from the frontend.
func (a *AppService) MergeWindowToMain(windowID string, tabState string) {
	log.Printf("[MergeWindowToMain] merging window %s to main", windowID)
	a.app.Event.Emit("window:tabs-merged", map[string]interface{}{
		"fromWindowId": windowID,
		"tabState":     tabState,
	})
	a.winMgr.unregister(windowID)
}

// GetOpenWindows returns info about all open windows.
func (a *AppService) GetOpenWindows() []WindowInfo {
	a.winMgr.mu.Lock()
	defer a.winMgr.mu.Unlock()
	result := make([]WindowInfo, 0, len(a.winMgr.windows))
	for id, entry := range a.winMgr.windows {
		result = append(result, WindowInfo{ID: id, TabIDs: entry.TabIDs})
	}
	return result
}
```

Note: `github.com/google/uuid` must be added: `go get github.com/google/uuid`

**Step 4: Add `winMgr` field to AppService + register main window**

In `app.go`, add to `AppService` struct:
```go
winMgr *windowManager
```

In `NewAppService`:
```go
svc := &AppService{
    // ...existing...
    winMgr: newWindowManager(app),
}
```

In `SetMainWindow`:
```go
func (a *AppService) SetMainWindow(w *application.WebviewWindow) {
    a.mainWindow = w
    a.winMgr.register("main", w, nil) // main window owns all tabs initially
}
```

**Step 5: Run tests**

```bash
go test ./internal/backend/... -run TestWindowManager -v
```

Expected: PASS.

**Step 6: Build check**

```bash
go build ./... 2>&1
```

**Step 7: Commit**

```bash
git add internal/backend/app_window.go internal/backend/app_window_test.go internal/backend/app.go go.mod go.sum
git commit -m "feat(wails-v3): add WindowManager with DetachTab and MergeWindowToMain"
```

---

## Phase 3: Frontend Multi-Window

### Task 9: lib/window.ts — window identity helpers

**Files:**
- Create: `frontend/src/lib/window.ts`

**Step 1: Write the file**

```ts
// frontend/src/lib/window.ts
// Helpers for multi-window identity. Each Wails window loads the same
// frontend assets but with different URL query params.

/** Returns the windowId for this window instance (e.g. "main", "win-abc123"). */
export function getWindowId(): string {
  const params = new URLSearchParams(window.location.search);
  return params.get('windowId') ?? 'main';
}

/** Returns true if this is the main application window. */
export function isMainWindow(): boolean {
  return getWindowId() === 'main';
}

/** Returns the initial tab IDs this window should display (empty = main window, loads all). */
export function getInitialTabs(): string[] {
  const params = new URLSearchParams(window.location.search);
  const tabs = params.get('tabs');
  return tabs ? tabs.split(',').filter(Boolean) : [];
}
```

**Step 2: Build check**

```bash
cd frontend && npm run build 2>&1 | grep -i "window.ts\|error" | head -10
```

**Step 3: Commit**

```bash
git add frontend/src/lib/window.ts
git commit -m "feat(wails-v3): add window identity helpers (lib/window.ts)"
```

---

### Task 10: App.svelte — multi-window initialization

**Files:**
- Modify: `frontend/src/App.svelte`

**Context:** On mount, App.svelte must check if it's a secondary window (has `?tabs=` param) and initialize only those tabs. Secondary windows skip normal session restore. Main window works as before.

**Step 1: Import window helpers and update onMount**

In the `<script lang="ts">` section of `App.svelte`, add imports:
```ts
import { getWindowId, isMainWindow, getInitialTabs } from './lib/window';
```

In `onMount`, at the top before existing logic:
```ts
const windowId = getWindowId();
const initialTabs = getInitialTabs();
const _isMain = isMainWindow();

if (!_isMain && initialTabs.length > 0) {
    // Secondary window: only load the assigned tabs
    // Tabs are already in the store if they survived the window creation
    // (the detach flow will populate them before opening the window)
    // Just skip the normal session restore:
    return; // exit onMount early for secondary window
}
// ... existing main window logic continues ...
```

**Step 2: Listen for window:tabs-merged event (main window only)**

In `onMount` of `App.svelte` (main window path):

```ts
if (_isMain) {
    Events.On('window:tabs-merged', (event: any) => {
        const { tabState } = event.data;
        // Parse tabState JSON and add tabs to the store
        try {
            const incoming = JSON.parse(tabState);
            if (incoming?.tabs) {
                for (const tab of incoming.tabs) {
                    tabStore.addTabFromState(tab);
                }
            }
        } catch (e) {
            console.error('Failed to parse merged tab state', e);
        }
    });
}
```

Note: `tabStore.addTabFromState` may not exist yet — add it to `stores/tabs.ts` as:
```ts
addTabFromState(tab: Tab) {
    tabs.update(state => {
        state.tabs.push(tab);
        return state;
    });
}
```

**Step 3: Build check**

```bash
cd frontend && npm run build 2>&1 | grep -i "error\|warn" | head -20
```

**Step 4: Commit**

```bash
git add frontend/src/App.svelte frontend/src/stores/tabs.ts
git commit -m "feat(wails-v3): multi-window init in App.svelte + tabs-merged handler"
```

---

### Task 11: TabBar.svelte — drag & drop + right-click context menu

**Files:**
- Modify: `frontend/src/components/TabBar.svelte`

**Context:** Current `TabBar.svelte` has simple click/double-click handlers. We add:
1. `draggable="true"` + `dragend` to detect drag-out-of-window
2. Right-click context menu with "In neuem Fenster öffnen"

**Step 1: Update TabBar.svelte**

In the `<script>` section, add:

```ts
import * as App from '../../wailsjs/go/backend/AppService';
import { getWindowId, isMainWindow } from '../lib/window';

const _isMain = isMainWindow();
const _windowId = getWindowId();

let contextMenu: { tabId: string; x: number; y: number } | null = null;

function handleDragStart(e: DragEvent, tabId: string) {
    e.dataTransfer?.setData('tabId', tabId);
}

async function handleDragEnd(e: DragEvent, tabId: string) {
    // Check if dropped outside the window bounds
    const outside = e.clientX < 0 || e.clientX > window.innerWidth
                 || e.clientY < 0 || e.clientY > window.innerHeight;
    if (outside) {
        await detachTab(tabId);
    }
}

async function detachTab(tabId: string) {
    try {
        await App.DetachTab(tabId, _windowId);
        tabStore.closeTab(tabId); // remove from this window's store
    } catch (err) {
        console.error('DetachTab failed', err);
    }
}

function handleContextMenu(e: MouseEvent, tabId: string) {
    if (!_isMain) return; // no "new window" option in secondary windows
    e.preventDefault();
    contextMenu = { tabId, x: e.clientX, y: e.clientY };
}

function closeContextMenu() {
    contextMenu = null;
}
```

In the template, update the tab element:

```svelte
{#each $allTabs as tab (tab.id)}
  <button
    class="tab"
    class:active={tab.id === activeTabId}
    draggable="true"
    on:click={() => handleTabClick(tab.id)}
    on:dblclick={() => handleTabDblClick(tab.id)}
    on:dragstart={(e) => handleDragStart(e, tab.id)}
    on:dragend={(e) => handleDragEnd(e, tab.id)}
    on:contextmenu={(e) => handleContextMenu(e, tab.id)}
  >
    <!-- existing content unchanged -->
  </button>
{/each}

<!-- Context menu -->
{#if contextMenu}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="ctx-overlay" on:click={closeContextMenu}>
    <div class="ctx-menu" style="left:{contextMenu.x}px; top:{contextMenu.y}px"
         on:click|stopPropagation>
      <button class="ctx-item" on:click={() => { detachTab(contextMenu.tabId); closeContextMenu(); }}>
        In neuem Fenster öffnen
      </button>
      <div class="ctx-separator"></div>
      <button class="ctx-item ctx-item-danger"
              on:click={() => { tabStore.closeTab(contextMenu.tabId); closeContextMenu(); }}>
        Tab schließen
      </button>
    </div>
  </div>
{/if}
```

Add styles:

```css
.ctx-overlay {
    position: fixed; inset: 0; z-index: 1000;
}
.ctx-menu {
    position: fixed;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 4px;
    min-width: 180px;
    box-shadow: 0 4px 16px rgba(0,0,0,0.4);
    z-index: 1001;
}
.ctx-item {
    display: block; width: 100%;
    padding: 7px 12px; text-align: left;
    background: none; border: none;
    color: var(--fg); font-size: 13px;
    border-radius: 4px; cursor: pointer;
}
.ctx-item:hover { background: var(--bg-tertiary); }
.ctx-item-danger { color: #f87171; }
.ctx-separator { height: 1px; background: var(--border); margin: 4px 0; }
```

**Step 2: Build check**

```bash
cd frontend && npm run build 2>&1 | grep -i "error" | head -10
```

**Step 3: Commit**

```bash
git add frontend/src/components/TabBar.svelte
git commit -m "feat(wails-v3): tab drag-out and right-click context menu for new window"
```

---

### Task 12: Merge-on-close for secondary windows

**Files:**
- Modify: `frontend/src/App.svelte`

**Context:** When a secondary window is closed, it must serialize its tab state and call `App.MergeWindowToMain` so the main window receives the tabs back.

**Step 1: Add beforeunload handler in App.svelte (secondary windows only)**

In `onMount` (inside the `if (!_isMain)` block from Task 10):

```ts
if (!_isMain) {
    window.addEventListener('beforeunload', () => {
        const state = JSON.stringify({ tabs: get(allTabs) });
        // beforeunload must be synchronous — use sendBeacon or sync call
        App.MergeWindowToMain(_windowId, state);
    });
}
```

Note: Wails WebView2 `beforeunload` fires unreliably (see CLAUDE.md). Use the Wails window `OnBeforeClose` hook instead. In `app_window.go`, when creating secondary windows:

```go
win.OnBeforeClose(func() bool {
    // Emit event to frontend to trigger merge
    a.app.Event.Emit("window:before-close", map[string]string{"windowId": newID})
    return true // allow close
})
```

In the secondary window's frontend, listen for this event:

```ts
// In App.svelte, inside !_isMain block:
Events.On('window:before-close', async (event: any) => {
    if (event.data.windowId !== _windowId) return;
    const state = JSON.stringify({ tabs: get(allTabs) });
    await App.MergeWindowToMain(_windowId, state);
});
```

**Step 2: Add highlight animation for merged tabs in main window**

In `frontend/src/stores/tabs.ts`, in `addTabFromState`:

```ts
addTabFromState(tab: Tab) {
    tabs.update(state => ({
        ...state,
        tabs: [...state.tabs, { ...tab, highlight: true }],
        activeTabId: tab.id,
    }));
    // Remove highlight after 2s
    setTimeout(() => {
        tabs.update(state => ({
            ...state,
            tabs: state.tabs.map(t => t.id === tab.id ? { ...t, highlight: false } : t),
        }));
    }, 2000);
}
```

In `TabBar.svelte`, add `class:highlight={tab.highlight}` to the tab element and CSS:

```css
.tab.highlight {
    animation: tab-arrive 0.4s ease-out;
}
@keyframes tab-arrive {
    from { background: var(--accent); color: var(--bg); }
    to   { background: transparent; }
}
```

**Step 3: Build check**

```bash
cd frontend && npm run build 2>&1 | grep -i "error" | head -10
```

**Step 4: Full wails build + smoke test**

```bash
wails build -debug
```

Test:
1. Launch app
2. Right-click tab → "In neuem Fenster öffnen" → second window opens with that tab
3. Drag tab outside window → second window opens
4. Close second window → tab appears back in main window with highlight
5. Close main window → no errors

**Step 5: Commit**

```bash
git add frontend/src/App.svelte frontend/src/stores/tabs.ts frontend/src/components/TabBar.svelte \
        internal/backend/app_window.go
git commit -m "feat(wails-v3): merge-on-close for secondary windows with tab highlight"
```

---

## Post-Implementation

**Update CLAUDE.md** — add new files to Project Structure section:

```
internal/backend/
    app_window.go    Window manager, DetachTab, MergeWindowToMain
    app_events.go    Event payload types (TerminalOutputEvent, etc.)
frontend/src/lib/
    window.ts        Window identity helpers (getWindowId, isMainWindow)
```

**Update GitHub issue #89** — comment with "Implemented on wails-v3-alpha branch".

**Cherry-pick any v2 bugfixes** from `main` that landed after branch creation:
```bash
git log main..HEAD --oneline  # see what's diverged
git cherry-pick <sha>          # apply v2 fixes
```

**Push branch:**
```bash
git push -u origin wails-v3-alpha
```
