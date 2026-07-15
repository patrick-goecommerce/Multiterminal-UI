package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Note: testEngine is defined in triage_test.go (same package).

const validPlanJSON = `{
	"card_id": "feat-auth",
	"complexity": "medium",
	"steps": [
		{
			"id": "01",
			"title": "Create auth endpoint",
			"wave": 1,
			"depends_on": [],
			"parallel_ok": true,
			"model": "sonnet",
			"files_modify": ["internal/backend/app_auth.go"],
			"files_create": ["internal/backend/app_auth_test.go"],
			"must_haves": {"truths": ["Auth endpoint returns 200"], "artifacts": [{"path": "internal/backend/app_auth.go", "min_lines": 30}]},
			"verify": [{"command": "go test ./...", "description": "tests pass"}],
			"status": "pending"
		}
	]
}`

func planResult(output string) ExecutionResult {
	return ExecutionResult{
		StepID: "plan",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: output}},
	}
}

func TestGeneratePlan_ValidPlan(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(planResult(validPlanJSON))

	plan, err := GeneratePlan(context.Background(), eng, "Auth feature", "Add auth endpoint", "go project")
	if err != nil {
		t.Fatal(err)
	}
	if plan.CardID != "feat-auth" {
		t.Errorf("got card_id %q, want feat-auth", plan.CardID)
	}
	if plan.Complexity != "medium" {
		t.Errorf("got complexity %q, want medium", plan.Complexity)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(plan.Steps))
	}
	if plan.Steps[0].ID != "01" {
		t.Errorf("got step id %q, want 01", plan.Steps[0].ID)
	}
	if plan.Steps[0].Model != "sonnet" {
		t.Errorf("got step model %q, want sonnet", plan.Steps[0].Model)
	}
}

func TestGeneratePlan_InvalidJSON_RepairSucceeds(t *testing.T) {
	eng := newTestEngine()
	// First call returns invalid JSON
	eng.addResult(planResult(`{"card_id": "test", not valid json`))
	// Repair call returns valid JSON
	eng.addResult(planResult(validPlanJSON))

	plan, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}
	if plan.CardID != "feat-auth" {
		t.Errorf("got card_id %q, want feat-auth", plan.CardID)
	}
	if eng.callCount() != 2 {
		t.Errorf("expected 2 calls (plan + 1 repair), got %d", eng.callCount())
	}
}

func TestGeneratePlan_InvalidJSON_RepairFails3Times(t *testing.T) {
	eng := newTestEngine()
	// Initial plan returns invalid JSON
	eng.addResult(planResult(`{bad json`))
	// 3 repair attempts all return invalid JSON
	eng.addResult(planResult(`{still bad`))
	eng.addResult(planResult(`{nope`))
	eng.addResult(planResult(`{again`))

	_, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err == nil {
		t.Fatal("expected error after 3 failed repairs")
	}
	if !strings.Contains(err.Error(), "3 repair attempts") {
		t.Errorf("error should mention 3 repair attempts, got: %s", err.Error())
	}
	if eng.callCount() != 4 {
		t.Errorf("expected 4 calls (1 plan + 3 repairs), got %d", eng.callCount())
	}
}

func TestGeneratePlan_EmptyOutput(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(ExecutionResult{
		StepID: "plan",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: ""}},
	})

	_, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err == nil {
		t.Fatal("expected error for empty output")
	}
	if !strings.Contains(err.Error(), "empty output") {
		t.Errorf("error should mention empty output, got: %s", err.Error())
	}
}

func TestGeneratePlan_EngineError(t *testing.T) {
	eng := newTestEngine()
	eng.setError("plan", errors.New("connection refused"))

	_, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err == nil {
		t.Fatal("expected error on engine failure")
	}
	if !strings.Contains(err.Error(), "plan generation failed") {
		t.Errorf("error should mention plan generation failed, got: %s", err.Error())
	}
}

func TestGeneratePlan_MissingCardID_RepairSucceeds(t *testing.T) {
	eng := newTestEngine()
	// Valid JSON but missing card_id
	eng.addResult(planResult(`{"complexity":"medium","steps":[{"id":"01","title":"Do thing","model":"sonnet"}]}`))
	// Repair returns valid plan
	eng.addResult(planResult(validPlanJSON))

	plan, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}
	if plan.CardID != "feat-auth" {
		t.Errorf("got card_id %q, want feat-auth", plan.CardID)
	}
}

func TestGeneratePlan_MissingSteps(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(planResult(`{"card_id":"test","complexity":"medium","steps":[]}`))
	// 3 repair attempts all return empty steps
	eng.addResult(planResult(`{"card_id":"test","steps":[]}`))
	eng.addResult(planResult(`{"card_id":"test","steps":[]}`))
	eng.addResult(planResult(`{"card_id":"test","steps":[]}`))

	_, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err == nil {
		t.Fatal("expected error for missing steps")
	}
	if !strings.Contains(err.Error(), "no steps") {
		t.Errorf("error should mention no steps, got: %s", err.Error())
	}
}

func TestGeneratePlan_MissingStepFields(t *testing.T) {
	eng := newTestEngine()
	// Step missing title
	eng.addResult(planResult(`{"card_id":"test","steps":[{"id":"01","model":"sonnet"}]}`))
	// Repair returns valid plan
	eng.addResult(planResult(validPlanJSON))

	plan, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Title != "Create auth endpoint" {
		t.Errorf("got title %q, want 'Create auth endpoint'", plan.Steps[0].Title)
	}
}

func TestGeneratePlan_UsesOpusModel(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(planResult(validPlanJSON))

	_, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}

	calls := eng.calls
	if len(calls) < 1 {
		t.Fatal("expected at least 1 call")
	}
	if calls[0].Model != "opus" {
		t.Errorf("plan generation model: got %q, want opus", calls[0].Model)
	}
	if calls[0].StepID != "plan" {
		t.Errorf("plan generation stepID: got %q, want plan", calls[0].StepID)
	}
}

func TestGeneratePlan_RepairUsesHaikuModel(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(planResult(`{invalid`))
	eng.addResult(planResult(validPlanJSON))

	_, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}

	calls := eng.calls
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", len(calls))
	}
	if calls[1].Model != "haiku" {
		t.Errorf("repair model: got %q, want haiku", calls[1].Model)
	}
	if calls[1].StepID != "plan-repair-1" {
		t.Errorf("repair stepID: got %q, want plan-repair-1", calls[1].StepID)
	}
}

func TestGeneratePlan_RepairEngineError_ContinuesRetrying(t *testing.T) {
	eng := newTestEngine()
	// Initial plan returns invalid JSON
	eng.addResult(planResult(`{bad`))
	// First repair fails with engine error
	eng.setError("plan-repair-1", errors.New("timeout"))
	// Second repair succeeds
	eng.addResult(planResult(validPlanJSON))

	plan, err := GeneratePlan(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}
	if plan.CardID != "feat-auth" {
		t.Errorf("got card_id %q, want feat-auth", plan.CardID)
	}
}

func TestParsePlan_MarkdownFences(t *testing.T) {
	fenced := "```json\n" + validPlanJSON + "\n```"
	plan, err := parsePlan(fenced)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CardID != "feat-auth" {
		t.Errorf("got card_id %q, want feat-auth", plan.CardID)
	}
}

func TestValidatePlan_MissingStepID(t *testing.T) {
	plan := Plan{
		CardID: "test",
		Steps:  []PlanStep{{Title: "step", Model: "sonnet"}},
	}
	err := validatePlan(plan)
	if err == nil || !strings.Contains(err.Error(), "missing id") {
		t.Errorf("expected 'missing id' error, got: %v", err)
	}
}

func TestValidatePlan_MissingStepTitle(t *testing.T) {
	plan := Plan{
		CardID: "test",
		Steps:  []PlanStep{{ID: "01", Model: "sonnet"}},
	}
	err := validatePlan(plan)
	if err == nil || !strings.Contains(err.Error(), "missing title") {
		t.Errorf("expected 'missing title' error, got: %v", err)
	}
}

func TestValidatePlan_MissingStepModel(t *testing.T) {
	plan := Plan{
		CardID: "test",
		Steps:  []PlanStep{{ID: "01", Title: "step"}},
	}
	err := validatePlan(plan)
	if err == nil || !strings.Contains(err.Error(), "missing model") {
		t.Errorf("expected 'missing model' error, got: %v", err)
	}
}
