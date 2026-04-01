package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GeneratePlan creates an execution plan for a card using Opus.
// techContext provides project-specific context (skill prompts will be integrated later).
// Returns a validated Plan or error after max 3 repair attempts.
func GeneratePlan(ctx context.Context, eng Engine, cardTitle, cardDescription, techContext string) (Plan, error) {
	prompt := buildPlanPrompt(cardTitle, cardDescription, techContext)

	req := ExecutionRequest{
		StepID:     "plan",
		Prompt:     prompt,
		Model:      "opus",
		BudgetUSD:  0.10,
		TimeoutSec: 120,
	}

	result, err := eng.Execute(ctx, req)
	if err != nil {
		return Plan{}, fmt.Errorf("plan generation failed: %w", err)
	}

	rawJSON := extractModelOutput(result)
	if rawJSON == "" {
		return Plan{}, fmt.Errorf("plan generation returned empty output")
	}

	// Try to parse directly
	plan, parseErr := parsePlan(rawJSON)
	if parseErr == nil {
		return plan, nil
	}

	// LLM Repair loop (max 3 attempts)
	for attempt := 0; attempt < 3; attempt++ {
		repairPrompt := buildRepairPrompt(rawJSON, parseErr.Error())
		repairReq := ExecutionRequest{
			StepID:     fmt.Sprintf("plan-repair-%d", attempt+1),
			Prompt:     repairPrompt,
			Model:      "haiku",
			BudgetUSD:  0.01,
			TimeoutSec: 30,
		}

		repairResult, repairErr := eng.Execute(ctx, repairReq)
		if repairErr != nil {
			continue
		}

		repairedJSON := extractModelOutput(repairResult)
		plan, parseErr = parsePlan(repairedJSON)
		if parseErr == nil {
			return plan, nil
		}
		// Update rawJSON for next repair attempt context
		rawJSON = repairedJSON
	}

	return Plan{}, fmt.Errorf("plan validation failed after 3 repair attempts: %w", parseErr)
}

func buildPlanPrompt(title, description, techContext string) string {
	return fmt.Sprintf(`Create an execution plan for the following task.

Task: %s
Description: %s
Tech Context: %s

Respond with ONLY valid JSON matching this schema:
{
  "card_id": "string (kebab-case identifier for this task)",
  "complexity": "trivial|medium|complex",
  "steps": [
    {
      "id": "string (unique step id, e.g. 01, 02)",
      "title": "string (short description of what this step does)",
      "wave": 1,
      "depends_on": ["list of step ids this depends on"],
      "parallel_ok": true,
      "model": "sonnet|haiku|opus",
      "files_modify": ["list of files to modify"],
      "files_create": ["list of files to create"],
      "must_haves": {
        "truths": ["assertions that must be true after this step"],
        "artifacts": [{"path": "file path", "min_lines": 10}]
      },
      "verify": [{"command": "shell command", "description": "what it checks"}],
      "status": "pending"
    }
  ]
}

Rules:
- Every step MUST have id, title, and model fields
- card_id is required and must be a kebab-case identifier
- At least one step is required
- Use "sonnet" for most implementation steps, "haiku" for simple tasks, "opus" for complex reasoning
- Group independent steps into the same wave for parallel execution
- Output ONLY the JSON object, no markdown fences or extra text`, title, description, techContext)
}

func buildRepairPrompt(invalidJSON string, validationError string) string {
	return fmt.Sprintf(`The following JSON plan is invalid. Fix it and return ONLY valid JSON.

Invalid JSON:
%s

Validation error: %s

Required schema:
- "card_id": non-empty string
- "complexity": "trivial"|"medium"|"complex"
- "steps": array with at least one step, each step must have:
  - "id": non-empty string
  - "title": non-empty string
  - "model": non-empty string ("sonnet"|"haiku"|"opus")
  - "wave": integer
  - "depends_on": array of strings
  - "parallel_ok": boolean
  - "status": "pending"

Return ONLY the corrected JSON, no explanations.`, invalidJSON, validationError)
}

// extractModelOutput gets the raw model output from an ExecutionResult.
// Phase 2 convention: raw model output is stored in the first non-empty VerifyResult.Output.
func extractModelOutput(result ExecutionResult) string {
	for _, v := range result.Verify {
		if v.Output != "" {
			return v.Output
		}
	}
	return ""
}

func parsePlan(rawJSON string) (Plan, error) {
	// Strip markdown code fences if present
	cleaned := strings.TrimSpace(rawJSON)
	if strings.HasPrefix(cleaned, "```") {
		if idx := strings.Index(cleaned[3:], "\n"); idx >= 0 {
			cleaned = cleaned[3+idx+1:]
		}
		if strings.HasSuffix(cleaned, "```") {
			cleaned = cleaned[:len(cleaned)-3]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var plan Plan
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return Plan{}, fmt.Errorf("invalid JSON: %w", err)
	}

	if err := validatePlan(plan); err != nil {
		return Plan{}, err
	}

	return plan, nil
}

func validatePlan(plan Plan) error {
	if plan.CardID == "" {
		return fmt.Errorf("plan missing card_id")
	}
	if len(plan.Steps) == 0 {
		return fmt.Errorf("plan has no steps")
	}
	for i, step := range plan.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d missing id", i)
		}
		if step.Title == "" {
			return fmt.Errorf("step %d missing title", i)
		}
		if step.Model == "" {
			return fmt.Errorf("step %d missing model", i)
		}
	}
	return nil
}
