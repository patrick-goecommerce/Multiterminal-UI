package orchestrator

import (
	"fmt"
	"strings"
)

const baseSystemPrompt = `You are an AI coding agent working on a specific implementation task.
Follow these rules strictly:
- Implement EXACTLY what is specified, nothing more
- Write tests for your implementation
- Commit your work when done
- If you encounter issues, report them clearly
- Do not modify files outside the specified scope`

// BuildSystemPrompt composes the system prompt from base + skill prompts.
// Skill prompts are appended in order (already sorted by priority).
func BuildSystemPrompt(skillPrompts []string) string {
	parts := []string{baseSystemPrompt}
	for _, sp := range skillPrompts {
		if strings.TrimSpace(sp) != "" {
			parts = append(parts, sp)
		}
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// BuildStepPrompt composes the prompt for a specific execution step.
// Does NOT include file contents — only references to files the agent should read.
func BuildStepPrompt(step PlanStep, cardTitle, cardDescription string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## Task: %s\n\n", step.Title)
	fmt.Fprintf(&b, "Card: %s\n", cardTitle)
	if cardDescription != "" {
		fmt.Fprintf(&b, "Description: %s\n\n", cardDescription)
	}

	if len(step.FilesModify) > 0 {
		b.WriteString("\n### Files to modify\n")
		for _, f := range step.FilesModify {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	if len(step.FilesCreate) > 0 {
		b.WriteString("### Files to create\n")
		for _, f := range step.FilesCreate {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	if len(step.MustHaves.Truths) > 0 {
		b.WriteString("### Must-haves\n")
		for _, t := range step.MustHaves.Truths {
			fmt.Fprintf(&b, "- %s\n", t)
		}
		b.WriteString("\n")
	}

	if len(step.Verify) > 0 {
		b.WriteString("### Verification\n")
		b.WriteString("After implementation, these commands must pass:\n")
		for _, v := range step.Verify {
			fmt.Fprintf(&b, "- `%s` — %s\n", v.Command, v.Description)
		}
	}

	return b.String()
}

// BuildExecutionRequest constructs the full request for the engine.
// mergedVerify combines step-specific and skill-wide verify commands.
func BuildExecutionRequest(
	step PlanStep,
	cardID, cardTitle, cardDescription string,
	skillPrompts []string,
	mergedVerify []VerifyStep,
	worktreeSlot int,
	budgetUSD float64,
	timeoutSec int,
) ExecutionRequest {
	return ExecutionRequest{
		StepID:       step.ID,
		CardID:       cardID,
		WorktreeSlot: worktreeSlot,
		Prompt:       BuildStepPrompt(step, cardTitle, cardDescription),
		SystemPrompt: BuildSystemPrompt(skillPrompts),
		Model:        step.Model,
		Verify:       mergedVerify,
		BudgetUSD:    budgetUSD,
		TimeoutSec:   timeoutSec,
		SkillPrompts: skillPrompts,
	}
}
