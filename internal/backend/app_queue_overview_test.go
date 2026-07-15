package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ---------------------------------------------------------------------------
// GetAllQueues — cross-session queue aggregation
// ---------------------------------------------------------------------------

func TestGetAllQueues_Empty(t *testing.T) {
	app := newTestApp()
	result := app.GetAllQueues()
	if len(result) != 0 {
		t.Errorf("expected 0 queue items, got %d", len(result))
	}
}

func TestGetAllQueues_SkipsEmptyQueues(t *testing.T) {
	app := newTestApp()
	sess := terminal.NewSession(1, 24, 80)
	app.sessions[1] = sess
	app.queues[1] = &sessionQueue{items: []QueueItem{}}

	result := app.GetAllQueues()
	if len(result) != 0 {
		t.Errorf("expected 0 (empty queue skipped), got %d", len(result))
	}
}

func TestGetAllQueues_SkipsOrphanQueues(t *testing.T) {
	app := newTestApp()
	// Queue exists but no session
	app.queues[99] = &sessionQueue{items: []QueueItem{{ID: 1, Prompt: "x", Status: "pending"}}}

	result := app.GetAllQueues()
	if len(result) != 0 {
		t.Errorf("expected 0 (orphan queue skipped), got %d", len(result))
	}
}

func TestGetAllQueues_ReturnsMatchingQueues(t *testing.T) {
	app := newTestApp()
	sess := terminal.NewSession(1, 24, 80)
	app.sessions[1] = sess
	app.queues[1] = &sessionQueue{
		items: []QueueItem{
			{ID: 1, Prompt: "hello", Status: "pending"},
			{ID: 2, Prompt: "world", Status: "sent"},
		},
	}

	result := app.GetAllQueues()
	if len(result) != 1 {
		t.Fatalf("expected 1 overview item, got %d", len(result))
	}
	if result[0].SessionID != 1 {
		t.Errorf("session ID = %d, want 1", result[0].SessionID)
	}
	if len(result[0].Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result[0].Items))
	}
}
