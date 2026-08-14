package backend

import (
	"testing"
	"time"

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

	dir := t.TempDir()
	sess := terminal.NewSession(sessID, 24, 80)
	sess.Screen.Write([]byte("$ "))

	app := &AppService{
		sessions: map[int]*terminal.Session{sessID: sess},
		queues:   map[int]*sessionQueue{},
	}
	var reports []issueProgressEvent
	app.issueProgressHook = func(_ int, ev issueProgressEvent) {
		reports = append(reports, ev)
	}

	hm := newHookManager(dir, func(mtID int) *terminal.Session {
		if mtID == sessID {
			return sess
		}
		return nil
	}, app.onHookActivity)

	// Claude starts working.
	writeTestHookEvent(t, dir, "claude-sess-71", testHookEvent{
		Ts: time.Now().Unix(), Event: "UserPromptSubmit", SessionID: "claude-sess-71", MtID: sessID, Message: "los",
	})
	hm.processDirectory()
	confirmViaScan(t, app, sessID)

	// Claude finishes: exactly one Stop event, one real completion.
	sess.LastOutputAt = time.Now()
	writeTestHookEvent(t, dir, "claude-sess-71", testHookEvent{
		Ts: time.Now().Unix(), Event: "Stop", SessionID: "claude-sess-71", MtID: sessID,
	})
	hm.processDirectory()
	confirmViaScan(t, app, sessID)

	// Further ticks on the settled state must add nothing.
	app.scanAllSessions()
	app.scanAllSessions()

	if len(reports) != 1 || reports[0] != progressDone {
		t.Fatalf("reportIssueProgress calls = %v, want exactly one %q — a single completion must report once", reports, progressDone)
	}
}

// The hook callback exists for latency: it repaints the badge a debounce window
// before the scan loop confirms. That is all it may do — every side effect
// belongs to the one confirmed change in scanAllSessions.
func TestOnHookActivity_TriggersNoSideEffects(t *testing.T) {
	const sessID = 72
	cleanupActivityTracking(sessID)

	sess := terminal.NewSession(sessID, 24, 80)
	app := &AppService{
		sessions: map[int]*terminal.Session{sessID: sess},
		queues:   map[int]*sessionQueue{},
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
	app.scanAllSessions()
	prevActivityMu.Lock()
	since, ok := pendingSince[sessID]
	if ok {
		pendingSince[sessID] = since.Add(-debounceWindow)
	}
	prevActivityMu.Unlock()
	if !ok {
		t.Fatalf("no debounce candidate armed for session %d — the scan never saw the new state", sessID)
	}
	app.scanAllSessions()
}
