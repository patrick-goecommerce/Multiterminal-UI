package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// newTestApp (shared helper, initializes all maps incl. finishStates) lives
// in app_queue_test.go.

func TestStartFinish_QueueNotEmptyBlocks(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, 1, terminal.NewSession(1, 24, 80))
	a.addToQueue(1, "vorhandener prompt")
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "blocked" {
		t.Fatalf("phase = %+v, want blocked (pending queue)", st)
	}
}

func TestStartFinish_SetsPreparingAndEnqueuesPrep(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, 1, terminal.NewSession(1, 24, 80))
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "preparing" || st.PrepItemID == 0 {
		t.Fatalf("state = %+v, want preparing with PrepItemID", st)
	}
	q := a.getQueue(1)
	if len(q) != 1 || q[0].ID != st.PrepItemID {
		t.Fatalf("prep item not enqueued: %+v", q)
	}
}

func TestStartFinish_DoubleClickIsNoop(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, 1, terminal.NewSession(1, 24, 80))
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	first := a.getFinishState(1).PrepItemID
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	if got := a.getFinishState(1).PrepItemID; got != first {
		t.Errorf("second start changed PrepItemID %d → %d", first, got)
	}
	if got := len(a.getQueue(1)); got != 1 {
		t.Errorf("queue has %d items, want 1", got)
	}
}

func TestCancelFinish_ResetsStateAndRemovesPrepItem(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, 1, terminal.NewSession(1, 24, 80))
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.cancelWorktreeFinish(1)
	if st := a.getFinishState(1); st != nil {
		t.Errorf("state not cleared: %+v", st)
	}
	if got := len(a.getQueue(1)); got != 0 {
		t.Errorf("prep item not removed, queue: %d", got)
	}
}

func TestBlockedRetry_StartsNewPrepCycle(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, 1, terminal.NewSession(1, 24, 80))
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.setFinishBlocked(1, "test reason")
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "preparing" {
		t.Fatalf("retry from blocked did not re-enter preparing: %+v", st)
	}
	if st.PrepItemID == 0 {
		t.Fatal("retry must enqueue a fresh prep item (survives the Task-8 AddToQueue finish lock)")
	}
}

func TestNotifyFinishOnActivity_WaitingKeepsPreparing(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, 1, terminal.NewSession(1, 24, 80))
	a.startWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.notifyFinishOnActivity(1, "waitingAnswer")
	if st := a.getFinishState(1); st == nil || st.Phase != "preparing" {
		t.Fatalf("waitingAnswer must NOT change phase: %+v", st)
	}
	// Non-finish sessions and other states are ignored:
	a.notifyFinishOnActivity(2, "waitingAnswer")
	a.notifyFinishOnActivity(1, "active")
}
