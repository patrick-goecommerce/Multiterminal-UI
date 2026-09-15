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
