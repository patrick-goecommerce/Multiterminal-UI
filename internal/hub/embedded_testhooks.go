package hub

import (
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// Hooks for tests in other packages.
//
// They are production files rather than _test.go because internal/backend's
// tests need them and Go does not export test helpers across packages. Both
// put a session into a state that starting a real process cannot reach on
// demand, and neither is called from production code.

// AdoptForTest registers a session object the caller built itself, without
// starting a process. It exists for tests that need a session in a known state
// (a frozen screen, a given activity) and must not be used in production: a
// session adopted this way has no output pump and no exit watcher.
func (h *Embedded) AdoptForTest(id int, sess *terminal.Session) {
	m := &managed{
		sess:      sess,
		ring:      NewRing(h.ringBytes),
		spec:      CreateSpec{ID: id},
		startedAt: time.Now(),
	}
	h.mu.Lock()
	h.sessions[id] = m
	if id > h.nextID {
		h.nextID = id
	}
	h.mu.Unlock()
}

// SetOriginForTest marks an already-registered session as having come from
// somewhere other than the UI. Production sets Origin in the CreateSpec; this
// is for a test that adopted a session object instead of starting one.
func (h *Embedded) SetOriginForTest(id int, origin string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.sessions[id]; m != nil {
		m.spec.Origin = origin
	}
}
