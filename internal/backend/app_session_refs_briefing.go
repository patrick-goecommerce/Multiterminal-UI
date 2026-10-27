package backend

import "github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"

// The briefing bindings, translated at the window boundary like those in
// app_session_refs.go. The logic is in app_briefing.go.

// GetSessionBriefing returns what the host measured when the pane's last turn
// ended, or nil when there is none (or the ref is not this window's host).
func (a *AppService) GetSessionBriefing(r hub.Ref) *hub.Briefing {
	return a.sessionBriefing(a.local(r))
}

// SetSessionTarget names the branch the pane's work is measured against.
func (a *AppService) SetSessionTarget(r hub.Ref, branch string) {
	a.setSessionTarget(a.local(r), branch)
}
