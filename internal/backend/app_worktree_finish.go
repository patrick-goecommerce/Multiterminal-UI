// internal/backend/app_worktree_finish.go
// Backend finish state machine (spec 4.3). Lives in the backend so it
// survives tab detach, webview reloads and multi-window moves; the frontend
// only renders worktree:finish-* events.
package backend

import (
	"fmt"
	"log"
	"time"
)

const prepPromptTemplate = "Committe alle offenen Änderungen in nachvollziehbaren Commits. " +
	"Committe keine Secrets, .env-Dateien oder Build-Artefakte — ergänze für solche Dateien " +
	".gitignore-Einträge oder lass sie untracked und erwähne sie. " +
	"Rebase dann %s auf den lokalen %s. Bei Rebase-Konflikten: nicht selbst lösen, " +
	"`git rebase --abort` ausführen und die Konfliktdateien nennen. " +
	"Merge nicht selbst, pushe nicht, erstelle keinen PR."

const finishPrepTimeout = 10 * time.Minute

// finishState tracks one session's finish flow. Guarded by a.mu.
type finishState struct {
	Phase        string // "" | "preparing" | "ready" | "blocked" | "merging" | "merged" | "cleanup"
	TargetBranch string
	WorktreePath string
	Branch       string
	BlockReason  string
	Mode         string // "claude" | "shell"
	PrepItemID   int
	startedAt    time.Time
}

func (a *AppService) getFinishState(sessionId int) *finishState {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.finishStates[sessionId]
	if st == nil {
		return nil
	}
	cp := *st
	return &cp
}

func (a *AppService) emitFinishBlocked(sessionId int, phase, reason string) {
	a.emitFinishBlockedEvent(sessionId, phase, reason, false)
}

func (a *AppService) emitFinishBlockedEvent(sessionId int, phase, reason string, cleanupFailed bool) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("worktree:finish-blocked", WorktreeFinishBlockedEvent{
		SessionID: sessionId, Phase: phase, Reason: reason, CleanupFailed: cleanupFailed,
	})
}

func (a *AppService) setFinishBlocked(sessionId int, reason string) {
	a.mu.Lock()
	st := a.finishStates[sessionId]
	if st != nil {
		st.Phase = "blocked"
		st.BlockReason = reason
	}
	a.mu.Unlock()
	if st != nil {
		a.emitFinishBlocked(sessionId, "blocked", reason)
	}
}

// setFinishCleanupBlocked parks a post-merge cleanup failure: the merge is
// through and the marker persists, so the phase stays "cleanup" (NOT "blocked")
// and the emitted event carries CleanupFailed so the overlay offers "Cleanup
// erneut versuchen" — that retry routes through FinishWorktree, which accepts
// "cleanup"+marker and resumes straight into the cleanup (no re-prep, no
// re-merge). Routing it through Start instead would re-prep the already-closed
// session and strand the flow in "preparing" (spec 4.3 transition table).
func (a *AppService) setFinishCleanupBlocked(sessionId int, reason string) {
	a.mu.Lock()
	st := a.finishStates[sessionId]
	if st != nil {
		st.Phase = "cleanup"
		st.BlockReason = reason
	}
	a.mu.Unlock()
	if st != nil {
		a.emitFinishBlockedEvent(sessionId, "cleanup", reason, true)
	}
}

// StartWorktreeFinish begins (or retries) the finish flow for a session.
// No-op while a phase other than "blocked" is active (double-click guard).
func (a *AppService) StartWorktreeFinish(sessionId int, worktreePath, branch, target, mode string) {
	a.mu.Lock()
	prevPrepID := 0
	if st := a.finishStates[sessionId]; st != nil {
		if st.Phase != "blocked" {
			a.mu.Unlock()
			return
		}
		prevPrepID = st.PrepItemID
		// Retry from "blocked": remove the stale entry now so no state exists
		// during the upcoming AddToQueue call below. Task 8's finish lock in
		// AddToQueue rejects every item while a.finishStates[sessionId] != nil,
		// which would otherwise reject the fresh prep item (ID 0) and strand
		// the retry in "preparing" forever.
		delete(a.finishStates, sessionId)
	}
	q := a.queues[sessionId]
	pending := 0
	if q != nil {
		for _, it := range q.items {
			// Exclude the previous attempt's own prep item: it belongs to the
			// flow being retried, not to unrelated user prompts, and should
			// never block its own retry (regressed even if still "sent" —
			// e.g. Claude hasn't flipped it to "done" yet).
			if prevPrepID != 0 && it.ID == prevPrepID {
				continue
			}
			if it.Status == "pending" || it.Status == "sent" {
				pending++
			}
		}
	}
	if pending > 0 {
		reason := fmt.Sprintf("Queue nicht leer (%d Prompts) — erst abarbeiten oder verwerfen", pending)
		a.finishStates[sessionId] = &finishState{
			Phase: "blocked", TargetBranch: target, WorktreePath: worktreePath,
			Branch: branch, Mode: mode,
			BlockReason: reason,
		}
		a.mu.Unlock()
		a.emitFinishBlocked(sessionId, "blocked", reason)
		return
	}
	a.mu.Unlock()

	if mode == "shell" {
		// Shell panes skip the prompt: frontend runs the staging dialog first
		// (task 17), then calls CheckWorktreeFinish directly.
		a.mu.Lock()
		a.finishStates[sessionId] = &finishState{
			Phase: "preparing", TargetBranch: target, WorktreePath: worktreePath,
			Branch: branch, Mode: mode, startedAt: time.Now(),
		}
		a.mu.Unlock()
		return
	}

	prompt := fmt.Sprintf(prepPromptTemplate, branch, target)
	item := a.AddToQueue(sessionId, prompt) // enqueue BEFORE state exists (queue lock, task 8)
	a.mu.Lock()
	a.finishStates[sessionId] = &finishState{
		Phase: "preparing", TargetBranch: target, WorktreePath: worktreePath,
		Branch: branch, Mode: mode, PrepItemID: item.ID, startedAt: time.Now(),
	}
	a.mu.Unlock()
	log.Printf("[finish] session %d: preparing (prep item %d)", sessionId, item.ID)

	time.AfterFunc(finishPrepTimeout, func() {
		if st := a.getFinishState(sessionId); st != nil && st.Phase == "preparing" && st.PrepItemID == item.ID {
			a.emitFinishBlocked(sessionId, "preparing",
				"Vorbereitung läuft seit 10 Minuten — prüfen oder abbrechen")
		}
	})
}

// CancelWorktreeFinish aborts the flow (allowed in preparing/ready/blocked).
func (a *AppService) CancelWorktreeFinish(sessionId int) {
	a.mu.Lock()
	st := a.finishStates[sessionId]
	if st == nil || st.Phase == "merging" || st.Phase == "merged" || st.Phase == "cleanup" {
		a.mu.Unlock()
		return
	}
	prepID := st.PrepItemID
	delete(a.finishStates, sessionId)
	a.mu.Unlock()
	if prepID != 0 {
		a.forceRemoveQueueItem(sessionId, prepID)
	}
	a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen")
}

// forceRemoveQueueItem drops the finish flow's own prep-item tracking entry
// regardless of its Status. Unlike RemoveFromQueue (which refuses to remove
// "sent" items because they are generically in flight for the caller), a
// cancelled finish flow owns this entry outright: the prompt text may
// already sit in the pane, but no purpose is served by leaving a stale
// tracking row behind once the flow itself is torn down.
func (a *AppService) forceRemoveQueueItem(sessionId, itemId int) {
	a.mu.Lock()
	q := a.queues[sessionId]
	if q != nil {
		for i, it := range q.items {
			if it.ID == itemId {
				q.items = append(q.items[:i], q.items[i+1:]...)
				break
			}
		}
	}
	a.mu.Unlock()
	a.emitQueueUpdate(sessionId)
}

// CheckWorktreeFinish runs the verification gate and moves preparing→ready/blocked.
// Called by onQueueItemDone (claude) and by the frontend after the shell
// staging dialog committed+rebased.
func (a *AppService) CheckWorktreeFinish(sessionId int) {
	st := a.getFinishState(sessionId)
	if st == nil {
		return
	}
	status := a.GetWorktreeFinishStatus(st.WorktreePath, st.Branch, st.TargetBranch)
	if status.State == "blocked" {
		a.setFinishBlocked(sessionId, status.Reason)
		return
	}
	a.mu.Lock()
	cur := a.finishStates[sessionId]
	stillActive := cur != nil
	if stillActive {
		cur.Phase = "ready"
		cur.BlockReason = ""
	}
	a.mu.Unlock()
	// Only emit if the flow wasn't cancelled while the async git check ran —
	// otherwise a stale "ready" event would resurrect a torn-down flow.
	if stillActive && a.app != nil {
		a.app.Event.Emit("worktree:finish-ready", WorktreeFinishReadyEvent{
			SessionID: sessionId, TargetBranch: st.TargetBranch,
			Commits: status.Commits, Stat: status.Stat, Untracked: status.Untracked,
			CleanupOnly: status.State == "cleanup_only",
		})
	}
}

// onQueueItemDone is invoked by processQueue whenever an item transitions to
// "done". Only the exact prep item of an active preparing flow triggers the
// check — generic done transitions (earlier items, other turns) do nothing
// (spec 5.1/2, red-team L-K4/U-H1).
func (a *AppService) onQueueItemDone(sessionId, itemID int) {
	st := a.getFinishState(sessionId)
	if st == nil || st.Phase != "preparing" || st.PrepItemID != itemID {
		return
	}
	go a.CheckWorktreeFinish(sessionId)
}

// notifyFinishOnActivity surfaces "Claude has a question" while preparing.
// The phase stays preparing — the prep item correlation keeps running.
func (a *AppService) notifyFinishOnActivity(sessionId int, actStr string) {
	if actStr != "waitingPermission" && actStr != "waitingAnswer" {
		return
	}
	if st := a.getFinishState(sessionId); st != nil && st.Phase == "preparing" {
		a.emitFinishBlocked(sessionId, "preparing", "Claude hat eine Rückfrage — bitte im Pane antworten")
	}
}
