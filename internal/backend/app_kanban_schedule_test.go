package backend

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// computeNextRun — scheduling calculation
// ---------------------------------------------------------------------------

func TestComputeNextRun_Hourly(t *testing.T) {
	result := computeNextRun("hourly")
	ts, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// Should be roughly 1 hour from now (within 5 seconds tolerance)
	diff := time.Until(ts)
	if diff < 59*time.Minute || diff > 61*time.Minute {
		t.Errorf("hourly next run should be ~1h from now, got %v", diff)
	}
}

func TestComputeNextRun_Daily(t *testing.T) {
	result := computeNextRun("daily")
	ts, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// Should be tomorrow at 09:00
	if ts.Hour() != 9 || ts.Minute() != 0 {
		t.Errorf("daily should be at 09:00, got %02d:%02d", ts.Hour(), ts.Minute())
	}
	// Should be in the future
	if !ts.After(time.Now()) {
		t.Error("daily next run should be in the future")
	}
}

func TestComputeNextRun_Weekly(t *testing.T) {
	result := computeNextRun("weekly")
	ts, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// Should be a Monday at 09:00
	if ts.Weekday() != time.Monday {
		t.Errorf("weekly should be on Monday, got %s", ts.Weekday())
	}
	if ts.Hour() != 9 {
		t.Errorf("weekly should be at 09:00, got %02d:00", ts.Hour())
	}
	if !ts.After(time.Now()) {
		t.Error("weekly next run should be in the future")
	}
}

func TestComputeNextRun_Unknown(t *testing.T) {
	// Unknown schedule type defaults to hourly behavior
	result := computeNextRun("unknown")
	ts, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	diff := time.Until(ts)
	if diff < 59*time.Minute || diff > 61*time.Minute {
		t.Errorf("unknown schedule should default to ~1h, got %v", diff)
	}
}

// ---------------------------------------------------------------------------
// Schedule CRUD
// ---------------------------------------------------------------------------

func TestCreateSchedule(t *testing.T) {
	dir := t.TempDir()
	saveKanbanState(dir, newKanbanState())

	app := newTestApp()
	task, err := app.CreateSchedule(dir, ScheduledTask{
		Name:     "Daily test",
		Prompt:   "Run tests",
		Schedule: "daily",
		Mode:     "claude",
	})
	if err != nil {
		t.Fatalf("create schedule error: %v", err)
	}
	if task.ID == "" {
		t.Error("schedule ID should be auto-generated")
	}
	if task.NextRun == "" {
		t.Error("next_run should be computed")
	}

	loaded, _ := loadKanbanState(dir)
	if len(loaded.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(loaded.Schedules))
	}
}

func TestDeleteSchedule(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Schedules = []ScheduledTask{{ID: "s1", Name: "T"}, {ID: "s2", Name: "T2"}}
	saveKanbanState(dir, state)

	app := newTestApp()
	if err := app.DeleteSchedule(dir, "s1"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if len(loaded.Schedules) != 1 || loaded.Schedules[0].ID != "s2" {
		t.Error("wrong schedule deleted")
	}
}

func TestToggleSchedule(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Schedules = []ScheduledTask{{ID: "s1", Schedule: "hourly", Enabled: false}}
	saveKanbanState(dir, state)

	app := newTestApp()
	if err := app.ToggleSchedule(dir, "s1"); err != nil {
		t.Fatalf("toggle error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if !loaded.Schedules[0].Enabled {
		t.Error("schedule should be enabled after toggle")
	}
	if loaded.Schedules[0].NextRun == "" {
		t.Error("next_run should be set when enabled")
	}

	// Toggle again → disable
	if err := app.ToggleSchedule(dir, "s1"); err != nil {
		t.Fatalf("toggle error: %v", err)
	}
	loaded, _ = loadKanbanState(dir)
	if loaded.Schedules[0].Enabled {
		t.Error("schedule should be disabled after second toggle")
	}
	if loaded.Schedules[0].NextRun != "" {
		t.Error("next_run should be empty when disabled")
	}
}
