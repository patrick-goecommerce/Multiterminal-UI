package main

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

func TestWaitingCell(t *testing.T) {
	now := time.Unix(10_000, 0)
	at := func(state hub.Activity, seen bool, ago time.Duration) hub.SessionSummary {
		return hub.SessionSummary{Attention: hub.Attention{State: state, Seen: seen, Since: now.Add(-ago)}}
	}
	cases := []struct {
		s    hub.SessionSummary
		want string
	}{
		{at(hub.ActivityWaitingPermission, false, 2*time.Minute), "Freigabe 2m"},
		{at(hub.ActivityWaitingAnswer, true, 30*time.Second), "Frage <1m"},
		{at(hub.ActivityDone, false, 90*time.Minute), "fertig 1h30m"},
		{at(hub.ActivityDone, true, time.Minute), "-"},
		{at(hub.ActivityActive, false, time.Minute), "-"},
		{hub.SessionSummary{}, "-"}, // an older daemon: no record
	}
	for _, c := range cases {
		if got := waitingCell(c.s, now); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.s.Attention, got, c.want)
		}
	}
}
