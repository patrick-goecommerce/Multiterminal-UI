package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Session files written before refs carried a hub have a bare number in
// session_id. Refusing them would drop the whole saved layout; loading them as
// "7" lets the restore match the pane to its still-running session.
func TestLoadSession_AcceptsLegacyNumericSessionID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	legacy := `{"active_tab":0,"tabs":[{"name":"t","dir":"/p","panes":[` +
		`{"name":"a","mode":1,"session_id":7},` +
		`{"name":"b","mode":1,"session_id":0},` +
		`{"name":"c","mode":1,"session_id":"h1:9"},` +
		`{"name":"d","mode":1}]}]}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	state := loadSessionFrom(path)
	if state == nil || len(state.Tabs) != 1 {
		t.Fatalf("legacy session file did not load: %+v", state)
	}
	var got []PaneSessionRef
	for _, p := range state.Tabs[0].Panes {
		got = append(got, p.SessionID)
	}
	want := []PaneSessionRef{"7", "", "h1:9", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pane %d session_id = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSaveSession_WritesTheRefAsAString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	state := SessionState{Tabs: []SavedTab{{Name: "t", Panes: []SavedPane{{Name: "a", SessionID: "h1:3"}}}}}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}
	back := loadSessionFrom(path)
	if back == nil || back.Tabs[0].Panes[0].SessionID != "h1:3" {
		t.Errorf("round trip lost the ref: %+v", back)
	}
}
