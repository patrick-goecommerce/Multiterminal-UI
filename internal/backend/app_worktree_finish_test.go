package backend

import (
	"testing"
)

// newTestApp (shared helper, initializes all maps incl. finishStates) lives
// in app_queue_test.go.

func TestStartFinish_QueueNotEmptyBlocks(t *testing.T) {
	a := newTestApp()
	a.AddToQueue(1, "vorhandener prompt")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "blocked" {
		t.Fatalf("phase = %+v, want blocked (pending queue)", st)
	}
}

func TestStartFinish_SetsPreparingAndEnqueuesPrep(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "preparing" || st.PrepItemID == 0 {
		t.Fatalf("state = %+v, want preparing with PrepItemID", st)
	}
	q := a.GetQueue(1)
	if len(q) != 1 || q[0].ID != st.PrepItemID {
		t.Fatalf("prep item not enqueued: %+v", q)
	}
}

func TestStartFinish_DoubleClickIsNoop(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	first := a.getFinishState(1).PrepItemID
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	if got := a.getFinishState(1).PrepItemID; got != first {
		t.Errorf("second start changed PrepItemID %d → %d", first, got)
	}
	if got := len(a.GetQueue(1)); got != 1 {
		t.Errorf("queue has %d items, want 1", got)
	}
}

func TestCancelFinish_ResetsStateAndRemovesPrepItem(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.CancelWorktreeFinish(1)
	if st := a.getFinishState(1); st != nil {
		t.Errorf("state not cleared: %+v", st)
	}
	if got := len(a.GetQueue(1)); got != 0 {
		t.Errorf("prep item not removed, queue: %d", got)
	}
}

func TestBlockedRetry_StartsNewPrepCycle(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.setFinishBlocked(1, "test reason")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "preparing" {
		t.Fatalf("retry from blocked did not re-enter preparing: %+v", st)
	}
	if st.PrepItemID == 0 {
		t.Fatal("retry must enqueue a fresh prep item (survives the Task-8 AddToQueue finish lock)")
	}
}
