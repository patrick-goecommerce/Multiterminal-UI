# claudelog — JSONL Session Reader (Foundation)

- **Date:** 2026-06-23
- **Status:** Approved design, pre-implementation
- **Branch target:** `alpha-main` (feature branch off it)
- **Scope:** Foundation only. Replaces fragile screen-scraping of Claude token/cost data with a structured reader of Claude Code's JSONL session logs, and adds a turn-state–based "working" signal plus a convergence ("stuck loop") warning. Subagents and todos are parsed but not yet rendered (explicit follow-on).

## Context

MTUI currently derives Claude token/cost/activity data by scraping the **rendered terminal screen** (`internal/terminal/activity.go`):

- `ScanTokens()` matches `$\d+\.\d+` for cost and `"15.2k input"` / `"3.8k output"` regexes — only two token counters, no cache tokens, dependent on whatever Claude happens to print (theme, width, CLI version).
- `DetectActivity()` / `classifyScreenState()` classify activity from prompt/question heuristics over the last 15 screen rows.

This is inaccurate and version-fragile. Claude Code already writes complete, structured data to disk under `~/.claude/projects/{encoded_path}/{sessionId}.jsonl`. Reading it directly yields exact tokens (four counters), per-model cost, subagent state, todos, and a precise turn-state.

**MTUI's structural advantage over external monitors (e.g. Agent-Dashboard):** MTUI owns the PTY. Critically, it **launches Claude with an explicit `--session-id <uuid>` flag** (verified in `CreateSession`: `claude.exe --dangerously-skip-permissions --session-id 4e8cc0b5-…`), so the session id is **deterministically known at launch** — no `ps`/`lsof` discovery, no hook dependency, no newest-file guess. It also knows the working directory (`session.Dir`) and start time. The whole external process-discovery layer that makes Agent-Dashboard Unix-only is unnecessary here — this design is Windows-native by construction.

### The Windows path-encoding finding

Claude encodes the absolute project path into the project directory name by replacing path separators and punctuation with `-`. On Windows this includes the drive colon and backslash, which a Unix-only encoder (like Agent-Dashboard's, which only replaces `/`, `.`, `_`) does not handle. Verified on the target machine:

```
D:\repos\Multiterminal  →  ~/.claude/projects/D--repos-Multiterminal/
```

So the encoder must replace **`:`, `\`, `/`, `.`, `_` → `-`**. The logs for this very project already exist on disk, so the approach is validated on Windows.

## Goals

1. Exact Claude token usage (input / output / cache-creation / cache-read) and computed API-equivalent cost per pane, replacing the regex scrape when JSONL is resolvable.
2. A distinct **working** (generating now) vs. **owes-you** (waiting on user) signal derived from conversation turn-state.
3. A **convergence** warning flag when the agent repeats the same tool 5× in a row (stuck loop).
4. Zero regression: non-Claude panes and unresolvable Claude sessions keep today's screen-scraping behavior via a soft fallback.

## Non-Goals (deferred follow-ons)

- Rendering subagents and todos in the UI (they are parsed into `SessionData` and shipped on the activity event, but UI visualization is a separate plan).
- `linesAdded` / `filesModified` / `gitCommits` / `toolErrors` extraction (cheap to add to the same parse pass later; no foundation value).
- A real-time fsnotify watcher subsystem (Approach C). Polling within the existing scan loop is the foundation; the package API must not preclude a later watcher.
- Multi-provider (Codex/Gemini) JSONL parsing.

## Architecture

New leaf package **`internal/terminal/claudelog`** — depends only on the standard library, no backend imports, independently testable. Max 300 lines per file (project rule).

| File | Responsibility |
|---|---|
| `encode.go` | Windows-aware path encoder + JSONL session-file resolution |
| `parse.go` | JSONL line parsing: token sum, turn-state, tool tracking, subagents, todos |
| `pricing.go` | `MODEL_PRICING` table + cost computation |
| `types.go` | `SessionData` and nested result types |
| `*_test.go` | per-file table tests + `testdata/*.jsonl` fixtures |

### Data model (`types.go`)

```go
type TokenUsage struct {
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
}

type PendingTool struct {
    ID      string
    Tool    string
    Pattern string // command or file path the agent is blocked on
}

type SubAgent struct {
    ID              string
    Type            string
    Status          string
    CurrentAction   string
    TokensUsed      int
    DurationSeconds int
}

type Todo struct {
    Subject string
    Status  string // pending | in_progress | completed
}

type SessionData struct {
    SessionID       string
    Model           string
    Tokens          TokenUsage
    CostUSD         float64
    TurnOpen        bool          // agent owes the next step
    PendingTool     *PendingTool  // last tool_use without a matching tool_result
    Convergence     bool          // last 5 tool calls identical
    ConvergenceTool string
    Subagents       []SubAgent
    Todos           []Todo
    UpdatedAt       time.Time
}
```

## Components

### 1. Encoder + resolver (`encode.go`)

```go
func EncodePath(absPath string) string            // replace : \ / . _ → -
func ConfigDir(ptyEnv []string) string            // CLAUDE_CONFIG_DIR or ~/.claude
func ResolveSessionFile(configDir, cwd, sessionID, hookSessionID string, startedAt time.Time) (string, bool)
```

Resolution order (most authoritative first):

1. Project dir = `{configDir}/projects/{EncodePath(cwd)}`.
2. **`sessionID` from the session's own launch argv** (MTUI passes `--session-id <uuid>`) → `{projDir}/{sessionID}.jsonl`. This is the primary, deterministic path.
3. Fallback `hookSessionID` (if set by hooks and `sessionID` was unavailable) → `{projDir}/{hookSessionID}.jsonl`.
4. Last-resort fallback → list `{projDir}/*.jsonl`, pick newest mtime where `mtime >= startedAt`. Covers sessions MTUI did not launch with an explicit id (e.g. restored/attached).
5. cwd empty or `/` → no file (skip).

> Implementation note: the session id must be threaded from the launch argv onto the `Session` struct (parse the `--session-id` value in `Start`, store it). This is a small prerequisite tracked in the implementation plan.

### 2. Parser (`parse.go`)

Two-tier read (pattern proven by Agent-Dashboard's parser):

- **Tail read — last 32 KB** for live state: pair `tool_use`↔`tool_result`, record the trailing entry type, the last N tool names, subagent references, todos (from `TodoWrite` tool inputs), and the model.
  - `TurnOpen = (lastEntryType == "user") || (PendingTool != nil)`.
  - `PendingTool` = the most recent `tool_use` with no matching `tool_result`.
  - `Convergence` = the last 5 recorded tool names are all identical → set flag + `ConvergenceTool`.
- **Full-file token sum** for authoritative totals: byte-level prefilter (only unmarshal lines containing `"usage"` or `"compact_boundary"`), sum every assistant message's per-message usage across the whole file (compaction-safe). The 32 KB tail cannot be trusted for totals on long/compacted sessions.

Robustness: the JSONL schema is undocumented and version-dependent. Tolerate unknown fields; on any parse error, return `(nil, false)` so the caller falls back to screen-scraping. Never panic, never emit zeroed-out data that would overwrite good screen-scrape values.

### 3. Pricing (`pricing.go`)

```go
var MODEL_PRICING = map[string]Pricing{ /* per-1M-token rates incl. cache tiers */ }
func CostUSD(model string, t TokenUsage) float64
```

Cost is **API-equivalent estimate**, not actual billing for Pro/Max plans — surfaced as such in the UI.

### 4. Caching

Cache the resolved file path + file identity (size, mtime) keyed by **MTUI session ID**. The expensive full-file token scan runs only on a cache miss (file changed). TTL ≈ the scan interval. The package API must remain pure (`Read(args) → SessionData`) so a later fsnotify watcher can replace polling without changing callers.

## Data flow (scan-loop integration)

In `scanAllSessions` (`internal/backend/app_scan.go`), for Claude panes:

1. `data, ok := claudelog.Read(session)`.
2. **Tokens/cost:** if `ok` → set `s.Tokens` from `data` (four counters) and cost from `data.CostUSD`. Else → keep today's `ScanTokens()` regex as fallback.
3. **Activity — additive priority, existing logic preserved:**
   1. **Hooks** (`HasHookData()`) — unchanged, highest authority (permission/stop).
   2. **JSONL turn-state** — *new*: provides the fine **working** (generating) vs. **owes-you** (waiting on user) distinction. Fused with PTY liveness: recent PTY output (existing `LastOutputAt` < ~1.5 s gate) → generating; turn closed + quiesced → done/waiting.
   3. **Screen-scrape** (`DetectActivity`) — fallback for non-Claude / unresolved sessions.
4. **Convergence** is a separate warning flag (not an `ActivityState`) surfaced as a badge.

### Struct & binding changes

- Extend `terminal.TokenInfo` with `CacheCreationTokens`, `CacheReadTokens`.
- Extend the `ActivityInfo` event payload with `working bool` and `convergence bool` (and ship parsed subagents/todos for the follow-on UI).
- **Mandatory (per CLAUDE.md & memory — recurring silent-strip bug):** manually mirror every new field into `frontend/wailsjs/go/models.ts` (class field declaration + constructor assignment). Tracked as its own plan step.

## Testing

- **Encoder:** table tests with Windows paths (`D:\repos\…`, `C:\Users\…`), a worktree path, empty cwd.
- **Resolver:** hook-id path vs. newest-file path; two sessions in one cwd disambiguated by `startedAt`; missing project dir.
- **Parser:** token sum across a compaction boundary; turn-state open/closed; pending tool; convergence (5 identical / not); subagents; todos. Own `testdata/*.jsonl` fixtures.
- **Pricing:** per-model incl. cache tiers; unknown model → 0 (no crash).
- **Integration:** a fake `~/.claude/projects/…` tree + a fake `Session` with `Dir` set → assert `SessionData`.
- **Fallback:** corrupt/truncated JSONL → `(nil, false)`, no crash, screen-scrape still drives state.
- `go test ./internal/terminal/...` green under the race detector.

## Risks

- **Schema drift:** Claude's JSONL format may change between versions → isolated parsing, tolerant unmarshal, soft fallback.
- **Cost accuracy:** estimate only, not real Pro/Max billing → label in UI.
- **Performance:** full-file scan on large sessions → mitigated by the size/mtime cache and byte-level prefilter; full scan runs only on change.
- **Config override:** `CLAUDE_CONFIG_DIR` must be read from the PTY environment, not assumed to be `~/.claude`.

## Follow-ons (out of scope here)

1. Subagent + todo visualization in the pane / future Kanban card.
2. `linesAdded` / `filesModified` / `gitCommits` / `toolErrors` extraction (same parse pass).
3. fsnotify watcher subsystem for append-time updates (Approach C).
4. Multi-provider (Codex/Gemini) session resolution.
