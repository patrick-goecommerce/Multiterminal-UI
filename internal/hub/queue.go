package hub

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// The prompt queue.
//
// A queue is a list of prompts for one session, sent one at a time: the next
// one goes out when the agent finishes the previous one. It lives here rather
// than in a window because an enqueued task has to keep moving while nobody is
// watching, which is the whole reason the daemon exists. A queue in the window
// means a task queued at 18:00 waits until somebody opens the app again.
//
// What is NOT here is what a queue event means to a product: MTUI's
// worktree-finish flow enqueues a prep prompt and watches for it to complete.
// That is workflow, it asks the user questions, and it stays with the window.
// The host emits the two facts it knows (the queue changed, an item finished)
// and takes no view on what they are for.

// QueueItem is one prompt waiting for a session.
type QueueItem struct {
	ID     int    `json:"id" yaml:"id"`
	Prompt string `json:"prompt" yaml:"prompt"`
	// Status is "pending", "sent" or "done".
	Status string `json:"status" yaml:"status"`
}

// Queue item statuses.
const (
	QueuePending = "pending"
	QueueSent    = "sent"
	QueueDone    = "done"
)

// enterDelay is how long the queue waits between the prompt text and the
// Return that submits it.
//
// Agent CLIs redraw their input box as characters arrive, and a Return in the
// same write is sometimes swallowed by that redraw. The pause is below human
// perception and removes the whole class of "the prompt was typed but never
// sent".
const enterDelay = 100 * time.Millisecond

// sessionQueue is one session's list.
type sessionQueue struct {
	mu     sync.Mutex
	items  []QueueItem
	nextID int
}

func (q *sessionQueue) snapshot() []QueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]QueueItem, len(q.items))
	copy(out, q.items)
	return out
}

func (q *sessionQueue) hasStatus(status string) bool {
	for _, it := range q.items {
		if it.Status == status {
			return true
		}
	}
	return false
}

// queueFor returns a session's queue, creating it on first use.
func (h *Embedded) queueFor(id int) (*sessionQueue, error) {
	if _, err := h.lookup(id); err != nil {
		return nil, err
	}
	h.queueMu.Lock()
	defer h.queueMu.Unlock()
	q := h.queues[id]
	if q == nil {
		q = &sessionQueue{}
		h.queues[id] = q
	}
	return q, nil
}

// QueueAdd implements Host.
func (h *Embedded) QueueAdd(id int, prompt string) (QueueItem, error) {
	q, err := h.queueFor(id)
	if err != nil {
		return QueueItem{}, err
	}

	q.mu.Lock()
	q.nextID++
	item := QueueItem{ID: q.nextID, Prompt: prompt, Status: QueuePending}
	q.items = append(q.items, item)
	inFlight := q.hasStatus(QueueSent)
	q.mu.Unlock()

	log.Printf("[queue] session %d: added item %d: %q", id, item.ID, truncate(prompt, 60))
	h.emitQueue(id, q)
	// Send it now only if nothing is in flight AND the agent is between
	// turns. Both halves matter: without the first the prompts would pile on
	// top of each other, and without the second a prompt added while the agent
	// is mid-turn would be typed straight into its working screen. An agent
	// that is busy gets it on its next confirmed "done".
	if !inFlight && h.readyForPrompt(id) {
		h.QueueAdvance(id)
	}
	return item, nil
}

// QueueList implements Host.
func (h *Embedded) QueueList(id int) []QueueItem {
	h.queueMu.Lock()
	q := h.queues[id]
	h.queueMu.Unlock()
	if q == nil {
		return []QueueItem{}
	}
	return q.snapshot()
}

// QueueRemove implements Host. An item that is already in flight cannot be
// taken back: the agent has it.
func (h *Embedded) QueueRemove(id, itemID int, force bool) (bool, error) {
	q, err := h.queueFor(id)
	if err != nil {
		return false, err
	}
	q.mu.Lock()
	removed := false
	for i, item := range q.items {
		if item.ID == itemID && (force || item.Status != QueueSent) {
			q.items = append(q.items[:i], q.items[i+1:]...)
			removed = true
			break
		}
	}
	q.mu.Unlock()
	if removed {
		log.Printf("[queue] session %d: removed item %d", id, itemID)
	}
	h.emitQueue(id, q)
	return removed, nil
}

// QueueClear implements Host. doneOnly keeps everything still to come.
func (h *Embedded) QueueClear(id int, doneOnly bool) error {
	q, err := h.queueFor(id)
	if err != nil {
		return err
	}
	q.mu.Lock()
	if doneOnly {
		kept := make([]QueueItem, 0, len(q.items))
		for _, item := range q.items {
			if item.Status != QueueDone {
				kept = append(kept, item)
			}
		}
		q.items = kept
	} else {
		q.items = nil
	}
	q.mu.Unlock()
	h.emitQueue(id, q)
	return nil
}

// readyForPrompt reports whether a session can take a prompt right now.
//
// A sleeping pane counts: it cannot take one, but QueueAdvance turns the
// attempt into a wake-up, and without that an item enqueued for a sleeping
// pane would sit there until somebody woke it by hand.
func (h *Embedded) readyForPrompt(id int) bool {
	summary, err := h.Get(id)
	if err != nil {
		return false
	}
	if summary.Asleep() {
		return true
	}
	state, _ := h.ConfirmedActivity(id)
	return state == "" || state == ActivityIdle || state == ActivityDone
}

// forgetQueue drops a closed session's queue.
func (h *Embedded) forgetQueue(id int) {
	h.queueMu.Lock()
	delete(h.queues, id)
	h.queueMu.Unlock()
}

// emitQueue reports a session's queue to whoever is listening.
func (h *Embedded) emitQueue(id int, q *sessionQueue) {
	h.emit(EventQueueUpdate, QueueUpdate{SessionID: id, Items: q.snapshot()})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

var _ = fmt.Sprintf
