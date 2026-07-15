package board

import (
	"errors"
	"strings"
	"testing"
)

func TestSaveAndGetPlan(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	plan := Plan{
		CardID:     "plan-1",
		Complexity: ComplexityComplex,
		Steps: []PlanStep{
			{
				ID:          "s1",
				Title:       "Create schema",
				Wave:        1,
				ParallelOk:  true,
				Model:       "opus",
				FilesModify: []string{"db/schema.sql"},
				FilesCreate: []string{"db/migrations/001.sql"},
				Status:      "pending",
			},
			{
				ID:        "s2",
				Title:     "Write handler",
				Wave:      2,
				DependsOn: []string{"s1"},
				Model:     "sonnet",
				Status:    "pending",
			},
		},
	}

	if err := b.SavePlan("plan-1", plan); err != nil {
		t.Fatalf("SavePlan: %v", err)
	}

	got, err := b.GetPlan("plan-1")
	if err != nil {
		t.Fatalf("GetPlan: %v", err)
	}
	if got.CardID != "plan-1" {
		t.Errorf("CardID: got %q, want %q", got.CardID, "plan-1")
	}
	if got.Complexity != ComplexityComplex {
		t.Errorf("Complexity: got %q, want %q", got.Complexity, ComplexityComplex)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(got.Steps))
	}
	if got.Steps[0].Title != "Create schema" {
		t.Errorf("Step[0].Title: got %q", got.Steps[0].Title)
	}
	if got.Steps[1].DependsOn[0] != "s1" {
		t.Errorf("Step[1].DependsOn: got %v", got.Steps[1].DependsOn)
	}
}

func TestSavePlanTooLarge(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	// Create a plan with a huge step title to exceed 50KB.
	bigTitle := strings.Repeat("X", 60*1024)
	plan := Plan{
		CardID: "big-plan",
		Steps: []PlanStep{
			{ID: "s1", Title: bigTitle, Status: "pending"},
		},
	}

	err := b.SavePlan("big-plan", plan)
	if err == nil {
		t.Fatal("expected ErrPlanTooLarge, got nil")
	}
	if !errors.Is(err, ErrPlanTooLarge) {
		t.Errorf("expected ErrPlanTooLarge, got: %v", err)
	}
}

func TestGetPlanNotFound(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	_, err := b.GetPlan("no-such-plan")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("expected ErrRefNotFound, got: %v", err)
	}
}
