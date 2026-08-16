package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig_ScrollbackDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TerminalScrollback != DefaultTerminalScrollback {
		t.Errorf("TerminalScrollback = %d, want %d", cfg.TerminalScrollback, DefaultTerminalScrollback)
	}
	if !validScrollbackSizes[cfg.TerminalScrollback] {
		t.Errorf("the default %d is not among the offered sizes", cfg.TerminalScrollback)
	}
}

// The scrollback buffer is held per pane in the WebView heap, so the default
// must stay well below the largest offered size — otherwise a handful of panes
// dominates the app's memory before the user has configured anything.
func TestDefaultConfig_ScrollbackIsNotTheLargestOption(t *testing.T) {
	largest := 0
	for size := range validScrollbackSizes {
		if size > largest {
			largest = size
		}
	}
	if DefaultTerminalScrollback >= largest {
		t.Errorf("default %d is the largest offered size (%d); pick a smaller one", DefaultTerminalScrollback, largest)
	}
}

// Validation runs inside Load(), so exercise it through Load() rather than
// re-implementing the rule in the test — a copied rule passes even when the
// real one is broken.
func TestConfig_Validation_TerminalScrollback(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"smallest offered", 1000, 1000},
		{"offered", 2500, 2500},
		{"offered", 5000, 5000},
		{"offered", 10000, 10000},
		{"largest offered", 100000, 100000},
		{"missing", 0, DefaultTerminalScrollback},
		{"negative", -5, DefaultTerminalScrollback},
		{"not offered", 9999, DefaultTerminalScrollback},
		{"absurd", 1000000, DefaultTerminalScrollback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Setenv("USERPROFILE", dir)

			body, err := yaml.Marshal(map[string]int{"terminal_scrollback": tt.input})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, ".multiterminal.yaml"), body, 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			got := Load().TerminalScrollback
			if got != tt.want {
				t.Errorf("terminal_scrollback %d loaded as %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
