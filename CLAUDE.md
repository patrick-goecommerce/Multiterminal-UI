# Multiterminal

A GUI terminal multiplexer built for Claude Code power users.

## Code Rules
- **Max 300 lines per Go file.** Split into logically grouped files (e.g. `app_scan.go`, `app_stream.go`).
- **Go structs** exposed to frontend need both `yaml` and `json` tags.
- **UI text is German**, code/comments are English.

## Platform Gotchas (Windows)
- CLI tools (`claude`, `npm`) are `.cmd` shims — must wrap via `os.Getenv("COMSPEC")` + `/c` for ConPTY.
  Never use bare `cmd.exe` (Go resolves relative to exe dir).
- `CLAUDECODE` env var must be stripped from PTY environment (see `session.go:Start`).
- `beforeunload` does NOT fire reliably in WebView2 — use reactive auto-save (store subscription + debounce).

## Concurrency
- **`Screen.mu`** — cells, cursor, parser. All public Screen methods lock internally.
  Use batch methods (`PlainTextRows`) over loops (`PlainTextRow`×N) to minimize lock contention.
- **`Session.mu`** — Status, Title, LastOutputAt, Activity, Tokens, PTY handle.
- **`App.mu`** — sessions map, queues map, nextID.
- **Never allocate under lock** — use pre-allocated templates (e.g. `blankLine` in scroll ops).

## Tech Stack
- **Language:** Go 1.21+ (backend) + TypeScript/Svelte (frontend)
- **GUI framework:** Wails v2 (Go ↔ WebView bridge)
- **Frontend:** Svelte 4 + Vite + xterm.js
- **Terminal emulation:** xterm.js (frontend) + VT100 screen buffer for activity scanning (backend)
- **PTY management:** go-pty (cross-platform: Unix PTY + Windows ConPTY)
- **Config:** YAML (~/.multiterminal.yaml)

## Project Structure
```
internal/
  backend/
    app.go                       Wails App struct, session lifecycle, bindings
    app_stream.go                PTY output streaming + adaptive coalescing
    app_scan.go                  Periodic activity detection & token scanning
    app_queue.go                 Pipeline queue (prompt batching per session)
    app_files.go                 Filesystem API (list dir, search files)
    app_git.go                   Git helpers (branch, commit, conflict)
    app_git_branch.go            Branch detection & switching
    app_issues.go                GitHub issue integration
    app_issues_parse.go          Issue body parsing
    app_session_issue.go         Session ↔ issue linking
    app_issue_progress.go        Issue progress reporting
    app_worktree.go              Git worktree management
    app_claude_detect.go         Claude CLI path resolution
    app_notify.go                Desktop notifications
    app_health.go                Crash detection & health tracking
    app_audio.go                 Audio notification playback
    app_version.go               Version info
  terminal/
    session.go                   PTY session lifecycle (start, read, close)
    session_helpers.go           Default shell, PTY console helpers
    activity.go                  Claude activity detection & token scanning
    screen.go                    VT100 screen buffer core
    screen_parser.go             ANSI escape sequence byte processor
    screen_csi.go                CSI dispatch, SGR handling, color parsing
    screen_ops.go                Screen operations (scroll, erase, insert, delete)
    screen_render.go             Screen rendering (Render, RenderRegion, PlainText)
  config/
    config.go                    YAML configuration loader
    session.go                   Session state persistence (JSON)
frontend/src/
  App.svelte                     Root application component
  main.ts                        Entry point
  stores/
    tabs.ts                      Tab & pane state management
    config.ts                    App configuration store
    theme.ts                     Theme management (5 built-in themes)
  components/
    TerminalPane.svelte          xterm.js terminal wrapper with titlebar
    PaneGrid.svelte              Grid layout for terminal panes
    TabBar.svelte                Tab bar with add/close/rename
    Toolbar.svelte               Action toolbar
    Sidebar.svelte               File browser with search & git status
    Footer.svelte                Status bar (branch, cost, shortcuts)
    LaunchDialog.svelte          Shell/Claude/YOLO launch dialog
    QueuePanel.svelte            Pipeline queue panel
    SettingsDialog.svelte        Settings UI
    CommandPalette.svelte        Command palette (Ctrl+Shift+P)
    IssueDialog.svelte           GitHub issue picker
    SourceControlView.svelte     Git source control panel
  lib/
    terminal.ts                  xterm.js setup, theme config & search addon
    clipboard.ts                 Clipboard integration (copy/paste)
    shortcuts.ts                 Global keyboard shortcut handler
    session.ts                   Session restore logic
    launch.ts                    Session launch helpers (shell/claude/yolo)
    notifications.ts             Desktop notification wrapper
    audio.ts                     Audio playback (done/input sounds)
    git-polling.ts               Git status polling
```

## Build & Run
```bash
wails dev              # Development (hot-reload)
wails build            # Production build
wails build -debug     # Debug build (with devtools)
# Binary: build/bin/mtui-portable.exe (Windows)
```

## Testing
```bash
go test ./internal/terminal/...   # Screen buffer, activity, session
go test ./internal/config/...     # Config, session persistence
go test ./internal/backend/...    # Scan, queue, git, issues
go vet ./...                      # Static analysis
```

## Architecture

```
┌─ Wails Window (Native OS Window) ────────────────────────┐
│  ┌─ Svelte Frontend ──────────────────────────────────┐  │
│  │  Tab Bar → Toolbar → Pane Grid (xterm.js) → Footer │  │
│  └────────────────────────────────────────────────────┘  │
│                    ↕ Wails Bindings + Events              │
│  ┌─ Go Backend ───────────────────────────────────────┐  │
│  │  PTY Sessions · Activity Scanner · Config · Git    │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**Data flow:**
- Keyboard input → xterm.js `onData` → Wails binding `WriteToSession` → PTY
- PTY output → Go `RawOutputCh` (blocking, 256-buf) → `streamOutput` (adaptive coalesce) → Wails event `terminal:output` → xterm.js `write`
- Activity/tokens → Go `scanLoop` (adaptive interval) → Wails event `terminal:activity` → UI update

## Key Shortcuts
| Key              | Action                                        |
|------------------|-----------------------------------------------|
| Ctrl+T           | New tab                                       |
| Ctrl+W           | Close tab                                     |
| Ctrl+N           | New pane (opens launch dialog)                |
| Ctrl+Z           | Zoom (maximise / restore) focused pane        |
| Ctrl+B           | Toggle file browser sidebar                   |
| Ctrl+F           | Search in terminal output (per pane)          |
| Ctrl+1-9         | Focus pane by index (1 = first pane)          |

## Smart Features

### Token / Cost Tracker
Claude Code panes automatically scan for token usage and cost information.
- **Per-pane cost** is shown in the pane title bar (e.g. `$0.12`)
- **Total cost** across all Claude panes is shown in the global footer

### Auto-detect Claude Activity
The pane border **flashes** when Claude changes state:
- **Green glow** — Claude finished generating (prompt returned)
- **Yellow pulse** — Claude needs user input (confirmation, Y/n, etc.)

### Themes
Five built-in colour themes. Set `theme` in `~/.multiterminal.yaml`:
- `dark` (default) — Catppuccin Mocha inspired
- `light` — Clean light theme
- `dracula` — Dracula color scheme
- `nord` — Nord color scheme
- `solarized` — Solarized Dark

## Configuration
See `~/.multiterminal.yaml` for defaults (auto-created on first run).

```yaml
theme: dracula
default_dir: /path/to/project
max_panes_per_tab: 12
sidebar_width: 30
claude_command: claude
commit_reminder_minutes: 30
claude_models:
  - label: Default
    id: ""
  - label: Opus 4.6
    id: claude-opus-4-6
  - label: Sonnet 4.5
    id: claude-sonnet-4-5-20250929
  - label: Haiku 4.5
    id: claude-haiku-4-5-20251001
```
