package app

import tea "github.com/charmbracelet/bubbletea"

// ---------------------------------------------------------------------------
// Key‐binding helpers
// ---------------------------------------------------------------------------

// isKey checks whether a tea.KeyMsg matches a given key type (e.g. tea.KeyCtrlT).
func isKey(msg tea.KeyMsg, k tea.KeyType) bool {
	return msg.Type == k
}

// isRune checks whether a tea.KeyMsg is a specific rune.
func isRune(msg tea.KeyMsg, r rune) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == r
}

// ---------------------------------------------------------------------------
// Shortcut help text (shown in the help overlay)
// ---------------------------------------------------------------------------

// ShortcutHelp returns the full help text displayed when the user presses '?'.
func ShortcutHelp() string {
	return `
╔════════════════════════════════════════════════════════════╗
║                 Multiterminal – Shortcuts                  ║
╠════════════════════════════════════════════════════════════╣
║                                                            ║
║  Tabs                                                      ║
║    Ctrl+T         Create new tab                           ║
║    Ctrl+W         Close current tab                        ║
║    1-9            Switch to tab N (when not typing)        ║
║                                                            ║
║  Panes                                                     ║
║    Ctrl+N         New pane (opens launch dialog)           ║
║    Ctrl+X         Close focused pane                       ║
║    Ctrl+Z         Zoom (maximise/restore) focused pane     ║
║    Ctrl+Scroll    Zoom in (up) / zoom out (down)           ║
║    ←↑↓→           Navigate between panes                   ║
║    Tab            Cycle focus to next pane                  ║
║    Ctrl+G         Passthrough mode (all keys to terminal)  ║
║    Alt+Enter      Shift+Enter (newline in Claude Code)     ║
║                                                            ║
║  File Browser (Sidebar)                                    ║
║    Ctrl+B         Toggle file browser                      ║
║    Ctrl+F         Focus/unfocus sidebar                    ║
║    ↑↓             Navigate files (when sidebar focused)    ║
║    Enter           Dir: expand/collapse                    ║
║                    File: insert path into terminal         ║
║    /               Start search (filters file list)        ║
║    Esc             Return focus to panes                   ║
║                                                            ║
║  General                                                   ║
║    Ctrl+S         Set working directory for tab            ║
║    ?              Show/hide this help                      ║
║    Ctrl+C (×2)    Quit                                     ║
║                                                            ║
║  Smart Features                                            ║
║    Token/cost tracker shown in footer and pane titles      ║
║    Pane border flashes green when Claude finishes          ║
║    Pane border flashes yellow when input is needed         ║
║    Commit reminder after 30 min (configurable in YAML)    ║
║    Theme: set "theme" in ~/.multiterminal.yaml             ║
║      Available: dark, light, dracula, nord, solarized      ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝`
}
