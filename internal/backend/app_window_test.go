package backend

import (
	"testing"
)

func TestWindowManagerRegisterUnregister(t *testing.T) {
	wm := newWindowManager(nil)

	wm.register("win1", nil, []string{"tab1", "tab2"})
	if len(wm.windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(wm.windows))
	}

	wm.unregister("win1")
	if len(wm.windows) != 0 {
		t.Fatalf("expected 0 windows after unregister")
	}
}

func TestWindowManagerGetTabWindow(t *testing.T) {
	wm := newWindowManager(nil)
	wm.register("win1", nil, []string{"tab-a", "tab-b"})

	winID := wm.getWindowForTab("tab-a")
	if winID != "win1" {
		t.Errorf("expected win1, got %q", winID)
	}
	winID = wm.getWindowForTab("unknown")
	if winID != "" {
		t.Errorf("expected empty for unknown tab, got %q", winID)
	}
}

func TestWindowManagerMoveTab(t *testing.T) {
	wm := newWindowManager(nil)
	wm.register("win1", nil, []string{"tab-a", "tab-b"})
	wm.register("win2", nil, []string{"tab-c"})

	wm.moveTab("tab-a", "win2")

	if wm.getWindowForTab("tab-a") != "win2" {
		t.Error("tab-a should now belong to win2")
	}
	entry := wm.windows["win1"]
	for _, id := range entry.TabIDs {
		if id == "tab-a" {
			t.Error("win1 should not contain tab-a anymore")
		}
	}
}
