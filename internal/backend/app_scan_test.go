package backend

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// TestActivityInfoCarriesStatuslineFields guards that the event payload exposes
// context%/model so the frontend can render them.
func TestActivityInfoCarriesStatuslineFields(t *testing.T) {
	info := ActivityInfo{ID: hub.Local("h1", 1), Activity: "active", Cost: "$1.23", ContextPct: 40, Model: "Opus 4.8"}
	if info.ContextPct != 40 || info.Model != "Opus 4.8" {
		t.Fatalf("ActivityInfo = %+v, want ContextPct=40 Model=Opus 4.8", info)
	}
}

// ---------------------------------------------------------------------------
// The ActivityState-to-string mapping moved to internal/hub, where the wire
// format lives; its tests moved with it (TestActivityMapping_RoundTrips).

func TestScan_TracksOSCTitleChange(t *testing.T) {
	sess := terminal.NewSession(7, 24, 80)
	// OSC 2 ; <title> BEL — Claude/shell sets the window title
	sess.Screen.Write([]byte("\x1b]2;my-pane\x07"))

	app := &AppService{
		host: testHost(map[int]*terminal.Session{7: sess}),
	}

	cleanupActivityTracking(7) // start from a clean tracking state
	app.applyScanResults(app.host.ScanActivity())

	prevEmitMu.Lock()
	got := prevTitle[7]
	prevEmitMu.Unlock()

	if got != "my-pane" {
		t.Fatalf("after scan, prevTitle[7] = %q, want %q", got, "my-pane")
	}

	cleanupActivityTracking(7)
	prevEmitMu.Lock()
	_, exists := prevTitle[7]
	prevEmitMu.Unlock()
	if exists {
		t.Fatal("cleanupActivityTracking should remove the prevTitle entry")
	}
}

// TestScanGuard_StaleActiveHookFallsBackToScreen reproduces the reported bug:
// once a pane's first UserPromptSubmit hook fires, HasHookData() latches true
// forever (only SessionEnd clears it), so the PTY heuristic is skipped for
// the rest of the session's life. If the terminating Stop hook event is lost
// or delayed, the pane — and the pipeline queue waiting on its "done"
// transition — hung on "active" forever. The scan must fall back to the PTY
// screen once output has gone stale.
func TestScanGuard_StaleActiveHookFallsBackToScreen(t *testing.T) {
	sess := terminal.NewSession(9, 10, 80)
	sess.SetHookActivity(terminal.ActivityActive) // e.g. from UserPromptSubmit/PostToolUse
	sess.Screen.Write([]byte(
		"\x1b[32m✓ Task completed successfully\x1b[0m\r\n" +
			"\x1b[1;35m❯\x1b[0m ",
	))
	// Simulate the Stop hook never arriving: PTY output stopped a while ago.
	sess.SetLastOutputAtForTest(time.Now().Add(-2 * time.Second))

	app := &AppService{
		host: testHost(map[int]*terminal.Session{9: sess}),
	}

	cleanupActivityTracking(9)
	// A single tick only arms the debounce candidate (#188): the state has to
	// hold for a window before it confirms. Back-date the candidate instead of
	// sleeping the test, then tick again so it confirms.
	app.applyScanResults(app.host.ScanActivity())
	backdateCandidate(t, app, 9)
	app.applyScanResults(app.host.ScanActivity())

	raw := sess.GetActivity()
	// The fallback is not persisted into sess.Activity (same as the existing
	// done→waitingAnswer cross-check), so assert on the state the host
	// confirmed rather than on GetActivity().
	emitted, _ := confirmedOf(app, 9)
	if emitted != "done" {
		t.Fatalf("after scan with stale active hook + completed-prompt screen, emitted activity = %q (raw hook state %d), want %q — Stop-event-lost fallback not working", emitted, raw, "done")
	}
}

func TestScanGuard_HookActivityNotOverwrittenByScan(t *testing.T) {
	// Setup: a session with hook-driven WaitingPermission state
	// and NO PTY output (LastOutputAt = zero, no screen content).
	// Without the guard, DetectActivity() would return Idle or Done
	// (since there's no PTY output matching the needsInput pattern).
	// With the guard, the session stays at WaitingPermission.
	sess := terminal.NewSession(42, 24, 80)
	sess.SetHookActivity(terminal.ActivityWaitingPermission)

	// Build a minimal AppService with this session
	app := &AppService{
		host: testHost(map[int]*terminal.Session{42: sess}),
	}

	// Run one scan cycle
	app.applyScanResults(app.host.ScanActivity())

	// After scanning, the activity must still be WaitingPermission
	// (the hook guard must have prevented DetectActivity() from resetting it)
	if got := sess.GetActivity(); got != terminal.ActivityWaitingPermission {
		t.Errorf("after scan, activity = %d, want ActivityWaitingPermission — hook guard not working", got)
	}
}

// A tick that emits only because the cost or the title moved must not repeat
// the confirmed state: it would paint over a fresher hook state still inside
// the debounce window.
func TestScanActivityInfo_StateOnlyOnItsOwnChange(t *testing.T) {
	ref := hub.Local("h", 3)
	quiet := scanActivityInfo(ref, hub.ScanResult{ID: 3, Activity: hub.ActivityDone, Title: "neu"}, "$0.10")
	if quiet.Activity != "" || quiet.ActivitySince != 0 {
		t.Errorf("title-only emit carries a state: %+v", quiet)
	}
	if quiet.Title != "neu" || quiet.Cost != "$0.10" {
		t.Errorf("title-only emit lost its payload: %+v", quiet)
	}
	changed := scanActivityInfo(ref, hub.ScanResult{ID: 3, Activity: hub.ActivityDone, Changed: true}, "")
	if changed.Activity != "done" {
		t.Errorf("a real change must carry the state, got %q", changed.Activity)
	}
}
