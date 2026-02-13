package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FooterData holds the information displayed in the global status footer.
type FooterData struct {
	Branch   string // current git branch of the focused pane
	Model    string // Claude model of the focused pane
	Mode     string // "Shell" / "Claude" / "YOLO"
	TabCount int    // total number of tabs
	TabIdx   int    // 0-based active tab index
	PaneIdx  int    // 0-based active pane index in the tab
	PaneName string // name of the focused pane
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

	// Tab / Pane indicator
	sections = append(sections,
		FooterDimStyle.Render(
			fmt.Sprintf("Tab %d/%d  Pane %d", d.TabIdx+1, d.TabCount, d.PaneIdx+1),
		))

	// Shortcuts hint (right-aligned)
	shortcuts := FooterDimStyle.Render("Ctrl+N:new  Ctrl+T:tab  Ctrl+B:files  ?:help")

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
