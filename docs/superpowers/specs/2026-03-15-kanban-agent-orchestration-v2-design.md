# Kanban-gesteuerte Agent-Orchestrierung v2

> Supersedes: `2026-03-13-ai-workspace-orchestration-design.md`
> Date: 2026-03-15
> Status: Draft

## Problem

The current implementation (worktree `ai-workspace-orchestration`) uses a custom MCP server, screen-buffer JSON parsing, and custom agent coordination — too many moving parts, fragile, and the generated plans are shallow. The user has to "hand-hold" Claude in the terminal.

## Goal

A Kanban-driven agent orchestration system where the user clicks a card, answers a quiz, reviews a plan, and agents execute autonomously in the background — with full visibility but zero babysitting.

## Design Principles

1. **No custom agent infrastructure** — use Claude Code CLI (`claude -p`, Agent Teams env var) as the execution engine
2. **Structured I/O over screen scraping** — `--output-format json` for all headless calls
3. **Prompts via stdin, not argv** — all prompts piped via stdin to avoid OS argument length limits (Windows: ~32K chars)
4. **Self-organizing documentation** — `docs/mtui/` managed autonomously by Claude
5. **Progressive complexity** — trivial cards skip the pipeline, complex cards get full orchestration
6. **Issue context is never lost** — every interaction carries the full card context chain
7. **Cost visibility** — per-card cost tracking, configurable budget limits via `--max-budget-usd`

## Architecture Overview

```
┌─ Card Detail UI ─────────────────────────────────────────────┐
│  [Plan erstellen]  [Chat]  [Terminal]  [Starten]             │
│                                                               │
│  Quiz-Flow → Plan-Review → Sub-Cards mit Status              │
└──────────────────────────────────────────────────────────────┘
        │                        │                    │
        ▼                        ▼                    ▼
  claude -p --json         Chat: claude -p      Agent Teams
  (Headless Planning)      (Card-Kontext)       (Parallel Worktrees)
        │                                             │
        ▼                                             ▼
  docs/mtui/                                    Lead-Agent
  (Self-organizing Memory)                      ├── Worker A (worktree)
                                                ├── Worker B (worktree)
                                                └── Reviewer (QA-Loop)
```

## Phase 1: Quiz-Flow + Plan Generation

### Trigger

User opens Card Detail → clicks "Plan erstellen".

### Step 1: Complexity Assessment

A fast headless call classifies the card:

```bash
claude -p "Classify this issue as trivial/medium/complex.
Issue: <title + body>
Repo context: <docs/mtui/README.md if exists>
Respond with JSON: {\"complexity\": \"trivial|medium|complex\", \"reason\": \"...\"}" \
  --output-format json
```

**Routing:**
- **Trivial** (typo, config change): Single `claude -p` execution, no sub-cards, no quiz
- **Medium** (1-2 file feature): Quiz with 2-3 questions, simple plan, sequential execution
- **Complex** (multi-file feature): Full quiz, detailed plan with dependencies, parallel execution

### Step 2: Question Generation

For medium/complex cards:

```bash
claude -p "Analyze this issue and the codebase. Generate clarifying questions.
Issue: <title + body>
Return JSON: {\"questions\": [{\"id\": \"1\", \"text\": \"...\", \"type\": \"choice|text\", \"options\": [...]}]}" \
  --output-format json
```

### Step 3: Quiz UI

Frontend renders questions as an interactive quiz — one at a time, like flashcards:
- `type: "choice"` → buttons/radio
- `type: "text"` → input field
- Progress indicator: "Frage 2 von 5"
- Animations between questions for fluid feel

### Step 4: Plan Generation

After quiz completion:

```bash
claude -p "Create an implementation plan for this issue.
Issue: <title + body>
User answers: <quiz answers JSON>
Codebase context: <docs/mtui/ contents if available>
Return JSON: {
  \"plan\": {
    \"summary\": \"...\",
    \"steps\": [{
      \"id\": \"1\",
      \"title\": \"...\",
      \"description\": \"...\",
      \"parallel_ok\": true|false,
      \"depends_on\": [],
      \"files\": [\"path/to/file.go\"]
    }]
  }
}" --output-format json
```

### Step 5: Plan Review UI

Card Detail shows:
- Plan summary
- Steps as a checklist (read-only preview of future sub-cards)
- **[Starten]** button
- **Korrektur-Feld** — free text input → triggers re-generation with correction context
- **[Im Terminal verfeinern]** — opens full Claude session with plan context

### Error Handling

### Error Handling (All Headless Calls)

**JSON parsing failures:**
1. Retry once with explicit schema reminder in prompt
2. If still malformed: show raw output in Card Detail with "Plan konnte nicht erstellt werden" message + option to open Terminal

**Process-level failures:**
- **Non-zero exit code** (auth failure, rate limit): Show error message, offer retry
- **Timeout** (no output within deadline): Kill process, show "Zeitüberschreitung" + retry option
- **OOM / killed by OS**: Detect via exit signal, show "Prozess abgebrochen" + retry with smaller context
- **Claude refusal** ("I can't help with that"): Surface response text to user, offer Terminal switch
- **Partial JSON** (process killed mid-stream): Treat as malformed, retry once

### Cancellation

Every `RunHeadless` call receives a `context.Context` derived from the card's lifecycle:
- User clicks "Abbrechen" → context cancelled → `cmd.Process.Kill()` → cleanup
- Card deleted while running → same cancellation path
- App shutdown → all running contexts cancelled via `ServiceShutdown`

## Data Model Changes

### KanbanCard Struct (Go)

```go
type KanbanCard struct {
    ID          string            `json:"id" yaml:"id"`
    Title       string            `json:"title" yaml:"title"`
    Body        string            `json:"body" yaml:"body"`
    Labels      []string          `json:"labels" yaml:"labels"`
    // New fields for orchestration:
    ParentID    string            `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
    Status      string            `json:"status,omitempty" yaml:"status,omitempty"`       // pending|in_progress|finished|fail
    Complexity  string            `json:"complexity,omitempty" yaml:"complexity,omitempty"` // trivial|medium|complex
    QuizAnswers map[string]string `json:"quiz_answers,omitempty" yaml:"quiz_answers,omitempty"`
    Plan        *CardPlan         `json:"plan,omitempty" yaml:"plan,omitempty"`
    SubCards    []string          `json:"sub_cards,omitempty" yaml:"sub_cards,omitempty"` // IDs of child cards
    AgentID     string            `json:"agent_id,omitempty" yaml:"agent_id,omitempty"`   // assigned agent
    Cost        float64           `json:"cost,omitempty" yaml:"cost,omitempty"`           // accumulated USD
}

type CardPlan struct {
    Summary string     `json:"summary" yaml:"summary"`
    Steps   []PlanStep `json:"steps" yaml:"steps"`
}

type PlanStep struct {
    ID         string   `json:"id" yaml:"id"`
    Title      string   `json:"title" yaml:"title"`
    Desc       string   `json:"description" yaml:"description"`
    ParallelOK bool     `json:"parallel_ok" yaml:"parallel_ok"`
    DependsOn  []string `json:"depends_on" yaml:"depends_on"`
    Files      []string `json:"files" yaml:"files"`
}
```

**IMPORTANT:** `models.ts` must be updated manually to match (per CLAUDE.md Wails binding sync rule).

### Trivial Card Execution

For trivial cards (no quiz, no sub-cards):
1. Single `claude -p` call with issue context + "fix this"
2. On success: card moves to "Done" column, cost recorded
3. On failure: card stays, error shown in Card Detail + Terminal switch option

## Phase 2: Execution + QA Loop

### Sub-Card Creation

When user clicks "Starten", each plan step becomes a sub-card:
- Sub-cards are visually indented under the parent card
- States: `pending` → `in_progress` → `finished` / `fail`
- Toggle switch to show/hide sub-cards (board stays clean)
- Parent card shows aggregate progress: "3/7 Tasks"

### Sequential Execution (Medium Complexity)

Single `claude -p` process works through steps:

```bash
claude -p "Execute step N of the plan:
Plan: <full plan JSON>
Current step: <step details>
Previous results: <summary of completed steps>
Project: <docs/mtui/ context>
Original issue: <card title + body>

After completion, return JSON: {
  \"status\": \"done|failed\",
  \"summary\": \"what was done\",
  \"files_changed\": [\"...\"],
  \"learnings\": [\"...\"]
}" --output-format json
```

Sub-card updates to `in_progress` when started, `finished`/`fail` when done.

### QA Review Loop

After each sub-card completion, an automatic review runs:

```bash
claude -p "Review this change against the requirements.
Original requirement: <card title + body>
Step requirement: <sub-card description>
Diff: <git diff of changes>
Does this meet the requirement? Return JSON: {
  \"approved\": true|false,
  \"issues\": [\"...\"],
  \"suggestion\": \"...\"
}" --output-format json
```

- **Approved** → sub-card "finished", proceed to next
- **Issues found** → sub-card stays "in_progress", correction applied automatically
- **After 3 failed reviews** → sub-card "fail", escalate to user

### Anti-Drift Checkpoint

After every 2nd completed sub-card, verify alignment:

```bash
claude -p "Compare completed work against the original plan.
Original issue: <card context>
Plan: <plan JSON>
Completed steps: <summaries>
Are we still on track? Any drift? Return JSON: {
  \"on_track\": true|false,
  \"drift\": \"description if any\",
  \"recommendation\": \"continue|adjust|stop\"
}" --output-format json
```

If drift detected → pause execution, notify user via footer badge.

### Agent Markdown Definitions

```
docs/mtui/agents/
  lead.md          — Coordination, plan execution, error decisions
  coder.md         — Implementation, follows plan steps
  reviewer.md      — Code review, QA validation
```

Injected as system prompt via Go code: `RunHeadless(ctx, prompt, readFile("docs/mtui/agents/coder.md"), ...)`

> **Note:** All bash examples in this spec are conceptual. The Go implementation reads files and constructs arguments in Go, not via shell expansion (`$(cat ...)` does not work in Go's `exec.Command`).

### Chat Popup

Available at any time from Card Detail:
- Opens a popup chat interface within the card
- Each message is a `claude -p` call with the full card context (issue, quiz answers, plan, sub-card states)
- **Stateless by design:** Each call carries the full context + previous chat messages as conversation history in the prompt. This means growing prompt size per exchange — acceptable for "quick clarification" use case (5-10 messages max). For deeper work → Terminal switch.
- Chat history is stored in `.mtui/chat/<card-id>.json` (gitignored, not in `docs/mtui/`)

### Terminal Switch

"Im Terminal öffnen" button in Card Detail:
- Opens a full Claude PTY session in a new pane
- Session is pre-loaded with the entire card context via `injectLeadPrompt`:
  - Issue title + body
  - Quiz answers
  - Current plan + sub-card states
  - Relevant `docs/mtui/` context
- **On session close**: automatic reconciliation
  - Update parent card status/summary
  - Update all affected sub-card states
  - Write session learnings to `docs/mtui/`
  - If GitHub-linked: update/close issues

## Phase 3: Parallel Agents + Teams

> **Risk:** Agent Teams is experimental. The exact CLI interface may change.
> Phase 3 should only begin once Phase 1+2 are stable and the Agent Teams API is verified.

### Agent Teams

Uses Claude Code's experimental Agent Teams feature:
- `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` (already injected in session.go)
- Coordination happens through shared state in `~/.claude/teams/<team-name>/`
- Exact CLI flags for team/teammate mode TBD — must be verified against current `claude --help` before implementation
- Fallback: if Agent Teams API changes, parallel execution degrades gracefully to sequential (Phase 2 behavior)

### Parallel Worktree Execution

For complex cards with `parallel_ok` steps:
1. Lead agent reviews the plan and identifies parallelizable groups
2. Each parallel group gets its own git worktree
3. Worker agents execute in isolated worktrees
4. On completion: lead agent reviews + merges worktrees sequentially
5. Merge conflicts → lead agent resolves or escalates to user

### Claims-Based Work Distribution

Each sub-card is a "claim":
- Worker takes a sub-card → `in_progress` + assigned agent ID
- No PTY output for N minutes (configurable) → claim expires
- Lead agent detects stall via `LastOutputAt` health monitoring
- Expired claim → reassign to new worker or escalate
- Partial work preserved in worktree git state

### Lead-Agent Error Handling

When a worker fails:
1. Lead agent receives failure context (error message, diff, logs)
2. Decision matrix:
   - **Retryable** (test flake, network timeout): Retry same step, max 2 attempts
   - **Fixable** (missing import, wrong API call): Apply correction, retry
   - **Architectural** (wrong approach): Replan this step, new sub-card
   - **Blocked** (needs human decision): Escalate → sub-card "fail" + notification

## Self-Organizing Memory Layer

### Structure

```
docs/mtui/
  README.md                     ← Auto-maintained index
  agents/                       ← Agent role definitions (markdown)
    lead.md
    coder.md
    reviewer.md
  board/                        ← Per-card context (auto-managed)
    <card-id>.md               ← Quiz answers, plan, decisions, chat history
  ...                           ← Claude organizes additional structure as needed
```

### Lifecycle

**After every agent session:**

```bash
claude -p "Review and update docs/mtui/:
1. Update relevant card file with new learnings
2. Remove outdated information
3. Update README.md index if structure changed
4. Add project-level learnings if applicable
Context: <session summary>" --output-format json
```

### Pattern Reuse

When generating a plan for a new card, include completed card summaries:

```bash
claude -p "...
Similar completed cards:
- <card-abc>: OAuth login — approach: ..., learnings: ...
Use these as reference if relevant.
..."
```

## UI Components

### Card Detail View (Enhanced)

```
┌─ Card: OAuth Login implementieren ──────────────────────┐
│  Status: In Arbeit (4/7)              [Chat] [Terminal]  │
│                                                          │
│  ┌─ Plan ──────────────────────────────────────────────┐ │
│  │ OAuth2 PKCE Flow mit Google Provider               │ │
│  │ implementieren. Session-Token in HttpOnly Cookie.  │ │
│  └────────────────────────────────────────────────────┘ │
│                                                          │
│  Sub-Tasks:                          [▼ Ein-/Ausblenden] │
│    ✓ OAuth Provider Config erstellen                     │
│    ✓ Callback-Endpoint implementieren                    │
│    ✓ Token-Exchange Service                              │
│    ⏳ Session-Middleware (in Arbeit...)                   │
│    ○ Frontend Login-Button                               │
│    ○ Protected Route Guard                               │
│    ○ Integration Tests                                   │
│                                                          │
│  Korrektur: [____________________________] [Anwenden]    │
└──────────────────────────────────────────────────────────┘
```

### Footer Badge

```
┌─ Footer ─────────────────────────────────────────────────┐
│  main · $1.23  │  OAuth Login: 4/7 · $0.34               │
└──────────────────────────────────────────────────────────┘
```

Click → jumps to Board with card focused.

### Quiz Flow

```
┌─ Plan erstellen ────────────────────────────────────┐
│                                                      │
│  Frage 2 von 4                         ●●○○          │
│                                                      │
│  Welche OAuth Provider sollen                        │
│  unterstützt werden?                                 │
│                                                      │
│  [Google]  [GitHub]  [Beide]  [Custom]               │
│                                                      │
└──────────────────────────────────────────────────────┘
```

## Issue Context Chain — Core Invariant

**Every interaction carries the full context chain. No exceptions.**

The context payload passed to every `claude -p` call and every terminal session:

```json
{
  "card": {
    "id": "abc123",
    "title": "OAuth Login implementieren",
    "body": "Full issue description...",
    "column": "in_progress"
  },
  "quiz_answers": { "provider": "Google", "test_coverage": "Unit+Integration" },
  "plan": { "steps": [...] },
  "sub_cards": [
    { "id": "sub1", "title": "OAuth Config", "status": "finished" },
    { "id": "sub2", "title": "Callback Endpoint", "status": "in_progress" }
  ],
  "memory": "Contents of docs/mtui/board/abc123.md"
}
```

**On session close** (terminal, agent completion, chat end):
- Parent card updated (status, summary)
- All affected sub-cards set to correct state
- `docs/mtui/` updated with learnings
- If GitHub-linked: issues/sub-issues closed or commented

## Migration from v1 (Worktree Code)

### Keep
- `KanbanBoard.svelte` UI improvements (drag fixes, card detail view)
- `KanbanCard.svelte` enhancements
- `KanbanCardDetail.svelte` (adapt for quiz + plan UI)
- `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` injection in `session.go`
- `app_kanban.go` + `app_kanban_helpers.go` (board state management)

### Remove
- `app_mcp.go` + `app_mcp_state.go` — no custom MCP server needed
- `app_mcp_test.go` + `app_mcp_state_test.go`
- `app_team.go` — replace with simpler `claude -p` based orchestration
- `app_skills.go` + `app_skills_test.go` — skills move to `docs/mtui/agents/`
- `app_workspace.go` + `app_workspace_test.go` — workspace config not needed
- `internal/backend/skills/` — move to `docs/mtui/agents/`
- `TeamPlanningDialog.svelte` — replace with Quiz-Flow component
- `KanbanTeamView.svelte` — sub-cards render inline in Card Detail

### Remove (also from worktree, if not listed above)
- `app_orchestrator.go` + `app_orchestrator_review.go` + `app_orchestrator_schedule.go` — replaced by new orchestrate files

### New
- `app_orchestrate.go` — `RunHeadless` subprocess management, cancellation, cost tracking
- `app_orchestrate_plan.go` — quiz + plan generation logic, complexity routing
- `app_orchestrate_exec.go` — execution loop, sub-card state machine
- `app_orchestrate_qa.go` — QA review loop, anti-drift checkpoints
- `app_orchestrate_memory.go` — `docs/mtui/` memory lifecycle
- `KanbanQuizFlow.svelte` — interactive quiz component
- `KanbanPlanReview.svelte` — plan display + correction field
- `KanbanChatPopup.svelte` — inline chat for quick questions
- `KanbanSubCards.svelte` — collapsible sub-card list
- `FooterTeamBadge.svelte` — team progress badge

## Technical Notes

### claude -p Subprocess Management

```go
// RunHeadless executes a single claude -p call and returns parsed JSON.
// Prompts are piped via stdin (not argv) to avoid OS argument length limits.
// Uses COMSPEC wrapping on Windows (same pattern as session.go:Start).
func (a *AppService) RunHeadless(ctx context.Context, prompt, systemPrompt, workDir string, budgetUSD float64) (json.RawMessage, error) {
    claudePath := a.resolvedClaudePath
    if claudePath == "" {
        claudePath = "claude"
    }
    args := []string{"/c", claudePath, "-p", "--output-format", "json"}
    if systemPrompt != "" {
        args = append(args, "--system-prompt", systemPrompt)
    }
    if budgetUSD > 0 {
        args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", budgetUSD))
    }
    comspec := os.Getenv("COMSPEC")
    if comspec == "" {
        comspec = `C:\Windows\System32\cmd.exe`
    }
    cmd := exec.CommandContext(ctx, comspec, args...)
    cmd.Dir = workDir
    cmd.Stdin = strings.NewReader(prompt) // prompt via stdin, not argv
    cmd.Env = stripClaudeCodeEnv(os.Environ())
    // ... execute, parse JSON, validate schema
}
```

### Cost Tracking

Each `RunHeadless` call returns cost metadata (via `--verbose` or parsed from JSON output).
Cost is accumulated on the `KanbanCard.Cost` field and displayed in:
- Card Detail header: "$0.34"
- Footer badge: "OAuth Login: 4/7 · $0.34"
- Configurable per-card budget limit via `--max-budget-usd` (default: from config)

### JSON Schema Validation

All `claude -p --output-format json` responses are validated against expected schema.
On validation failure: retry once with schema error feedback in prompt.
On second failure: surface raw output to user.

### Event Flow

```
Go Backend                           Svelte Frontend
─────────                           ───────────────
RunHeadless("questions...")  ──►    kanban:quiz-ready
                             ◄──    kanban:quiz-answered
RunHeadless("plan...")       ──►    kanban:plan-ready
                             ◄──    kanban:plan-approved
ExecuteStep(1)               ──►    kanban:subcard-update
ReviewStep(1)                ──►    kanban:subcard-update
ExecuteStep(2)               ──►    kanban:subcard-update
...
UpdateMemory()               ──►    kanban:card-complete
```

## Phased Rollout

### Phase 1: Quiz-Flow + Plan (Foundation)
- `app_orchestrate.go` — `RunHeadless` with stdin piping, COMSPEC, cancellation
- `app_orchestrate_plan.go` — complexity routing, question generation, plan generation
- `KanbanQuizFlow.svelte` + `KanbanPlanReview.svelte`
- `KanbanCard` struct extensions + `models.ts` sync
- Card Detail UI with plan display + correction field
- `docs/mtui/` initial structure + agent markdown definitions
- Trivial card direct execution path

### Phase 2: Execution + QA
- `app_orchestrate_exec.go` — sequential execution, sub-card state machine
- `app_orchestrate_qa.go` — QA review loop, anti-drift checkpoints
- `app_orchestrate_memory.go` — memory lifecycle after sessions
- `KanbanSubCards.svelte` + `FooterTeamBadge.svelte`
- Chat popup (`.mtui/chat/` gitignored) + Terminal switch
- Cost tracking + budget limits per card

### Phase 3: Parallel Agents + Teams (after verifying Agent Teams CLI)
- Agent Teams integration (exact flags TBD, verified against `claude --help`)
- Parallel worktree execution
- Claims-based work distribution with stall detection
- Lead-agent error handling (retry/fix/replan/escalate)
- Pattern reuse from completed cards
- Graceful fallback to sequential if Agent Teams unavailable

## References

- tmux-ide Agent Teams: https://tmux.thijsverreck.com/docs/agent-teams
- Aperant: https://github.com/AndyMik90/Aperant
- ruflo: https://github.com/ruvnet/ruflo
- Claude Code CLI: `claude -p`, `--output-format json`, `--system-prompt`
- Agent Teams: `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`
- Previous spec: `docs/superpowers/specs/2026-03-13-ai-workspace-orchestration-design.md`
