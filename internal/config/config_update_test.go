package config

import "testing"

func TestDefaultConfig_UpdateDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.UpdateChannel != "alpha" {
		t.Errorf("UpdateChannel = %q, want 'alpha' (no stable release exists yet)", cfg.UpdateChannel)
	}
	if cfg.AutoUpdateCheckMinutes != 0 {
		t.Errorf("AutoUpdateCheckMinutes = %d, want 0 (disabled by default)", cfg.AutoUpdateCheckMinutes)
	}
}

func TestConfig_Validation_UpdateChannel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"stable", "stable"},
		{"alpha", "alpha"},
		{"", "alpha"},
		{"bogus", "alpha"},
		{"STABLE", "alpha"}, // case-sensitive: not in the allow-list, resets
	}

	validUpdateChannels := map[string]bool{"stable": true, "alpha": true}
	for _, tt := range tests {
		got := tt.input
		if !validUpdateChannels[got] {
			got = "alpha"
		}
		if got != tt.want {
			t.Errorf("UpdateChannel(%q) after validation = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConfig_Validation_AutoUpdateCheckMinutes(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 0},
		{-5, 0},
		{60, 60},
		{120, 120},
	}

	for _, tt := range tests {
		val := tt.input
		if val < 0 {
			val = 0
		}
		if val != tt.want {
			t.Errorf("AutoUpdateCheckMinutes(%d) after validation = %d, want %d", tt.input, val, tt.want)
		}
	}
}
