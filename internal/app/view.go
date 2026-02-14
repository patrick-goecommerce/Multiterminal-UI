package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/patrick-goecommerce/multiterminal/internal/ui"
)

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

	// Zoom mode: only render the focused pane at full size
	if m.zoomed && tab.FocusIdx >= 0 && tab.FocusIdx < len(tab.Panes) {
		fullRect := ui.Rect{X: 0, Y: 0, Width: areaW, Height: areaH}
		return ui.RenderPane(tab.Panes[tab.FocusIdx], fullRect)
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
