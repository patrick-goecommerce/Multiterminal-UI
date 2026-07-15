// Package backend provides scheduled task management for Kanban automation.
package backend

import (
	"fmt"
	"log"
	"time"
)

// ScheduledTask represents a recurring automation task.
type ScheduledTask struct {
	ID       string `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Dir      string `json:"dir" yaml:"dir"`
	Prompt   string `json:"prompt" yaml:"prompt"`
	Schedule string `json:"schedule" yaml:"schedule"` // hourly/daily/weekly/cron
	Mode     string `json:"mode" yaml:"mode"`         // claude/claude-yolo
	Model    string `json:"model" yaml:"model"`
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	LastRun  string `json:"last_run" yaml:"last_run"`
	NextRun  string `json:"next_run" yaml:"next_run"`
}

// CreateSchedule adds a new scheduled task.
func (a *AppService) CreateSchedule(dir string, task ScheduledTask) (ScheduledTask, error) {
	state, err := loadKanbanState(dir)
	if err != nil {
		return task, fmt.Errorf("load kanban: %w", err)
	}

	if task.ID == "" {
		task.ID = generateID()
	}
	if task.Dir == "" {
		task.Dir = dir
	}
	task.NextRun = computeNextRun(task.Schedule)

	state.Schedules = append(state.Schedules, task)
	if err := saveKanbanState(dir, state); err != nil {
		return task, fmt.Errorf("save schedule: %w", err)
	}

	log.Printf("[kanban] created schedule %s: %q (%s)", task.ID, task.Name, task.Schedule)
	return task, nil
}

// GetSchedules returns all scheduled tasks for a project.
func (a *AppService) GetSchedules(dir string) []ScheduledTask {
	state, err := loadKanbanState(dir)
	if err != nil {
		return []ScheduledTask{}
	}
	return state.Schedules
}

// UpdateSchedule updates an existing scheduled task.
func (a *AppService) UpdateSchedule(dir string, task ScheduledTask) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for i, s := range state.Schedules {
		if s.ID == task.ID {
			task.NextRun = computeNextRun(task.Schedule)
			state.Schedules[i] = task
			return saveKanbanState(dir, state)
		}
	}
	return fmt.Errorf("schedule %s not found", task.ID)
}

// DeleteSchedule removes a scheduled task.
func (a *AppService) DeleteSchedule(dir string, taskID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for i, s := range state.Schedules {
		if s.ID == taskID {
			state.Schedules = append(state.Schedules[:i], state.Schedules[i+1:]...)
			log.Printf("[kanban] deleted schedule %s", taskID)
			return saveKanbanState(dir, state)
		}
	}
	return fmt.Errorf("schedule %s not found", taskID)
}

// ToggleSchedule enables or disables a scheduled task.
func (a *AppService) ToggleSchedule(dir string, taskID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for i, s := range state.Schedules {
		if s.ID == taskID {
			state.Schedules[i].Enabled = !s.Enabled
			if state.Schedules[i].Enabled {
				state.Schedules[i].NextRun = computeNextRun(s.Schedule)
			} else {
				state.Schedules[i].NextRun = ""
			}
			return saveKanbanState(dir, state)
		}
	}
	return fmt.Errorf("schedule %s not found", taskID)
}

// computeNextRun calculates the next run time based on schedule type.
func computeNextRun(schedule string) string {
	now := time.Now()
	var next time.Time

	switch schedule {
	case "hourly":
		next = now.Add(time.Hour)
	case "daily":
		next = time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, now.Location())
	case "weekly":
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		next = time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 9, 0, 0, 0, now.Location())
	default:
		// For cron or unknown, default to next hour
		next = now.Add(time.Hour)
	}

	return next.Format(time.RFC3339)
}
