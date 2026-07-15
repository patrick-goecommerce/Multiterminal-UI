// Package backend provides the runtime loop for scheduled task execution.
package backend

import (
	"context"
	"log"
	"os"
	"time"
)

// scheduleLoop periodically checks for due scheduled tasks and executes them.
// Runs every 30 seconds to minimize overhead.
func (a *AppService) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.checkSchedules()
		}
	}
}

// checkSchedules scans all known project directories for due scheduled tasks.
func (a *AppService) checkSchedules() {
	// Collect unique project directories from active sessions
	a.mu.Lock()
	dirs := make(map[string]bool)
	for _, sess := range a.sessions {
		if sess.Dir != "" {
			dirs[sess.Dir] = true
		}
	}
	a.mu.Unlock()

	now := time.Now()
	for dir := range dirs {
		a.runDueSchedules(dir, now)
	}
}

// runDueSchedules checks and executes any due schedules for a project directory.
func (a *AppService) runDueSchedules(dir string, now time.Time) {
	state, err := loadKanbanState(dir)
	if err != nil {
		return
	}

	changed := false
	for i, task := range state.Schedules {
		if !task.Enabled || task.NextRun == "" {
			continue
		}

		nextRun, err := time.Parse(time.RFC3339, task.NextRun)
		if err != nil {
			log.Printf("[scheduler] invalid next_run for task %s: %v", task.ID, err)
			continue
		}

		if now.Before(nextRun) {
			continue
		}

		// Task is due — execute it
		log.Printf("[scheduler] executing task %q (id=%s, schedule=%s)", task.Name, task.ID, task.Schedule)
		a.executeScheduledTask(task, dir)

		// Update last_run and compute next_run
		state.Schedules[i].LastRun = now.Format(time.RFC3339)
		state.Schedules[i].NextRun = computeNextRun(task.Schedule)
		changed = true
	}

	if changed {
		if err := saveKanbanState(dir, state); err != nil {
			log.Printf("[scheduler] save error for %s: %v", dir, err)
		}
		// Notify frontend about schedule updates
		if a.app != nil {
			a.app.Event.Emit("kanban:schedules_updated", map[string]string{"dir": dir})
		}
	}
}

// executeScheduledTask launches a new Claude session for the scheduled task.
func (a *AppService) executeScheduledTask(task ScheduledTask, dir string) {
	taskDir := task.Dir
	if taskDir == "" {
		taskDir = dir
	}
	// Verify directory exists
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		log.Printf("[scheduler] dir %s does not exist, skipping task %s", taskDir, task.ID)
		return
	}

	mode := task.Mode
	if mode == "" {
		mode = "claude"
	}

	// Build argv based on mode
	var argv []string
	switch mode {
	case "claude-yolo":
		path := a.resolvedClaudePath
		if path == "" {
			path = "claude"
		}
		argv = []string{path, "--dangerously-skip-permissions"}
	default:
		path := a.resolvedClaudePath
		if path == "" {
			path = "claude"
		}
		argv = []string{path}
	}

	if task.Model != "" {
		argv = append(argv, "--model", task.Model)
	}

	sessionID := a.CreateSession(argv, taskDir, 24, 80, mode)
	if sessionID < 0 {
		log.Printf("[scheduler] failed to create session for task %s", task.ID)
		return
	}

	// Send the prompt after startup delay
	go func() {
		time.Sleep(2 * time.Second)
		a.mu.Lock()
		sess := a.sessions[sessionID]
		a.mu.Unlock()
		if sess != nil {
			sess.Write([]byte(task.Prompt + "\r"))
			log.Printf("[scheduler] sent prompt to session %d for task %q", sessionID, task.Name)
		}
	}()
}
