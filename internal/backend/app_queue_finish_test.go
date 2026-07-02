package backend

import (
	"testing"
	"time"
)

// pinPrevActivity keeps AddToQueue's pre-existing auto-dispatch
// (tryProcessQueue) from firing during test setup. Unit tests have no real
// session, so prevActivity[sessionId] defaults to "" on first read, which
// tryProcessQueue treats as idle and immediately flips a freshly enqueued
// prep item from "pending" to "sent" — before the test can exercise it. A
// real session mid-turn would report a busy activity here instead, so this
// mirrors production. Also guards against state leaking to other tests via
// the shared prevActivity map (session ID 1 is reused across this file).
func pinPrevActivity(t *testing.T, sessionId int, activity string) {
	t.Helper()
	prevActivityMu.Lock()
	prevActivity[sessionId] = activity
	prevActivityMu.Unlock()
	t.Cleanup(func() {
		prevActivityMu.Lock()
		delete(prevActivity, sessionId)
		prevActivityMu.Unlock()
	})
}

func TestProcessQueue_ReportsItemDone(t *testing.T) {
	a := newTestApp()
	pinPrevActivity(t, 1, "generating")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	prepID := a.getFinishState(1).PrepItemID
	// Simulate the scan loop: first done sends the item, second done completes it.
	a.processQueue(1) // pending → sent (no session ⇒ write skipped, status still advances)
	a.processQueue(1) // sent → done ⇒ onQueueItemDone(1, prepID) fires
	q := a.GetQueue(1)
	if len(q) != 1 || q[0].ID != prepID || q[0].Status != "done" {
		t.Fatalf("prep item not completed: %+v", q)
	}
	// CheckWorktreeFinish runs async against a nonexistent path ⇒ blocked.
	// Wait briefly for the goroutine, then assert the transition happened.
	deadline := time.After(2 * time.Second)
	for {
		st := a.getFinishState(1)
		if st != nil && st.Phase == "blocked" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("onQueueItemDone never advanced state: %+v", a.getFinishState(1))
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRemovePrepItem_ResetsFinish(t *testing.T) {
	a := newTestApp()
	pinPrevActivity(t, 1, "generating")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	prepID := a.getFinishState(1).PrepItemID
	a.RemoveFromQueue(1, prepID)
	if st := a.getFinishState(1); st != nil {
		t.Errorf("finish state survived prep item removal: %+v", st)
	}
}

func TestClearQueue_ResetsFinish(t *testing.T) {
	a := newTestApp()
	pinPrevActivity(t, 1, "generating")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.ClearQueue(1)
	if st := a.getFinishState(1); st != nil {
		t.Errorf("finish state survived ClearQueue: %+v", st)
	}
}

func TestAddToQueue_LockedDuringFinish(t *testing.T) {
	a := newTestApp()
	pinPrevActivity(t, 1, "generating")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	item := a.AddToQueue(1, "sollte abgelehnt werden")
	if item.ID != 0 {
		t.Errorf("queue accepted item during active finish: %+v", item)
	}
	if got := len(a.GetQueue(1)); got != 1 {
		t.Errorf("queue length %d, want 1 (only prep item)", got)
	}
}
