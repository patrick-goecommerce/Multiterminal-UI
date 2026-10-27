package hub

import (
	"testing"
	"time"
)

func TestRaiseRefusesWaiting(t *testing.T) {
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Minute)

	for _, state := range []Activity{ActivityActive, ActivityWaitingPermission, ActivityWaitingAnswer} {
		b := newAttentionBook()
		before, _ := b.confirmed(1, state, t0)
		if _, ok := b.raise(1, "ci", "CI rot: backend", t1); ok {
			t.Errorf("%s: raise = true, want false", state)
		}
		if got := b.get(1); got != before {
			t.Errorf("%s: record changed to %+v, want %+v", state, got, before)
		}
	}

	b := newAttentionBook()
	b.confirmed(2, ActivityDone, t0)
	b.markSeen(2, true)
	a, ok := b.raise(2, "ci", "CI rot: backend", t1)
	if !ok {
		t.Fatal("raise on a seen done session = false, want true")
	}
	want := Attention{State: ActivityError, Since: t1, Seen: false, Source: "ci", Detail: "CI rot: backend", Rank: RankError}
	if a != want {
		t.Errorf("raise = %+v, want %+v", a, want)
	}
	if got := b.get(2); got != want {
		t.Errorf("stored = %+v, want %+v", got, want)
	}

	// A session without a record yet is quiet, and quiet can be raised.
	if a, ok := b.raise(3, "ci", "PR #214: Konflikte", t1); !ok || a.Rank != RankError {
		t.Errorf("raise on a session without a record = %+v, %v", a, ok)
	}
}
