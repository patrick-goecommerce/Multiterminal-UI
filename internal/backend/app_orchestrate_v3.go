package backend

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/engine"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// Active v3 orchestrations tracker (per card).
var (
	activeOrchV3Mu sync.Mutex
	activeOrchsV3  = map[string]context.CancelFunc{} // cardID -> cancel
)

// StartCardOrchestration begins the full v3 orchestration pipeline for a card.
// Runs in a goroutine — returns immediately. Progress is communicated via events:
//   - orchestration:started
//   - orchestration:awaiting-review (plan generated, user must approve)
//   - orchestration:error
func (a *AppService) StartCardOrchestration(dir, cardID string) error {
	activeOrchV3Mu.Lock()
	if _, exists := activeOrchsV3[cardID]; exists {
		activeOrchV3Mu.Unlock()
		return fmt.Errorf("orchestration already running for card %s", cardID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	activeOrchsV3[cardID] = cancel
	activeOrchV3Mu.Unlock()

	orch, err := a.createOrchestrator(dir)
	if err != nil {
		cancel()
		activeOrchV3Mu.Lock()
		delete(activeOrchsV3, cardID)
		activeOrchV3Mu.Unlock()
		return err
	}

	go a.runCardOrchestration(ctx, orch, dir, cardID)
	return nil
}

// ResumeCardOrchestration continues orchestration after user approves the plan.
// Card must be in "review" state. Also runs in a background goroutine.
// Events: orchestration:resumed, orchestration:completed, orchestration:error.
func (a *AppService) ResumeCardOrchestration(dir, cardID string) error {
	activeOrchV3Mu.Lock()
	if _, exists := activeOrchsV3[cardID]; exists {
		activeOrchV3Mu.Unlock()
		return fmt.Errorf("orchestration already running for card %s", cardID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	activeOrchsV3[cardID] = cancel
	activeOrchV3Mu.Unlock()

	orch, err := a.createOrchestrator(dir)
	if err != nil {
		cancel()
		activeOrchV3Mu.Lock()
		delete(activeOrchsV3, cardID)
		activeOrchV3Mu.Unlock()
		return err
	}

	go a.runResumeOrchestration(ctx, orch, dir, cardID)
	return nil
}

// CancelCardOrchestration stops a running v3 orchestration for a card.
func (a *AppService) CancelCardOrchestration(cardID string) error {
	activeOrchV3Mu.Lock()
	cancel, exists := activeOrchsV3[cardID]
	activeOrchV3Mu.Unlock()
	if !exists {
		return fmt.Errorf("no active orchestration for card %s", cardID)
	}
	cancel()
	return nil
}

// IsCardOrchestrationRunning checks if a card has an active v3 orchestration.
func (a *AppService) IsCardOrchestrationRunning(cardID string) bool {
	activeOrchV3Mu.Lock()
	_, exists := activeOrchsV3[cardID]
	activeOrchV3Mu.Unlock()
	return exists
}

// createOrchestrator builds the orchestrator stack for a directory.
func (a *AppService) createOrchestrator(dir string) (*orchestrator.Orchestrator, error) {
	if err := board.ValidateGitRepo(dir); err != nil {
		return nil, err
	}
	b := board.NewBoard(dir)
	eng := engine.NewHeadlessEngine(dir, 4)
	skillDir := filepath.Join(dir, ".mtui", "skills")
	return orchestrator.NewOrchestrator(b, eng, skillDir), nil
}

// runCardOrchestration is the background goroutine for StartCardOrchestration.
func (a *AppService) runCardOrchestration(
	ctx context.Context,
	orch *orchestrator.Orchestrator,
	dir, cardID string,
) {
	defer a.clearActiveOrch(cardID)

	log.Printf("[orchestrate-v3] starting for card %s in %s", cardID, dir)
	a.emitOrchV3Event("orchestration:started", cardID, dir, "")

	err := orch.RunCard(ctx, dir, cardID)
	if err != nil {
		log.Printf("[orchestrate-v3] card %s error: %v", cardID, err)
		a.emitOrchV3Event("orchestration:error", cardID, dir, err.Error())
		return
	}

	// RunCard stops at "review" state for non-trivial cards.
	log.Printf("[orchestrate-v3] card %s: awaiting review", cardID)
	a.emitOrchV3Event("orchestration:awaiting-review", cardID, dir, "")
}

// runResumeOrchestration is the background goroutine for ResumeCardOrchestration.
func (a *AppService) runResumeOrchestration(
	ctx context.Context,
	orch *orchestrator.Orchestrator,
	dir, cardID string,
) {
	defer a.clearActiveOrch(cardID)

	log.Printf("[orchestrate-v3] resuming after review for card %s", cardID)
	a.emitOrchV3Event("orchestration:resumed", cardID, dir, "")

	err := orch.ResumeAfterReview(ctx, dir, cardID)
	if err != nil {
		log.Printf("[orchestrate-v3] card %s resume error: %v", cardID, err)
		a.emitOrchV3Event("orchestration:error", cardID, dir, err.Error())
		return
	}

	log.Printf("[orchestrate-v3] card %s: completed successfully", cardID)
	a.emitOrchV3Event("orchestration:completed", cardID, dir, "")
}

// clearActiveOrch removes a card from the active orchestrations map.
func (a *AppService) clearActiveOrch(cardID string) {
	activeOrchV3Mu.Lock()
	delete(activeOrchsV3, cardID)
	activeOrchV3Mu.Unlock()
}

// emitOrchV3Event sends an orchestration v3 event to the frontend.
func (a *AppService) emitOrchV3Event(event, cardID, dir, errMsg string) {
	if a.app == nil {
		return
	}
	payload := map[string]string{
		"card_id": cardID,
		"dir":     dir,
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	a.app.Event.Emit(event, payload)
}
