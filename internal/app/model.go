// Package app contains the main Bubbletea model that orchestrates
// every component of the Multiterminal TUI.
package app

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/patrick-goecommerce/multiterminal/internal/config"
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
	Tab        ui.Tab
	Panes      []ui.PaneInfo
	FocusIdx   int
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

	showHelp      bool
	quitting      bool
	lastCtrlC     time.Time // for double-Ctrl+C quit
	nextSessionID int

	// passthrough: when true, all key events go to the focused terminal
	// instead of being handled by the app. Toggle with Ctrl+G (escape hatch).
	passthrough bool

	// sidebarFocused: when true, arrow keys and Enter navigate the sidebar
	// instead of panes. Toggled with Ctrl+F.
	sidebarFocused bool

	// zoomed: when true, the focused pane is maximised to fill the content area.
	zoomed bool

	// Commit reminder tracking
	lastCommitCheck time.Time
	lastCommitTime  time.Time
	commitReminder  string
}

// New creates the initial Model. If session restore is enabled and a
// previous session file exists, the saved tabs and panes are re-created
// so the user can continue where they left off.
func New(cfg config.Config) Model {
	// Apply theme from configuration
	ui.SetTheme(cfg.Theme)

	dir := cfg.DefaultDir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	m := Model{
		cfg:     cfg,
		dialog:  ui.NewDialog(cfg),
		sidebar: ui.NewSidebar(dir, cfg.SidebarWidth),
	}

	// Try restoring a previous session
	if cfg.ShouldRestoreSession() {
		if m.restoreSession(dir) {
			return m
		}
	}

	// No saved session — start with one tab containing one shell pane
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
		// Scan Claude panes for token/cost info and activity changes
		m.scanClaudePanes()
		// Check commit reminder
		m.checkCommitReminder()
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
		// Ctrl+Scroll: zoom in / zoom out
		if msg.Ctrl && msg.Button == tea.MouseButtonWheelUp {
			if !m.zoomed {
				m.zoomed = true
				m.resizeAllPanes()
			}
			return m, nil
		}
		if msg.Ctrl && msg.Button == tea.MouseButtonWheelDown {
			if m.zoomed {
				m.zoomed = false
				m.resizeAllPanes()
			}
			return m, nil
		}
		return m, nil
	}

	return m, nil
}
