package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// executeWave runs all steps in a wave sequentially.
// Phase 2: steps execute sequentially even within a wave (no real parallelism without worktrees).
// Phase 3 will add parallel execution with worktree isolation.
func (o *Orchestrator) executeWave(ctx context.Context, dir, cardID string, wave Wave, merged MergedPolicy) error {
	for _, step := range wave.Steps {
		if err := o.executeStep(ctx, dir, cardID, step, len(wave.Steps), merged); err != nil {
			return err
		}
	}
	return nil
}

// executeStep runs a single step and drives it through the escalation pipeline
// when it gets stuck. A model-escalated step is retried with the higher model;
// only a successful (or terminally escalated) step ends the loop.
func (o *Orchestrator) executeStep(ctx context.Context, dir, cardID string, step PlanStep, stepsInWave int, merged MergedPolicy) error {
	for {
		result, err := o.runStepOnce(ctx, dir, cardID, step, stepsInWave, merged)
		if err != nil {
			return err
		}

		// Record cost
		o.budget.Spend(cardID, result.CostUSD)

		switch result.Status {
		case StepSuccess:
			return nil

		case StepStuck:
			esc, err := o.handleStuckStep(ctx, dir, cardID, step, "step stuck during execution")
			if err != nil {
				return err
			}
			switch esc.Action {
			case "model_escalated":
				step.Model = esc.NewModel
				continue // retry the same step with the escalated model
			case "replanned":
				return o.executeSubSteps(ctx, dir, cardID, esc.SubSteps, merged)
			default:
				return nil
			}

		default:
			// TODO(Bug D): QA Fix Loop for a failed verify before giving up.
			return fmt.Errorf("step %s failed with status %s", step.ID, result.Status)
		}
	}
}

// executeSubSteps runs the sub-steps produced by a re-planning escalation.
// Each sub-step goes through the same execution path (including its own
// escalation), so a stuck sub-step is recovered like any other step.
func (o *Orchestrator) executeSubSteps(ctx context.Context, dir, cardID string, subSteps []PlanStep, merged MergedPolicy) error {
	for _, sub := range subSteps {
		if err := o.executeStep(ctx, dir, cardID, sub, len(subSteps), merged); err != nil {
			return err
		}
	}
	return nil
}

// runStepOnce builds the execution request for a step and runs it exactly once.
func (o *Orchestrator) runStepOnce(ctx context.Context, dir, cardID string, step PlanStep, stepsInWave int, merged MergedPolicy) (ExecutionResult, error) {
	// Build context paths
	ctxPaths, _ := BuildContext(dir, step)

	// Merge verify commands: step-specific + skill-wide
	mergedVerify := mergeVerifyCommands(step.Verify, merged.Verify)

	// Build execution request
	card, err := o.board.GetTask(cardID)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("get card for step %s: %w", step.ID, err)
	}
	req := BuildExecutionRequest(
		step,
		cardID, card.Title, card.Description,
		merged.SkillPrompts,
		mergedVerify,
		0, // worktree slot — Phase 3
		o.stepBudget(cardID, stepsInWave),
		120, // timeout seconds
	)

	// Add context paths to prompt
	req.Prompt = addContextToPrompt(req.Prompt, ctxPaths)

	result, err := o.engine.Execute(ctx, req)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("step %s failed: %w", step.ID, err)
	}
	return result, nil
}

// handleStuckStep transitions a card into the escalation pipeline and returns
// the escalation outcome so the caller can act on it (retry with a higher
// model, execute sub-steps, or stop). A terminal escalation to human_review is
// returned both as the result and as an error.
func (o *Orchestrator) handleStuckStep(ctx context.Context, dir, cardID string, failedStep PlanStep, reason string) (EscalationResult, error) {
	card, err := o.board.GetTask(cardID)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("get card for stuck handling: %w", err)
	}

	// Transition executing → stuck
	result, err := o.sm.Transition(card, board.EventStepStuck)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("transition to stuck: %w", err)
	}
	card.State = result.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return EscalationResult{}, fmt.Errorf("update card to stuck: %w", err)
	}

	// Use real escalation pipeline
	escResult, err := o.Escalate(ctx, dir, cardID, failedStep, reason)
	if err != nil {
		return EscalationResult{}, fmt.Errorf("escalation failed: %w", err)
	}

	if escResult.Action == "human_review" {
		return escResult, fmt.Errorf("step %s stuck, escalated to human review: %s", failedStep.ID, escResult.Reason)
	}
	return escResult, nil
}

// stepBudget calculates per-step budget from remaining card budget.
func (o *Orchestrator) stepBudget(cardID string, stepsInWave int) float64 {
	remaining, err := o.budget.Remaining(cardID)
	if err != nil {
		return 0
	}
	if stepsInWave <= 0 {
		stepsInWave = 1
	}
	return remaining / float64(stepsInWave)
}

// mergeVerifyCommands combines step-specific and skill-wide verify, deduplicating.
func mergeVerifyCommands(stepVerify, skillVerify []VerifyStep) []VerifyStep {
	seen := map[string]bool{}
	var result []VerifyStep
	for _, v := range stepVerify {
		if !seen[v.Command] {
			seen[v.Command] = true
			result = append(result, v)
		}
	}
	for _, v := range skillVerify {
		if !seen[v.Command] {
			seen[v.Command] = true
			result = append(result, v)
		}
	}
	return result
}

// addContextToPrompt appends file path references to a step prompt.
func addContextToPrompt(prompt string, paths ContextPaths) string {
	var b strings.Builder
	b.WriteString(prompt)

	if len(paths.DirectFiles) > 0 {
		b.WriteString("\n### Read these files first\n")
		for _, f := range paths.DirectFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}

	if len(paths.NeighborFiles) > 0 {
		b.WriteString("\n### Neighbor files (for context)\n")
		for _, f := range paths.NeighborFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}

	if len(paths.TestFiles) > 0 {
		b.WriteString("\n### Test files\n")
		for _, f := range paths.TestFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}

	return b.String()
}
