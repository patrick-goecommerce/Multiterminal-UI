# Claude telemetry capture — statusline-primary, JSONL-enrichment (Foundation)

- **Date:** 2026-06-23
- **Status:** Rev 4 — re-sourced after a clean-start senior review + a verified spike. Supersedes the JSONL-first design of Rev 1–3.1.
- **Branch target:** `alpha-main` (feature branch off it)
- **One-line goal:** Capture accurate Claude cost / context / model from the source MTUI already controls (its installed statusline), attributed to the exact pane; enrich with JSONL-derived signals (a "working" turn-state badge, convergence warning) only where the statusline can't provide them — without ever perturbing the pipeline `active→done` edge.

> **Why Rev 4 (the pivot).** Rev 1–3.1 built a transcript-JSONL reader (path encoder, file resolver, `pricing.go`, dedup-by-id) and red-teamed it across 3 rounds. A 4th, *unprimed* review found the design committed to the hardest, most fragile data source while cheaper, already-integrated sources sat unused in the same codebase — and that the JSONL token sum **undercounts cost by ~40 %+** because subagent spend lives in separate files (measured: main 6.78 M vs subagents 5.09 M tokens on one live session), whereas the thing it replaces (screen-scrape of Claude's printed total) already includes it. A spike then **verified** the better source. This rev re-sources accordingly. The cut machinery (encoder, resolver, pricing table) is gone from the critical path.

---

## Spike findings (verified in code, 2026-06-23)

1. **Attribution is free.** `CreateSession` injects `MULTITERMINAL_SESSION_ID=<id>` and `MTUI_PORT=<port>` into the PTY env (`internal/backend/app.go:205,208`). The statusline script runs as a descendant of the `claude` process inside that PTY, so it **inherits both**. `$env:MULTITERMINAL_SESSION_ID` maps a statusline invocation to the exact MTUI session — no file resolution, no encoder, no mtime race.
2. **The return channel already exists.** A localhost HTTP server runs on `MTUI_PORT` (`internal/backend/app_tmux_api.go`, `http.NewServeMux`, `/api/tmux/log`). Adding `/api/statusline` is the same pattern.
3. **The official numbers are already on the wire.** MTUI installs `~/.claude/mtui-statusline.ps1` (`app_statusline.go`), invoked as `powershell -NonInteractive -NoProfile -File <script>` with Claude's JSON on stdin. The script already reads `$d.cost.total_cost_usd`, `$d.cost.total_duration_ms`, `$d.context_window.used_percentage`, `$d.model.display_name`, `$d.workspace.current_dir` — then `Write-Host`s and **discards** them. `total_cost_usd` is Claude's official cumulative cost (includes subagents). Claude's standard statusline schema also carries `session_id` and `transcript_path` (the absolute path to the session JSONL).
4. **The pipeline `done` edge is independent of token state** (verified across earlier rounds): `app_scan.go:157` drives `processQueue`/`notifyOrchestratorDone` from `DetectActivity`/`HasHookData`; the queue reads `prevActivity`, never `GetTokens`. Feeding cost/tokens from a new source cannot perturb it. This invariant is preserved.

---

## Architecture

Two sources, by what each is best at:

| Signal | Source | Why |
|---|---|---|
| **Total cost** (headline), **context %**, **model**, **duration** | **Statusline push** | Official `total_cost_usd` (incl. subagents); event-driven (no polling); attributed by `MULTITERMINAL_SESSION_ID`; no file resolution |
| **"working" turn-state**, **convergence**, per-counter token breakdown, subagent/todo detail | **JSONL tail** (enrichment) | Not in the statusline payload; read the file located by `transcript_path` (from the statusline payload) or `HookSessionID()` — **never** by path-encoding/resolution |

The statusline push is the primary, simple, accurate path and the value increment. JSONL becomes a smaller, optional enrichment whose file is *handed to us*, so the entire Rev-1–3 resolver/encoder is deleted.

### Data flow — statusline capture (primary)

```
Claude Code ──(JSON on stdin, per turn)──▶ mtui-statusline.ps1
                                              │  reads $env:MULTITERMINAL_SESSION_ID, $env:MTUI_PORT
                                              │  Write-Host (unchanged display)
                                              └─ fire-and-forget POST raw JSON + sessionId
                                                 to http://127.0.0.1:$MTUI_PORT/api/statusline
                                                          │
AppService /api/statusline handler ◀──────────────────────┘
   parse → update Session{Cost, ContextPct, Model, Duration}, Confidence=High, set via s.mu (assignment only)
```

- **Forwarder (`buildStatusLineScript`):** after the existing `Write-Host`, append a guarded POST of the **entire raw stdin JSON** plus the MTUI session id. Must be **fire-and-forget and failure-silent** (try/catch, short timeout) — if MTUI is down or the port is stale, the statusline display must still render. Never let the forward break Claude's statusline.
- **Endpoint (`/api/statusline`):** localhost only (same trust model as `/api/tmux/log`). Body = `{ sessionId int, payload <raw claude statusline json> }`. Handler parses `payload.cost.total_cost_usd`, `payload.context_window.used_percentage`, `payload.model.display_name`, `payload.cost.total_duration_ms`, and (if present) `payload.session_id`, `payload.transcript_path`.
- **Session update:** set the cost/context/model fields on `Session` under `s.mu` (assignment only, no I/O under lock). Mark the cost source authoritative so the scan loop's `ScanTokens()` scrape does not overwrite it (see §Integration).

### Data flow — JSONL enrichment (secondary, smaller sub-plan)

A trimmed `internal/terminal/claudelog` package — **parse only, no resolve/encode**:
- File path is supplied: `transcript_path` from the statusline payload, else `HookSessionID()` → `{configDir}/projects/.../{uuid}.jsonl` (the uuid→filename mapping already used at `app_hooks.go:205`). If neither is known → no enrichment (cost still comes from the statusline).
- Parses, from the tail (last ≤64 KB of complete `\n`-terminated lines):
  - **turn-state** `TurnOpen` (last `user`/`assistant` line owes the next step; metadata line types skipped) → fused with `LastOutputAt` for a **display-only "working"** signal.
  - **convergence** (last 5 tool names identical) → display-only warning.
  - **pending tool** (a `tool_use` whose `tool_use_id` has no later `tool_result` — matched by id, scanning forward, not by adjacency).
- **Token breakdown (optional, if we want the four counters):** full-file dedup-by-`message.id` sum **across the main file AND `{uuid}/subagents/*.jsonl`** (disjoint ids; one map). This is only needed if we surface per-counter tokens; the headline cost comes from the statusline regardless.

## Pinned decisions

| # | Decision | Value |
|---|---|---|
| D1 | Headline cost source | Statusline `total_cost_usd` (official, incl. subagents). No `pricing.go` reconstruction. |
| D2 | Pane attribution | `$env:MULTITERMINAL_SESSION_ID` → MTUI session id. Direct; no JSONL resolution for cost. |
| D3 | Forwarder failure mode | Fire-and-forget, failure-silent, short timeout. Statusline display must never break. |
| D4 | Cost source priority | statusline-push (`High`) > screen-scrape fallback. JSONL is **not** a cost source unless per-counter tokens are explicitly surfaced (then sums subagents too). |
| D5 | "working"/convergence | Display-only signals, **decoupled from the `done` edge** (never trigger `processQueue`). |
| D6 | JSONL file location | `transcript_path` (statusline payload) → else `HookSessionID()`. **No path encoder / cwd+gitBranch / mtime resolver.** |
| D7 | User has own statusline | If MTUI did not install the statusline (`HasExisting` / not `IsOurs`), no push for that setup → fall back to screen-scrape. Documented limitation; offer to install in settings UI. |
| D8 | Subagents/todos detail | Deferred to the visualization follow-on; not parsed in the foundation. |

## Sub-plans

**Sub-plan 1 — Statusline capture (the value increment).**
- Extend `buildStatusLineScript` with the guarded forwarder (D3).
- Add `/api/statusline` to the existing localhost server; parse payload; update `Session` cost/context/model/duration (D1, D2), assignment under `s.mu` only.
- Gate the existing `ScanTokens()` scrape so it does not overwrite a statusline-sourced cost (define one authoritative writer per tick).
- Extend `ActivityInfo`/`TokenInfo` as needed for context %/model; **verify** whether the new fields cross a binding return (then sync `models.ts` per CLAUDE.md) or only an event (no sync) — they currently flow as the `terminal:activity` event payload, so likely no `models.ts` class change.
- **Tests:** endpoint parses a real captured statusline JSON → correct Session fields; missing/garbage payload → no crash, no overwrite; the **`done`-edge regression guard** (replay activity transitions, assert `processQueue` fires exactly once per turn, unaffected by cost updates); forwarder POST is non-blocking.
- **Deliverable:** accurate official cost + context % + model per pane, attributed exactly. Replaces the fragile regex for installed-statusline panes.

**Sub-plan 2 — JSONL enrichment: "working" + convergence.**
- Trimmed `claudelog` package: `parse.go` (tail framing, turn-state on user/assistant lines, pending-tool by forward id-match, convergence) + `types.go`. **No `encode.go`/`resolve.go`/`pricing.go`.**
- File path from `transcript_path` or `HookSessionID()` (D6).
- Tail read off the scan-loop goroutine (per-session goroutine; non-blocking `Latest()`); display-only `working`/`convergence` fused with `LastOutputAt`, priority hooks > JSONL > scrape, **never feeding the authoritative activity state** (D5).
- Extend `ActivityInfo` with `working bool`, `convergence bool`; transition-table tests asserting these never trigger `processQueue`.
- **Deliverable:** live "generating" badge + stuck-loop warning.

**Sub-plan 3 — Subagent/todo detail + UI (follow-on).**
- Per-counter token breakdown incl. subagents (dedup-by-id across main + `subagents/*.jsonl`), subagent roster, todo list, and their visualization. Captures the deferred D8 work when there's a consumer.

## Risks (residual)

- **Statusline coverage:** only panes whose statusline MTUI installed push data (D7). Mitigation: screen-scrape fallback + an install prompt. Quantify in practice; the default MTUI-managed setup covers the common case.
- **Statusline cadence:** updates per Claude turn, not continuously — fine for cost; live "working" comes from JSONL/PTY, not the statusline.
- **JSONL schema drift (Sub-plan 2 only):** isolated tail parser, tolerant unmarshal, soft fallback; far smaller surface now that resolution/pricing are gone.
- **Localhost endpoint:** loopback only, same trust model as the existing tmux API; payload (cost/cwd) is local.

## Follow-ons (out of scope)

1. Subagent + todo visualization and per-counter token breakdown (Sub-plan 3).
2. `linesAdded`/`filesModified`/`gitCommits` (the statusline payload exposes some of these — `cost.total_lines_added/removed` — capture opportunistically later).
3. fsnotify watcher for the enrichment tail (replaces the per-session goroutine's polling).
4. Multi-provider (Codex/Gemini) — they have their own statusline/telemetry stories.

## Build order

**Sub-plan 1 (value lands here) → Sub-plan 2 → Sub-plan 3.** Every step keeps the branch green: any pane without a captured statusline push or resolvable JSONL keeps today's screen-scrape behavior by construction (zero-regression goal).
