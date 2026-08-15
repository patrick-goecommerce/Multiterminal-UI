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
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// launchSpec remembers how a session was started so a resume can rebuild an
// identical command line.
type launchSpec struct {
	argv []string
	dir  string
	mode string
}

// isClaudeMode reports whether the mode is backed by the claude CLI. Only those
// understand --resume; codex/gemini have no verified resume path (design D6).
func isClaudeMode(mode string) bool {
	return mode == "claude" || mode == "claude-auto" || mode == "claude-yolo"
}

// sessionEnv builds the PTY environment for a session.
//
// CreateSession and ResumeSession MUST both use it: a woken pane that loses
// MULTITERMINAL_SESSION_ID drops its hook and statusline wiring (the whole
// activity detection goes back to screen scraping), and one that loses
// MULTITERMINAL_FORCE_WORKTREE_ROOT silently loses the worktree firewall in
// cmd/mtui-hook. Both failures are invisible until something goes wrong.
func (a *AppService) sessionEnv(id int, dir, mode string) []string {
	var env []string
	if a.tmuxAPIPort > 0 {
		env = append(env, fmt.Sprintf("MTUI_PORT=%d", a.tmuxAPIPort))
	}
	if isClaudeMode(mode) {
		env = append(env, fmt.Sprintf("MULTITERMINAL_SESSION_ID=%d", id))
		env = append(env, worktreeEnvVars(dir)...)
		// Worktree-mandatory policy, resolved once here (global setting +
		// per-project override) so mtui-hook only has to read one env var.
		// Empty when the policy is off or dir is not a git repo.
		if root := a.forceWorktreeRoot(dir); root != "" {
			env = append(env, "MULTITERMINAL_FORCE_WORKTREE_ROOT="+root)
		}
	}
	return env
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

// claudeSessionIDFromArgv extracts the Claude session UUID from a launch argv.
// The frontend generates it (claude.ts) and passes it as --session-id, or as
// --resume when a conversation is continued; the backend never saw it before.
//
// sess.HookSessionID() stays authoritative — it is also right when Claude picks
// a different UUID internally — this is only the value known at launch time.
// A bare `--resume` (the interactive picker) has no value and yields "".
func claudeSessionIDFromArgv(argv []string) string {
	for i, arg := range argv {
		switch {
		case arg == "--session-id", arg == "--resume":
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				return argv[i+1]
			}
		case strings.HasPrefix(arg, "--session-id="):
			return strings.TrimPrefix(arg, "--session-id=")
		case strings.HasPrefix(arg, "--resume="):
			return strings.TrimPrefix(arg, "--resume=")
		}
	}
	return ""
}

// resumeArgv rewrites a launch argv into a resume argv: every existing
// --session-id/--resume is dropped (they are mutually exclusive) and a single
// `--resume <id>` is appended.
func resumeArgv(argv []string, resumeID string) []string {
	out := make([]string, 0, len(argv)+2)
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--session-id" || arg == "--resume" {
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++ // skip the value too
			}
			continue
		}
		if strings.HasPrefix(arg, "--session-id=") || strings.HasPrefix(arg, "--resume=") {
			continue
		}
		out = append(out, arg)
	}
	if resumeID != "" {
		out = append(out, "--resume", resumeID)
	}
	return out
}

// resumeIDFor returns the UUID to resume a session with: the hook-reported one
// wins, the argv-parsed one is the fallback.
func resumeIDFor(sess *terminal.Session) string {
	if id := sess.HookSessionID(); id != "" {
		return id
	}
	return sess.ResumeID()
}

// SuspendSession puts a finished Claude pane to sleep. Returns an error when
// the pane is not eligible; the kill itself runs asynchronously because
// taskkill takes 100–300 ms and must never run under a lock.
func (a *AppService) SuspendSession(id int) error {
	a.mu.Lock()
	sess := a.sessions[id]
	mode := a.sessionMode[id]
	a.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("session %d not found", id)
	}
	if !isClaudeMode(mode) {
		return fmt.Errorf("sleeping is only supported for claude panes (mode %q)", mode)
	}
	if sess.IsSuspendedOrSuspending() {
		return nil // already asleep or on its way — idempotent
	}
	resumeID := resumeIDFor(sess)
	if resumeID == "" {
		return errors.New("no claude session id known for this pane — it cannot be resumed")
	}
	sess.SetResumeID(resumeID)

	// Phase one of the two-phase commit: re-check under Session.mu.
	if !sess.TrySuspend() {
		return errors.New("pane is not idle — only a finished (done) claude pane can sleep")
	}
	go a.completeSuspend(id, sess)
	return nil
}

// completeSuspend runs the kill outside every lock and finalises the suspend.
func (a *AppService) completeSuspend(id int, sess *terminal.Session) {
	// Phase two: readLoop flags any chunk that arrived after arming. Killing a
	// pane that just woke up would destroy work in flight.
	if sess.SuspendAborted() {
		sess.AbortSuspend()
		log.Printf("[suspend] session %d: aborted, output arrived after arming", id)
		return
	}

	// Order is mandatory: taskkill /T must see the whole tree. After
	// Process.Kill() (inside FinishSuspend) the grandchildren are orphaned and
	// the node/MCP processes survive.
	killProcessTree(sess.Pid())
	if !sess.FinishSuspend() {
		log.Printf("[suspend] session %d: FinishSuspend found no armed suspend", id)
		return
	}
	log.Printf("[suspend] session %d asleep (resume id %s)", id, sess.ResumeID())
	a.emitLifecycleActivity(id, "sleeping")
}

// ResumeSession wakes a sleeping pane by restarting claude with --resume into
// the same session object. Calling it on an awake pane is a no-op.
func (a *AppService) ResumeSession(id int) error {
	a.mu.Lock()
	sess := a.sessions[id]
	spec := a.launches[id]
	a.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("session %d not found", id)
	}
	if !sess.IsSuspended() {
		return nil
	}

	resumeID := resumeIDFor(sess)
	if resumeID == "" {
		return errors.New("no claude session id known for this pane — it cannot be resumed")
	}
	dir := spec.dir
	if dir == "" {
		dir = sess.Dir
	}
	argv := resumeArgv(spec.argv, resumeID)
	env := a.sessionEnv(id, dir, spec.mode)

	if err := sess.Resume(argv, dir, env); err != nil {
		if errors.Is(err, terminal.ErrNotSuspended) {
			return nil // a concurrent wake won the race — nothing to do
		}
		log.Printf("[resume] session %d failed: %v", id, err)
		return err
	}
	log.Printf("[resume] session %d waking up (resume id %s)", id, resumeID)
	// ~12–15 s pass before Claude has replayed the transcript; the pane shows
	// "wacht auf" until the scan loop reports a real state again.
	a.emitLifecycleActivity(id, "resuming")
	return nil
}

// IsSessionSuspended reports whether a pane is currently asleep.
func (a *AppService) IsSessionSuspended(id int) bool {
	a.mu.Lock()
	sess := a.sessions[id]
	a.mu.Unlock()
	return sess != nil && sess.IsSuspended()
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
			a.mu.Lock()
			sess := a.sessions[id]
			a.mu.Unlock()
			if sess == nil {
				return
			}
			if !sess.IsSuspendedOrSuspending() {
				return // awake already (or the suspend was aborted)
			}
			if sess.IsSuspended() {
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
// prevActivity is updated in the same step so the scan loop treats the next
// real state as a change (and, for "sleeping", does not immediately re-emit the
// state it saw before the pane fell asleep). forceActivity also stamps
// activitySince to now and clears any armed debounce candidate — a bare
// prevActivity write would leave the badge's timestamp on the state the pane
// had before sleeping, and let a stale candidate confirm on the next tick
// (issue #188).
func (a *AppService) emitLifecycleActivity(id int, state string) {
	now := time.Now()
	prevActivityMu.Lock()
	forceActivity(id, state, now)
	prevActivityMu.Unlock()
	if a.app == nil {
		return
	}
	a.app.Event.Emit("terminal:activity", ActivityInfo{ID: id, Activity: state, ActivitySince: now.Unix()})
}
