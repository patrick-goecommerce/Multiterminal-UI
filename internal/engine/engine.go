package engine

import (
	"context"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// ExecutionEngine is the contract between Orchestrator and Engine.
// The Orchestrator sends ExecutionRequests, the Engine returns ExecutionResults.
// The Orchestrator assembles prompts, the Engine only executes.
type ExecutionEngine interface {
	Execute(ctx context.Context, req orchestrator.ExecutionRequest) (orchestrator.ExecutionResult, error)
	Cancel(stepID string) error
}
