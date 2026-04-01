package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// setupCardInReview creates a card, runs it through RunCard to reach "review" state,
// and returns the orchestrator, board, engine, and dir.
func setupCardInReview(t *testing.T, cardID string, numSteps int) (*Orchestrator, *board.Board, *testEngine, string) {
	t.Helper()
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, cardID, "Test card", "Test description")

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"multi-step"}`))
	eng.addResult(planResult(makePlanJSON(cardID, numSteps)))

	if err := orch.RunCard(context.Background(), dir, cardID); err != nil {
		t.Fatalf("RunCard: %v", err)
	}

	card, err := b.GetTask(cardID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if card.State != board.StateReview {
		t.Fatalf("expected review state, got %s", card.State)
	}
	return orch, b, eng, dir
}

// makeMultiWavePlanJSON builds a plan with steps across multiple waves via dependencies.
func makeMultiWavePlanJSON(cardID string) string {
	return `{
		"card_id": "` + cardID + `",
		"complexity": "medium",
		"steps": [
			{
				"id": "01",
				"title": "Step 01 (wave 1)",
				"wave": 1,
				"depends_on": [],
				"parallel_ok": true,
				"model": "sonnet",
				"files_modify": [],
				"files_create": ["file01.go"],
				"must_haves": {"truths": [], "artifacts": []},
				"verify": [],
				"status": "pending"
			},
			{
				"id": "02",
				"title": "Step 02 (wave 2, depends on 01)",
				"wave": 2,
				"depends_on": ["01"],
				"parallel_ok": true,
				"model": "sonnet",
				"files_modify": [],
				"files_create": ["file02.go"],
				"must_haves": {"truths": [], "artifacts": []},
				"verify": [],
				"status": "pending"
			}
		]
	}`
}

func TestResumeAfterReview_AllStepsSucceed(t *testing.T) {
	orch, b, eng, dir := setupCardInReview(t, "card-resume", 2)

	// Add success results for each step execution.
	eng.addResult(ExecutionResult{StepID: "01", Status: StepSuccess, CostUSD: 0.10})
	eng.addResult(ExecutionResult{StepID: "02", Status: StepSuccess, CostUSD: 0.15})

	err := orch.ResumeAfterReview(context.Background(), dir, "card-resume")
	if err != nil {
		t.Fatal(err)
	}

	// Card should be in done state (QA passes — no must_haves artifacts).
	card, err := b.GetTask("card-resume")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateDone {
		t.Errorf("state: got %q, want %q", card.State, board.StateDone)
	}
}

func TestResumeAfterReview_WrongState(t *testing.T) {
	orch, b, _, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-wrong", "Wrong state", "Not in review")

	err := orch.ResumeAfterReview(context.Background(), dir, "card-wrong")
	if err == nil {
		t.Fatal("expected error for wrong state")
	}
	if !strings.Contains(err.Error(), "expected review") {
		t.Errorf("error should mention expected review, got: %v", err)
	}
}

func TestResumeAfterReview_NonExistentCard(t *testing.T) {
	orch, _, _, dir := setupTestOrchestrator(t)

	err := orch.ResumeAfterReview(context.Background(), dir, "no-such-card")
	if err == nil {
		t.Fatal("expected error for non-existent card")
	}
	if !strings.Contains(err.Error(), "get card") {
		t.Errorf("error should mention 'get card', got: %v", err)
	}
}

func TestResumeAfterReview_BudgetExhausted(t *testing.T) {
	orch, b, eng, dir := setupCardInReview(t, "card-budget-ex", 1)

	// Drain the budget completely before resuming.
	orch.Budget().Spend("card-budget-ex", 2.00)

	// Even though we add a result, it should never be called.
	eng.addResult(ExecutionResult{StepID: "01", Status: StepSuccess, CostUSD: 0.01})

	err := orch.ResumeAfterReview(context.Background(), dir, "card-budget-ex")
	if err == nil {
		t.Fatal("expected budget exhaustion error")
	}
	if !strings.Contains(err.Error(), "budget exhausted") {
		t.Errorf("error should mention budget exhausted, got: %v", err)
	}

	// Card should still be in executing (transitioned from review before budget check).
	card, err := b.GetTask("card-budget-ex")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateExecuting {
		t.Errorf("state: got %q, want %q", card.State, board.StateExecuting)
	}
}

func TestResumeAfterReview_StuckStep(t *testing.T) {
	orch, b, eng, dir := setupCardInReview(t, "card-stuck", 1)

	eng.addResult(ExecutionResult{StepID: "01", Status: StepStuck, CostUSD: 0.05})

	err := orch.ResumeAfterReview(context.Background(), dir, "card-stuck")
	if err == nil {
		t.Fatal("expected stuck error")
	}
	if !strings.Contains(err.Error(), "stuck") {
		t.Errorf("error should mention stuck, got: %v", err)
	}

	// Card should end in human_review.
	card, err := b.GetTask("card-stuck")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateHumanReview {
		t.Errorf("state: got %q, want %q", card.State, board.StateHumanReview)
	}
	if card.ReviewReason != "step_stuck" {
		t.Errorf("review_reason: got %q, want %q", card.ReviewReason, "step_stuck")
	}
}

func TestResumeAfterReview_FailedStep(t *testing.T) {
	orch, _, eng, dir := setupCardInReview(t, "card-failed", 1)

	eng.addResult(ExecutionResult{StepID: "01", Status: StepFailed, CostUSD: 0.05})

	err := orch.ResumeAfterReview(context.Background(), dir, "card-failed")
	if err == nil {
		t.Fatal("expected failure error")
	}
	if !strings.Contains(err.Error(), "failed with status") {
		t.Errorf("error should mention failed status, got: %v", err)
	}
}

func TestResumeAfterReview_MultiWaveOrder(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	cardID := "card-multiwave"
	createBacklogCard(t, b, cardID, "Multi wave", "Test wave ordering")

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"multi-wave"}`))
	eng.addResult(planResult(makeMultiWavePlanJSON(cardID)))

	if err := orch.RunCard(context.Background(), dir, cardID); err != nil {
		t.Fatal(err)
	}

	// Add results for 2 steps across 2 waves.
	eng.addResult(ExecutionResult{StepID: "01", Status: StepSuccess, CostUSD: 0.10})
	eng.addResult(ExecutionResult{StepID: "02", Status: StepSuccess, CostUSD: 0.10})

	err := orch.ResumeAfterReview(context.Background(), dir, cardID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify wave 1 (step 01) executed before wave 2 (step 02).
	// The engine calls include triage + plan + step01 + step02.
	eng.mu.Lock()
	defer eng.mu.Unlock()
	if len(eng.calls) < 4 {
		t.Fatalf("expected at least 4 engine calls, got %d", len(eng.calls))
	}
	// Calls [2] and [3] are the step executions.
	if eng.calls[2].StepID != "01" {
		t.Errorf("first step call: got %q, want %q", eng.calls[2].StepID, "01")
	}
	if eng.calls[3].StepID != "02" {
		t.Errorf("second step call: got %q, want %q", eng.calls[3].StepID, "02")
	}

	card, err := b.GetTask(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateDone {
		t.Errorf("state: got %q, want %q", card.State, board.StateDone)
	}
}

func TestResumeAfterReview_CostTracked(t *testing.T) {
	orch, _, eng, dir := setupCardInReview(t, "card-cost", 2)

	eng.addResult(ExecutionResult{StepID: "01", Status: StepSuccess, CostUSD: 0.30})
	eng.addResult(ExecutionResult{StepID: "02", Status: StepSuccess, CostUSD: 0.50})

	err := orch.ResumeAfterReview(context.Background(), dir, "card-cost")
	if err != nil {
		t.Fatal(err)
	}

	remaining, err := orch.Budget().Remaining("card-cost")
	if err != nil {
		t.Fatal(err)
	}
	// Medium budget = $2.00, spent $0.30 + $0.50 = $0.80, remaining = $1.20
	expected := 1.20
	if fmt.Sprintf("%.2f", remaining) != fmt.Sprintf("%.2f", expected) {
		t.Errorf("remaining: got %.2f, want %.2f", remaining, expected)
	}
}

func TestMergeVerifyCommands(t *testing.T) {
	stepVerify := []VerifyStep{
		{Command: "go test ./...", Description: "Run tests"},
		{Command: "go vet ./...", Description: "Vet"},
	}
	skillVerify := []VerifyStep{
		{Command: "go vet ./...", Description: "Vet (skill)"}, // duplicate
		{Command: "golint ./...", Description: "Lint"},
	}

	result := mergeVerifyCommands(stepVerify, skillVerify)

	if len(result) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(result))
	}

	cmds := make([]string, len(result))
	for i, v := range result {
		cmds[i] = v.Command
	}

	expected := []string{"go test ./...", "go vet ./...", "golint ./..."}
	for i, exp := range expected {
		if cmds[i] != exp {
			t.Errorf("command[%d]: got %q, want %q", i, cmds[i], exp)
		}
	}

	// The "go vet" description should be from step (first wins).
	for _, v := range result {
		if v.Command == "go vet ./..." && v.Description != "Vet" {
			t.Errorf("description for duplicate should be from step, got %q", v.Description)
		}
	}
}

func TestAddContextToPrompt(t *testing.T) {
	prompt := "## Task: Do stuff\n"
	paths := ContextPaths{
		DirectFiles:   []string{"internal/foo.go", "internal/bar.go"},
		NeighborFiles: []string{"internal/baz.go"},
		TestFiles:     []string{"internal/foo_test.go"},
	}

	result := addContextToPrompt(prompt, paths)

	if !strings.Contains(result, "### Read these files first") {
		t.Error("should contain direct files section")
	}
	if !strings.Contains(result, "`internal/foo.go`") {
		t.Error("should contain direct file path")
	}
	if !strings.Contains(result, "### Neighbor files") {
		t.Error("should contain neighbor files section")
	}
	if !strings.Contains(result, "`internal/baz.go`") {
		t.Error("should contain neighbor file path")
	}
	if !strings.Contains(result, "### Test files") {
		t.Error("should contain test files section")
	}
	if !strings.Contains(result, "`internal/foo_test.go`") {
		t.Error("should contain test file path")
	}
}

func TestAddContextToPrompt_EmptyPaths(t *testing.T) {
	prompt := "## Task: Do stuff\n"
	paths := ContextPaths{}

	result := addContextToPrompt(prompt, paths)

	// Should just be the original prompt, no extra sections.
	if result != prompt {
		t.Errorf("empty paths should not modify prompt, got: %q", result)
	}
}
