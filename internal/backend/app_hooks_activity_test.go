package backend

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// The spec requires that a real completion reports issue progress exactly once
// (regression to #188). Before this, the hook callback fired processQueue and
// onActivityChangeForIssue the instant the Stop event landed, and the scan loop
// fired both again ~2 s later on the confirmed change. reportIssueProgress has
// no deduplication, so with auto_comment_on_done that meant two GitHub comments
// per completion and, with auto_close_issue, two close attempts.
//
// This drives the whole production path: the real HookManager wired to the real
// production callback (a.onHookActivity), then the real scan loop confirming
// the state.
func TestHookDrivenCompletion_ReportsIssueProgressExactlyOnce(t *testing.T) {
	const sessID = 71
	cleanupActivityTracking(sessID) // isolate from any prior test using this ID

	sess := terminal.NewSession(sessID, 24, 80)
	sess.Screen.Write([]byte("$ "))

	app := &AppService{
		host: testHost(map[int]*terminal.Session{sessID: sess}),
	}
	var reports []issueProgressEvent
	app.issueProgressHook = func(_ int, ev issueProgressEvent) {
		reports = append(reports, ev)
	}

	hook := func(event string, activity hub.Activity) {
		sess.SetHookActivity(terminalActivityForTest(activity))
		app.onHookReport(hub.HookReport{Session: sessID, Event: event, Activity: activity})
	}

	// Claude starts working.
	hook("UserPromptSubmit", hub.ActivityActive)
	confirmViaScan(t, app, sessID)

	// Claude finishes: exactly one Stop event, one real completion.
	sess.LastOutputAt = time.Now()
	hook("Stop", hub.ActivityDone)
	confirmViaScan(t, app, sessID)

	// Further ticks on the settled state must add nothing.
	app.applyScanResults(app.host.ScanActivity())
	app.applyScanResults(app.host.ScanActivity())

	if len(reports) != 1 || reports[0] != progressDone {
		t.Fatalf("reportIssueProgress calls = %v, want exactly one %q — a single completion must report once", reports, progressDone)
	}
}

// The hook callback exists for latency: it repaints the badge a debounce window
// before the scan loop confirms. That is all it may do — every side effect
// belongs to the one confirmed change in applyScanResults.
func TestOnHookActivity_TriggersNoSideEffects(t *testing.T) {
	const sessID = 72
	cleanupActivityTracking(sessID)

	sess := terminal.NewSession(sessID, 24, 80)
	app := &AppService{
		host: testHost(map[int]*terminal.Session{sessID: sess}),
	}
	reports := 0
	app.issueProgressHook = func(int, issueProgressEvent) { reports++ }

	app.AddToQueue(sessID, "erster")
	app.AddToQueue(sessID, "zweiter")
	if got := app.GetQueue(sessID); len(got) != 2 || got[0].Status != "sent" || got[1].Status != "pending" {
		t.Fatalf("queue setup = %+v, want item 1 'sent' and item 2 'pending'", got)
	}

	app.onHookActivity(sessID, "done", "$1.00")

	if reports != 0 {
		t.Errorf("onHookActivity reported issue progress %d times, want 0 — that belongs to the confirmed change", reports)
	}
	if got := app.GetQueue(sessID); got[0].Status != "sent" || got[1].Status != "pending" {
		t.Errorf("queue after onHookActivity = %+v, want unchanged — the queue advances on the confirmed change only", got)
	}
}

// confirmViaScan runs the scan loop until the currently observed state is
// confirmed: one tick arms the debounce candidate, then the candidate's start
// is back-dated past the window (instead of sleeping the test) and a second
// tick confirms it.
func confirmViaScan(t *testing.T, app *AppService, sessID int) {
	t.Helper()
	app.applyScanResults(app.host.ScanActivity())
	backdateCandidate(t, app, sessID)
	app.applyScanResults(app.host.ScanActivity())
}
