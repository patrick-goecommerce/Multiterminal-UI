// Package app contains the main Bubbletea model that orchestrates
// every component of the Multiterminal TUI.
package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/patrick-goecommerce/multiterminal/internal/config"
	"github.com/patrick-goecommerce/multiterminal/internal/terminal"
	"github.com/patrick-goecommerce/multiterminal/internal/ui"
)

// ---------------------------------------------------------------------------
// Bubbletea messages
// ---------------------------------------------------------------------------

// termOutputMsg is sent when a terminal session produces new output.
type termOutputMsg struct {
	sessionID int
}

// termExitMsg is sent when a terminal session's process exits.
type termExitMsg struct {
	sessionID int
}

// tickMsg fires periodically to refresh git branch info and detect output.
type tickMsg time.Time

// ---------------------------------------------------------------------------
// Per-tab state
// ---------------------------------------------------------------------------

// tabState holds all panes belonging to one tab.
type tabState struct {
	Tab       ui.Tab
	Panes     []ui.PaneInfo
	FocusIdx  int
	NextPaneID int // monotonically increasing pane ID counter
}

// ---------------------------------------------------------------------------
// Model – the top-level Bubbletea model
// ---------------------------------------------------------------------------

// Model is the root application model.
type Model struct {
	cfg    config.Config
	tabs   []tabState
	tabIdx int // active tab

	width  int
	height int

	dialog  ui.Dialog
	sidebar ui.Sidebar

	showHelp     bool
	quitting     bool
	lastCtrlC    time.Time // for double-Ctrl+C quit
	nextSessionID int

	// passthrough: when true, all key events go to the focused terminal
	// instead of being handled by the app. Toggle with Ctrl+G (escape hatch).
	passthrough bool

	// sidebarFocused: when true, arrow keys and Enter navigate the sidebar
	// instead of panes. Toggled with Ctrl+F.
	sidebarFocused bool
}

// New creates the initial Model.
func New(cfg config.Config) Model {
	dir := cfg.DefaultDir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	m := Model{
		cfg:     cfg,
		dialog:  ui.NewDialog(cfg),
		sidebar: ui.NewSidebar(dir, cfg.SidebarWidth),
	}

	// Start with one tab containing one shell pane
	m.addTab("Workspace", dir)

	return m
}

// Init is the Bubbletea initialiser. We start a periodic tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd())
}

// tickCmd returns a command that fires a tickMsg every 500ms.
func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update processes incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeAllPanes()
		return m, nil

	case tickMsg:
		// Refresh git branch for the focused pane
		m.refreshGitBranch()
		// Check for new output from all sessions
		m.checkSessionOutput()
		return m, tickCmd()

	case termOutputMsg:
		// Handled by tick now; kept for future direct signalling
		return m, nil

	case termExitMsg:
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		// Future: mouse support for clicking on tabs/panes
		return m, nil
	}

	return m, nil
}

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

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the entire UI.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	if m.width == 0 || m.height == 0 {
		return "Initialising…"
	}

	// Help overlay takes over the whole screen
	if m.showHelp {
		help := ShortcutHelp()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, help)
	}

	// Dialog overlay takes over the whole screen
	if m.dialog.Visible {
		// Render the normal UI underneath, then overlay the dialog
		base := m.renderNormal()
		overlay := m.dialog.Render(m.width, m.height)
		_ = base
		return overlay
	}

	return m.renderNormal()
}

// renderNormal draws the standard layout: tab bar + sidebar + panes + footer.
func (m Model) renderNormal() string {
	// Reserve space: 1 row tab bar, 1 row footer
	tabBar := ui.RenderTabBar(m.allTabs(), m.tabIdx, m.width)
	footer := ui.RenderFooter(m.footerData(), m.width)

	contentH := m.height - 2 // tab bar + footer
	if contentH < 1 {
		contentH = 1
	}

	// Sidebar
	var sidebarStr string
	contentW := m.width
	if m.sidebar.Visible {
		sidebarStr = m.sidebar.Render(contentH)
		contentW -= m.sidebar.Width
		if contentW < 10 {
			contentW = 10
		}
	}

	// Pane grid
	panesStr := m.renderPanes(contentW, contentH)

	// Compose the middle section
	var middle string
	if m.sidebar.Visible {
		middle = lipgloss.JoinHorizontal(lipgloss.Top, sidebarStr, panesStr)
	} else {
		middle = panesStr
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, middle, footer)
}

// renderPanes draws all panes in the active tab using the grid layout.
func (m Model) renderPanes(areaW, areaH int) string {
	tab := m.activeTab()
	if tab == nil || len(tab.Panes) == 0 {
		placeholder := lipgloss.NewStyle().
			Width(areaW).
			Height(areaH).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(ui.ColorMuted).
			Render("No panes. Press Ctrl+N to create one.")
		return placeholder
	}

	rects := ui.ComputeGrid(len(tab.Panes), areaW, areaH)

	// Render each pane into its rect
	// For simplicity we build rows of joined panes.
	// Since panes can be different sizes, we place each one independently.
	// Use a canvas approach: create an empty buffer and "stamp" each pane.
	canvas := make([][]rune, areaH)
	for r := range canvas {
		canvas[r] = make([]rune, areaW)
		for c := range canvas[r] {
			canvas[r][c] = ' '
		}
	}

	for i, pi := range tab.Panes {
		if i >= len(rects) {
			break
		}
		rect := rects[i]
		rendered := ui.RenderPane(pi, rect)
		// Stamp the rendered string onto the canvas
		stampOnCanvas(canvas, rendered, rect.X, rect.Y, rect.Width, rect.Height)
	}

	// Convert canvas to string
	var b strings.Builder
	for r, row := range canvas {
		if r > 0 {
			b.WriteByte('\n')
		}
		for _, ch := range row {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// stampOnCanvas writes a rendered string block onto the rune canvas.
func stampOnCanvas(canvas [][]rune, rendered string, x, y, w, h int) {
	lines := strings.Split(rendered, "\n")
	for dy, line := range lines {
		if y+dy >= len(canvas) {
			break
		}
		col := x
		for _, ch := range line {
			if col >= x+w || col >= len(canvas[y+dy]) {
				break
			}
			canvas[y+dy][col] = ch
			col++
		}
	}
}

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

// keyToBytes converts a Bubbletea key message to raw bytes for PTY input.
func keyToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyBackspace:
		return []byte{0x7f}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyCtrlA:
		return []byte{0x01}
	case tea.KeyCtrlB:
		return []byte{0x02}
	case tea.KeyCtrlC:
		return []byte{0x03}
	case tea.KeyCtrlD:
		return []byte{0x04}
	case tea.KeyCtrlE:
		return []byte{0x05}
	case tea.KeyCtrlF:
		return []byte{0x06}
	case tea.KeyCtrlG:
		return []byte{0x07}
	case tea.KeyCtrlH:
		return []byte{0x08}
	case tea.KeyCtrlJ:
		return []byte{0x0a}
	case tea.KeyCtrlK:
		return []byte{0x0b}
	case tea.KeyCtrlL:
		return []byte{0x0c}
	case tea.KeyCtrlN:
		return []byte{0x0e}
	case tea.KeyCtrlO:
		return []byte{0x0f}
	case tea.KeyCtrlP:
		return []byte{0x10}
	case tea.KeyCtrlQ:
		return []byte{0x11}
	case tea.KeyCtrlR:
		return []byte{0x12}
	case tea.KeyCtrlS:
		return []byte{0x13}
	case tea.KeyCtrlT:
		return []byte{0x14}
	case tea.KeyCtrlU:
		return []byte{0x15}
	case tea.KeyCtrlV:
		return []byte{0x16}
	case tea.KeyCtrlW:
		return []byte{0x17}
	case tea.KeyCtrlX:
		return []byte{0x18}
	case tea.KeyCtrlY:
		return []byte{0x19}
	case tea.KeyCtrlZ:
		return []byte{0x1a}
	case tea.KeyUp:
		return []byte{0x1b, '[', 'A'}
	case tea.KeyDown:
		return []byte{0x1b, '[', 'B'}
	case tea.KeyRight:
		return []byte{0x1b, '[', 'C'}
	case tea.KeyLeft:
		return []byte{0x1b, '[', 'D'}
	case tea.KeyHome:
		return []byte{0x1b, '[', 'H'}
	case tea.KeyEnd:
		return []byte{0x1b, '[', 'F'}
	case tea.KeyDelete:
		return []byte{0x1b, '[', '3', '~'}
	case tea.KeyPgUp:
		return []byte{0x1b, '[', '5', '~'}
	case tea.KeyPgDown:
		return []byte{0x1b, '[', '6', '~'}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

// resizeAllPanes recalculates dimensions for all panes in the active tab.
func (m *Model) resizeAllPanes() {
	tab := m.activeTab()
	if tab == nil {
		return
	}

	contentH := m.height - 2 // tab bar + footer
	contentW := m.width
	if m.sidebar.Visible {
		contentW -= m.sidebar.Width
	}
	if contentW < 10 {
		contentW = 10
	}
	if contentH < 3 {
		contentH = 3
	}

	rects := ui.ComputeGrid(len(tab.Panes), contentW, contentH)
	for i, p := range tab.Panes {
		if i >= len(rects) {
			break
		}
		r := rects[i]
		// Inner size = rect minus border (2 cols, 2 rows) minus title (1 row)
		innerW := r.Width - 2
		innerH := r.Height - 3
		if innerW < 1 {
			innerW = 1
		}
		if innerH < 1 {
			innerH = 1
		}
		if p.Session != nil {
			p.Session.Resize(innerH, innerW)
		}
	}
}

// refreshGitBranch updates the Branch field of the focused pane.
func (m *Model) refreshGitBranch() {
	tab := m.activeTab()
	if tab == nil || len(tab.Panes) == 0 {
		return
	}
	idx := tab.FocusIdx
	if idx < 0 || idx >= len(tab.Panes) {
		return
	}

	dir := tab.Tab.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	branch := gitBranch(dir)
	tab.Panes[idx].Branch = branch
}

// gitBranch returns the current git branch name for the given directory.
func gitBranch(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// checkSessionOutput is a no-op placeholder. In a more advanced version,
// this would read from session output channels. The tick-based re-render
// handles display updates.
func (m *Model) checkSessionOutput() {}

// currentDir returns the working directory of the active tab.
func (m *Model) currentDir() string {
	tab := m.activeTab()
	if tab != nil && tab.Tab.Dir != "" {
		return tab.Tab.Dir
	}
	dir, _ := os.Getwd()
	return dir
}

// footerData assembles the data needed to render the footer.
func (m *Model) footerData() ui.FooterData {
	d := ui.FooterData{
		TabCount: len(m.tabs),
		TabIdx:   m.tabIdx,
	}

	tab := m.activeTab()
	if tab == nil {
		return d
	}

	d.PaneIdx = tab.FocusIdx
	if tab.FocusIdx >= 0 && tab.FocusIdx < len(tab.Panes) {
		p := tab.Panes[tab.FocusIdx]
		d.Branch = p.Branch
		d.Model = p.Model
		d.PaneName = p.Name
		switch p.Mode {
		case ui.PaneModeShell:
			d.Mode = "Shell"
		case ui.PaneModeClaudeNormal:
			d.Mode = "Claude"
		case ui.PaneModeClaudeYOLO:
			d.Mode = "YOLO"
		}
	}
	return d
}

// closeAllSessions closes every session across all tabs.
func (m *Model) closeAllSessions() {
	for _, ts := range m.tabs {
		for _, p := range ts.Panes {
			if p.Session != nil {
				go p.Session.Close()
			}
		}
	}
}
