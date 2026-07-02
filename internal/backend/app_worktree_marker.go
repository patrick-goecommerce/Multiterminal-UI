// Crash-safe finish phase persistence. Keyed by ABSOLUTE worktree path and
// written ONLY by the backend: session ids do not survive restarts and the
// session JSON is rewritten wholesale by the frontend (spec 4.4).
package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type finishMarker struct {
	Phase        string `json:"phase"` // "merged" | "cleanup"
	Branch       string `json:"branch"`
	TargetBranch string `json:"target_branch"`
}

func finishMarkerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".multiterminal-worktree-finish.json")
}

func loadFinishMarkers(path string) map[string]finishMarker {
	markers := map[string]finishMarker{}
	data, err := os.ReadFile(path)
	if err != nil {
		return markers
	}
	_ = json.Unmarshal(data, &markers)
	return markers
}

func writeFinishMarkers(path string, markers map[string]finishMarker) error {
	if len(markers) == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := json.MarshalIndent(markers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func saveFinishMarker(path, wtPath string, m finishMarker) error {
	markers := loadFinishMarkers(path)
	markers[wtPath] = m
	return writeFinishMarkers(path, markers)
}

func deleteFinishMarker(path, wtPath string) error {
	markers := loadFinishMarkers(path)
	delete(markers, wtPath)
	return writeFinishMarkers(path, markers)
}
