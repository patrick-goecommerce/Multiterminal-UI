package hub

import (
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// MarkSeen records that the person looked at the session. It clears "done"
// and "error" from the waiting list; a permission or a question stays,
// because only the agent moving on answers those.
func (h *Embedded) MarkSeen(id int) error { return h.setSeen(id, true) }

// MarkUnseen puts a finished or failed session back on the waiting list.
func (h *Embedded) MarkUnseen(id int) error { return h.setSeen(id, false) }

func (h *Embedded) setSeen(id int, seen bool) error {
	if _, err := h.lookup(id); err != nil {
		return err
	}
	if a, changed := h.attention.markSeen(id, seen); changed {
		h.emit(EventSessionAttention, AttentionUpdate{ID: id, Attention: a})
	}
	return nil
}

// recordConfirmed turns a confirmed state into a new attention record and
// tells the clients. Lifecycle labels (sleeping, resuming) leave it alone.
func (h *Embedded) recordConfirmed(id int, state Activity, since time.Time) {
	if a, changed := h.attention.confirmed(id, state, since); changed {
		h.emit(EventSessionAttention, AttentionUpdate{ID: id, Attention: a})
	}
}

// subagentWorkAfterTurn reports a subagent's tool event arriving after the
// main turn ended. Claude Code marks those with agent_id. A background
// subagent at work must not repaint a finished pane as running: that hid the
// closing question of the turn (spec Teil C4). A subagent that needs a
// permission or asks something still counts, because it blocks.
func subagentWorkAfterTurn(ev hooks.Event, next Activity, current terminal.ActivityState) bool {
	return ev.AgentID != "" && next == ActivityActive &&
		(current == terminal.ActivityDone || current == terminal.ActivityWaitingAnswer)
}
