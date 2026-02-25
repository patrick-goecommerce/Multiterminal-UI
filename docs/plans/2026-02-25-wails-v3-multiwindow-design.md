# Wails v3 Migration + Multi-Window Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate Multiterminal from Wails v2 to Wails v3 on a dedicated `wails-v3-alpha` branch, and implement browser-like multi-window support (tab drag & drop, right-click "In neuem Fenster öffnen", merge-on-close).

**Architecture:** Shared `AppService` singleton (Wails v3 service pattern) manages all PTY sessions globally. Windows are thin views — PTY events broadcast to all windows, each filters its own sessions. Tab state canonical in Go, synchronized via events.

**Tech Stack:** Go 1.24+ · Wails v3 (alpha) · Svelte 4 · xterm.js · go-pty

**Branch:** `wails-v3-alpha` — long-lived alpha branch. Bug fixes go to `main` first, then cherry-picked here. When v3 stable + feature complete, this becomes the new `main`.

---

## Branch Strategy

```
main             ──── stable (Wails v2, v1.x.x releases)
                          ↑ cherry-pick bugfixes
wails-v3-alpha   ──── alpha (Wails v3, v2.0.0-alpha.X releases)
```

- Create `wails-v3-alpha` from current `main`
- GitHub Actions: separate workflow builds alpha tags from this branch
- CLAUDE.md on this branch documents the alpha-specific setup

---

## Section 1: Wails v3 Migration

### go.mod Changes
```
github.com/wailsapp/wails/v2 v2.11.0  →  github.com/wailsapp/wails/v3 (latest alpha)
```

### main.go Restructure
```go
// v2 (old)
wails.Run(&options.App{
    OnStartup:  app.Startup,
    OnShutdown: app.Shutdown,
    Bind:       []interface{}{app},
})

// v3 (new)
app := application.New(application.Options{
    Name: "Multiterminal",
    Assets: application.AssetOptions{
        Handler: application.AssetFileServerFS(assets),
    },
})
svc := backend.NewAppService(app, cfg)
app.RegisterService(application.NewService(svc))
mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
    Title:  backend.VersionTitle(),
    Width:  1400,
    Height: 900,
    URL:    "/?windowId=main",
})
app.Run()
```

### AppService Migration
| v2 | v3 |
|---|---|
| `App struct { ctx context.Context }` | `AppService struct { app *application.Application }` |
| `func (a *App) Startup(ctx context.Context)` | `func (s *AppService) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error` |
| `runtime.EventsEmit(a.ctx, name, data)` | `s.app.Event.Emit(name, data)` |
| `runtime.EventsOn(ctx, name, fn)` | `s.app.Event.On(name, fn)` |
| `runtime.OpenDirectoryDialog(a.ctx, opts)` | `s.app.Dialog.OpenDirectory(opts)` |
| `runtime.WindowSetTitle(a.ctx, t)` | `window.SetTitle(t)` |

All ~30 `runtime.Events*` calls in `app_stream.go`, `app_scan.go`, `app_notify.go` etc. updated.

### Frontend Bindings
Wails v3 regenerates `wailsjs/` from service methods. Import paths change:
```ts
// v2
import * as App from '../../wailsjs/go/backend/App';
// v3
import * as AppService from '../../wailsjs/go/backend/AppService';
```

---

## Section 2: Multi-Window Architecture (Go)

### New file: `internal/backend/app_window.go`

```go
type WindowEntry struct {
    Window *application.WebviewWindow
    TabIDs []string
}

// WindowManager embedded in AppService
type WindowManager struct {
    mu      sync.Mutex
    windows map[string]*WindowEntry  // windowID → entry
    app     *application.Application
}
```

### Exposed Methods (frontend-callable)
```go
// Returns the windowID for the calling window (read from URL query param)
func (s *AppService) GetWindowID() string

// Creates new window hosting tabID, returns new windowID
func (s *AppService) DetachTab(tabID string, sourceWindowID string) (string, error)

// Moves tab from one window to another (drag between existing windows)
func (s *AppService) MoveTabToWindow(tabID string, targetWindowID string)

// Called by secondary window before close — main window receives tabs
func (s *AppService) MergeWindowToMain(windowID string, tabState []TabStateJSON)

// Returns all open window IDs (for future window picker)
func (s *AppService) GetOpenWindows() []WindowInfo
```

### PTY Event Broadcasting
No routing change needed — all events already use session ID as discriminator:
```
terminal:output   { id: sessionID, data: ... }
terminal:activity { id: sessionID, ... }
```
Every window's frontend subscribes only to sessions it owns. Works automatically.

### Window Lifecycle
1. `DetachTab(tabID)` → creates `WebviewWindow` with URL `/?windowId=<uuid>&tabs=<tabID>`
2. New window offset: +30px x/y from source window position
3. Window title: `Multiterminal — <Tab-Name>`
4. Secondary window `BeforeClose` → frontend calls `MergeWindowToMain` → main receives `window:tabs-merged` event
5. App shutdown → `AppService.Shutdown()` iterates all secondary windows → calls close on each → merge happens automatically

---

## Section 3: Frontend (Svelte)

### Window Identity
```ts
// frontend/src/lib/window.ts (new)
export function getWindowId(): string {
    const params = new URLSearchParams(window.location.search);
    return params.get('windowId') ?? 'main';
}

export function isMainWindow(): boolean {
    return getWindowId() === 'main';
}

export function getInitialTabs(): string[] {
    const params = new URLSearchParams(window.location.search);
    const tabs = params.get('tabs');
    return tabs ? tabs.split(',') : [];
}
```

### Tab Drag & Drop (`TabBar.svelte`)
```svelte
<!-- Tab element -->
<div
  class="tab"
  draggable="true"
  on:dragstart={handleDragStart}
  on:dragend={handleDragEnd}
>

<script>
function handleDragEnd(e: DragEvent) {
    // Detect drop outside window bounds
    const outside = e.clientX < 0 || e.clientX > window.innerWidth
                 || e.clientY < 0 || e.clientY > window.innerHeight;
    if (outside) {
        dispatch('detachTab', { tabId: tab.id });
    }
}
</script>
```

### Right-Click Context Menu (Tab)
```
Tab context menu:
├── Umbenennen
├── ── separator ──
├── In neuem Fenster öffnen   ← new (hidden on secondary windows)
└── Tab schließen
```

### Merge-on-Close
```ts
// App.svelte (secondary window only)
if (!isMainWindow()) {
    window.addEventListener('beforeunload', () => {
        AppService.MergeWindowToMain(getWindowId(), serializeTabState());
    });
}
```

Main window receives `window:tabs-merged` event → appends tabs to tab bar with brief highlight animation.

### Initialization
```ts
// App.svelte onMount
const windowId = getWindowId();
const initialTabs = getInitialTabs();

if (initialTabs.length > 0) {
    // Secondary window: load only assigned tabs
    loadTabsFromIDs(initialTabs);
} else {
    // Main window: normal session restore
    const saved = await AppService.LoadTabs();
    if (saved) restoreSession(saved);
}
```

---

## Non-Goals (YAGNI)
- No nested windows (secondary windows have no "Neues Fenster" menu item)
- No drag between existing open windows (Phase 2 if needed)
- No per-window settings/config
- No window persistence across app restarts (tabs merge to main on close anyway)
