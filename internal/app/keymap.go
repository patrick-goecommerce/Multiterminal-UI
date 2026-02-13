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
╔═══════════════════════════════════════════════════════╗
║               Multiterminal – Shortcuts               ║
╠═══════════════════════════════════════════════════════╣
║                                                       ║
║  Tabs                                                 ║
║    Ctrl+T        Create new tab                       ║
║    Ctrl+W        Close current tab                    ║
║    1-9           Switch to tab N (when not typing)    ║
║    Ctrl+PgUp     Previous tab                         ║
║    Ctrl+PgDn     Next tab                             ║
║                                                       ║
║  Panes                                                ║
║    Ctrl+N        New pane (opens launch dialog)       ║
║    Ctrl+X        Close focused pane                   ║
║    ←↑↓→          Navigate between panes               ║
║    Tab           Cycle focus to next pane              ║
║                                                       ║
║  Sidebar                                              ║
║    Ctrl+B        Toggle file browser                  ║
║    /              Start search in sidebar             ║
║    Enter          Expand/collapse directory            ║
║                                                       ║
║  General                                              ║
║    Ctrl+S        Set working directory for tab        ║
║    ?             Show/hide this help                  ║
║    Ctrl+C (×2)   Quit                                 ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝`
}
