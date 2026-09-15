package backend

import "github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"

// Everything the backend knows about a session it asks the host.
//
// Twenty files used to reach into a map on AppService and read
// *terminal.Session fields directly. None do any more: what is left here are
// the two questions asked often enough to deserve a name, and both of them are
// answerable by a host in another process. That is the property that lets
// a.host become a hub.Remote without touching the callers.
//
// Design: docs/superpowers/specs/2026-09-15-mtuid-daemon-architecture-design.md

// hasSession reports whether the ID names a live session.
func (a *AppService) hasSession(id int) bool {
	if a.host == nil {
		return false
	}
	_, err := a.host.Get(id)
	return err == nil
}

// sessionCount returns how many sessions are running.
func (a *AppService) sessionCount() int {
	return len(a.sessionSummaries())
}

// sessionSummaries returns the host's view of every session.
func (a *AppService) sessionSummaries() []hub.SessionSummary {
	if a.host == nil {
		return nil
	}
	return a.host.List()
}
