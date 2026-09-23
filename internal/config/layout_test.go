package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayout_DefaultsToGridAndUnmovedWindow(t *testing.T) {
	l := DefaultConfig().Layout
	if l.Mode != LayoutGrid {
		t.Errorf("Mode = %q, want %q", l.Mode, LayoutGrid)
	}
	if l.FloatX >= 0 || l.FloatY >= 0 {
		t.Errorf("float position = (%d, %d), want negative (never moved)", l.FloatX, l.FloatY)
	}
}

func TestLayout_UnknownModeFallsBackToGrid(t *testing.T) {
	for _, mode := range []string{"", "Focus", "tiles"} {
		l := LayoutSettings{Mode: mode}
		normalizeLayout(&l)
		if l.Mode != LayoutGrid {
			t.Errorf("mode %q normalized to %q, want %q", mode, l.Mode, LayoutGrid)
		}
	}
	l := LayoutSettings{Mode: LayoutFocus}
	normalizeLayout(&l)
	if l.Mode != LayoutFocus {
		t.Errorf("focus normalized to %q", l.Mode)
	}
}

// A config file written before the field existed must not read as "the
// floating window was dragged to the top left corner".
func TestLayout_OldConfigKeepsTheDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.WriteFile(filepath.Join(home, ".multiterminal.yaml"), []byte("theme: nord\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := Load().Layout
	if l.Mode != LayoutGrid || l.FloatX >= 0 || l.FloatY >= 0 {
		t.Errorf("Layout = %+v, want grid and a negative float position", l)
	}
}

// The focus layout's order travels with the tab in the session file, so a
// restart keeps which panes were big.
func TestSavedTabRoundTripsFocusOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	state := SessionState{Tabs: []SavedTab{
		{Name: "A", Panes: []SavedPane{{Name: "p0"}, {Name: "p1"}, {Name: "p2"}}, FocusOrder: []int{2, 0, 1}},
		{Name: "B", Panes: []SavedPane{{Name: "q0"}}},
	}}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}
	got := loadSessionFrom(path)
	if got == nil || len(got.Tabs) != 2 {
		t.Fatalf("loaded %+v", got)
	}
	if o := got.Tabs[0].FocusOrder; len(o) != 3 || o[0] != 2 || o[1] != 0 || o[2] != 1 {
		t.Errorf("FocusOrder = %v, want [2 0 1]", o)
	}
	if got.Tabs[1].FocusOrder != nil {
		t.Errorf("a tab without an order must stay without one, got %v", got.Tabs[1].FocusOrder)
	}
}
