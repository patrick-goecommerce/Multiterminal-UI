package backend

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// An older daemon sends Rank 0 with an empty state; that must not read as a
// permission request (Review Focus 2).
func TestAttentionInfo_RecomputesTheRank(t *testing.T) {
	got := attentionInfo(hub.Local("h", 1), hub.Attention{State: hub.ActivityDone, Rank: 0})
	if got.Rank != hub.RankDone {
		t.Errorf("rank %d, want %d", got.Rank, hub.RankDone)
	}
	since := time.Unix(1700000000, 0)
	got = attentionInfo(hub.Local("h", 1), hub.Attention{State: hub.ActivityWaitingAnswer, Since: since, Detail: "Pushen?"})
	if got.Since != 1700000000 || got.Detail != "Pushen?" || got.State != "waitingAnswer" {
		t.Errorf("got %+v", got)
	}
}

func TestDashboardActivity_PrefersTheConfirmedState(t *testing.T) {
	s := hub.SessionSummary{Activity: hub.ActivityActive, Attention: hub.Attention{State: hub.ActivityDone}}
	if got := dashboardActivity(s); got != "done" {
		t.Errorf("got %q, want done", got)
	}
	s.Attention = hub.Attention{}
	if got := dashboardActivity(s); got != "active" {
		t.Errorf("without a record: got %q, want the raw active", got)
	}
}
