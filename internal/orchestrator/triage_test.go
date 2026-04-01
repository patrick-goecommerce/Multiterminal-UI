package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// testEngine is a minimal mock that satisfies the Engine interface for testing.
type testEngine struct {
	mu              sync.Mutex
	calls           []ExecutionRequest
	sequentialQueue []ExecutionResult
	seqIndex        int
	errorOnStep     map[string]error
}

func newTestEngine() *testEngine {
	return &testEngine{errorOnStep: make(map[string]error)}
}

func (e *testEngine) Execute(ctx context.Context, req ExecutionRequest) (ExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, req)

	if err, ok := e.errorOnStep[req.StepID]; ok {
		return ExecutionResult{}, err
	}
	if e.seqIndex < len(e.sequentialQueue) {
		r := e.sequentialQueue[e.seqIndex]
		e.seqIndex++
		return r, nil
	}
	return ExecutionResult{}, fmt.Errorf("no result configured")
}

func (e *testEngine) Cancel(stepID string) error { return nil }

func (e *testEngine) addResult(r ExecutionResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sequentialQueue = append(e.sequentialQueue, r)
}

func (e *testEngine) setError(stepID string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errorOnStep[stepID] = err
}

func (e *testEngine) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func (e *testEngine) lastCall() ExecutionRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls[len(e.calls)-1]
}

// --- helpers ---

func triageResult(output string) ExecutionResult {
	return ExecutionResult{
		StepID: "triage",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: output}},
	}
}

// --- tests ---

func TestAssessComplexity_Trivial(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(triageResult(`{"complexity":"trivial","reasoning":"simple config change"}`))

	result, err := AssessComplexity(context.Background(), eng, "Fix typo", "Fix typo in README", "go project")
	if err != nil {
		t.Fatal(err)
	}
	if result.Complexity != "trivial" {
		t.Errorf("got %s, want trivial", result.Complexity)
	}
	if result.Reasoning != "simple config change" {
		t.Errorf("got reasoning %q, want %q", result.Reasoning, "simple config change")
	}
}

func TestAssessComplexity_Medium(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(triageResult(`{"complexity":"medium","reasoning":"new API endpoint"}`))

	result, err := AssessComplexity(context.Background(), eng, "Add endpoint", "Add /api/users endpoint", "go project")
	if err != nil {
		t.Fatal(err)
	}
	if result.Complexity != "medium" {
		t.Errorf("got %s, want medium", result.Complexity)
	}
}

func TestAssessComplexity_Complex(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(triageResult(`{"complexity":"complex","reasoning":"new auth subsystem"}`))

	result, err := AssessComplexity(context.Background(), eng, "Auth system", "Implement OAuth2 flow", "go project")
	if err != nil {
		t.Fatal(err)
	}
	if result.Complexity != "complex" {
		t.Errorf("got %s, want complex", result.Complexity)
	}
}

func TestAssessComplexity_InvalidJSON_FallbackMedium(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(triageResult(`not valid json at all`))

	result, err := AssessComplexity(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}
	if result.Complexity != "medium" {
		t.Errorf("got %s, want medium (fallback)", result.Complexity)
	}
	if result.Reasoning != "could not parse triage response" {
		t.Errorf("got reasoning %q, want fallback reasoning", result.Reasoning)
	}
}

func TestAssessComplexity_EngineError_FallbackMedium(t *testing.T) {
	eng := newTestEngine()
	eng.setError("triage", errors.New("connection refused"))

	result, err := AssessComplexity(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal("expected no error on engine failure, got:", err)
	}
	if result.Complexity != "medium" {
		t.Errorf("got %s, want medium (fallback)", result.Complexity)
	}
	if result.Reasoning != "triage failed: connection refused" {
		t.Errorf("got reasoning %q, want error reasoning", result.Reasoning)
	}
}

func TestAssessComplexity_EmptyVerifyOutput_FallbackMedium(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(ExecutionResult{
		StepID: "triage",
		Status: StepSuccess,
		Verify: []VerifyResult{{Output: ""}},
	})

	result, err := AssessComplexity(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}
	if result.Complexity != "medium" {
		t.Errorf("got %s, want medium (fallback)", result.Complexity)
	}
}

func TestAssessComplexity_UnknownComplexity_FallbackMedium(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(triageResult(`{"complexity":"huge","reasoning":"very big"}`))

	result, err := AssessComplexity(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}
	if result.Complexity != "medium" {
		t.Errorf("got %s, want medium (fallback for unknown value)", result.Complexity)
	}
}

func TestAssessComplexity_UsesHaikuModel(t *testing.T) {
	eng := newTestEngine()
	eng.addResult(triageResult(`{"complexity":"trivial","reasoning":"test"}`))

	_, err := AssessComplexity(context.Background(), eng, "Task", "Desc", "ctx")
	if err != nil {
		t.Fatal(err)
	}

	if eng.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", eng.callCount())
	}
	call := eng.lastCall()
	if call.Model != "haiku" {
		t.Errorf("got model %q, want haiku", call.Model)
	}
	if call.StepID != "triage" {
		t.Errorf("got stepID %q, want triage", call.StepID)
	}
}
