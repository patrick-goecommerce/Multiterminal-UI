package backend

import (
	"strings"
	"testing"

	"github.com/patrick-goecommerce/multiterminal/internal/config"
)

// ---------------------------------------------------------------------------
// Format functions – pure tests
// ---------------------------------------------------------------------------

func TestFormatStartComment_WithBranch(t *testing.T) {
	msg := formatStartComment("issue/42-fix-bug")
	if !strings.Contains(msg, "Agent gestartet") {
		t.Error("expected 'Agent gestartet' in message")
	}
	if !strings.Contains(msg, "`issue/42-fix-bug`") {
		t.Error("expected branch name in message")
	}
}

func TestFormatStartComment_NoBranch(t *testing.T) {
	msg := formatStartComment("")
	if strings.Contains(msg, "Branch:") {
		t.Error("expected no branch line when branch is empty")
	}
}

func TestFormatDoneComment_WithCost(t *testing.T) {
	msg := formatDoneComment("issue/7-feat", "$1.23", true)
	if !strings.Contains(msg, "abgeschlossen") {
		t.Error("expected 'abgeschlossen' in message")
	}
	if !strings.Contains(msg, "$1.23") {
		t.Error("expected cost in message")
	}
	if !strings.Contains(msg, "`issue/7-feat`") {
		t.Error("expected branch in message")
	}
}

func TestFormatDoneComment_CostDisabled(t *testing.T) {
	msg := formatDoneComment("main", "$5.00", false)
	if strings.Contains(msg, "$5.00") {
		t.Error("cost should not appear when includeCost is false")
	}
}

func TestFormatDoneComment_EmptyCost(t *testing.T) {
	msg := formatDoneComment("main", "", true)
	if strings.Contains(msg, "Kosten:") {
		t.Error("cost line should not appear when cost is empty")
	}
}

func TestFormatCloseComment_WithBranchAndCost(t *testing.T) {
	msg := formatCloseComment("issue/1-test", "$0.50", true)
	if !strings.Contains(msg, "Session beendet") {
		t.Error("expected 'Session beendet' in message")
	}
	if !strings.Contains(msg, "$0.50") {
		t.Error("expected cost in message")
	}
}

func TestFormatCloseComment_NoBranchNoCost(t *testing.T) {
	msg := formatCloseComment("", "", true)
	if strings.Contains(msg, "Branch:") {
		t.Error("expected no branch line")
	}
	if strings.Contains(msg, "Kosten:") {
		t.Error("expected no cost line")
	}
}

// ---------------------------------------------------------------------------
// reportIssueProgress – config gating
// ---------------------------------------------------------------------------

func TestReportIssueProgress_NilSession(t *testing.T) {
	a := newTestApp()
	// Should not panic when session doesn't exist
	a.reportIssueProgress(999, progressStart, "")
}

func TestReportIssueProgress_NoIssueLinked(t *testing.T) {
	a := newTestApp()
	// Session exists but no issue linked
	a.reportIssueProgress(1, progressDone, "$1.00")
}

func TestReportIssueProgress_StartDisabled(t *testing.T) {
	a := newTestApp()
	a.cfg.IssueTracking = config.IssueTracking{
		AutoCommentOnStart: false,
		AutoCommentOnDone:  true,
	}
	a.sessionIssues[1] = &sessionIssue{
		Number: 10, Title: "Test", Branch: "issue/10-test", Dir: "/tmp",
	}
	// This should be a no-op because AutoCommentOnStart is false.
	// We can't easily verify the gh call wasn't made, but at least
	// it shouldn't panic.
	a.reportIssueProgress(1, progressStart, "")
}

func TestReportIssueProgress_DoneDisabled(t *testing.T) {
	a := newTestApp()
	a.cfg.IssueTracking = config.IssueTracking{
		AutoCommentOnDone: false,
	}
	a.sessionIssues[1] = &sessionIssue{
		Number: 10, Title: "Test", Branch: "issue/10-test", Dir: "/tmp",
	}
	a.reportIssueProgress(1, progressDone, "$1.00")
}

func TestReportIssueProgress_CloseDisabled(t *testing.T) {
	a := newTestApp()
	a.cfg.IssueTracking = config.IssueTracking{
		AutoCommentOnClose: false,
	}
	a.sessionIssues[1] = &sessionIssue{
		Number: 10, Title: "Test", Branch: "issue/10-test", Dir: "/tmp",
	}
	a.reportIssueProgress(1, progressClose, "$2.00")
}
