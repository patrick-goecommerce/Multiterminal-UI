package backend

import "testing"

func TestLinkSessionIssue_Basic(t *testing.T) {
	a := newTestApp()
	a.LinkSessionIssue(1, 42, "Fix bug", "issue/42-fix-bug", "/tmp/repo")

	num := a.GetSessionIssue(1)
	if num != 42 {
		t.Fatalf("expected issue 42, got %d", num)
	}
}

func TestGetSessionIssue_NoLink(t *testing.T) {
	a := newTestApp()
	num := a.GetSessionIssue(999)
	if num != 0 {
		t.Fatalf("expected 0 for unlinked session, got %d", num)
	}
}

func TestLinkSessionIssue_OverwritesPrevious(t *testing.T) {
	a := newTestApp()
	a.LinkSessionIssue(1, 10, "First", "issue/10-first", "/tmp")
	a.LinkSessionIssue(1, 20, "Second", "issue/20-second", "/tmp")

	num := a.GetSessionIssue(1)
	if num != 20 {
		t.Fatalf("expected issue 20 after overwrite, got %d", num)
	}
}

func TestLinkSessionIssue_MultipleSessions(t *testing.T) {
	a := newTestApp()
	a.LinkSessionIssue(1, 10, "Issue A", "issue/10-a", "/tmp")
	a.LinkSessionIssue(2, 20, "Issue B", "issue/20-b", "/tmp")
	a.LinkSessionIssue(3, 30, "Issue C", "issue/30-c", "/tmp")

	if got := a.GetSessionIssue(1); got != 10 {
		t.Errorf("session 1: expected 10, got %d", got)
	}
	if got := a.GetSessionIssue(2); got != 20 {
		t.Errorf("session 2: expected 20, got %d", got)
	}
	if got := a.GetSessionIssue(3); got != 30 {
		t.Errorf("session 3: expected 30, got %d", got)
	}
}

func TestGetSessionCost_Default(t *testing.T) {
	a := newTestApp()
	cost := a.getSessionCost(1)
	if cost != "" {
		t.Fatalf("expected empty cost for unknown session, got %q", cost)
	}
}
