package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FooterData holds the information displayed in the global status footer.
type FooterData struct {
	Branch         string // current git branch of the focused pane
	Model          string // Claude model of the focused pane
	Mode           string // "Shell" / "Claude" / "YOLO"
	TabCount       int    // total number of tabs
	TabIdx         int    // 0-based active tab index
	PaneIdx        int    // 0-based active pane index in the tab
	PaneName       string // name of the focused pane
	TotalCost      string // total token cost across all Claude panes
	CommitReminder string // commit reminder message (empty = no reminder)
	ThemeName      string // active theme name
	Zoomed         bool   // whether a pane is maximized
}

// RenderFooter draws the global status bar at the bottom of the screen.
// It shows: branch (copyable hint), model, mode, and quick shortcut help.
func RenderFooter(d FooterData, width int) string {
	var sections []string

	// Branch
	if d.Branch != "" {
		sections = append(sections,
			FooterKeyStyle.Render("branch:")+
				FooterValStyle.Render(" "+d.Branch))
	}

	// Model
	if d.Model != "" {
		sections = append(sections,
			FooterKeyStyle.Render("model:")+
				FooterValStyle.Render(" "+d.Model))
	}

	// Mode
	if d.Mode != "" {
		sections = append(sections,
			FooterKeyStyle.Render("mode:")+
				FooterValStyle.Render(" "+d.Mode))
	}

	// Total cost across all Claude panes
	if d.TotalCost != "" {
		sections = append(sections,
			FooterKeyStyle.Render("cost:")+
				lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render(" "+d.TotalCost))
	}

	// Tab / Pane indicator
	tabInfo := fmt.Sprintf("Tab %d/%d  Pane %d", d.TabIdx+1, d.TabCount, d.PaneIdx+1)
	if d.Zoomed {
		tabInfo += " [ZOOM]"
	}
	sections = append(sections, FooterDimStyle.Render(tabInfo))

	// Commit reminder (flashes with warning color)
	if d.CommitReminder != "" {
		sections = append(sections,
			lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render(d.CommitReminder))
	}

	// Shortcuts hint (right-aligned)
	shortcuts := FooterDimStyle.Render("Ctrl+N:new  Ctrl+Z:zoom  Ctrl+B:files  ?:help")

	left := strings.Join(sections, FooterSepStyle.Render(""))
	right := shortcuts

	// Calculate gap to right-align the shortcuts
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth - 2
	if gap < 1 {
		gap = 1
	}

	line := left + strings.Repeat(" ", gap) + right

	return FooterStyle.Width(width).Render(line)
}
