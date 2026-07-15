package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// EscalationResult describes the outcome of an escalation attempt.
type EscalationResult struct {
	Action   string     // "model_escalated" | "replanned" | "human_review"
	Reason   string
	NewModel string     // set if Action == "model_escalated"
	SubSteps []PlanStep // set if Action == "replanned"
}

// modelTier returns the priority of a model (higher = more capable).
func modelTier(model string) int {
	switch model {
	case "haiku":
		return 1
	case "sonnet":
		return 2
	case "opus":
		return 3
	default:
		return 2
	}
}

// nextModel returns the next higher model tier, if one exists.
func nextModel(current string) (string, bool) {
	switch current {
	case "haiku":
		return "sonnet", true
	case "sonnet":
		return "opus", true
	default:
		return "", false
	}
}

// Escalate attempts to recover a stuck step through progressively stronger
// measures: 1. Model escalation  2. Re-planning  3. human_review.
//
// card.EscAttempts tracks total escalation attempts (max 2 before human_review).
func (o *Orchestrator) Escalate(ctx context.Context, dir, cardID string, failedStep PlanStep, failureReason string) (EscalationResult, error) {
	card, err := o.board.GetTask(cardID)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("get card: %w", err)
	}

	if card.State != board.StateStuck {
		return EscalationResult{}, fmt.Errorf("card %s in state %s, expected stuck", cardID, card.State)
	}

	// Max escalations reached -> human_review immediately.
	if card.EscAttempts >= 2 {
		return o.escalateToHumanReview(card, board.EventMaxEscalations,
			fmt.Sprintf("max escalations reached (%d attempts)", card.EscAttempts),
			"max_escalations_reached",
		)
	}

	// Attempt 1: Model escalation (if a higher model exists).
	if nextMdl, ok := nextModel(failedStep.Model); ok {
		card.EscAttempts++
		if err := o.board.UpdateTask(card); err != nil {
			return EscalationResult{}, fmt.Errorf("update esc_attempts: %w", err)
		}

		result, err := o.sm.Transition(card, board.EventModelEscalated)
		if err != nil {
			return EscalationResult{}, fmt.Errorf("transition model_escalated: %w", err)
		}
		card.State = result.NewState
		if err := o.board.UpdateTask(card); err != nil {
			return EscalationResult{}, fmt.Errorf("update card state: %w", err)
		}

		return EscalationResult{
			Action:   "model_escalated",
			Reason:   fmt.Sprintf("escalated from %s to %s", failedStep.Model, nextMdl),
			NewModel: nextMdl,
		}, nil
	}

	// Attempt 2: Re-planning (model is already opus, can't escalate further).
	replanRes, err := o.attemptReplan(ctx, dir, cardID, failedStep, failureReason)
	if err != nil {
		// Re-planning failed -> scope_expansion_required -> human_review.
		return o.escalateToHumanReview(card, board.EventScopeExpansion,
			fmt.Sprintf("re-planning failed: %v", err),
			"scope_expansion_required",
		)
	}

	card.EscAttempts++
	if err := o.board.UpdateTask(card); err != nil {
		return EscalationResult{}, fmt.Errorf("update esc_attempts: %w", err)
	}

	result, err := o.sm.Transition(card, board.EventReplanCompleted)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("transition replan_completed: %w", err)
	}
	card.State = result.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return EscalationResult{}, fmt.Errorf("update card state: %w", err)
	}

	return replanRes, nil
}

// escalateToHumanReview transitions the card to human_review via the given event.
func (o *Orchestrator) escalateToHumanReview(card board.TaskCard, event board.Event, reason, reviewReason string) (EscalationResult, error) {
	result, err := o.sm.Transition(card, event)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("transition to human_review: %w", err)
	}
	card.State = result.NewState
	card.ReviewReason = reviewReason
	if err := o.board.UpdateTask(card); err != nil {
		return EscalationResult{}, fmt.Errorf("update card: %w", err)
	}
	return EscalationResult{
		Action: "human_review",
		Reason: reason,
	}, nil
}

// attemptReplan tries to break the failed step into smaller sub-steps.
// Constraints: max 3 sub-steps, only files from original scope.
func (o *Orchestrator) attemptReplan(ctx context.Context, dir, cardID string, failedStep PlanStep, failureReason string) (EscalationResult, error) {
	prompt := buildReplanPrompt(failedStep, failureReason)

	req := ExecutionRequest{
		StepID:     "replan-" + failedStep.ID,
		CardID:     cardID,
		Prompt:     prompt,
		Model:      "opus",
		BudgetUSD:  0.10,
		TimeoutSec: 60,
	}

	result, err := o.engine.Execute(ctx, req)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("replan execution failed: %w", err)
	}

	subSteps, err := parseReplanResult(result, failedStep)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("replan parsing failed: %w", err)
	}

	if err := validateReplanConstraints(subSteps, failedStep); err != nil {
		return EscalationResult{}, fmt.Errorf("replan violates constraints: %w", err)
	}

	return EscalationResult{
		Action:   "replanned",
		Reason:   fmt.Sprintf("split step %s into %d sub-steps", failedStep.ID, len(subSteps)),
		SubSteps: subSteps,
	}, nil
}

// buildReplanPrompt creates the prompt for the re-planning agent.
func buildReplanPrompt(step PlanStep, reason string) string {
	return fmt.Sprintf(`A step in the execution plan has failed and needs to be broken into smaller sub-steps.

Failed step: %s (ID: %s)
Failure reason: %s
Model used: %s
Files to modify: %v
Files to create: %v

Break this step into 2-3 smaller sub-steps. Constraints:
- Maximum 3 sub-steps
- Only use files from the original step scope (files_modify + files_create listed above)
- Do NOT add files outside the original scope
- Preserve the same must_haves as the original step

Respond with ONLY valid JSON array of steps:
[{"id":"sub-1","title":"...","model":"sonnet","files_modify":[...],"files_create":[...]}]`,
		step.Title, step.ID, reason, step.Model, step.FilesModify, step.FilesCreate)
}

// parseReplanResult extracts sub-steps from the engine result.
// Convention: raw model output is in VerifyResult.Output (Phase 2 convention).
func parseReplanResult(result ExecutionResult, original PlanStep) ([]PlanStep, error) {
	for _, v := range result.Verify {
		if v.Output == "" {
			continue
		}
		steps, err := parseSubStepsJSON(v.Output, original)
		if err != nil {
			continue
		}
		return steps, nil
	}
	return nil, fmt.Errorf("no parseable sub-steps in result")
}

// parseSubStepsJSON parses a JSON array of sub-step objects.
func parseSubStepsJSON(raw string, original PlanStep) ([]PlanStep, error) {
	// Minimal sub-step shape from the engine.
	type subStep struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Model       string   `json:"model"`
		FilesModify []string `json:"files_modify"`
		FilesCreate []string `json:"files_create"`
	}

	var subs []subStep
	if err := json.Unmarshal([]byte(raw), &subs); err != nil {
		return nil, err
	}

	out := make([]PlanStep, len(subs))
	for i, s := range subs {
		out[i] = PlanStep{
			ID:          s.ID,
			Title:       s.Title,
			Wave:        original.Wave,
			DependsOn:   original.DependsOn,
			ParallelOk:  false,
			Model:       s.Model,
			FilesModify: s.FilesModify,
			FilesCreate: s.FilesCreate,
			MustHaves:   original.MustHaves,
			Status:      "pending",
		}
	}
	return out, nil
}

// validateReplanConstraints checks that sub-steps respect scope limits.
func validateReplanConstraints(subSteps []PlanStep, original PlanStep) error {
	if len(subSteps) > 3 {
		return fmt.Errorf("too many sub-steps: %d (max 3)", len(subSteps))
	}
	if len(subSteps) == 0 {
		return fmt.Errorf("no sub-steps generated")
	}

	allowed := make(map[string]bool)
	for _, f := range original.FilesModify {
		allowed[f] = true
	}
	for _, f := range original.FilesCreate {
		allowed[f] = true
	}

	for _, s := range subSteps {
		for _, f := range s.FilesModify {
			if !allowed[f] {
				return fmt.Errorf("sub-step references file %s outside original scope", f)
			}
		}
		for _, f := range s.FilesCreate {
			if !allowed[f] {
				return fmt.Errorf("sub-step creates file %s outside original scope", f)
			}
		}
	}

	return nil
}
