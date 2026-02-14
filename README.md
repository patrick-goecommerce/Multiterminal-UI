# Multiterminal

A TUI terminal multiplexer built for Claude Code power users. Run multiple Claude Code sessions side-by-side, track token costs, and get visual notifications when Claude needs your attention.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)

## Features

- **Multi-pane terminals** — Run multiple shells and Claude Code sessions in a tabbed, tiled layout
- **Token / cost tracking** — Per-pane and total cost displayed automatically for Claude Code sessions
- **Activity detection** — Pane borders flash green (done) or yellow (needs input) so you never miss a prompt
- **File browser sidebar** — Navigate your project and insert file paths directly into the terminal
- **Zoom mode** — Maximise any pane with Ctrl+Z, restore with the same shortcut
- **Themes** — Five built-in colour themes: dark, light, dracula, nord, solarized
- **Commit reminder** — Configurable nudge when you haven't committed in a while
- **Session persistence** — Save and restore sessions across restarts
- **Cross-platform** — Linux, macOS, and Windows

## Tech Stack

- **Language:** Go 1.21+
- **TUI framework:** [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles)
- **Terminal emulation:** Custom VT100 screen buffer + go-pty
- **Config:** YAML (`~/.multiterminal.yaml`)

## Build & Run

```bash
# Linux / macOS
go build -o multiterminal .
./multiterminal

# Windows
go build -o multiterminal.exe .
.\multiterminal.exe

# Cross-compile for Windows from Linux/macOS
GOOS=windows GOARCH=amd64 go build -o multiterminal.exe .
```

## Keyboard Shortcuts

| Key              | Action                                        |
|------------------|-----------------------------------------------|
| Ctrl+T           | New tab                                       |
| Ctrl+W           | Close tab                                     |
| 1-9              | Switch tab                                    |
| Ctrl+N           | New pane (opens launch dialog)                |
| Ctrl+X           | Close focused pane                            |
| Ctrl+Z           | Zoom (maximise / restore) focused pane        |
| Ctrl+Scroll Up   | Zoom in (maximise)                            |
| Ctrl+Scroll Down | Zoom out (restore grid)                       |
| Arrow keys       | Navigate panes                                |
| Tab              | Cycle focus to next pane                      |
| Ctrl+G           | Passthrough mode (all keys to terminal)       |
| Alt+Enter        | Send Shift+Enter (newline in Claude Code)     |
| Ctrl+B           | Toggle file browser sidebar                   |
| Ctrl+F           | Focus/unfocus sidebar for navigation          |
| Enter (sidebar)  | Dir: expand/collapse · File: insert path      |
| / (sidebar)      | Start file search                             |
| Ctrl+S           | Set working directory for tab                 |
| ?                | Show keyboard shortcuts help                  |
| Ctrl+C (x2)     | Quit                                          |

## Smart Features

### Token / Cost Tracker

Claude Code panes automatically scan for token usage and cost information.

- **Per-pane cost** is shown in the pane title bar (e.g. `$0.12`)
- **Total cost** across all Claude panes is shown in the global footer

### Auto-detect Claude Activity

The pane border flashes when Claude changes state:

- **Green flash** (3s) — Claude finished generating (prompt returned)
- **Yellow flash** (5s) — Claude needs user input (confirmation, Y/n, etc.)

This works even when the pane is not focused, so you can work in another terminal and see at a glance when Claude needs attention.

### File Browser Sidebar

1. Open the sidebar with **Ctrl+B**
2. Focus it with **Ctrl+F** (title shows "Files [ACTIVE]")
3. Navigate with arrow keys, search with `/`
4. Press **Enter** on a file to insert its full path into the focused terminal
5. Works with images too — Claude Code reads images by path

### Shift+Enter Support

For Claude Code panes, the kitty keyboard protocol is auto-enabled so that Shift+Enter works natively for multiline input. As fallback, **Alt+Enter** sends the same CSI u sequence.

## Configuration

A config file is auto-created at `~/.multiterminal.yaml` on first run.

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
main.go                          Entry point
internal/
  app/                           Application logic (model, input, views, tabs)
  ui/                            UI components (styles, themes, layout, widgets)
  terminal/                      PTY session management & VT100 emulation
  config/                        YAML config & session persistence
```

## License

MIT
