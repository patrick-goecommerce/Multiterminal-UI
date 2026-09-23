// Package backend provides session-to-issue linking for the orchestration workflow.
package backend

// linkSessionIssue associates a GitHub issue with a session for tracking.
// It also posts a "start" progress comment on the issue if configured.
func (a *AppService) linkSessionIssue(sessionID int, number int, title string, branch string, dir string) {
	a.mu.Lock()
	a.sessionIssues[sessionID] = &sessionIssue{
		Number: number,
		Title:  title,
		Branch: branch,
		Dir:    dir,
	}
	a.mu.Unlock()

	a.reportIssueProgress(sessionID, progressStart, "")
}

// getSessionCost returns the current cost string for a session from the scan tracking.
func (a *AppService) getSessionCost(sessionID int) string {
	prevEmitMu.Lock()
	defer prevEmitMu.Unlock()
	return prevCost[sessionID]
}

// getSessionIssue returns the issue number linked to a session, or 0.
func (a *AppService) getSessionIssue(sessionID int) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if si := a.sessionIssues[sessionID]; si != nil {
		return si.Number
	}
	return 0
}
