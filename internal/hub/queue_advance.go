package hub

import (
	"log"
	"time"
)

// Advancing the queue.
//
// This is the part that has to be next to the sessions: it reads the agent's
// state, writes to the PTY, and wakes a sleeping pane. All three are things a
// window cannot do once the sessions live somewhere else.

// QueueAdvance implements Host.
//
// It marks the item in flight as done, then sends the next pending one. A
// sleeping pane is woken instead: writing to it would fail and the item would
// be marked sent without ever being delivered.
func (h *Embedded) QueueAdvance(id int) {
	summary, err := h.Get(id)
	if err != nil {
		return
	}
	if summary.Asleep() {
		// Leave the queue untouched. The resumed pane's next confirmed "done"
		// runs this again, and by then there is a process to write to.
		if h.hasPending(id) {
			log.Printf("[queue] session %d: queued while asleep, waking up", id)
			go func() {
				if err := h.Wake(id); err != nil {
					log.Printf("[queue] session %d: wake failed: %v", id, err)
				}
			}()
		}
		return
	}

	h.queueMu.Lock()
	q := h.queues[id]
	h.queueMu.Unlock()
	if q == nil {
		return
	}

	q.mu.Lock()
	if len(q.items) == 0 {
		q.mu.Unlock()
		return
	}
	doneID := 0
	for i := range q.items {
		if q.items[i].Status == QueueSent {
			q.items[i].Status = QueueDone
			doneID = q.items[i].ID
			break
		}
	}
	var next QueueItem
	hasNext := false
	for i := range q.items {
		if q.items[i].Status == QueuePending {
			q.items[i].Status = QueueSent
			next = q.items[i]
			hasNext = true
			break
		}
	}
	q.mu.Unlock()

	if doneID != 0 {
		// Somebody else decides what a finished item means. The host only
		// knows that one finished.
		h.emit(EventQueueItemDone, QueueItemDone{SessionID: id, ItemID: doneID})
	}
	if hasNext {
		h.sendQueued(id, next)
	}
	h.emitQueue(id, q)
}

// hasPending reports whether anything is waiting to be sent.
func (h *Embedded) hasPending(id int) bool {
	h.queueMu.Lock()
	q := h.queues[id]
	h.queueMu.Unlock()
	if q == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.hasStatus(QueuePending)
}

// sendQueued types an item into the session and submits it.
func (h *Embedded) sendQueued(id int, item QueueItem) {
	if err := h.Write(id, []byte(item.Prompt)); err != nil {
		log.Printf("[queue] session %d: writing item %d: %v", id, item.ID, err)
		return
	}
	time.Sleep(enterDelay)
	if err := h.Write(id, []byte("\r")); err != nil {
		log.Printf("[queue] session %d: submitting item %d: %v", id, item.ID, err)
		return
	}
	log.Printf("[queue] session %d: sent item %d: %q", id, item.ID, truncate(item.Prompt, 60))

	// Put the state back to idle so the NEXT "done" reads as a transition.
	// Without this the confirmed state is still "done" from the previous item
	// and the scan would never see a change, leaving the queue stuck with an
	// item in flight forever. ForceActivity also stamps the start and clears
	// any armed candidate: setting the state alone would leave a stale
	// candidate that confirms on one unrelated tick (#188).
	_ = h.ResetActivity(id)
	_ = h.ForceActivity(id, ActivityIdle, time.Now())
}

// advanceQueuesAfterScan runs the queue for every session that just confirmed
// finishing its turn.
//
// Only "done", deliberately. A settled "idle" means the classifier did not
// recognise the screen (a pager, a TUI, a running npm script), not that the
// agent is ready for the next prompt, and typing into a pager is worse than
// waiting. A caller that knows better about one session (MTUI's
// worktree-finish flow does) nudges it with QueueAdvance.
func (h *Embedded) advanceQueuesAfterScan(results []ScanResult) {
	for _, r := range results {
		if r.Asleep || !r.Changed || r.Activity != ActivityDone {
			continue
		}
		if !h.hasPending(r.ID) && !h.hasInFlight(r.ID) {
			continue
		}
		h.QueueAdvance(r.ID)
	}
}

// hasInFlight reports whether an item is waiting to be marked done.
func (h *Embedded) hasInFlight(id int) bool {
	h.queueMu.Lock()
	q := h.queues[id]
	h.queueMu.Unlock()
	if q == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.hasStatus(QueueSent)
}
