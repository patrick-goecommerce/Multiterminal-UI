# Kanban Agent Orchestration v2 — Phase 3: Parallel Execution + Memory

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Steps marked `parallel_ok` in a plan execute simultaneously in isolated git worktrees, with stall detection, lead-agent error handling, and a self-organizing memory layer that persists learnings across cards.

**Architecture:** `runPlanExecution` is refactored to group steps by dependency waves. Each wave's `parallel_ok` steps get their own worktree + `RunHeadless` goroutine. A stall detector monitors `LastOutputAt` and reassigns stuck claims. After all steps complete, a memory update writes learnings to `docs/mtui/`. Pattern reuse injects completed card summaries into new plan prompts.

**Tech Stack:** Go 1.21+ (backend), Svelte 4 (frontend), Git worktrees, `claude -p`, Wails v3 events

**Spec:** `docs/superpowers/specs/2026-03-15-kanban-agent-orchestration-v2-design.md` (Phase 3)

---

## File Structure

### New Files (Backend)
- `internal/backend/app_orchestrate_parallel.go` — Parallel step execution: dependency wave grouping, worktree-per-step spawning, merge logic
- `internal/backend/app_orchestrate_parallel_test.go` — Tests for wave grouping and merge logic
- `internal/backend/app_orchestrate_memory.go` — Memory lifecycle: post-execution learnings, pattern reuse for new plans
- `internal/backend/app_orchestrate_memory_test.go` — Tests for memory file operations

### Modified Files (Backend)
- `internal/backend/app_orchestrate_exec.go` — Refactor `runPlanExecution` to call parallel execution for waves with `parallel_ok` steps
- `internal/backend/app_orchestrate_plan.go` — Inject pattern reuse context into plan generation prompts

### Modified Files (Frontend)
- `frontend/src/components/KanbanSubCards.svelte` — Show parallel grouping indicator on sub-cards

---

## Chunk 1: Parallel Execution Engine

### Task 1: Implement dependency wave grouping

**Files:**
- Create: `internal/backend/app_orchestrate_parallel.go`
- Create: `internal/backend/app_orchestrate_parallel_test.go`

- [ ] **Step 1: Write tests for wave grouping**

Create `internal/backend/app_orchestrate_parallel_test.go`:

```go
package backend

import (
	"testing"
)

func TestGroupStepsIntoWaves(t *testing.T) {
	steps := []CardPlanStep{
		{ID: "1", Title: "Backend", ParallelOK: true, DependsOn: nil},
		{ID: "2", Title: "Frontend", ParallelOK: true, DependsOn: nil},
		{ID: "3", Title: "Integration", ParallelOK: false, DependsOn: []string{"1", "2"}},
		{ID: "4", Title: "Docs", ParallelOK: true, DependsOn: []string{"3"}},
	}
	waves := groupStepsIntoWaves(steps)
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(waves))
	}
	// Wave 1: steps 1+2 (no dependencies, parallel)
	if len(waves[0]) != 2 {
		t.Errorf("wave 0: expected 2 steps, got %d", len(waves[0]))
	}
	// Wave 2: step 3 (depends on 1+2)
	if len(waves[1]) != 1 {
		t.Errorf("wave 1: expected 1 step, got %d", len(waves[1]))
	}
	// Wave 3: step 4 (depends on 3)
	if len(waves[2]) != 1 {
		t.Errorf("wave 2: expected 1 step, got %d", len(waves[2]))
	}
}

func TestGroupStepsIntoWaves_AllSequential(t *testing.T) {
	steps := []CardPlanStep{
		{ID: "1", Title: "A", ParallelOK: false},
		{ID: "2", Title: "B", ParallelOK: false, DependsOn: []string{"1"}},
		{ID: "3", Title: "C", ParallelOK: false, DependsOn: []string{"2"}},
	}
	waves := groupStepsIntoWaves(steps)
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(waves))
	}
	for i, w := range waves {
		if len(w) != 1 {
			t.Errorf("wave %d: expected 1 step, got %d", i, len(w))
		}
	}
}

func TestGroupStepsIntoWaves_AllParallel(t *testing.T) {
	steps := []CardPlanStep{
		{ID: "1", Title: "A", ParallelOK: true},
		{ID: "2", Title: "B", ParallelOK: true},
		{ID: "3", Title: "C", ParallelOK: true},
	}
	waves := groupStepsIntoWaves(steps)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if len(waves[0]) != 3 {
		t.Errorf("wave 0: expected 3 steps, got %d", len(waves[0]))
	}
}

func TestGroupStepsIntoWaves_Empty(t *testing.T) {
	waves := groupStepsIntoWaves(nil)
	if len(waves) != 0 {
		t.Errorf("expected 0 waves, got %d", len(waves))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run TestGroupSteps -v 2>&1 | tail -5`
Expected: FAIL

- [ ] **Step 3: Implement parallel execution**

Create `internal/backend/app_orchestrate_parallel.go`:

```go
// Package backend provides parallel step execution in git worktrees.
package backend

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"
)

// executionWave is a group of steps that can run in parallel.
type executionWave struct {
	Steps []CardPlanStep
}

// groupStepsIntoWaves partitions plan steps into dependency-ordered waves.
// Steps with no unresolved dependencies and parallel_ok=true are grouped together.
// Steps with parallel_ok=false always form a single-step wave.
func groupStepsIntoWaves(steps []CardPlanStep) [][]CardPlanStep {
	if len(steps) == 0 {
		return nil
	}

	completed := map[string]bool{}
	remaining := make([]CardPlanStep, len(steps))
	copy(remaining, steps)

	var waves [][]CardPlanStep

	for len(remaining) > 0 {
		var wave []CardPlanStep
		var next []CardPlanStep

		for _, step := range remaining {
			if depsResolved(step.DependsOn, completed) {
				wave = append(wave, step)
			} else {
				next = append(next, step)
			}
		}

		if len(wave) == 0 {
			// Circular dependency or error — dump remaining as sequential
			for _, s := range remaining {
				waves = append(waves, []CardPlanStep{s})
			}
			break
		}

		// Split non-parallel steps into individual waves
		var parallel []CardPlanStep
		for _, s := range wave {
			if s.ParallelOK && len(parallel) > 0 || (s.ParallelOK && len(parallel) == 0) {
				parallel = append(parallel, s)
			} else {
				// Non-parallel step gets its own wave
				if len(parallel) > 0 {
					waves = append(waves, parallel)
					parallel = nil
				}
				waves = append(waves, []CardPlanStep{s})
				completed[s.ID] = true
			}
		}
		if len(parallel) > 0 {
			waves = append(waves, parallel)
			for _, s := range parallel {
				completed[s.ID] = true
			}
		}

		// Mark non-parallel steps completed too
		for _, s := range wave {
			completed[s.ID] = true
		}

		remaining = next
	}

	return waves
}

func depsResolved(deps []string, completed map[string]bool) bool {
	for _, d := range deps {
		if !completed[d] {
			return false
		}
	}
	return true
}

// parallelResult holds the outcome of one parallel step execution.
type parallelResult struct {
	StepIndex int
	StepID    string
	Success   bool
	Summary   string
	Err       error
}

// executeWaveParallel runs all steps in a wave concurrently, each in its own worktree.
// Returns results for each step.
func (a *AppService) executeWaveParallel(
	ctx context.Context,
	dir string,
	card *KanbanCard,
	wave []CardPlanStep,
	subCardIDs []string,
	stepIndexOffset int,
	prevSummaries []string,
	coderPrompt string,
) []parallelResult {
	results := make([]parallelResult, len(wave))
	var wg sync.WaitGroup

	for i, step := range wave {
		wg.Add(1)
		go func(idx int, s CardPlanStep) {
			defer wg.Done()

			subCardID := subCardIDs[stepIndexOffset+idx]
			a.updateSubCardStatus(dir, subCardID, "in_progress")

			// Create isolated worktree for this step
			wtName := fmt.Sprintf("step-%s-%s", card.ID, s.ID)
			wt, err := a.CreateNamedWorktree(dir, wtName, "")
			if err != nil {
				log.Printf("[parallel] worktree creation failed for step %s: %v", s.ID, err)
				results[idx] = parallelResult{StepIndex: stepIndexOffset + idx, StepID: s.ID, Err: err}
				a.updateSubCardStatus(dir, subCardID, "fail")
				a.emitStepUpdate(card.ID, subCardID, "fail", fmt.Sprintf("Worktree error: %v", err))
				return
			}

			// Execute in worktree
			prompt := buildStepExecutionPrompt(card.Title, card.Prompt, &s, prevSummaries, coderPrompt)
			stepCtx, stepCancel := context.WithTimeout(ctx, 300*time.Second)
			result, err := a.RunHeadless(stepCtx, prompt, coderPrompt, wt.Path, 0)
			stepCancel()

			if err != nil {
				results[idx] = parallelResult{StepIndex: stepIndexOffset + idx, StepID: s.ID, Err: err}
				a.updateSubCardStatus(dir, subCardID, "fail")
				a.emitStepUpdate(card.ID, subCardID, "fail", fmt.Sprintf("Execution error: %v", err))
				return
			}

			stepResult, err := parseStepResult(result.Data)
			if err != nil || stepResult.Status == "failed" {
				msg := "parse error"
				if stepResult != nil {
					msg = stepResult.Summary
				}
				results[idx] = parallelResult{StepIndex: stepIndexOffset + idx, StepID: s.ID, Err: fmt.Errorf(msg)}
				a.updateSubCardStatus(dir, subCardID, "fail")
				a.emitStepUpdate(card.ID, subCardID, "fail", msg)
				return
			}

			results[idx] = parallelResult{
				StepIndex: stepIndexOffset + idx,
				StepID:    s.ID,
				Success:   true,
				Summary:   stepResult.Summary,
			}
			a.updateSubCardStatus(dir, subCardID, "finished")
			a.emitStepUpdate(card.ID, subCardID, "finished", stepResult.Summary)
		}(i, step)
	}

	wg.Wait()

	// Merge worktrees back — sequential to avoid conflicts
	for _, step := range wave {
		wtName := fmt.Sprintf("step-%s-%s", card.ID, step.ID)
		wtPath := filepath.Join(dir, ".mt-worktrees", wtName)
		mergeErr := mergeWorktreeChanges(dir, wtPath, wtName)
		if mergeErr != nil {
			log.Printf("[parallel] merge failed for step %s: %v", step.ID, mergeErr)
		}
		// Cleanup worktree
		_ = removeNamedWorktree(dir, wtName)
	}

	return results
}

// mergeWorktreeChanges merges changes from a worktree branch back to the main branch.
func mergeWorktreeChanges(mainDir, wtPath, branchName string) error {
	// Commit any changes in the worktree first
	commitCtx, commitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer commitCancel()

	cmd := execCmd(commitCtx, "git", "add", "-A")
	cmd.Dir = wtPath
	_ = cmd.Run() // Ignore if nothing to add

	cmd = execCmd(commitCtx, "git", "commit", "-m", fmt.Sprintf("step: %s", branchName))
	cmd.Dir = wtPath
	_ = cmd.Run() // Ignore if nothing to commit

	// Merge the worktree branch into the main working directory
	mergeCtx, mergeCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer mergeCancel()

	cmd = execCmd(mergeCtx, "git", "merge", fmt.Sprintf("terminal/%s", branchName), "--no-edit")
	cmd.Dir = mainDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge failed: %s", out)
	}
	return nil
}

// removeNamedWorktree removes a named worktree.
func removeNamedWorktree(dir, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := execCmd(ctx, "git", "worktree", "remove", filepath.Join(".mt-worktrees", name), "--force")
	cmd.Dir = dir
	return cmd.Run()
}

// execCmd creates an exec.Command (not using COMSPEC — git works directly on Windows).
func execCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
```

**Note:** Add `"os/exec"` to the imports.

- [ ] **Step 4: Run tests**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run TestGroupSteps -v`
Expected: All PASS

Run: `cd D:/repos/Multiterminal && go vet ./internal/backend/...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_orchestrate_parallel.go internal/backend/app_orchestrate_parallel_test.go
git commit -m "feat(backend): add parallel step execution with worktree isolation and wave grouping"
```

---

### Task 2: Refactor execution loop to use waves

**Files:**
- Modify: `internal/backend/app_orchestrate_exec.go`

- [ ] **Step 1: Replace sequential loop with wave-based execution**

Replace the `runPlanExecution` method. The new version:
1. Groups steps into waves via `groupStepsIntoWaves`
2. For single-step waves: runs sequentially (existing behavior)
3. For multi-step waves: calls `executeWaveParallel`
4. Tracks completed summaries across waves
5. Runs drift check after every 2nd completed step (same as before)

The key change is replacing the `for i, step := range card.CardPlan.Steps` loop with:

```go
func (a *AppService) runPlanExecution(ctx context.Context, dir, cardID string) {
	defer func() {
		a.mu.Lock()
		delete(a.activeExecutions, cardID)
		a.mu.Unlock()
	}()

	state, err := loadKanbanState(dir)
	if err != nil {
		log.Printf("[orchestrate] load state failed: %v", err)
		return
	}
	card, _ := findCard(state, cardID)
	if card == nil || card.CardPlan == nil {
		return
	}

	completedSummaries := []string{}
	coderPrompt := loadAgentDef(dir, "coder")
	completedCount := 0
	waves := groupStepsIntoWaves(card.CardPlan.Steps)
	stepOffset := 0

	for _, wave := range waves {
		if ctx.Err() != nil {
			return
		}

		if len(wave) == 1 {
			// Sequential: single step (existing logic)
			step := wave[0]
			subCardID := card.SubCards[stepOffset]
			a.updateSubCardStatus(dir, subCardID, "in_progress")

			prompt := buildStepExecutionPrompt(card.Title, card.Prompt, &step, completedSummaries, coderPrompt)
			stepCtx, stepCancel := context.WithTimeout(ctx, 300*time.Second)
			result, err := a.RunHeadless(stepCtx, prompt, coderPrompt, dir, 0)
			stepCancel()

			if err != nil {
				a.updateSubCardStatus(dir, subCardID, "fail")
				a.emitStepUpdate(cardID, subCardID, "fail", fmt.Sprintf("Error: %v", err))
				stepOffset++
				continue
			}

			stepResult, parseErr := parseStepResult(result.Data)
			if parseErr != nil || stepResult.Status == "failed" {
				msg := "parse error"
				if stepResult != nil {
					msg = stepResult.Summary
				}
				a.updateSubCardStatus(dir, subCardID, "fail")
				a.emitStepUpdate(cardID, subCardID, "fail", msg)
				stepOffset++
				continue
			}

			// QA review
			diff := getDiff(dir)
			qaCtx, qaCancel := context.WithTimeout(ctx, 120*time.Second)
			approved, issues, _ := a.reviewStep(qaCtx, dir, card.Title, step.Desc, diff)
			qaCancel()
			if !approved {
				corrPrompt := fmt.Sprintf("Fix these QA issues:\n%s", joinStrs(issues))
				corrCtx, corrCancel := context.WithTimeout(ctx, 300*time.Second)
				_, _ = a.RunHeadless(corrCtx, corrPrompt, coderPrompt, dir, 0)
				corrCancel()
			}

			completedCount++
			completedSummaries = append(completedSummaries, stepResult.Summary)
			a.updateSubCardStatus(dir, subCardID, "finished")
			a.emitStepUpdate(cardID, subCardID, "finished", stepResult.Summary)
			stepOffset++
		} else {
			// Parallel: multiple steps in worktrees
			results := a.executeWaveParallel(ctx, dir, card, wave, card.SubCards, stepOffset, completedSummaries, coderPrompt)
			for _, r := range results {
				if r.Success {
					completedCount++
					completedSummaries = append(completedSummaries, r.Summary)
				}
			}
			stepOffset += len(wave)
		}

		// Anti-drift every 2nd completed step
		if completedCount > 0 && completedCount%2 == 0 {
			onTrack, drift, _ := a.checkDrift(ctx, dir, card, completedSummaries)
			if !onTrack {
				a.emitStepUpdate(cardID, "", "drift", drift)
				return
			}
		}
	}

	finalStatus := "finished"
	if completedCount < len(card.CardPlan.Steps) {
		finalStatus = "fail"
	}
	a.updateParentStatus(dir, cardID, finalStatus)
	a.app.Event.Emit("kanban:execution-complete", map[string]any{
		"card_id": cardID, "status": finalStatus,
		"completed": completedCount, "total": len(card.CardPlan.Steps),
	})
}
```

- [ ] **Step 2: Verify tests still pass**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestCreateSub|TestBuildStep|TestParseStep|TestGroupSteps" -v`
Expected: All PASS

- [ ] **Step 3: Verify file size**

Run: `wc -l internal/backend/app_orchestrate_exec.go`
Expected: Under 300 lines (the old sequential loop is replaced, not expanded)

- [ ] **Step 4: Commit**

```bash
git add internal/backend/app_orchestrate_exec.go
git commit -m "refactor(backend): replace sequential execution with wave-based parallel/sequential hybrid"
```

---

## Chunk 2: Memory Layer + Pattern Reuse

### Task 3: Implement memory lifecycle

**Files:**
- Create: `internal/backend/app_orchestrate_memory.go`
- Create: `internal/backend/app_orchestrate_memory_test.go`

- [ ] **Step 1: Write tests**

Create `internal/backend/app_orchestrate_memory_test.go`:

```go
package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCardMemory(t *testing.T) {
	dir := t.TempDir()
	card := &KanbanCard{
		ID:    "test-1",
		Title: "OAuth Login",
		QuizAnswers: map[string]string{"q1": "Google"},
	}
	learnings := []string{"API uses v2", "Needs HTTPS locally"}

	err := writeCardMemory(dir, card, learnings)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	path := filepath.Join(dir, "docs", "mtui", "board", "test-1.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Error("file should not be empty")
	}
}

func TestLoadCompletedCardSummaries(t *testing.T) {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, "docs", "mtui", "board")
	_ = os.MkdirAll(boardDir, 0o755)
	_ = os.WriteFile(filepath.Join(boardDir, "card-a.md"), []byte("# Card A\nSummary: Built auth"), 0o644)
	_ = os.WriteFile(filepath.Join(boardDir, "card-b.md"), []byte("# Card B\nSummary: Added tests"), 0o644)

	summaries := loadCompletedCardSummaries(dir)
	if len(summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestLoadCompletedCardSummaries_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	summaries := loadCompletedCardSummaries(dir)
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}
```

- [ ] **Step 2: Implement memory operations**

Create `internal/backend/app_orchestrate_memory.go`:

```go
// Package backend provides memory persistence for card execution learnings.
package backend

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// writeCardMemory persists execution learnings for a card to docs/mtui/board/.
func writeCardMemory(dir string, card *KanbanCard, learnings []string) error {
	boardDir := filepath.Join(dir, "docs", "mtui", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		return fmt.Errorf("create board dir: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", card.Title))

	if len(card.QuizAnswers) > 0 {
		sb.WriteString("## Entscheidungen\n\n")
		for q, a := range card.QuizAnswers {
			sb.WriteString(fmt.Sprintf("- **%s:** %s\n", q, a))
		}
		sb.WriteString("\n")
	}

	if card.CardPlan != nil {
		sb.WriteString(fmt.Sprintf("## Plan\n\n%s\n\n", card.CardPlan.Summary))
		sb.WriteString(fmt.Sprintf("Schritte: %d\n\n", len(card.CardPlan.Steps)))
	}

	if len(learnings) > 0 {
		sb.WriteString("## Erkenntnisse\n\n")
		for _, l := range learnings {
			sb.WriteString(fmt.Sprintf("- %s\n", l))
		}
		sb.WriteString("\n")
	}

	path := filepath.Join(boardDir, card.ID+".md")
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// loadCompletedCardSummaries reads all card memory files for pattern reuse.
func loadCompletedCardSummaries(dir string) []string {
	boardDir := filepath.Join(dir, "docs", "mtui", "board")
	entries, err := os.ReadDir(boardDir)
	if err != nil {
		return nil
	}

	var summaries []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(boardDir, e.Name()))
		if err != nil {
			continue
		}
		// Truncate to first 500 chars per card
		s := string(data)
		if len(s) > 500 {
			s = s[:500] + "..."
		}
		summaries = append(summaries, s)
	}
	return summaries
}

// updateMemoryAfterExecution writes learnings and optionally runs a headless
// Claude call to organize docs/mtui/.
func (a *AppService) updateMemoryAfterExecution(dir string, card *KanbanCard, learnings []string) {
	if err := writeCardMemory(dir, card, learnings); err != nil {
		log.Printf("[memory] write card memory failed: %v", err)
		return
	}

	// Optional: headless Claude call to organize docs/mtui/
	// Only run if there are significant learnings
	if len(learnings) < 2 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`Review and update the project memory in docs/mtui/:
Card completed: %s
Learnings: %s

Update docs/mtui/README.md index if needed.
Add any project-level learnings that would help future cards.
Respond with JSON: {"updated_files": ["file1.md"]}`, card.Title, strings.Join(learnings, "; "))

	_, err := a.RunHeadless(ctx, prompt, "", dir, 0)
	if err != nil {
		log.Printf("[memory] memory organization failed (non-fatal): %v", err)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestWriteCard|TestLoadCompleted" -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/backend/app_orchestrate_memory.go internal/backend/app_orchestrate_memory_test.go
git commit -m "feat(backend): add memory persistence layer for card execution learnings"
```

---

### Task 4: Wire memory into execution + plan generation

**Files:**
- Modify: `internal/backend/app_orchestrate_exec.go` — call `updateMemoryAfterExecution` at end of `runPlanExecution`
- Modify: `internal/backend/app_orchestrate_plan.go` — inject pattern reuse into plan prompts

- [ ] **Step 1: Add memory update call to execution completion**

In `runPlanExecution`, before the final `a.app.Event.Emit("kanban:execution-complete", ...)`, add:

```go
	// Persist learnings to docs/mtui/
	go a.updateMemoryAfterExecution(dir, card, completedSummaries)
```

- [ ] **Step 2: Add pattern reuse to plan generation**

In `app_orchestrate_plan.go`, modify `GenerateCardPlan` to load completed card summaries.
Before calling `buildPlanPrompt`, add:

```go
	// Pattern reuse: include summaries from completed cards
	pastCards := loadCompletedCardSummaries(dir)
	patternCtx := ""
	if len(pastCards) > 0 {
		patternCtx = "\nPrevious completed cards for reference:\n"
		for i, s := range pastCards {
			if i >= 3 { break } // Max 3 references
			patternCtx += s + "\n---\n"
		}
	}
```

Then pass `patternCtx` into the prompt (append to `memCtx`):

```go
	memCtx := loadMemoryContext(dir) + patternCtx
```

- [ ] **Step 3: Verify compilation and tests**

Run: `cd D:/repos/Multiterminal && go vet ./internal/backend/... && go test ./internal/backend/ -run "TestParse|TestBuild|TestCreate|TestGroup|TestWrite|TestLoad" -v 2>&1 | tail -20`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/backend/app_orchestrate_exec.go internal/backend/app_orchestrate_plan.go
git commit -m "feat(backend): wire memory persistence into execution and pattern reuse into plan generation"
```

---

### Task 5: Add parallel indicator to KanbanSubCards

**Files:**
- Modify: `frontend/src/components/KanbanSubCards.svelte`

- [ ] **Step 1: Show parallel grouping**

In the sub-card list, add a visual indicator when consecutive sub-cards are part of a parallel wave. Add a `parallel` badge next to sub-cards that are marked as `parallel_ok` in the parent plan.

Since sub-cards don't directly carry the `parallel_ok` flag, derive it from the parent's `card_plan.steps`:

```typescript
  function isParallel(index: number): boolean {
    const steps = parentCard.card_plan?.steps;
    if (!steps || index >= steps.length) return false;
    return steps[index]?.parallel_ok ?? false;
  }
```

In the template, after `{sc.title}`, add:

```svelte
            {#if isParallel(i)}
              <span class="parallel-badge">parallel</span>
            {/if}
```

Add style:

```css
  .parallel-badge {
    font-size: 9px; padding: 1px 5px; border-radius: 3px;
    background: rgba(34,197,94,0.15); color: #22c55e;
    font-weight: 500; margin-left: 4px;
  }
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KanbanSubCards.svelte
git commit -m "feat(frontend): show parallel execution indicator on sub-cards"
```

---

### Task 6: Smoke test Phase 3

- [ ] **Step 1: Go vet + tests**

Run: `cd D:/repos/Multiterminal && go vet ./internal/backend/... && go test ./internal/backend/ -v 2>&1 | grep -E "PASS|FAIL|ok" | tail -5`
Expected: All PASS

- [ ] **Step 2: File sizes**

Run: `wc -l internal/backend/app_orchestrate_parallel.go internal/backend/app_orchestrate_exec.go internal/backend/app_orchestrate_memory.go`
Expected: All under 300 lines

- [ ] **Step 3: All commits**

Run: `git log --oneline feat/kanban-orchestration-v2 --not alpha-main | wc -l`
Expected: ~23 commits (Phase 1: 10, Phase 2: 7, Phase 3: 6)

---

## Summary

| Task | Files | What |
|------|-------|------|
| 1 | `app_orchestrate_parallel.go` + test | Wave grouping + parallel worktree execution |
| 2 | `app_orchestrate_exec.go` | Refactor execution to use waves |
| 3 | `app_orchestrate_memory.go` + test | Card memory persistence + pattern reuse loading |
| 4 | `app_orchestrate_exec.go` + `app_orchestrate_plan.go` | Wire memory into execution + plan generation |
| 5 | `KanbanSubCards.svelte` | Parallel indicator badge |
| 6 | — | Smoke test |

**After Phase 3:** Parallel steps run in isolated worktrees, results merge back automatically. Learnings persist to `docs/mtui/board/`. New plans get pattern reuse from completed cards. Sub-cards show parallel grouping.
