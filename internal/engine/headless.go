package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// Compile-time interface check.
var _ ExecutionEngine = (*HeadlessEngine)(nil)

// HeadlessEngine executes steps via `claude -p --output-format json` in isolated worktrees.
type HeadlessEngine struct {
	mu         sync.Mutex
	slots      *WorktreeSlotManager
	stepDetect *StepLoopDetector
	repoDetect *RepoLoopDetector
	checkpoint *CheckpointGuard
	repoDir    string
	cancels    map[string]context.CancelFunc // stepID → cancel
}

// NewHeadlessEngine creates a HeadlessEngine for the given repo directory.
func NewHeadlessEngine(repoDir string, maxSlots int) *HeadlessEngine {
	return &HeadlessEngine{
		slots:      NewWorktreeSlotManager(repoDir, maxSlots),
		stepDetect: NewStepLoopDetector(),
		repoDetect: NewRepoLoopDetector(),
		checkpoint: NewCheckpointGuard(3),
		repoDir:    repoDir,
		cancels:    make(map[string]context.CancelFunc),
	}
}

// Execute runs a step in an isolated worktree using claude -p.
func (e *HeadlessEngine) Execute(ctx context.Context, req orchestrator.ExecutionRequest) (orchestrator.ExecutionResult, error) {
	start := time.Now()
	result := orchestrator.ExecutionResult{StepID: req.StepID}

	// 1. Determine work directory.
	// WorktreeSlot == -1 means "run in repo dir" (for triage, planning — no file changes).
	// WorktreeSlot >= 0 means "allocate an isolated worktree" (for code execution).
	var workDir string
	var slotID int = -1
	if req.WorktreeSlot >= 0 && req.CardID != "" && req.StepID != "" &&
		req.StepID != "triage" && req.StepID != "plan" &&
		!strings.HasPrefix(req.StepID, "plan-repair") &&
		!strings.HasPrefix(req.StepID, "replan") {
		branch := fmt.Sprintf("mtui/%s/%s", req.CardID, req.StepID)
		var err error
		slotID, workDir, err = e.slots.Allocate(ctx, branch)
		if err != nil {
			return result, fmt.Errorf("allocate worktree: %w", err)
		}
		defer func() {
			_ = e.slots.Release(slotID)
		}()
	} else {
		workDir = e.repoDir
	}

	// 2. Create cancellable context with timeout.
	timeout := time.Duration(req.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute // default
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	e.mu.Lock()
	e.cancels[req.StepID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, req.StepID)
		e.mu.Unlock()
	}()

	// 3. Run claude -p in the worktree.
	log.Printf("[headless] step=%s running claude -p in %s", req.StepID, workDir)
	claudeOut, err := runClaude(execCtx, workDir, req.Prompt, req.SystemPrompt, req.Model)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Status = orchestrator.StepTimeout
			result.Error = &orchestrator.StepError{Class: "timeout", Message: "claude -p timed out"}
			result.DurationSec = int(time.Since(start).Seconds())
			return result, nil
		}
		return result, fmt.Errorf("claude -p: %w", err)
	}

	// 4. Parse claude output for cost info.
	result.CostUSD = parseCostFromOutput(claudeOut)

	// 5. Run verify commands in the worktree.
	if len(req.Verify) > 0 {
		result.Verify = runVerify(execCtx, workDir, req.Verify)
	}

	// 6. Collect files_changed via git diff --name-only.
	files, err := getFilesChanged(execCtx, workDir)
	if err == nil {
		result.FilesChanged = files
	}

	// 7. Run loop detection.
	diffLines := countDiffLines(execCtx, workDir)
	e.stepDetect.Record(result.Verify, diffLines)
	loopSignals := e.stepDetect.Detect()
	repoSignals := e.repoDetect.Detect(execCtx, workDir)
	result.LoopSignals = append(loopSignals, repoSignals...)

	// 8. Determine status.
	result.Status = determineStatus(result.Verify, result.LoopSignals)
	if result.Status == orchestrator.StepFailed {
		result.Error = &orchestrator.StepError{
			Class:   "test",
			Message: summarizeFailures(result.Verify),
		}
	}

	result.DurationSec = int(time.Since(start).Seconds())
	return result, nil
}

// Cancel stops an active execution for the given stepID.
func (e *HeadlessEngine) Cancel(stepID string) error {
	e.mu.Lock()
	cancel, ok := e.cancels[stepID]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active execution for step %s", stepID)
	}
	cancel()
	return nil
}

// Slots returns the underlying WorktreeSlotManager for inspection.
func (e *HeadlessEngine) Slots() *WorktreeSlotManager {
	return e.slots
}

// claudeOutputJSON is the minimal structure we parse from claude --output-format json.
type claudeOutputJSON struct {
	CostUSD float64 `json:"cost_usd"`
	Result  string  `json:"result"`
}

// parseCostFromOutput attempts to extract cost from claude JSON output.
func parseCostFromOutput(output []byte) float64 {
	var parsed claudeOutputJSON
	if err := json.Unmarshal(output, &parsed); err != nil {
		return 0
	}
	return parsed.CostUSD
}

// determineStatus computes the step status from verify results and loop signals.
func determineStatus(verify []orchestrator.VerifyResult, loops []orchestrator.LoopSignal) orchestrator.StepStatus {
	if len(loops) > 0 {
		return orchestrator.StepStuck
	}
	for _, v := range verify {
		if !v.Passed {
			return orchestrator.StepFailed
		}
	}
	return orchestrator.StepSuccess
}

// summarizeFailures builds a short message from failed verify results.
func summarizeFailures(verify []orchestrator.VerifyResult) string {
	var parts []string
	for _, v := range verify {
		if !v.Passed {
			msg := v.Command
			if v.Description != "" {
				msg = v.Description
			}
			parts = append(parts, fmt.Sprintf("%s (exit %d)", msg, v.ExitCode))
		}
	}
	return strings.Join(parts, "; ")
}

// countDiffLines returns the total number of changed lines (added+deleted) in the worktree.
func countDiffLines(ctx context.Context, workDir string) int {
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return 0
	}
	// The last line of --stat is like " 3 files changed, 10 insertions(+), 5 deletions(-)"
	// Just count output lines as an approximation.
	return len(lines)
}

