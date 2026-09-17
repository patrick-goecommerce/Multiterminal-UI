package backend

import (
	"fmt"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
)

// A pane for a session this window did not open.
//
// The agent-control tools themselves are internal/mcpsrv now: opening, feeding
// and closing a session is host work, and putting it here meant an agent could
// only delegate while a window happened to be open. What is left is the one
// half that genuinely needs a window, namely showing the session, and it keys
// off the host's own event, so it works the same whether the session was
// created in this process or in the daemon while nobody was watching.

// SessionSpawnedEvent tells the frontend to attach a visible pane to a session
// that was created outside this window's UI.
type SessionSpawnedEvent struct {
	ID    int    `json:"id" yaml:"id"`
	Tool  string `json:"tool" yaml:"tool"`
	Model string `json:"model" yaml:"model"`
	Dir   string `json:"dir" yaml:"dir"`
	Name  string `json:"name" yaml:"name"`
	// Origin is who asked for it: "agent" through the MCP server, "cli"
	// through mt new. The frontend shows both the same way; it is here so a
	// pane can say where it came from without a second event.
	Origin string `json:"origin" yaml:"origin"`
}

// onSessionCreated reacts to a new session on the host.
func (a *AppService) onSessionCreated(s hub.SessionSummary) {
	ev, ok := spawnedEvent(s)
	if !ok || a.app == nil {
		return
	}
	a.app.Event.Emit("mtui:session-spawned", ev)
}

// spawnedEvent decides whether a session needs a pane drawn for it, and what
// to call it.
//
// Only sessions with an origin do. An empty origin is a window's own UI, which
// already drew its pane when it asked for the session; drawing a second one
// would double every pane in the app.
func spawnedEvent(s hub.SessionSummary) (SessionSpawnedEvent, bool) {
	if s.Origin == "" {
		return SessionSpawnedEvent{}, false
	}
	name := launch.AgentDisplayName(s.Mode)
	if s.Model != "" {
		name = fmt.Sprintf("%s (%s)", name, s.Model)
	}
	return SessionSpawnedEvent{
		ID:     s.ID,
		Tool:   s.Mode,
		Model:  s.Model,
		Dir:    s.Dir,
		Name:   name,
		Origin: s.Origin,
	}, true
}
