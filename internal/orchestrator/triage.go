package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
)

// Engine is the execution interface used by the orchestrator.
// Defined here to avoid an import cycle (engine package imports orchestrator for types).
// engine.ExecutionEngine satisfies this interface.
type Engine interface {
	Execute(ctx context.Context, req ExecutionRequest) (ExecutionResult, error)
	Cancel(stepID string) error
}

// TriageResult contains the complexity assessment for a card.
type TriageResult struct {
	Complexity string `json:"complexity"` // "trivial" | "medium" | "complex"
	Reasoning  string `json:"reasoning"`
}

// validComplexities defines accepted complexity values.
var validComplexities = map[string]bool{
	"trivial": true,
	"medium":  true,
	"complex": true,
}

// AssessComplexity runs a triage agent (Haiku) to assess card complexity.
// Returns "trivial", "medium", or "complex". Falls back to "medium" on any error.
func AssessComplexity(ctx context.Context, eng Engine, cardTitle, cardDescription, techContext string) (TriageResult, error) {
	prompt := buildTriagePrompt(cardTitle, cardDescription, techContext)

	req := ExecutionRequest{
		StepID:     "triage",
		Prompt:     prompt,
		Model:      "haiku",
		BudgetUSD:  0.01,
		TimeoutSec: 30,
	}

	result, err := eng.Execute(ctx, req)
	if err != nil {
		return TriageResult{
			Complexity: "medium",
			Reasoning:  "triage failed: " + err.Error(),
		}, nil
	}

	return parseTriageResponse(result), nil
}

func buildTriagePrompt(title, description, techContext string) string {
	return fmt.Sprintf(`Assess the complexity of this task.

Task: %s
Description: %s
Tech Context: %s

Respond with ONLY valid JSON:
{"complexity": "trivial|medium|complex", "reasoning": "brief explanation"}

Rules:
- trivial: single file change, typo fix, config update, under 20 lines
- medium: multi-file change, new function/endpoint, 20-200 lines
- complex: new subsystem, architectural change, cross-cutting concerns, 200+ lines`, title, description, techContext)
}

// parseTriageResponse extracts complexity from the engine result.
// Convention: the raw model output is stored in VerifyResult.Output
// (Phase 2 convention -- Phase 3 HeadlessEngine will provide proper output).
func parseTriageResponse(result ExecutionResult) TriageResult {
	for _, v := range result.Verify {
		if v.Output == "" {
			continue
		}
		var tr TriageResult
		if err := json.Unmarshal([]byte(v.Output), &tr); err != nil {
			continue
		}
		if validComplexities[tr.Complexity] {
			return tr
		}
	}

	return TriageResult{
		Complexity: "medium",
		Reasoning:  "could not parse triage response",
	}
}
