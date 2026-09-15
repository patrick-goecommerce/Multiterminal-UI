package backend

import (
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// Session lookup goes through this file.
//
// Twenty files used to reach into a map on AppService, each with its own
// locking dance around it. The sessions now belong to a.host, and this is the
// only place that knows it. When the host becomes a hub.Remote and the
// sessions live in the daemon, this file is what changes.
//
// The Locked suffixes are kept where callers hold a.mu for their own reasons
// (the dashboard reads a.queues in the same critical section). They no longer
// mean "a.mu protects the sessions" — the host has its own lock — but dropping
// the names would hide which callers are inside a critical section, and that
// is the thing a reader needs to see.
//
// Design: docs/superpowers/specs/2026-09-15-mtuid-daemon-architecture-design.md

// session returns the session with the given ID, or nil if there is none.
//
// It hands out the terminal.Session itself, which a remote host cannot do.
// Every remaining caller is therefore a caller that has to move behind the
// Host interface before the daemon can be switched on; they are being worked
// through one at a time (see the spec's phase 1d).
func (a *AppService) session(id int) *terminal.Session {
	if a.host == nil {
		return nil
	}
	return a.host.Session(id)
}

// sessionLocked is session for callers that already hold a.mu.
func (a *AppService) sessionLocked(id int) *terminal.Session {
	return a.session(id)
}

// hasSession reports whether the ID names a live session.
func (a *AppService) hasSession(id int) bool {
	return a.session(id) != nil
}

// sessionCount returns how many sessions are running.
func (a *AppService) sessionCount() int {
	if a.host == nil {
		return 0
	}
	return len(a.host.List())
}

// liveSessions returns every session in no particular order.
func (a *AppService) liveSessions() []*terminal.Session {
	_, sessions := a.liveSessionsByID()
	return sessions
}

// liveSessionsByID returns a snapshot of IDs and sessions in matching order.
func (a *AppService) liveSessionsByID() ([]int, []*terminal.Session) {
	if a.host == nil {
		return nil, nil
	}
	return a.host.Sessions()
}

// sessionEntry pairs a session with its ID for iteration.
type sessionEntry struct {
	ID      int
	Session *terminal.Session
}

// sessionEntriesLocked returns every session for a caller that already holds
// a.mu and needs to look at other fields in the same critical section.
func (a *AppService) sessionEntriesLocked() []sessionEntry {
	ids, sessions := a.liveSessionsByID()
	out := make([]sessionEntry, 0, len(ids))
	for i, id := range ids {
		out = append(out, sessionEntry{ID: id, Session: sessions[i]})
	}
	return out
}

// sessionSummaries returns the host's view of every session, without handing
// out the session objects. This is the shape that survives the move to the
// daemon.
func (a *AppService) sessionSummaries() []hub.SessionSummary {
	if a.host == nil {
		return nil
	}
	return a.host.List()
}
