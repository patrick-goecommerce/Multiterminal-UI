package hub

import (
	"fmt"
	"os"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// Starting a session.
//
// Split out of embedded.go for the file-size rule, but the two halves are also
// genuinely different jobs: this one turns a request into a running process,
// and the rest of that file is the bookkeeping around one that already runs.

// Create implements Host.
func (h *Embedded) Create(spec CreateSpec) (int, error) {
	if spec.Launch != nil {
		var err error
		if spec, err = h.resolveLaunch(spec); err != nil {
			return 0, err
		}
	}
	if spec.Rows < 5 {
		spec.Rows = 24
	}
	if spec.Cols < 20 {
		spec.Cols = 80
	}
	if spec.Dir == "" {
		spec.Dir, _ = os.Getwd()
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return 0, ErrClosed
	}
	id := spec.ID
	if id == 0 {
		h.nextID++
		id = h.nextID
	} else if id > h.nextID {
		// A reserved ID that came from somewhere else still has to move the
		// counter, or the next allocation would hand out the same number.
		h.nextID = id
	}
	if _, taken := h.sessions[id]; taken {
		h.mu.Unlock()
		return 0, fmt.Errorf("hub: session %d already exists", id)
	}
	h.mu.Unlock()

	sess := terminal.NewSession(id, spec.Rows, spec.Cols)
	if spec.ResumeID != "" {
		sess.SetResumeID(spec.ResumeID)
	}
	if err := sess.Start(spec.Argv, spec.Dir, spec.Env); err != nil {
		return 0, fmt.Errorf("hub: start session %d: %w", id, err)
	}
	m := &managed{
		sess:      sess,
		ring:      NewRing(h.ringBytes),
		spec:      spec,
		startedAt: time.Now(),
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		sess.Close()
		return 0, ErrClosed
	}
	h.sessions[id] = m
	h.mu.Unlock()

	// Report the session before starting its watchers. A process that exits
	// at once (a typo in argv, a one-line script) would otherwise race its own
	// creation, and a client that receives an exit for a session it has never
	// heard of has no way to make sense of it. Output is not at risk from the
	// later start: RawOutputCh buffers, and nothing is read from it elsewhere.
	h.emit(EventSessionCreated, SessionCreated{Session: summarize(id, m)})

	go h.pump(m)
	go h.watchExit(id, sess)
	return id, nil
}

// resolveLaunch turns a launch request into a concrete spec.
//
// The ID is reserved first, because the environment names the session it
// belongs to: MULTITERMINAL_SESSION_ID is what the lifecycle hook and the
// statusline shim report back with, so it has to exist before Env can be
// built. That ordering is why CreateSpec has both ID and Launch rather than a
// callback, which would not survive the wire.
func (h *Embedded) resolveLaunch(spec CreateSpec) (CreateSpec, error) {
	if h.launcher == nil {
		return spec, fmt.Errorf(
			"hub: this host cannot launch %q by name; it was built without a launcher",
			spec.Launch.Tool)
	}
	argv, err := h.launcher.Argv(spec.Launch.Tool, spec.Launch.Model)
	if err != nil {
		return spec, fmt.Errorf("hub: %w", err)
	}
	if spec.ID == 0 {
		if spec.ID, err = h.Reserve(); err != nil {
			return spec, err
		}
	}
	if spec.Mode == "" {
		spec.Mode = spec.Launch.Tool
	}
	if spec.Dir == "" {
		spec.Dir, _ = os.Getwd()
	}
	spec.Argv = argv
	spec.Env = h.launcher.Env(spec.ID, spec.Dir, spec.Mode)
	return spec, nil
}
