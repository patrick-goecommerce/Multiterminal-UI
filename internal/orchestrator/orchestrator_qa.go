package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// DecisionBriefingStub is a simplified decision briefing.
// The full engine.BuildBriefing (secrets scanner, conflict risk, dependency risk)
// is called from the Wails binding layer (app_orchestrate_v3.go) which can import
// the engine package without import cycles.
type DecisionBriefingStub struct {
	FilesChanged   int    `json:"files_changed"`
	ScopeStatus    string `json:"scope_status"`    // "within_limits" | "exceeded"
	Recommendation string `json:"recommendation"` // "proceed_to_qa" | "needs_human_review"
}

// buildBriefingStub creates a minimal decision briefing.
// TODO: Real briefing via engine.BuildBriefing is called from app_orchestrate_v3.go
// (the Wails binding layer), which can import both engine and orchestrator packages.
func buildBriefingStub(dir string) DecisionBriefingStub {
	return DecisionBriefingStub{
		ScopeStatus:    "within_limits",
		Recommendation: "proceed_to_qa",
	}
}

// RunQA executes the QA phase for a card.
// Called after all waves complete and card is in "qa" state.
func (o *Orchestrator) RunQA(ctx context.Context, dir, cardID string) error {
	card, err := o.board.GetTask(cardID)
	if err != nil {
		return fmt.Errorf("get card for QA: %w", err)
	}

	if card.State != board.StateQA {
		return fmt.Errorf("card %s is in state %s, expected qa", cardID, card.State)
	}

	// 1. Decision Briefing (Phase 2 stub)
	briefing := buildBriefingStub(dir)
	if briefing.Recommendation == "needs_human_review" {
		// Briefing flagged issues — escalate via stuck pipeline
		trResult, err := o.sm.Transition(card, board.EventStepStuck)
		if err != nil {
			return fmt.Errorf("transition qa->stuck for briefing: %w", err)
		}
		card.State = trResult.NewState
		if err := o.board.UpdateTask(card); err != nil {
			return fmt.Errorf("update card to stuck: %w", err)
		}
		_, escErr := o.Escalate(ctx, dir, cardID, PlanStep{ID: "briefing"}, "decision briefing flagged for human review")
		if escErr != nil {
			return fmt.Errorf("briefing escalation: %w", escErr)
		}
		return nil
	}

	// 2. Load plan to check must_haves
	boardPlan, err := o.board.GetPlan(cardID)
	if err != nil {
		return fmt.Errorf("load plan for QA: %w", err)
	}
	plan := fromBoardPlan(boardPlan)

	// 3. Check must_haves
	passed, failures := checkMustHaves(dir, plan)

	if passed {
		return o.transitionQAToMergeToDone(card)
	}

	// 4. QA failed -> QA Fix Loop
	return o.qaFixLoop(ctx, dir, cardID, plan, failures)
}

// transitionQAToMergeToDone moves a card from qa -> merging -> done.
// In Phase 2: no real merge (worktrees come in Phase 3), just mark done.
func (o *Orchestrator) transitionQAToMergeToDone(card board.TaskCard) error {
	// qa -> merging
	result, err := o.sm.Transition(card, board.EventQAPassed)
	if err != nil {
		return fmt.Errorf("transition qa->merging: %w", err)
	}
	card.State = result.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return fmt.Errorf("update card to merging: %w", err)
	}

	// merging -> done (Phase 2: no real merge, just transition)
	result, err = o.sm.Transition(card, board.EventMergeSuccess)
	if err != nil {
		return fmt.Errorf("transition merging->done: %w", err)
	}
	card.State = result.NewState
	return o.board.UpdateTask(card)
}

// checkMustHaves verifies the plan's must_haves against the actual filesystem.
// Phase 2: only checks artifacts (file existence + min_lines).
// Phase 3 will add truth verification via Haiku.
func checkMustHaves(dir string, plan Plan) (bool, []string) {
	var failures []string

	for _, step := range plan.Steps {
		for _, art := range step.MustHaves.Artifacts {
			fullPath := filepath.Join(dir, art.Path)

			// Check file exists
			info, err := os.Stat(fullPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("artifact missing: %s", art.Path))
				continue
			}
			if info.IsDir() {
				failures = append(failures, fmt.Sprintf("artifact is directory, expected file: %s", art.Path))
				continue
			}

			// Check min_lines if specified
			if art.MinLines > 0 {
				data, err := os.ReadFile(fullPath)
				if err != nil {
					failures = append(failures, fmt.Sprintf("cannot read %s: %v", art.Path, err))
					continue
				}
				lines := len(strings.Split(string(data), "\n"))
				if lines < art.MinLines {
					failures = append(failures, fmt.Sprintf("artifact %s has %d lines, need %d", art.Path, lines, art.MinLines))
				}
			}
		}
		// TODO: Truth verification via Haiku with codebase context (requires engine package).
	}

	return len(failures) == 0, failures
}

// qaFixLoop attempts to fix QA failures, max 3 times.
func (o *Orchestrator) qaFixLoop(ctx context.Context, dir, cardID string, plan Plan, failures []string) error {
	for attempt := 0; attempt < 3; attempt++ {
		card, err := o.board.GetTask(cardID)
		if err != nil {
			return fmt.Errorf("get card for QA fix: %w", err)
		}

		// Transition: qa -> executing (QA fix)
		result, err := o.sm.Transition(card, board.EventQAFailed)
		if err != nil {
			// Guard blocked (QAAttempts >= 3) — escalate via stuck pipeline
			return o.escalateQAExhausted(ctx, dir, cardID, card, failures)
		}
		card.State = result.NewState
		card.QAAttempts++
		card.ExecutionMode = "qa_fix"
		if err := o.board.UpdateTask(card); err != nil {
			return fmt.Errorf("update card for QA fix: %w", err)
		}

		// Build fix prompt and execute
		fixPrompt := buildQAFixPrompt(failures)
		req := ExecutionRequest{
			StepID:     fmt.Sprintf("qa-fix-%d", attempt+1),
			CardID:     cardID,
			Prompt:     fixPrompt,
			Model:      "sonnet",
			BudgetUSD:  0.20,
			TimeoutSec: 120,
		}

		execResult, err := o.engine.Execute(ctx, req)
		if err != nil {
			failures = append(failures, fmt.Sprintf("fix attempt %d failed: %v", attempt+1, err))
			// Transition back to QA for next attempt
			card, _ = o.board.GetTask(cardID)
			card.State = board.StateQA
			card.ExecutionMode = ""
			_ = o.board.UpdateTask(card)
			continue
		}
		o.budget.Spend(cardID, execResult.CostUSD)

		// Transition back to QA for re-check
		card, _ = o.board.GetTask(cardID)
		card.State = board.StateQA
		card.ExecutionMode = ""
		_ = o.board.UpdateTask(card)

		// Re-check must_haves
		passed, newFailures := checkMustHaves(dir, plan)
		if passed {
			return o.transitionQAToMergeToDone(card)
		}
		failures = newFailures
	}

	// All 3 attempts failed — escalate via stuck pipeline
	card, _ := o.board.GetTask(cardID)
	return o.escalateQAExhausted(ctx, dir, cardID, card, failures)
}

// buildQAFixPrompt creates a prompt for the engine to fix QA failures.
func buildQAFixPrompt(failures []string) string {
	return fmt.Sprintf(
		"The following QA checks failed. Please fix them:\n\n%s\n\nFix ONLY these issues. Do not change anything else.",
		strings.Join(failures, "\n"),
	)
}

// escalateQAExhausted transitions a card from QA through stuck into the escalation pipeline.
func (o *Orchestrator) escalateQAExhausted(ctx context.Context, dir, cardID string, card board.TaskCard, failures []string) error {
	// Transition qa -> stuck
	trResult, err := o.sm.Transition(card, board.EventStepStuck)
	if err != nil {
		// Fallback: set human_review directly if transition fails
		card.State = board.StateHumanReview
		card.ReviewReason = "qa_fix_exhausted"
		_ = o.board.UpdateTask(card)
		return fmt.Errorf("QA fix loop exhausted after %d attempts", card.QAAttempts)
	}
	card.State = trResult.NewState
	if err := o.board.UpdateTask(card); err != nil {
		return fmt.Errorf("update card to stuck: %w", err)
	}

	reason := fmt.Sprintf("QA fix loop exhausted after %d attempts, failures: %v", card.QAAttempts, failures)
	escResult, err := o.Escalate(ctx, dir, cardID, PlanStep{ID: "qa-fix", Model: "sonnet"}, reason)
	if err != nil {
		return fmt.Errorf("QA escalation failed: %w", err)
	}

	switch escResult.Action {
	case "model_escalated", "replanned":
		return nil
	case "human_review":
		return fmt.Errorf("QA fix loop exhausted, escalated to human review: %s", escResult.Reason)
	}

	return nil
}
