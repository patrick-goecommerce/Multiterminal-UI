package backend

import (
	"log"
	"time"
)

// QueueItem represents a single prompt in a session's pipeline queue.
type QueueItem struct {
	ID     int    `json:"id"`
	Prompt string `json:"prompt"`
	Status string `json:"status"` // "pending", "sent", "done"
}

// sessionQueue holds the pipeline queue for a single session.
type sessionQueue struct {
	items  []QueueItem
	nextID int
}

func queueHasStatus(items []QueueItem, status string) bool {
	for _, it := range items {
		if it.Status == status {
			return true
		}
	}
	return false
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// AddToQueue adds a prompt to a session's pipeline queue.
// If the session is idle/done and nothing is in-flight, it triggers immediately.
func (a *AppService) AddToQueue(sessionId int, prompt string) QueueItem {
	a.mu.Lock()
	// Queue is locked for new items while a finish flow is active. The prep
	// item itself is enqueued BEFORE the state is created (task 7 ordering).
	if st := a.finishStates[sessionId]; st != nil {
		a.mu.Unlock()
		log.Printf("[queue] session %d: rejected item during finish phase %q", sessionId, st.Phase)
		return QueueItem{}
	}
	q := a.queues[sessionId]
	if q == nil {
		q = &sessionQueue{}
		a.queues[sessionId] = q
	}
	q.nextID++
	item := QueueItem{ID: q.nextID, Prompt: prompt, Status: "pending"}
	q.items = append(q.items, item)
	shouldTrigger := !queueHasStatus(q.items, "sent")
	a.mu.Unlock()

	log.Printf("[queue] session %d: added item %d: %q", sessionId, item.ID, truncateStr(prompt, 60))
	a.emitQueueUpdate(sessionId)

	if shouldTrigger {
		a.tryProcessQueue(sessionId)
	}
	return item
}

// GetQueue returns the current pipeline queue for a session.
func (a *AppService) GetQueue(sessionId int) []QueueItem {
	a.mu.Lock()
	defer a.mu.Unlock()
	q := a.queues[sessionId]
	if q == nil {
		return []QueueItem{}
	}
	result := make([]QueueItem, len(q.items))
	copy(result, q.items)
	return result
}

// RemoveFromQueue removes a single item by ID.
// Items with status "sent" (currently executing) cannot be removed.
func (a *AppService) RemoveFromQueue(sessionId int, itemId int) {
	a.mu.Lock()
	q := a.queues[sessionId]
	removed := false
	if q != nil {
		for i, item := range q.items {
			if item.ID == itemId && item.Status != "sent" {
				q.items = append(q.items[:i], q.items[i+1:]...)
				log.Printf("[queue] session %d: removed item %d", sessionId, itemId)
				removed = true
				break
			}
		}
	}
	a.mu.Unlock()
	a.emitQueueUpdate(sessionId)

	if removed {
		if st := a.getFinishState(sessionId); st != nil && st.PrepItemID == itemId {
			a.mu.Lock()
			delete(a.finishStates, sessionId)
			a.mu.Unlock()
			a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen (Prep-Prompt entfernt)")
		}
	}
}

// ClearDoneFromQueue removes all completed items from the queue.
func (a *AppService) ClearDoneFromQueue(sessionId int) {
	a.mu.Lock()
	q := a.queues[sessionId]
	if q != nil {
		filtered := make([]QueueItem, 0, len(q.items))
		for _, item := range q.items {
			if item.Status != "done" {
				filtered = append(filtered, item)
			}
		}
		q.items = filtered
	}
	a.mu.Unlock()
	a.emitQueueUpdate(sessionId)
}

// ClearQueue removes all items from a session's queue.
func (a *AppService) ClearQueue(sessionId int) {
	a.mu.Lock()
	delete(a.queues, sessionId)
	a.mu.Unlock()
	a.emitQueueUpdate(sessionId)

	if st := a.getFinishState(sessionId); st != nil && st.Phase == "preparing" {
		a.mu.Lock()
		delete(a.finishStates, sessionId)
		a.mu.Unlock()
		a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen (Queue geleert)")
	}
}

// tryProcessQueue sends the next pending item if the session is ready.
//
// "sleeping" must be in this set: a sleeping pane emits no further activity of
// its own, so without it AddToQueue would enqueue an item that nothing ever
// picks up — a silent hang. processQueue turns the attempt into a wake-up.
func (a *AppService) tryProcessQueue(sessionId int) {
	prevActivityMu.Lock()
	act := prevActivity[sessionId]
	prevActivityMu.Unlock()

	if act == "done" || act == "idle" || act == "" || act == "sleeping" {
		a.processQueue(sessionId)
	}
}

// processQueue advances the queue: marks "sent" as "done", sends next "pending".
// Called on activity→done transitions and when new items are added to idle sessions.
func (a *AppService) processQueue(sessionId int) {
	// A sleeping pane cannot take a prompt: sess.Write would fail and the item
	// would be marked "sent" without ever being delivered. Wake it instead and
	// leave the queue untouched — the resumed pane's next "done" transition
	// (scanAllSessions) runs processQueue again.
	a.mu.Lock()
	suspendCandidate := a.sessions[sessionId]
	a.mu.Unlock()
	if suspendCandidate != nil && suspendCandidate.IsSuspendedOrSuspending() {
		log.Printf("[queue] session %d: queued while asleep — waking up", sessionId)
		a.wakeSession(sessionId)
		return
	}

	a.mu.Lock()
	q := a.queues[sessionId]
	if q == nil || len(q.items) == 0 {
		a.mu.Unlock()
		return
	}

	// Mark current "sent" item as "done"
	doneItemID := 0
	for i := range q.items {
		if q.items[i].Status == "sent" {
			q.items[i].Status = "done"
			doneItemID = q.items[i].ID
			break
		}
	}

	// Find and send the next "pending" item (copy value to avoid dangling pointer after unlock)
	var next QueueItem
	var hasNext bool
	for i := range q.items {
		if q.items[i].Status == "pending" {
			q.items[i].Status = "sent"
			next = q.items[i]
			hasNext = true
			break
		}
	}

	sess := a.sessions[sessionId]
	a.mu.Unlock()

	if doneItemID != 0 {
		a.onQueueItemDone(sessionId, doneItemID)
	}

	if hasNext && sess != nil {
		// Write prompt text first, then Enter separately with a small delay.
		// Claude Code's TUI needs time to process the pasted text before
		// receiving the Enter key — writing everything in one chunk can cause
		// the \r to be swallowed.
		_, err := sess.Write([]byte(next.Prompt))
		if err != nil {
			log.Printf("[queue] session %d: write error for item %d: %v", sessionId, next.ID, err)
		} else {
			time.Sleep(100 * time.Millisecond)
			_, err = sess.Write([]byte("\r"))
			if err != nil {
				log.Printf("[queue] session %d: enter error for item %d: %v", sessionId, next.ID, err)
			} else {
				log.Printf("[queue] session %d: sent item %d: %q", sessionId, next.ID, truncateStr(next.Prompt, 60))
			}
		}
		// Reset activity so the next "done" transition is detected as a change.
		// Without this, prevActivity might already be "done" from the previous
		// item, causing the scan loop to miss the transition.
		sess.ResetActivity()
		prevActivityMu.Lock()
		prevActivity[sessionId] = "idle"
		prevActivityMu.Unlock()
	}

	a.emitQueueUpdate(sessionId)
}

// emitQueueUpdate notifies the frontend that a session's queue changed.
func (a *AppService) emitQueueUpdate(sessionId int) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("queue:update", sessionId)
}
