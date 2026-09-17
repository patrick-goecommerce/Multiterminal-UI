package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ---------------------------------------------------------------------------
// cleanupActivityTracking
// ---------------------------------------------------------------------------

func TestCleanupActivityTracking_RemovesEntries(t *testing.T) {
	// This only covers the emit mirrors now. The activity half of the old
	// tracking moved to the host with the debounce, and the host forgets a
	// session in Close; TestCloseForgetsTheDebounceState in internal/hub
	// covers that side.
	prevEmitMu.Lock()
	prevCost[100] = "$1.23"
	prevTitle[100] = "eins"
	prevCost[200] = "$4.56"
	prevTitle[200] = "zwei"
	prevEmitMu.Unlock()

	cleanupActivityTracking(100)

	prevEmitMu.Lock()
	defer prevEmitMu.Unlock()

	if _, exists := prevCost[100]; exists {
		t.Fatal("cost entry for session 100 should be removed")
	}
	if _, exists := prevTitle[100]; exists {
		t.Fatal("title entry for session 100 should be removed")
	}
	if prevCost[200] != "$4.56" || prevTitle[200] != "zwei" {
		t.Fatal("session 200 should be untouched")
	}
}

func TestCleanupActivityTracking_NonExistentSession(t *testing.T) {
	// Should not panic
	cleanupActivityTracking(99999)
}

// ---------------------------------------------------------------------------
// ActivityInfo struct
// ---------------------------------------------------------------------------

func TestActivityInfo_Fields(t *testing.T) {
	info := ActivityInfo{
		ID:       5,
		Activity: "done",
		Cost:     "$0.42",
	}
	if info.ID != 5 {
		t.Fatalf("expected ID 5, got %d", info.ID)
	}
	if info.Activity != "done" {
		t.Fatalf("expected 'done', got %q", info.Activity)
	}
	if info.Cost != "$0.42" {
		t.Fatalf("expected '$0.42', got %q", info.Cost)
	}
}

// ---------------------------------------------------------------------------
// Prev activity tracking state isolation
// ---------------------------------------------------------------------------

func TestActivityTracking_IsolatedPerSession(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	adopt(t, a, 301, terminal.NewSession(301, 24, 80))
	adopt(t, a, 302, terminal.NewSession(302, 24, 80))

	setConfirmed(t, a, 301, "active")
	setConfirmed(t, a, 302, "done")
	prevEmitMu.Lock()
	prevCost[301] = "$0.10"
	prevCost[302] = "$0.20"
	prevEmitMu.Unlock()

	cleanupActivityTracking(301)
	if err := a.host.Close(301); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if state, _ := confirmedOf(a, 301); state != "" {
		t.Fatalf("session 301 kept its confirmed state %q after closing", state)
	}
	if state, _ := confirmedOf(a, 302); state != "done" {
		t.Fatalf("session 302 activity = %q, want done", state)
	}
	prevEmitMu.Lock()
	defer prevEmitMu.Unlock()
	if _, exists := prevCost[301]; exists {
		t.Fatal("session 301 cost should be cleaned up")
	}
	if prevCost[302] != "$0.20" {
		t.Fatal("session 302 cost should be untouched")
	}
}
