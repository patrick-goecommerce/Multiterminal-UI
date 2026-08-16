package backend

import (
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// readyToSuspend builds an app plus session that passes every gate, so each
// test can break exactly one condition and see it caught.
func readyToSuspend(t *testing.T) (*AppService, *terminal.Session, int, time.Time) {
	t.Helper()
	resetActivityDebounceForTest()

	a := newTestApp()
	enabled := true
	a.cfg.IdleSuspend = config.IdleSuspendSettings{Enabled: &enabled, TimeoutMinutes: 30}

	const id = 1
	sess := terminal.NewSession(id, 24, 80)
	sess.SetHookActivity(terminal.ActivityDone) // sets hasHookData
	sess.SetResumeID("11111111-2222-3333-4444-555555555555")

	a.mu.Lock()
	a.sessions[id] = sess
	a.sessionMode[id] = "claude"
	a.mu.Unlock()

	now := time.Now()
	prevActivityMu.Lock()
	prevActivity[id] = "done"
	activitySince[id] = now.Add(-45 * time.Minute)
	prevActivityMu.Unlock()

	return a, sess, id, now
}

func TestSuspendBlocker_AllowsAnIdleFinishedPane(t *testing.T) {
	a, sess, id, now := readyToSuspend(t)
	if reason := a.suspendBlocker(id, sess, 30*time.Minute, now); reason != "" {
		t.Fatalf("a pane idle for 45 minutes was blocked: %s", reason)
	}
}

// Every one of these blocks a real way to lose work.
func TestSuspendBlocker_Blocks(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(a *AppService, sess *terminal.Session, id int)
		want   string
	}{
		{
			name: "a shell pane has no conversation to resume",
			break_: func(a *AppService, _ *terminal.Session, id int) {
				a.mu.Lock()
				a.sessionMode[id] = "shell"
				a.mu.Unlock()
			},
			want: "not a claude pane",
		},
		{
			name: "codex has no verified resume path",
			break_: func(a *AppService, _ *terminal.Session, id int) {
				a.mu.Lock()
				a.sessionMode[id] = "codex"
				a.mu.Unlock()
			},
			want: "not a claude pane",
		},
		{
			name: "without a resume id there is nothing to come back to",
			break_: func(_ *AppService, sess *terminal.Session, _ int) {
				sess.SetResumeID("")
			},
			want: "no resume id",
		},
		{
			name: "a queued prompt is about to be sent",
			break_: func(a *AppService, _ *terminal.Session, id int) {
				a.mu.Lock()
				a.queues[id] = &sessionQueue{items: []QueueItem{{ID: 1, Prompt: "x", Status: "pending"}}}
				a.mu.Unlock()
			},
			want: "queued prompts waiting",
		},
		{
			name: "a prompt already sent is still being worked on",
			break_: func(a *AppService, _ *terminal.Session, id int) {
				a.mu.Lock()
				a.queues[id] = &sessionQueue{items: []QueueItem{{ID: 1, Prompt: "x", Status: "sent"}}}
				a.mu.Unlock()
			},
			want: "queued prompts waiting",
		},
		{
			name: "a finish flow is mid-merge",
			break_: func(a *AppService, _ *terminal.Session, id int) {
				a.mu.Lock()
				a.finishStates[id] = &finishState{Phase: "preparing"}
				a.mu.Unlock()
			},
			want: "worktree finish in progress",
		},
		{
			name: "an agent-control session belongs to another agent",
			break_: func(a *AppService, _ *terminal.Session, id int) {
				a.mu.Lock()
				a.agentSessions[id] = AgentSessionInfo{}
				a.mu.Unlock()
			},
			want: "agent-control session",
		},
		{
			name: "a working pane must never be touched",
			break_: func(_ *AppService, _ *terminal.Session, id int) {
				prevActivityMu.Lock()
				prevActivity[id] = "active"
				prevActivityMu.Unlock()
			},
			want: "state is active",
		},
		{
			name: "a pane waiting for the user is deliberately left open",
			break_: func(_ *AppService, _ *terminal.Session, id int) {
				prevActivityMu.Lock()
				prevActivity[id] = "waitingPermission"
				prevActivityMu.Unlock()
			},
			want: "state is waitingPermission",
		},
		{
			// "idle" means the classifier did not recognise the screen — a pager,
			// a TUI, a running npm script (issue #188). Killing that is the worst
			// case this gate exists for.
			name: "an unrecognised screen is not a finished one",
			break_: func(_ *AppService, _ *terminal.Session, id int) {
				prevActivityMu.Lock()
				prevActivity[id] = "idle"
				prevActivityMu.Unlock()
			},
			want: "state is idle",
		},
		{
			name: "an error state still needs the user's eyes",
			break_: func(_ *AppService, _ *terminal.Session, id int) {
				prevActivityMu.Lock()
				prevActivity[id] = "error"
				prevActivityMu.Unlock()
			},
			want: "state is error",
		},
		{
			name: "a pane that finished a minute ago is not idle",
			break_: func(_ *AppService, _ *terminal.Session, id int) {
				prevActivityMu.Lock()
				activitySince[id] = time.Now().Add(-time.Minute)
				prevActivityMu.Unlock()
			},
			want: "idle for",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, sess, id, now := readyToSuspend(t)
			tt.break_(a, sess, id)

			reason := a.suspendBlocker(id, sess, 30*time.Minute, now)
			if reason == "" {
				t.Fatalf("pane was allowed to suspend, expected a block containing %q", tt.want)
			}
			if len(tt.want) > 0 && !strings.Contains(reason, tt.want) {
				t.Errorf("blocked with %q, want something containing %q", reason, tt.want)
			}
		})
	}
}

func TestSuspendBlocker_DisabledFeatureBlocksEverything(t *testing.T) {
	a, sess, id, now := readyToSuspend(t)
	if reason := a.suspendBlocker(id, sess, 0, now); reason != "feature disabled" {
		t.Errorf("blocker = %q, want %q", reason, "feature disabled")
	}
}

func TestIdleSuspendTimeout_OffUnlessEnabled(t *testing.T) {
	a := newTestApp()

	a.cfg.IdleSuspend = config.IdleSuspendSettings{Enabled: nil, TimeoutMinutes: 30}
	if got := a.idleSuspendTimeout(); got != 0 {
		t.Errorf("unset Enabled yielded %v, want 0 — the feature must be opt-in", got)
	}

	off := false
	a.cfg.IdleSuspend = config.IdleSuspendSettings{Enabled: &off, TimeoutMinutes: 30}
	if got := a.idleSuspendTimeout(); got != 0 {
		t.Errorf("disabled yielded %v, want 0", got)
	}

	on := true
	a.cfg.IdleSuspend = config.IdleSuspendSettings{Enabled: &on, TimeoutMinutes: 30}
	if got := a.idleSuspendTimeout(); got != 30*time.Minute {
		t.Errorf("enabled yielded %v, want 30m", got)
	}
}

// A pane suspended once must not be picked up again on the next tick.
func TestSuspendBlocker_SkipsAnAlreadySuspendedPane(t *testing.T) {
	a, sess, id, now := readyToSuspend(t)
	if !sess.TrySuspend() {
		t.Fatal("TrySuspend refused a done session")
	}
	if reason := a.suspendBlocker(id, sess, 30*time.Minute, now); reason != "already suspended" {
		t.Errorf("blocker = %q, want %q", reason, "already suspended")
	}
}
