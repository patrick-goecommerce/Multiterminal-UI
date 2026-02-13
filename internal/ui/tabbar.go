package ui

import (
	"fmt"
	"strings"
)

// Tab holds the metadata for a single workspace tab.
type Tab struct {
	Name string // display name (user-editable)
	Dir  string // working directory for all panes in this tab
}

// RenderTabBar produces the tab bar string for the top of the screen.
// activeIdx is the currently selected tab index.
func RenderTabBar(tabs []Tab, activeIdx, width int) string {
	var parts []string

	for i, t := range tabs {
		label := t.Name
		if label == "" {
			label = fmt.Sprintf("Tab %d", i+1)
		}
		// Prefix with 1-indexed number for keyboard shortcut hint
		display := fmt.Sprintf(" %d: %s ", i+1, label)

		if i == activeIdx {
			parts = append(parts, TabActive.Render(display))
		} else {
			parts = append(parts, TabInactive.Render(display))
		}
	}

	// "+" button to add a new tab
	parts = append(parts, TabAdd.Render(" + "))

	bar := strings.Join(parts, " ")

	// Pad to full width
	return TabBarStyle.Width(width).Render(bar)
}
