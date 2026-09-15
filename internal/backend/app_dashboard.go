// Package backend provides aggregated dashboard statistics across all projects.
package backend

import (
	"fmt"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"os"
	"path/filepath"
	"strings"
)

// DashboardStats holds aggregated metrics for the dashboard view.
type DashboardStats struct {
	Projects      []ProjectStats `json:"projects" yaml:"projects"`
	TotalCost     string         `json:"total_cost" yaml:"total_cost"`
	TotalSessions int            `json:"total_sessions" yaml:"total_sessions"`
}

// ProjectStats holds per-project metrics derived from open sessions.
type ProjectStats struct {
	Dir            string  `json:"dir" yaml:"dir"`
	Name           string  `json:"name" yaml:"name"`
	ActiveSessions int     `json:"active_sessions" yaml:"active_sessions"`
	TotalCost      string  `json:"total_cost" yaml:"total_cost"`
	CostValue      float64 `json:"cost_value" yaml:"cost_value"`
	Branch         string  `json:"branch" yaml:"branch"`
	QueueDepth     int     `json:"queue_depth" yaml:"queue_depth"`
	IsInitialized  bool    `json:"is_initialized" yaml:"is_initialized"`
}

// GetDashboardStats returns aggregated statistics grouped by project directory.
func (a *AppService) GetDashboardStats() DashboardStats {
	a.mu.Lock()

	type dirInfo struct {
		dir       string
		sessions  int
		cost      float64
		queuePend int
	}

	projectMap := make(map[string]*dirInfo)
	var totalCost float64
	totalSessions := 0

	for _, s := range a.sessionSummaries() {
		dir := s.Dir
		if dir == "" {
			continue
		}

		di, ok := projectMap[dir]
		if !ok {
			di = &dirInfo{dir: dir}
			projectMap[dir] = di
		}

		di.sessions++
		totalSessions++

		if s.Cost > 0 {
			di.cost += s.Cost
			totalCost += s.Cost
		}

		// Aggregate queue depth
		q := a.queues[s.ID]
		if q != nil {
			for _, item := range q.items {
				if item.Status == "pending" || item.Status == "sent" {
					di.queuePend++
				}
			}
		}
	}
	a.mu.Unlock()

	// Build project list (no lock needed for filesystem checks)
	projects := make([]ProjectStats, 0, len(projectMap))
	for _, di := range projectMap {
		ps := ProjectStats{
			Dir:            di.dir,
			Name:           filepath.Base(di.dir),
			ActiveSessions: di.sessions,
			CostValue:      di.cost,
			QueueDepth:     di.queuePend,
			IsInitialized:  dirExists(filepath.Join(di.dir, ".mtui")),
		}
		if di.cost > 0 {
			ps.TotalCost = formatDashCost(di.cost)
		}
		// Read git branch directly from .git/HEAD to avoid exec under lock
		ps.Branch = readGitHeadBranch(di.dir)
		projects = append(projects, ps)
	}

	stats := DashboardStats{
		Projects:      projects,
		TotalSessions: totalSessions,
	}
	if totalCost > 0 {
		stats.TotalCost = formatDashCost(totalCost)
	}
	return stats
}

// readGitHeadBranch reads .git/HEAD to extract the branch name without exec.
func readGitHeadBranch(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	const prefix = "ref: refs/heads/"
	if strings.HasPrefix(head, prefix) {
		return head[len(prefix):]
	}
	// Detached HEAD — return short hash
	if len(head) >= 8 {
		return head[:8]
	}
	return head
}

// DashboardPane holds per-session data for the dashboard swim lanes.
// The frontend uses this to render pane cards without needing local store data.
type DashboardPane struct {
	SessionID   int    `json:"session_id" yaml:"session_id"`
	Name        string `json:"name" yaml:"name"`
	Activity    string `json:"activity" yaml:"activity"` // idle, active, done, waitingPermission, waitingAnswer, error, starting
	Cost        string `json:"cost" yaml:"cost"`
	Dir         string `json:"dir" yaml:"dir"`
	Branch      string `json:"branch" yaml:"branch"`
	Running     bool   `json:"running" yaml:"running"`
	IssueNumber int    `json:"issue_number" yaml:"issue_number"`
	IssueTitle  string `json:"issue_title" yaml:"issue_title"`
}

// GetDashboardPanes returns all sessions as pane cards for the dashboard view.
// This works in any window (main or secondary) since it reads from the backend.
func (a *AppService) GetDashboardPanes() []DashboardPane {
	a.mu.Lock()
	defer a.mu.Unlock()

	summaries := a.sessionSummaries()
	panes := make([]DashboardPane, 0, len(summaries))
	for _, s := range summaries {
		id := s.ID
		costStr := ""
		if s.Cost > 0 {
			costStr = fmt.Sprintf("$%.2f", s.Cost)
		}

		dp := DashboardPane{
			SessionID: id,
			Name:      s.Name,
			Activity:  string(s.Activity),
			Cost:      costStr,
			Dir:       s.Dir,
			Branch:    readGitHeadBranch(s.Dir),
			Running:   s.Status == hub.StatusRunning,
		}

		if dp.Name == "" {
			dp.Name = fmt.Sprintf("Session %d", id)
		}

		// Attach linked issue info
		if issue, ok := a.sessionIssues[id]; ok {
			dp.IssueNumber = issue.Number
			dp.IssueTitle = issue.Title
			if dp.Branch == "" && issue.Branch != "" {
				dp.Branch = issue.Branch
			}
		}

		panes = append(panes, dp)
	}
	return panes
}

// formatDashCost formats a float cost as dollar string.
func formatDashCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}
