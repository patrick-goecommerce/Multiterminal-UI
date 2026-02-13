// Package ui provides all Bubbletea components for the Multiterminal TUI.
package ui

import "github.com/charmbracelet/lipgloss"

// ---------------------------------------------------------------------------
// Colour palette
// ---------------------------------------------------------------------------

var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // violet-600
	ColorSecondary = lipgloss.Color("#06B6D4") // cyan-500
	ColorSuccess   = lipgloss.Color("#22C55E") // green-500
	ColorWarning   = lipgloss.Color("#F59E0B") // amber-500
	ColorDanger    = lipgloss.Color("#EF4444") // red-500
	ColorMuted     = lipgloss.Color("#6B7280") // gray-500
	ColorBG        = lipgloss.Color("#1E1E2E") // dark background
	ColorSurface   = lipgloss.Color("#313244") // slightly lighter
	ColorText      = lipgloss.Color("#CDD6F4") // light text
	ColorTextDim   = lipgloss.Color("#6C7086") // dim text
	ColorBorder    = lipgloss.Color("#45475A") // subtle border
	ColorHighlight = lipgloss.Color("#F5C2E7") // pink highlight
)

// ---------------------------------------------------------------------------
// Shared styles
// ---------------------------------------------------------------------------

// TabBar styles
var (
	TabBarStyle = lipgloss.NewStyle().
			Background(ColorBG).
			Padding(0, 1)

	TabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBG).
			Background(ColorPrimary).
			Padding(0, 2)

	TabInactive = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Background(ColorSurface).
			Padding(0, 2)

	TabAdd = lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)
)

// Pane styles
var (
	PaneBorderFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	PaneBorderUnfocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder)

	PaneTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Padding(0, 1)

	PaneStatusRunning = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	PaneStatusExited = lipgloss.NewStyle().
				Foreground(ColorWarning)

	PaneStatusError = lipgloss.NewStyle().
			Foreground(ColorDanger)
)

// Footer styles
var (
	FooterStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorText).
			Padding(0, 1)

	FooterKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	FooterValStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	FooterDimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	FooterSepStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			SetString(" │ ")
)

// Sidebar styles
var (
	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), false, true, false, false).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	SidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 0, 1, 0)

	SidebarDir = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	SidebarFile = lipgloss.NewStyle().
			Foreground(ColorText)

	SidebarSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHighlight)

	SidebarSearch = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Italic(true)
)

// Dialog styles
var (
	DialogOverlay = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Width(52)

	DialogTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 0, 1, 0)

	DialogOption = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 2)

	DialogOptionSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorHighlight).
				Padding(0, 2)

	DialogHint = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Italic(true)
)
