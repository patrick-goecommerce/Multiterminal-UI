// Package backend provides automatic issue progress reporting.
// When a session is linked to a GitHub issue, activity changes
// (start, done, close) are posted as comments on the issue.
package backend

import (
	"fmt"
	"log"
)

// issueProgressEvent describes what happened to an issue-linked session.
type issueProgressEvent string

const (
	progressStart issueProgressEvent = "start"
	progressDone  issueProgressEvent = "done"
	progressClose issueProgressEvent = "close"
)

// reportIssueProgress posts a status comment on the linked GitHub issue.
// Called internally when activity changes on an issue-linked session.
func (a *App) reportIssueProgress(sessionID int, event issueProgressEvent, cost string) {
	a.mu.Lock()
	si := a.sessionIssues[sessionID]
	cfg := a.cfg
	a.mu.Unlock()

	if si == nil || si.Number == 0 || si.Dir == "" {
		return
	}

	tc := cfg.IssueTracking
	var body string

	switch event {
	case progressStart:
		if !tc.AutoCommentOnStart {
			return
		}
		body = formatStartComment(si.Branch)
	case progressDone:
		if !tc.AutoCommentOnDone {
			return
		}
		body = formatDoneComment(si.Branch, cost, tc.IncludeCostInReport)
	case progressClose:
		if !tc.AutoCommentOnClose {
			return
		}
		body = formatCloseComment(si.Branch, cost, tc.IncludeCostInReport)
	default:
		return
	}

	// Post comment asynchronously to avoid blocking the scan loop
	go func() {
		if err := a.AddIssueComment(si.Dir, si.Number, body); err != nil {
			log.Printf("[reportIssueProgress] failed to comment on #%d: %v", si.Number, err)
		} else {
			log.Printf("[reportIssueProgress] posted %s comment on #%d", event, si.Number)
		}

		// Auto-close issue if configured and event is "done"
		if event == progressDone && tc.AutoCloseIssue {
			if err := a.UpdateIssue(si.Dir, si.Number, "", "", "closed"); err != nil {
				log.Printf("[reportIssueProgress] failed to close #%d: %v", si.Number, err)
			}
		}
	}()
}

func formatStartComment(branch string) string {
	msg := "**Multiterminal Agent Update**\n\nStatus: Agent gestartet"
	if branch != "" {
		msg += fmt.Sprintf("\nBranch: `%s`", branch)
	}
	return msg
}

func formatDoneComment(branch string, cost string, includeCost bool) string {
	msg := "**Multiterminal Agent Update**\n\nStatus: Aufgabe abgeschlossen"
	if branch != "" {
		msg += fmt.Sprintf("\nBranch: `%s`", branch)
	}
	if includeCost && cost != "" {
		msg += fmt.Sprintf("\nKosten: %s", cost)
	}
	return msg
}

func formatCloseComment(branch string, cost string, includeCost bool) string {
	msg := "**Multiterminal Agent Update**\n\nStatus: Session beendet"
	if branch != "" {
		msg += fmt.Sprintf("\nBranch: `%s`", branch)
	}
	if includeCost && cost != "" {
		msg += fmt.Sprintf("\nKosten: %s", cost)
	}
	return msg
}
