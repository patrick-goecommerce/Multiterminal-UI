package orchestrator

import (
	"context"
	"sync"
	"testing"
)

// captureEmitter records emitted events for assertions.
type captureEmitter struct {
	mu     sync.Mutex
	events []string
}

func (c *captureEmitter) emit(event string, payload map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *captureEmitter) has(event string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e == event {
			return true
		}
	}
	return false
}

func TestRunCard_EmitsTriageAndPlanEvents(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-events", "Add feature", "desc")

	cap := &captureEmitter{}
	orch.SetProgressEmitter(cap.emit)

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"x"}`))
	eng.addResult(planResult(makePlanJSON("card-events", 2)))

	if err := orch.RunCard(context.Background(), dir, "card-events"); err != nil {
		t.Fatal(err)
	}

	if !cap.has("orchestration:triage-done") {
		t.Error("expected orchestration:triage-done event")
	}
	if !cap.has("orchestration:plan-ready") {
		t.Error("expected orchestration:plan-ready event")
	}
}

func TestResumeAfterReview_EmitsWaveStepAndQAEvents(t *testing.T) {
	orch, b, eng, dir := setupCardInReview(t, "card-events2", 1)

	cap := &captureEmitter{}
	orch.SetProgressEmitter(cap.emit)

	eng.addResult(ExecutionResult{StepID: "01", Status: StepSuccess, CostUSD: 0.10})

	if err := orch.ResumeAfterReview(context.Background(), dir, "card-events2"); err != nil {
		t.Fatal(err)
	}

	_ = b
	if !cap.has("orchestration:wave-started") {
		t.Error("expected orchestration:wave-started event")
	}
	if !cap.has("orchestration:step-done") {
		t.Error("expected orchestration:step-done event")
	}
	if !cap.has("orchestration:qa-result") {
		t.Error("expected orchestration:qa-result event")
	}
}
