package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// The prompt queue, as the window sees it.
//
// The queue itself is the host's (internal/hub/queue.go): an enqueued task has
// to keep moving while no window is open, which is the whole point of the
// daemon. What stays here is the worktree-finish flow, which enqueues a prep
// prompt and watches for it to complete. That is product workflow, it asks the
// user questions, and it has no business in a session host.
//
// So this file is two things: the finish-flow guards around the queue calls,
// and the bindings the frontend already uses.

// QueueItem is the frontend-facing shape. Aliased rather than redeclared: two
// identical structs would be one Wails deserialization away from diverging.
type QueueItem = hub.QueueItem

// addToQueue adds a prompt to a session's queue.
func (a *AppService) addToQueue(sessionId int, prompt string) QueueItem {
	// The queue is closed to new items while a finish flow runs. The prep item
	// itself is enqueued BEFORE the state is created, which is why this check
	// can be unconditional.
	a.mu.Lock()
	st := a.finishStates[sessionId]
	a.mu.Unlock()
	if st != nil {
		log.Printf("[queue] session %d: rejected item during finish phase %q", sessionId, st.Phase)
		return QueueItem{}
	}

	item, err := a.host.QueueAdd(sessionId, prompt)
	if err != nil {
		log.Printf("[queue] session %d: add failed: %v", sessionId, err)
		return QueueItem{}
	}
	return item
}

// getQueue returns the current queue for a session.
func (a *AppService) getQueue(sessionId int) []QueueItem {
	return a.host.QueueList(sessionId)
}

// removeFromQueue removes a single item. An item already in flight stays.
func (a *AppService) removeFromQueue(sessionId int, itemId int) {
	removed, err := a.host.QueueRemove(sessionId, itemId, false)
	if err != nil {
		log.Printf("[queue] session %d: remove failed: %v", sessionId, err)
		return
	}
	if !removed {
		return
	}
	// Removing the prep prompt is how a user cancels a finish flow.
	if st := a.getFinishState(sessionId); st != nil && st.PrepItemID == itemId {
		a.mu.Lock()
		delete(a.finishStates, sessionId)
		a.mu.Unlock()
		a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen (Prep-Prompt entfernt)")
	}
}

// clearDoneFromQueue removes the completed items.
func (a *AppService) clearDoneFromQueue(sessionId int) {
	if err := a.host.QueueClear(sessionId, true); err != nil {
		log.Printf("[queue] session %d: clearing done items failed: %v", sessionId, err)
	}
}

// clearQueue removes everything, which also cancels a finish flow that was
// still waiting for its prep prompt.
func (a *AppService) clearQueue(sessionId int) {
	if err := a.host.QueueClear(sessionId, false); err != nil {
		log.Printf("[queue] session %d: clear failed: %v", sessionId, err)
		return
	}
	if st := a.getFinishState(sessionId); st != nil && st.Phase == "preparing" {
		a.mu.Lock()
		delete(a.finishStates, sessionId)
		a.mu.Unlock()
		a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen (Queue geleert)")
	}
}

// onQueueUpdate forwards the host's queue changes to the frontend.
func (a *AppService) onQueueUpdate(sessionId int) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("queue:update", a.ref(sessionId))
}

// queueBusy reports whether a session still has prompts coming. The idle
// suspend asks: putting a pane to sleep with work waiting for it would stall
// the queue until somebody woke it by hand.
func (a *AppService) queueBusy(sessionId int) bool {
	for _, item := range a.host.QueueList(sessionId) {
		if item.Status == hub.QueuePending || item.Status == hub.QueueSent {
			return true
		}
	}
	return false
}

// truncateStr shortens a string for a log line or a prompt echo.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
