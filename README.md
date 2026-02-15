# Multiterminal UI (mtui)

A native GUI terminal multiplexer built for Claude Code power users. Run multiple Claude Code sessions side-by-side, track token costs, and get visual notifications when Claude needs your attention.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails-v2-red?logo=wails)
![Svelte](https://img.shields.io/badge/Svelte-4-FF3E00?logo=svelte&logoColor=white)

## Features

- **Native desktop app** — Real GUI window with tabs, toolbar, and styled terminal panes (powered by Wails + Svelte)
- **Multi-pane terminals** — Run up to 10 shells and Claude Code sessions per tab in a tiled grid layout
- **Project tabs** — Each tab has its own working directory; add projects via folder picker
- **Token / cost tracking** — Per-pane and total cost displayed automatically for Claude Code sessions
- **Activity detection** — Pane borders glow green (done) or blink red (needs input) so you never miss a prompt
- **File browser sidebar** — Navigate your project and insert file paths directly into the terminal
- **Zoom** — Ctrl+Z to maximise/restore a pane, Ctrl+Mouse Wheel to zoom font size per terminal
- **Custom accent color** — Pick your terminal color via color wheel, hex input, or presets (default: toxic green)
- **Themes** — Five built-in colour themes: dark, light, dracula, nord, solarized
- **Commit reminder** — Footer shows time since last commit with green/yellow/red color coding
- **Session persistence** — Tabs, panes, and layout are saved automatically and restored on restart
- **Clipboard support** — Ctrl+V paste, Ctrl+C copy (when text selected)
- **Pane rename** — Double-click any pane name to rename it
- **Cross-platform** — Windows, Linux, macOS

## Tech Stack

- **Language:** Go 1.21+
- **GUI framework:** [Wails v2](https://wails.io/) (Go backend + WebView2 frontend)
- **Frontend:** [Svelte 4](https://svelte.dev/) + [Vite](https://vitejs.dev/)
- **Terminal emulation:** [xterm.js](https://xtermjs.org/) with FitAddon
- **PTY:** [go-pty](https://github.com/aymanbagabas/go-pty) (cross-platform: Unix PTY / Windows ConPTY)
- **Config:** YAML (`~/.multiterminal.yaml`)

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Build & Run

```bash
# Development mode (hot-reload)
wails dev

# Production build
wails build

# Debug build (with DevTools)
wails build -debug
```

The binary is output to `build/bin/mtui.exe` (Windows) or `build/bin/mtui` (Linux/macOS).

## Keyboard Shortcuts

| Key              | Action                                        |
|------------------|-----------------------------------------------|
| Ctrl+T           | New project tab (opens folder picker)         |
| Ctrl+W           | Close tab                                     |
| Ctrl+N           | New terminal pane (opens launch dialog)       |
| Ctrl+Z           | Maximise / restore focused pane               |
| Ctrl+Scroll      | Zoom in/out (font size per terminal)          |
| Ctrl+V           | Paste from clipboard                          |
| Ctrl+C           | Copy selection to clipboard                   |
| Ctrl+B           | Toggle file browser sidebar                   |
| Esc              | Close dialogs                                 |

## Smart Features

### Token / Cost Tracker

Claude Code panes automatically scan for token usage and cost information.

- **Per-pane cost** is shown in the pane title bar (e.g. `$0.12`)
- **Total cost** across all Claude panes is shown in the global footer

### Activity Detection

Pane borders change when Claude changes state:

- **Green glow** — Claude finished generating (prompt returned)
- **Red blink** — Claude needs user input (confirmation, Y/n, permission, etc.)
- **Pulsing dot** — Claude is actively working

This works across all panes, so you can work in one terminal and see at a glance when another needs attention.

### Commit Reminder

The footer shows how long ago the last git commit was, with color coding:

- **Green** — under 15 minutes
- **Yellow** — 15 to 29 minutes
- **Red (pulsing)** — 30+ minutes

### Custom Terminal Color

Open Settings (gear icon in toolbar) to pick your accent color:

- Native color wheel picker
- Direct hex code input
- 8 preset colors (toxic green, matrix green, cyan, orange, purple, rose, gold, sky blue)
- Live preview — changes apply instantly

### File Browser Sidebar

1. Open the sidebar with **Ctrl+B**
2. Navigate and search files
3. Click a file to insert its path into the focused terminal

## Configuration

A config file is auto-created at `~/.multiterminal.yaml` on first run.

```yaml
theme: dark
terminal_color: "#39ff14"
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

### Available Themes

| Theme       | Description                  |
|-------------|------------------------------|
| `dark`      | Catppuccin Mocha (default)   |
| `light`     | Clean light theme            |
| `dracula`   | Dracula color scheme         |
| `nord`      | Nord color scheme            |
| `solarized` | Solarized Dark               |

## Project Structure

```
main.go                          Wails entry point
wails.json                       Wails project config
frontend/                        Svelte + Vite frontend
  src/
    App.svelte                   Root component
    components/
      TabBar.svelte              Project tabs
      Toolbar.svelte             Action bar (new terminal, files, settings)
      PaneGrid.svelte            Terminal grid layout
      TerminalPane.svelte        xterm.js terminal wrapper
      Sidebar.svelte             File browser
      Footer.svelte              Status bar (branch, cost, commit age)
      LaunchDialog.svelte        Shell/Claude/YOLO picker
      ProjectDialog.svelte       Add project folder dialog
      SettingsDialog.svelte      Color picker settings
    stores/
      tabs.ts                    Tab & pane state management
      config.ts                  App configuration store
      theme.ts                   Theme management & accent color
    lib/
      terminal.ts                xterm.js setup & themes
internal/
  backend/                       Wails-bound Go backend
    app.go                       Main app struct & session management
    app_scan.go                  Activity detection loop
    app_git.go                   Git branch & commit helpers
    app_files.go                 File system API
  terminal/                      PTY session & VT100 emulation
    session.go                   PTY lifecycle (start, read, resize, close)
    activity.go                  Claude activity & token scanning
    screen.go                    VT100 screen buffer
    screen_csi.go                CSI dispatch & SGR handling
    screen_ops.go                Screen operations
    screen_render.go             Screen rendering & plain text
  config/                        Configuration & persistence
    config.go                    YAML config loader
    session.go                   Session state persistence
```

## License

MIT
