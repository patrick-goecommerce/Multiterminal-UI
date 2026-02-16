package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func newTestApp() *App {
	return &App{
		sessions: make(map[int]*terminal.Session),
		queues:   make(map[int]*sessionQueue),
	}
}

func TestAddToQueue(t *testing.T) {
	a := newTestApp()
	item := a.addToQueueInternal(1, "prompt 1")
	if item.ID != 1 {
		t.Fatalf("expected ID 1, got %d", item.ID)
	}
	if item.Status != "pending" {
		t.Fatalf("expected pending, got %s", item.Status)
	}
	if item.Prompt != "prompt 1" {
		t.Fatalf("expected 'prompt 1', got %q", item.Prompt)
	}
}

func TestGetQueue(t *testing.T) {
	a := newTestApp()
	a.addToQueueInternal(1, "first")
	a.addToQueueInternal(1, "second")
	a.addToQueueInternal(2, "other session")

	q := a.GetQueue(1)
	if len(q) != 2 {
		t.Fatalf("expected 2 items, got %d", len(q))
	}
	if q[0].Prompt != "first" || q[1].Prompt != "second" {
		t.Fatalf("unexpected prompts: %v", q)
	}

	q2 := a.GetQueue(2)
	if len(q2) != 1 {
		t.Fatalf("expected 1 item for session 2, got %d", len(q2))
	}

	// Non-existent session returns empty slice
	q3 := a.GetQueue(999)
	if len(q3) != 0 {
		t.Fatalf("expected empty queue, got %d items", len(q3))
	}
}

func TestRemoveFromQueue(t *testing.T) {
	a := newTestApp()
	a.addToQueueInternal(1, "keep")
	item2 := a.addToQueueInternal(1, "remove me")
	a.addToQueueInternal(1, "also keep")

	a.RemoveFromQueue(1, item2.ID)

	q := a.GetQueue(1)
	if len(q) != 2 {
		t.Fatalf("expected 2 items after removal, got %d", len(q))
	}
	for _, it := range q {
		if it.ID == item2.ID {
			t.Fatal("removed item should not be in queue")
		}
	}
}

func TestRemoveFromQueueCannotRemoveSent(t *testing.T) {
	a := newTestApp()
	item := a.addToQueueInternal(1, "in flight")

	// Manually mark as sent
	a.mu.Lock()
	a.queues[1].items[0].Status = "sent"
	a.mu.Unlock()

	a.RemoveFromQueue(1, item.ID)

	q := a.GetQueue(1)
	if len(q) != 1 {
		t.Fatal("sent items should not be removable")
	}
}

func TestClearDoneFromQueue(t *testing.T) {
	a := newTestApp()
	a.addToQueueInternal(1, "done item")
	a.addToQueueInternal(1, "pending item")

	// Mark first as done
	a.mu.Lock()
	a.queues[1].items[0].Status = "done"
	a.mu.Unlock()

	a.ClearDoneFromQueue(1)

	q := a.GetQueue(1)
	if len(q) != 1 {
		t.Fatalf("expected 1 item after clearing done, got %d", len(q))
	}
	if q[0].Prompt != "pending item" {
		t.Fatalf("wrong item remaining: %q", q[0].Prompt)
	}
}

func TestClearQueue(t *testing.T) {
	a := newTestApp()
	a.addToQueueInternal(1, "one")
	a.addToQueueInternal(1, "two")

	a.ClearQueue(1)

	q := a.GetQueue(1)
	if len(q) != 0 {
		t.Fatalf("expected empty queue after clear, got %d items", len(q))
	}
}

func TestProcessQueueAdvancesItems(t *testing.T) {
	a := newTestApp()
	a.addToQueueInternal(1, "first")
	a.addToQueueInternal(1, "second")

	// Simulate: mark first as "sent" (as if it was dispatched)
	a.mu.Lock()
	a.queues[1].items[0].Status = "sent"
	a.mu.Unlock()

	// processQueue should: mark "sent" → "done", pick next "pending" → "sent"
	a.processQueue(1)

	q := a.GetQueue(1)
	if q[0].Status != "done" {
		t.Fatalf("expected first item done, got %s", q[0].Status)
	}
	if q[1].Status != "sent" {
		t.Fatalf("expected second item sent, got %s", q[1].Status)
	}
}

func TestProcessQueueEmptyIsNoop(t *testing.T) {
	a := newTestApp()
	// Should not panic on empty/non-existent queue
	a.processQueue(999)
}

func TestQueueHasStatus(t *testing.T) {
	items := []QueueItem{
		{ID: 1, Status: "pending"},
		{ID: 2, Status: "done"},
	}

	if !queueHasStatus(items, "pending") {
		t.Fatal("should find pending")
	}
	if !queueHasStatus(items, "done") {
		t.Fatal("should find done")
	}
	if queueHasStatus(items, "sent") {
		t.Fatal("should not find sent")
	}
}

func TestTruncateStr(t *testing.T) {
	if truncateStr("short", 10) != "short" {
		t.Fatal("short string should not be truncated")
	}
	result := truncateStr("this is a very long string", 10)
	if result != "this is a ..." {
		t.Fatalf("unexpected truncation: %q", result)
	}
}

// addToQueueInternal adds to queue without triggering events (for testing).
func (a *App) addToQueueInternal(sessionId int, prompt string) QueueItem {
	a.mu.Lock()
	defer a.mu.Unlock()
	q := a.queues[sessionId]
	if q == nil {
		q = &sessionQueue{}
		a.queues[sessionId] = q
	}
	q.nextID++
	item := QueueItem{ID: q.nextID, Prompt: prompt, Status: "pending"}
	q.items = append(q.items, item)
	return item
}
