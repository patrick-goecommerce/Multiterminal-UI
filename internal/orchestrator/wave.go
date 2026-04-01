package orchestrator

import (
	"errors"
	"fmt"
	"sort"
)

// ErrCyclicDependency is returned when the dependency graph has a cycle.
var ErrCyclicDependency = errors.New("cyclic dependency detected")

// Wave is a group of steps that can execute in parallel.
type Wave struct {
	Number int        `json:"number"`
	Steps  []PlanStep `json:"steps"`
}

// ComputeWaves groups plan steps into dependency-ordered waves.
// Rules:
// 1. A step can only be in a wave if ALL its depends_on are in earlier waves.
// 2. parallel_ok=false forces a step into its own exclusive wave.
// 3. Steps with files_modify overlap cannot be in the same wave.
// 4. Deterministic: same input always produces same output (sort by step ID).
func ComputeWaves(steps []PlanStep) ([]Wave, error) {
	if len(steps) == 0 {
		return nil, nil
	}

	// Build index and validate dependencies.
	idx := make(map[string]int, len(steps))
	for i, s := range steps {
		idx[s.ID] = i
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, ok := idx[dep]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", s.ID, dep)
			}
		}
	}

	// Detect cycles via DFS.
	if err := detectCycles(steps, idx); err != nil {
		return nil, err
	}

	// Assign initial wave numbers based on dependencies.
	waveNum := make(map[string]int, len(steps))
	if err := assignWaves(steps, idx, waveNum); err != nil {
		return nil, err
	}

	// Group steps by wave number.
	grouped := groupByWave(steps, waveNum)

	// Post-process: split on parallel_ok=false and files_modify overlap.
	waves := postProcess(grouped)

	// Compact wave numbers and sort steps within each wave.
	for i := range waves {
		waves[i].Number = i + 1
		sort.Slice(waves[i].Steps, func(a, b int) bool {
			return waves[i].Steps[a].ID < waves[i].Steps[b].ID
		})
	}

	return waves, nil
}

// detectCycles uses DFS coloring to find cycles.
// white=0, gray=1, black=2.
func detectCycles(steps []PlanStep, idx map[string]int) error {
	color := make(map[string]int, len(steps))
	for _, s := range steps {
		color[s.ID] = 0
	}

	var visit func(id string) error
	visit = func(id string) error {
		color[id] = 1 // gray
		step := steps[idx[id]]
		for _, dep := range step.DependsOn {
			if color[dep] == 1 {
				return ErrCyclicDependency
			}
			if color[dep] == 0 {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[id] = 2 // black
		return nil
	}

	// Visit in sorted order for determinism.
	ids := make([]string, 0, len(steps))
	for _, s := range steps {
		ids = append(ids, s.ID)
	}
	sort.Strings(ids)

	for _, id := range ids {
		if color[id] == 0 {
			if err := visit(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// assignWaves computes wave number for each step using memoized DFS.
func assignWaves(steps []PlanStep, idx map[string]int, waveNum map[string]int) error {
	computed := make(map[string]bool, len(steps))

	var compute func(id string) int
	compute = func(id string) int {
		if computed[id] {
			return waveNum[id]
		}
		step := steps[idx[id]]
		maxDep := 0
		for _, dep := range step.DependsOn {
			w := compute(dep)
			if w > maxDep {
				maxDep = w
			}
		}
		waveNum[id] = maxDep + 1
		computed[id] = true
		return waveNum[id]
	}

	for _, s := range steps {
		compute(s.ID)
	}
	return nil
}

// groupByWave collects steps into wave buckets, sorted by wave number.
func groupByWave(steps []PlanStep, waveNum map[string]int) [][]PlanStep {
	maxWave := 0
	for _, w := range waveNum {
		if w > maxWave {
			maxWave = w
		}
	}

	groups := make([][]PlanStep, maxWave)
	for _, s := range steps {
		w := waveNum[s.ID] - 1 // 0-indexed
		groups[w] = append(groups[w], s)
	}
	return groups
}

// postProcess splits waves for parallel_ok=false and files_modify overlap.
func postProcess(groups [][]PlanStep) []Wave {
	var waves []Wave

	for _, group := range groups {
		// Sort group by ID for determinism before splitting.
		sort.Slice(group, func(a, b int) bool {
			return group[a].ID < group[b].ID
		})

		// Separate exclusive (parallel_ok=false) from parallel steps.
		var parallel, exclusive []PlanStep
		for _, s := range group {
			if !s.ParallelOk {
				exclusive = append(exclusive, s)
			} else {
				parallel = append(parallel, s)
			}
		}

		// Split parallel steps by files_modify overlap.
		subWaves := splitByFileConflict(parallel)
		for _, sw := range subWaves {
			waves = append(waves, Wave{Steps: sw})
		}

		// Each exclusive step gets its own wave.
		for _, s := range exclusive {
			waves = append(waves, Wave{Steps: []PlanStep{s}})
		}
	}

	return waves
}

// splitByFileConflict splits a set of parallel steps into sub-waves
// so no two steps in the same sub-wave modify the same file.
func splitByFileConflict(steps []PlanStep) [][]PlanStep {
	if len(steps) == 0 {
		return nil
	}

	var subWaves [][]PlanStep

	for _, step := range steps {
		placed := false
		for i := range subWaves {
			if !hasFileConflict(subWaves[i], step) {
				subWaves[i] = append(subWaves[i], step)
				placed = true
				break
			}
		}
		if !placed {
			subWaves = append(subWaves, []PlanStep{step})
		}
	}

	return subWaves
}

// hasFileConflict checks if adding step to wave would create a
// files_modify overlap with any existing step.
func hasFileConflict(wave []PlanStep, step PlanStep) bool {
	for _, existing := range wave {
		for _, f1 := range existing.FilesModify {
			for _, f2 := range step.FilesModify {
				if f1 == f2 {
					return true
				}
			}
		}
	}
	return false
}
