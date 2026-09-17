package backend

import (
	"strings"
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

// A pane is not just a row in the tab store: the window has to stream the
// session's bytes and know its mode. Without the subscription the pane appears
// and stays black, which is exactly what happened when the MCP server stopped
// going through CreateSession.
func TestOnSessionCreated_AdoptsTheSession(t *testing.T) {
	a := newTestApp()

	id, err := a.host.Create(hub.CreateSpec{
		Argv:   printArgv("delegiert"),
		Dir:    sessionDir(t),
		Mode:   "claude",
		Origin: hub.OriginCLI,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = a.host.Close(id) })

	a.mu.Lock()
	mode := a.sessionMode[id]
	a.mu.Unlock()
	if mode != "claude" {
		t.Errorf("mode = %q, want %q — mode-dependent features read this", mode, "claude")
	}

	if got := drainBatcher(t, a, id, "delegiert"); !strings.Contains(got, "delegiert") {
		t.Errorf("the session's output never reached the pane: %q", got)
	}
}

// The window's own panes are already streamed by CreateSession. A second
// subscription would deliver every byte twice.
func TestAdoptSession_IsNotDoneTwice(t *testing.T) {
	a := newTestApp()
	a.mu.Lock()
	a.sessionMode[9] = "claude"
	a.mu.Unlock()

	// No session with this ID exists, so a stream attempt would log a failed
	// attach. What is asserted is that it does not get that far.
	a.adoptSession(hub.SessionSummary{ID: 9, Mode: "shell"})

	a.mu.Lock()
	mode := a.sessionMode[9]
	a.mu.Unlock()
	if mode != "claude" {
		t.Errorf("mode = %q, want the window's own %q", mode, "claude")
	}
}
