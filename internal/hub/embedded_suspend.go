package hub

import (
	"errors"
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// Suspend implements Host.
//
// Sleeping a pane is a two-phase commit, and both phases live here because
// both of them touch the session: arming happens under the session's own lock
// so a chunk of output arriving at the same moment can mark the suspend
// aborted, and the kill happens outside every lock because taskkill takes
// 100 to 300 ms.
func (h *Embedded) Suspend(id int) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	if m.sess.IsSuspendedOrSuspending() {
		return nil // already asleep or on its way, so this is idempotent
	}
	resumeID := effectiveResumeID(m.sess)
	if resumeID == "" {
		return ErrNoResumeID
	}
	// Pin the effective ID onto the session: the hook-reported one is the
	// better of the two and resuming must not fall back to the launch-time
	// guess afterwards.
	m.sess.SetResumeID(resumeID)

	if !m.sess.TrySuspend() {
		return ErrNotIdle
	}
	go h.completeSuspend(id, m.sess)
	return nil
}

// completeSuspend runs the kill outside every lock and finalises the suspend.
func (h *Embedded) completeSuspend(id int, sess *terminal.Session) {
	// The read loop flags any chunk that arrived after arming. Killing a pane
	// that just woke up would destroy work in flight.
	if sess.SuspendAborted() {
		sess.AbortSuspend()
		log.Printf("[hub] session %d: suspend aborted, output arrived after arming", id)
		return
	}

	// Order is mandatory: the tree has to be killed while its root still
	// exists. After Process.Kill (inside FinishSuspend) the grandchildren are
	// orphaned and the node/MCP processes survive.
	if h.killTree != nil {
		h.killTree(sess.Pid())
	}
	if !sess.FinishSuspend() {
		log.Printf("[hub] session %d: FinishSuspend found no armed suspend", id)
		return
	}
	log.Printf("[hub] session %d asleep (resume id %s)", id, sess.ResumeID())
	h.emit(EventSessionSuspended, SessionSuspended{ID: id, ResumeID: sess.ResumeID()})
}

// Resume implements Host.
func (h *Embedded) Resume(id int, argv []string, dir string, env []string) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	if !m.sess.IsSuspended() {
		return nil // awake already, nothing to do
	}
	if err := m.sess.Resume(argv, dir, env); err != nil {
		if errors.Is(err, terminal.ErrNotSuspended) {
			return nil // a concurrent wake won the race
		}
		return err
	}
	resumeID := m.sess.ResumeID()
	log.Printf("[hub] session %d waking up (resume id %s)", id, resumeID)
	h.emit(EventSessionResumed, SessionResumed{ID: id, ResumeID: resumeID})
	return nil
}
