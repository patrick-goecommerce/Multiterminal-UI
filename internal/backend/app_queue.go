package backend

import (
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
func (a *App) AddToQueue(sessionId int, prompt string) QueueItem {
	a.mu.Lock()
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
func (a *App) GetQueue(sessionId int) []QueueItem {
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
func (a *App) RemoveFromQueue(sessionId int, itemId int) {
	a.mu.Lock()
	q := a.queues[sessionId]
	if q != nil {
		for i, item := range q.items {
			if item.ID == itemId && item.Status != "sent" {
				q.items = append(q.items[:i], q.items[i+1:]...)
				log.Printf("[queue] session %d: removed item %d", sessionId, itemId)
				break
			}
		}
	}
	a.mu.Unlock()
	a.emitQueueUpdate(sessionId)
}

// ClearDoneFromQueue removes all completed items from the queue.
func (a *App) ClearDoneFromQueue(sessionId int) {
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
func (a *App) ClearQueue(sessionId int) {
	a.mu.Lock()
	delete(a.queues, sessionId)
	a.mu.Unlock()
	a.emitQueueUpdate(sessionId)
}

// tryProcessQueue sends the next pending item if the session is ready.
func (a *App) tryProcessQueue(sessionId int) {
	prevActivityMu.Lock()
	act := prevActivity[sessionId]
	prevActivityMu.Unlock()

	if act == "done" || act == "idle" || act == "" {
		a.processQueue(sessionId)
	}
}

// processQueue advances the queue: marks "sent" as "done", sends next "pending".
// Called on activity→done transitions and when new items are added to idle sessions.
func (a *App) processQueue(sessionId int) {
	a.mu.Lock()
	q := a.queues[sessionId]
	if q == nil || len(q.items) == 0 {
		a.mu.Unlock()
		return
	}

	// Mark current "sent" item as "done"
	for i := range q.items {
		if q.items[i].Status == "sent" {
			q.items[i].Status = "done"
			break
		}
	}

	// Find and send the next "pending" item
	var next *QueueItem
	for i := range q.items {
		if q.items[i].Status == "pending" {
			q.items[i].Status = "sent"
			next = &q.items[i]
			break
		}
	}

	sess := a.sessions[sessionId]
	a.mu.Unlock()

	if next != nil && sess != nil {
		sess.Write([]byte(next.Prompt + "\n"))
		log.Printf("[queue] session %d: sent item %d: %q", sessionId, next.ID, truncateStr(next.Prompt, 60))
	}

	a.emitQueueUpdate(sessionId)
}

// emitQueueUpdate notifies the frontend that a session's queue changed.
func (a *App) emitQueueUpdate(sessionId int) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "queue:update", sessionId)
}
