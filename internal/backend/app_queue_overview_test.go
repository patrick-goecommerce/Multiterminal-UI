package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ---------------------------------------------------------------------------
// GetAllQueues — cross-session queue aggregation
// ---------------------------------------------------------------------------
//
// The queues live on the host now, so these fill them through it. The orphan
// case is gone as a test because it is gone as a state: a queue belongs to a
// session and cannot outlive it.

func TestGetAllQueues_Empty(t *testing.T) {
	app := newTestApp()
	t.Cleanup(app.host.Release)
	if result := app.GetAllQueues(); len(result) != 0 {
		t.Errorf("expected 0 queue items, got %d", len(result))
	}
}

func TestGetAllQueues_SkipsSessionsWithoutQueues(t *testing.T) {
	app := newTestApp()
	t.Cleanup(app.host.Release)
	adopt(t, app, 1, terminal.NewSession(1, 24, 80))

	if result := app.GetAllQueues(); len(result) != 0 {
		t.Errorf("expected 0 (no queue on that session), got %d", len(result))
	}
}

func TestGetAllQueues_ReturnsMatchingQueues(t *testing.T) {
	app := newTestApp()
	t.Cleanup(app.host.Release)
	adopt(t, app, 1, terminal.NewSession(1, 24, 80))

	if _, err := app.host.QueueAdd(1, "hello"); err != nil {
		t.Fatalf("QueueAdd: %v", err)
	}
	if _, err := app.host.QueueAdd(1, "world"); err != nil {
		t.Fatalf("QueueAdd: %v", err)
	}

	result := app.GetAllQueues()
	if len(result) != 1 {
		t.Fatalf("expected 1 overview item, got %d", len(result))
	}
	if result[0].SessionID != app.ref(1) {
		t.Errorf("session ID = %q, want %q", result[0].SessionID, app.ref(1))
	}
	if len(result[0].Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result[0].Items))
	}
}

// A queue goes away with its session, so an overview never lists a pane that
// is no longer there.
func TestGetAllQueues_ForgetsAClosedSession(t *testing.T) {
	app := newTestApp()
	t.Cleanup(app.host.Release)
	adopt(t, app, 1, terminal.NewSession(1, 24, 80))
	if _, err := app.host.QueueAdd(1, "hello"); err != nil {
		t.Fatalf("QueueAdd: %v", err)
	}

	if err := app.host.Close(1); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if result := app.GetAllQueues(); len(result) != 0 {
		t.Errorf("expected 0 after the session closed, got %d", len(result))
	}
}
