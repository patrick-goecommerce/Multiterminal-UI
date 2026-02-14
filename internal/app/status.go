package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/patrick-goecommerce/multiterminal/internal/terminal"
	"github.com/patrick-goecommerce/multiterminal/internal/ui"
)

// ---------------------------------------------------------------------------
// Layout & resize
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

	// Zoom mode: give the focused pane the full content area
	if m.zoomed && tab.FocusIdx >= 0 && tab.FocusIdx < len(tab.Panes) {
		p := tab.Panes[tab.FocusIdx]
		innerW := contentW - 2
		innerH := contentH - 3
		if innerW < 1 {
			innerW = 1
		}
		if innerH < 1 {
			innerH = 1
		}
		if p.Session != nil {
			p.Session.Resize(innerH, innerW)
		}
		return
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

// ---------------------------------------------------------------------------
// Git helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Claude activity & token scanning
// ---------------------------------------------------------------------------

// scanClaudePanes checks all Claude panes for activity changes and token info.
// When Claude finishes or needs input, the pane border flashes.
func (m *Model) scanClaudePanes() {
	for ti := range m.tabs {
		for pi := range m.tabs[ti].Panes {
			p := &m.tabs[ti].Panes[pi]
			if p.Session == nil {
				continue
			}
			if p.Mode != ui.PaneModeClaudeNormal && p.Mode != ui.PaneModeClaudeYOLO {
				continue
			}

			// Detect activity state changes
			state := p.Session.DetectActivity()
			switch state {
			case terminal.ActivityDone:
				// Flash green for 3 seconds
				if time.Now().After(p.FlashUntil) {
					p.FlashUntil = time.Now().Add(3 * time.Second)
					p.FlashColor = ui.ColorSuccess
					p.Session.ResetActivity()
				}
			case terminal.ActivityNeedsInput:
				// Flash yellow/warning until user interacts
				if time.Now().After(p.FlashUntil) {
					p.FlashUntil = time.Now().Add(5 * time.Second)
					p.FlashColor = ui.ColorWarning
					p.Session.ResetActivity()
				}
			}

			// Update token cost display per pane
			if p.Session.Tokens.TotalCost > 0 {
				p.TokenCost = fmt.Sprintf("$%.2f", p.Session.Tokens.TotalCost)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Commit reminder
// ---------------------------------------------------------------------------

// checkCommitReminder checks if it's time to show a commit reminder.
func (m *Model) checkCommitReminder() {
	if m.cfg.CommitReminderMinutes <= 0 {
		m.commitReminder = ""
		return
	}

	// Only check every 30 seconds to avoid hammering git
	if time.Since(m.lastCommitCheck) < 30*time.Second {
		return
	}
	m.lastCommitCheck = time.Now()

	dir := m.currentDir()
	lastCommit := gitLastCommitTime(dir)

	if lastCommit.IsZero() {
		m.commitReminder = ""
		return
	}

	m.lastCommitTime = lastCommit
	elapsed := time.Since(lastCommit)
	threshold := time.Duration(m.cfg.CommitReminderMinutes) * time.Minute

	if elapsed >= threshold {
		mins := int(elapsed.Minutes())
		m.commitReminder = fmt.Sprintf("COMMIT REMINDER: %dm since last commit!", mins)
	} else {
		m.commitReminder = ""
	}
}

// gitLastCommitTime returns the time of the last git commit in the given dir.
func gitLastCommitTime(dir string) time.Time {
	cmd := exec.Command("git", "-C", dir, "log", "-1", "--format=%ct")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}
	}
	ts := strings.TrimSpace(string(out))
	if ts == "" {
		return time.Time{}
	}
	var sec int64
	_, err = fmt.Sscanf(ts, "%d", &sec)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// currentDir returns the working directory of the active tab.
func (m *Model) currentDir() string {
	tab := m.activeTab()
	if tab != nil && tab.Tab.Dir != "" {
		return tab.Tab.Dir
	}
	dir, _ := os.Getwd()
	return dir
}

// ---------------------------------------------------------------------------
// Footer
// ---------------------------------------------------------------------------

// footerData assembles the data needed to render the footer.
func (m *Model) footerData() ui.FooterData {
	d := ui.FooterData{
		TabCount:       len(m.tabs),
		TabIdx:         m.tabIdx,
		ThemeName:      ui.ActiveTheme.Name,
		Zoomed:         m.zoomed,
		CommitReminder: m.commitReminder,
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

	// Calculate total cost across all Claude panes in all tabs
	var totalCost float64
	for _, ts := range m.tabs {
		for _, p := range ts.Panes {
			if p.Session != nil && (p.Mode == ui.PaneModeClaudeNormal || p.Mode == ui.PaneModeClaudeYOLO) {
				p.Session.ScanTokens()
				totalCost += p.Session.Tokens.TotalCost
			}
		}
	}
	if totalCost > 0 {
		d.TotalCost = fmt.Sprintf("$%.2f", totalCost)
	}

	return d
}
