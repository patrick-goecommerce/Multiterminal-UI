# Multiterminal

A TUI terminal multiplexer built for Claude Code power users.

## Tech Stack
- **Language:** Go 1.21+
- **TUI framework:** Bubbletea + Lipgloss + Bubbles
- **Terminal emulation:** Custom VT100 screen buffer + go-pty (cross-platform)
- **Config:** YAML (~/.multiterminal.yaml)

## Project Structure
```
main.go                          Entry point
internal/
  app/
    model.go                     Main Bubbletea model (orchestrates everything)
    keymap.go                    Key binding definitions
  ui/
    styles.go                    Lipgloss style constants
    themes.go                    Theme definitions (dark, light, dracula, nord, solarized)
    layout.go                    Grid layout calculator for panes
    tabbar.go                    Tab bar component
    pane.go                      Terminal pane component (wraps a session)
    footer.go                    Global status footer (branch, model, cost, shortcuts)
    sidebar.go                   File browser sidebar with search
    dialog.go                    Launch dialog (Shell / Claude / YOLO / model picker)
  terminal/
    screen.go                    VT100 screen buffer + ANSI parser
    session.go                   PTY session lifecycle + token tracker + activity detection
  config/
    config.go                    YAML configuration loader
```

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

## Key Shortcuts
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
| /  (sidebar)     | Start file search                             |
| Ctrl+S           | Set working directory for tab                 |
| ?                | Show keyboard shortcuts help                  |
| Ctrl+C (x2)      | Quit                                          |

## Smart Features

### Token / Cost Tracker
Claude Code panes automatically scan for token usage and cost information.
- **Per-pane cost** is shown in the pane title bar (e.g. `$0.12`)
- **Total cost** across all Claude panes is shown in the global footer

### Auto-detect Claude Activity
The pane border **flashes** when Claude changes state:
- **Green flash** (3s) — Claude finished generating (prompt returned)
- **Yellow flash** (5s) — Claude needs user input (confirmation, Y/n, etc.)

This works even when the pane is not focused, so you can work in another
terminal and see at a glance when Claude needs attention.

### Commit Reminder
A configurable reminder appears in the footer when too much time has passed
since the last git commit. Default: 30 minutes.
Set `commit_reminder_minutes: 0` in config to disable.

### Zoom (Maximise Pane)
Press **Ctrl+Z** to toggle the focused pane between maximised (full content
area) and normal grid layout. Useful when you want to focus on one Claude
session's output. Footer shows `[ZOOM]` when active.

### Themes
Five built-in colour themes. Set `theme` in `~/.multiterminal.yaml`:
- `dark` (default) — Catppuccin Mocha inspired
- `light` — Clean light theme
- `dracula` — Dracula color scheme
- `nord` — Nord color scheme
- `solarized` — Solarized Dark

## File Insertion from Sidebar
1. Open the sidebar with **Ctrl+B**
2. Focus it with **Ctrl+F** (title shows "Files [ACTIVE]")
3. Navigate with arrow keys, search with `/`
4. Press **Enter** on a file → its full path is typed into the focused terminal
5. Works with images too — Claude Code reads images by path

## Shift+Enter Support
For Claude Code panes, the kitty keyboard protocol is auto-enabled so that
Shift+Enter works natively for multiline input. As fallback, **Alt+Enter**
sends the same CSI u sequence (`ESC[13;2u`).

## Configuration
See `~/.multiterminal.yaml` for defaults (auto-created on first run).

```yaml
# Example configuration
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
