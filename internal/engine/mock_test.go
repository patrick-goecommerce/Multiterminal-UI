package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

func TestMockEngine_ReturnsResultForStepID(t *testing.T) {
	m := NewMockEngine()
	expected := orchestrator.ExecutionResult{
		StepID: "step-1",
		Status: orchestrator.StepSuccess,
		CostUSD: 0.05,
	}
	m.SetResult("step-1", expected)

	got, err := m.Execute(context.Background(), orchestrator.ExecutionRequest{StepID: "step-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StepID != expected.StepID || got.Status != expected.Status || got.CostUSD != expected.CostUSD {
		t.Errorf("got %+v, want %+v", got, expected)
	}
}

func TestMockEngine_ReturnsDefaultForUnknownStepID(t *testing.T) {
	m := NewMockEngine()
	m.DefaultResult = orchestrator.ExecutionResult{
		Status: orchestrator.StepSuccess,
		CostUSD: 0.01,
	}

	got, err := m.Execute(context.Background(), orchestrator.ExecutionRequest{StepID: "unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != orchestrator.StepSuccess || got.CostUSD != 0.01 {
		t.Errorf("got %+v, want default result", got)
	}
}

func TestMockEngine_ReturnsErrorForStepID(t *testing.T) {
	m := NewMockEngine()
	expectedErr := errors.New("engine crashed")
	m.SetError("step-bad", expectedErr)

	_, err := m.Execute(context.Background(), orchestrator.ExecutionRequest{StepID: "step-bad"})
	if !errors.Is(err, expectedErr) {
		t.Errorf("got error %v, want %v", err, expectedErr)
	}
}

func TestMockEngine_SequentialResults(t *testing.T) {
	m := NewMockEngine()
	m.AddSequentialResult(orchestrator.ExecutionResult{StepID: "seq-1", Status: orchestrator.StepSuccess})
	m.AddSequentialResult(orchestrator.ExecutionResult{StepID: "seq-2", Status: orchestrator.StepFailed})
	m.AddSequentialResult(orchestrator.ExecutionResult{StepID: "seq-3", Status: orchestrator.StepTimeout})

	for i, want := range []orchestrator.StepStatus{orchestrator.StepSuccess, orchestrator.StepFailed, orchestrator.StepTimeout} {
		got, err := m.Execute(context.Background(), orchestrator.ExecutionRequest{StepID: "any"})
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if got.Status != want {
			t.Errorf("call %d: got status %q, want %q", i, got.Status, want)
		}
	}

	// After exhausting sequential results, should return DefaultResult.
	got, err := m.Execute(context.Background(), orchestrator.ExecutionRequest{StepID: "any"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "" {
		t.Errorf("expected empty default status, got %q", got.Status)
	}
}

func TestMockEngine_CallsRecorded(t *testing.T) {
	m := NewMockEngine()

	reqs := []orchestrator.ExecutionRequest{
		{StepID: "a", Prompt: "do thing A"},
		{StepID: "b", Prompt: "do thing B"},
		{StepID: "c", Prompt: "do thing C"},
	}

	for _, req := range reqs {
		_, _ = m.Execute(context.Background(), req)
	}

	if m.CallCount() != 3 {
		t.Fatalf("got %d calls, want 3", m.CallCount())
	}

	last := m.LastCall()
	if last.StepID != "c" || last.Prompt != "do thing C" {
		t.Errorf("LastCall = %+v, want stepID=c", last)
	}
}

func TestMockEngine_Reset(t *testing.T) {
	m := NewMockEngine()
	m.AddSequentialResult(orchestrator.ExecutionResult{StepID: "s1", Status: orchestrator.StepSuccess})

	_, _ = m.Execute(context.Background(), orchestrator.ExecutionRequest{StepID: "x"})

	if m.CallCount() != 1 {
		t.Fatalf("expected 1 call before reset")
	}

	m.Reset()

	if m.CallCount() != 0 {
		t.Errorf("expected 0 calls after reset, got %d", m.CallCount())
	}

	// Sequential index should also be reset, so the sequential result
	// should NOT be available (it's still in the slice but index is 0,
	// meaning it will be returned again).
	got, _ := m.Execute(context.Background(), orchestrator.ExecutionRequest{StepID: "y"})
	if got.StepID != "s1" {
		t.Errorf("expected sequential result after reset, got %+v", got)
	}
}

func TestMockEngine_CancelIsNoop(t *testing.T) {
	m := NewMockEngine()
	if err := m.Cancel("anything"); err != nil {
		t.Errorf("Cancel returned error: %v", err)
	}
}
