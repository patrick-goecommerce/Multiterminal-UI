package app

import (
	"github.com/patrick-goecommerce/multiterminal/internal/config"
	"github.com/patrick-goecommerce/multiterminal/internal/ui"
)

// ---------------------------------------------------------------------------
// Session persistence
// ---------------------------------------------------------------------------

// saveSession persists the current tab/pane layout to disk so it can be
// restored on the next launch.
func (m *Model) saveSession() {
	state := config.SessionState{
		ActiveTab: m.tabIdx,
	}
	for _, ts := range m.tabs {
		st := config.SavedTab{
			Name:     ts.Tab.Name,
			Dir:      ts.Tab.Dir,
			FocusIdx: ts.FocusIdx,
		}
		for _, p := range ts.Panes {
			sp := config.SavedPane{
				Name:  p.Name,
				Mode:  int(p.Mode),
				Model: p.Model,
			}
			st.Panes = append(st.Panes, sp)
		}
		state.Tabs = append(state.Tabs, st)
	}
	_ = config.SaveSession(state)
}

// restoreSession attempts to load a saved session and recreate all tabs
// and panes. Returns true if the session was successfully restored.
func (m *Model) restoreSession(fallbackDir string) bool {
	saved := config.LoadSession()
	if saved == nil {
		return false
	}

	for _, st := range saved.Tabs {
		dir := st.Dir
		if dir == "" {
			dir = fallbackDir
		}

		tabIdx := len(m.tabs)
		m.addTab(st.Name, dir)

		// addTab already set the tab name and dir; now launch panes
		for _, sp := range st.Panes {
			mode := ui.PaneMode(sp.Mode)

			// Build argv for the pane
			var argv []string
			var choiceType ui.LaunchType
			switch mode {
			case ui.PaneModeClaudeNormal:
				choiceType = ui.LaunchClaude
			case ui.PaneModeClaudeYOLO:
				choiceType = ui.LaunchClaudeYOLO
			default:
				choiceType = ui.LaunchShell
			}

			if choiceType == ui.LaunchShell {
				argv = nil // uses default shell
			} else {
				// Reconstruct Claude command from config
				cmd := m.cfg.ClaudeCommand
				if cmd == "" {
					cmd = "claude"
				}
				argv = []string{cmd}
				if choiceType == ui.LaunchClaudeYOLO {
					argv = append(argv, "--dangerously-skip-permissions")
				}
				// Find model ID from label
				modelID := m.modelIDFromLabel(sp.Model)
				if modelID != "" {
					argv = append(argv, "--model", modelID)
				}
			}

			choice := ui.LaunchChoice{
				Type:  choiceType,
				Model: sp.Model,
				Argv:  argv,
			}
			m.launchPane(choice)

			// Override the auto-generated name with the saved one
			tab := &m.tabs[tabIdx]
			if len(tab.Panes) > 0 && sp.Name != "" {
				tab.Panes[len(tab.Panes)-1].Name = sp.Name
			}
		}

		// Restore focus index
		tab := &m.tabs[tabIdx]
		if st.FocusIdx >= 0 && st.FocusIdx < len(tab.Panes) {
			tab.FocusIdx = st.FocusIdx
			for i := range tab.Panes {
				tab.Panes[i].Focused = (i == st.FocusIdx)
			}
		}
	}

	// Restore active tab
	if saved.ActiveTab >= 0 && saved.ActiveTab < len(m.tabs) {
		m.tabIdx = saved.ActiveTab
	}

	return len(m.tabs) > 0
}

// modelIDFromLabel looks up the model ID for a given display label.
func (m *Model) modelIDFromLabel(label string) string {
	for _, me := range m.cfg.ClaudeModels {
		if me.Label == label {
			return me.ID
		}
	}
	return ""
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
