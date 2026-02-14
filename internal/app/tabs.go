package app

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/patrick-goecommerce/multiterminal/internal/terminal"
	"github.com/patrick-goecommerce/multiterminal/internal/ui"
)

// ---------------------------------------------------------------------------
// Tab & pane management
// ---------------------------------------------------------------------------

// addTab creates a new tab with the given name and working directory.
func (m *Model) addTab(name, dir string) {
	if name == "" {
		name = fmt.Sprintf("Tab %d", len(m.tabs)+1)
	}
	ts := tabState{
		Tab: ui.Tab{Name: name, Dir: dir},
	}
	m.tabs = append(m.tabs, ts)
	m.tabIdx = len(m.tabs) - 1
}

// closeCurrentTab closes the active tab and all its panes.
func (m *Model) closeCurrentTab() {
	if len(m.tabs) <= 1 {
		return // don't close the last tab
	}
	tab := m.activeTab()
	if tab != nil {
		for _, p := range tab.Panes {
			if p.Session != nil {
				go p.Session.Close()
			}
		}
	}
	m.tabs = append(m.tabs[:m.tabIdx], m.tabs[m.tabIdx+1:]...)
	if m.tabIdx >= len(m.tabs) {
		m.tabIdx = len(m.tabs) - 1
	}
}

// activeTab returns a pointer to the current tab state, or nil.
func (m *Model) activeTab() *tabState {
	if m.tabIdx < 0 || m.tabIdx >= len(m.tabs) {
		return nil
	}
	return &m.tabs[m.tabIdx]
}

// allTabs returns a slice of ui.Tab for rendering the tab bar.
func (m *Model) allTabs() []ui.Tab {
	tabs := make([]ui.Tab, len(m.tabs))
	for i, ts := range m.tabs {
		tabs[i] = ts.Tab
	}
	return tabs
}

// launchPane creates a new pane in the active tab from a LaunchChoice.
func (m *Model) launchPane(choice ui.LaunchChoice) {
	tab := m.activeTab()
	if tab == nil {
		return
	}
	if len(tab.Panes) >= m.cfg.MaxPanesPerTab {
		return
	}

	m.nextSessionID++
	sid := m.nextSessionID

	// Calculate pane dimensions (rough estimate; will be corrected on next resize)
	paneH := m.height - 4 // minus tab bar + footer + borders
	paneW := m.width - 4
	if m.sidebar.Visible {
		paneW -= m.sidebar.Width
	}
	if paneH < 5 {
		paneH = 5
	}
	if paneW < 20 {
		paneW = 20
	}

	sess := terminal.NewSession(sid, paneH, paneW)

	dir := tab.Tab.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	var mode ui.PaneMode
	switch choice.Type {
	case ui.LaunchClaude:
		mode = ui.PaneModeClaudeNormal
	case ui.LaunchClaudeYOLO:
		mode = ui.PaneModeClaudeYOLO
	default:
		mode = ui.PaneModeShell
	}

	pane := ui.PaneInfo{
		Session: sess,
		Name:    paneName(mode, sid),
		Mode:    mode,
		Model:   choice.Model,
		Focused: true,
	}

	// Unfocus all existing panes
	for i := range tab.Panes {
		tab.Panes[i].Focused = false
	}
	tab.Panes = append(tab.Panes, pane)
	tab.FocusIdx = len(tab.Panes) - 1

	// Start the process
	_ = sess.Start(choice.Argv, dir, nil)

	// Enable kitty keyboard protocol for Claude Code panes so that
	// Shift+Enter is reported as CSI 13;2 u (multiline input).
	if mode == ui.PaneModeClaudeNormal || mode == ui.PaneModeClaudeYOLO {
		sess.EnableKittyKeyboard()
	}

	m.resizeAllPanes()
}

// paneName generates a default name for a pane.
func paneName(mode ui.PaneMode, id int) string {
	switch mode {
	case ui.PaneModeClaudeNormal:
		return fmt.Sprintf("Claude #%d", id)
	case ui.PaneModeClaudeYOLO:
		return fmt.Sprintf("YOLO #%d", id)
	default:
		return fmt.Sprintf("Shell #%d", id)
	}
}

// closeFocusedPane closes the currently focused pane.
func (m *Model) closeFocusedPane() {
	tab := m.activeTab()
	if tab == nil || len(tab.Panes) == 0 {
		return
	}
	idx := tab.FocusIdx
	if idx < 0 || idx >= len(tab.Panes) {
		return
	}

	pane := tab.Panes[idx]
	if pane.Session != nil {
		go pane.Session.Close()
	}

	tab.Panes = append(tab.Panes[:idx], tab.Panes[idx+1:]...)
	if tab.FocusIdx >= len(tab.Panes) {
		tab.FocusIdx = len(tab.Panes) - 1
	}
	// Update focus
	for i := range tab.Panes {
		tab.Panes[i].Focused = (i == tab.FocusIdx)
	}
	m.resizeAllPanes()
}

// cyclePaneFocus moves focus to the next pane.
func (m *Model) cyclePaneFocus() {
	tab := m.activeTab()
	if tab == nil || len(tab.Panes) <= 1 {
		return
	}
	tab.FocusIdx = (tab.FocusIdx + 1) % len(tab.Panes)
	for i := range tab.Panes {
		tab.Panes[i].Focused = (i == tab.FocusIdx)
	}
}

// navigatePane moves focus based on arrow key direction.
func (m *Model) navigatePane(key tea.KeyType) {
	tab := m.activeTab()
	if tab == nil || len(tab.Panes) <= 1 {
		return
	}

	n := len(tab.Panes)
	rects := ui.ComputeGrid(n, m.width, m.height-2)
	if len(rects) != n {
		return
	}

	cur := rects[tab.FocusIdx]
	best := -1
	bestDist := 999999

	for i, r := range rects {
		if i == tab.FocusIdx {
			continue
		}
		match := false
		switch key {
		case tea.KeyUp:
			match = r.Y+r.Height <= cur.Y
		case tea.KeyDown:
			match = r.Y >= cur.Y+cur.Height
		case tea.KeyLeft:
			match = r.X+r.Width <= cur.X
		case tea.KeyRight:
			match = r.X >= cur.X+cur.Width
		}
		if match {
			// Manhattan distance from centre to centre
			dx := (r.X + r.Width/2) - (cur.X + cur.Width/2)
			dy := (r.Y + r.Height/2) - (cur.Y + cur.Height/2)
			dist := abs(dx) + abs(dy)
			if dist < bestDist {
				bestDist = dist
				best = i
			}
		}
	}

	if best >= 0 {
		tab.FocusIdx = best
		for i := range tab.Panes {
			tab.Panes[i].Focused = (i == tab.FocusIdx)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// insertFilePathToTerminal types the selected sidebar file path into the
// focused terminal pane. For image files, it adds the path as-is – Claude
// Code can read images by path reference.
func (m *Model) insertFilePathToTerminal() {
	path := m.sidebar.SelectedPath()
	if path == "" {
		return
	}

	// Quote the path if it contains spaces
	if strings.Contains(path, " ") {
		path = "\"" + path + "\""
	}

	m.sendBytesToTerminal([]byte(path))
}

// sendBytesToTerminal writes raw bytes to the focused terminal's PTY.
func (m *Model) sendBytesToTerminal(data []byte) {
	tab := m.activeTab()
	if tab == nil || len(tab.Panes) == 0 {
		return
	}
	idx := tab.FocusIdx
	if idx < 0 || idx >= len(tab.Panes) {
		return
	}
	sess := tab.Panes[idx].Session
	if sess == nil || !sess.IsRunning() {
		return
	}
	sess.Write(data)
}

// sendKeyToTerminal forwards a key event to the focused terminal session.
func (m *Model) sendKeyToTerminal(msg tea.KeyMsg) {
	tab := m.activeTab()
	if tab == nil || len(tab.Panes) == 0 {
		return
	}
	idx := tab.FocusIdx
	if idx < 0 || idx >= len(tab.Panes) {
		return
	}
	sess := tab.Panes[idx].Session
	if sess == nil || !sess.IsRunning() {
		return
	}

	// Convert tea.KeyMsg to raw bytes for the PTY
	data := keyToBytes(msg)
	if len(data) > 0 {
		sess.Write(data)
	}
}
