# Multiterminal

A TUI terminal multiplexer built for Claude Code power users.

## Tech Stack
- **Language:** Go 1.21+
- **TUI framework:** Bubbletea + Lipgloss + Bubbles
- **Terminal emulation:** Custom VT100 screen buffer + creack/pty
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
    layout.go                    Grid layout calculator for panes
    tabbar.go                    Tab bar component
    pane.go                      Terminal pane component (wraps a session)
    footer.go                    Global status footer (branch, model, shortcuts)
    sidebar.go                   File browser sidebar with search
    dialog.go                    Launch dialog (Shell / Claude / YOLO / model picker)
  terminal/
    screen.go                    VT100 screen buffer + ANSI parser
    session.go                   PTY session lifecycle (start, resize, write, close)
  config/
    config.go                    YAML configuration loader
```

## Build & Run
```bash
go build -o multiterminal .
./multiterminal
```

## Key Shortcuts
| Key              | Action                                        |
|------------------|-----------------------------------------------|
| Ctrl+T           | New tab                                       |
| Ctrl+W           | Close tab                                     |
| 1-9              | Switch tab                                    |
| Ctrl+N           | New pane (opens launch dialog)                |
| Ctrl+X           | Close focused pane                            |
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
| Ctrl+C (×2)      | Quit                                          |

## File Insertion from Sidebar
1. Open the sidebar with **Ctrl+B**
2. Focus it with **Ctrl+F** (title shows "Files [ACTIVE]")
3. Navigate with arrow keys, search with `/`
4. Press **Enter** on a file → its full path is typed into the focused terminal
5. Works with images too – Claude Code reads images by path

## Shift+Enter Support
For Claude Code panes, the kitty keyboard protocol is auto-enabled so that
Shift+Enter works natively for multiline input. As fallback, **Alt+Enter**
sends the same CSI u sequence (`ESC[13;2u`).

## Configuration
See `~/.multiterminal.yaml` for defaults (auto-created on first run).
