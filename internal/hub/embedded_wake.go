package hub

import (
	"fmt"
	"log"
)

// Waking a sleeping pane, from the host's own side.
//
// Before this, only a caller that remembered how a session was launched could
// resume it, and that caller was the window. With the sessions in a daemon
// that is the wrong place for it: a prompt queued for a sleeping agent while
// no window is open would sit there until somebody opened one, which is
// exactly the case the daemon exists for.
//
// The host already has everything it needs. It kept the CreateSpec, so it
// knows the argv, the directory and the mode; the agent's own conversation ID
// is on the session; and the Launcher rebuilds the environment. What it must
// NOT do is reuse the environment from the original launch: a resumed pane
// gets a fresh one, because the shim port and the worktree policy may both
// have changed since.

// Wake resumes a suspended session, working out the command line itself.
//
// It returns nil for a session that is already awake, so a caller racing
// another wake does not have to care which one won.
func (h *Embedded) Wake(id int) error {
	m, err := h.lookup(id)
	if err != nil {
		return err
	}
	if !m.sess.IsSuspended() {
		return nil
	}
	// effectiveResumeID, not sess.ResumeID: what a lifecycle hook reported
	// wins over what was parsed out of argv at launch, because the agent may
	// have picked a different ID internally. Suspend and the summary already
	// resolve it that way, and a Wake that disagreed would refuse to wake
	// exactly the sessions that do report hooks.
	resumeID := effectiveResumeID(m.sess)
	if resumeID == "" {
		// Without the agent's conversation ID a resume would start a fresh
		// conversation in the same pane, silently losing the work the pane was
		// put to sleep to preserve. Refusing is the safe half of that choice.
		//
		// Defensive: Suspend refuses a session with no conversation ID in the
		// first place, so this is not reachable through the public API today.
		// It is here because the two guards protect the same thing and only
		// one of them is where the damage would happen.
		return fmt.Errorf("hub: session %d: %w", id, ErrNoResumeID)
	}
	if h.launcher == nil {
		return fmt.Errorf("hub: session %d cannot be woken here; this host has no launcher", id)
	}

	h.mu.Lock()
	spec := m.spec
	h.mu.Unlock()

	argv := h.launcher.ResumeArgv(spec.Argv, resumeID)
	env := h.launcher.Env(id, spec.Dir, spec.Mode)
	log.Printf("[hub] waking session %d (resume id %s)", id, resumeID)
	return h.Resume(id, argv, spec.Dir, env)
}
