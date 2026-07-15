# Kanban-Driven Agent Orchestration

**Date:** 2026-03-10
**Status:** Approved
**Parent Issue:** TBD

## Summary

Multiterminal becomes a fully autonomous development orchestrator. A main chat session acts as project manager, breaking GitHub Issues into sub-tickets on a Kanban board. Agents (Claude Code PTY sessions in isolated Git Worktrees) pick up tickets, implement them, and pass through automated review — all with minimal human intervention.

## Workflow

```
Define → Refine → Approved → Ready → In Progress → Auto Review → Done
```

### Column Definitions

| Column | Description | Trigger |
|--------|-------------|---------|
| **Define** | Main ticket (GitHub Issue) created with feature description | Manual or Claude |
| **Refine** | Orchestrator creates plan, decomposes into sub-tickets. User edits interactively | Automatic on plan start |
| **Approved** | Plan confirmed. Sub-tickets wait for user to greenlight execution | User confirms plan |
| **Ready** | Sub-tickets released for agent execution. Waits for free agent slot | Manual move or `auto_start: true` |
| **In Progress** | Agent working in dedicated Git Worktree + PTY session | Scheduler assigns agent |
| **Auto Review** | Agent done. Tests + Review-Agent + PR creation running | Activity detection "done" |
| **Done** | Review passed. PR created (or auto-merged). Worktree cleaned up | Review success |

## Architecture

```
┌─ Multiterminal (Orchestrator) ──────────────────────────┐
│                                                          │
│  Main Chat (User = Project Manager)                      │
│    │                                                     │
│    ├─ "I want Feature XYZ"                               │
│    │                                                     │
│    ▼                                                     │
│  Plan-Agent (Claude Session)                             │
│    │  Creates plan + sub-tickets                         │
│    │                                                     │
│    ▼                                                     │
│  ┌─ Kanban Board ─────────────────────────────────────┐  │
│  │Define│Refine│Approved│Ready│InProgress│Review│Done  │  │
│  │  ●   │  ●●  │        │ ●●● │   ●●    │  ●   │ ●   │  │
│  └────────────────────────────────────────────────────┘  │
│    │                    ▲                                 │
│    ▼                    │ Result                          │
│  Agent Pool (max_parallel_agents, default: 3)            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                 │
│  │ Agent 1  │ │ Agent 2  │ │ Agent 3  │                  │
│  │ Worktree │ │ Worktree │ │ Worktree │                  │
│  │ PTY Sess │ │ PTY Sess │ │ PTY Sess │                  │
│  └──────────┘ └──────────┘ └──────────┘                  │
│       │             │            │                        │
│       ▼             ▼            ▼                        │
│  Review Pipeline (per agent):                            │
│    1. Tests (go test, go vet, lint)                      │
│    2. Review-Agent (diff analysis)                       │
│    3. PR creation                                        │
│    4. Auto-merge (if auto_merge flag set)                │
└──────────────────────────────────────────────────────────┘
```

## Agent Isolation

Each agent works in a **dedicated Git Worktree**:
- Created when ticket moves to "In Progress"
- Branch name derived from ticket (e.g., `agent/ticket-42-add-auth`)
- Agent has full repo copy, no conflict with other agents
- On completion: PR created from worktree branch → target branch
- On auto-merge success or manual merge: worktree cleaned up

Existing `app_worktree.go` provides the infrastructure.

## Sub-Ticket Model

Sub-tickets are **local Kanban cards** (in `.mtui/kanban.json`) with reference to a parent GitHub Issue. Optional sync to GitHub Issues via config flag.

### Extended KanbanCard

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

    // New fields for agent orchestration
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

## Kanban Columns (7)

```go
var KanbanColumns = []string{
    "define", "refine", "approved", "ready",
    "in_progress", "auto_review", "done",
}
```

## Config Extension

```yaml
orchestrator:
  max_parallel_agents: 3        # Max concurrent agent sessions
  default_auto_merge: false     # Default for new sub-tickets
  default_auto_start: false     # Default: manual Ready transition
  max_retries: 2                # Max retry attempts on review failure
  review_command: "go test ./... && go vet ./..."
  sync_subtasks_to_github: false # Create GitHub Issues for sub-tickets
```

## Scheduler Loop

Runs every 5 seconds in a background goroutine:

```
1. Check "ready" column for tickets with fulfilled dependencies
2. If agent slot free (running < max_parallel_agents):
   a. Pick next Ready ticket (priority → dependency order)
   b. Create Git Worktree (branch: agent/<ticket-id>-<slug>)
   c. Spawn Claude PTY session with ticket prompt
   d. Link session to ticket
   e. Move ticket → "in_progress"

3. Check "in_progress" tickets for activity "done":
   a. Run tests in worktree (review_command)
   b. Spawn Review-Agent with diff
   c. Move ticket → "auto_review"

4. Check "auto_review" tickets:
   a. Tests + review passed →
      - Create PR from worktree branch
      - If auto_merge → merge PR, cleanup worktree
      - Move ticket → "done"
   b. Failed + retries < max →
      - Increment retry_count
      - Feed error context back to agent
      - Move ticket → "in_progress"
   c. Failed + retries >= max →
      - Keep in "auto_review"
      - Notify user (desktop notification)

5. Check if all sub-tickets of a parent issue are "done":
   - Close parent GitHub Issue
   - Post summary comment on parent issue
```

## Autonomy Control

Per-ticket flags control automation level:

| Flag | Default | Effect |
|------|---------|--------|
| `auto_start` | `false` | `true`: skip manual Approved→Ready transition |
| `auto_merge` | `false` | `true`: merge PR without user approval |
| `max_retries` | `2` | How often agent retries on review failure |

## Plan Creation Flow

1. User creates/selects GitHub Issue (or describes feature in main chat)
2. Orchestrator spawns Plan-Agent session
3. Plan-Agent analyzes issue, proposes sub-tickets with:
   - Title, prompt, dependencies, priority, auto_merge flag
4. User reviews and adjusts plan interactively
5. User approves → sub-tickets created as Kanban cards in "Approved" column
6. User moves to "Ready" (or auto_start kicks in)

## Events (Wails)

| Event | Payload | Trigger |
|-------|---------|---------|
| `orchestrator:ticket_moved` | `{cardId, from, to}` | Card changes column |
| `orchestrator:agent_started` | `{cardId, sessionId, worktree}` | Agent spawned |
| `orchestrator:agent_done` | `{cardId, sessionId}` | Agent activity → done |
| `orchestrator:review_result` | `{cardId, passed, details}` | Review completed |
| `orchestrator:pr_created` | `{cardId, prNumber, url}` | PR created |
| `orchestrator:parent_closed` | `{issueNumber}` | All sub-tickets done |

## Review Pipeline

When an agent's activity transitions to "done":

1. **Test Runner** — Execute `review_command` in the agent's worktree
2. **Review Agent** — Spawn a Claude session that:
   - Gets the full diff (`git diff main...agent-branch`)
   - Checks against CLAUDE.md rules
   - Checks for security issues (OWASP top 10)
   - Produces pass/fail with findings
3. **PR Creation** — On pass: `gh pr create` from worktree branch
4. **Auto-Merge** — If `auto_merge: true` and PR checks pass: `gh pr merge`

On failure: error context (test output + review findings) is fed back to the original agent via the queue system.

## UI Changes

### Kanban Board
- 7 columns instead of 5
- Sub-ticket cards show: parent issue link, agent status, worktree branch, PR link
- Color coding: green (auto_merge), yellow (manual review needed)
- Progress indicator on parent issue cards (3/5 sub-tickets done)

### Agent Indicators
- Active agent sessions visible as terminal panes (existing functionality)
- Pane title shows ticket ID and status
- Footer shows: running agents / max, total cost across agents

## Dependencies on Existing Code

| Component | File | Status |
|-----------|------|--------|
| Kanban board | `app_kanban.go`, `KanbanBoard.svelte` | Extend columns + card fields |
| Git Worktrees | `app_worktree.go` | Existing, extend for auto-creation |
| Issue integration | `app_issues.go` | Existing, add sub-issue creation |
| Orchestrator | `app_orchestrator.go` | Existing, major rewrite for new workflow |
| Plan system | `app_kanban_plan.go` | Existing, extend for sub-ticket generation |
| Queue system | `app_queue.go` | Existing, use for error feedback |
| Activity detection | `activity.go` | Existing, triggers review pipeline |
| Progress reporting | `app_issue_progress.go` | Existing, extend for parent tracking |
| Background agents | `background-agents.ts` | Existing pattern, adapt for review agents |
| Session management | `session.go` | Existing, no changes needed |
