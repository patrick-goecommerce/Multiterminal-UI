// Package backend provides multi-agent orchestration over Kanban plans.
package backend

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// OrchestratorConfig controls multi-agent execution behavior.
type OrchestratorConfig struct {
	MaxParallelAgents int  `json:"max_parallel_agents" yaml:"max_parallel_agents"`
	AutoStartNext     bool `json:"auto_start_next" yaml:"auto_start_next"`
}

// orchestratorState tracks a running plan execution.
type orchestratorState struct {
	mu       sync.Mutex
	planID   string
	dir      string
	running  map[string]int // cardID → sessionID
	done     map[string]bool
	cancel   chan struct{}
	active   bool
}

var (
	orchMu      sync.Mutex
	orchestrators = make(map[string]*orchestratorState) // planID → state
)

// ExecutePlan starts the orchestrator for an approved plan.
func (a *AppService) ExecutePlan(dir string, planID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	var plan *Plan
	for i := range state.Plans {
		if state.Plans[i].ID == planID {
			plan = &state.Plans[i]
			break
		}
	}
	if plan == nil {
		return fmt.Errorf("plan %s not found", planID)
	}
	if plan.Status != "approved" {
		return fmt.Errorf("plan must be approved before execution (current: %s)", plan.Status)
	}

	// Mark plan as running
	plan.Status = "running"
	if err := saveKanbanState(dir, state); err != nil {
		return fmt.Errorf("save kanban: %w", err)
	}

	orch := &orchestratorState{
		planID:  planID,
		dir:     dir,
		running: make(map[string]int),
		done:    make(map[string]bool),
		cancel:  make(chan struct{}),
		active:  true,
	}

	orchMu.Lock()
	orchestrators[planID] = orch
	orchMu.Unlock()

	go a.runOrchestrator(orch)

	log.Printf("[orchestrator] started plan %s with %d steps", planID, len(plan.Steps))
	return nil
}

// StopPlan stops a running orchestrator.
func (a *AppService) StopPlan(dir string, planID string) error {
	orchMu.Lock()
	orch, ok := orchestrators[planID]
	orchMu.Unlock()

	if !ok || !orch.active {
		return fmt.Errorf("plan %s is not running", planID)
	}

	close(orch.cancel)
	orch.active = false

	// Update plan status
	state, err := loadKanbanState(dir)
	if err == nil {
		for i := range state.Plans {
			if state.Plans[i].ID == planID {
				state.Plans[i].Status = "approved" // Back to approved, not cancelled
				break
			}
		}
		saveKanbanState(dir, state)
	}

	log.Printf("[orchestrator] stopped plan %s", planID)
	return nil
}

// runOrchestrator executes plan steps sequentially, starting parallel steps together.
func (a *AppService) runOrchestrator(orch *orchestratorState) {
	defer func() {
		orch.active = false
		orchMu.Lock()
		delete(orchestrators, orch.planID)
		orchMu.Unlock()
	}()

	for {
		select {
		case <-orch.cancel:
			return
		default:
		}

		state, err := loadKanbanState(orch.dir)
		if err != nil {
			log.Printf("[orchestrator] load error: %v", err)
			return
		}

		var plan *Plan
		for i := range state.Plans {
			if state.Plans[i].ID == orch.planID {
				plan = &state.Plans[i]
				break
			}
		}
		if plan == nil {
			return
		}

		// Find next pending steps
		allDone := true
		startedAny := false
		for i, step := range plan.Steps {
			if step.Status == "pending" && !orch.isRunning(step.CardID) {
				allDone = false
				// Check dependencies: all previous non-parallel steps must be done
				canStart := true
				for j := 0; j < i; j++ {
					prev := plan.Steps[j]
					if !prev.Parallel && prev.Status != "done" && prev.Status != "skipped" {
						canStart = false
						break
					}
				}
				if canStart {
					a.startPlanStep(orch, plan, i, &state)
					startedAny = true
				}
			} else if step.Status != "done" && step.Status != "skipped" {
				allDone = false
			}
		}

		if allDone {
			plan.Status = "done"
			saveKanbanState(orch.dir, state)
			a.emitOrchestratorEvent(orch.planID, "done")
			log.Printf("[orchestrator] plan %s completed", orch.planID)
			return
		}

		if startedAny {
			saveKanbanState(orch.dir, state)
		}

		// Wait before checking again
		select {
		case <-orch.cancel:
			return
		case <-time.After(5 * time.Second):
		}

		// Check if running steps have completed
		a.checkRunningSteps(orch)
	}
}

// startPlanStep creates a session for a plan step.
func (a *AppService) startPlanStep(orch *orchestratorState, plan *Plan, stepIdx int, state *KanbanState) {
	step := &plan.Steps[stepIdx]

	// Build command for the step
	mode := "claude"
	argv := []string{a.resolvedClaudePath}
	if argv[0] == "" {
		argv[0] = "claude"
	}

	sessionID := a.CreateSession(argv, orch.dir, 24, 80, mode)
	step.SessionID = sessionID
	step.Status = "running"

	orch.mu.Lock()
	orch.running[step.CardID] = sessionID
	orch.mu.Unlock()

	// Move card to in_progress
	a.moveCardToColumn(state, step.CardID, ColInProgress)

	// Send the prompt after a brief delay for session startup
	go func() {
		time.Sleep(2 * time.Second)
		sess := a.sessions[sessionID]
		if sess != nil {
			sess.Write([]byte(step.Prompt + "\r"))
		}
	}()

	log.Printf("[orchestrator] started step %d (card %s) in session %d", stepIdx, step.CardID, sessionID)
	a.emitOrchestratorEvent(orch.planID, "step_started")
}

// checkRunningSteps checks if any running sessions have completed.
func (a *AppService) checkRunningSteps(orch *orchestratorState) {
	orch.mu.Lock()
	defer orch.mu.Unlock()

	for cardID, sessionID := range orch.running {
		a.mu.Lock()
		sess := a.sessions[sessionID]
		a.mu.Unlock()

		if sess == nil {
			orch.done[cardID] = true
			delete(orch.running, cardID)
			a.markStepDone(orch, cardID)
			continue
		}

		activity := activityString(sess.GetActivity())
		if activity == "done" {
			orch.done[cardID] = true
			delete(orch.running, cardID)
			a.markStepDone(orch, cardID)
		}
	}
}

// markStepDone updates the plan step and kanban card status.
func (a *AppService) markStepDone(orch *orchestratorState, cardID string) {
	state, err := loadKanbanState(orch.dir)
	if err != nil {
		return
	}

	for i := range state.Plans {
		if state.Plans[i].ID == orch.planID {
			for j := range state.Plans[i].Steps {
				if state.Plans[i].Steps[j].CardID == cardID {
					state.Plans[i].Steps[j].Status = "done"
					break
				}
			}
			break
		}
	}

	a.moveCardToColumn(&state, cardID, ColAutoReview)
	saveKanbanState(orch.dir, state)
	a.emitOrchestratorEvent(orch.planID, "step_done")
}

func (orch *orchestratorState) isRunning(cardID string) bool {
	orch.mu.Lock()
	defer orch.mu.Unlock()
	_, ok := orch.running[cardID]
	return ok
}

func (a *AppService) emitOrchestratorEvent(planID string, eventType string) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("orchestrator:update", map[string]string{
		"planId": planID,
		"type":   eventType,
	})
}
