package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// MockEngine is a configurable fake ExecutionEngine for testing.
// Phase 2 orchestrator tests use this instead of real claude -p calls.
type MockEngine struct {
	mu sync.Mutex

	// Results maps stepID to the result that Execute should return.
	// If a stepID isn't in the map, DefaultResult is used.
	Results       map[string]orchestrator.ExecutionResult
	DefaultResult orchestrator.ExecutionResult

	// Calls records all Execute calls for assertions.
	Calls []orchestrator.ExecutionRequest

	// ErrorOnStep makes Execute return an error for specific stepIDs.
	ErrorOnStep map[string]error

	// SequentialResults returns results in order, ignoring stepID.
	// Useful for triage/plan where stepID isn't known ahead of time.
	SequentialResults []orchestrator.ExecutionResult
	sequentialIndex   int
}

// Compile-time interface check.
var _ ExecutionEngine = (*MockEngine)(nil)

// NewMockEngine creates a new MockEngine with initialized maps.
func NewMockEngine() *MockEngine {
	return &MockEngine{
		Results:     make(map[string]orchestrator.ExecutionResult),
		ErrorOnStep: make(map[string]error),
	}
}

// Execute records the call and returns a configured result.
// Resolution order:
//  1. ErrorOnStep[req.StepID] → return error
//  2. Results[req.StepID] → return that result
//  3. SequentialResults → return next in sequence
//  4. DefaultResult
func (m *MockEngine) Execute(ctx context.Context, req orchestrator.ExecutionRequest) (orchestrator.ExecutionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, req)

	if err, ok := m.ErrorOnStep[req.StepID]; ok {
		return orchestrator.ExecutionResult{}, err
	}

	if result, ok := m.Results[req.StepID]; ok {
		return result, nil
	}

	if m.sequentialIndex < len(m.SequentialResults) {
		result := m.SequentialResults[m.sequentialIndex]
		m.sequentialIndex++
		return result, nil
	}

	return m.DefaultResult, nil
}

// Cancel is a no-op for the mock engine.
func (m *MockEngine) Cancel(stepID string) error {
	return nil
}

// SetResult configures a specific result for a stepID.
func (m *MockEngine) SetResult(stepID string, result orchestrator.ExecutionResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Results[stepID] = result
}

// SetError configures Execute to return an error for a stepID.
func (m *MockEngine) SetError(stepID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorOnStep[stepID] = err
}

// AddSequentialResult appends a result to the sequential queue.
func (m *MockEngine) AddSequentialResult(result orchestrator.ExecutionResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SequentialResults = append(m.SequentialResults, result)
}

// CallCount returns the number of Execute calls recorded.
func (m *MockEngine) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Calls)
}

// LastCall returns the most recent Execute call. Panics if no calls recorded.
func (m *MockEngine) LastCall() orchestrator.ExecutionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Calls) == 0 {
		panic(fmt.Sprintf("MockEngine.LastCall: no calls recorded"))
	}
	return m.Calls[len(m.Calls)-1]
}

// Reset clears all recorded calls and sequential state.
func (m *MockEngine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = nil
	m.sequentialIndex = 0
}
