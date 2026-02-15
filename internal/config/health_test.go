package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkStarting_AddsDirtyEntry(t *testing.T) {
	h := HealthState{}
	MarkStarting(&h)

	if len(h.Shutdowns) != 1 {
		t.Fatalf("Shutdowns length = %d, want 1", len(h.Shutdowns))
	}
	if h.Shutdowns[0] != false {
		t.Error("MarkStarting should add a dirty (false) entry")
	}
}

func TestMarkStarting_CapsHistory(t *testing.T) {
	h := HealthState{Shutdowns: []bool{true, true, true, true, true}}
	MarkStarting(&h)

	if len(h.Shutdowns) != maxShutdownHistory {
		t.Errorf("Shutdowns length = %d, want %d", len(h.Shutdowns), maxShutdownHistory)
	}
	// Oldest should have been trimmed, newest is false
	if h.Shutdowns[len(h.Shutdowns)-1] != false {
		t.Error("Last entry should be dirty (false)")
	}
}

func TestMarkCleanShutdown(t *testing.T) {
	h := HealthState{Shutdowns: []bool{false}}
	MarkCleanShutdown(&h)

	if h.Shutdowns[0] != true {
		t.Error("MarkCleanShutdown should set last entry to true")
	}
}

func TestMarkCleanShutdown_IncrementsAutoCounter(t *testing.T) {
	h := HealthState{
		Shutdowns:      []bool{false},
		LoggingAuto:    true,
		CleanSinceAuto: 1,
	}
	MarkCleanShutdown(&h)

	if h.CleanSinceAuto != 2 {
		t.Errorf("CleanSinceAuto = %d, want 2", h.CleanSinceAuto)
	}
}

func TestMarkCleanShutdown_NoIncrementWithoutAuto(t *testing.T) {
	h := HealthState{Shutdowns: []bool{false}, LoggingAuto: false}
	MarkCleanShutdown(&h)

	if h.CleanSinceAuto != 0 {
		t.Errorf("CleanSinceAuto = %d, want 0 (auto not enabled)", h.CleanSinceAuto)
	}
}

func TestHasRepeatedCrashes_TwoDirty(t *testing.T) {
	// Simulate: 2 past dirty shutdowns + current dirty session
	h := HealthState{Shutdowns: []bool{false, false, false}}
	if !HasRepeatedCrashes(&h) {
		t.Error("Should detect repeated crashes with 2 prior dirty shutdowns")
	}
}

func TestHasRepeatedCrashes_OneDirtyOneClean(t *testing.T) {
	h := HealthState{Shutdowns: []bool{true, false, false}}
	if HasRepeatedCrashes(&h) {
		t.Error("Should not detect repeated crashes with only 1 prior dirty")
	}
}

func TestHasRepeatedCrashes_TooFewEntries(t *testing.T) {
	h := HealthState{Shutdowns: []bool{false}}
	if HasRepeatedCrashes(&h) {
		t.Error("Should not detect crashes with only 1 entry")
	}
}

func TestHasRepeatedCrashes_AllClean(t *testing.T) {
	h := HealthState{Shutdowns: []bool{true, true, false}}
	if HasRepeatedCrashes(&h) {
		t.Error("Should not detect crashes when prior shutdowns were clean")
	}
}

func TestShouldAutoDisableLogging(t *testing.T) {
	tests := []struct {
		name   string
		auto   bool
		count  int
		expect bool
	}{
		{"auto off", false, 5, false},
		{"auto on, 2 clean", true, 2, false},
		{"auto on, 3 clean", true, 3, true},
		{"auto on, 5 clean", true, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := HealthState{LoggingAuto: tt.auto, CleanSinceAuto: tt.count}
			if ShouldAutoDisableLogging(&h) != tt.expect {
				t.Errorf("ShouldAutoDisableLogging() = %v, want %v", !tt.expect, tt.expect)
			}
		})
	}
}

func TestEnableAutoLogging(t *testing.T) {
	h := HealthState{CleanSinceAuto: 5}
	EnableAutoLogging(&h)

	if !h.LoggingAuto {
		t.Error("LoggingAuto should be true")
	}
	if h.CleanSinceAuto != 0 {
		t.Errorf("CleanSinceAuto = %d, want 0 (reset)", h.CleanSinceAuto)
	}
}

func TestDisableAutoLogging(t *testing.T) {
	h := HealthState{LoggingAuto: true, CleanSinceAuto: 3}
	DisableAutoLogging(&h)

	if h.LoggingAuto {
		t.Error("LoggingAuto should be false")
	}
	if h.CleanSinceAuto != 0 {
		t.Errorf("CleanSinceAuto = %d, want 0", h.CleanSinceAuto)
	}
}

func TestHealthState_JSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "health.json")

	original := HealthState{
		Shutdowns:      []bool{true, false, true},
		LoggingAuto:    true,
		CleanSinceAuto: 2,
	}

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	readData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded HealthState
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(loaded.Shutdowns) != 3 {
		t.Errorf("Shutdowns length = %d, want 3", len(loaded.Shutdowns))
	}
	if !loaded.LoggingAuto {
		t.Error("LoggingAuto should be true")
	}
	if loaded.CleanSinceAuto != 2 {
		t.Errorf("CleanSinceAuto = %d, want 2", loaded.CleanSinceAuto)
	}
}

func TestFullLifecycle(t *testing.T) {
	h := HealthState{}

	// Session 1: crash
	MarkStarting(&h)
	// No MarkCleanShutdown → dirty

	// Session 2: crash
	MarkStarting(&h)
	// No MarkCleanShutdown → dirty

	// Session 3: starts, detects crashes
	MarkStarting(&h)
	if !HasRepeatedCrashes(&h) {
		t.Error("Should detect repeated crashes after 2 dirty sessions")
	}

	// User enables auto-logging
	EnableAutoLogging(&h)

	// Session 3: clean shutdown
	MarkCleanShutdown(&h)
	if h.CleanSinceAuto != 1 {
		t.Errorf("CleanSinceAuto = %d, want 1", h.CleanSinceAuto)
	}

	// Session 4: clean
	MarkStarting(&h)
	MarkCleanShutdown(&h)
	if h.CleanSinceAuto != 2 {
		t.Errorf("CleanSinceAuto = %d, want 2", h.CleanSinceAuto)
	}
	if ShouldAutoDisableLogging(&h) {
		t.Error("Should not auto-disable after only 2 clean shutdowns")
	}

	// Session 5: clean
	MarkStarting(&h)
	MarkCleanShutdown(&h)
	if h.CleanSinceAuto != 3 {
		t.Errorf("CleanSinceAuto = %d, want 3", h.CleanSinceAuto)
	}
	if !ShouldAutoDisableLogging(&h) {
		t.Error("Should auto-disable after 3 clean shutdowns")
	}

	// Disable auto-logging
	DisableAutoLogging(&h)
	if h.LoggingAuto {
		t.Error("LoggingAuto should be false after disable")
	}
}
