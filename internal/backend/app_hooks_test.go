package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// The lifecycle-hook reader moved to the session host: reading the files and
// deciding what an event says about a session is tested in internal/hooks and
// internal/hub. What is left here is what the window does about an event.

func TestOnHookReport_IgnoresAnEventWithoutASession(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	calls := 0
	a.hookWorktree = func(int, string, string, string) { calls++ }

	a.onHookReport(hub.HookReport{Event: "PreToolUse"})

	if calls != 0 {
		t.Errorf("an event without a session id reached the handlers %d times", calls)
	}
}

// An EnterWorktree detection is how a pane learns it moved into a worktree.
func TestOnHookReport_ReportsAWorktreeChange(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	var gotPath, gotBranch, gotCwd string
	calls := 0
	a.hookWorktree = func(_ int, path, branch, cwd string) {
		calls++
		gotPath, gotBranch, gotCwd = path, branch, cwd
	}

	a.onHookReport(hub.HookReport{
		Session: 1, Event: "PostToolUse", Tool: "EnterWorktree",
		Cwd:            `D:\repos\proj\.claude\worktrees\feature-a`,
		WorktreePath:   `D:\repos\proj\.claude\worktrees\feature-a`,
		WorktreeBranch: "worktree-feature-a",
	})

	if calls != 1 {
		t.Fatalf("worktree handler called %d times, want 1", calls)
	}
	if gotPath != `D:\repos\proj\.claude\worktrees\feature-a` || gotBranch != "worktree-feature-a" {
		t.Errorf("got path=%q branch=%q", gotPath, gotBranch)
	}
	if gotCwd != `D:\repos\proj\.claude\worktrees\feature-a` {
		t.Errorf("got cwd=%q", gotCwd)
	}
}

// Every event reports the cwd, not just the worktree ones: that is how a pane
// that left a worktree is noticed at all.
func TestOnHookReport_AnOrdinaryEventStillReportsTheCwd(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	var gotPath, gotCwd string
	calls := 0
	a.hookWorktree = func(_ int, path, _, cwd string) {
		calls++
		gotPath, gotCwd = path, cwd
	}

	a.onHookReport(hub.HookReport{Session: 1, Event: "PostToolUse", Tool: "Bash", Cwd: `D:\repos\proj`})

	if calls != 1 {
		t.Fatalf("worktree handler called %d times, want 1", calls)
	}
	if gotPath != "" {
		t.Errorf("worktreePath = %q, want empty for an ordinary event", gotPath)
	}
	if gotCwd != `D:\repos\proj` {
		t.Errorf("cwd = %q", gotCwd)
	}
}

func TestOnHookReport_ReportsABlockedPath(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	var gotPath, gotReason string
	calls := 0
	a.hookWorktree = func(int, string, string, string) {}
	a.hookPathBlocked = func(_ int, path, reason string) {
		calls++
		gotPath, gotReason = path, reason
	}

	a.onHookReport(hub.HookReport{
		Session: 1, Event: "PreToolUse", Tool: "Edit",
		BlockedPath: `D:\repo\internal\backend\app.go`,
		BlockReason: "Pfad liegt im Hauptrepo...",
	})

	if calls != 1 {
		t.Fatalf("blocked-path handler called %d times, want 1", calls)
	}
	if gotPath != `D:\repo\internal\backend\app.go` || gotReason != "Pfad liegt im Hauptrepo..." {
		t.Errorf("got path=%q reason=%q", gotPath, gotReason)
	}
}

func TestOnHookReport_NoBlockedPathMeansNoDialog(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	calls := 0
	a.hookWorktree = func(int, string, string, string) {}
	a.hookPathBlocked = func(int, string, string) { calls++ }

	a.onHookReport(hub.HookReport{Session: 1, Event: "PreToolUse", Tool: "Edit"})

	if calls != 0 {
		t.Errorf("blocked-path handler fired %d times without a blocked path", calls)
	}
}

// The prompt text is what an automatic pane name is derived from.
func TestOnHookReport_ForwardsAPromptForNaming(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	var gotPrompt string
	a.hookWorktree = func(int, string, string, string) {}
	a.hookPrompt = func(_ int, prompt string) { gotPrompt = prompt }

	a.onHookReport(hub.HookReport{Session: 1, Event: "UserPromptSubmit", Message: "bau mir was"})

	if gotPrompt != "bau mir was" {
		t.Errorf("prompt = %q, want %q", gotPrompt, "bau mir was")
	}
}

func TestOnHookReport_AnEmptyPromptIsNotAName(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	calls := 0
	a.hookWorktree = func(int, string, string, string) {}
	a.hookPrompt = func(int, string) { calls++ }

	a.onHookReport(hub.HookReport{Session: 1, Event: "UserPromptSubmit"})

	if calls != 0 {
		t.Errorf("naming was triggered %d times by an empty prompt", calls)
	}
}
