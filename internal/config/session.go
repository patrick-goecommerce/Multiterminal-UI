// Package config – session state persistence.
//
// Saves and restores the user's tab/pane layout between runs so they can
// pick up exactly where they left off.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SessionState is the top-level structure serialised to disk.
type SessionState struct {
	ActiveTab int        `json:"active_tab"`
	Tabs      []SavedTab `json:"tabs"`
}

// SavedTab captures a single tab's layout.
type SavedTab struct {
	Name     string      `json:"name"`
	Dir      string      `json:"dir"`
	FocusIdx int         `json:"focus_idx"`
	Panes    []SavedPane `json:"panes"`
}

// SavedPane captures enough information to re-launch a single pane.
type SavedPane struct {
	Name        string `json:"name"`
	Mode        int    `json:"mode"`                   // maps to ui.PaneMode (0=shell, 1=claude, 2=yolo)
	Model       string `json:"model"`                  // model label (empty for shell)
	IssueNumber int    `json:"issue_number,omitempty"` // linked GitHub issue number
	IssueBranch string `json:"issue_branch,omitempty"` // branch created for issue
	ZoomDelta   int    `json:"zoom_delta,omitempty"`   // per-pane font zoom offset
}

// sessionPath returns the path to ~/.multiterminal-session.json.
func sessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".multiterminal-session.json")
}

// SaveSession writes the session state to disk.
func SaveSession(state SessionState) error {
	p := sessionPath()
	if p == "" {
		return nil
	}
	return saveSessionTo(p, state)
}

// LoadSession reads a previously saved session state from disk.
// Returns nil if no session file exists or it cannot be parsed.
func LoadSession() *SessionState {
	p := sessionPath()
	if p == "" {
		return nil
	}
	return loadSessionFrom(p)
}

// ClearSession removes the session file from disk.
func ClearSession() {
	p := sessionPath()
	if p != "" {
		os.Remove(p)
	}
}

// RemoveTab removes the first tab matching name from the session file.
// Returns (true, nil) if found and removed, (false, nil) if not found,
// or (false, err) on read/write failure.
func RemoveTab(name string) (bool, error) {
	return removeTabFrom(sessionPath(), name)
}

// removeTabFrom is the testable core, operating on an explicit path.
func removeTabFrom(path string, name string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return false, err
	}

	idx := -1
	for i, t := range state.Tabs {
		if t.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, nil
	}

	state.Tabs = append(state.Tabs[:idx], state.Tabs[idx+1:]...)
	if state.ActiveTab >= len(state.Tabs) && len(state.Tabs) > 0 {
		state.ActiveTab = len(state.Tabs) - 1
	}

	return true, saveSessionTo(path, state)
}

// saveSessionTo is the testable core for SaveSession.
func saveSessionTo(path string, state SessionState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// loadSessionFrom is the testable core for LoadSession.
func loadSessionFrom(path string) *SessionState {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	if len(state.Tabs) == 0 {
		return nil
	}
	return &state
}
