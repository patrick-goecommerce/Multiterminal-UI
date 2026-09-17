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
// half that genuinely needs a window — showing the delegated session — and it
// keys off the host's own event, so it works the same whether the session was
// created in this process or in the daemon while nobody was watching.

// AgentSessionSpawnedEvent tells the frontend to attach a visible pane to a
// session that was created outside the UI.
type AgentSessionSpawnedEvent struct {
	ID    int    `json:"id" yaml:"id"`
	Tool  string `json:"tool" yaml:"tool"`
	Model string `json:"model" yaml:"model"`
	Dir   string `json:"dir" yaml:"dir"`
	Name  string `json:"name" yaml:"name"`
}

// onSessionCreated reacts to a new session on the host.
//
// Only the agent-spawned ones get a pane from here. Everything else is either
// this window's own creation, which already has one, or a session it has no
// business drawing.
func (a *AppService) onSessionCreated(s hub.SessionSummary) {
	if s.Origin != hub.OriginAgent || a.app == nil {
		return
	}
	name := launch.AgentDisplayName(s.Mode)
	if s.Model != "" {
		name = fmt.Sprintf("%s (%s)", name, s.Model)
	}
	a.app.Event.Emit("mtui:session-spawned", AgentSessionSpawnedEvent{
		ID:    s.ID,
		Tool:  s.Mode,
		Model: s.Model,
		Dir:   s.Dir,
		Name:  name,
	})
}
