# claudelog — JSONL Session Reader (Foundation)

- **Date:** 2026-06-23
- **Status:** Rev 2 — revised after red-team round 1 (empirical findings against real logs)
- **Branch target:** `alpha-main` (feature branch off it)
- **Scope:** Foundation only. Replaces fragile screen-scraping of Claude token/cost data with a structured reader of Claude Code's JSONL session logs, and adds a turn-state–based "working" display signal plus a convergence ("stuck loop") warning. Subagents and todos are parsed but not yet rendered (explicit follow-on).

> **Rev 2 note.** Round 1 of red-teaming verified the original assumptions against the real logs on disk and refuted several. The biggest: Claude writes **one JSONL line per content block, each repeating the same `usage` object**, so a naive whole-file sum over-counts tokens/cost by ~2.6× — and the Agent-Dashboard parser this spec originally cited as "proven" has exactly that bug. Treat AD as a structural reference only, not a correctness oracle. All corrections are folded in below and called out with **[R1]**.

## Context

MTUI currently derives Claude token/cost/activity data by scraping the **rendered terminal screen** (`internal/terminal/activity.go`):

- `ScanTokens()` matches `$\d+\.\d+` for cost and `"15.2k input"` / `"3.8k output"` regexes — only two token counters, no cache tokens, dependent on whatever Claude happens to print.
- `DetectActivity()` / `classifyScreenState()` classify activity from prompt/question heuristics over the last 15 screen rows.

Claude Code already writes complete structured data to `~/.claude/projects/{encoded_path}/{sessionId}.jsonl`. Reading it directly yields exact tokens, per-model cost, subagent state, todos, and a turn-state — **if parsed correctly** (see the format hazards below).

**MTUI's structural advantage:** MTUI owns the PTY. It can know each session's working directory (`session.Dir`), and — when it launches Claude itself — the session id and start time. This removes the `ps`/`lsof` discovery layer that makes external monitors Unix-only; this design is Windows-native by construction. **[R1] But "MTUI always knows the session id at launch" is false** for several real launch paths (worktree panes, background agents, keepalive, restored sessions via `--resume`, and `claude` typed manually in a shell pane). The resolver must not assume it (see Components §1).

### Format hazards verified against real logs **[R1]**

Verified against `C:\Users\PatrickHenniggoeComm\.claude\projects\D--repos-Multiterminal\` (64 jsonl files, 6 branches) and sibling projects:

1. **Per-block usage duplication (the critical one).** A single API response (one `message.id`, one `requestId`, one `usage`) is written as multiple JSONL lines — one per content block (`thinking`, `text`, `tool_use`) — and **each line repeats the identical `usage`**. Measured over-count: 2.0–2.7× across every file. The token sum **must deduplicate by `message.id`** (count each distinct id's usage once; occurrences are identical).
2. **~10 line types, not 3.** Besides `assistant`/`user`/`system`, real files contain `mode`, `permission-mode`, `attachment`, `last-prompt`, `ai-title`, `file-history-snapshot`, `queue-operation`. Trailing lines are usually one of these, NOT `user`/`assistant`. Turn-state logic must consider only `user`/`assistant` lines and skip the rest.
3. **Partial trailing line is the steady state during generation** — the live file's last bytes are an unterminated append on nearly every read. This is normal, not an error.
4. **Token field names confirmed exact** on `type:"assistant"` `message.usage`: `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`. Model at `message.model` (e.g. `claude-opus-4-8`).
5. **tool_use/tool_result pairing confirmed:** `tool_use` (with `id`) in an assistant line's `content[]`; matching `tool_result` (with `tool_use_id`) in the next `user` line's `content[]`.
6. **compact_boundary confirmed** (`type:"system", subtype:"compact_boundary"`). Per-message usage is per-turn (not cumulative), so dedup-by-id summation is compaction-safe.
7. **Subagents:** live in `{sessionId}/subagents/*.jsonl` with a sibling `*.meta.json` (`agentType`, `description`, `toolUseId`). There is **no `Status` field** in the data — status must be derived. Confirmed present on this machine.
8. **TodoWrite todos:** shape (`tool_use.input.todos[] = {content, status, ...}`) is unconfirmed on this machine — no real TodoWrite call exists in local logs. A fixture from a real TodoWrite session is required before claiming todos work.

### The Windows path-encoding finding

Claude encodes the absolute project path into the directory name by replacing separators/punctuation with `-`. Verified: `D:\repos\Multiterminal` → `D--repos-Multiterminal`; `D:\repos\Multiterminal\.worktrees\chat-pane-display-mode` → `D--repos-Multiterminal--worktrees-chat-pane-display-mode`. **[R1]** The real encoder also replaces **`~`** (real dir `C--Users-PATRIC-1-…` came from `PATRIC~1`). So the set is **`: \ / . _ ~ → -`**. The encoding is **non-injective** (`.`, `_`, `-`, separators all collapse to `-`): `D:\repos\a.b`, `…\a_b`, `…\a-b` all encode to `D--repos-a-b`. The resolver must treat a >1-candidate match as ambiguous (see §1).

## Goals

1. Exact Claude token usage (four counters, **deduped by `message.id`**) and computed API-equivalent cost per pane, replacing the regex scrape when the JSONL is confidently resolved.
2. A distinct **working** (generating) display signal derived from turn-state fused with PTY liveness — **as a display hint only, decoupled from the pipeline `done` edge**.
3. A **convergence** warning flag when the agent repeats the same tool 5× in a row.
4. Zero regression: non-Claude panes, unresolvable/ambiguous Claude sessions, and torn reads keep today's screen-scraping behavior via a soft fallback. **The pipeline queue/orchestrator `active→done` edge must behave byte-identically to today.**

## Non-Goals (deferred follow-ons)

- Rendering subagents/todos in the UI (parsed and shipped on the event; visualization is a separate plan).
- `linesAdded` / `filesModified` / `gitCommits` / `toolErrors` extraction.
- fsnotify watcher subsystem. (But the off-loop reader below is the stepping stone to it.)
- Multi-provider (Codex/Gemini) parsing.

## Prerequisites (small enabling changes, tracked in the plan) **[R1]**

These do not exist today and must land first:

1. **`Session.SessionID string`** — the Claude session id. Source of truth is the frontend tab-store pane field `claudeSessionId` (`frontend/src/stores/tabs.ts:25`); thread it through `CreateSession(...)` as an explicit parameter onto the struct. Do **not** rely on argv-parsing as the primary source. (Argv-parse in `Start` may serve as a secondary recovery, and must handle both `--session-id <uuid>` and `--session-id=<uuid>`.)
2. **`Session.StartedAt time.Time`** — set in `Start()`. Required by the fallback resolver; only `LastOutputAt` exists today.
3. **Pin the id on id-less launch sites** so they become resolvable: worktree panes (`App.svelte:392`), background agents (`background-agents.ts:56`), keepalive (`keepalive.ts:46`) must pass a generated `sessionId`.
4. **Persist/expose `CLAUDE_CONFIG_DIR`** per session if MTUI is to honor an override: MTUI does not set it today and `Start` keeps the assembled env only as a local. Either capture the relevant env onto the session or accept `~/.claude` as the only supported location in this foundation (and document the limitation). Default decision: **support `~/.claude` and an explicit `CLAUDE_CONFIG_DIR` read from the MTUI process env**; per-session overrides are out of scope for the foundation.

## Architecture

New leaf package **`internal/terminal/claudelog`** — stdlib only, no backend imports, independently testable. Max 300 lines per file.

| File | Responsibility |
|---|---|
| `encode.go` | path encoder (`: \ / . _ ~ → -`), normalization, config-dir resolution |
| `resolve.go` | session-file resolution + ambiguity detection |
| `parse.go` | line framing, token sum (dedup by message.id), turn-state, tool tracking, subagents, todos |
| `pricing.go` | `MODEL_PRICING` table + cost computation |
| `reader.go` | per-session off-loop reader: cache, incremental offset, last-known-good `SessionData` |
| `types.go` | result types |
| `*_test.go` | per-file table tests + `testdata/*.jsonl` fixtures |

### Data model (`types.go`)

```go
type TokenUsage struct {
    InputTokens, OutputTokens, CacheCreationTokens, CacheReadTokens int
}
type PendingTool struct { ID, Tool, Pattern string }
type SubAgent struct { ID, Type, Description string; ToolUseID string; TokensUsed, DurationSeconds int; Status string /* derived */ }
type Todo struct { Content, Status string }

type SessionData struct {
    SessionID, Model string
    Tokens           TokenUsage   // deduped by message.id
    CostUSD          float64
    TurnOpen         bool         // agent owes next step; user/assistant lines only
    PendingTool      *PendingTool
    Convergence      bool
    ConvergenceTool  string
    Subagents        []SubAgent
    Todos            []Todo
    Confidence       Confidence   // High (id match) | Low (fallback guess)  [R1]
    UpdatedAt        time.Time
}
```

`Confidence` lets the caller refuse to overwrite good data with a low-confidence fallback guess.

## Components

### 1. Encoder + resolver (`encode.go`, `resolve.go`) **[R1]**

```go
func EncodePath(absPath string) string   // normalize then replace : \ / . _ ~ → -
func ConfigDir() string                  // CLAUDE_CONFIG_DIR (process env) or ~/.claude
func ResolveSessionFile(configDir, cwd, sessionID string, startedAt time.Time) (path string, conf Confidence, ok bool)
```

`EncodePath` first **normalizes**: strip trailing separators, no case-fold of the drive letter is applied to the *encoded* string (NTFS lookup is case-insensitive, but the cache key must use the normalized form consistently).

Resolution order:

1. Project dir = `{configDir}/projects/{EncodePath(cwd)}`. cwd empty/`/` → not ok.
2. **`sessionID` known** → `{projDir}/{sessionID}.jsonl` if it exists → `Confidence=High`. (Restored sessions resolve here too: `--resume <id>` keeps the same `<id>.jsonl`, verified.)
3. **Unknown id** → list `{projDir}/*.jsonl`. To pick:
   - Read the **last complete line** of each candidate and match its embedded `cwd` **and** `gitBranch` against the session's dir/branch. This kills cross-branch contamination (the dir mixes 6 branches' files).
   - Among matches, require mtime **strictly >** `startedAt` (sub-second; Go exposes ns mtime on NTFS) and pick the newest. → `Confidence=Low`.
   - If **zero or >1** candidates survive matching → `ok=false` (ambiguous) → caller keeps screen-scrape. Never guess under ambiguity.

### 2. Parser (`parse.go`) **[R1]**

**Line framing first:** split the read buffer on `\n`; **discard any bytes after the last `\n`** as an incomplete append. Never `Unmarshal` a partial line; never let a partial trailing line trigger the error/fallback path. Only a *complete* line that fails to parse counts as corruption.

- **Token total — dedup by `message.id`:** scan all complete lines; for each `type:"assistant"` line, key by `message.id` and record its `usage` once (occurrences are identical). Sum over distinct ids. Byte-prefilter on `"usage"`/`"compact_boundary"` to skip irrelevant lines cheaply. Compaction-safe (per-turn usage). **Validate the dedup with a multi-block fixture** — a same-`message.id`-across-3-lines case — or the test passes while still 2.6× wrong.
- **Turn-state:** track the last line whose type ∈ {`user`,`assistant`}; ignore all metadata types. `TurnOpen = (lastRelevantType == "user") || (PendingTool != nil)`. `PendingTool` = most recent `tool_use` with no matching `tool_result`.
- **Convergence:** last 5 recorded tool names identical → flag + name.
- **Subagents:** enumerate `{projDir}/{sessionID}/subagents/*.jsonl` + `*.meta.json` sidecars; `Type←agentType`, link via `toolUseId`; `Status` **derived** (e.g. process/pid liveness or presence of a terminal record), not read.
- **Todos:** last-write-wins over `TodoWrite` `tool_use.input.todos[]`. Gated behind a real fixture before being trusted.

Robustness: tolerate unknown fields; any I/O error (open/stat/read) or corrupt complete line → return last-known-good with `ok=false`, never panic, never log-spam.

### 3. Pricing (`pricing.go`)

`MODEL_PRICING` map of per-1M-token rates incl. cache tiers; `CostUSD(model, TokenUsage) float64`. Unknown model → 0, no crash. Cost is an **API-equivalent estimate**, labeled as such in the UI.

### 4. Off-loop reader + cache (`reader.go`) **[R1 — the key integration fix]**

The reader does **not** run on the scan-loop goroutine. Each Claude session gets a lightweight reader that:

- Holds the last-known-good `SessionData`, published behind a small mutex / atomic pointer.
- Refreshes on its own cadence (a worker pool with a hard per-read deadline, or a per-session goroutine), so a slow/large file never blocks other panes.
- Uses an **incremental byte offset**: remember the last offset parsed; on refresh, read only appended bytes for the running token delta and tail state. A full recompute runs at a slow cadence (e.g. every N seconds), not every tick. This is the actual fix for "continuous append = permanent cache miss."
- **Cache invalidation primarily on file size** (monotonic for an append log; more reliable than NTFS mtime). Tolerate **shrink/truncate** (compaction rewrites the file) by resetting the offset and re-reading.
- **Eviction:** hook teardown into the existing `cleanupActivityTracking(id)` call site so closed sessions don't leak cache entries.

`scanAllSessions` only ever reads the published last-known-good struct — **non-blocking**.

## Data flow (scan-loop integration) **[R1]**

In `scanAllSessions` (`internal/backend/app_scan.go`), for Claude panes:

1. `data, ok := reader.Latest(session)` — non-blocking read of last-known-good.
2. **Tokens/cost:** if `ok && data.Confidence==High` → set `s.Tokens` from `data` + `data.CostUSD`. Else (`Low`/`!ok`) → keep today's `ScanTokens()` regex; do **not** overwrite good values with a low-confidence guess.
3. **Lock discipline:** the file read/parse happens entirely in the reader, off-loop and outside any session lock. `scanAllSessions` acquires `s.mu` **only** for the final `s.Tokens = …` assignment and releases immediately. Never hold `s.mu` across I/O (`activity.go` `ScanTokens` shape must not be extended to read files under its lock).

**Activity — the `done` edge is sacrosanct:**

- The pipeline edge (`activityChanged && actStr=="done"` → `processQueue` + `notifyOrchestratorDone`, `app_scan.go:157`) stays driven by the **existing** `DetectActivity` path, debounced by the existing 1.5s PTY-quiescence gate. **JSONL turn-state does NOT move or replace this edge in the foundation.**
- The new **`working`** signal (JSONL `TurnOpen` fused with `LastOutputAt`) and **`convergence`** flag are **display-only**, emitted as additional fields on the activity event. They never trigger `processQueue`/`notifyOrchestratorDone`.
- Priority for the *display* `working`/state hint: hooks (`HasHookData`) > JSONL (when `Confidence==High`) > screen-scrape. The *authoritative* activity state that gates the queue is unchanged.

A transition table for the display hint (inputs: hasHookData, JSONL ok+Confidence, TurnOpen, LastOutputAt<1.5s → output state) is defined in the plan and covered by tests.

### Struct & binding changes

- Extend `terminal.TokenInfo` with `CacheCreationTokens`, `CacheReadTokens`.
- Extend the `ActivityInfo` event payload with `working bool`, `convergence bool` (+ parsed subagents/todos for the follow-on UI).
- **Mandatory (CLAUDE.md & memory — recurring silent-strip bug):** mirror every new field into `frontend/wailsjs/go/models.ts` (class field + constructor assignment). Tracked as its own plan step.

## Testing **[R1 — adds the edge tests the v1 plan lacked]**

- **Encoder:** Windows paths (`D:\…`, `C:\…`), `~`/8.3 (`PATRIC~1`→`PATRIC-1`), worktree paths, trailing separator, collision cases asserted as ambiguous.
- **Resolver:** id-known High path; id-unknown fallback with cwd+gitBranch match; **two sessions same cwd different branch** → correct file or ambiguous; >1 match → `ok=false`; missing dir.
- **Parser — token dedup:** a **multi-block, same-`message.id`** fixture asserting the sum counts the message once (the regression guard for the 2.6× bug). Plus compaction-boundary fixture.
- **Parser — framing:** a fixture whose last line is a partial append → parser ignores it, returns valid data for the complete prefix, `ok` stays true.
- **Parser — turn-state:** trailing metadata lines (`mode`, `system`, …) must not flip `TurnOpen`; user-trailing → open; pending-tool → open; assistant-final → closed. Convergence 5-identical / not.
- **Subagents:** `.meta.json` sidecar parsing; derived status. **Todos:** behind a real fixture (acquire one first).
- **Pricing:** per-model incl. cache tiers; unknown → 0.
- **Scan-loop integration (the highest-stakes test):** replay an append-by-append JSONL stream through the actual activity-derivation and assert `processQueue` fires **exactly once per turn** — never zero, never twice — and that the `working`/`convergence` display fields never trigger it.
- **Race:** run the reader concurrently with a `readLoop`-style writer to `s.LastOutputAt`/`s.Tokens` under `-race`; assert no lock held across I/O.
- **Fallback:** corrupt complete line + missing file → `ok=false`, no crash, screen-scrape drives state.

## Risks (residual, after fixes)

- **Schema drift:** Claude's JSONL is undocumented and version-dependent → isolated parser, tolerant unmarshal, soft fallback, fixtures pinned to a known CLI version.
- **Low-confidence misattribution:** mitigated by cwd+gitBranch matching and the "ambiguous → don't guess" rule; residual risk when two sessions on the same branch+cwd run concurrently without ids → falls back to screen-scrape (acceptable).
- **Cost accuracy:** estimate only, labeled.
- **Todos unverified locally** until a real fixture is captured.

## Follow-ons (out of scope here)

1. Subagent + todo visualization (pane / future Kanban card).
2. `linesAdded`/`filesModified`/`gitCommits`/`toolErrors`.
3. fsnotify watcher (replaces the off-loop reader's polling without changing callers).
4. Multi-provider (Codex/Gemini) resolution.
