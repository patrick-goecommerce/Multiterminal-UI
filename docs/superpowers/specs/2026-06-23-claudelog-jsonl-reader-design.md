# Claude telemetry capture — statusline-primary, JSONL-enrichment (Foundation)

- **Date:** 2026-06-23
- **Status:** Rev 4 — re-sourced after a clean-start senior review + a verified spike. Supersedes the JSONL-first design of Rev 1–3.1.
- **Branch target:** `alpha-main` (feature branch off it)
- **One-line goal:** Capture accurate Claude cost / context / model from the source MTUI already controls (its installed statusline), attributed to the exact pane; enrich with JSONL-derived signals (a "working" turn-state badge, convergence warning) only where the statusline can't provide them — without ever perturbing the pipeline `active→done` edge.

> **Why Rev 4 (the pivot).** Rev 1–3.1 built a transcript-JSONL reader (path encoder, file resolver, `pricing.go`, dedup-by-id) and red-teamed it across 3 rounds. A 4th, *unprimed* review found the design committed to the hardest, most fragile data source while cheaper, already-integrated sources sat unused in the same codebase — and that the JSONL token sum **undercounts cost by ~40 %+** because subagent spend lives in separate files (measured: main 6.78 M vs subagents 5.09 M tokens on one live session), whereas the thing it replaces (screen-scrape of Claude's printed total) already includes it. A spike then **verified** the better source. This rev re-sources accordingly. The cut machinery (encoder, resolver, pricing table) is gone from the critical path.

---

## Spike findings (verified in code, 2026-06-23)

1. **Attribution is (almost) free.** `CreateSession` injects `MTUI_PORT=<port>` unconditionally (when the API is up) and `MULTITERMINAL_SESSION_ID=<id>` **for claude modes only** (`claude`/`claude-auto`/`claude-yolo` — `internal/backend/app.go:204-209`; it is NOT injected for shell/codex/gemini). Since only claude panes run the statusline, the gated set is exactly the set that matters. The statusline command runs as a descendant of the `claude` process inside that PTY, so it inherits both vars. `$MULTITERMINAL_SESSION_ID` maps a statusline invocation to the exact MTUI session — no file resolution, no encoder, no mtime race. **Caveat:** the PTY→`cmd.exe /c claude`→statusline env inheritance is inferred from code, **not yet run end-to-end** — Sub-plan 1 must verify it at runtime (see test list).
2. **The return channel already exists, and so does the right forwarder pattern.** A localhost HTTP server runs on `MTUI_PORT` (`internal/backend/app_tmux_api.go`, `http.NewServeMux`, `/api/tmux/log`). MTUI already ships a **compiled Go shim** for exactly this kind of fire-and-forget POST: `cmd/tmux-shim/main.go` reads `MTUI_PORT` from env at runtime (line 35) and POSTs with `http.Client{Timeout: 2s}` (line 215). The exe directory is prepended to the PTY `PATH` (`session.go:123-133`), so a sibling forwarder binary is callable by name. Adding `/api/statusline` is the same pattern.
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
Claude Code ──(JSON on stdin, per turn)──▶ statusline command
                                              = mtui-statusline-forward.exe (Go shim)
                                              │  reads $MTUI_PORT, $MULTITERMINAL_SESSION_ID (env, fresh)
                                              │  tees stdin: ① pass through to the real statusline
                                              │             (MTUI's PS1, or the user's pre-existing one)
                                              │             → its stdout is the displayed status line
                                              │  ② fire-and-forget POST {sessionId, rawJSON}
                                              │     to http://127.0.0.1:$MTUI_PORT/api/statusline (2s client)
                                              └────────────────────────────┐
AppService /api/statusline handler ◀──────────────────────────────────────┘
   parse → SetStatuslineData(sessionId, cost, ctx%, model, dur), costSource=statusline (assignment under s.mu)
```

- **Forwarder = a new compiled Go shim** `cmd/statusline-forward` (NOT PowerShell). PowerShell's `Invoke-RestMethod` is blocking and Claude **cancels the in-flight statusline command** if a new update arrives, so a blocking/hanging POST is actively harmful. The Go shim mirrors `cmd/tmux-shim`: reads `MTUI_PORT`/`MULTITERMINAL_SESSION_ID` from env, POSTs with a short-timeout `http.Client`, and **fails silently** (MTUI down / stale port → display still renders). Built alongside the main binary, lands in the exe dir already on `PATH`.
- **Wrap, don't replace [fixes D7].** The shim **tees**: it forwards a copy of stdin to MTUI *and* pipes stdin to the real statusline command, relaying that command's stdout/exit so the user sees an unchanged status line. `applyStatusLine` registers the shim as the `statusLine.command`, passing the *previous* command (MTUI's `mtui-statusline.ps1`, or the user's pre-existing one captured from `ExistingCommand`) as the wrapped target. This makes capture work **regardless of whose statusline runs** — the power-user-with-custom-statusline case (previously a silent value-killer) is now covered.
- **Endpoint (`/api/statusline`):** localhost only, POST-only, no auth/CORS (same trust model as `/api/tmux/log`; not browser-originated). Body = `{ sessionId int, payload <raw claude statusline json> }`. Handler parses `payload.cost.total_cost_usd`, `payload.context_window.used_percentage`, `payload.model.display_name`, `payload.cost.total_duration_ms`, and (if present) `payload.session_id`, `payload.transcript_path`. Finds the session via `a.sessions[id]` under `a.mu`.
- **Session update:** a new `Session.SetStatuslineData(...)` setter (mirroring `SetHookActivity` in `session_hooks.go`) writes new `Session` fields (`StatuslineCost float64`, `ContextPct int`, `Model string`, + cache counters as available) under `s.mu` — assignment only, no I/O under lock — and sets `costSource = statusline` so the scan loop's `ScanTokens()` cannot clobber it (see Integration).

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
| D2 | Pane attribution | `$MULTITERMINAL_SESSION_ID` (env, claude-modes-only — `app.go:207`) → MTUI session id. Direct; no JSONL resolution for cost. |
| D3 | Forwarder | A **compiled Go shim** `cmd/statusline-forward` (not PowerShell — Claude cancels in-flight statusline commands, so a blocking POST is harmful). Reads `MTUI_PORT` fresh from env, short-timeout `http.Client`, fails silently. **Tees** stdin to the wrapped real statusline so display is unchanged. |
| D3b | Cross-restart staleness | Env vars are fixed per process; a claude pane from a previous app run holds the old `MTUI_PORT` and posts to a now-dead port → capture silently stops for that pane until relaunch (no corruption). Documented; acceptable. |
| D4 | Cost source priority | statusline-push (`High`) > screen-scrape fallback. JSONL is **not** a cost source unless per-counter tokens are explicitly surfaced (then sums subagents too). |
| D5 | "working"/convergence | Display-only signals, **decoupled from the `done` edge** (never trigger `processQueue`). |
| D6 | JSONL file location | `transcript_path` (statusline payload) → else `HookSessionID()`. **No path encoder / cwd+gitBranch / mtime resolver.** |
| D7 | User has own statusline | **Wrap it, don't skip it.** `applyStatusLine` registers the Go forwarder as `statusLine.command` and passes the previous command (`ExistingCommand`, or MTUI's PS1) as the wrapped target; the forwarder tees. Capture works for users with custom statuslines (ccusage/starship/etc.) too — the target audience. Only a user who removes the statusline entirely loses capture → screen-scrape fallback. |
| D8 | Subagents/todos detail | Deferred to the visualization follow-on; not parsed in the foundation. |

## Sub-plans

**Sub-plan 1 — Statusline capture (the value increment).**
- **Build `cmd/statusline-forward` (Go shim, D3):** reads `MTUI_PORT` + `MULTITERMINAL_SESSION_ID` from env; tees stdin → (a) pipes to the wrapped real statusline command (relaying its stdout + exit code), (b) fire-and-forget POST `{sessionId, rawJSON}` to `/api/statusline` with a short-timeout client, failing silently. Build it alongside the main binary so it lands in the exe dir (already on the PTY `PATH`).
- **`applyStatusLine` wraps instead of installs-only-when-absent (D7):** register the forwarder as `statusLine.command`, passing the previous command (`ExistingCommand` if a user statusline exists, else MTUI's `mtui-statusline.ps1`) as the wrapped target.
- **Add `/api/statusline`** to the existing localhost server (POST-only, loopback, no auth); parse payload; call a new `Session.SetStatuslineData(...)`.
- **New `Session` fields + setter:** `StatuslineCost float64`, `ContextPct int`, `Model string` (+ a `costSource` enum: `scrape` | `statusline`). `SetStatuslineData` writes them under `s.mu` (assignment only) and sets `costSource = statusline`.
- **Gate `ScanTokens()`** so it skips/does-not-overwrite cost when `costSource == statusline` (the single-authoritative-writer rule; `ScanTokens` runs unconditionally at `app_scan.go:106` today).
- **Event/bindings:** extend the `terminal:activity` event payload (`ActivityInfo`) with context %/model; this flows as an **event**, not a binding return, so **no `models.ts` class sync needed** — but verify nothing new crosses a binding return (CLAUDE.md silent-strip rule applies only there).
- **Tests / verification gates:**
  - **(blocker) End-to-end env inheritance:** a real PTY → `cmd.exe /c claude` → forwarder run asserting `MULTITERMINAL_SESSION_ID` + `MTUI_PORT` are readable by the shim. Not a unit test — the inferred inheritance must be proven once.
  - **(blocker) Forwarder non-blocking under cancellation:** the POST must detach/return fast enough that Claude killing the statusline mid-render does not drop the wrapped display nor hang.
  - Endpoint parses a real captured statusline JSON → correct `Session` fields; missing/garbage payload → no crash, no overwrite.
  - **`done`-edge regression guard:** replay activity transitions; assert `processQueue` fires exactly once per turn, unaffected by cost updates.
  - `ScanTokens` does not overwrite a `statusline`-sourced cost.
  - Wrapped statusline: user's existing command output still renders unchanged.
- **Assumption to state:** all MTUI-launched claude panes run the interactive TUI (verified: `buildClaudeArgv` never emits `--print`/`-p`), so the statusline fires. A future headless pane would silently lose capture → screen-scrape.
- **Deliverable:** accurate official cost (incl. subagents) + context % + model per pane, attributed exactly — for any pane with a statusline (MTUI's or the user's).

**Sub-plan 2 — JSONL enrichment: "working" + convergence.**
- Trimmed `claudelog` package: `parse.go` (tail framing, turn-state on user/assistant lines, pending-tool by forward id-match, convergence) + `types.go`. **No `encode.go`/`resolve.go`/`pricing.go`.**
- File path from `transcript_path` or `HookSessionID()` (D6).
- Tail read off the scan-loop goroutine (per-session goroutine; non-blocking `Latest()`); display-only `working`/`convergence` fused with `LastOutputAt`, priority hooks > JSONL > scrape, **never feeding the authoritative activity state** (D5).
- Extend `ActivityInfo` with `working bool`, `convergence bool`; transition-table tests asserting these never trigger `processQueue`.
- **Deliverable:** live "generating" badge + stuck-loop warning.

**Sub-plan 3 — Subagent/todo detail + UI (follow-on).**
- Per-counter token breakdown incl. subagents (dedup-by-id across main + `subagents/*.jsonl`), subagent roster, todo list, and their visualization. Captures the deferred D8 work when there's a consumer.

## Risks (residual)

- **Statusline coverage:** the wrap-existing forwarder (D7) covers MTUI's own statusline *and* a user's custom one; only a pane with no statusline at all loses capture → screen-scrape fallback. Cross-restart panes lose capture until relaunch (D3b).
- **Statusline cadence:** Claude invokes the command after each assistant message / `/compact` / mode change, debounced ~300 ms — fine for cumulative cost; live "working" comes from JSONL/PTY, not the statusline.
- **Headless panes:** statusline does not fire under `--print`/SDK mode; no MTUI claude pane uses that today, but a future one would silently fall back to screen-scrape.
- **JSONL schema drift (Sub-plan 2 only):** isolated tail parser, tolerant unmarshal, soft fallback; far smaller surface now that resolution/pricing are gone.
- **Localhost endpoint:** loopback only, same trust model as the existing tmux API; payload (cost/cwd) is local.

## Follow-ons (out of scope)

1. Subagent + todo visualization and per-counter token breakdown (Sub-plan 3).
2. `linesAdded`/`filesModified`/`gitCommits` (the statusline payload exposes some of these — `cost.total_lines_added/removed` — capture opportunistically later).
3. fsnotify watcher for the enrichment tail (replaces the per-session goroutine's polling).
4. Multi-provider (Codex/Gemini) — they have their own statusline/telemetry stories.

## Build order

**Sub-plan 1 (value lands here) → Sub-plan 2 → Sub-plan 3.** Every step keeps the branch green: any pane without a captured statusline push or resolvable JSONL keeps today's screen-scrape behavior by construction (zero-regression goal).
