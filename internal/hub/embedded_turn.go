package hub

import (
	"context"
	"errors"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

// A turn runs from the user's prompt to the agent's Stop (or the process's
// end, or a hang). When it ends the host measures it in git and publishes a
// Briefing, so whoever looks at the pane next knows what happened without
// reading the scrollback.

// onTurnHook folds a hook event into the session's current turn.
func (h *Embedded) onTurnHook(ev hooks.Event) {
	id := ev.MtID
	switch ev.Event {
	case "UserPromptSubmit":
		cwd := ev.Cwd
		if cwd == "" {
			cwd = h.sessionDir(id)
		}
		if keepAlive := h.desk.turns.begin(id, ev.PromptID, ev.Message, cwd); keepAlive || cwd == "" {
			return
		}
		go h.desk.startSnapshot(id, ev.PromptID, cwd)
	case "PostToolUse", "PostToolUseFailure":
		h.desk.turns.observe(id, ev)
		if publishCommand(ev.Command) {
			h.ciPublished(id)
		}
	case "Stop":
		h.finishTurn(id, false, closingWords(ev.Message))
	}
}

// finishTurn closes the session's turn and builds its briefing in the
// background. A turn that is not open, or was the keep-alive's own, changes
// nothing: the last briefing stays.
func (h *Embedded) finishTurn(id int, endedError bool, closing string) {
	t, ok := h.desk.turns.close(id, "")
	if !ok || t.keepAlive {
		return
	}
	builtAt := time.Now().Unix()
	ctx, gen := h.desk.begin(id)
	go h.buildBriefing(ctx, id, gen, t, endedError, closing, builtAt)
}

// buildBriefing measures a finished turn and publishes the result, unless a
// newer build replaced it or the session went away meanwhile.
func (h *Embedded) buildBriefing(ctx context.Context, id int, gen uint64, t turn, endedError bool, closing string, builtAt int64) {
	d := h.desk
	select {
	case d.sem <- struct{}{}:
	case <-ctx.Done():
		d.finish(id, gen, nil)
		return
	}
	defer func() { <-d.sem }()

	b := Briefing{
		PromptID: t.promptID, BuiltAt: builtAt, Prompt: t.prompt, Closing: closing, Checks: t.checks,
	}
	var res gitResult
	if t.cwd != "" {
		pol := d.currentPolicy()
		limited, cancel := context.WithTimeout(ctx, d.limit)
		r, isRepo := measureGit(limited, d.snapshot, t.cwd, t.startTree, t.startTop, d.target(id), d.prBase, pol)
		slow := errors.Is(limited.Err(), context.DeadlineExceeded)
		cancel()
		if ctx.Err() != nil {
			d.finish(id, gen, nil) // superseded or forgotten
			return
		}
		res = r
		if isRepo {
			f := r.Facts
			b.Git = &f
			// What the watcher knows goes in now, not one gh round-trip later.
			if pr := d.knownPR(r, pol); pr != nil {
				b.PR = pr
				if ciStale(pr, r.Head) {
					b.Notes = append(b.Notes, NoteCIStale)
				}
			}
		}
		if slow {
			b.Notes = append(b.Notes, NoteDiffSlow)
		}
	}
	b.Warnings = warningsFor(b, endedError, res.BigDiff)
	b = b.clone()

	if _, err := h.lookup(id); err != nil {
		d.finish(id, gen, nil)
		return
	}
	if !d.finish(id, gen, &deskEntry{briefing: b, endedError: endedError, bigDiff: res.BigDiff, res: res}) {
		return
	}
	h.emit(EventSessionBriefing, BriefingUpdate{ID: id, Briefing: b.clone()})
	// After the event, so whatever the watcher learns next goes out after
	// this build's own briefing and is not painted over by it.
	if d.afterBuild != nil {
		d.afterBuild(id, res, t.publish)
	}
}

// Briefing returns the session's last briefing; false when no turn of it has
// been measured yet.
func (h *Embedded) Briefing(id int) (Briefing, bool, error) {
	if _, err := h.lookup(id); err != nil {
		return Briefing{}, false, err
	}
	b, ok := h.desk.get(id)
	return b, ok, nil
}

// SetTarget tells the host which branch the window means as the session's
// target. Empty clears it.
func (h *Embedded) SetTarget(id int, branch string) error {
	if _, err := h.lookup(id); err != nil {
		return err
	}
	h.desk.setTarget(id, branch)
	return nil
}

// sessionDir is the directory the session was started in.
func (h *Embedded) sessionDir(id int) string {
	m, err := h.lookup(id)
	if err != nil {
		return ""
	}
	return m.spec.Dir
}
