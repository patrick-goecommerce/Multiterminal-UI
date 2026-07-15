// Package backend provides the orchestrator scheduler loop.
// This file contains the main tick loop, agent spawning, and completion handling.
package backend

import (
	"fmt"
	"log"
	"time"
)

// runOrchestrator is the main scheduler goroutine. It ticks every 5 seconds,
// checks for ready cards, spawns agents in worktrees, and detects completions.
func (a *AppService) runOrchestrator(orch *orchestratorState) {
	defer func() {
		orch.mu.Lock()
		orch.active = false
		orch.mu.Unlock()

		orchMu.Lock()
		delete(orchestrators, orch.dir)
		orchMu.Unlock()

		log.Printf("[orchestrator] scheduler exited for dir=%s", orch.dir)
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-orch.cancel:
			return
		case <-ticker.C:
			a.orchestratorTick(orch)
		}
	}
}

// orchestratorTick runs one cycle of the scheduler:
// 1. Check "ready" column for spawnable cards
// 2. Spawn agents for cards with fulfilled dependencies
// 3. Check "in_progress" cards for completion
func (a *AppService) orchestratorTick(orch *orchestratorState) {
	state, err := loadKanbanState(orch.dir)
	if err != nil {
		log.Printf("[orchestrator] tick load error: %v", err)
		return
	}

	maxAgents := getMaxParallel(a.cfg.Orchestrator.MaxParallelAgents)
	changed := false

	// Phase 1: Check in_progress sessions for completion (fallback polling).
	// Primary detection is via notifyOrchestratorDone from scan loop,
	// but we poll here as a safety net for edge cases.
	changed = a.pollRunningAgents(orch, &state) || changed

	// Phase 2: Spawn agents for ready cards
	orch.mu.Lock()
	runningCount := len(orch.running)
	orch.mu.Unlock()

	if runningCount < maxAgents {
		readyCards := state.Columns[ColReady]
		for _, card := range readyCards {
			if runningCount >= maxAgents {
				break
			}
			if !cardDependenciesMet(&state, card) {
				continue
			}
			if a.spawnAgent(orch, &state, card) {
				runningCount++
				changed = true
			}
		}
	}

	if changed {
		if err := saveKanbanState(orch.dir, state); err != nil {
			log.Printf("[orchestrator] tick save error: %v", err)
		}
		a.emitOrchestratorEvent(orch.dir, "tick")
	}

	// Check if all work is done
	if a.isOrchestrationComplete(orch, &state) {
		log.Printf("[orchestrator] all cards processed for dir=%s", orch.dir)
		a.emitOrchestratorEvent(orch.dir, "done")
		close(orch.cancel) // triggers loop exit
	}
}

// spawnAgent creates a Git worktree and Claude session for a card.
// Returns true if the agent was successfully spawned.
func (a *AppService) spawnAgent(orch *orchestratorState, state *KanbanState, card KanbanCard) bool {
	// Create a named worktree for this card
	wtName := fmt.Sprintf("card-%s", card.ID)
	wt, err := a.CreateNamedWorktree(orch.dir, wtName, "")
	if err != nil {
		log.Printf("[orchestrator] worktree creation failed for card %s: %v", card.ID, err)
		return false
	}

	// Build Claude command
	argv := []string{a.resolvedClaudePath}
	if argv[0] == "" {
		argv[0] = "claude"
	}

	// Spawn PTY session in the worktree directory
	sessionID := a.CreateSession(argv, wt.Path, 24, 80, "claude")
	if sessionID < 0 {
		log.Printf("[orchestrator] session creation failed for card %s", card.ID)
		return false
	}

	// Update card metadata
	updateCardInState(state, card.ID, func(c *KanbanCard) {
		c.AgentSessionID = sessionID
		c.WorktreePath = wt.Path
		c.WorktreeBranch = wt.Branch
	})

	// Link session to parent issue for progress tracking
	if card.ParentIssue > 0 {
		a.LinkSessionIssue(sessionID, card.ParentIssue, card.Title, wt.Branch, orch.dir)
	}

	// Move card to in_progress
	a.moveCardToColumn(state, card.ID, ColInProgress)

	// Track in orchestrator state
	orch.mu.Lock()
	orch.running[card.ID] = sessionID
	orch.mu.Unlock()

	// Send prompt after a brief delay for Claude startup
	if card.Prompt != "" {
		go func() {
			time.Sleep(2 * time.Second)
			a.AddToQueue(sessionID, card.Prompt)
		}()
	}

	log.Printf("[orchestrator] spawned agent: card=%s session=%d worktree=%s", card.ID, sessionID, wt.Path)
	a.emitOrchestratorEvent(orch.dir, "agent_started")
	return true
}

// pollRunningAgents checks in_progress sessions for completion as a safety net.
// Returns true if any state changes were made.
func (a *AppService) pollRunningAgents(orch *orchestratorState, state *KanbanState) bool {
	orch.mu.Lock()
	// Copy map to avoid holding lock during session access
	running := make(map[string]int, len(orch.running))
	for k, v := range orch.running {
		running[k] = v
	}
	orch.mu.Unlock()

	changed := false
	for cardID, sessionID := range running {
		a.mu.Lock()
		sess := a.sessions[sessionID]
		a.mu.Unlock()

		if sess == nil {
			// Session closed externally — treat as done
			orch.mu.Lock()
			delete(orch.running, cardID)
			orch.done[cardID] = true
			orch.mu.Unlock()
			a.moveCardToColumn(state, cardID, ColDone)
			changed = true
			log.Printf("[orchestrator] session %d gone, card %s moved to done", sessionID, cardID)
			continue
		}

		activity := activityString(sess.DetectActivity())
		if activity == "done" {
			orch.mu.Lock()
			// Only transition if not already handled by notifyOrchestratorDone
			if _, stillRunning := orch.running[cardID]; stillRunning {
				delete(orch.running, cardID)
				orch.review[cardID] = sessionID
				orch.mu.Unlock()
				a.moveCardToColumn(state, cardID, ColAutoReview)
				changed = true
				log.Printf("[orchestrator] poll detected done: card=%s session=%d → auto_review", cardID, sessionID)
			} else {
				orch.mu.Unlock()
			}
		}
	}
	return changed
}

// isOrchestrationComplete checks if all work is finished:
// no cards in ready, in_progress, or auto_review, and at least one card done.
func (a *AppService) isOrchestrationComplete(orch *orchestratorState, state *KanbanState) bool {
	orch.mu.Lock()
	runningCount := len(orch.running)
	reviewCount := len(orch.review)
	doneCount := len(orch.done)
	orch.mu.Unlock()

	readyCount := len(state.Columns[ColReady])
	return readyCount == 0 && runningCount == 0 && reviewCount == 0 && doneCount > 0
}

// updateCardInState finds a card by ID across all columns and applies fn to it.
func updateCardInState(state *KanbanState, cardID string, fn func(*KanbanCard)) bool {
	for col := range state.Columns {
		for i := range state.Columns[col] {
			if state.Columns[col][i].ID == cardID {
				fn(&state.Columns[col][i])
				return true
			}
		}
	}
	return false
}

// getMaxParallel returns the configured max parallel agents, defaulting to 3.
func getMaxParallel(maxCfg int) int {
	if maxCfg <= 0 {
		return 3
	}
	return maxCfg
}

// cardDependenciesMet checks if all dependency issues for a card are done.
func cardDependenciesMet(state *KanbanState, card KanbanCard) bool {
	if len(card.Dependencies) == 0 {
		return true
	}
	doneIssues := make(map[int]bool)
	for _, c := range state.Columns[ColDone] {
		if c.IssueNumber > 0 {
			doneIssues[c.IssueNumber] = true
		}
	}
	for _, dep := range card.Dependencies {
		if !doneIssues[dep] {
			return false
		}
	}
	return true
}
