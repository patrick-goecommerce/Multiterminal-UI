package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// Which sessions a window draws a pane for.
//
// The rule has to hold in both directions. Draw for too few and a delegated
// session is invisible until the next restart; draw for too many and every
// pane the user opens appears twice, because the frontend already added one
// when it asked for the session.

func TestSpawnedEvent_OnlyForSessionsFromElsewhere(t *testing.T) {
	tests := []struct {
		origin string
		want   bool
	}{
		{"", false},
		{hub.OriginAgent, true},
		{hub.OriginCLI, true},
	}

	for _, tt := range tests {
		t.Run("origin="+tt.origin, func(t *testing.T) {
			got, ok := spawnedEvent(hub.SessionSummary{
				ID: 7, Mode: "claude", Dir: "/tmp", Origin: tt.origin,
			})
			if ok != tt.want {
				t.Fatalf("a pane for origin %q: %t, want %t", tt.origin, ok, tt.want)
			}
			if ok && got.ID != 7 {
				t.Errorf("event carried session %d, want 7", got.ID)
			}
		})
	}
}

// The pane's label is the one thing the event exists to carry that the summary
// does not already say plainly.
func TestSpawnedEvent_NamesThePane(t *testing.T) {
	got, _ := spawnedEvent(hub.SessionSummary{
		ID: 3, Mode: "claude", Model: "opus", Origin: hub.OriginAgent,
	})
	if got.Name != "Claude (opus)" {
		t.Errorf("name = %q, want %q", got.Name, "Claude (opus)")
	}

	got, _ = spawnedEvent(hub.SessionSummary{ID: 4, Mode: "codex", Origin: hub.OriginCLI})
	if got.Name != "Codex" {
		t.Errorf("name = %q, want %q", got.Name, "Codex")
	}
}
