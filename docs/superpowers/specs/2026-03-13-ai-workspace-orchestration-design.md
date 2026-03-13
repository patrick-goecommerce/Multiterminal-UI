# AI Workspace Orchestration — Design Spec

**Date:** 2026-03-13
**Status:** Approved
**Inspiration:** tmux-ide (thijsverreck.com), wavyrai/tmux-ide

---

## Overview

Add native multi-agent orchestration to mtui: a `.mtui.yml` workspace file defines project layout and agent roles; a built-in MCP server coordinates agents in real time; the Kanban board is the source of truth and live status view.

This is an additive layer — existing single-terminal and manual multi-pane workflows are unchanged.

---

## Goals

- User opens a project, starts an issue from Kanban → a configured Agent Team launches automatically
- Lead agent coordinates teammates via MCP tools (no file polling, no race conditions)
- Before starting autonomous work, the Lead asks bundled clarifying questions — user answers once, then the team goes
- Agent roles are backed by deep, researched skills (not one-liners); skills are evaluated and adapted per issue
- Kanban board reflects agent state live
- Works with or without `.mtui.yml` (backwards compatible)
- Minimal team state persisted across app restarts (resume option)

---

## Architecture

```
.mtui.yml (project root, optional)
      │
      ▼
mtui loads workspace config
      │
      ├── applies pane layout (PTY sessions per role)
      ├── starts MCP server goroutine (HTTP/SSE, random port, single instance per app)
      └── writes MCP config → ~/.mtui/mcp-{sessionID}.json  (type: http, url: .../mcp)
                    │
    User clicks "Team starten" on a Kanban issue
                    │
                    ▼
         Lead agent spawns
         (claude --mcp-config ~/.mtui/mcp-{sessionID}.json)
                    │
         Lead uses MCP tools to coordinate:
         ┌──────────────────────────────┐
         │ spawn_teammate(role, task)   │ ← Lead only
         │ get_tasks()                  │ ← all agents
         │ claim_task(id)               │ ← all agents
         │ update_task(id, status, msg) │ ← all agents
         │ post_message(to, content)    │ ← all agents
         └──────────────────────────────┘
                    │
                    ▼
         Kanban board updates live via Wails events
```

**Key principles:**
- Agents communicate only through the MCP server — no file polling, no race conditions.
- One MCP server per app instance (not per team session). Sessions are routed by bearer token.
- `spawn_teammate` posts pane creation via a channel back to the main service goroutine — never calls `CreateSession` directly from an HTTP handler goroutine.

---

## MCP Transport

Claude Code supports the **Streamable HTTP** transport via `--mcp-config` (the older SSE transport is deprecated). The generated config file (`~/.mtui/mcp-{sessionID}.json`) has this format:

```json
{
  "mcpServers": {
    "mtui": {
      "type": "http",
      "url": "http://127.0.0.1:{PORT}/mcp",
      "headers": {
        "Authorization": "Bearer {SESSION_TOKEN}"
      }
    }
  }
}
```

- `PORT` — randomly chosen at app startup (same pattern as existing `tmuxAPIPort`)
- `SESSION_TOKEN` — cryptographically random 32-byte hex string, generated per team session
- The MCP server rejects any request missing the correct `Authorization: Bearer {token}` header

**Streamable HTTP protocol:** Each JSON-RPC request arrives as a separate HTTP POST to `/mcp`. The server responds inline (for simple results) or opens a per-request SSE stream (for streamed responses) that closes once the response is complete. There are no persistent connections.

**Implementation note:** Verify the exact `--mcp-config` JSON schema empirically before writing the config-file generator (e.g., test with `claude mcp add-json` as reference). If the schema differs from the `mcpServers` object format above, update the config writer accordingly.

The MCP server shares the existing random-port HTTP listener infrastructure in `app.go` (same pattern as `tmuxAPIPort`, different mux).

---

## `.mtui.yml` Format

```yaml
name: my-project
stack: go                       # optional; auto-detected if omitted (first match wins, see table below)

layout:
  - row: 70%
    panes:
      - role: lead
        size: 60%
      - role: reviewer
        size: 40%
  - row: 30%
    panes:
      - command: go test ./... -v
        title: Tests
      - title: Shell

team:
  model: claude-opus-4-6        # optional model override
  max_messages: 200             # optional, default 200; older messages evicted when exceeded
  roles:
    # String form: inline system prompt (simple / backwards compatible)
    lead: "You are the lead developer. Coordinate the team. Use spawn_teammate() for complex sub-tasks."
    # Object form: references a named skill file + role-specific focus
    reviewer:
      skill: code-reviewer       # loads from .mtui/skills/ or ~/.multiterminal/skills/
      focus: "Review the lead's code and report issues via update_task()."
```

**Both role formats are supported by the YAML parser.** A plain string value is treated as an inline prompt; an object with `skill` + `focus` loads a skill file and appends the focus. This allows gradual migration from inline prompts to full skill-backed roles.

**Rules:**
- `.mtui.yml` is optional. Without it, mtui works exactly as today.
- `.mtui.yml` is looked up in the `dir` passed to `StartTeam`. No directory traversal.
- `stack` auto-detected from project files if omitted. Detection runs in table order; first match wins.
- `layout` mirrors the tmux-ide row/pane model but maps to mtui PTY sessions.
- `team` section is optional — layout-only workspaces are valid.

---

## MCP Server

**Implementation:** Single Go HTTP/SSE server goroutine per app instance, started on first team session launch. Port shared with session routing by bearer token. Lives in `internal/backend/app_mcp.go` (~250 lines) and `internal/backend/app_mcp_state.go` (~200 lines).

### Tools Exposed

| Tool | Who | Description |
|------|-----|-------------|
| `get_tasks()` | all | Returns all tasks for the caller's team session |
| `claim_task(id)` | all | Atomically claims a task; error if already claimed |
| `update_task(id, status, msg)` | all | Updates task progress. Status: `pending`, `in_progress`, `done`, `blocked` |
| `post_message(to, content)` | all | Send message to a role or `"all"`; capped at `max_messages` (evict oldest) |
| `spawn_teammate(role, task)` | lead only | Posts pane-creation request to main goroutine via channel; returns teammate session ID |

### Security

Every request must include `Authorization: Bearer {SESSION_TOKEN}`. Requests without a valid token receive HTTP 401. Tokens are scoped to a single `TeamSession`.

### Shutdown & Cleanup

On `ServiceShutdown` (or when the last team session ends):
1. MCP server goroutine receives cancel via context; stops accepting new POST requests.
2. In-flight request-response pairs complete (bounded by handler return time — no persistent connections exist under Streamable HTTP).
3. Any active server-push GET streams are closed.
4. `~/.mtui/mcp-{sessionID}.json` files are deleted.
5. Stale files from previous crashed sessions are cleaned up on next app start (scan `~/.mtui/mcp-*.json`, verify port is live, delete if not).

---

## Data Model

Lives in `internal/backend/app_mcp_state.go` (not in `app_kanban.go`, which is already at the 300-line limit).

```go
// TeamSession tracks one active agent team.
type TeamSession struct {
    ID          string            `json:"id"          yaml:"id"`
    IssueID     string            `json:"issue_id"    yaml:"issue_id"`
    Token       string            `json:"-"           yaml:"-"`        // never serialized (frontend or disk); regenerated on resume
    Tasks       []TeamTask        `json:"tasks"       yaml:"tasks"`
    Messages    []TeamMessage     `json:"messages"    yaml:"messages"`
    Agents      map[string]string `json:"agents"      yaml:"agents"`   // role → pane sessionID
    MaxMessages int               `json:"max_messages" yaml:"max_messages"`
}

// TeamTask is a unit of work claimed by one agent.
type TeamTask struct {
    ID        string `json:"id"         yaml:"id"`
    Title     string `json:"title"      yaml:"title"`
    ClaimedBy string `json:"claimed_by" yaml:"claimed_by"` // agent role or ""
    Status    string `json:"status"     yaml:"status"`     // pending|in_progress|done|blocked
    Message   string `json:"message"    yaml:"message"`
}

// TeamMessage is inter-agent communication.
type TeamMessage struct {
    From      string `json:"from"      yaml:"from"`
    To        string `json:"to"        yaml:"to"`       // role or "all"
    Content   string `json:"content"   yaml:"content"`
    Timestamp int64  `json:"timestamp" yaml:"timestamp"`
}
```

**Relationship to existing orchestrator:** `TeamSession` is a parallel concept to `orchestratorState` in `app_orchestrator.go`. They do not merge — the orchestrator handles plan-based single-agent workflows; `TeamSession` handles multi-agent live coordination. `TeamTask.ID` may reference a `KanbanCard.ID` but is not required to.

---

## Agent Launch Flow

1. User selects a Kanban issue → clicks "Team starten"
2. mtui reads `.mtui.yml` from the session's working directory (no traversal)
3. MCP server goroutine starts if not already running; new `TeamSession` created with fresh bearer token
4. `~/.mtui/mcp-{sessionID}.json` written with MCP HTTP URL + token
5. Panes created according to layout
6. Lead pane launches: `claude --mcp-config ~/.mtui/mcp-{sessionID}.json`
7. Initial prompt injected via `WriteToSession` after Claude's prompt appears (detected by activity scanner); prompt = issue title + body
8. Lead calls `spawn_teammate(role, task)` → request sent via channel to main service goroutine → new PTY pane opened, new Claude process spawned with same `--mcp-config`
9. On agent session exit → `TeamSession.Agents[role]` cleared; Kanban card status auto-updates via Wails event

---

## Planning Dialog (Clarifying Questions Before Go)

Before autonomous execution begins, the Lead enters a **planning phase**:

1. Lead pane spawns with a planning prompt: analyze the issue, then output **3–5 bundled clarifying questions** in a structured format (JSON block that mtui can parse).
2. `app_mcp.go` registers a screen-content hook on the Lead session; after each `ActivityDone` event, it scans the screen buffer for the JSON sentinel (`{"mtui_planning":`). When found, it emits a Wails `team:planning_ready` event with the parsed questions. The activity scanner (`app_scan.go`) remains unaware of planning — separation of concerns is preserved.
3. User answers all questions in one dialog — no back-and-forth.
4. mtui injects the answers back into the Lead's session via `WriteToSession`.
5. Lead acknowledges ("Got it, starting...") and begins autonomous work.

**Structured question format** (Lead outputs this, mtui parses it):
```json
{"mtui_planning": [
  {"id": "1", "question": "Should the fix include tests?", "default": "yes"},
  {"id": "2", "question": "Target Go version constraint?", "default": "1.21+"}
]}
```

`TeamPlanningDialog.svelte` — modal dialog, props: `questions: PlanningQuestion[]`, emits `answered(answers: Record<string, string>)`. Displayed over the Kanban board. User can accept defaults or type custom answers.

---

## Agent Skills

### Current State

Existing mtui skills (`backend`, `devops`, `docs-technical`, `git-workflow`) are one-liners — not deep enough for effective agent roles in autonomous AI development.

### Skill Loading (B+C Mix)

**B — Role-referenced skills:** `.mtui.yml` role definitions reference a skill by name:

```yaml
team:
  roles:
    lead:
      skill: backend-lead          # loads from .mtui/skills/ or built-in
      focus: "coordinate and implement"
    reviewer:
      skill: code-reviewer
      focus: "review and catch regressions"
```

The skill file provides a deep system prompt (architecture principles, best practices, tooling knowledge). The `focus` field adds role-specific context on top.

**C — Issue-adaptive generation:** When a team session starts, mtui generates an **issue-specific addendum** appended to the base skill:

```
## Current Task Context
Issue: {title} (#{number})
Stack: {detected_stack}
Key files involved: {top_N_relevant_files from git/search}
Constraints from planning answers: {user_answers}
```

This gives each agent accurate context without rewriting the base skill per issue.

### Pre-Implementation Research Requirement

**⚠️ Before implementing agent skills, the following research must be completed:**

1. Search for current best practices in multi-agent AI development system prompts (e.g., Anthropic docs, research papers, community resources like Awesome-Claude-Prompts).
2. Evaluate the 4 existing mtui skills — are they reusable as a base, or should they be rewritten?
3. Write deep skill files for at minimum: `backend-lead`, `code-reviewer`, `frontend-specialist`. Each should cover: mental model of the role, decision-making heuristics, tool usage patterns, communication norms with teammates.
4. Skills are stored as markdown files in `.mtui/skills/{name}.md` (project-local) or `~/.multiterminal/skills/{name}.md` (global). Global skills ship with mtui as built-ins.

---

## Initial Prompt Injection

After the planning dialog completes, mtui waits for `ActivityIdle` (Claude prompt is ready, via activity scanner), then calls `WriteToSession(sessionID, prompt)` where prompt combines:

```
Working on: {issue.Title} (#{issue.Number})

{issue.Body}

## Answers to your planning questions
{formatted_planning_answers}

Use your MCP tools (get_tasks, claim_task, update_task, spawn_teammate) to coordinate the team.
Begin by breaking this into tasks and claiming the first one.
```

This is the same pattern used by the existing pipeline queue (`app_queue.go`). YOLO mode recommended for autonomous operation.

---

## Session Persistence (Minimal)

On app shutdown, the active `TeamSession` state (tasks + statuses + messages, no PTY content) is saved to `~/.multiterminal/team-sessions/{sessionID}.json`.

On next startup, if a saved session exists whose `IssueID` matches a `KanbanCard.ID` currently in the board, a **"Team fortsetzen?"** banner appears in the Kanban. `IssueID` is always the `KanbanCard.ID` string — this is the stable resume key. `ServiceStartup` creates `~/.multiterminal/` and `~/.multiterminal/team-sessions/` via `os.MkdirAll` if they do not exist. User can resume (re-spawn agents with prior task context) or dismiss (discard saved state).

Persistence format: same `TeamSession` struct serialized to JSON. No PTY replay — agents start fresh with a context summary injected as initial prompt.

---

## UI Changes

### New Components

- `KanbanTeamView.svelte` — team status panel within Kanban. Props: `session: TeamSession`. Displays: agent list (role + status badge), task list (claimed by / status), messages log (last N). Emits no events; read-only view driven by Wails `team:update` events. Design detail TBD during implementation.
- `TeamPlanningDialog.svelte` — modal over Kanban. Props: `questions: PlanningQuestion[]`. Emits: `answered(Record<string, string>)`. Shows Lead's parsed questions with input fields (pre-filled with defaults). "Los gehts" button submits. **Distinct from the existing `KanbanPlanDialog.svelte`** (which handles orchestration plan approval — different purpose).

### Extended Components

- `KanbanCard.svelte` — shows agent badge (role name) when an agent has claimed the card's task
- `LaunchDialog.svelte` — adds "Als Team starten" button when `.mtui.yml` is detected in the session dir
- `KanbanColumn.svelte` — "Team starten" shortcut on issue cards

### New Backend Files

| File | Purpose | Est. lines |
|------|---------|-----------|
| `internal/backend/app_mcp.go` | MCP HTTP/SSE server, tool handlers, request routing | ~250 |
| `internal/backend/app_mcp_state.go` | `TeamSession` CRUD, token store, message eviction | ~200 |
| `internal/backend/app_workspace.go` | `.mtui.yml` parser, stack auto-detection | ~150 |
| `internal/backend/app_skills.go` | Skill file loading (project-local + global), issue-adaptive addendum generation | ~150 |

### Extended Backend Files

| File | Change |
|------|--------|
| `internal/backend/app.go` | MCP server start/stop in `ServiceStartup`/`ServiceShutdown`; `spawnCh` channel for teammate requests |
| `internal/backend/app_kanban.go` | Wire `TeamSession` events to Kanban card state (agent badges, status) |

---

## Stack Auto-Detection

Detection runs in order; first match wins.

| File found | Detected stack | Default roles |
|-----------|---------------|--------------|
| `go.mod` | go | lead, reviewer |
| `next.config.*` | nextjs | lead, reviewer |
| `package.json` | node | lead, reviewer |
| `requirements.txt` / `pyproject.toml` | python | lead, reviewer |
| _(none)_ | default | lead only |

---

## Backwards Compatibility

- No `.mtui.yml` → mtui works exactly as before
- MCP server only starts on first `StartTeam` call
- Existing PTY sessions, manual panes, shell, pipeline queue — all unchanged
- No new required config in `~/.multiterminal.yaml`

---

## Out of Scope

- Cross-machine agent teams
- Agent-to-agent direct messaging without MCP
- Visual workflow editor for `.mtui.yml`
- Merging `TeamSession` with `orchestratorState` (future unification path, not now)
