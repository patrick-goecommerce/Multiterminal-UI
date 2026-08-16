package backend

import (
	"context"
	"log"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// suspendCheckInterval is how often idle panes are looked for. The timeout is
// measured in minutes, so checking every half minute is precise enough and
// costs nothing.
const suspendCheckInterval = 30 * time.Second

// idleSuspendTimeout returns the configured timeout, already clamped by config
// validation, or 0 when the feature is off.
func (a *AppService) idleSuspendTimeout() time.Duration {
	if a.cfg.IdleSuspend.Enabled == nil || !*a.cfg.IdleSuspend.Enabled {
		return 0
	}
	return time.Duration(a.cfg.IdleSuspend.TimeoutMinutes) * time.Minute
}

// suspendBlocker names the reason a pane may not be suspended, or "" when it
// may. Returning the reason rather than a bool is what makes this debuggable:
// "why did that pane never sleep" is otherwise unanswerable from the outside.
//
// Every condition here is a way to lose work or wedge the app, not a matter of
// taste:
//   - Only Claude panes can be resumed at all — there is no verified resume
//     path for codex/gemini, and a shell has no conversation to restore.
//   - Without a resume id there is nothing to come back to.
//   - Without hook data the pane may not be a live Claude process at all: a
//     user who exited Claude and kept the shell would lose that shell.
//   - Anything other than a confirmed "done" means work, a pending question, or
//     a screen the classifier did not understand — see issue #188 for why
//     "idle" in particular is not a safe signal.
//   - A queued prompt, a running finish flow, an orchestrator card or an
//     agent-control session all mean something is about to write to this pane.
func (a *AppService) suspendBlocker(id int, sess *terminal.Session, timeout time.Duration, now time.Time) string {
	if timeout <= 0 {
		return "feature disabled"
	}
	if sess == nil {
		return "no session"
	}
	if sess.IsSuspendedOrSuspending() {
		return "already suspended"
	}
	if !sess.IsRunning() {
		return "not running"
	}

	a.mu.Lock()
	mode := a.sessionMode[id]
	queue := a.queues[id]
	_, isAgent := a.agentSessions[id]
	finish := a.finishStates[id]
	a.mu.Unlock()

	if !isClaudeMode(mode) {
		return "not a claude pane"
	}
	if isAgent {
		return "agent-control session"
	}
	if finish != nil {
		return "worktree finish in progress"
	}
	if resumeIDFor(sess) == "" {
		return "no resume id"
	}
	if !sess.HasHookData() {
		return "no hook data yet"
	}
	if queue != nil && (queueHasStatus(queue.items, "pending") || queueHasStatus(queue.items, "sent")) {
		return "queued prompts waiting"
	}
	if orchestratorHolds(id) {
		return "orchestrator is using this pane"
	}

	prevActivityMu.Lock()
	state := prevActivity[id]
	since := activitySince[id]
	prevActivityMu.Unlock()

	if state != "done" {
		return "state is " + stateOrUnknown(state)
	}
	if since.IsZero() {
		return "no confirmed state yet"
	}
	if idle := now.Sub(since); idle < timeout {
		return "idle for " + idle.Round(time.Second).String() + ", needs " + timeout.String()
	}
	return ""
}

// stateOrUnknown keeps the blocker message readable for a pane whose state has
// not been confirmed yet.
func stateOrUnknown(state string) string {
	if state == "" {
		return "unknown"
	}
	return state
}

// suspendIdleSessions suspends every pane that has been finished and quiet for
// longer than the configured timeout.
//
// The session list is copied under App.mu and released before anything else
// happens: suspending runs taskkill, and process I/O must never hold a lock.
func (a *AppService) suspendIdleSessions(now time.Time) {
	timeout := a.idleSuspendTimeout()
	if timeout <= 0 {
		return
	}

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
		if reason := a.suspendBlocker(id, sess, timeout, now); reason != "" {
			continue
		}
		log.Printf("[idle-suspend] session %d idle past %s, suspending", id, timeout)
		if err := a.SuspendSession(id); err != nil {
			log.Printf("[idle-suspend] session %d: %v", id, err)
		}
	}
}

// orchestratorHolds reports whether any orchestrator is currently running or
// reviewing a card in this session. Suspending such a pane would strand the
// card: the scheduler waits on a process that is no longer there.
//
// Takes orchMu and then each orchestrator's own mu, the same order
// StartOrchestration uses. Neither App.mu nor Session.mu may be held here.
func orchestratorHolds(sessionID int) bool {
	orchMu.Lock()
	states := make([]*orchestratorState, 0, len(orchestrators))
	for _, o := range orchestrators {
		states = append(states, o)
	}
	orchMu.Unlock()

	for _, o := range states {
		o.mu.Lock()
		for _, sid := range o.running {
			if sid == sessionID {
				o.mu.Unlock()
				return true
			}
		}
		for _, sid := range o.review {
			if sid == sessionID {
				o.mu.Unlock()
				return true
			}
		}
		o.mu.Unlock()
	}
	return false
}

// idleSuspendLoop watches for idle panes until ctx is done. The timeout is read
// on every tick, so toggling the setting takes effect without a restart.
func (a *AppService) idleSuspendLoop(ctx context.Context) {
	ticker := time.NewTicker(suspendCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.suspendIdleSessions(now)
		}
	}
}
