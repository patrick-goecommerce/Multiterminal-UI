package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// Orchestrator coordinates the full card lifecycle: triage → plan → execute → QA → done.
type Orchestrator struct {
	board    *board.Board
	sm       *board.StateMachine
	engine   Engine
	budget   *BudgetTracker
	skillDir string
}

// NewOrchestrator creates a new orchestrator instance.
func NewOrchestrator(b *board.Board, eng Engine, skillDir string) *Orchestrator {
	return &Orchestrator{
		board:    b,
		sm:       board.NewStateMachine(),
		engine:   eng,
		budget:   NewBudgetTracker(DefaultBudgets()),
		skillDir: skillDir,
	}
}

// RunCard executes the full orchestration pipeline for a card.
// In this v3-2.8a skeleton: Triage → [Planning] → Review gate.
// Wave execution (2.8b) and QA (2.8c) will be added later.
func (o *Orchestrator) RunCard(ctx context.Context, dir, cardID string) error {
	// 1. Get card from board
	card, err := o.board.GetTask(cardID)
	if err != nil {
		return fmt.Errorf("get card: %w", err)
	}

	// 2. Transition: backlog → triage
	result, err := o.sm.Transition(card, board.EventStartTriage)
	if err != nil {
		return fmt.Errorf("transition to triage: %w", err)
	}
	card.State = result.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return fmt.Errorf("update card state: %w", err)
	}

	// 3. Detect tech stack and load skills
	detected := DetectStack(dir)
	allSkills, _ := LoadSkills(o.skillDir)
	matched := MatchSkills(detected, allSkills)
	_ = MergePolicies(matched, o.skillDir) // available for 2.8b
	techContext := buildTechContext(detected, matched)

	// 4. Assess complexity
	triage, err := AssessComplexity(ctx, o.engine, card.Title, card.Description, techContext)
	if err != nil {
		return fmt.Errorf("triage: %w", err)
	}
	card.Complexity = board.Complexity(triage.Complexity)

	// 5. Allocate budget
	o.budget.Allocate(cardID, triage.Complexity)

	// 6. Route by complexity
	if triage.Complexity == "trivial" {
		result, err = o.sm.Transition(card, board.EventComplexityTrivial)
		if err != nil {
			return fmt.Errorf("transition trivial: %w", err)
		}
		card.State = result.NewState
		if err := o.board.UpdateTask(card); err != nil {
			return fmt.Errorf("update card: %w", err)
		}
		// TODO(2.8b): Execute single step for trivial cards
		return nil
	}

	// Medium/Complex: go to planning
	result, err = o.sm.Transition(card, board.EventComplexityNonTrivial)
	if err != nil {
		return fmt.Errorf("transition to planning: %w", err)
	}
	card.State = result.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return fmt.Errorf("update card: %w", err)
	}

	// 7. Generate plan
	plan, err := GeneratePlan(ctx, o.engine, card.Title, card.Description, techContext)
	if err != nil {
		return fmt.Errorf("plan generation: %w", err)
	}
	plan.CardID = cardID
	plan.Complexity = triage.Complexity

	// 8. Save plan to board
	boardPlan := toBoardPlan(plan)
	if err := o.board.SavePlan(cardID, boardPlan); err != nil {
		return fmt.Errorf("save plan: %w", err)
	}

	// 9. Transition: planning → review
	result, err = o.sm.Transition(card, board.EventPlanReady)
	if err != nil {
		return fmt.Errorf("transition to review: %w", err)
	}
	card.State = result.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return fmt.Errorf("update card: %w", err)
	}

	// STOP: Card is now in "review" state.
	// The user must approve the plan before execution begins (2.8b).
	return nil
}

// ResumeAfterReview continues execution after user approves the plan.
// This will be implemented in 2.8b.
func (o *Orchestrator) ResumeAfterReview(ctx context.Context, dir, cardID string) error {
	return fmt.Errorf("not implemented: wave execution comes in v3-2.8b")
}

// Budget returns the orchestrator's budget tracker for external inspection.
func (o *Orchestrator) Budget() *BudgetTracker {
	return o.budget
}

// buildTechContext creates a human-readable tech context string.
func buildTechContext(detected []string, skills []Skill) string {
	var parts []string
	if len(detected) > 0 {
		parts = append(parts, "Detected: "+strings.Join(detected, ", "))
	}
	if len(skills) > 0 {
		names := make([]string, len(skills))
		for i, s := range skills {
			names[i] = s.Name
		}
		parts = append(parts, "Skills: "+strings.Join(names, ", "))
	}
	if len(parts) == 0 {
		return "No specific tech context detected"
	}
	return strings.Join(parts, ". ")
}

// toBoardPlan converts orchestrator.Plan to board.Plan for storage.
func toBoardPlan(p Plan) board.Plan {
	steps := make([]board.PlanStep, len(p.Steps))
	for i, s := range p.Steps {
		steps[i] = board.PlanStep{
			ID:          s.ID,
			Title:       s.Title,
			Wave:        s.Wave,
			DependsOn:   s.DependsOn,
			ParallelOk:  s.ParallelOk,
			Model:       s.Model,
			FilesModify: s.FilesModify,
			FilesCreate: s.FilesCreate,
			Status:      s.Status,
		}
	}
	return board.Plan{
		CardID:     p.CardID,
		Complexity: board.Complexity(p.Complexity),
		Steps:      steps,
	}
}
