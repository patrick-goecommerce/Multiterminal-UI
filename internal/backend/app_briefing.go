package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// BriefingInfo is what the frontend learns when a session's turn was measured
// (terminal:briefing). An event payload: it needs no models.ts mirror.
type BriefingInfo struct {
	ID       hub.Ref      `json:"id" yaml:"id"`
	Briefing hub.Briefing `json:"briefing" yaml:"briefing"`
}

// emitBriefing forwards one briefing to the frontend.
func (a *AppService) emitBriefing(id int, b hub.Briefing) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("terminal:briefing", BriefingInfo{ID: a.ref(id), Briefing: b})
}

// sessionBriefing asks the host for a session's last briefing. Any error means
// "no briefing": an older mtuid answers the route with 404, which Remote
// reports as ErrNoSession, and that must not read as a dead session.
func (a *AppService) sessionBriefing(id int) *hub.Briefing {
	if id == 0 {
		return nil
	}
	b, ok, err := a.host.Briefing(id)
	if err != nil || !ok {
		return nil
	}
	return &b
}

// setSessionTarget tells the host which branch a session's work is measured
// against. A failure is logged and changes nothing else.
func (a *AppService) setSessionTarget(id int, branch string) {
	if id == 0 {
		return
	}
	if err := a.host.SetTarget(id, branch); err != nil {
		log.Printf("[briefing] SetTarget: %v", err)
	}
}
