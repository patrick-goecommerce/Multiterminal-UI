package backend

import (
	"testing"

	"github.com/patrick-goecommerce/multiterminal/internal/terminal"
)

// ---------------------------------------------------------------------------
// cleanupActivityTracking
// ---------------------------------------------------------------------------

func TestCleanupActivityTracking_RemovesEntries(t *testing.T) {
	// Set up some tracking data
	prevActivityMu.Lock()
	prevActivity[100] = "active"
	prevCost[100] = "$1.23"
	prevActivity[200] = "done"
	prevCost[200] = "$4.56"
	prevActivityMu.Unlock()

	// Clean up session 100
	cleanupActivityTracking(100)

	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	if _, exists := prevActivity[100]; exists {
		t.Fatal("activity entry for session 100 should be removed")
	}
	if _, exists := prevCost[100]; exists {
		t.Fatal("cost entry for session 100 should be removed")
	}

	// Session 200 should still exist
	if prevActivity[200] != "done" {
		t.Fatal("session 200 activity should be untouched")
	}
	if prevCost[200] != "$4.56" {
		t.Fatal("session 200 cost should be untouched")
	}
}

func TestCleanupActivityTracking_NonExistentSession(t *testing.T) {
	// Should not panic
	cleanupActivityTracking(99999)
}

// ---------------------------------------------------------------------------
// activityString – comprehensive tests
// ---------------------------------------------------------------------------

func TestActivityString_Active(t *testing.T) {
	if s := activityString(terminal.ActivityActive); s != "active" {
		t.Fatalf("expected 'active', got %q", s)
	}
}

func TestActivityString_Done(t *testing.T) {
	if s := activityString(terminal.ActivityDone); s != "done" {
		t.Fatalf("expected 'done', got %q", s)
	}
}

func TestActivityString_NeedsInput(t *testing.T) {
	if s := activityString(terminal.ActivityNeedsInput); s != "needsInput" {
		t.Fatalf("expected 'needsInput', got %q", s)
	}
}

func TestActivityString_Idle(t *testing.T) {
	if s := activityString(terminal.ActivityIdle); s != "idle" {
		t.Fatalf("expected 'idle', got %q", s)
	}
}

func TestActivityString_UnknownDefaultsToIdle(t *testing.T) {
	if s := activityString(terminal.ActivityState(255)); s != "idle" {
		t.Fatalf("expected 'idle' for unknown state, got %q", s)
	}
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

func TestPrevActivityTracking_IsolatedPerSession(t *testing.T) {
	// Clean state
	prevActivityMu.Lock()
	prevActivity[301] = "active"
	prevActivity[302] = "done"
	prevCost[301] = "$0.10"
	prevCost[302] = "$0.20"
	prevActivityMu.Unlock()

	// Cleanup only 301
	cleanupActivityTracking(301)

	prevActivityMu.Lock()
	defer prevActivityMu.Unlock()

	if _, exists := prevActivity[301]; exists {
		t.Fatal("session 301 should be cleaned up")
	}
	if prevActivity[302] != "done" {
		t.Fatal("session 302 should be untouched")
	}
	if prevCost[302] != "$0.20" {
		t.Fatal("session 302 cost should be untouched")
	}

	// Clean up 302 to avoid test pollution
	delete(prevActivity, 302)
	delete(prevCost, 302)
}
