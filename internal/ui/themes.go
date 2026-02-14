package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds a complete color palette for the application.
type Theme struct {
	Name      string
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Danger    lipgloss.Color
	Muted     lipgloss.Color
	BG        lipgloss.Color
	Surface   lipgloss.Color
	Text      lipgloss.Color
	TextDim   lipgloss.Color
	Border    lipgloss.Color
	Highlight lipgloss.Color
}

// Themes is the registry of all available color themes.
var Themes = map[string]Theme{
	"dark": {
		Name:      "dark",
		Primary:   lipgloss.Color("#7C3AED"),
		Secondary: lipgloss.Color("#06B6D4"),
		Success:   lipgloss.Color("#22C55E"),
		Warning:   lipgloss.Color("#F59E0B"),
		Danger:    lipgloss.Color("#EF4444"),
		Muted:     lipgloss.Color("#6B7280"),
		BG:        lipgloss.Color("#1E1E2E"),
		Surface:   lipgloss.Color("#313244"),
		Text:      lipgloss.Color("#CDD6F4"),
		TextDim:   lipgloss.Color("#6C7086"),
		Border:    lipgloss.Color("#45475A"),
		Highlight: lipgloss.Color("#F5C2E7"),
	},
	"light": {
		Name:      "light",
		Primary:   lipgloss.Color("#7C3AED"),
		Secondary: lipgloss.Color("#0891B2"),
		Success:   lipgloss.Color("#16A34A"),
		Warning:   lipgloss.Color("#D97706"),
		Danger:    lipgloss.Color("#DC2626"),
		Muted:     lipgloss.Color("#9CA3AF"),
		BG:        lipgloss.Color("#F8FAFC"),
		Surface:   lipgloss.Color("#E2E8F0"),
		Text:      lipgloss.Color("#1E293B"),
		TextDim:   lipgloss.Color("#94A3B8"),
		Border:    lipgloss.Color("#CBD5E1"),
		Highlight: lipgloss.Color("#A855F7"),
	},
	"dracula": {
		Name:      "dracula",
		Primary:   lipgloss.Color("#BD93F9"),
		Secondary: lipgloss.Color("#8BE9FD"),
		Success:   lipgloss.Color("#50FA7B"),
		Warning:   lipgloss.Color("#F1FA8C"),
		Danger:    lipgloss.Color("#FF5555"),
		Muted:     lipgloss.Color("#6272A4"),
		BG:        lipgloss.Color("#282A36"),
		Surface:   lipgloss.Color("#44475A"),
		Text:      lipgloss.Color("#F8F8F2"),
		TextDim:   lipgloss.Color("#6272A4"),
		Border:    lipgloss.Color("#44475A"),
		Highlight: lipgloss.Color("#FF79C6"),
	},
	"nord": {
		Name:      "nord",
		Primary:   lipgloss.Color("#88C0D0"),
		Secondary: lipgloss.Color("#81A1C1"),
		Success:   lipgloss.Color("#A3BE8C"),
		Warning:   lipgloss.Color("#EBCB8B"),
		Danger:    lipgloss.Color("#BF616A"),
		Muted:     lipgloss.Color("#4C566A"),
		BG:        lipgloss.Color("#2E3440"),
		Surface:   lipgloss.Color("#3B4252"),
		Text:      lipgloss.Color("#ECEFF4"),
		TextDim:   lipgloss.Color("#4C566A"),
		Border:    lipgloss.Color("#434C5E"),
		Highlight: lipgloss.Color("#88C0D0"),
	},
	"solarized": {
		Name:      "solarized",
		Primary:   lipgloss.Color("#268BD2"),
		Secondary: lipgloss.Color("#2AA198"),
		Success:   lipgloss.Color("#859900"),
		Warning:   lipgloss.Color("#B58900"),
		Danger:    lipgloss.Color("#DC322F"),
		Muted:     lipgloss.Color("#586E75"),
		BG:        lipgloss.Color("#002B36"),
		Surface:   lipgloss.Color("#073642"),
		Text:      lipgloss.Color("#839496"),
		TextDim:   lipgloss.Color("#586E75"),
		Border:    lipgloss.Color("#073642"),
		Highlight: lipgloss.Color("#B58900"),
	},
}

// ActiveTheme is the currently active theme.
var ActiveTheme = Themes["dark"]

// ThemeNames returns a sorted list of available theme names.
func ThemeNames() []string {
	return []string{"dark", "light", "dracula", "nord", "solarized"}
}

// SetTheme activates a theme by name and rebuilds all styles.
// Returns false if the theme name is not recognised.
func SetTheme(name string) bool {
	t, ok := Themes[name]
	if !ok {
		return false
	}
	ActiveTheme = t

	// Update color aliases
	ColorPrimary = t.Primary
	ColorSecondary = t.Secondary
	ColorSuccess = t.Success
	ColorWarning = t.Warning
	ColorDanger = t.Danger
	ColorMuted = t.Muted
	ColorBG = t.BG
	ColorSurface = t.Surface
	ColorText = t.Text
	ColorTextDim = t.TextDim
	ColorBorder = t.Border
	ColorHighlight = t.Highlight

	// Rebuild all lipgloss styles
	rebuildStyles()
	return true
}

// rebuildStyles re-creates every style variable using the current colors.
func rebuildStyles() {
	// Tab bar
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

	// Pane
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

	// Footer
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
		SetString(" | ")

	// Sidebar
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

	// Dialog
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
}
