package orchestrator

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_NoSkills(t *testing.T) {
	got := BuildSystemPrompt(nil)
	if got != baseSystemPrompt {
		t.Errorf("expected base prompt only, got:\n%s", got)
	}
}

func TestBuildSystemPrompt_WithSkills(t *testing.T) {
	skills := []string{"Skill A instructions", "Skill B instructions"}
	got := BuildSystemPrompt(skills)

	if !strings.HasPrefix(got, baseSystemPrompt) {
		t.Error("should start with base system prompt")
	}
	if !strings.Contains(got, "---") {
		t.Error("sections should be separated by ---")
	}
	if !strings.Contains(got, "Skill A instructions") {
		t.Error("should contain skill A")
	}
	if !strings.Contains(got, "Skill B instructions") {
		t.Error("should contain skill B")
	}

	// Verify order: base, then A, then B
	idxA := strings.Index(got, "Skill A")
	idxB := strings.Index(got, "Skill B")
	if idxA >= idxB {
		t.Error("skills should appear in order")
	}
}

func TestBuildSystemPrompt_SkipsEmptySkills(t *testing.T) {
	skills := []string{"Valid skill", "", "  ", "\t\n"}
	got := BuildSystemPrompt(skills)

	parts := strings.Split(got, "\n\n---\n\n")
	if len(parts) != 2 {
		t.Errorf("expected 2 sections (base + 1 valid skill), got %d", len(parts))
	}
	if !strings.Contains(got, "Valid skill") {
		t.Error("should contain the valid skill")
	}
}

func TestBuildStepPrompt_AllSections(t *testing.T) {
	step := PlanStep{
		ID:          "step-1",
		Title:       "Implement widget",
		FilesModify: []string{"internal/widget.go"},
		FilesCreate: []string{"internal/widget_test.go"},
		MustHaves: MustHaves{
			Truths: []string{"Widget struct exists", "Tests pass"},
		},
		Verify: []VerifyStep{
			{Command: "go test ./...", Description: "all tests pass"},
		},
	}

	got := BuildStepPrompt(step, "Add Widget Feature", "Build a reusable widget component")

	checks := []string{
		"## Task: Implement widget",
		"Card: Add Widget Feature",
		"Description: Build a reusable widget component",
		"### Files to modify",
		"`internal/widget.go`",
		"### Files to create",
		"`internal/widget_test.go`",
		"### Must-haves",
		"Widget struct exists",
		"Tests pass",
		"### Verification",
		"`go test ./...` — all tests pass",
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("prompt missing expected content: %q", c)
		}
	}
}

func TestBuildStepPrompt_MinimalStep(t *testing.T) {
	step := PlanStep{
		ID:    "step-min",
		Title: "Simple task",
	}

	got := BuildStepPrompt(step, "Card Title", "")

	if !strings.Contains(got, "## Task: Simple task") {
		t.Error("should contain task title")
	}
	if !strings.Contains(got, "Card: Card Title") {
		t.Error("should contain card title")
	}

	// Should NOT contain optional sections
	for _, section := range []string{"### Files to modify", "### Files to create", "### Must-haves", "### Verification"} {
		if strings.Contains(got, section) {
			t.Errorf("minimal prompt should not contain %q", section)
		}
	}

	// No description line when empty
	if strings.Contains(got, "Description:") {
		t.Error("should not render Description when empty")
	}
}

func TestBuildExecutionRequest_WiresEverything(t *testing.T) {
	step := PlanStep{
		ID:    "s-1",
		Title: "Do stuff",
		Model: "claude-sonnet-4-5-20250929",
	}
	skills := []string{"Backend Specialist"}
	verify := []VerifyStep{{Command: "go vet ./...", Description: "vet passes"}}

	req := BuildExecutionRequest(step, "card-42", "The Card", "Card desc", skills, verify, 3, 0.50, 300)

	if req.StepID != "s-1" {
		t.Errorf("StepID = %q, want %q", req.StepID, "s-1")
	}
	if req.CardID != "card-42" {
		t.Errorf("CardID = %q, want %q", req.CardID, "card-42")
	}
	if req.WorktreeSlot != 3 {
		t.Errorf("WorktreeSlot = %d, want 3", req.WorktreeSlot)
	}
	if req.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("Model = %q, want step model", req.Model)
	}
	if req.BudgetUSD != 0.50 {
		t.Errorf("BudgetUSD = %f, want 0.50", req.BudgetUSD)
	}
	if req.TimeoutSec != 300 {
		t.Errorf("TimeoutSec = %d, want 300", req.TimeoutSec)
	}
	if len(req.Verify) != 1 || req.Verify[0].Command != "go vet ./..." {
		t.Error("Verify not wired correctly")
	}
	if len(req.SkillPrompts) != 1 || req.SkillPrompts[0] != "Backend Specialist" {
		t.Error("SkillPrompts not wired correctly")
	}
	if !strings.Contains(req.Prompt, "Do stuff") {
		t.Error("Prompt should contain step title")
	}
	if !strings.Contains(req.SystemPrompt, baseSystemPrompt) {
		t.Error("SystemPrompt should contain base prompt")
	}
	if !strings.Contains(req.SystemPrompt, "Backend Specialist") {
		t.Error("SystemPrompt should contain skill prompt")
	}
}

func TestBuildExecutionRequest_UsesStepModel(t *testing.T) {
	step := PlanStep{
		ID:    "s-2",
		Title: "Model test",
		Model: "claude-opus-4-6",
	}

	req := BuildExecutionRequest(step, "c-1", "Card", "", nil, nil, 0, 1.0, 600)

	if req.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q (from step)", req.Model, "claude-opus-4-6")
	}
}
