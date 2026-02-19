package backend

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ActivityInfo is sent to the frontend when a session's activity state changes.
type ActivityInfo struct {
	ID       int    `json:"id"`
	Activity string `json:"activity"` // "idle", "active", "done", "needsInput"
	Cost     string `json:"cost"`
}

// prevActivity tracks the last emitted state per session to avoid spamming.
var (
	prevActivityMu sync.Mutex
	prevActivity   = make(map[int]string)
	prevCost       = make(map[int]string)
)

// scanInterval returns the scan tick duration based on the number of active sessions.
// More sessions → slower ticks to reduce overhead.
func (a *App) scanInterval() time.Duration {
	a.mu.Lock()
	n := len(a.sessions)
	a.mu.Unlock()
	switch {
	case n <= 3:
		return 500 * time.Millisecond
	case n <= 6:
		return 600 * time.Millisecond
	default:
		return 750 * time.Millisecond
	}
}

// scanLoop periodically scans all sessions for activity changes and token info.
// The interval adapts to the number of active sessions.
func (a *App) scanLoop(ctx context.Context) {
	interval := a.scanInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.scanAllSessions()
			// Re-check if interval should change
			if newInterval := a.scanInterval(); newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
			}
		}
	}
}

func activityString(a terminal.ActivityState) string {
	switch a {
	case terminal.ActivityActive:
		return "active"
	case terminal.ActivityDone:
		return "done"
	case terminal.ActivityNeedsInput:
		return "needsInput"
	default:
		return "idle"
	}
}

// cleanupActivityTracking removes stale tracking data for a closed session.
func cleanupActivityTracking(id int) {
	prevActivityMu.Lock()
	delete(prevActivity, id)
	delete(prevCost, id)
	prevActivityMu.Unlock()
}

// scanAllSessions checks each session for activity and token updates.
func (a *App) scanAllSessions() {
	a.mu.Lock()
	ids := make([]int, 0, len(a.sessions))
	sessions := make([]*terminal.Session, 0, len(a.sessions))
	for id, s := range a.sessions {
		ids = append(ids, id)
		sessions = append(sessions, s)
	}
	a.mu.Unlock()

	for i, sess := range sessions {
		id := ids[i]
		sess.ScanTokens()
		activity := sess.DetectActivity()
		actStr := activityString(activity)

		tokens := sess.GetTokens()
		costStr := ""
		if tokens.TotalCost > 0 {
			costStr = fmt.Sprintf("$%.2f", tokens.TotalCost)
		}

		// Only emit when state or cost actually changed
		prevActivityMu.Lock()
		activityChanged := prevActivity[id] != actStr
		costChanged := prevCost[id] != costStr
		changed := activityChanged || costChanged
		if changed {
			prevActivity[id] = actStr
			prevCost[id] = costStr
		}
		prevActivityMu.Unlock()

		if changed {
			log.Printf("[scan] session %d: activity=%s cost=%s", id, actStr, costStr)
			runtime.EventsEmit(a.ctx, "terminal:activity", ActivityInfo{
				ID:       id,
				Activity: actStr,
				Cost:     costStr,
			})
		}

		// Trigger pipeline queue on fresh "done" transition
		if activityChanged && actStr == "done" {
			a.processQueue(id)
		}

		// Report issue progress on activity transitions
		if activityChanged {
			a.onActivityChangeForIssue(id, actStr, costStr)
		}
	}
}

// onActivityChangeForIssue triggers issue progress reports when
// a session linked to an issue changes activity state.
func (a *App) onActivityChangeForIssue(sessionID int, newActivity string, cost string) {
	if newActivity == "done" {
		a.reportIssueProgress(sessionID, progressDone, cost)
	}
}
