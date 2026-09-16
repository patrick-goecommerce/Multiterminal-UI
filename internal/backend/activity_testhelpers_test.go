package backend

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// The debounce lives on the host now (internal/hub/activity_debounce.go), so a
// test that wants a session to be in a known confirmed state asks the host
// rather than writing a package-level map.
//
// That is also why there is no reset helper any more: the state belongs to an
// Embedded, and every test builds its own, so two tests can no longer leak
// into each other through a global.

// setConfirmed puts a session in a confirmed state, starting now.
func setConfirmed(t *testing.T, a *AppService, id int, state string) {
	t.Helper()
	setConfirmedSince(t, a, id, state, time.Now())
}

// setConfirmedSince puts a session in a confirmed state that began at since.
func setConfirmedSince(t *testing.T, a *AppService, id int, state string, since time.Time) {
	t.Helper()
	if err := a.host.ForceActivity(id, hub.Activity(state), since); err != nil {
		t.Fatalf("ForceActivity(%d, %q): %v", id, state, err)
	}
}

// confirmedOf reads back what the host has confirmed for a session.
func confirmedOf(a *AppService, id int) (string, time.Time) {
	state, since := a.host.ConfirmedActivity(id)
	return string(state), since
}

// currentStateForTest is the confirmed state, for a rewrite that only wants to
// move the timestamp and leave the state where it is.
func currentStateForTest(a *AppService, id int) string {
	state, _ := a.host.ConfirmedActivity(id)
	return string(state)
}

// backdateCandidate pushes an armed debounce candidate past the window so the
// next scan confirms it, instead of sleeping the test. It fails when nothing
// was armed, because a test that back-dates nothing and then asserts a
// confirmation would pass without having exercised the debounce at all.
func backdateCandidate(t *testing.T, a *AppService, id int) {
	t.Helper()
	emb, ok := a.host.(*hub.Embedded)
	if !ok {
		t.Fatalf("host is %T, not an *hub.Embedded", a.host)
	}
	if !emb.BackdateActivityForTest(id) {
		t.Fatal("no debounce candidate was armed; the scan never observed the new state")
	}
}
