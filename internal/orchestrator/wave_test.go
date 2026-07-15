package orchestrator

import (
	"errors"
	"testing"
)

func step(id string, deps []string, parallel bool, files []string) PlanStep {
	return PlanStep{
		ID:          id,
		Title:       "Step " + id,
		DependsOn:   deps,
		ParallelOk:  parallel,
		FilesModify: files,
		Status:      "pending",
	}
}

func TestLinearGraph(t *testing.T) {
	// A→B→C → 3 waves, 1 step each
	steps := []PlanStep{
		step("A", nil, true, nil),
		step("B", []string{"A"}, true, nil),
		step("C", []string{"B"}, true, nil),
	}

	waves, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(waves))
	}
	for i, w := range waves {
		if len(w.Steps) != 1 {
			t.Errorf("wave %d: expected 1 step, got %d", i+1, len(w.Steps))
		}
		if w.Number != i+1 {
			t.Errorf("wave %d: expected number %d, got %d", i, i+1, w.Number)
		}
	}
	if waves[0].Steps[0].ID != "A" || waves[1].Steps[0].ID != "B" || waves[2].Steps[0].ID != "C" {
		t.Error("wrong step order")
	}
}

func TestParallelGraph(t *testing.T) {
	// A, B, C (no deps, all parallel_ok) → 1 wave, 3 steps
	steps := []PlanStep{
		step("A", nil, true, nil),
		step("B", nil, true, nil),
		step("C", nil, true, nil),
	}

	waves, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if len(waves[0].Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(waves[0].Steps))
	}
}

func TestDiamondDependency(t *testing.T) {
	// A→C, B→C, A and B independent → Wave 1: [A,B], Wave 2: [C]
	steps := []PlanStep{
		step("A", nil, true, nil),
		step("B", nil, true, nil),
		step("C", []string{"A", "B"}, true, nil),
	}

	waves, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if len(waves[0].Steps) != 2 {
		t.Errorf("wave 1: expected 2 steps, got %d", len(waves[0].Steps))
	}
	if waves[0].Steps[0].ID != "A" || waves[0].Steps[1].ID != "B" {
		t.Error("wave 1: wrong steps")
	}
	if len(waves[1].Steps) != 1 || waves[1].Steps[0].ID != "C" {
		t.Error("wave 2: expected [C]")
	}
}

func TestCycleDetection(t *testing.T) {
	// A→B→A → ErrCyclicDependency
	steps := []PlanStep{
		step("A", []string{"B"}, true, nil),
		step("B", []string{"A"}, true, nil),
	}

	_, err := ComputeWaves(steps)
	if !errors.Is(err, ErrCyclicDependency) {
		t.Fatalf("expected ErrCyclicDependency, got %v", err)
	}
}

func TestFilesModifyOverlap(t *testing.T) {
	// A and B both modify router.go, no deps → 2 waves
	steps := []PlanStep{
		step("A", nil, true, []string{"router.go"}),
		step("B", nil, true, []string{"router.go"}),
	}

	waves, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if waves[0].Steps[0].ID != "A" {
		t.Error("wave 1: expected step A")
	}
	if waves[1].Steps[0].ID != "B" {
		t.Error("wave 2: expected step B")
	}
}

func TestParallelOkFalse(t *testing.T) {
	// A (parallel_ok=false), B, C all independent → A gets own wave
	steps := []PlanStep{
		step("A", nil, false, nil),
		step("B", nil, true, nil),
		step("C", nil, true, nil),
	}

	waves, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// B and C should be in one wave, A alone in another.
	foundExclusive := false
	foundParallel := false
	for _, w := range waves {
		if len(w.Steps) == 1 && w.Steps[0].ID == "A" {
			foundExclusive = true
		}
		if len(w.Steps) == 2 {
			foundParallel = true
		}
	}
	if !foundExclusive {
		t.Error("expected A in its own exclusive wave")
	}
	if !foundParallel {
		t.Error("expected B and C in same wave")
	}
}

func TestMixedComplex(t *testing.T) {
	// D depends on A and B. A and B are parallel but share a file.
	// C is parallel_ok=false with no deps. E depends on D.
	steps := []PlanStep{
		step("A", nil, true, []string{"shared.go"}),
		step("B", nil, true, []string{"shared.go"}),
		step("C", nil, false, nil),
		step("D", []string{"A", "B"}, true, nil),
		step("E", []string{"D"}, true, nil),
	}

	waves, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify ordering constraints.
	waveOf := make(map[string]int)
	for _, w := range waves {
		for _, s := range w.Steps {
			waveOf[s.ID] = w.Number
		}
	}

	// A and B must be in different waves (file conflict).
	if waveOf["A"] == waveOf["B"] {
		t.Error("A and B should not be in the same wave (file conflict)")
	}
	// D must come after both A and B.
	if waveOf["D"] <= waveOf["A"] || waveOf["D"] <= waveOf["B"] {
		t.Error("D must be after A and B")
	}
	// E must come after D.
	if waveOf["E"] <= waveOf["D"] {
		t.Error("E must be after D")
	}
	// C (exclusive) must be alone in its wave.
	for _, w := range waves {
		for _, s := range w.Steps {
			if s.ID == "C" && len(w.Steps) != 1 {
				t.Error("C (parallel_ok=false) must be alone in its wave")
			}
		}
	}
}

func TestEmptySteps(t *testing.T) {
	waves, err := ComputeWaves(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if waves != nil {
		t.Fatalf("expected nil waves for empty input, got %v", waves)
	}
}

func TestSingleStep(t *testing.T) {
	steps := []PlanStep{step("A", nil, true, nil)}

	waves, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if len(waves[0].Steps) != 1 || waves[0].Steps[0].ID != "A" {
		t.Error("expected single step A")
	}
	if waves[0].Number != 1 {
		t.Errorf("expected wave number 1, got %d", waves[0].Number)
	}
}

func TestDeterministic(t *testing.T) {
	steps := []PlanStep{
		step("C", nil, true, nil),
		step("A", nil, true, nil),
		step("B", nil, true, nil),
	}

	waves1, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waves2, err := ComputeWaves(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(waves1) != len(waves2) {
		t.Fatal("different number of waves")
	}
	for i := range waves1 {
		if len(waves1[i].Steps) != len(waves2[i].Steps) {
			t.Fatalf("wave %d: different step counts", i)
		}
		for j := range waves1[i].Steps {
			if waves1[i].Steps[j].ID != waves2[i].Steps[j].ID {
				t.Errorf("wave %d step %d: %s != %s",
					i, j, waves1[i].Steps[j].ID, waves2[i].Steps[j].ID)
			}
		}
	}
}
