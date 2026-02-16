# Multiterminal

A GUI terminal multiplexer built for Claude Code power users.

## Code Rules
- **Max 300 lines per Go file.** If a file grows beyond 300 lines, split it into
  logically grouped smaller files within the same package. Keep each file focused
  on a single responsibility.

## Tech Stack
- **Language:** Go 1.21+ (backend) + TypeScript/Svelte (frontend)
- **GUI framework:** Wails v2 (Go ↔ WebView bridge)
- **Frontend:** Svelte 4 + Vite + xterm.js
- **Terminal emulation:** xterm.js (frontend) + VT100 screen buffer for activity scanning (backend)
- **PTY management:** go-pty (cross-platform: Unix PTY + Windows ConPTY)
- **Config:** YAML (~/.multiterminal.yaml)

## Project Structure
```
main.go                          Wails entry point
wails.json                       Wails project configuration
app.go                           (reserved for future use)
internal/
  backend/
    app.go                       Main Wails App struct, session management, bindings
    app_scan.go                  Periodic activity detection & token scanning
    app_queue.go                 Pipeline queue (prompt batching per session)
    app_files.go                 Filesystem API (list dir, search files)
    app_git.go                   Git branch & commit info helpers
  terminal/
    session.go                   PTY session lifecycle (start, read, close)
    activity.go                  Claude activity detection & token scanning
    screen.go                    VT100 screen buffer core + ANSI parser
    screen_csi.go                CSI dispatch, SGR handling, color parsing
    screen_ops.go                Screen operations (scroll, erase, insert, delete)
    screen_render.go             Screen rendering (Render, RenderRegion, PlainText)
  config/
    config.go                    YAML configuration loader
    session.go                   Session state persistence (JSON)
frontend/
  src/
    App.svelte                   Root application component
    main.ts                      Entry point
    stores/
      tabs.ts                    Tab & pane state management
      config.ts                  App configuration store
      theme.ts                   Theme management (5 built-in themes)
    components/
      TabBar.svelte              Tab bar with add/close/rename
      Toolbar.svelte             Action toolbar (new terminal, files, etc.)
      PaneGrid.svelte            Grid layout for terminal panes
      TerminalPane.svelte        xterm.js terminal wrapper with titlebar
      QueuePanel.svelte          Pipeline queue panel (per-pane prompt batching)
      Sidebar.svelte             File browser with search & git status
      Footer.svelte              Status bar (branch, cost, shortcuts)
      LaunchDialog.svelte        Shell/Claude/YOLO launch dialog
    lib/
      terminal.ts                xterm.js setup, theme config & search addon
```

## Build & Run
```bash
# Development (hot-reload)
wails dev

# Production build
wails build

# The binary is placed in build/bin/multiterminal.exe (Windows)
# or build/bin/multiterminal (Linux/macOS)
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
- PTY output → Go `RawOutputCh` → Wails event `terminal:output` → xterm.js `write`
- Activity/tokens → Go scan loop → Wails event `terminal:activity` → UI update

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
