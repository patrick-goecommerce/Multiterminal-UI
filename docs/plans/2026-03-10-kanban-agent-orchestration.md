# Kanban-Driven Agent Orchestration — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform Multiterminal's Kanban board into an autonomous agent orchestrator that decomposes GitHub Issues into sub-tickets, assigns them to Claude Code agents in isolated Git Worktrees, and manages the full lifecycle through automated review and PR creation.

**Architecture:** Extend the existing orchestrator (`app_orchestrator.go`) to manage a 7-column Kanban workflow (Define → Refine → Approved → Ready → In Progress → Auto Review → Done). Each agent runs in a dedicated Git Worktree. A review pipeline (tests + review-agent + PR) validates work before completion.

**Tech Stack:** Go 1.21+ (backend), Svelte 4 (frontend), Git Worktrees, `gh` CLI, existing PTY/Queue infrastructure.

**Design Doc:** `docs/plans/2026-03-10-kanban-agent-orchestration-design.md`

---

## Task 1: Extend Config with Orchestrator Settings

**Files:**
- Modify: `internal/config/config.go` (add `Orchestrator` field to `Config` struct)

**Step 1: Add OrchestratorSettings struct and field to Config**

In `config.go`, add after the `BackgroundAgents` struct definition:

```go
type OrchestratorSettings struct {
	MaxParallelAgents    int    `yaml:"max_parallel_agents" json:"max_parallel_agents"`
	DefaultAutoMerge     bool   `yaml:"default_auto_merge" json:"default_auto_merge"`
	DefaultAutoStart     bool   `yaml:"default_auto_start" json:"default_auto_start"`
	MaxRetries           int    `yaml:"max_retries" json:"max_retries"`
	ReviewCommand        string `yaml:"review_command" json:"review_command"`
	SyncSubtasksToGitHub bool   `yaml:"sync_subtasks_to_github" json:"sync_subtasks_to_github"`
}
```

Add to `Config` struct:

```go
Orchestrator OrchestratorSettings `yaml:"orchestrator" json:"orchestrator"`
```

**Step 2: Set defaults in `DefaultConfig()`**

```go
Orchestrator: OrchestratorSettings{
    MaxParallelAgents:    3,
    DefaultAutoMerge:     false,
    DefaultAutoStart:     false,
    MaxRetries:           2,
    ReviewCommand:        "go test ./... && go vet ./...",
    SyncSubtasksToGitHub: false,
},
```

**Step 3: Update frontend models.ts**

Add `OrchestratorSettings` class to `frontend/wailsjs/go/models.ts` and add the `orchestrator` field to the `Config` class constructor.

**Step 4: Commit**

```bash
git add internal/config/config.go frontend/wailsjs/go/models.ts
git commit -m "feat: add orchestrator settings to config"
```

---

## Task 2: Extend KanbanCard with Agent Orchestration Fields

**Files:**
- Modify: `internal/backend/app_kanban.go` (extend `KanbanCard` struct, update column constants)

**Step 1: Update column constants**

Replace the existing 5 column constants with 7:

```go
const (
	ColDefine     = "define"
	ColRefine     = "refine"
	ColApproved   = "approved"
	ColReady      = "ready"
	ColInProgress = "in_progress"
	ColAutoReview = "auto_review"
	ColDone       = "done"
)
```

Update `newKanbanState()` to initialize all 7 columns. Keep backward compatibility: if loading an old state with 5 columns, migrate `backlog` → `define`, `planned` → `approved`, `review` → `auto_review`.

**Step 2: Extend KanbanCard struct**

Add new fields after existing ones:

```go
type KanbanCard struct {
	// Existing fields
	ID           string   `json:"id" yaml:"id"`
	IssueNumber  int      `json:"issue_number" yaml:"issue_number"`
	Title        string   `json:"title" yaml:"title"`
	Labels       []string `json:"labels" yaml:"labels"`
	Dir          string   `json:"dir" yaml:"dir"`
	SessionID    int      `json:"session_id" yaml:"session_id"`
	Priority     int      `json:"priority" yaml:"priority"`
	Dependencies []string `json:"dependencies" yaml:"dependencies"`
	PlanID       string   `json:"plan_id" yaml:"plan_id"`
	ScheduleID   string   `json:"schedule_id" yaml:"schedule_id"`
	CreatedAt    string   `json:"created_at" yaml:"created_at"`

	// Agent orchestration fields
	ParentIssue    int    `json:"parent_issue" yaml:"parent_issue"`
	Prompt         string `json:"prompt" yaml:"prompt"`
	AutoMerge      bool   `json:"auto_merge" yaml:"auto_merge"`
	AutoStart      bool   `json:"auto_start" yaml:"auto_start"`
	WorktreePath   string `json:"worktree_path" yaml:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch" yaml:"worktree_branch"`
	AgentSessionID int    `json:"agent_session_id" yaml:"agent_session_id"`
	ReviewResult   string `json:"review_result" yaml:"review_result"`
	PRNumber       int    `json:"pr_number" yaml:"pr_number"`
	RetryCount     int    `json:"retry_count" yaml:"retry_count"`
	MaxRetries     int    `json:"max_retries" yaml:"max_retries"`
}
```

**Step 3: Add migration logic in `loadKanbanState()`**

After loading, check for old column names and migrate:

```go
func migrateColumns(state *KanbanState) {
	migrations := map[string]string{
		"backlog": ColDefine,
		"planned": ColApproved,
		"review":  ColAutoReview,
	}
	for old, new := range migrations {
		if cards, ok := state.Columns[old]; ok && len(cards) > 0 {
			state.Columns[new] = append(state.Columns[new], cards...)
			delete(state.Columns, old)
		}
	}
}
```

**Step 4: Update frontend models.ts**

Update the `KanbanCard` class in `models.ts` with all new fields.

**Step 5: Commit**

```bash
git add internal/backend/app_kanban.go frontend/wailsjs/go/models.ts
git commit -m "feat: extend KanbanCard with agent orchestration fields, 7 columns"
```

---

## Task 3: Update Frontend Kanban Store for 7 Columns

**Files:**
- Modify: `frontend/src/stores/kanban.ts`

**Step 1: Update COLUMN_IDS, labels, and colors**

```typescript
export const COLUMN_IDS = [
  'define', 'refine', 'approved', 'ready',
  'in_progress', 'auto_review', 'done'
] as const;

export type ColumnID = typeof COLUMN_IDS[number];

export const COLUMN_LABELS: Record<ColumnID, string> = {
  define: 'Definieren',
  refine: 'Verfeinern',
  approved: 'Genehmigt',
  ready: 'Bereit',
  in_progress: 'In Arbeit',
  auto_review: 'Auto-Review',
  done: 'Erledigt',
};

export const COLUMN_COLORS: Record<ColumnID, string> = {
  define: '#9ca3af',
  refine: '#f59e0b',
  approved: '#8b5cf6',
  ready: '#3b82f6',
  in_progress: '#f97316',
  auto_review: '#06b6d4',
  done: '#22c55e',
};
```

**Step 2: Extend KanbanCard interface**

Add new fields to match the Go struct:

```typescript
export interface KanbanCard {
  // ... existing fields ...
  parent_issue: number;
  prompt: string;
  auto_merge: boolean;
  auto_start: boolean;
  worktree_path: string;
  worktree_branch: string;
  agent_session_id: number;
  review_result: string;
  pr_number: number;
  retry_count: number;
  max_retries: number;
}
```

**Step 3: Update default state initialization**

Ensure `createKanbanStore()` initializes all 7 columns in the default state.

**Step 4: Commit**

```bash
git add frontend/src/stores/kanban.ts
git commit -m "feat: update kanban store for 7 columns and extended card fields"
```

---

## Task 4: Update KanbanBoard.svelte for 7 Columns

**Files:**
- Modify: `frontend/src/components/KanbanBoard.svelte`

**Step 1: Update grid layout**

Change CSS `grid-template-columns` from `repeat(5, 1fr)` to `repeat(7, 1fr)`. Use the `COLUMN_IDS` import instead of hardcoded array.

**Step 2: Update column iteration**

Replace hardcoded column references with `COLUMN_IDS` loop:

```svelte
{#each COLUMN_IDS as colId}
  <KanbanColumn
    columnId={colId}
    cards={$kanban.state.columns[colId] || []}
    on:drop={handleDrop}
    on:cardClick={handleCardClick}
    on:cardDragStart={handleCardDragStart}
  />
{/each}
```

**Step 3: Commit**

```bash
git add frontend/src/components/KanbanBoard.svelte
git commit -m "feat: update KanbanBoard layout for 7 columns"
```

---

## Task 5: Update KanbanCard.svelte with Agent Status

**Files:**
- Modify: `frontend/src/components/KanbanCard.svelte`

**Step 1: Add agent status indicators**

Show additional info on cards:
- Parent issue link: `↑#N` if `parent_issue > 0`
- Auto-merge badge: green `⚡` if `auto_merge`
- Worktree branch: small text showing `worktree_branch` if set
- PR link: `PR #N` if `pr_number > 0`
- Review result: colored dot (green=pass, red=fail, gray=pending)
- Retry count: `↻ N/M` if `retry_count > 0`

**Step 2: Add color coding**

Cards with `auto_merge: true` get a subtle green left border. Cards with `review_result: "fail"` get a red left border.

**Step 3: Commit**

```bash
git add frontend/src/components/KanbanCard.svelte
git commit -m "feat: add agent status indicators to KanbanCard"
```

---

## Task 6: Rewrite Orchestrator — Scheduler Loop

**Files:**
- Modify: `internal/backend/app_orchestrator.go` (rewrite `runOrchestrator`)
- Create: `internal/backend/app_orchestrator_review.go` (review pipeline)

**Step 1: Add new orchestrator state fields**

Update `orchestratorState` to track the new workflow:

```go
type orchestratorState struct {
	mu      sync.Mutex
	planID  string
	dir     string
	running map[string]int    // cardID → sessionID (in_progress agents)
	review  map[string]int    // cardID → review sessionID (auto_review)
	done    map[string]bool
	cancel  chan struct{}
	active  bool
	cfg     config.OrchestratorSettings
}
```

**Step 2: Rewrite `runOrchestrator` with new scheduler logic**

The main loop (every 5 seconds):

```
1. Check "ready" cards with fulfilled dependencies
2. If running < max_parallel_agents:
   a. Create worktree for card
   b. Spawn Claude session with prompt
   c. Link session to parent issue
   d. Move card → "in_progress"
3. Check "in_progress" cards for activity "done":
   a. Move card → "auto_review"
   b. Start review pipeline (Task 7)
4. Check "auto_review" cards for review completion:
   a. Passed → create PR, move → "done", auto-merge if flagged
   b. Failed + retries left → feed error back, move → "in_progress"
   c. Failed + no retries → notify user, keep in "auto_review"
5. Check if all sub-tickets of parent issue are "done" → close parent
```

**Step 3: Implement worktree creation for agents**

```go
func (a *AppService) createAgentWorktree(card *KanbanCard, dir string) error {
	slug := sanitizeWorktreeName(card.Title)
	name := fmt.Sprintf("agent-%s-%s", card.ID, slug)
	wt, err := a.CreateNamedWorktree(dir, name, "")
	if err != nil {
		return err
	}
	card.WorktreePath = wt.Path
	card.WorktreeBranch = wt.Branch
	return nil
}
```

**Step 4: Implement agent session spawning**

```go
func (a *AppService) spawnAgent(orch *orchestratorState, card *KanbanCard, state *KanbanState) error {
	// Create worktree
	if err := a.createAgentWorktree(card, orch.dir); err != nil {
		return err
	}
	// Spawn Claude session in worktree directory
	sessionID := a.CreateSession([]string{"claude"}, card.WorktreePath, 24, 80, "claude")
	card.AgentSessionID = sessionID
	orch.mu.Lock()
	orch.running[card.ID] = sessionID
	orch.mu.Unlock()
	// Send prompt after 2s delay
	go func() {
		time.Sleep(2 * time.Second)
		a.AddToQueue(sessionID, card.Prompt)
	}()
	// Link to parent issue for progress reporting
	if card.ParentIssue > 0 {
		a.LinkSessionIssue(sessionID, card.ParentIssue, card.Title, card.WorktreeBranch, orch.dir)
	}
	// Move card
	a.moveCardToColumn(state, card.ID, ColInProgress)
	return nil
}
```

**Step 5: Commit**

```bash
git add internal/backend/app_orchestrator.go
git commit -m "feat: rewrite orchestrator scheduler for agent worktree workflow"
```

---

## Task 7: Review Pipeline

**Files:**
- Create: `internal/backend/app_orchestrator_review.go`

**Step 1: Implement test runner**

```go
func (a *AppService) runReviewTests(card *KanbanCard, reviewCmd string) (bool, string) {
	// Run review_command in the worktree directory
	cmd := exec.Command(os.Getenv("COMSPEC"), "/c", reviewCmd)
	cmd.Dir = card.WorktreePath
	hideConsole(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(output)
	}
	return true, string(output)
}
```

**Step 2: Implement review agent spawning**

```go
func (a *AppService) spawnReviewAgent(card *KanbanCard, dir string) int {
	// Get diff between agent branch and base
	diff := a.getWorktreeDiff(card.WorktreePath)
	prompt := fmt.Sprintf(
		"Review this diff for code quality, security issues, and adherence to CLAUDE.md rules.\n"+
			"Respond with PASS or FAIL followed by findings.\n\nDiff:\n%s", diff)
	sessionID := a.CreateSession([]string{"claude"}, dir, 24, 80, "claude")
	go func() {
		time.Sleep(2 * time.Second)
		a.AddToQueue(sessionID, prompt)
	}()
	return sessionID
}
```

**Step 3: Implement PR creation**

```go
func (a *AppService) createPRForCard(card *KanbanCard, dir string) (int, error) {
	title := fmt.Sprintf("agent/%s: %s", card.ID, card.Title)
	body := fmt.Sprintf("Closes sub-ticket %s\nParent issue: #%d", card.ID, card.ParentIssue)
	cmd := exec.Command("gh", "pr", "create",
		"--head", card.WorktreeBranch,
		"--title", title,
		"--body", body)
	cmd.Dir = dir
	hideConsole(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pr create failed: %s", string(output))
	}
	// Parse PR number from URL
	prNumber := parsePRNumber(string(output))
	card.PRNumber = prNumber
	return prNumber, nil
}
```

**Step 4: Implement auto-merge**

```go
func (a *AppService) autoMergeCard(card *KanbanCard, dir string) error {
	cmd := exec.Command("gh", "pr", "merge", fmt.Sprintf("%d", card.PRNumber),
		"--squash", "--delete-branch")
	cmd.Dir = dir
	hideConsole(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge failed: %s", string(output))
	}
	// Cleanup worktree
	return a.removeAgentWorktree(card)
}
```

**Step 5: Implement parent issue closing**

```go
func (a *AppService) checkAndCloseParentIssue(dir string, parentIssue int, state *KanbanState) {
	if parentIssue <= 0 {
		return
	}
	// Check if ALL sub-tickets with this parent are done
	allDone := true
	for _, col := range []string{ColDefine, ColRefine, ColApproved, ColReady, ColInProgress, ColAutoReview} {
		for _, c := range state.Columns[col] {
			if c.ParentIssue == parentIssue {
				allDone = false
				break
			}
		}
	}
	if allDone {
		a.UpdateIssue(dir, parentIssue, "", "", "closed")
		a.AddIssueComment(dir, parentIssue,
			"**Multiterminal Agent Orchestrator**\nAlle Sub-Tickets abgeschlossen. Issue wird geschlossen.")
	}
}
```

**Step 6: Commit**

```bash
git add internal/backend/app_orchestrator_review.go
git commit -m "feat: add review pipeline (tests, review-agent, PR, auto-merge)"
```

---

## Task 8: Plan-to-SubTicket Generation

**Files:**
- Modify: `internal/backend/app_kanban_plan.go`

**Step 1: Add `GenerateSubTickets` method**

When a plan is approved, generate sub-tickets as KanbanCards with `parent_issue` reference:

```go
func (a *AppService) GenerateSubTickets(dir string, planID string, parentIssue int) error {
	state := loadKanbanState(dir)
	plan := findPlan(state, planID)
	if plan == nil {
		return fmt.Errorf("plan not found: %s", planID)
	}
	cfg := a.cfg.Orchestrator
	for _, step := range plan.Steps {
		card := KanbanCard{
			ID:          generateID(),
			Title:       step.Title,
			Prompt:      step.Prompt,
			ParentIssue: parentIssue,
			PlanID:      planID,
			Priority:    step.Order,
			AutoMerge:   cfg.DefaultAutoMerge,
			AutoStart:   cfg.DefaultAutoStart,
			MaxRetries:  cfg.MaxRetries,
			CreatedAt:   time.Now().Format(time.RFC3339),
		}
		// Copy dependencies from step
		if step.CardID != "" {
			card.Dependencies = append(card.Dependencies, step.CardID)
		}
		targetCol := ColApproved
		if card.AutoStart {
			targetCol = ColReady
		}
		state.Columns[targetCol] = append(state.Columns[targetCol], card)
		// Update step with new card ID
		step.CardID = card.ID
	}
	return saveKanbanState(dir, state)
}
```

**Step 2: Update `ApprovePlan` to call `GenerateSubTickets`**

Modify `ApprovePlan` to accept a `parentIssue` parameter and delegate to `GenerateSubTickets`.

**Step 3: Commit**

```bash
git add internal/backend/app_kanban_plan.go
git commit -m "feat: generate sub-tickets from approved plans"
```

---

## Task 9: Orchestrator Events and Frontend Integration

**Files:**
- Modify: `frontend/src/components/KanbanBoard.svelte` (listen to orchestrator events)
- Modify: `frontend/src/stores/kanban.ts` (add event handling)

**Step 1: Add event listeners for orchestrator updates**

In `KanbanBoard.svelte`, listen to new events:

```typescript
import { Events } from '@anthropic-ai/wailsjs/runtime';

onMount(() => {
  Events.On('orchestrator:ticket_moved', () => loadBoard());
  Events.On('orchestrator:agent_started', () => loadBoard());
  Events.On('orchestrator:agent_done', () => loadBoard());
  Events.On('orchestrator:review_result', () => loadBoard());
  Events.On('orchestrator:pr_created', () => loadBoard());
  Events.On('orchestrator:parent_closed', () => loadBoard());
});
```

**Step 2: Add plan approval and execution controls to KanbanBoard toolbar**

Add buttons:
- "Plan erstellen" — calls `App.GeneratePlan(dir, selectedCardIDs)`
- "Plan genehmigen" — calls `App.ApprovePlan(dir, planID, parentIssue)`
- "Starten" — moves all "Approved" cards to "Ready" (or selected cards)
- "Stoppen" — calls `App.StopPlan(dir, planID)`

**Step 3: Add parent issue progress indicator**

For each unique `parent_issue` in the board, show a progress bar: `N/M sub-tickets done`.

**Step 4: Commit**

```bash
git add frontend/src/components/KanbanBoard.svelte frontend/src/stores/kanban.ts
git commit -m "feat: add orchestrator controls and event listeners to KanbanBoard"
```

---

## Task 10: Backend API — Start/Stop Orchestration

**Files:**
- Modify: `internal/backend/app_orchestrator.go`

**Step 1: Add `StartOrchestration` binding**

High-level method that starts the scheduler for a given directory:

```go
func (a *AppService) StartOrchestration(dir string) error {
	// Find all cards in "ready" column, start scheduler loop
	// Uses config.Orchestrator settings
}
```

**Step 2: Add `StopOrchestration` binding**

```go
func (a *AppService) StopOrchestration(dir string) error {
	// Stop the scheduler, do NOT close running sessions
	// Cards stay in their current column
}
```

**Step 3: Add `GetOrchestrationStatus` binding**

```go
type OrchestrationStatus struct {
	Active         bool   `json:"active"`
	RunningAgents  int    `json:"running_agents"`
	MaxAgents      int    `json:"max_agents"`
	PendingTickets int    `json:"pending_tickets"`
	ReviewTickets  int    `json:"review_tickets"`
	DoneTickets    int    `json:"done_tickets"`
}

func (a *AppService) GetOrchestrationStatus(dir string) OrchestrationStatus
```

**Step 4: Commit**

```bash
git add internal/backend/app_orchestrator.go
git commit -m "feat: add start/stop/status orchestration bindings"
```

---

## Task 11: Integration — Wire Scan Loop to Orchestrator

**Files:**
- Modify: `internal/backend/app_scan.go`

**Step 1: Add orchestrator notification on activity change**

In `scanAllSessions()`, after detecting "done" transition, notify the orchestrator:

```go
// After existing processQueue call:
a.notifyOrchestratorDone(id)
```

**Step 2: Implement `notifyOrchestratorDone`**

```go
func (a *AppService) notifyOrchestratorDone(sessionID int) {
	orchMu.Lock()
	defer orchMu.Unlock()
	for _, orch := range orchestrators {
		orch.mu.Lock()
		for cardID, sid := range orch.running {
			if sid == sessionID {
				// Agent finished — trigger review pipeline
				go a.startReviewPipeline(orch, cardID)
			}
		}
		orch.mu.Unlock()
	}
}
```

**Step 3: Commit**

```bash
git add internal/backend/app_scan.go
git commit -m "feat: wire scan loop to orchestrator for agent completion detection"
```

---

## Task 12: Orchestrator Settings UI

**Files:**
- Modify: `frontend/src/components/SettingsDialog.svelte`

**Step 1: Add Orchestrator section to settings**

Add a new section "Agent-Orchestrierung" with controls for:
- `max_parallel_agents` — Number input (1-8)
- `default_auto_merge` — Toggle
- `default_auto_start` — Toggle
- `max_retries` — Number input (0-5)
- `review_command` — Text input
- `sync_subtasks_to_github` — Toggle

**IMPORTANT:** Follow the CLAUDE.md rule — use `$: if (visible) initDialog();` pattern. Do NOT put assignments in reactive blocks.

**Step 2: Commit**

```bash
git add frontend/src/components/SettingsDialog.svelte
git commit -m "feat: add orchestrator settings section to SettingsDialog"
```

---

## Task 13: End-to-End Test — Manual Verification

**Step 1: Build and run**

```bash
cd frontend && npm run build && cd ..
go build -o build/bin/multiterminal.exe -tags desktop .
./build/bin/multiterminal.exe
```

**Step 2: Test the workflow manually**

1. Open Kanban board for a project directory
2. Create a card in "Define" column
3. Verify 7 columns render correctly
4. Move cards between columns via drag-drop
5. Check settings dialog shows orchestrator options
6. Verify `.mtui/kanban.json` persists new fields

**Step 3: Test backward compatibility**

1. Use an existing `.mtui/kanban.json` with old 5-column format
2. Verify migration runs: backlog→define, planned→approved, review→auto_review
3. Verify no data loss

---

## Implementation Order & Dependencies

```
Task 1 (Config) ──────────────────────┐
Task 2 (KanbanCard + Columns) ───────┤
                                      ├─→ Task 6 (Orchestrator Rewrite)
Task 3 (Frontend Store) ─────────────┤       │
Task 4 (KanbanBoard Layout) ─────────┤       ├─→ Task 7 (Review Pipeline)
Task 5 (KanbanCard UI) ──────────────┘       │        │
                                              ├─→ Task 8 (Sub-Ticket Gen)
                                              │        │
                                              ├─→ Task 10 (Start/Stop API)
                                              │        │
                                              └─→ Task 11 (Scan Integration)
                                                       │
Task 9 (Frontend Events) ←────────────────────────────┘
Task 12 (Settings UI) ← Task 1
Task 13 (E2E Test) ← All
```

**Parallelizable groups:**
- Group A (independent): Tasks 1, 2, 3, 4, 5 (data model + UI layout)
- Group B (depends on A): Tasks 6, 7, 8 (orchestrator core)
- Group C (depends on B): Tasks 9, 10, 11 (integration)
- Group D (independent of B/C): Task 12 (settings UI, depends only on Task 1)
- Final: Task 13 (E2E test)
