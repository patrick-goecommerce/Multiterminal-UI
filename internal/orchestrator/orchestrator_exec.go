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
		// Build context paths
		ctxPaths, _ := BuildContext(dir, step)

		// Merge verify commands: step-specific + skill-wide
		mergedVerify := mergeVerifyCommands(step.Verify, merged.Verify)

		// Build execution request
		card, err := o.board.GetTask(cardID)
		if err != nil {
			return fmt.Errorf("get card for step %s: %w", step.ID, err)
		}
		req := BuildExecutionRequest(
			step,
			cardID, card.Title, card.Description,
			merged.SkillPrompts,
			mergedVerify,
			0, // worktree slot — Phase 3
			o.stepBudget(cardID, len(wave.Steps)),
			120, // timeout seconds
		)

		// Add context paths to prompt
		req.Prompt = addContextToPrompt(req.Prompt, ctxPaths)

		// Execute
		result, err := o.engine.Execute(ctx, req)
		if err != nil {
			return fmt.Errorf("step %s failed: %w", step.ID, err)
		}

		// Record cost
		o.budget.Spend(cardID, result.CostUSD)

		// Check result
		if result.Status == StepStuck {
			return o.handleStuckStep(ctx, dir, cardID, step, "step stuck during execution")
		}

		if result.Status != StepSuccess {
			// TODO(Phase 3): QA Fix Loop
			return fmt.Errorf("step %s failed with status %s", step.ID, result.Status)
		}
	}

	return nil
}

// handleStuckStep transitions a card through the escalation pipeline.
// Tries model escalation → re-planning → human_review.
func (o *Orchestrator) handleStuckStep(ctx context.Context, dir, cardID string, failedStep PlanStep, reason string) error {
	card, err := o.board.GetTask(cardID)
	if err != nil {
		return fmt.Errorf("get card for stuck handling: %w", err)
	}

	// Transition executing → stuck
	result, err := o.sm.Transition(card, board.EventStepStuck)
	if err != nil {
		return fmt.Errorf("transition to stuck: %w", err)
	}
	card.State = result.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return fmt.Errorf("update card to stuck: %w", err)
	}

	// Use real escalation pipeline
	escResult, err := o.Escalate(ctx, dir, cardID, failedStep, reason)
	if err != nil {
		return fmt.Errorf("escalation failed: %w", err)
	}

	switch escResult.Action {
	case "model_escalated":
		// Card is back in executing with higher model — caller should retry
		return nil
	case "replanned":
		// Card is back in executing with sub-steps — caller should execute them
		return nil
	case "human_review":
		return fmt.Errorf("step %s stuck, escalated to human review: %s", failedStep.ID, escResult.Reason)
	}

	return nil
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
