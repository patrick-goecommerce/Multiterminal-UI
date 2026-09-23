package backend

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// The frontend gets a session as "hub:id" and hands exactly that back. What
// CreateSession returns has to resolve to the same session on the way in.
func TestSessionRef_RoundTripsThroughTheFrontend(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	r := a.CreateSession(sleepArgvForTest(), sessionDir(t), 24, 80, "shell")
	if r.IsZero() || r.Hub != a.host.Info().HubID {
		t.Fatalf("CreateSession returned %q, want a ref on hub %q", r, a.host.Info().HubID)
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back hub.Ref
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !a.AttachSession(back, 24, 80) {
		t.Errorf("AttachSession(%s) refused the session CreateSession returned", b)
	}
}

// A ref for another hub names somebody else's session 3. Acting on our own
// session 3 instead would type into the wrong agent, and a legacy ref without
// a hub cannot say which one it meant.
func TestSessionRef_ForeignOrLegacyRefIsNoSession(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	id := a.createSession(sleepArgvForTest(), sessionDir(t), 24, 80, "shell")
	if id <= 0 {
		t.Fatalf("createSession returned %d", id)
	}
	for _, r := range []hub.Ref{hub.Local("another-hub", id), {ID: id}, {}} {
		if got := a.local(r); got != 0 {
			t.Errorf("local(%+v) = %d, want 0", r, got)
		}
		if a.AttachSession(r, 24, 80) {
			t.Errorf("AttachSession(%+v) attached a session of this hub", r)
		}
	}
	if got := a.local(a.ref(id)); got != id {
		t.Errorf("local(ref(%d)) = %d", id, got)
	}
}

type seedCall struct {
	id    int
	state hub.Activity
	at    time.Time
}

// seedRecorder is a Host that notes SeedActivity calls and forwards the rest.
type seedRecorder struct {
	hub.Host
	calls []seedCall
}

func (s *seedRecorder) SeedActivity(id int, state hub.Activity, at time.Time) error {
	s.calls = append(s.calls, seedCall{id, state, at})
	return nil
}

// The restore has called SeedActivitySince since #189, and the binding went
// missing when the debounce moved to the host. The call then failed quietly
// in the frontend and every pane's duration started over after a restart.
func TestSeedActivitySince_ReachesTheHost(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	rec := &seedRecorder{Host: a.host}
	a.host = rec

	a.SeedActivitySince(a.ref(4), 1700000000, "done")
	a.SeedActivitySince(a.ref(4), 0, "done")                           // no timestamp
	a.SeedActivitySince(a.ref(4), 1700000000, "")                      // no state
	a.SeedActivitySince(hub.Local("elsewhere", 4), 1700000000, "done") // not ours

	want := seedCall{4, hub.Activity("done"), time.Unix(1700000000, 0)}
	if len(rec.calls) != 1 || rec.calls[0] != want {
		t.Errorf("seeds = %+v, want exactly %+v", rec.calls, want)
	}
}
