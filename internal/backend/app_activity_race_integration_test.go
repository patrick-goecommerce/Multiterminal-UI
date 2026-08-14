package backend

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// TestActivityRace_QueueAdvancesDespiteStrayDetectActivityCall reproduces the
// real production sequence from multiterminal-2026-07-03.log (session 17,
// 2026-07-06 15:33:47-15:34:33, see memory reference_mtui_runtime_log /
// project_activity_hookdata_race_fix): a queued prompt is sent, Claude's hooks
// correctly report active -> done, but something else calls DetectActivity()
// directly in the race window right after Stop fires (in production this was
// app_orchestrator_schedule.go's pollRunningAgents, which polls sessions with
// no HasHookData() guard). Before the fix, that stray call clobbered the
// hook-authoritative "done" back to "active" and nothing ever corrected it —
// the pipeline queue item stayed "sent" forever and the pane badge never left
// "running" ("läuft"). This drives the whole path end-to-end with the real
// HookManager, the real pipeline queue, and the real scan loop — not just the
// isolated DetectActivity() unit — so a regression anywhere along that chain
// (hook wiring, queue advancement, scan tick) would fail it.
func TestActivityRace_QueueAdvancesDespiteStrayDetectActivityCall(t *testing.T) {
	const sessID = 17
	cleanupActivityTracking(sessID) // isolate from any prior test using this ID

	dir := t.TempDir()
	sess := terminal.NewSession(sessID, 24, 80)
	sess.Screen.Write([]byte("$ "))

	app := &AppService{
		sessions: map[int]*terminal.Session{sessID: sess},
		queues:   map[int]*sessionQueue{},
	}

	// Mirrors the real wiring in app_hooks_setup.go: the hook manager's
	// onActivity callback advances the pipeline queue immediately on "done".
	hm := newHookManager(dir, func(mtID int) *terminal.Session {
		if mtID == sessID {
			return sess
		}
		return nil
	}, func(sessionID int, activity string, cost string) {
		if activity == "done" {
			app.processQueue(sessionID)
		}
	})

	// User queues a prompt; the session is idle, so it is sent immediately.
	app.AddToQueue(sessID, "test")
	if got := app.GetQueue(sessID); len(got) != 1 || got[0].Status != "sent" {
		t.Fatalf("queue after AddToQueue = %+v, want 1 item with status 'sent'", got)
	}

	// Claude Code's UserPromptSubmit hook fires for the queued prompt.
	writeTestHookEvent(t, dir, "claude-sess-17", testHookEvent{
		Ts: time.Now().Unix(), Event: "UserPromptSubmit", SessionID: "claude-sess-17", MtID: sessID, Message: "test",
	})
	hm.processDirectory()
	if got := sess.GetActivity(); got != terminal.ActivityActive {
		t.Fatalf("after UserPromptSubmit: activity = %d, want ActivityActive", got)
	}

	// Claude's TUI redraws (spinner) right before finishing — recent PTY output,
	// same as the <1.5s window DetectActivity() treats as "still active".
	// LastOutputAt is exported specifically so tests can drive this timing
	// without a real PTY (see internal/terminal/session.go).
	sess.LastOutputAt = time.Now()

	// Claude finishes: the Stop hook fires.
	writeTestHookEvent(t, dir, "claude-sess-17", testHookEvent{
		Ts: time.Now().Unix(), Event: "Stop", SessionID: "claude-sess-17", MtID: sessID,
	})
	hm.processDirectory()
	if got := sess.GetActivity(); got != terminal.ActivityDone {
		t.Fatalf("after Stop: activity = %d, want ActivityDone", got)
	}

	// RACE: something calls DetectActivity() directly and unguarded in the
	// same window (in production: app_orchestrator_schedule.go's
	// pollRunningAgents). This must defer to the hook state, not clobber it.
	if raced := sess.DetectActivity(); raced != terminal.ActivityDone {
		t.Errorf("stray DetectActivity() call returned %d, want it to defer to hook state (ActivityDone)", raced)
	}

	// The periodic scan tick must also observe "done" — this is what actually
	// drives the pane badge and re-triggers the pipeline queue in production.
	// A single tick only arms the debounce candidate (confirmActivity, task 3 /
	// issue #188) — it takes debounceWindow of a stable state to confirm. Back-
	// date the pending timestamp instead of sleeping the test, then tick again
	// so the candidate confirms.
	app.scanAllSessions()
	prevActivityMu.Lock()
	if since, ok := pendingSince[sessID]; ok {
		pendingSince[sessID] = since.Add(-debounceWindow)
	}
	prevActivityMu.Unlock()
	app.scanAllSessions()

	if got := activityString(sess.GetActivity()); got != "done" {
		t.Fatalf("after scanAllSessions: activity = %q, want %q — pane would be stuck on 'läuft'", got, "done")
	}
	prevActivityMu.Lock()
	gotPrev := prevActivity[sessID]
	prevActivityMu.Unlock()
	if gotPrev != "done" {
		t.Errorf("prevActivity[%d] = %q, want %q", sessID, gotPrev, "done")
	}

	// The queue item itself must have advanced to "done", not be stuck as "sent".
	queue := app.GetQueue(sessID)
	if len(queue) != 1 || queue[0].Status != "done" {
		t.Fatalf("queue after done transition = %+v, want 1 item with status 'done'", queue)
	}
}
