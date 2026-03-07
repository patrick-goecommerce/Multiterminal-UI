package backend

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// runDueSchedules — detects and marks due tasks
// ---------------------------------------------------------------------------

func TestRunDueSchedules_ExecutesDueTask(t *testing.T) {
	dir := t.TempDir()
	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	state := newKanbanState()
	state.Schedules = []ScheduledTask{
		{
			ID:       "s1",
			Name:     "Due task",
			Prompt:   "Run tests",
			Schedule: "hourly",
			Mode:     "claude",
			Enabled:  true,
			NextRun:  pastTime,
		},
	}
	saveKanbanState(dir, state)

	app := newTestApp()
	// Will try to CreateSession which returns -1 (no PTY), but
	// the schedule state should still be updated.
	app.runDueSchedules(dir, time.Now())

	loaded, _ := loadKanbanState(dir)
	if loaded.Schedules[0].LastRun == "" {
		t.Error("last_run should be set after execution")
	}
	// NextRun should be recalculated (not the old past time)
	nextRun, err := time.Parse(time.RFC3339, loaded.Schedules[0].NextRun)
	if err != nil {
		t.Fatalf("parse next_run: %v", err)
	}
	if !nextRun.After(time.Now()) {
		t.Error("next_run should be in the future")
	}
}

func TestRunDueSchedules_SkipsDisabled(t *testing.T) {
	dir := t.TempDir()
	pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	state := newKanbanState()
	state.Schedules = []ScheduledTask{
		{ID: "s1", Enabled: false, NextRun: pastTime, Schedule: "hourly"},
	}
	saveKanbanState(dir, state)

	app := newTestApp()
	app.runDueSchedules(dir, time.Now())

	loaded, _ := loadKanbanState(dir)
	if loaded.Schedules[0].LastRun != "" {
		t.Error("disabled task should not have been executed")
	}
}

func TestRunDueSchedules_SkipsFuture(t *testing.T) {
	dir := t.TempDir()
	futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	state := newKanbanState()
	state.Schedules = []ScheduledTask{
		{ID: "s1", Enabled: true, NextRun: futureTime, Schedule: "hourly"},
	}
	saveKanbanState(dir, state)

	app := newTestApp()
	app.runDueSchedules(dir, time.Now())

	loaded, _ := loadKanbanState(dir)
	if loaded.Schedules[0].LastRun != "" {
		t.Error("future task should not have been executed")
	}
}

func TestRunDueSchedules_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	// No kanban.json exists — should not panic
	app := newTestApp()
	app.runDueSchedules(dir, time.Now())
}
