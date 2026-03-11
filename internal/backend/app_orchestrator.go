// Package backend provides multi-agent orchestration over Kanban plans.
// The scheduler polls the "ready" column every 5 seconds, spawns agents
// in isolated Git worktrees, and transitions cards through the Kanban columns.
package backend

import (
	"fmt"
	"log"
	"sync"
)

// OrchestrationStatus is the JSON-serialisable status returned to the frontend.
type OrchestrationStatus struct {
	Active         bool `json:"active"`
	RunningAgents  int  `json:"running_agents"`
	MaxAgents      int  `json:"max_agents"`
	PendingTickets int  `json:"pending_tickets"`
	ReviewTickets  int  `json:"review_tickets"`
	DoneTickets    int  `json:"done_tickets"`
}

// orchestratorState tracks a running orchestration for a single directory.
type orchestratorState struct {
	mu      sync.Mutex
	planID  string
	dir     string
	running map[string]int  // cardID → sessionID (in_progress agents)
	review  map[string]int  // cardID → review sessionID (auto_review)
	done    map[string]bool // cardID → true
	cancel  chan struct{}
	active  bool
}

// Global registry of orchestrators, keyed by directory.
var (
	orchMu        sync.Mutex
	orchestrators = make(map[string]*orchestratorState) // dir → state
)

// StartOrchestration starts the Kanban scheduler for a project directory.
// It loads the kanban state, validates that cards exist in "ready", and
// starts the background scheduler goroutine.
func (a *AppService) StartOrchestration(dir string) error {
	orchMu.Lock()
	if orch, ok := orchestrators[dir]; ok && orch.active {
		orchMu.Unlock()
		return fmt.Errorf("orchestration already running for %s", dir)
	}
	orchMu.Unlock()

	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	// Find the running plan (if any) for this dir
	planID := ""
	for _, p := range state.Plans {
		if p.Dir == dir && (p.Status == "running" || p.Status == "approved") {
			planID = p.ID
			break
		}
	}

	orch := &orchestratorState{
		planID:  planID,
		dir:     dir,
		running: make(map[string]int),
		review:  make(map[string]int),
		done:    make(map[string]bool),
		cancel:  make(chan struct{}),
		active:  true,
	}

	orchMu.Lock()
	orchestrators[dir] = orch
	orchMu.Unlock()

	go a.runOrchestrator(orch)

	log.Printf("[orchestrator] started for dir=%s planID=%s", dir, planID)
	a.emitOrchestratorEvent(dir, "started")
	return nil
}

// StopOrchestration stops the scheduler for a directory.
// Running sessions are NOT closed — they keep running independently.
func (a *AppService) StopOrchestration(dir string) error {
	orchMu.Lock()
	orch, ok := orchestrators[dir]
	orchMu.Unlock()

	if !ok || !orch.active {
		return fmt.Errorf("no active orchestration for %s", dir)
	}

	close(orch.cancel)
	orch.mu.Lock()
	orch.active = false
	orch.mu.Unlock()

	log.Printf("[orchestrator] stopped for dir=%s", dir)
	a.emitOrchestratorEvent(dir, "stopped")
	return nil
}

// GetOrchestrationStatus returns the current status of orchestration for a directory.
func (a *AppService) GetOrchestrationStatus(dir string) OrchestrationStatus {
	orchMu.Lock()
	orch, ok := orchestrators[dir]
	orchMu.Unlock()

	if !ok {
		return OrchestrationStatus{MaxAgents: getMaxParallel(a.cfg.Orchestrator.MaxParallelAgents)}
	}

	orch.mu.Lock()
	runningCount := len(orch.running)
	reviewCount := len(orch.review)
	doneCount := len(orch.done)
	active := orch.active
	orch.mu.Unlock()

	// Count pending (ready column) cards
	state, _ := loadKanbanState(dir)
	pendingCount := len(state.Columns[ColReady])

	return OrchestrationStatus{
		Active:         active,
		RunningAgents:  runningCount,
		MaxAgents:      getMaxParallel(a.cfg.Orchestrator.MaxParallelAgents),
		PendingTickets: pendingCount,
		ReviewTickets:  reviewCount,
		DoneTickets:    doneCount,
	}
}

// notifyOrchestratorDone is called from the scan loop when a session's
// activity transitions to "done". It looks up the session across all
// orchestrators and triggers the review transition.
func (a *AppService) notifyOrchestratorDone(sessionID int) {
	orchMu.Lock()
	orchList := make([]*orchestratorState, 0, len(orchestrators))
	for _, o := range orchestrators {
		orchList = append(orchList, o)
	}
	orchMu.Unlock()

	for _, orch := range orchList {
		orch.mu.Lock()
		for cardID, sid := range orch.running {
			if sid == sessionID {
				// Found: move card from in_progress to auto_review
				delete(orch.running, cardID)
				orch.review[cardID] = sessionID
				orch.mu.Unlock()

				a.transitionToReview(orch, cardID)
				return
			}
		}
		orch.mu.Unlock()
	}
}

// transitionToReview moves a card to auto_review and persists the change.
func (a *AppService) transitionToReview(orch *orchestratorState, cardID string) {
	state, err := loadKanbanState(orch.dir)
	if err != nil {
		log.Printf("[orchestrator] load error during review transition: %v", err)
		return
	}

	a.moveCardToColumn(&state, cardID, ColAutoReview)
	if err := saveKanbanState(orch.dir, state); err != nil {
		log.Printf("[orchestrator] save error during review transition: %v", err)
		return
	}

	log.Printf("[orchestrator] card %s moved to auto_review", cardID)
	a.emitOrchestratorEvent(orch.dir, "agent_done")
}

// emitOrchestratorEvent sends an orchestrator update event to the frontend.
func (a *AppService) emitOrchestratorEvent(dir string, eventType string) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("orchestrator:update", map[string]string{
		"dir":  dir,
		"type": eventType,
	})
}

// lookupOrchestrator returns the orchestrator for a directory, if any.
func lookupOrchestrator(dir string) *orchestratorState {
	orchMu.Lock()
	defer orchMu.Unlock()
	return orchestrators[dir]
}
