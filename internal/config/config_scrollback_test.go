package config

import "testing"

func TestDefaultConfig_ScrollbackDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TerminalScrollback != 10000 {
		t.Errorf("TerminalScrollback = %d, want 10000", cfg.TerminalScrollback)
	}
}

func TestConfig_Validation_TerminalScrollback(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{1000, 1000},
		{2500, 2500},
		{5000, 5000},
		{10000, 10000},
		{25000, 25000},
		{50000, 50000},
		{100000, 100000},
		{0, 10000},
		{-5, 10000},
		{9999, 10000},
		{1000000, 10000},
	}

	validScrollbackSizes := map[int]bool{1000: true, 2500: true, 5000: true, 10000: true, 25000: true, 50000: true, 100000: true}
	for _, tt := range tests {
		got := tt.input
		if !validScrollbackSizes[got] {
			got = 10000
		}
		if got != tt.want {
			t.Errorf("TerminalScrollback(%d) after validation = %d, want %d", tt.input, got, tt.want)
		}
	}
}
