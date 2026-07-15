package main

import (
	"fmt"
	"testing"
)

func TestParseStatusExtractsFields(t *testing.T) {
	raw := []byte(`{"model":{"display_name":"Opus 4.8"},"context_window":{"used_percentage":45.7},"cost":{"total_cost_usd":0.42,"total_duration_ms":65000},"workspace":{"current_dir":"/p"}}`)
	s := parseStatus(raw)
	if s.Model != "Opus 4.8" {
		t.Fatalf("model %q", s.Model)
	}
	if s.ContextPct == nil || *s.ContextPct != 46 { // [int] rounds 45.7 -> 46
		t.Fatalf("ctx %v want 46", s.ContextPct)
	}
	if s.CostUSD == nil || *s.CostUSD != 0.42 {
		t.Fatalf("cost %v", s.CostUSD)
	}
	if s.DurationMs == nil || *s.DurationMs != 65000 {
		t.Fatalf("dur %v", s.DurationMs)
	}
	if s.CurrentDir != "/p" {
		t.Fatalf("dir %q", s.CurrentDir)
	}
}

func TestParseStatusMissingFieldsAreNil(t *testing.T) {
	s := parseStatus([]byte(`{}`))
	if s.ContextPct != nil || s.CostUSD != nil || s.DurationMs != nil {
		t.Fatalf("expected nil absent fields: %+v", s)
	}
}

func TestParseStatusRoundsHalfToEven(t *testing.T) {
	// PowerShell [int] uses round-half-to-even: 44.5 -> 44 (even), 45.5 -> 46 (even).
	for in, want := range map[float64]int{44.5: 44, 45.5: 46} {
		raw := []byte(fmt.Sprintf(`{"context_window":{"used_percentage":%v}}`, in))
		s := parseStatus(raw)
		if s.ContextPct == nil || *s.ContextPct != want {
			t.Fatalf("used_percentage %v -> %v, want %d", in, s.ContextPct, want)
		}
	}
}
