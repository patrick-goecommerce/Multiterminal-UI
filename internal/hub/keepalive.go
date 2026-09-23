package hub

import (
	"log"
	"time"
)

// The keep-alive.
//
// An agent CLI that sits idle long enough stops being useful: its session goes
// cold. The keep-alive sends a short message after a stretch of silence, so
// the pane is warm when its owner comes back.
//
// It runs on the host, and that is the whole point. In a window it stops the
// moment the window closes, which is precisely the stretch it exists for: a
// daemon holding agents overnight with nobody watching is the case where a
// session goes cold unnoticed. It also has to see every session, not one
// window's, and the host is the only thing that does.
//
// What stays with the window: creating a pane when none exists. That adds a
// pane to a tab, which is a thing only a window has.

// KeepAlive is the policy for one tick, as the caller sees it now.
type KeepAlive struct {
	// Every is how long a session may be silent before it is nudged. Zero
	// turns the keep-alive off.
	Every time.Duration
	// Message is what gets typed. An empty message turns it off: sending a
	// bare Return into an agent is not a keep-alive, it is a stray keystroke.
	Message string
	// Modes are the session modes eligible for a nudge. It is data rather than
	// a rule this package knows, because which CLIs need one is launch policy
	// and the host has no opinion about agents.
	Modes []string
}

// off reports whether this policy does nothing.
func (k KeepAlive) off() bool {
	return k.Every <= 0 || k.Message == "" || len(k.Modes) == 0
}

// keepAlivePoll is how often the host looks, independent of the configured
// interval. Checking only once per interval would mean a pane crossing the
// threshold waits up to another full interval, which for a multi-hour setting
// is most of a working day.
const keepAlivePoll = time.Minute

// keepAliveLoop nudges an idle agent for as long as the host lives.
func (h *Embedded) keepAliveLoop() {
	ticker := time.NewTicker(keepAlivePoll)
	defer ticker.Stop()
	var lastPing time.Time
	for {
		select {
		case <-h.stop:
			return
		case now := <-ticker.C:
			if id, ok := h.keepAliveTarget(now, lastPing); ok {
				h.sendKeepAlive(id)
				lastPing = now
			}
		}
	}
}

// keepAliveTarget picks the session to nudge, if any.
//
// The oldest eligible session wins: IDs are handed out in order, so the lowest
// one is the pane that has been open longest, and nudging a different pane
// each time would scatter the message across a workspace.
func (h *Embedded) keepAliveTarget(now, lastPing time.Time) (int, bool) {
	policy := h.keepAlive()
	if policy.off() {
		return 0, false
	}
	// Do not nudge twice within one interval, whatever the sessions look like.
	if !lastPing.IsZero() && now.Sub(lastPing) < policy.Every {
		return 0, false
	}

	eligible := make(map[string]bool, len(policy.Modes))
	for _, m := range policy.Modes {
		eligible[m] = true
	}

	target := 0
	var newest time.Time
	for _, s := range h.List() {
		// Anything that produced output counts towards the silence, including
		// a shell: somebody working in the next pane is not idle.
		if s.LastOutputAt.After(newest) {
			newest = s.LastOutputAt
		}
		if s.Status != StatusRunning || !eligible[s.Mode] {
			continue
		}
		if target == 0 || s.ID < target {
			target = s.ID
		}
	}
	if target == 0 {
		return 0, false
	}
	// Silence is measured across every session, not just the target's: a
	// message typed into a pane nobody left is noise.
	if !newest.IsZero() && now.Sub(newest) < policy.Every {
		return 0, false
	}
	return target, true
}

// sendKeepAlive types the message and submits it, the same way the queue does
// and for the same reason: an agent's redraw can swallow a Return that arrives
// in the same write as the text.
func (h *Embedded) sendKeepAlive(id int) {
	policy := h.keepAlive()
	if err := h.Write(id, []byte(policy.Message)); err != nil {
		log.Printf("[keepalive] session %d: %v", id, err)
		return
	}
	time.Sleep(enterDelay)
	if err := h.Write(id, []byte("\r")); err != nil {
		log.Printf("[keepalive] session %d: submitting: %v", id, err)
		return
	}
	log.Printf("[keepalive] nudged session %d after %s of silence", id, policy.Every)
}

// keepAlive reads the current policy, or a zero value when none was
// configured. It is read per tick rather than captured once, so a setting
// changed in a running app applies without restarting anything.
func (h *Embedded) keepAlive() KeepAlive {
	if h.keepAliveFn == nil {
		return KeepAlive{}
	}
	return h.keepAliveFn()
}
