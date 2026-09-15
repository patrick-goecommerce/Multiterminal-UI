package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// testHost builds a session host holding sessions the test constructed itself.
//
// Most of these sessions have no process behind them: they are screens in a
// known state, or objects whose activity a test sets by hand. AdoptForTest is
// what puts such a session into the host without starting anything.
func testHost(sessions map[int]*terminal.Session) *hub.Embedded {
	h := hub.NewEmbedded(hub.Options{})
	for id, s := range sessions {
		h.AdoptForTest(id, s)
	}
	return h
}

// countingSessions is a hookSessions that owns nothing and counts lookups, for
// the case where the manager must not look anything up at all.
type countingSessions struct{ gets int }

func (c *countingSessions) Get(int) (hub.SessionSummary, error) {
	c.gets++
	return hub.SessionSummary{}, hub.ErrNoSession
}
func (c *countingSessions) SetHookSessionID(int, string) error      { return hub.ErrNoSession }
func (c *countingSessions) SetHookActivity(int, hub.Activity) error { return hub.ErrNoSession }
func (c *countingSessions) ClearHookData(int) error                 { return hub.ErrNoSession }

// adopt puts a hand-built session into an app's host. AppService.host is the
// Host interface, so the test-only adoption hatch needs the concrete type.
func adopt(t *testing.T, a *AppService, id int, sess *terminal.Session) {
	t.Helper()
	h, ok := a.host.(*hub.Embedded)
	if !ok {
		t.Fatalf("app host is %T, want an embedded host", a.host)
	}
	h.AdoptForTest(id, sess)
}
