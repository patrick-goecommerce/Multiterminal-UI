package hub

import (
	"context"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

// The hook reader belongs to the host for the same reason the scan does: an
// agent reports what it is doing through lifecycle hooks, and those reports
// have to be recorded while no window is open. What follows from an event in
// the UI (naming a pane, tracking a worktree, showing a blocked-path dialog)
// is the client's business, so every event is passed on as EventSessionHook.

// startHookReader tails dir and applies what it finds to this host's sessions.
func (h *Embedded) startHookReader(dir string) {
	// The callback needs the watcher (to forget a finished session's file) and
	// the watcher needs the callback, so the variable breaks the circle. It is
	// assigned before Start, which is what any of it runs from.
	var w *hooks.Watcher
	w = hooks.NewWatcher(dir, func(ev hooks.Event) { h.applyHookEvent(w, ev) })
	h.hookWatcher = w
	go w.Start(stopContext(h.stop))
}

// stopContext turns the host's stop channel into a context, which is what the
// watcher takes.
func stopContext(stop <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stop
		cancel()
	}()
	return ctx
}

// applyHookEvent records a hook event against its session and reports it.
func (h *Embedded) applyHookEvent(w *hooks.Watcher, ev hooks.Event) {
	m, err := h.lookup(ev.MtID)
	if err != nil {
		return // an event for a session this host does not have
	}

	// Record the agent's own session UUID on the first event that carries one.
	if ev.SessionID != "" && m.sess.HookSessionID() == "" {
		m.sess.SetHookSessionID(ev.SessionID)
	}

	report := HookReport{
		Session:        ev.MtID,
		Event:          ev.Event,
		AgentSessionID: ev.SessionID,
		Tool:           ev.Tool,
		Message:        ev.Message,
		Cwd:            ev.Cwd,
		WorktreePath:   ev.WorktreePath,
		WorktreeBranch: ev.WorktreeBranch,
		BlockedPath:    ev.BlockedPath,
		BlockReason:    ev.BlockReason,
	}

	if ev.Event == "SessionEnd" {
		m.sess.ClearHookData()
		if w != nil {
			w.Forget(ev.SessionID)
		}
		h.emit(EventSessionHook, report)
		return
	}

	if activity, ok := hookActivity(ev); ok {
		m.sess.SetHookActivity(terminalActivity(activity))
		report.Activity = activity
	}
	h.emit(EventSessionHook, report)
}
