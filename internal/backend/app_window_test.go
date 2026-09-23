package backend

import (
	"encoding/json"
	"testing"
)

func TestWindowManagerRegisterUnregister(t *testing.T) {
	wm := newWindowManager(nil)

	wm.register("win1", nil, []string{"tab1", "tab2"}, "")
	if len(wm.windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(wm.windows))
	}

	wm.unregister("win1")
	if len(wm.windows) != 0 {
		t.Fatalf("expected 0 windows after unregister")
	}
}

// A detached window that closes before its first SaveWindowTabs (300 ms after
// its store changes) still has a state to hand back, in the shape the main
// window's merge handler reads.
func TestWindowStateOf_WrapsOneTab(t *testing.T) {
	got := windowStateOf(`{"id":"tab-3","panes":[{"sessionId":"h1:4"}]}`)
	var parsed struct {
		Tabs []struct {
			ID string `json:"id"`
		} `json:"tabs"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("state %q does not parse: %v", got, err)
	}
	if len(parsed.Tabs) != 1 || parsed.Tabs[0].ID != "tab-3" {
		t.Errorf("state = %s, want one tab tab-3", got)
	}
	for _, bad := range []string{"", "not json", "[1,2]", `"x"`} {
		if s := windowStateOf(bad); s != "" {
			t.Errorf("windowStateOf(%q) = %q, want empty", bad, s)
		}
	}
}

func TestWindowManager_RegisterSeedsTheState(t *testing.T) {
	wm := newWindowManager(nil)
	wm.register("win-1", nil, []string{"tab-3"}, `{"tabs":[]}`)
	if got := wm.windows["win-1"].tabStateJSON; got != `{"tabs":[]}` {
		t.Errorf("seeded state = %q", got)
	}
}

func TestWindowManager_OthersExcludesTheNamedWindow(t *testing.T) {
	wm := newWindowManager(nil)
	wm.register("main", nil, nil, "")
	if n := wm.others("main"); n != 0 {
		t.Errorf("others(main) with only main = %d, want 0", n)
	}
	wm.register("win-1", nil, nil, "")
	wm.register("win-2", nil, nil, "")
	if n := wm.others("main"); n != 2 {
		t.Errorf("others(main) = %d, want 2", n)
	}
}
