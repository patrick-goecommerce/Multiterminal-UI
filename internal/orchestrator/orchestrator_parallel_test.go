package orchestrator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// barrierEngine records the maximum number of concurrent Execute calls. Each
// call blocks until `need` calls are simultaneously in flight (or ctx expires),
// so a sequential caller can never reach a concurrency above 1.
type barrierEngine struct {
	mu         sync.Mutex
	need       int
	conc       int
	maxConc    int
	arrived    int
	gate       chan struct{}
	gateClosed bool
}

func newBarrierEngine(need int) *barrierEngine {
	return &barrierEngine{need: need, gate: make(chan struct{})}
}

func (e *barrierEngine) Execute(ctx context.Context, req ExecutionRequest) (ExecutionResult, error) {
	e.mu.Lock()
	e.conc++
	e.arrived++
	if e.conc > e.maxConc {
		e.maxConc = e.conc
	}
	if e.arrived >= e.need && !e.gateClosed {
		e.gateClosed = true
		close(e.gate)
	}
	e.mu.Unlock()

	select {
	case <-e.gate:
	case <-ctx.Done():
	}

	e.mu.Lock()
	e.conc--
	e.mu.Unlock()
	return ExecutionResult{StepID: req.StepID, Status: StepSuccess}, nil
}

func (e *barrierEngine) Cancel(string) error { return nil }

func (e *barrierEngine) maxConcurrency() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.maxConc
}

func TestExecuteWave_RunsStepsInParallel(t *testing.T) {
	_, b, _, dir := setupTestOrchestrator(t)
	cardID := "card-parallel"
	createBacklogCard(t, b, cardID, "Parallel", "two independent steps")

	// Place the card in review with a single wave of two independent steps.
	card, _ := b.GetTask(cardID)
	card.State = board.StateReview
	card.Complexity = board.ComplexityMedium
	if err := b.UpdateTask(card); err != nil {
		t.Fatal(err)
	}
	plan := board.Plan{
		CardID:     cardID,
		Complexity: board.ComplexityMedium,
		Steps: []board.PlanStep{
			{ID: "01", Title: "S1", Wave: 1, ParallelOk: true, Model: "sonnet", FilesCreate: []string{"f01.go"}},
			{ID: "02", Title: "S2", Wave: 1, ParallelOk: true, Model: "sonnet", FilesCreate: []string{"f02.go"}},
		},
	}
	if err := b.SavePlan(cardID, plan); err != nil {
		t.Fatal(err)
	}

	barrier := newBarrierEngine(2)
	skillDir := filepath.Join(dir, ".mtui", "skills")
	orch := NewOrchestrator(b, barrier, skillDir)
	orch.Budget().Allocate(cardID, "medium")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := orch.ResumeAfterReview(ctx, dir, cardID); err != nil {
		t.Fatalf("ResumeAfterReview: %v", err)
	}

	if mc := barrier.maxConcurrency(); mc != 2 {
		t.Errorf("expected the wave's 2 steps to run concurrently, max concurrency was %d", mc)
	}
}
