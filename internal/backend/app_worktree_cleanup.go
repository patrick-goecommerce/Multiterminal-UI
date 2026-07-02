// internal/backend/app_worktree_cleanup.go
// Merge + cleanup primitives and the FinishWorktree orchestration. The merge
// MUST run in the MAIN worktree: git merge only ever moves the HEAD of the
// worktree it runs in — anywhere else would leave the target branch untouched
// and then delete the work (spec 5.4).
package backend

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// mergeWorktreeBranch re-verifies ff and merges branch into target inside the
// main worktree. Never uses --force anything; a non-ff state aborts.
func mergeWorktreeBranch(mainRoot, branch, target string) error {
	if got := checkedOutBranch(mainRoot); got != target {
		return fmt.Errorf("Haupt-Worktree steht auf %q, nicht auf %q", got, target)
	}
	if !isAncestor(mainRoot, target, branch) {
		return fmt.Errorf("Ziel-Branch hat sich bewegt — erneut vorbereiten (kein ff-Merge möglich)")
	}
	out, err := gitCmd(mainRoot, "merge", "--ff-only", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ff-merge fehlgeschlagen: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// cleanupWorktree removes the worktree dir (retry with backoff — Windows
// releases handles lazily; --force only as last resort AFTER the merge is
// verified through) and deletes the branch with -d (never -D: -d is the last
// safety net against data loss).
func cleanupWorktree(mainRoot, wtPath, branch string) error {
	var lastErr error
	delays := []time.Duration{0, 200 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second}
	for i, d := range delays {
		time.Sleep(d)
		args := []string{"worktree", "remove", wtPath}
		if i == len(delays)-1 {
			args = []string{"worktree", "remove", "--force", wtPath}
		}
		out, err := gitCmd(mainRoot, args...).CombinedOutput()
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = fmt.Errorf("worktree remove: %s – %w", strings.TrimSpace(string(out)), err)
	}
	if lastErr != nil {
		return lastErr
	}
	_ = gitCmd(mainRoot, "worktree", "prune").Run()
	if out, err := gitCmd(mainRoot, "branch", "-d", branch).CombinedOutput(); err != nil {
		// Deliberately NO -D fallback (spec 5.4/5): report and keep the branch.
		return fmt.Errorf("branch -d verweigert (Branch bleibt stehen, manuell prüfen): %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ReconcileFinishMarkers resumes interrupted cleanups after a restart: for
// every marker of THIS repo whose worktree still exists, the idempotent
// cleanup is re-run — a merge is NEVER repeated (the marker is written only
// after the merge succeeded, spec 4.4). Markers whose worktree directory
// vanished are pruned and dropped. Called once from the frontend on startup.
func (a *AppService) ReconcileFinishMarkers(dir string) {
	root, err := mainRepoRoot(dir)
	if err != nil {
		log.Printf("[finish] reconcile: %v — skipping", err)
		return
	}
	path := finishMarkerPath()
	// Startup-only: hold finishMu across the whole pass so every marker access
	// (load, delete) and the cleanups run serialized, like the live flow (f).
	a.finishMu.Lock()
	defer a.finishMu.Unlock()
	for wtPath, m := range loadFinishMarkers(path) {
		// Worktree dir gone entirely: prune leftover admin files and drop the
		// marker. No merge, no branch delete.
		if _, statErr := os.Stat(wtPath); os.IsNotExist(statErr) {
			log.Printf("[finish] reconcile: worktree %s gone — pruning marker", wtPath)
			_ = gitCmd(root, "worktree", "prune").Run()
			_ = deleteFinishMarker(path, wtPath)
			continue
		}
		// Only touch markers that belong to THIS repo.
		sub, subErr := mainRepoRoot(wtPath)
		if subErr != nil || !strings.EqualFold(sub, root) {
			log.Printf("[finish] reconcile: %s belongs to another repo or is unreadable — skipping", wtPath)
			continue
		}
		// Worktree still present ⇒ merge already happened; resume cleanup only.
		log.Printf("[finish] reconcile: resuming cleanup for %s (phase %s)", wtPath, m.Phase)
		if cleanupErr := cleanupWorktree(root, wtPath, m.Branch); cleanupErr != nil {
			log.Printf("[finish] reconcile failed for %s: %v", wtPath, cleanupErr)
			continue
		}
		_ = deleteFinishMarker(path, wtPath)
		log.Printf("[finish] reconcile: cleaned up %s", wtPath)
	}
}

// FinishWorktree executes merge + cleanup after the user confirmed the
// overlay. Runs in a goroutine (the remove retry may take seconds — a Wails
// binding must not block, spec 5.4) serialized by finishMu. Accepts the
// "ready" phase, or a "blocked" retry whose merge already went through (a
// marker exists): the goroutine then skips the merge via the count==0 path.
func (a *AppService) FinishWorktree(sessionId int) {
	a.mu.Lock()
	st := a.finishStates[sessionId]
	if st == nil {
		a.mu.Unlock()
		return
	}
	phase, wtPath := st.Phase, st.WorktreePath
	a.mu.Unlock()

	// A "blocked" retry is only valid once the merge already went through
	// (a marker exists) — a pre-merge block must go back through Start. The
	// marker read runs under finishMu (never a.mu), keeping every marker
	// access serialized (f); it may briefly wait on an in-flight finish,
	// which is the intended global serialization.
	if phase != "ready" {
		if phase != "blocked" {
			return
		}
		a.finishMu.Lock()
		_, hasMarker := loadFinishMarkers(finishMarkerPath())[wtPath]
		a.finishMu.Unlock()
		if !hasMarker {
			return
		}
	}

	a.mu.Lock()
	st = a.finishStates[sessionId]
	if st == nil { // cancelled while we were reading the marker
		a.mu.Unlock()
		return
	}
	st.Phase = "merging"
	cp := *st
	sess := a.sessions[sessionId]
	a.mu.Unlock()

	go func() {
		a.finishMu.Lock()
		defer a.finishMu.Unlock()

		root, err := mainRepoRoot(cp.WorktreePath)
		if err != nil {
			a.setFinishBlocked(sessionId, err.Error())
			return
		}
		// Re-check whether the branch is already contained (cleanup_only path
		// or crash/retry recovery); otherwise merge.
		count, err := revCount(root, cp.TargetBranch, cp.Branch)
		if err != nil {
			a.setFinishBlocked(sessionId, err.Error())
			return
		}
		if count > 0 {
			if err := mergeWorktreeBranch(root, cp.Branch, cp.TargetBranch); err != nil {
				a.setFinishBlocked(sessionId, err.Error())
				return
			}
		}
		_ = saveFinishMarker(finishMarkerPath(), cp.WorktreePath, finishMarker{
			Phase: "merged", Branch: cp.Branch, TargetBranch: cp.TargetBranch,
		})
		a.mu.Lock()
		if cur := a.finishStates[sessionId]; cur != nil {
			cur.Phase = "cleanup"
		}
		a.mu.Unlock()

		// Kill the whole tree BEFORE Close (spec 5.2), then close synchronously.
		if sess != nil {
			killProcessTree(sess.Pid())
			sess.Close()
		}
		if err := cleanupWorktree(root, cp.WorktreePath, cp.Branch); err != nil {
			a.setFinishBlocked(sessionId, "Merge ist durch, Cleanup fehlgeschlagen: "+err.Error()+" — erneut versuchen")
			return
		}
		_ = deleteFinishMarker(finishMarkerPath(), cp.WorktreePath)
		a.mu.Lock()
		delete(a.finishStates, sessionId)
		delete(a.sessions, sessionId)
		delete(a.queues, sessionId)
		a.mu.Unlock()
		if a.app != nil {
			a.app.Event.Emit("worktree:finish-done", WorktreeFinishDoneEvent{
				SessionID: sessionId, MainRoot: root,
				TargetBranch: cp.TargetBranch, Mode: cp.Mode,
			})
		}
		log.Printf("[finish] session %d: merged %s into %s and cleaned up", sessionId, cp.Branch, cp.TargetBranch)
	}()
}
