package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// executeWave runs a wave's steps in two phases:
//
//	Phase 1 (parallel): each step's first attempt runs concurrently in its own
//	  isolated worktree. This phase only reads board state, so it is race-free.
//	Phase 2 (sequential): results are processed one at a time — cost accounting,
//	  the QA fix loop, escalation and retries all mutate shared card state, so
//	  they must be serialized.
//
// ComputeWaves guarantees the steps in a wave are independent (no cross-deps,
// no files_modify overlap), so parallel first attempts cannot conflict.
func (o *Orchestrator) executeWave(ctx context.Context, dir, cardID string, wave Wave, merged MergedPolicy) error {
	o.emitEvent(EventWaveStarted, map[string]string{
		"card_id": cardID,
		"wave":    fmt.Sprintf("%d", wave.Number),
		"steps":   fmt.Sprintf("%d", len(wave.Steps)),
	})

	steps := wave.Steps
	results := make([]ExecutionResult, len(steps))
	errs := make([]error, len(steps))

	// Phase 1: parallel first attempts.
	var wg sync.WaitGroup
	for i := range steps {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = o.runStepOnce(ctx, dir, cardID, steps[i], len(steps), merged)
		}(i)
	}
	wg.Wait()

	// Phase 2: sequential result processing and recovery.
	for i, step := range steps {
		if errs[i] != nil {
			return errs[i]
		}
		if err := o.driveStep(ctx, dir, cardID, step, results[i], len(steps), merged); err != nil {
			return err
		}
		o.emitEvent(EventStepDone, map[string]string{"card_id": cardID, "step": step.ID})
	}
	return nil
}

// executeStep runs a step from scratch and drives it to completion. Used for
// re-planned sub-steps, which are executed sequentially.
func (o *Orchestrator) executeStep(ctx context.Context, dir, cardID string, step PlanStep, stepsInWave int, merged MergedPolicy) error {
	result, err := o.runStepOnce(ctx, dir, cardID, step, stepsInWave, merged)
	if err != nil {
		return err
	}
	return o.driveStep(ctx, dir, cardID, step, result, stepsInWave, merged)
}

// driveStep processes an execution result and recovers a stuck or failed step
// through the QA fix loop and escalation pipeline, retrying the (possibly
// model-bumped) step until it succeeds or escalation is terminal. It mutates
// shared card state and must only be called sequentially.
func (o *Orchestrator) driveStep(ctx context.Context, dir, cardID string, step PlanStep, result ExecutionResult, stepsInWave int, merged MergedPolicy) error {
	for {
		// Record cost
		o.budget.Spend(cardID, result.CostUSD)

		switch result.Status {
		case StepSuccess:
			return nil

		case StepStuck:
			next, retry, err := o.escalateAndDecide(ctx, dir, cardID, step, "step stuck during execution", merged)
			if err != nil {
				return err
			}
			if !retry {
				return nil
			}
			step = next

		case StepFailed:
			// A failed verify gets a Sonnet fix loop first; only an exhausted
			// fix loop is treated as stuck and routed through escalation.
			if o.qaFixStep(ctx, dir, cardID, step, result, stepsInWave, merged) {
				return nil
			}
			next, retry, err := o.escalateAndDecide(ctx, dir, cardID, step, "step verify failed after fix attempts", merged)
			if err != nil {
				return err
			}
			if !retry {
				return nil
			}
			step = next

		default:
			return fmt.Errorf("step %s failed with status %s", step.ID, result.Status)
		}

		// Retry the (possibly model-bumped) step.
		var err error
		result, err = o.runStepOnce(ctx, dir, cardID, step, stepsInWave, merged)
		if err != nil {
			return err
		}
	}
}

// maxStepFixAttempts bounds the per-step QA fix loop before a step is
// treated as stuck (spec §6.6).
const maxStepFixAttempts = 3

// qaFixStep attempts to repair a step whose verify failed, using a Sonnet fix
// agent for up to maxStepFixAttempts iterations. It returns true as soon as a
// fix attempt produces a step that passes verification.
func (o *Orchestrator) qaFixStep(ctx context.Context, dir, cardID string, step PlanStep, failed ExecutionResult, stepsInWave int, merged MergedPolicy) bool {
	mergedVerify := mergeVerifyCommands(step.Verify, merged.Verify)
	failures := summarizeVerifyFailures(failed.Verify)

	for attempt := 1; attempt <= maxStepFixAttempts; attempt++ {
		card, err := o.board.GetTask(cardID)
		if err != nil {
			return false
		}
		req := ExecutionRequest{
			StepID:       fmt.Sprintf("%s-fix-%d", step.ID, attempt),
			CardID:       cardID,
			WorktreeSlot: 0,
			Prompt:       buildStepFixPrompt(step, card.Title, card.Description, failures),
			SystemPrompt: BuildSystemPrompt(merged.SkillPrompts),
			Model:        "sonnet",
			Verify:       mergedVerify,
			BudgetUSD:    o.stepBudget(cardID, stepsInWave),
			TimeoutSec:   120,
			SkillPrompts: merged.SkillPrompts,
		}

		result, err := o.engine.Execute(ctx, req)
		if err != nil {
			return false
		}
		o.budget.Spend(cardID, result.CostUSD)

		if result.Status == StepSuccess {
			return true
		}
		failures = summarizeVerifyFailures(result.Verify)
	}
	return false
}

// summarizeVerifyFailures renders the failed verify results into a prompt-ready list.
func summarizeVerifyFailures(verify []VerifyResult) string {
	var parts []string
	for _, v := range verify {
		if !v.Passed {
			parts = append(parts, fmt.Sprintf("- `%s` (exit %d): %s", v.Command, v.ExitCode, v.Output))
		}
	}
	return strings.Join(parts, "\n")
}

// buildStepFixPrompt builds the fix-agent prompt: the original task plus the
// concrete verification failures to repair, scoped to the original files.
func buildStepFixPrompt(step PlanStep, cardTitle, cardDescription, failures string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "A previous attempt at this step failed verification. Fix ONLY the issues below.\n\n")
	fmt.Fprintf(&b, "## Original task: %s\n", step.Title)
	fmt.Fprintf(&b, "Card: %s\n", cardTitle)
	if cardDescription != "" {
		fmt.Fprintf(&b, "Description: %s\n", cardDescription)
	}
	if len(step.FilesModify) > 0 {
		fmt.Fprintf(&b, "\nFiles in scope (modify): %s\n", strings.Join(step.FilesModify, ", "))
	}
	if len(step.FilesCreate) > 0 {
		fmt.Fprintf(&b, "Files in scope (create): %s\n", strings.Join(step.FilesCreate, ", "))
	}
	fmt.Fprintf(&b, "\n## Verification failures\n%s\n", failures)
	b.WriteString("\nFix these failures. Do not modify files outside the listed scope.")
	return b.String()
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

// escalateAndDecide drives a stuck or fix-exhausted step through the escalation
// pipeline and reports how the caller should proceed: retry the (possibly
// model-bumped) step, or stop — either because sub-steps were executed
// (re-planning) or because escalation hit a terminal human_review (err).
func (o *Orchestrator) escalateAndDecide(ctx context.Context, dir, cardID string, step PlanStep, reason string, merged MergedPolicy) (next PlanStep, retry bool, err error) {
	esc, err := o.handleStuckStep(ctx, dir, cardID, step, reason)
	if err != nil {
		return step, false, err
	}
	switch esc.Action {
	case "model_escalated":
		step.Model = esc.NewModel
		return step, true, nil
	case "replanned":
		return step, false, o.executeSubSteps(ctx, dir, cardID, esc.SubSteps, merged)
	default:
		return step, false, nil
	}
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
