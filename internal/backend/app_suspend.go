package backend

// Pane sleeping (issue #180, steps 1–3): a finished Claude pane can be put to
// sleep on purpose. Its process tree (claude.exe plus the node/MCP
// grandchildren — measured at 856–861 MB) is hard-killed, while the session
// object, its numeric ID, screen, tokens and hook wiring stay alive. Waking it
// restarts claude with `--resume <uuid>` into the very same session object, so
// the pane keeps its xterm.js scrollback and every ID-keyed backend map.
//
// This file holds only the manual path. No timer, no automatic gate, no config
// and no persistence across restarts — those are steps 4–7.

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// launchSpec remembers how a session was started so a resume can rebuild an
// identical command line.
type launchSpec struct {
	argv []string
	dir  string
	mode string
}

// sessionEnv builds the PTY environment for a session. The policy itself is
// internal/launch, because the daemon has to build the same environment and
// cannot import this package.
func (a *AppService) sessionEnv(id int, dir, mode string) []string {
	return a.launchPolicy().Env(id, dir, mode)
}

// rememberLaunch stores how a session was launched (for ResumeSession).
//
// The map is created on demand: an AppService built as a struct literal (tests)
// would otherwise panic on the write — while holding a.mu, which deadlocks
// everything that follows.
func (a *AppService) rememberLaunch(id int, argv []string, dir, mode string) {
	cp := append([]string(nil), argv...)
	a.mu.Lock()
	if a.launches == nil {
		a.launches = make(map[int]launchSpec)
	}
	a.launches[id] = launchSpec{argv: cp, dir: dir, mode: mode}
	a.mu.Unlock()
}

// claudeSessionIDFromArgv and resumeArgv moved to internal/launch: building a
// command line is launch policy, and the daemon has to be able to wake a pane
// without a window. See launch_delegate.go.

// SuspendSession puts a finished Claude pane to sleep. Returns an error when
// the pane is not eligible; the kill itself runs asynchronously because
// taskkill takes 100–300 ms and must never run under a lock.
func (a *AppService) SuspendSession(id int) error {
	a.mu.Lock()
	mode := a.sessionMode[id]
	a.mu.Unlock()
	if !a.hasSession(id) {
		return fmt.Errorf("session %d not found", id)
	}
	if !isClaudeMode(mode) {
		return fmt.Errorf("sleeping is only supported for claude panes (mode %q)", mode)
	}

	// The host runs the two-phase commit and reports the finished suspend as
	// an event; the messages here are what the user sees, so they stay.
	err := a.host.Suspend(id)
	switch {
	case errors.Is(err, hub.ErrNoResumeID):
		return errors.New("no claude session id known for this pane — it cannot be resumed")
	case errors.Is(err, hub.ErrNotIdle):
		return errors.New("pane is not idle — only a finished (done) claude pane can sleep")
	}
	return err
}

// ResumeSession wakes a sleeping pane by restarting claude with --resume into
// the same session object. Calling it on an awake pane is a no-op.
func (a *AppService) ResumeSession(id int) error {
	a.mu.Lock()
	spec := a.launches[id]
	a.mu.Unlock()

	summary, err := a.host.Get(id)
	if err != nil {
		return fmt.Errorf("session %d not found", id)
	}
	if summary.Status != hub.StatusSuspended {
		return nil
	}
	if summary.ResumeID == "" {
		return errors.New("no claude session id known for this pane — it cannot be resumed")
	}
	dir := spec.dir
	if dir == "" {
		dir = summary.Dir
	}
	argv := resumeArgv(spec.argv, summary.ResumeID)
	env := a.sessionEnv(id, dir, spec.mode)

	if err := a.host.Resume(id, argv, dir, env); err != nil {
		log.Printf("[resume] session %d failed: %v", id, err)
		return err
	}
	return nil
}

// IsSessionSuspended reports whether a pane is currently asleep.
func (a *AppService) IsSessionSuspended(id int) bool {
	summary, err := a.host.Get(id)
	return err == nil && summary.Status == hub.StatusSuspended
}

// wakeSession resumes a pane in the background. Used by the write paths, which
// must not block on a 12–15 s resume.
//
// A suspend that is still arming (taskkill in flight, 100–300 ms) is waited out
// instead of ignored: otherwise a wake request landing in that window would be
// dropped and the pane would stay asleep with a prompt waiting for it.
func (a *AppService) wakeSession(id int) {
	go func() {
		for i := 0; i < wakeSettleAttempts; i++ {
			summary, err := a.host.Get(id)
			if err != nil {
				return
			}
			if !summary.Asleep() {
				return // awake already (or the suspend was aborted)
			}
			if summary.Status == hub.StatusSuspended {
				break
			}
			time.Sleep(wakeSettleInterval)
		}
		if err := a.ResumeSession(id); err != nil {
			log.Printf("[resume] implicit wake of session %d failed: %v", id, err)
		}
	}()
}

// How long wakeSession waits for an arming suspend to settle before resuming.
const (
	wakeSettleAttempts = 100
	wakeSettleInterval = 50 * time.Millisecond
)

// emitLifecycleActivity publishes a suspend/resume state to the frontend.
//
// The host's confirmed state is updated in the same step so the scan treats
// the next real state as a change, and so a waking pane does not immediately
// re-emit what it was doing before it fell asleep. ForceActivity also stamps
// the start and clears any armed candidate: setting the state alone would
// leave the badge's timestamp on the pre-sleep state and let a stale candidate
// confirm on the next tick (#188).
func (a *AppService) emitLifecycleActivity(id int, state string) {
	now := time.Now()
	_ = a.host.ForceActivity(id, hub.Activity(state), now)
	if a.app == nil {
		return
	}
	a.app.Event.Emit("terminal:activity", ActivityInfo{ID: id, Activity: state, ActivitySince: now.Unix()})
}
