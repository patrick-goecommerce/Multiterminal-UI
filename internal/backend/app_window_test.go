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

