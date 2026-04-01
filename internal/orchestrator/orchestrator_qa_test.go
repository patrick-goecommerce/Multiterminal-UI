package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// makePlanWithArtifacts builds a plan JSON with must_haves artifact requirements.
func makePlanWithArtifacts(cardID string, artifacts []struct{ path string; minLines int }) string {
	var artJSON []string
	for _, a := range artifacts {
		artJSON = append(artJSON, fmt.Sprintf(`{"path": %q, "min_lines": %d}`, a.path, a.minLines))
	}
	return `{
		"card_id": "` + cardID + `",
		"complexity": "medium",
		"steps": [{
			"id": "01",
			"title": "Implement feature",
			"wave": 1,
			"depends_on": [],
			"parallel_ok": true,
			"model": "sonnet",
			"files_modify": [],
			"files_create": [],
			"must_haves": {
				"truths": [],
				"artifacts": [` + strings.Join(artJSON, ",") + `]
			},
			"verify": [],
			"status": "pending"
		}]
	}`
}

// setupCardInQA creates a card in QA state with a plan that has artifact requirements.
// It runs through the full pipeline: backlog -> triage -> planning -> review -> executing -> qa.
func setupCardInQA(t *testing.T, cardID string, artifacts []struct{ path string; minLines int }) (*Orchestrator, *board.Board, *testEngine, string) {
	t.Helper()
	orch, b, eng, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, cardID, "Test card", "Test description")

	planJSON := makePlanWithArtifacts(cardID, artifacts)

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"multi-step"}`))
	eng.addResult(planResult(planJSON))

	if err := orch.RunCard(context.Background(), dir, cardID); err != nil {
		t.Fatalf("RunCard: %v", err)
	}

	// Add step execution result and resume to reach QA state
	eng.addResult(ExecutionResult{StepID: "01", Status: StepSuccess, CostUSD: 0.10})

	// We need to manually drive through ResumeAfterReview but stop before QA.
	// Instead, set up the card in QA state directly.
	card, err := b.GetTask(cardID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}

	// Transition review -> executing
	sm := board.NewStateMachine()
	result, err := sm.Transition(card, board.EventApproved)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	card.State = result.NewState
	b.UpdateTask(card)

	// Transition executing -> qa
	result, err = sm.Transition(card, board.EventAllStepsDone)
	if err != nil {
		t.Fatalf("all steps done: %v", err)
	}
	card.State = result.NewState
	b.UpdateTask(card)

	return orch, b, eng, dir
}

func TestRunQA_AllArtifactsExist(t *testing.T) {
	arts := []struct{ path string; minLines int }{
		{"internal/backend/app_auth.go", 5},
	}
	orch, b, _, dir := setupCardInQA(t, "card-qa-pass", arts)

	// Create the artifact file
	os.MkdirAll(filepath.Join(dir, "internal/backend"), 0755)
	content := "package backend\n\n// Auth handles authentication.\nfunc Auth() {\n\treturn\n}\n"
	os.WriteFile(filepath.Join(dir, "internal/backend/app_auth.go"), []byte(content), 0644)

	err := orch.RunQA(context.Background(), dir, "card-qa-pass")
	if err != nil {
		t.Fatal(err)
	}

	card, err := b.GetTask("card-qa-pass")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateDone {
		t.Errorf("state: got %q, want %q", card.State, board.StateDone)
	}
}

func TestRunQA_ArtifactMissing_FixSucceeds(t *testing.T) {
	arts := []struct{ path string; minLines int }{
		{"internal/backend/app_auth.go", 0},
	}
	orch, b, eng, dir := setupCardInQA(t, "card-qa-fix", arts)

	// File does NOT exist initially.
	// Add a fix result that "creates" the file.
	eng.addResult(ExecutionResult{
		StepID:  "qa-fix-1",
		Status:  StepSuccess,
		CostUSD: 0.05,
	})

	// The engine mock doesn't create files, so we simulate the fix by creating it
	// after the engine call. We'll use a custom approach: create the file before RunQA
	// but with wrong content (directory), then fix it.
	// Actually, simpler: create the file in a goroutine triggered by engine call.
	// Even simpler: override engine to create the file as a side-effect.

	// For this test, we use a wrapper approach: the engine creates the file.
	origExecute := eng.Execute
	_ = origExecute // We can't easily override. Instead, create file after first fix attempt.

	// Let's take a different approach: run RunQA in a goroutine and create the file
	// when the fix attempt happens. But that's racy.
	// Simplest: just pre-create the file so QA passes on first check.
	// But the test is supposed to test the fix loop.

	// Best approach: use the engine's Execute to create the file as side-effect.
	// We'll replace the engine with a custom one.
	fixDir := dir
	fixEng := &fixCreatingEngine{
		testEngine: eng,
		dir:        fixDir,
		filePath:   "internal/backend/app_auth.go",
		content:    "package backend\n// fixed\n",
	}
	orch2 := NewOrchestrator(b, fixEng, filepath.Join(dir, ".mtui", "skills"))
	// Copy budget allocation
	orch2.budget = orch.budget

	err := orch2.RunQA(context.Background(), dir, "card-qa-fix")
	if err != nil {
		t.Fatal(err)
	}

	card, err := b.GetTask("card-qa-fix")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateDone {
		t.Errorf("state: got %q, want %q", card.State, board.StateDone)
	}
}

// fixCreatingEngine wraps testEngine and creates a file on qa-fix Execute calls.
type fixCreatingEngine struct {
	*testEngine
	dir      string
	filePath string
	content  string
}

func (e *fixCreatingEngine) Execute(ctx context.Context, req ExecutionRequest) (ExecutionResult, error) {
	result, err := e.testEngine.Execute(ctx, req)
	if err != nil {
		return result, err
	}
	// If this is a qa-fix call, create the file as a side-effect.
	if strings.HasPrefix(req.StepID, "qa-fix-") {
		fullPath := filepath.Join(e.dir, e.filePath)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(e.content), 0644)
	}
	return result, nil
}

func TestRunQA_ArtifactTooFewLines(t *testing.T) {
	arts := []struct{ path string; minLines int }{
		{"internal/backend/app_auth.go", 30},
	}
	orch, b, eng, dir := setupCardInQA(t, "card-qa-lines", arts)

	// Create artifact with only 5 lines
	os.MkdirAll(filepath.Join(dir, "internal/backend"), 0755)
	os.WriteFile(filepath.Join(dir, "internal/backend/app_auth.go"), []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	// The fix engine will write a file with 30+ lines
	var longContent strings.Builder
	for i := 0; i < 35; i++ {
		fmt.Fprintf(&longContent, "line %d\n", i+1)
	}
	eng.addResult(ExecutionResult{StepID: "qa-fix-1", Status: StepSuccess, CostUSD: 0.05})

	fixEng := &fixCreatingEngine{
		testEngine: eng,
		dir:        dir,
		filePath:   "internal/backend/app_auth.go",
		content:    longContent.String(),
	}
	orch2 := NewOrchestrator(b, fixEng, filepath.Join(dir, ".mtui", "skills"))
	orch2.budget = orch.budget

	err := orch2.RunQA(context.Background(), dir, "card-qa-lines")
	if err != nil {
		t.Fatal(err)
	}

	card, err := b.GetTask("card-qa-lines")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateDone {
		t.Errorf("state: got %q, want %q", card.State, board.StateDone)
	}
}

func TestRunQA_FixLoopExhausted(t *testing.T) {
	arts := []struct{ path string; minLines int }{
		{"missing_file.go", 0},
	}
	orch, b, eng, dir := setupCardInQA(t, "card-qa-exhaust", arts)

	// File never gets created. Add 3 fix results that all "succeed" but don't fix anything.
	eng.addResult(ExecutionResult{StepID: "qa-fix-1", Status: StepSuccess, CostUSD: 0.05})
	eng.addResult(ExecutionResult{StepID: "qa-fix-2", Status: StepSuccess, CostUSD: 0.05})
	eng.addResult(ExecutionResult{StepID: "qa-fix-3", Status: StepSuccess, CostUSD: 0.05})

	err := orch.RunQA(context.Background(), dir, "card-qa-exhaust")
	if err == nil {
		t.Fatal("expected error after fix loop exhaustion")
	}
	if !strings.Contains(err.Error(), "QA fix loop exhausted") {
		t.Errorf("error should mention fix loop exhausted, got: %v", err)
	}

	card, err := b.GetTask("card-qa-exhaust")
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateHumanReview {
		t.Errorf("state: got %q, want %q", card.State, board.StateHumanReview)
	}
	if card.ReviewReason != "qa_fix_exhausted" {
		t.Errorf("review_reason: got %q, want %q", card.ReviewReason, "qa_fix_exhausted")
	}
}

func TestRunQA_WrongState(t *testing.T) {
	orch, b, _, dir := setupTestOrchestrator(t)
	createBacklogCard(t, b, "card-qa-wrong", "Wrong state", "Not in QA")

	err := orch.RunQA(context.Background(), dir, "card-qa-wrong")
	if err == nil {
		t.Fatal("expected error for wrong state")
	}
	if !strings.Contains(err.Error(), "expected qa") {
		t.Errorf("error should mention expected qa, got: %v", err)
	}
}

func TestRunQA_FixUsesSonnetModel(t *testing.T) {
	arts := []struct{ path string; minLines int }{
		{"missing_file.go", 0},
	}
	orch, _, eng, dir := setupCardInQA(t, "card-qa-model", arts)

	// File never gets created. We only need 1 fix attempt to check the model.
	eng.addResult(ExecutionResult{StepID: "qa-fix-1", Status: StepSuccess, CostUSD: 0.05})
	eng.addResult(ExecutionResult{StepID: "qa-fix-2", Status: StepSuccess, CostUSD: 0.05})
	eng.addResult(ExecutionResult{StepID: "qa-fix-3", Status: StepSuccess, CostUSD: 0.05})

	_ = orch.RunQA(context.Background(), dir, "card-qa-model")

	// Check that the QA fix calls used "sonnet" model.
	eng.mu.Lock()
	defer eng.mu.Unlock()

	found := false
	for _, call := range eng.calls {
		if strings.HasPrefix(call.StepID, "qa-fix-") {
			found = true
			if call.Model != "sonnet" {
				t.Errorf("QA fix call %s used model %q, want %q", call.StepID, call.Model, "sonnet")
			}
		}
	}
	if !found {
		t.Error("no QA fix calls found in engine")
	}
}

func TestResumeAfterReview_FullFlow_WavesQADone(t *testing.T) {
	orch, b, eng, dir := setupTestOrchestrator(t)
	cardID := "card-full-flow"
	createBacklogCard(t, b, cardID, "Full flow test", "End to end")

	// Plan with 1 step that requires an artifact
	planJSON := `{
		"card_id": "` + cardID + `",
		"complexity": "medium",
		"steps": [{
			"id": "01",
			"title": "Create auth module",
			"wave": 1,
			"depends_on": [],
			"parallel_ok": true,
			"model": "sonnet",
			"files_modify": [],
			"files_create": ["auth.go"],
			"must_haves": {
				"truths": [],
				"artifacts": [{"path": "auth.go", "min_lines": 3}]
			},
			"verify": [],
			"status": "pending"
		}]
	}`

	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"new module"}`))
	eng.addResult(planResult(planJSON))

	if err := orch.RunCard(context.Background(), dir, cardID); err != nil {
		t.Fatal(err)
	}

	// Create the artifact file before wave execution (simulating engine side-effect)
	content := "package main\n\nfunc auth() {\n\treturn\n}\n"
	os.WriteFile(filepath.Join(dir, "auth.go"), []byte(content), 0644)

	// Step execution succeeds
	eng.addResult(ExecutionResult{StepID: "01", Status: StepSuccess, CostUSD: 0.20})

	err := orch.ResumeAfterReview(context.Background(), dir, cardID)
	if err != nil {
		t.Fatal(err)
	}

	card, err := b.GetTask(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if card.State != board.StateDone {
		t.Errorf("state: got %q, want %q", card.State, board.StateDone)
	}
}

func TestCheckMustHaves_NoArtifacts(t *testing.T) {
	dir := t.TempDir()
	plan := Plan{
		Steps: []PlanStep{
			{ID: "01", MustHaves: MustHaves{}},
		},
	}

	passed, failures := checkMustHaves(dir, plan)
	if !passed {
		t.Errorf("expected pass with no artifacts, got failures: %v", failures)
	}
}

func TestCheckMustHaves_ArtifactIsDirectory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "some_dir"), 0755)

	plan := Plan{
		Steps: []PlanStep{
			{ID: "01", MustHaves: MustHaves{
				Artifacts: []ArtifactRequirement{{Path: "some_dir"}},
			}},
		},
	}

	passed, failures := checkMustHaves(dir, plan)
	if passed {
		t.Error("expected failure for directory artifact")
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "directory") {
		t.Errorf("expected directory failure, got: %v", failures)
	}
}

func TestBuildBriefingStub(t *testing.T) {
	briefing := buildBriefingStub(t.TempDir())
	if briefing.Recommendation != "proceed_to_qa" {
		t.Errorf("stub should always recommend proceed_to_qa, got %q", briefing.Recommendation)
	}
	if briefing.ScopeStatus != "within_limits" {
		t.Errorf("stub scope_status should be within_limits, got %q", briefing.ScopeStatus)
	}
}
