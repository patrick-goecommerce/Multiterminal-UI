package backend

import "github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"

// The queue and worktree-finish bindings, translated at the window boundary
// like those in app_session_refs.go.

// AddToQueue adds a prompt to a session's queue.
func (a *AppService) AddToQueue(r hub.Ref, prompt string) QueueItem {
	return a.addToQueue(a.local(r), prompt)
}

// GetQueue returns a session's queue.
func (a *AppService) GetQueue(r hub.Ref) []QueueItem { return a.getQueue(a.local(r)) }

// RemoveFromQueue drops one pending item.
func (a *AppService) RemoveFromQueue(r hub.Ref, itemId int) { a.removeFromQueue(a.local(r), itemId) }

// ClearDoneFromQueue drops the finished items.
func (a *AppService) ClearDoneFromQueue(r hub.Ref) { a.clearDoneFromQueue(a.local(r)) }

// ClearQueue drops every item.
func (a *AppService) ClearQueue(r hub.Ref) { a.clearQueue(a.local(r)) }

// StartWorktreeFinish begins merging a pane's worktree back.
func (a *AppService) StartWorktreeFinish(r hub.Ref, worktreePath, branch, target, mode string) {
	a.startWorktreeFinish(a.local(r), worktreePath, branch, target, mode)
}

// CancelWorktreeFinish abandons a finish in progress.
func (a *AppService) CancelWorktreeFinish(r hub.Ref) { a.cancelWorktreeFinish(a.local(r)) }

// FinishWorktree runs the confirmed merge and cleanup.
func (a *AppService) FinishWorktree(r hub.Ref) { a.finishWorktree(a.local(r)) }

// CheckWorktreeFinish re-runs the finish checks.
func (a *AppService) CheckWorktreeFinish(r hub.Ref) { a.checkWorktreeFinish(a.local(r)) }
