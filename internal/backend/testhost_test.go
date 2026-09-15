package backend

import (
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
