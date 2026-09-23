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
