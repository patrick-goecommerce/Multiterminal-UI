package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/patrick-goecommerce/multiterminal/internal/ui"
)

// handleKey routes keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ---------------------------------------------------------------
	// Dialog mode – keys go to the dialog
	// ---------------------------------------------------------------
	if m.dialog.Visible {
		return m.handleDialogKey(msg)
	}

	// ---------------------------------------------------------------
	// Help overlay – any key closes it
	// ---------------------------------------------------------------
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	// ---------------------------------------------------------------
	// Sidebar search mode – keys go to sidebar
	// ---------------------------------------------------------------
	if m.sidebar.Editing {
		return m.handleSidebarSearchKey(msg)
	}

	// ---------------------------------------------------------------
	// Sidebar focus mode – navigate files, Enter inserts path
	// ---------------------------------------------------------------
	if m.sidebarFocused && m.sidebar.Visible {
		return m.handleSidebarFocusKey(msg)
	}

	// ---------------------------------------------------------------
	// Passthrough mode – everything except Ctrl+G goes to terminal
	// ---------------------------------------------------------------
	if m.passthrough {
		if isKey(msg, tea.KeyCtrlG) {
			m.passthrough = false
			return m, nil
		}
		m.sendKeyToTerminal(msg)
		return m, nil
	}

	// ---------------------------------------------------------------
	// Global shortcuts
	// ---------------------------------------------------------------

	// Quit: double Ctrl+C
	if isKey(msg, tea.KeyCtrlC) {
		if time.Since(m.lastCtrlC) < 500*time.Millisecond {
			m.quitting = true
			m.saveSession()
			m.closeAllSessions()
			return m, tea.Quit
		}
		m.lastCtrlC = time.Now()
		// Also forward a single Ctrl+C to the focused terminal
		m.sendKeyToTerminal(msg)
		return m, nil
	}

	// Shift+Enter → send kitty CSI u sequence to child PTY.
	// Many terminals report Alt+Enter when Shift+Enter is pressed;
	// Bubbletea v1 surfaces this as KeyEnter with Alt=true.
	if isKey(msg, tea.KeyEnter) && msg.Alt {
		m.sendBytesToTerminal([]byte("\x1b[13;2u"))
		return m, nil
	}

	// New tab
	if isKey(msg, tea.KeyCtrlT) {
		dir := m.currentDir()
		m.addTab("", dir)
		return m, nil
	}

	// Close tab
	if isKey(msg, tea.KeyCtrlW) {
		m.closeCurrentTab()
		return m, nil
	}

	// New pane (launch dialog)
	if isKey(msg, tea.KeyCtrlN) {
		m.dialog.Open()
		return m, nil
	}

	// Close focused pane
	if isKey(msg, tea.KeyCtrlX) {
		m.closeFocusedPane()
		return m, nil
	}

	// Toggle sidebar
	if isKey(msg, tea.KeyCtrlB) {
		m.sidebar.Visible = !m.sidebar.Visible
		if !m.sidebar.Visible {
			m.sidebarFocused = false
			m.sidebar.Focused = false
		}
		m.resizeAllPanes()
		return m, nil
	}

	// Focus sidebar (Ctrl+F)
	if isKey(msg, tea.KeyCtrlF) {
		if m.sidebar.Visible {
			m.sidebarFocused = !m.sidebarFocused
			m.sidebar.Focused = m.sidebarFocused
		}
		return m, nil
	}

	// Zoom toggle (maximise / restore focused pane)
	if isKey(msg, tea.KeyCtrlZ) {
		m.zoomed = !m.zoomed
		m.resizeAllPanes()
		return m, nil
	}

	// Passthrough toggle
	if isKey(msg, tea.KeyCtrlG) {
		m.passthrough = true
		return m, nil
	}

	// Help
	if isRune(msg, '?') {
		m.showHelp = true
		return m, nil
	}

	// Tab switching with number keys 1-9
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r >= '1' && r <= '9' {
			idx := int(r - '1')
			if idx < len(m.tabs) {
				m.tabIdx = idx
				return m, nil
			}
		}
	}

	// Pane navigation with arrow keys
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight:
		m.navigatePane(msg.Type)
		return m, nil
	case tea.KeyTab:
		m.cyclePaneFocus()
		return m, nil
	}

	// Everything else → forward to focused terminal
	m.sendKeyToTerminal(msg)
	return m, nil
}

// handleDialogKey processes keys when the launch dialog is open.
func (m Model) handleDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.dialog.Close()
	case tea.KeyUp:
		m.dialog.MoveUp()
	case tea.KeyDown:
		m.dialog.MoveDown()
	case tea.KeyEnter:
		done := m.dialog.Select()
		if done && m.dialog.Choice.Type != ui.LaunchCancel {
			m.launchPane(m.dialog.Choice)
		}
	}
	return m, nil
}

// handleSidebarFocusKey processes keys when the sidebar is focused.
// Arrow keys navigate the file tree; Enter inserts a file path into the
// focused terminal pane (or toggles a directory).
func (m Model) handleSidebarFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlF:
		// Return focus to panes
		m.sidebarFocused = false
		m.sidebar.Focused = false
		return m, nil
	case tea.KeyUp:
		m.sidebar.MoveUp()
		return m, nil
	case tea.KeyDown:
		m.sidebar.MoveDown()
		return m, nil
	case tea.KeyEnter:
		if m.sidebar.IsSelectedDir() {
			m.sidebar.Toggle()
		} else {
			// Insert file path into focused terminal
			m.insertFilePathToTerminal()
		}
		return m, nil
	case tea.KeyCtrlB:
		// Close sidebar
		m.sidebar.Visible = false
		m.sidebarFocused = false
		m.sidebar.Focused = false
		m.resizeAllPanes()
		return m, nil
	}

	// Start search with /
	if isRune(msg, '/') {
		m.sidebar.Editing = true
		m.sidebar.Search = ""
		return m, nil
	}

	return m, nil
}

// handleSidebarSearchKey processes keys when the sidebar search is active.
func (m Model) handleSidebarSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.sidebar.Editing = false
		m.sidebar.Search = ""
		m.sidebar.Refresh()
	case tea.KeyEnter:
		m.sidebar.Editing = false
		m.sidebar.Toggle()
	case tea.KeyBackspace:
		if len(m.sidebar.Search) > 0 {
			m.sidebar.Search = m.sidebar.Search[:len(m.sidebar.Search)-1]
			m.sidebar.Refresh()
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.sidebar.Search += string(msg.Runes)
			m.sidebar.Refresh()
		}
	}
	return m, nil
}

