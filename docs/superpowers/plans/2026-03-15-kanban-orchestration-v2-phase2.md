# Kanban Agent Orchestration v2 — Phase 2: Execution + QA Loop

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When the user clicks "Starten" on a reviewed plan, sub-cards are created from plan steps, a headless execution loop works through them sequentially (with QA review after each), and the footer shows live progress.

**Architecture:** `ExecutePlan` creates sub-cards from `CardPlan.Steps`, then runs a background goroutine that executes each step via `RunHeadless`, runs a QA review after each, and emits events for real-time UI updates. The existing orchestrator infrastructure (`app_orchestrator.go`) is reused for session-based execution; the new headless execution is an alternative path for plan-driven work.

**Tech Stack:** Go 1.21+ (backend), Svelte 4 (frontend), Claude Code CLI (`claude -p`), Wails v3 events

**Spec:** `docs/superpowers/specs/2026-03-15-kanban-agent-orchestration-v2-design.md` (Phase 2 section)
**Phase 1 plan:** `docs/superpowers/plans/2026-03-15-kanban-orchestration-v2-phase1.md`

---

## File Structure

### New Files (Backend)
- `internal/backend/app_orchestrate_exec.go` — Plan execution loop: sub-card creation, sequential step execution via `RunHeadless`, state machine
- `internal/backend/app_orchestrate_exec_test.go` — Tests for sub-card creation and state transitions
- `internal/backend/app_orchestrate_qa.go` — QA review loop and anti-drift checkpoints via `RunHeadless`
- `internal/backend/app_orchestrate_qa_test.go` — Tests for QA prompt building and response parsing

### New Files (Frontend)
- `frontend/src/components/KanbanSubCards.svelte` — Collapsible sub-card list with status indicators

### Modified Files (Backend)
- `internal/backend/app.go` — Add `activeExecutions` map to AppService struct for tracking running plan executions

### Modified Files (Frontend)
- `frontend/src/components/KanbanCardDetail.svelte` — Wire `handlePlanStart` and `handleTerminalSwitch` to backend, add sub-cards display
- `frontend/src/components/KanbanCard.svelte` — Show sub-card progress indicator on parent cards
- `frontend/src/components/Footer.svelte` — Add execution progress badge
- `frontend/src/stores/kanban.ts` — Add helper for sub-card progress calculation
- `frontend/wailsjs/go/backend/App.js` — Add binding stubs for `ExecutePlan`, `CancelExecution`
- `frontend/wailsjs/go/backend/App.d.ts` — Add TypeScript declarations

---

## Chunk 1: Backend — Execution Engine + QA

### Task 1: Add execution tracking to AppService

**Files:**
- Modify: `internal/backend/app.go:28-53`

- [ ] **Step 1: Add activeExecutions field to AppService**

After `tmuxAPIPort` (line 52), add:

```go
	activeExecutions map[string]context.CancelFunc // card ID → cancel function
```

- [ ] **Step 2: Initialize the map in NewAppService**

In `NewAppService()`, add after the existing `make()` calls:

```go
		activeExecutions: make(map[string]context.CancelFunc),
```

- [ ] **Step 3: Verify compilation**

Run: `cd D:/repos/Multiterminal && go vet ./internal/backend/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/backend/app.go
git commit -m "feat(backend): add activeExecutions tracking to AppService"
```

---

### Task 2: Implement QA review and anti-drift

**Files:**
- Create: `internal/backend/app_orchestrate_qa.go`
- Create: `internal/backend/app_orchestrate_qa_test.go`

- [ ] **Step 1: Write tests for QA prompt builders and parsers**

Create `internal/backend/app_orchestrate_qa_test.go`:

```go
package backend

import (
	"encoding/json"
	"testing"
)

func TestParseQAResponse_Approved(t *testing.T) {
	raw := json.RawMessage(`{"approved":true,"issues":[],"suggestion":""}`)
	approved, issues, err := parseQAResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !approved {
		t.Error("expected approved=true")
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestParseQAResponse_Rejected(t *testing.T) {
	raw := json.RawMessage(`{"approved":false,"issues":["missing error handling","no test for edge case"],"suggestion":"add try-catch"}`)
	approved, issues, err := parseQAResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if approved {
		t.Error("expected approved=false")
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}
}

func TestParseQAResponse_MalformedJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	_, _, err := parseQAResponse(raw)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseDriftResponse_OnTrack(t *testing.T) {
	raw := json.RawMessage(`{"on_track":true,"drift":"","recommendation":"continue"}`)
	onTrack, _, rec, err := parseDriftResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !onTrack {
		t.Error("expected on_track=true")
	}
	if rec != "continue" {
		t.Errorf("recommendation=%q, want continue", rec)
	}
}

func TestParseDriftResponse_Drifted(t *testing.T) {
	raw := json.RawMessage(`{"on_track":false,"drift":"implementing wrong API version","recommendation":"stop"}`)
	onTrack, drift, rec, err := parseDriftResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if onTrack {
		t.Error("expected on_track=false")
	}
	if drift == "" {
		t.Error("expected drift description")
	}
	if rec != "stop" {
		t.Errorf("recommendation=%q, want stop", rec)
	}
}

func TestBuildQAPrompt(t *testing.T) {
	p := buildQAPrompt("Add auth endpoint", "Create /auth route", "diff --git ...")
	if p == "" {
		t.Error("prompt should not be empty")
	}
}

func TestBuildDriftPrompt(t *testing.T) {
	summaries := []string{"Created auth route", "Added JWT validation"}
	p := buildDriftPrompt("OAuth Login", `{"steps":[]}`, summaries)
	if p == "" {
		t.Error("prompt should not be empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestParseQA|TestParseDrift|TestBuildQA|TestBuildDrift" -v 2>&1 | tail -10`
Expected: FAIL (functions not defined)

- [ ] **Step 3: Implement app_orchestrate_qa.go**

Create `internal/backend/app_orchestrate_qa.go`:

```go
// Package backend provides QA review and anti-drift checks for plan execution.
package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// --- QA Review ---

func buildQAPrompt(cardTitle, stepDesc, diff string) string {
	// Truncate diff to avoid oversized prompts
	if len(diff) > 8000 {
		diff = diff[:8000] + "\n... (truncated)"
	}
	return fmt.Sprintf(`Review this code change against the requirements.

Original requirement: %s
Step requirement: %s

Diff:
%s

Does this change meet the requirement? Respond ONLY with JSON:
{"approved": true|false, "issues": ["issue1", "issue2"], "suggestion": "how to fix"}

Rules:
- approved=true only if the change fully meets the step requirement
- List specific issues if not approved
- Focus on functional correctness, not style`, cardTitle, stepDesc, diff)
}

func parseQAResponse(raw json.RawMessage) (bool, []string, error) {
	var resp struct {
		Approved   bool     `json:"approved"`
		Issues     []string `json:"issues"`
		Suggestion string   `json:"suggestion"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false, nil, fmt.Errorf("parse QA response: %w", err)
	}
	return resp.Approved, resp.Issues, nil
}

// reviewStep runs a QA review for a completed step via RunHeadless.
// Returns (approved, issues, error).
func (a *AppService) reviewStep(ctx context.Context, dir, cardTitle, stepDesc, diff string) (bool, []string, error) {
	prompt := buildQAPrompt(cardTitle, stepDesc, diff)
	result, err := a.RunHeadless(ctx, prompt, "", dir, 0)
	if err != nil {
		return false, nil, fmt.Errorf("QA review: %w", err)
	}
	return parseQAResponse(result.Data)
}

// --- Anti-Drift Checkpoint ---

func buildDriftPrompt(cardTitle, planJSON string, completedSummaries []string) string {
	summaries := strings.Join(completedSummaries, "\n- ")
	return fmt.Sprintf(`Compare completed work against the original plan.

Original issue: %s
Plan: %s

Completed steps:
- %s

Are we still on track? Any drift from the original requirements?
Respond ONLY with JSON:
{"on_track": true|false, "drift": "description if any", "recommendation": "continue|adjust|stop"}`, cardTitle, planJSON, summaries)
}

func parseDriftResponse(raw json.RawMessage) (bool, string, string, error) {
	var resp struct {
		OnTrack        bool   `json:"on_track"`
		Drift          string `json:"drift"`
		Recommendation string `json:"recommendation"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false, "", "", fmt.Errorf("parse drift response: %w", err)
	}
	return resp.OnTrack, resp.Drift, resp.Recommendation, nil
}

// checkDrift runs an anti-drift checkpoint after every 2nd completed step.
func (a *AppService) checkDrift(ctx context.Context, dir string, card *KanbanCard, completedSummaries []string) (bool, string, error) {
	planJSON, _ := json.Marshal(card.CardPlan)
	prompt := buildDriftPrompt(card.Title, string(planJSON), completedSummaries)

	result, err := a.RunHeadless(ctx, prompt, "", dir, 0)
	if err != nil {
		// Drift check is advisory — don't block execution on failure
		log.Printf("[orchestrate] drift check failed (non-fatal): %v", err)
		return true, "", nil
	}

	onTrack, drift, rec, err := parseDriftResponse(result.Data)
	if err != nil {
		log.Printf("[orchestrate] drift parse failed (non-fatal): %v", err)
		return true, "", nil
	}

	if !onTrack {
		log.Printf("[orchestrate] drift detected: %s (recommendation: %s)", drift, rec)
		a.app.Event.Emit("kanban:drift-detected", map[string]any{
			"card_id":        card.ID,
			"drift":          drift,
			"recommendation": rec,
		})
	}
	return onTrack, drift, nil
}

// getDiff returns the git diff for the working directory.
func getDiff(dir string) string {
	// Use git diff to capture changes made by the headless execution
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := runGitCmd(ctx, dir, "diff")
	if err != nil {
		log.Printf("[orchestrate] git diff failed: %v", err)
		return ""
	}
	return out
}

// runGitCmd executes a git command and returns stdout.
func runGitCmd(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := execCommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
```

**Note:** `execCommandContext` may need to use the COMSPEC pattern on Windows. Check if `exec.CommandContext` works for `git` directly (it should — git is not a .cmd shim). If not, wrap with COMSPEC.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestParseQA|TestParseDrift|TestBuildQA|TestBuildDrift" -v`
Expected: All PASS

- [ ] **Step 5: Verify compilation**

Run: `cd D:/repos/Multiterminal && go vet ./internal/backend/...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add internal/backend/app_orchestrate_qa.go internal/backend/app_orchestrate_qa_test.go
git commit -m "feat(backend): add QA review loop and anti-drift checkpoint via claude -p"
```

---

### Task 3: Implement plan execution engine

**Files:**
- Create: `internal/backend/app_orchestrate_exec.go`
- Create: `internal/backend/app_orchestrate_exec_test.go`

- [ ] **Step 1: Write tests for sub-card creation**

Create `internal/backend/app_orchestrate_exec_test.go`:

```go
package backend

import (
	"testing"
)

func TestCreateSubCards(t *testing.T) {
	plan := &CardPlan{
		Summary: "Add auth",
		Steps: []CardPlanStep{
			{ID: "1", Title: "Backend endpoint", Desc: "Create /auth"},
			{ID: "2", Title: "Frontend button", Desc: "Add login btn"},
			{ID: "3", Title: "Tests", Desc: "Write integration tests"},
		},
	}
	parentCard := &KanbanCard{ID: "parent-1", Title: "OAuth Login"}

	subCards := buildSubCards(parentCard, plan)
	if len(subCards) != 3 {
		t.Fatalf("expected 3 sub-cards, got %d", len(subCards))
	}
	for _, sc := range subCards {
		if sc.ParentCardID != "parent-1" {
			t.Errorf("parent_card_id=%q, want parent-1", sc.ParentCardID)
		}
		if sc.CardStatus != "pending" {
			t.Errorf("card_status=%q, want pending", sc.CardStatus)
		}
	}
	if subCards[0].Title != "Backend endpoint" {
		t.Errorf("title=%q, want 'Backend endpoint'", subCards[0].Title)
	}
}

func TestBuildStepExecutionPrompt(t *testing.T) {
	step := CardPlanStep{
		ID: "1", Title: "Create endpoint", Desc: "Build /auth route",
		Files: []string{"auth.go"},
	}
	p := buildStepExecutionPrompt("OAuth Login", "Add OAuth flow", &step, []string{"Setup done"}, "")
	if p == "" {
		t.Error("prompt should not be empty")
	}
}

func TestParseStepResult_Done(t *testing.T) {
	raw := []byte(`{"status":"done","summary":"Created /auth endpoint","files_changed":["auth.go"],"learnings":["API uses v2 format"]}`)
	result, err := parseStepResult(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if result.Status != "done" {
		t.Errorf("status=%q, want done", result.Status)
	}
	if len(result.FilesChanged) != 1 {
		t.Errorf("files_changed=%v, want 1 file", result.FilesChanged)
	}
}

func TestParseStepResult_Failed(t *testing.T) {
	raw := []byte(`{"status":"failed","summary":"Could not connect to DB","files_changed":[],"learnings":[]}`)
	result, err := parseStepResult(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if result.Status != "failed" {
		t.Error("expected failed status")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestCreateSub|TestBuildStep|TestParseStep" -v 2>&1 | tail -10`
Expected: FAIL

- [ ] **Step 3: Implement app_orchestrate_exec.go**

Create `internal/backend/app_orchestrate_exec.go`:

```go
// Package backend provides plan execution for Kanban cards.
package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// StepResult is the parsed response from a step execution.
type StepResult struct {
	Status       string   `json:"status"` // done|failed
	Summary      string   `json:"summary"`
	FilesChanged []string `json:"files_changed"`
	Learnings    []string `json:"learnings"`
}

// execCommandContext wraps exec.CommandContext (for testability and Windows compat).
var execCommandContext = exec.CommandContext

// buildSubCards creates sub-card structs from a plan's steps.
func buildSubCards(parent *KanbanCard, plan *CardPlan) []KanbanCard {
	cards := make([]KanbanCard, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		cards = append(cards, KanbanCard{
			ID:           fmt.Sprintf("%s-sub-%s", parent.ID, step.ID),
			Title:        step.Title,
			Prompt:       step.Desc,
			ParentCardID: parent.ID,
			CardStatus:   "pending",
			Dir:          parent.Dir,
		})
	}
	return cards
}

func buildStepExecutionPrompt(cardTitle, cardBody string, step *CardPlanStep, prevSummaries []string, agentDef string) string {
	prev := ""
	if len(prevSummaries) > 0 {
		prev = "\nPrevious steps completed:\n"
		for _, s := range prevSummaries {
			prev += "- " + s + "\n"
		}
	}
	files := ""
	if len(step.Files) > 0 {
		files = "\nFiles to modify: "
		for i, f := range step.Files {
			if i > 0 {
				files += ", "
			}
			files += f
		}
	}
	return fmt.Sprintf(`Execute this implementation step.

Original issue: %s
Issue description: %s

Current step: %s
Step description: %s
%s%s
Implement this step. After completion, respond ONLY with JSON:
{"status": "done|failed", "summary": "what was done", "files_changed": ["file1.go"], "learnings": ["insight1"]}

Rules:
- Only implement what this step requires — nothing more
- If blocked, respond with status "failed" and explain in summary
- List all files you created or modified`, cardTitle, cardBody, step.Title, step.Desc, files, prev)
}

func parseStepResult(raw []byte) (*StepResult, error) {
	var result StepResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse step result: %w", err)
	}
	if result.Status != "done" && result.Status != "failed" {
		return nil, fmt.Errorf("invalid status %q (expected done|failed)", result.Status)
	}
	return &result, nil
}

// ExecutePlan starts plan execution for a card. Creates sub-cards and runs
// the execution loop in a background goroutine.
func (a *AppService) ExecutePlan(dir, cardID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return err
	}
	card, _ := findCard(state, cardID)
	if card == nil {
		return fmt.Errorf("card %q not found", cardID)
	}
	if card.CardPlan == nil || len(card.CardPlan.Steps) == 0 {
		return fmt.Errorf("card %q has no plan", cardID)
	}

	// Create sub-cards
	subCards := buildSubCards(card, card.CardPlan)
	subIDs := make([]string, 0, len(subCards))
	for i := range subCards {
		sc := subCards[i]
		state.Columns[ColDefine] = append(state.Columns[ColDefine], sc)
		subIDs = append(subIDs, sc.ID)
	}
	card.SubCards = subIDs
	card.CardStatus = "in_progress"
	if err := saveCardUpdate(dir, state, card); err != nil {
		return err
	}
	if err := saveKanbanState(dir, state); err != nil {
		return err
	}

	a.app.Event.Emit("kanban:execution-started", map[string]any{
		"card_id":   cardID,
		"sub_cards": subIDs,
		"steps":     len(card.CardPlan.Steps),
	})

	// Start execution in background
	ctx, cancel := context.WithCancel(a.serviceCtx)
	a.mu.Lock()
	a.activeExecutions[cardID] = cancel
	a.mu.Unlock()

	go a.runPlanExecution(ctx, dir, cardID)
	return nil
}

// CancelExecution cancels a running plan execution.
func (a *AppService) CancelExecution(cardID string) {
	a.mu.Lock()
	cancel, ok := a.activeExecutions[cardID]
	a.mu.Unlock()
	if ok {
		cancel()
		log.Printf("[orchestrate] execution cancelled for card %s", cardID)
	}
}

// runPlanExecution is the main execution loop. Runs in a goroutine.
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
	coderPrompt := loadAgentDefinition(dir, "coder")
	completedCount := 0

	for i, step := range card.CardPlan.Steps {
		if ctx.Err() != nil {
			log.Printf("[orchestrate] execution cancelled at step %d", i+1)
			return
		}

		subCardID := card.SubCards[i]
		a.updateSubCardStatus(dir, subCardID, "in_progress")

		// Execute step
		prompt := buildStepExecutionPrompt(card.Title, card.Prompt, &step, completedSummaries, coderPrompt)
		stepCtx, stepCancel := context.WithTimeout(ctx, 300*time.Second)

		result, err := a.RunHeadless(stepCtx, prompt, coderPrompt, dir, 0)
		stepCancel()

		if err != nil {
			log.Printf("[orchestrate] step %d failed: %v", i+1, err)
			a.updateSubCardStatus(dir, subCardID, "fail")
			a.emitStepUpdate(cardID, subCardID, "fail", fmt.Sprintf("Execution error: %v", err))
			// Continue to next step (non-blocking failure for now)
			continue
		}

		stepResult, err := parseStepResult(result.Data)
		if err != nil || stepResult.Status == "failed" {
			msg := "parse error"
			if stepResult != nil {
				msg = stepResult.Summary
			}
			log.Printf("[orchestrate] step %d result: failed — %s", i+1, msg)
			a.updateSubCardStatus(dir, subCardID, "fail")
			a.emitStepUpdate(cardID, subCardID, "fail", msg)
			continue
		}

		// QA Review
		diff := getDiff(dir)
		qaCtx, qaCancel := context.WithTimeout(ctx, 120*time.Second)
		approved, issues, qaErr := a.reviewStep(qaCtx, dir, card.Title, step.Desc, diff)
		qaCancel()

		if qaErr != nil {
			log.Printf("[orchestrate] QA review error (non-fatal): %v", qaErr)
			approved = true // Don't block on QA failure
		}

		if !approved {
			log.Printf("[orchestrate] QA rejected step %d: %v", i+1, issues)
			// Try one correction
			correctionPrompt := fmt.Sprintf("Fix these QA issues with your previous change:\n%s",
				joinStrings(issues))
			corrCtx, corrCancel := context.WithTimeout(ctx, 300*time.Second)
			_, corrErr := a.RunHeadless(corrCtx, correctionPrompt, coderPrompt, dir, 0)
			corrCancel()
			if corrErr != nil {
				log.Printf("[orchestrate] correction failed: %v", corrErr)
				a.updateSubCardStatus(dir, subCardID, "fail")
				a.emitStepUpdate(cardID, subCardID, "fail", fmt.Sprintf("QA issues: %v", issues))
				continue
			}
		}

		// Step completed
		completedCount++
		completedSummaries = append(completedSummaries, stepResult.Summary)
		a.updateSubCardStatus(dir, subCardID, "finished")
		a.emitStepUpdate(cardID, subCardID, "finished", stepResult.Summary)

		// Anti-drift checkpoint every 2nd completed step
		if completedCount%2 == 0 && completedCount < len(card.CardPlan.Steps) {
			onTrack, drift, _ := a.checkDrift(ctx, dir, card, completedSummaries)
			if !onTrack {
				log.Printf("[orchestrate] drift detected, pausing: %s", drift)
				a.updateSubCardStatus(dir, cardID, "fail")
				a.emitStepUpdate(cardID, "", "drift", drift)
				return
			}
		}
	}

	// Execution complete
	finalStatus := "finished"
	if completedCount < len(card.CardPlan.Steps) {
		finalStatus = "fail"
	}
	a.updateParentCardStatus(dir, cardID, finalStatus)
	a.app.Event.Emit("kanban:execution-complete", map[string]any{
		"card_id":   cardID,
		"status":    finalStatus,
		"completed": completedCount,
		"total":     len(card.CardPlan.Steps),
	})
}

// --- Helpers ---

func (a *AppService) updateSubCardStatus(dir, subCardID, status string) {
	state, err := loadKanbanState(dir)
	if err != nil {
		return
	}
	card, _ := findCard(state, subCardID)
	if card == nil {
		return
	}
	card.CardStatus = status
	_ = saveCardUpdate(dir, state, card)
}

func (a *AppService) updateParentCardStatus(dir, cardID, status string) {
	state, err := loadKanbanState(dir)
	if err != nil {
		return
	}
	card, _ := findCard(state, cardID)
	if card == nil {
		return
	}
	card.CardStatus = status
	_ = saveCardUpdate(dir, state, card)
}

func (a *AppService) emitStepUpdate(cardID, subCardID, status, message string) {
	a.app.Event.Emit("kanban:step-update", map[string]any{
		"card_id":     cardID,
		"sub_card_id": subCardID,
		"status":      status,
		"message":     message,
	})
}

func loadAgentDefinition(dir, role string) string {
	data, err := readFileIfExists(dir, "docs", "mtui", "agents", role+".md")
	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

func readFileIfExists(parts ...string) ([]byte, error) {
	path := ""
	for _, p := range parts {
		if path == "" {
			path = p
		} else {
			path = path + "/" + p
		}
	}
	return os.ReadFile(path)
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += "\n"
		}
		result += "- " + s
	}
	return result
}
```

**Note:** Add `"os"` to the imports. The `readFileIfExists` helper uses `filepath.Join` — import `"path/filepath"` and use `filepath.Join(parts...)`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestCreateSub|TestBuildStep|TestParseStep" -v`
Expected: All PASS

- [ ] **Step 5: Verify compilation and file size**

Run: `cd D:/repos/Multiterminal && go vet ./internal/backend/... && wc -l internal/backend/app_orchestrate_exec.go`
Expected: No errors, under 300 lines

- [ ] **Step 6: Commit**

```bash
git add internal/backend/app_orchestrate_exec.go internal/backend/app_orchestrate_exec_test.go
git commit -m "feat(backend): add plan execution engine with sub-cards and QA loop"
```

---

## Chunk 2: Frontend — Sub-Cards + Footer Badge + Wiring

### Task 4: Create KanbanSubCards.svelte

**Files:**
- Create: `frontend/src/components/KanbanSubCards.svelte`

- [ ] **Step 1: Create the collapsible sub-card list component**

```svelte
<script lang="ts">
  import type { KanbanCard } from '../stores/kanban';

  export let parentCard: KanbanCard;
  export let allCards: KanbanCard[] = [];
  export let collapsed = true;

  $: subCards = findSubCards(parentCard, allCards);
  $: progress = calcProgress(subCards);

  function findSubCards(parent: KanbanCard, all: KanbanCard[]): KanbanCard[] {
    if (!parent.sub_cards?.length) return [];
    return parent.sub_cards
      .map(id => all.find(c => c.id === id))
      .filter((c): c is KanbanCard => !!c);
  }

  function calcProgress(cards: KanbanCard[]): { done: number; total: number; failed: number } {
    let done = 0, failed = 0;
    for (const c of cards) {
      if (c.card_status === 'finished') done++;
      if (c.card_status === 'fail') failed++;
    }
    return { done, total: cards.length, failed };
  }

  function statusIcon(status?: string): string {
    switch (status) {
      case 'finished': return '\u2713';
      case 'in_progress': return '\u231B';
      case 'fail': return '\u2717';
      default: return '\u25CB';
    }
  }

  function statusClass(status?: string): string {
    return `status-${status || 'pending'}`;
  }
</script>

{#if subCards.length > 0}
  <div class="sub-cards-section">
    <button class="toggle-btn" on:click={() => collapsed = !collapsed}>
      <span class="toggle-icon">{collapsed ? '\u25B6' : '\u25BC'}</span>
      <span class="progress-text">{progress.done}/{progress.total} Schritte</span>
      {#if progress.failed > 0}
        <span class="failed-count">{progress.failed} fehlgeschlagen</span>
      {/if}
      <div class="progress-mini">
        <div class="progress-fill" style="width: {progress.total > 0 ? (progress.done / progress.total) * 100 : 0}%"></div>
      </div>
    </button>

    {#if !collapsed}
      <div class="sub-card-list">
        {#each subCards as sc, i}
          <div class="sub-card-item {statusClass(sc.card_status)}">
            <span class="sub-card-icon {statusClass(sc.card_status)}">{statusIcon(sc.card_status)}</span>
            <span class="sub-card-num">{i + 1}</span>
            <span class="sub-card-title">{sc.title}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .sub-cards-section { margin-top: 8px; }
  .toggle-btn {
    display: flex; align-items: center; gap: 8px;
    background: none; border: 1px solid var(--border, #45475a);
    border-radius: 6px; padding: 6px 10px;
    color: var(--text-secondary, #a6adc8); cursor: pointer;
    font-size: 12px; width: 100%;
  }
  .toggle-btn:hover { border-color: var(--accent, #89b4fa); }
  .toggle-icon { font-size: 10px; width: 12px; }
  .progress-text { font-weight: 500; }
  .failed-count { color: #ef4444; font-size: 11px; }
  .progress-mini {
    flex: 1; height: 3px; background: var(--bg-primary, #181825);
    border-radius: 2px; overflow: hidden; margin-left: 4px;
  }
  .progress-fill {
    height: 100%; background: var(--accent, #89b4fa);
    border-radius: 2px; transition: width 0.3s ease;
  }
  .sub-card-list {
    display: flex; flex-direction: column; gap: 2px;
    margin-top: 4px; padding-left: 4px;
  }
  .sub-card-item {
    display: flex; align-items: center; gap: 8px;
    padding: 4px 8px; border-radius: 4px; font-size: 12px;
  }
  .sub-card-icon { width: 14px; text-align: center; font-size: 11px; }
  .sub-card-icon.status-finished { color: #22c55e; }
  .sub-card-icon.status-in_progress { color: #f59e0b; }
  .sub-card-icon.status-fail { color: #ef4444; }
  .sub-card-icon.status-pending { color: var(--text-muted, #6c7086); }
  .sub-card-num { color: var(--text-muted, #6c7086); font-size: 11px; min-width: 16px; }
  .sub-card-title { color: var(--text-primary, #cdd6f4); }
</style>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KanbanSubCards.svelte
git commit -m "feat(frontend): add KanbanSubCards component with collapsible progress view"
```

---

### Task 5: Add Wails binding stubs for ExecutePlan and CancelExecution

**Files:**
- Modify: `frontend/wailsjs/go/backend/App.js`
- Modify: `frontend/wailsjs/go/backend/App.d.ts`

- [ ] **Step 1: Add JS binding stubs**

Append to `App.js`:

```javascript
export function ExecutePlan(arg1, arg2) {
    return $Call.ByName("backend.AppService.ExecutePlan", arg1, arg2);
}

export function CancelExecution(arg1) {
    return $Call.ByName("backend.AppService.CancelExecution", arg1);
}
```

- [ ] **Step 2: Add TypeScript declarations**

Append to `App.d.ts`:

```typescript
export function ExecutePlan(arg1:string,arg2:string):Promise<void>;
export function CancelExecution(arg1:string):Promise<void>;
```

- [ ] **Step 3: Commit**

```bash
git add frontend/wailsjs/go/backend/App.js frontend/wailsjs/go/backend/App.d.ts
git commit -m "feat(bindings): add Wails stubs for ExecutePlan and CancelExecution"
```

---

### Task 6: Wire execution into KanbanCardDetail + add sub-cards display

**Files:**
- Modify: `frontend/src/components/KanbanCardDetail.svelte`

- [ ] **Step 1: Add imports and event listener**

Add import at top of script:

```typescript
import KanbanSubCards from './KanbanSubCards.svelte';
```

- [ ] **Step 2: Implement handlePlanStart**

Replace the stub `handlePlanStart` function with:

```typescript
  async function handlePlanStart() {
    if (!card || !dir) return;
    try {
      await App.ExecutePlan(dir, card.id);
      card = { ...card, card_status: 'in_progress' };
      dispatch('updated', { card });
    } catch (e) {
      console.error('Plan execution failed:', e);
    }
  }
```

- [ ] **Step 3: Add sub-cards section in template**

After the `<KanbanPlanReview>` component and before the labels section, add:

```svelte
        <!-- Sub-Cards progress -->
        {#if card?.sub_cards?.length}
          <KanbanSubCards
            parentCard={card}
            allCards={getAllCards()}
          />
        {/if}
```

Add the helper function in the script block:

```typescript
  function getAllCards(): KanbanCard[] {
    // Flatten all columns to find sub-cards
    // This is called reactively when card changes
    return []; // Will be populated via event updates from backend
  }
```

**Note:** A more robust approach is to pass sub-cards directly. Since the board state is in the kanban store, the implementer should import the store and derive sub-cards from it. Use:

```typescript
  import { kanban } from '../stores/kanban';

  $: allCardsFlat = Object.values($kanban.state.columns).flat();
```

Then pass `allCards={allCardsFlat}` to KanbanSubCards.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/KanbanCardDetail.svelte
git commit -m "feat(frontend): wire plan execution and sub-cards display into card detail"
```

---

### Task 7: Add execution progress badge to Footer

**Files:**
- Modify: `frontend/src/components/Footer.svelte`
- Modify: `frontend/src/stores/kanban.ts`

- [ ] **Step 1: Add derived store for active executions**

In `frontend/src/stores/kanban.ts`, add after the `parentIssueProgress` derived store:

```typescript
/** Derived: active card executions (cards with card_status=in_progress and sub_cards) */
export const activeExecutions = derived(kanban, $k => {
  const executions: Array<{ card: KanbanCard; done: number; total: number; cost: number }> = [];
  for (const cards of Object.values($k.state.columns)) {
    for (const card of cards) {
      if (card.card_status === 'in_progress' && card.sub_cards?.length) {
        let done = 0;
        for (const subId of card.sub_cards) {
          for (const col of Object.values($k.state.columns)) {
            const sub = col.find(c => c.id === subId);
            if (sub?.card_status === 'finished') done++;
          }
        }
        executions.push({ card, done, total: card.sub_cards.length, cost: card.cost || 0 });
      }
    }
  }
  return executions;
});
```

- [ ] **Step 2: Add badge to Footer.svelte**

Import the derived store and display a badge. In the Footer script:

```typescript
import { activeExecutions } from '../stores/kanban';
```

In the Footer template, in the center section (between branch and shortcuts), add:

```svelte
{#if $activeExecutions.length > 0}
  {@const exec = $activeExecutions[0]}
  <span class="exec-badge" title="Klicken für Board-Ansicht">
    {exec.card.title.substring(0, 25)}: {exec.done}/{exec.total}
    {#if exec.cost > 0}
      · ${exec.cost.toFixed(2)}
    {/if}
  </span>
{/if}
```

Add the style:

```css
.exec-badge {
  font-size: 0.72rem;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(137, 180, 250, 0.15);
  color: #89b4fa;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 250px;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/kanban.ts frontend/src/components/Footer.svelte
git commit -m "feat(frontend): add execution progress badge to footer and kanban store"
```

---

### Task 8: Add event listeners for real-time sub-card updates

**Files:**
- Modify: `frontend/src/components/KanbanBoard.svelte`

- [ ] **Step 1: Add Wails event listeners for execution events**

In the KanbanBoard's `onMount` or initialization section, add event listeners that refresh the board state when execution events arrive:

```typescript
import { Events } from '@anthropic-ai/sdk/wails'; // or however Wails v3 events are imported

// Listen for step updates and refresh board
const stepListener = Events.On('kanban:step-update', async () => {
  if (dir) {
    const state = await App.GetKanbanState(dir);
    kanban.setState(state);
  }
});

const execListener = Events.On('kanban:execution-complete', async () => {
  if (dir) {
    const state = await App.GetKanbanState(dir);
    kanban.setState(state);
  }
});
```

**Note:** The implementer should check how Wails v3 events are listened to in the existing codebase. Look for existing `Events.On` or `WailsEvent` patterns in the KanbanBoard or other components. Follow the same pattern.

Clean up listeners in `onDestroy`.

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KanbanBoard.svelte
git commit -m "feat(frontend): add event listeners for real-time execution progress updates"
```

---

### Task 9: End-to-end smoke test

- [ ] **Step 1: Verify Go backend compiles and tests pass**

Run: `cd D:/repos/Multiterminal && go vet ./internal/backend/... && go test ./internal/backend/ -v -run "TestParseQA|TestParseDrift|TestBuildQA|TestBuildDrift|TestCreateSub|TestBuildStep|TestParseStep" 2>&1 | tail -20`
Expected: All tests PASS

- [ ] **Step 2: Verify file sizes**

Run: `wc -l internal/backend/app_orchestrate_exec.go internal/backend/app_orchestrate_qa.go`
Expected: Both under 300 lines

- [ ] **Step 3: Check all commits**

Run: `git log --oneline feat/kanban-orchestration-v2 --not alpha-main`
Expected: Phase 1 + Phase 2 commits visible

---

## Summary

| Task | Files | What |
|------|-------|------|
| 1 | `app.go` | Add activeExecutions tracking |
| 2 | `app_orchestrate_qa.go` + test | QA review + anti-drift via claude -p |
| 3 | `app_orchestrate_exec.go` + test | Plan execution engine + sub-cards |
| 4 | `KanbanSubCards.svelte` | Collapsible sub-card progress view |
| 5 | `App.js` + `App.d.ts` | Wails binding stubs |
| 6 | `KanbanCardDetail.svelte` | Wire execution + sub-cards display |
| 7 | `Footer.svelte` + `kanban.ts` | Execution progress badge |
| 8 | `KanbanBoard.svelte` | Real-time event listeners |
| 9 | — | Smoke test |

**After Phase 2:** User can click "Starten" → sub-cards are created → headless execution works through each step → QA review after each → anti-drift every 2nd step → footer shows live progress → sub-cards show status inline.
