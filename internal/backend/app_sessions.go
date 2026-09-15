package backend

import "github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"

// Session lookup goes through this file.
//
// Twenty files used to reach into a.sessions directly, each with its own
// locking dance around it. That is fine while the map is a field of this
// struct and stops being fine the moment the sessions live in the daemon
// (cmd/mtuid): every one of those places would have to be found and changed
// at once. Routing them through a handful of accessors makes that swap a
// change to this file instead of a change to twenty.
//
// Two flavours exist on purpose. The plain form takes a.mu; the Locked form is
// for callers that already hold it, because a.mu is a plain Mutex and taking
// it twice deadlocks rather than failing a test.
//
// Design: docs/superpowers/specs/2026-09-15-mtuid-daemon-architecture-design.md

// session returns the session with the given ID, or nil if there is none.
func (a *AppService) session(id int) *terminal.Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions[id]
}

// sessionLocked is session for callers that already hold a.mu.
func (a *AppService) sessionLocked(id int) *terminal.Session {
	return a.sessions[id]
}

// hasSession reports whether the ID names a live session.
func (a *AppService) hasSession(id int) bool {
	return a.session(id) != nil
}

// sessionCount returns how many sessions are running.
func (a *AppService) sessionCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sessions)
}

// liveSessions returns every session in no particular order.
//
// It hands back a snapshot rather than iterating under the caller's lock: the
// callers scan screens and touch PTYs, and doing that while holding a.mu would
// block every other session operation for the duration.
func (a *AppService) liveSessions() []*terminal.Session {
	_, sessions := a.liveSessionsByID()
	return sessions
}

// liveSessionsByID returns a snapshot of IDs and sessions in matching order.
func (a *AppService) liveSessionsByID() ([]int, []*terminal.Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := make([]int, 0, len(a.sessions))
	sessions := make([]*terminal.Session, 0, len(a.sessions))
	for id, s := range a.sessions {
		ids = append(ids, id)
		sessions = append(sessions, s)
	}
	return ids, sessions
}

// sessionEntry pairs a session with its ID for iteration.
type sessionEntry struct {
	ID      int
	Session *terminal.Session
}

// sessionEntriesLocked returns every session for a caller that already holds
// a.mu and needs to look at other fields in the same critical section.
func (a *AppService) sessionEntriesLocked() []sessionEntry {
	out := make([]sessionEntry, 0, len(a.sessions))
	for id, s := range a.sessions {
		out = append(out, sessionEntry{ID: id, Session: s})
	}
	return out
}

// putSessionLocked registers a started session under its ID.
func (a *AppService) putSessionLocked(id int, s *terminal.Session) {
	a.sessions[id] = s
}

// dropSessionLocked forgets a session. It does not close it: closing blocks on
// the process, and a.mu is held here.
func (a *AppService) dropSessionLocked(id int) {
	delete(a.sessions, id)
}
