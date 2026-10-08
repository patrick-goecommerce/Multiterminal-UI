package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// AttentionInfo is what the frontend learns about a session's attention
// record (terminal:attention). An event payload: it needs no models.ts mirror.
type AttentionInfo struct {
	ID     hub.Ref `json:"id" yaml:"id"`
	State  string  `json:"state" yaml:"state"`
	Rank   int     `json:"rank" yaml:"rank"`
	Seen   bool    `json:"seen" yaml:"seen"`
	Detail string  `json:"detail" yaml:"detail"`
	Source string  `json:"source" yaml:"source"`
	// Since is unix seconds, 0 when unknown.
	Since int64 `json:"since" yaml:"since"`
}

// attentionInfo recomputes the rank instead of trusting the record's: a
// daemon from before attention existed sends a zero Rank, which is
// "permission".
func attentionInfo(ref hub.Ref, a hub.Attention) AttentionInfo {
	return AttentionInfo{
		ID:     ref,
		State:  string(a.State),
		Rank:   hub.AttentionRank(a.State, a.Seen),
		Seen:   a.Seen,
		Detail: a.Detail,
		Source: a.Source,
		Since:  unixOrZero(a.Since),
	}
}

// emitAttention forwards one record to the frontend.
func (a *AppService) emitAttention(id int, att hub.Attention) {
	if a.app == nil || att.State == "" {
		return
	}
	a.app.Event.Emit("terminal:attention", attentionInfo(a.ref(id), att))
}

// markSessionSeen asks the host to mark a session seen (MarkSessionSeen). 0
// is a ref this window cannot resolve: nothing to mark.
func (a *AppService) markSessionSeen(id int) {
	if id == 0 {
		return
	}
	if err := a.host.MarkSeen(id); err != nil {
		log.Printf("[attention] MarkSeen %d: %v", id, err)
	}
}

// markSessionUnseen is markSessionSeen's way back (MarkSessionUnseen).
func (a *AppService) markSessionUnseen(id int) {
	if id == 0 {
		return
	}
	if err := a.host.MarkUnseen(id); err != nil {
		log.Printf("[attention] MarkUnseen %d: %v", id, err)
	}
}

// dashboardActivity is the confirmed state, the one the pane badge shows. The
// raw reading flickers and disagreed with the badge (spec Befund 9).
func dashboardActivity(s hub.SessionSummary) string {
	if s.Attention.State != "" && !s.Asleep() {
		return string(s.Attention.State)
	}
	return string(s.Activity)
}
