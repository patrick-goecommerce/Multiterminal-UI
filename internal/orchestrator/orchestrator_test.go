package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// setupTestOrchestrator creates a temp git repo, board, mock engine, and orchestrator.
func setupTestOrchestrator(t *testing.T) (*Orchestrator, *board.Board, *testEngine, string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, stderr.String())
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")

	b := board.NewBoard(dir)
	eng := newTestEngine()
	skillDir := filepath.Join(dir, ".mtui", "skills")
	os.MkdirAll(skillDir, 0755)

	orch := NewOrchestrator(b, eng, skillDir)
	return orch, b, eng, dir
}

// createBacklogCard creates a card in backlog state on the board.
func createBacklogCard(t *testing.T, b *board.Board, id, title, desc string) {
	t.Helper()
	card := board.TaskCard{
		ID:          id,
		Title:       title,
		Description: desc,
		State:       board.StateBacklog,
		CardType:    board.CardTypeFeature,
	}
	if err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
}

// makePlanJSON builds a valid plan JSON string for testing.
func makePlanJSON(cardID string, numSteps int) string {
	var stepJSON []string
	for i := 1; i <= numSteps; i++ {
		id := fmt.Sprintf("%02d", i)
		stepJSON = append(stepJSON, `{
			"id": "`+id+`",
			"title": "Step `+id+`",
			"wave": 1,
			"depends_on": [],
			"parallel_ok": true,
			"model": "sonnet",
			"files_modify": [],
			"files_create": ["file`+id+`.go"],
			"must_haves": {"truths": [], "artifacts": []},
			"verify": [],
			"status": "pending"
		}`)
	}
	return `{
		"card_id": "` + cardID + `",
		"complexity": "medium",
		"steps": [` + strings.Join(stepJSON, ",") + `]
	}`
}

func TestRunCard_TrivialComplexity(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-trivial", "Fix typo", "Fix typo in README")

	// Engine returns trivial triage — no plan call expected.
	eng.addResult(triageResult(`{"complexity":"trivial","reasoning":"simple fix"}`))

	err := orch.RunCard(context.Background(), dir, "card-trivial")
	if err != nil {
		t.Fatal(err)
	}

	// Card should be in executing state (trivial skips planning).
	card, err := b.GetTask("card-trivial")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateExecuting {
		t.Errorf("state: got %q, want %q", card.State, board.StateExecuting)
	}
	if card.Complexity != board.ComplexityTrivial {
		t.Errorf("complexity: got %q, want %q", card.Complexity, board.ComplexityTrivial)
	}

	// Only triage call, no plan call.
	if eng.callCount() != 1 {
		t.Errorf("engine calls: got %d, want 1 (triage only)", eng.callCount())
	}
}

func TestRunCard_MediumComplexity(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-medium", "Add endpoint", "Add /api/users endpoint")

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"multi-file change"}`))
	eng.addResult(planResult(makePlanJSON("card-medium", 2)))

	err := orch.RunCard(context.Background(), dir, "card-medium")
	if err != nil {
		t.Fatal(err)
	}

	card, err := b.GetTask("card-medium")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateReview {
		t.Errorf("state: got %q, want %q", card.State, board.StateReview)
	}
	if card.Complexity != board.ComplexityMedium {
		t.Errorf("complexity: got %q, want %q", card.Complexity, board.ComplexityMedium)
	}
}

func TestRunCard_ComplexComplexity(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-complex", "Auth system", "Implement OAuth2 flow")

	eng.addResult(triageResult(`{"complexity":"complex","reasoning":"new subsystem"}`))
	eng.addResult(planResult(makePlanJSON("card-complex", 3)))

	err := orch.RunCard(context.Background(), dir, "card-complex")
	if err != nil {
		t.Fatal(err)
	}

	card, err := b.GetTask("card-complex")
	if err != nil {
		t.Fatal(err)
	}
	// Complex follows the same path as medium (quiz not implemented).
	if card.State != board.StateReview {
		t.Errorf("state: got %q, want %q", card.State, board.StateReview)
	}
	if card.Complexity != board.ComplexityComplex {
		t.Errorf("complexity: got %q, want %q", card.Complexity, board.ComplexityComplex)
	}
}

func TestRunCard_NonExistentCard(t *testing.T) {
	orch, _, _, dir := setupTestOrchestrator(t)

	err := orch.RunCard(context.Background(), dir, "no-such-card")
	if err == nil {
		t.Fatal("expected error for non-existent card")
	}
	if !strings.Contains(err.Error(), "get card") {
		t.Errorf("error should mention 'get card', got: %v", err)
	}
}

func TestRunCard_PlanSavedToBoard(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-plan", "Add feature", "Implement new feature")

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"new feature"}`))
	eng.addResult(planResult(makePlanJSON("card-plan", 2)))

	err := orch.RunCard(context.Background(), dir, "card-plan")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := b.GetPlan("card-plan")
	if err != nil {
		t.Fatalf("GetPlan: %v", err)
	}
	if plan.CardID != "card-plan" {
		t.Errorf("plan.CardID: got %q, want %q", plan.CardID, "card-plan")
	}
	if len(plan.Steps) != 2 {
		t.Errorf("plan.Steps: got %d, want 2", len(plan.Steps))
	}
}

func TestRunCard_StateTransitionsPersisted(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-states", "Refactor", "Refactor module X")

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"refactor"}`))
	eng.addResult(planResult(makePlanJSON("card-states", 1)))

	err := orch.RunCard(context.Background(), dir, "card-states")
	if err != nil {
		t.Fatal(err)
	}

	// Final state should be review, persisted via board.
	card, err := b.GetTask("card-states")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateReview {
		t.Errorf("persisted state: got %q, want %q", card.State, board.StateReview)
	}
}

func TestRunCard_BudgetAllocated(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-budget", "Task", "Some task")

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"test"}`))
	eng.addResult(planResult(makePlanJSON("card-budget", 1)))

	err := orch.RunCard(context.Background(), dir, "card-budget")
	if err != nil {
		t.Fatal(err)
	}

	remaining, err := orch.Budget().Remaining("card-budget")
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	// Medium default budget is $2.00.
	if remaining != 2.00 {
		t.Errorf("remaining budget: got %.2f, want 2.00", remaining)
	}
}

func TestRunCard_TechContextPassedToEngine(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)

	// Create a go.mod file so DetectStack finds it.
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	createBacklogCard(t, b, "card-ctx", "Add handler", "Add HTTP handler")

	eng.addResult(triageResult(`{"complexity":"trivial","reasoning":"simple"}`))

	err := orch.RunCard(context.Background(), dir, "card-ctx")
	if err != nil {
		t.Fatal(err)
	}

	// The triage call should include "go.mod" in the prompt (tech context).
	call := eng.lastCall()
	if !strings.Contains(call.Prompt, "go.mod") {
		t.Errorf("triage prompt should contain 'go.mod', got: %s", call.Prompt)
	}
}

func TestResumeAfterReview_NotImplemented(t *testing.T) {
	orch, _, _, dir := setupTestOrchestrator(t)

	err := orch.ResumeAfterReview(context.Background(), dir, "any-card")
	if err == nil {
		t.Fatal("expected error from stub")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got: %v", err)
	}
}
