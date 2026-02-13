package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/patrick-goecommerce/multiterminal/internal/terminal"
)

// PaneMode describes what kind of process a pane is running.
type PaneMode int

const (
	PaneModeShell PaneMode = iota // plain shell
	PaneModeClaudeNormal          // claude (normal)
	PaneModeClaudeYOLO            // claude --dangerously-skip-permissions
)

// PaneInfo holds the display metadata for a single terminal pane.
type PaneInfo struct {
	Session *terminal.Session
	Name    string   // user-assigned name
	Mode    PaneMode // what was launched
	Model   string   // Claude model ID (empty for shell)
	Branch  string   // git branch (updated periodically)
	Focused bool
}

// RenderPane draws a single terminal pane with its border, title bar and
// terminal content, sized to fit the given Rect.
func RenderPane(p PaneInfo, rect Rect) string {
	if rect.Width < 4 || rect.Height < 3 {
		return ""
	}

	// Choose border style based on focus
	border := PaneBorderUnfocused
	if p.Focused {
		border = PaneBorderFocused
	}

	// Build title line: name + mode indicator + status dot
	title := buildPaneTitle(p)

	// Inner dimensions (border takes 2 cols and 2 rows)
	innerW := rect.Width - 2
	innerH := rect.Height - 3 // -2 border, -1 title

	if innerW < 1 || innerH < 1 {
		return border.Width(rect.Width).Height(rect.Height).Render("")
	}

	// Render terminal content from the screen buffer
	content := renderScreenContent(p.Session, innerW, innerH)

	// Compose: title on top, content below
	titleLine := lipgloss.NewStyle().
		Width(innerW).
		MaxWidth(innerW).
		Render(title)

	body := titleLine + "\n" + content

	return border.
		Width(rect.Width).
		Height(rect.Height).
		Render(body)
}

// buildPaneTitle creates the title string shown at the top of a pane.
func buildPaneTitle(p PaneInfo) string {
	// Status indicator
	var statusDot string
	if p.Session != nil && p.Session.IsRunning() {
		statusDot = PaneStatusRunning.Render("●")
	} else {
		statusDot = PaneStatusExited.Render("●")
	}

	// Mode label
	var modeLabel string
	switch p.Mode {
	case PaneModeClaudeNormal:
		modeLabel = " [Claude]"
	case PaneModeClaudeYOLO:
		modeLabel = " [YOLO]"
	default:
		modeLabel = " [Shell]"
	}

	name := p.Name
	if name == "" {
		name = fmt.Sprintf("Pane %d", p.Session.ID)
	}

	// Model info (only for Claude modes)
	var modelInfo string
	if p.Model != "" && p.Mode != PaneModeShell {
		modelInfo = " (" + p.Model + ")"
	}

	return statusDot + " " + PaneTitleStyle.Render(name+modeLabel+modelInfo)
}

// renderScreenContent extracts the visible portion of the terminal screen
// buffer and returns it as a string, constrained to w×h.
func renderScreenContent(sess *terminal.Session, w, h int) string {
	if sess == nil {
		return strings.Repeat("\n", h-1)
	}

	screenRows := sess.Screen.Rows()
	screenCols := sess.Screen.Cols()

	// Determine which rows to show (bottom-aligned – show the latest output)
	startRow := 0
	if screenRows > h {
		startRow = screenRows - h
	}
	endRow := startRow + h - 1
	if endRow >= screenRows {
		endRow = screenRows - 1
	}

	endCol := screenCols - 1
	if endCol >= w {
		endCol = w - 1
	}

	return sess.Screen.RenderRegion(startRow, 0, endRow, endCol)
}
