package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// setupStuckCard creates an orchestrator with a card in "stuck" state.
func setupStuckCard(t *testing.T, cardID string, model string, escAttempts int) (*Orchestrator, *board.Board, *testEngine, string) {
	t.Helper()
	orch, b, eng, dir := setupTestOrchestrator(t)

	card := board.TaskCard{
		ID:          cardID,
		Title:       "Stuck card",
		Description: "A card that got stuck",
		State:       board.StateStuck,
		CardType:    board.CardTypeFeature,
		EscAttempts: escAttempts,
	}
	if err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	return orch, b, eng, dir
}

func makeFailedStep(model string) PlanStep {
	return PlanStep{
		ID:          "step-01",
		Title:       "Implement feature",
		Wave:        1,
		Model:       model,
		FilesModify: []string{"internal/foo.go"},
		FilesCreate: []string{"internal/bar.go"},
		MustHaves:   MustHaves{Truths: []string{"feature works"}},
		Status:      "stuck",
	}
}

func TestEscalate_HaikuToSonnet(t *testing.T) {
	orch, b, _, dir := setupStuckCard(t, "esc-haiku", "haiku", 0)
	step := makeFailedStep("haiku")

	res, err := orch.Escalate(context.Background(), dir, "esc-haiku", step, "build failed")
	if err != nil {
		t.Fatal(err)
	}

	if res.Action != "model_escalated" {
		t.Errorf("action: got %q, want model_escalated", res.Action)
	}
	if res.NewModel != "sonnet" {
		t.Errorf("new model: got %q, want sonnet", res.NewModel)
	}
	if !strings.Contains(res.Reason, "haiku") || !strings.Contains(res.Reason, "sonnet") {
		t.Errorf("reason should mention haiku and sonnet, got: %s", res.Reason)
	}

	card, _ := b.GetTask("esc-haiku")
	if card.State != board.StateExecuting {
		t.Errorf("state: got %q, want executing", card.State)
	}
	if card.EscAttempts != 1 {
		t.Errorf("esc_attempts: got %d, want 1", card.EscAttempts)
	}
}

func TestEscalate_SonnetToOpus(t *testing.T) {
	orch, b, _, dir := setupStuckCard(t, "esc-sonnet", "sonnet", 0)
	step := makeFailedStep("sonnet")

	res, err := orch.Escalate(context.Background(), dir, "esc-sonnet", step, "test failed")
	if err != nil {
		t.Fatal(err)
	}

	if res.Action != "model_escalated" {
		t.Errorf("action: got %q, want model_escalated", res.Action)
	}
	if res.NewModel != "opus" {
		t.Errorf("new model: got %q, want opus", res.NewModel)
	}

	card, _ := b.GetTask("esc-sonnet")
	if card.State != board.StateExecuting {
		t.Errorf("state: got %q, want executing", card.State)
	}
}

func TestEscalate_OpusFallsToReplan(t *testing.T) {
	orch, b, eng, dir := setupStuckCard(t, "esc-opus", "opus", 0)
	step := makeFailedStep("opus")

	// Provide a valid replan result.
	eng.addResult(ExecutionResult{
		StepID: "replan-step-01",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: `[
			{"id":"sub-1","title":"Part A","model":"sonnet","files_modify":["internal/foo.go"],"files_create":[]},
			{"id":"sub-2","title":"Part B","model":"sonnet","files_modify":[],"files_create":["internal/bar.go"]}
		]`}},
	})

	res, err := orch.Escalate(context.Background(), dir, "esc-opus", step, "stuck on complex logic")
	if err != nil {
		t.Fatal(err)
	}

	if res.Action != "replanned" {
		t.Errorf("action: got %q, want replanned", res.Action)
	}
	if len(res.SubSteps) != 2 {
		t.Fatalf("sub-steps: got %d, want 2", len(res.SubSteps))
	}

	card, _ := b.GetTask("esc-opus")
	if card.State != board.StateExecuting {
		t.Errorf("state: got %q, want executing", card.State)
	}
	if card.EscAttempts != 1 {
		t.Errorf("esc_attempts: got %d, want 1", card.EscAttempts)
	}
}

func TestEscalate_ReplanTooManySubSteps(t *testing.T) {
	orch, b, eng, dir := setupStuckCard(t, "esc-too-many", "opus", 0)
	step := makeFailedStep("opus")

	// Return 4 sub-steps (exceeds max of 3).
	eng.addResult(ExecutionResult{
		StepID: "replan-step-01",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: `[
			{"id":"s1","title":"A","model":"sonnet","files_modify":["internal/foo.go"],"files_create":[]},
			{"id":"s2","title":"B","model":"sonnet","files_modify":["internal/foo.go"],"files_create":[]},
			{"id":"s3","title":"C","model":"sonnet","files_modify":["internal/foo.go"],"files_create":[]},
			{"id":"s4","title":"D","model":"sonnet","files_modify":["internal/foo.go"],"files_create":[]}
		]`}},
	})

	res, err := orch.Escalate(context.Background(), dir, "esc-too-many", step, "stuck")
	if err != nil {
		t.Fatal(err)
	}

	if res.Action != "human_review" {
		t.Errorf("action: got %q, want human_review", res.Action)
	}

	card, _ := b.GetTask("esc-too-many")
	if card.State != board.StateHumanReview {
		t.Errorf("state: got %q, want human_review", card.State)
	}
	if card.ReviewReason != "scope_expansion_required" {
		t.Errorf("review_reason: got %q, want scope_expansion_required", card.ReviewReason)
	}
}

func TestEscalate_ReplanFileOutsideScope(t *testing.T) {
	orch, b, eng, dir := setupStuckCard(t, "esc-scope", "opus", 0)
	step := makeFailedStep("opus")

	// Sub-step references a file not in the original scope.
	eng.addResult(ExecutionResult{
		StepID: "replan-step-01",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: `[
			{"id":"s1","title":"A","model":"sonnet","files_modify":["internal/UNAUTHORIZED.go"],"files_create":[]}
		]`}},
	})

	res, err := orch.Escalate(context.Background(), dir, "esc-scope", step, "stuck")
	if err != nil {
		t.Fatal(err)
	}

	if res.Action != "human_review" {
		t.Errorf("action: got %q, want human_review", res.Action)
	}

	card, _ := b.GetTask("esc-scope")
	if card.State != board.StateHumanReview {
		t.Errorf("state: got %q, want human_review", card.State)
	}
}

func TestEscalate_MaxEscalationsReached(t *testing.T) {
	orch, b, _, dir := setupStuckCard(t, "esc-max", "sonnet", 2)
	step := makeFailedStep("sonnet")

	res, err := orch.Escalate(context.Background(), dir, "esc-max", step, "still stuck")
	if err != nil {
		t.Fatal(err)
	}

	if res.Action != "human_review" {
		t.Errorf("action: got %q, want human_review", res.Action)
	}
	if !strings.Contains(res.Reason, "max escalations") {
		t.Errorf("reason should mention max escalations, got: %s", res.Reason)
	}

	card, _ := b.GetTask("esc-max")
	if card.State != board.StateHumanReview {
		t.Errorf("state: got %q, want human_review", card.State)
	}
	if card.ReviewReason != "max_escalations_reached" {
		t.Errorf("review_reason: got %q, want max_escalations_reached", card.ReviewReason)
	}
}

func TestEscalate_WrongState(t *testing.T) {
	orch, b, _, dir := setupTestOrchestrator(t)

	card := board.TaskCard{
		ID:       "esc-wrong",
		Title:    "Not stuck",
		State:    board.StateExecuting,
		CardType: board.CardTypeFeature,
	}
	if err := b.CreateTask(card); err != nil {
		t.Fatal(err)
	}

	step := makeFailedStep("sonnet")
	_, err := orch.Escalate(context.Background(), dir, "esc-wrong", step, "reason")
	if err == nil {
		t.Fatal("expected error for wrong state")
	}
	if !strings.Contains(err.Error(), "expected stuck") {
		t.Errorf("error should mention expected stuck, got: %v", err)
	}
}

func TestEscalate_EscAttemptsIncremented(t *testing.T) {
	orch, b, _, dir := setupStuckCard(t, "esc-inc", "haiku", 0)
	step := makeFailedStep("haiku")

	_, err := orch.Escalate(context.Background(), dir, "esc-inc", step, "fail")
	if err != nil {
		t.Fatal(err)
	}

	card, _ := b.GetTask("esc-inc")
	if card.EscAttempts != 1 {
		t.Errorf("after first escalation: esc_attempts = %d, want 1", card.EscAttempts)
	}
}

func TestEscalate_ReplanUsesOpusModel(t *testing.T) {
	orch, _, eng, dir := setupStuckCard(t, "esc-model", "opus", 0)
	step := makeFailedStep("opus")

	// Provide a valid replan result.
	eng.addResult(ExecutionResult{
		StepID: "replan-step-01",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: `[
			{"id":"s1","title":"A","model":"sonnet","files_modify":["internal/foo.go"],"files_create":[]}
		]`}},
	})

	_, err := orch.Escalate(context.Background(), dir, "esc-model", step, "stuck")
	if err != nil {
		t.Fatal(err)
	}

	// The replan engine call should use opus.
	call := eng.lastCall()
	if call.Model != "opus" {
		t.Errorf("replan model: got %q, want opus", call.Model)
	}
	if !strings.HasPrefix(call.StepID, "replan-") {
		t.Errorf("replan step ID should start with 'replan-', got %q", call.StepID)
	}
}

func TestModelTier(t *testing.T) {
	cases := []struct {
		model string
		tier  int
	}{
		{"haiku", 1},
		{"sonnet", 2},
		{"opus", 3},
		{"unknown", 2},
	}
	for _, tc := range cases {
		if got := modelTier(tc.model); got != tc.tier {
			t.Errorf("modelTier(%q) = %d, want %d", tc.model, got, tc.tier)
		}
	}
}

func TestNextModel(t *testing.T) {
	cases := []struct {
		current  string
		expected string
		ok       bool
	}{
		{"haiku", "sonnet", true},
		{"sonnet", "opus", true},
		{"opus", "", false},
		{"unknown", "", false},
	}
	for _, tc := range cases {
		got, ok := nextModel(tc.current)
		if ok != tc.ok || got != tc.expected {
			t.Errorf("nextModel(%q) = (%q, %v), want (%q, %v)",
				tc.current, got, ok, tc.expected, tc.ok)
		}
	}
}

func TestValidateReplanConstraints(t *testing.T) {
	original := makeFailedStep("opus")

	t.Run("valid", func(t *testing.T) {
		steps := []PlanStep{
			{FilesModify: []string{"internal/foo.go"}, FilesCreate: []string{"internal/bar.go"}},
		}
		if err := validateReplanConstraints(steps, original); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		err := validateReplanConstraints(nil, original)
		if err == nil || !strings.Contains(err.Error(), "no sub-steps") {
			t.Errorf("expected 'no sub-steps' error, got: %v", err)
		}
	})

	t.Run("too_many", func(t *testing.T) {
		steps := make([]PlanStep, 4)
		err := validateReplanConstraints(steps, original)
		if err == nil || !strings.Contains(err.Error(), "too many") {
			t.Errorf("expected 'too many' error, got: %v", err)
		}
	})

	t.Run("file_outside_scope", func(t *testing.T) {
		steps := []PlanStep{
			{FilesModify: []string{"internal/unauthorized.go"}},
		}
		err := validateReplanConstraints(steps, original)
		if err == nil || !strings.Contains(err.Error(), "outside original scope") {
			t.Errorf("expected scope violation error, got: %v", err)
		}
	})
}
