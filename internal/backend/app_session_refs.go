package backend

import (
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// Session identity at the window boundary.
//
// A host numbers its sessions from 1, and that number only means something on
// that host. Everything this window keeps for itself (launches, queues, finish
// states) is keyed by it, because a window talks to exactly one host. What
// crosses to the frontend is a hub.Ref instead, the string "hub:id": the
// frontend saves it, compares it and hands it back, and a second hub later
// changes nothing there. The bindings below are that translation and nothing
// more; the logic lives in the unexported methods they call.

// hubID names the host this window talks to.
func (a *AppService) hubID() string {
	if a.host == nil {
		return ""
	}
	return a.host.Info().HubID
}

// ref names one of this host's sessions the way the frontend holds it.
func (a *AppService) ref(id int) hub.Ref {
	if id <= 0 {
		return hub.Ref{}
	}
	return hub.Local(a.hubID(), id)
}

// local resolves a ref from the frontend to a session ID on this host. A ref
// for another hub, or a legacy one without a hub, resolves to 0, which every
// caller already treats as "no such session": acting on session 3 of the
// wrong hub would type into somebody else's agent.
func (a *AppService) local(r hub.Ref) int {
	if r.IsZero() || r.Hub == "" || r.Hub != a.hubID() {
		return 0
	}
	return r.ID
}

// CreateSession spawns a session and returns its ref, or the zero ref if it
// could not be started.
func (a *AppService) CreateSession(argv []string, dir string, rows int, cols int, mode string) hub.Ref {
	return a.ref(a.createSession(argv, dir, rows, cols, mode))
}

// WriteToSession sends raw input (base64) to a session.
func (a *AppService) WriteToSession(r hub.Ref, b64data string) { a.writeToSession(a.local(r), b64data) }

// ResizeSession changes a session's dimensions.
func (a *AppService) ResizeSession(r hub.Ref, rows int, cols int) {
	a.resizeSession(a.local(r), rows, cols)
}

// CloseSession ends a session.
func (a *AppService) CloseSession(r hub.Ref) { a.closeSession(a.local(r)) }

// ResyncSession repaints a session whose stream has a hole.
func (a *AppService) ResyncSession(r hub.Ref) { a.resyncSession(a.local(r)) }

// AttachSession puts a running session back on screen.
func (a *AppService) AttachSession(r hub.Ref, rows int, cols int) bool {
	return a.attachSession(a.local(r), rows, cols)
}

// SuspendSession puts an idle pane to sleep.
func (a *AppService) SuspendSession(r hub.Ref) error { return a.suspendSession(a.local(r)) }

// ResumeSession wakes a sleeping pane.
func (a *AppService) ResumeSession(r hub.Ref) error { return a.resumeSession(a.local(r)) }

// IsSessionSuspended reports whether a pane is asleep.
func (a *AppService) IsSessionSuspended(r hub.Ref) bool { return a.isSessionSuspended(a.local(r)) }

// SeedActivitySince restores a pane's state-start timestamp after a restart,
// so its badge keeps the duration it had instead of starting over (#189). The
// seed only counts if the pane confirms that same state first; zero or an
// empty state is ignored.
func (a *AppService) SeedActivitySince(r hub.Ref, unix int64, state string) {
	id := a.local(r)
	if id == 0 || unix <= 0 || state == "" {
		return
	}
	_ = a.host.SeedActivity(id, hub.Activity(state), time.Unix(unix, 0))
}

// ResendActivity emits a session's current state once more. A pane that
// re-attached to a running session (daemon mode) exists in the frontend only
// after the attach returned, so the state has to be asked for then; waiting
// for the next change would leave it on "startet" until the agent does
// something.
func (a *AppService) ResendActivity(r hub.Ref) { a.resendActivity(a.local(r)) }

// GetFirstClaudeSessionID returns the first live Claude session, or the zero
// ref.
func (a *AppService) GetFirstClaudeSessionID() hub.Ref { return a.ref(a.firstClaudeSessionID()) }

// LinkSessionIssue ties a session to a GitHub issue for progress reports.
func (a *AppService) LinkSessionIssue(r hub.Ref, number int, title string, branch string, dir string) {
	a.linkSessionIssue(a.local(r), number, title, branch, dir)
}

// GetSessionIssue returns the issue linked to a session, or 0.
func (a *AppService) GetSessionIssue(r hub.Ref) int { return a.getSessionIssue(a.local(r)) }

// CheckAskUser looks for a pending question on a session's screen.
func (a *AppService) CheckAskUser(r hub.Ref) *AskUserQuestion { return a.checkAskUser(a.local(r)) }

// AnswerAskUser types an answer into a session.
func (a *AppService) AnswerAskUser(r hub.Ref, answer string) error {
	return a.answerAskUser(a.local(r), answer)
}

// DismissAskUser drops a pending question without answering it.
func (a *AppService) DismissAskUser(r hub.Ref) { a.dismissAskUser(a.local(r)) }
