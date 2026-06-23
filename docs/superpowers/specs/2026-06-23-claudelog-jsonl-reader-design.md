# claudelog — JSONL Session Reader (Foundation)

- **Date:** 2026-06-23
- **Status:** Rev 3 — decomposed into 3 sub-plans after red-team rounds 1 & 2 (all findings empirical against real logs)
- **Branch target:** `alpha-main` (feature branch off it)
- **One-line goal:** Replace fragile screen-scraping of Claude token/cost with a correct, structured reader of Claude Code's JSONL session logs; add a turn-state "working" display signal and a convergence ("stuck loop") warning — without ever perturbing the pipeline `active→done` edge.

> **Why Rev 3.** Round 1 refuted the naive token sum (per-block `usage` duplication → ~2.6× over-count) and the "id always known" assumption. Round 2 verified the dedup-by-id fix holds (measured 2.04–2.93×; zero id-less assistant lines; zero same-id usage conflicts; compaction-safe) and the `done`-edge decoupling holds (the queue reads `prevActivity`, never `GetTokens`), but found three remaining bugs and recommended decomposition. Rev 3 folds in all fixes, **cuts the over-engineered reader** (no worker pool, no incremental-delta summing), and splits the work into three independently-mergeable sub-plans. Findings tagged **[R1]/[R2]**.

---

## Shared model (applies to all sub-plans)

### Format facts verified against real logs

Verified against `C:\Users\PatrickHenniggoeComm\.claude\projects\D--repos-Multiterminal\` (64 jsonl, 6 branches) + siblings.

1. **Per-block `usage` duplication [R1].** One API response (one `message.id`, one `usage`) is written as multiple JSONL lines — one per content block (`thinking`/`text`/`tool_use`) — each repeating the identical `usage`. **Token totals MUST dedup by `message.id`** (count each id once). Verified: 0/3676 assistant lines lack `message.id`; 0 same-id usage conflicts across all 64 files; compaction does not reuse ids.
2. **~10 line types [R1].** Besides `assistant`/`user`/`system`: `mode`, `permission-mode`, `attachment`, `last-prompt`, `ai-title`, `file-history-snapshot`, `queue-operation`. Turn-state must consider only `user`/`assistant` lines.
3. **Partial trailing line is the steady state during generation [R1].** The live file's last bytes are an unterminated append on nearly every read — normal, not an error.
4. **Token field names (exact)** on `type:"assistant"` `message.usage`: `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`. Model at `message.model`.
5. **tool_use/tool_result:** `tool_use`(`id`) in assistant `content[]`; matching `tool_result`(`tool_use_id`) in the next `user` line's `content[]`.
6. **compact_boundary** (`type:"system", subtype:"compact_boundary"`). Per-message usage is per-turn → dedup-by-id sum is compaction-safe.
7. **`cwd` + `gitBranch` exist in every file but never on the last line [R2].** In 53/64 files the last complete line is metadata (`permission-mode`/`ai-title`/…) carrying neither. Both fields appear ≤16 lines from EOF in every file (0/64 lack them entirely). The resolver must scan **backward** for the last complete line that has both.
8. **Subagents:** `{sessionId}/subagents/*.jsonl` + sibling `*.meta.json` (`agentType`, `description`, `toolUseId`). **No `Status` field** — must be derived (deferred; ship empty).
9. **TodoWrite:** shape unverified locally (no real TodoWrite call in these logs). Parse path written but `Todos` ships empty until a real fixture is captured.

### Path encoding [R1]

Claude encodes the abs path by replacing **`: \ / . _ ~ → -`** (the `~` is real: `C:\Users\PATRIC~1\…` → `C--Users-PATRIC-1-…`). Verified: `D:\repos\Multiterminal` → `D--repos-Multiterminal`; worktree `…\.worktrees\chat-pane-display-mode` → `…--worktrees-chat-pane-display-mode`. The encoding is **non-injective** (`.`,`_`,`-`,separators all → `-`): `a.b`, `a_b`, `a-b` collide. The resolver treats a >1-candidate match as ambiguous → no overwrite.

### Pinned decisions [R2]

| # | Decision | Value |
|---|---|---|
| D1 | `Confidence` type | `type Confidence int; const (ConfNone Confidence = iota; ConfLow; ConfHigh)`. **Zero value = `ConfNone`** (safe: never accidentally High). `Latest()` returns `SessionData` whose `Confidence==ConfNone` means "no usable data" (replaces a separate `ok`). |
| D2 | Refresh cadence | **Tail-state** (turn-state, last tools, pending tool) recomputed **every scan tick** from the last ≤64 KB only. **Full token recompute** (whole-file dedup-by-id) at most **every 5 s**, or immediately when the file **shrank** (compaction). No per-read delta summing — full recompute rebuilds the id→usage map from scratch each time. |
| D3 | Concurrency model | **One goroutine per Claude session** (pane count is capped by `max_panes_per_tab`). **No worker pool** (cut as premature). The scan loop only reads published last-known-good — non-blocking. |
| D4 | Reader lifecycle owner | A `readerRegistry` owned by `AppService` (its own `sync.Mutex`, not `App.mu`). A reader is created lazily on the first `Latest(session)` for a Claude pane and torn down from the existing `cleanupActivityTracking(id)` site (`app.go:287`). |
| D5 | "Is a Claude pane" predicate | The session's launch `mode` starts with `"claude"` **and** a non-empty `SessionID` was resolved. `codex`/`gemini` panes are out of scope for the foundation. |
| D6 | Subagent `Status` | Ship `""` (unknown) in the foundation; derivation deferred to the visualization follow-on. |
| D7 | Todos | Parse path implemented but `Todos` ships **empty** until a real TodoWrite fixture is captured. |
| D8 | `CLAUDE_CONFIG_DIR` | Read **once** from the MTUI process env at startup; one config dir process-wide. No per-session override (documented limitation). |

### Token totals — the only correct algorithm [R1/R2]

```
ids := map[string]Usage{}          // message.id -> usage (rebuilt every full recompute)
for each COMPLETE line (terminated by \n):
    if type=="assistant" && message.id != "":
        ids[message.id] = message.usage   // identical on repeats; last-write is fine
total := sum(ids)                   // each id counted once
```

**No incremental delta. [R2]** 83 % of ids span >1 physical line (up to 13), so a "sum appended bytes" delta double-counts an id split across refreshes. The byte offset is used **only** for cheap tail-state, never for the token total. The full recompute reads the whole file on D2's cadence.

### Line framing — the only correct read [R1]

Split the buffer on `\n`; **discard bytes after the last `\n`** (incomplete append). Never `Unmarshal` a partial line; a partial trailing line is **not** an error. Only a *complete* line that fails to parse is corruption → keep last-known-good, do not panic, do not log-spam.

---

## Sub-plan 1 — `claudelog` package (pure, stdlib only)

A leaf package `internal/terminal/claudelog`, no backend imports, fully unit-tested with fixtures. **Zero product behavior change.** Max 300 lines/file.

| File | Responsibility |
|---|---|
| `types.go` | `TokenUsage`, `PendingTool`, `SubAgent`, `Todo`, `SessionData`, `Confidence` (D1) |
| `encode.go` | `EncodePath` (`: \ / . _ ~ → -`, normalize: strip trailing sep), `ConfigDir()` (D8) |
| `resolve.go` | `ResolveSessionFile(configDir, cwd, sessionID, startedAt) (path string, conf Confidence)` |
| `parse.go` | line framing, dedup-by-id token total, turn-state (user/assistant only), pending tool, convergence, subagents (status=""), todos (empty per D7) |
| `pricing.go` | `MODEL_PRICING` (per-1M incl. cache tiers), `CostUSD(model, TokenUsage)`, unknown→0 |

`ResolveSessionFile` order:
1. projDir = `{configDir}/projects/{EncodePath(cwd)}`; cwd empty/`/` → `ConfNone`.
2. `sessionID != ""` and `{projDir}/{sessionID}.jsonl` exists → `ConfHigh`.
3. else list `{projDir}/*.jsonl`; for each, **scan backward for the last complete line carrying both `cwd` and `gitBranch`** (≤32 lines) and match against the session's dir/branch; among matches require mtime `> startedAt` (ns), newest wins → `ConfLow`; **0 or >1 matches → `ConfNone`** (ambiguous, never guess).

**Tests:** encoder (Windows paths, `PATRIC~1`→`PATRIC-1`, worktree, trailing sep, collisions→ambiguous); resolver (High by id; Low by cwd+gitBranch backward-scan; >1 match→None; missing dir); **token dedup with a multi-block same-`message.id` fixture** (the 2.6× regression guard) + compaction fixture; framing (partial last line ignored, valid prefix returned); turn-state (metadata-trailing must not flip `TurnOpen`; user-trailing open; pending-tool open; assistant-final closed; trailing-assistant-with-tool_use open); convergence; pricing (incl. cache tiers, unknown→0). `go test ./internal/terminal/claudelog/...` green.

**Deliverable:** a tested library. Mergeable immediately, no behavior change.

---

## Sub-plan 2 — Tokens/cost integration (the value increment)

Delivers exact, deduped tokens/cost for the **already-id-pinned happy path** (launch-dialog and restored panes already emit `--session-id <uuid>` — verified `App.svelte:427`, `claude.ts:87`, `session.ts:30`).

**Minimal prerequisite — no frontend churn, no `CreateSession` signature change [R2]:**
- Add `Session.SessionID string`, populated by **parsing `--session-id` out of the launch argv in `Start()`** (before the `cmd.exe /c` wrap). Handle both `--session-id <uuid>` and `--session-id=<uuid>`. MTUI emits the two-token form; manual/id-less launches simply leave it empty → those fall back to screen-scrape (handled in Sub-plan 3). This deliberately avoids touching the ~13 `CreateSession` call sites + 3 Wails binding files; canonical-field threading is deferred to Sub-plan 3 only if needed.

**Reader (simplest correct form, D3 deferred):** a **synchronous** `Latest(session) SessionData` that, on the scan tick, re-reads + caches last-known-good (full recompute gated by D2's 5 s / shrink rule; tail every tick). Off-loop goroutines are introduced in Sub-plan 3 — Sub-plan 2 ships the synchronous version to de-risk lock discipline first.

**Wiring in `scanAllSessions`:**
- `data := claudelog.Latest(session)` for Claude panes (D5).
- If `data.Confidence == ConfHigh` → set `s.Tokens` (incl. new `CacheCreationTokens`/`CacheReadTokens`) + cost. Else → keep today's `ScanTokens()`. **Never overwrite good values with `ConfLow`/`ConfNone`.**
- **Lock discipline [R1]:** read/parse entirely outside any session lock; acquire `s.mu` only for the final assignment; never hold `s.mu` across I/O.
- **`done` edge untouched [R1/R2]:** the pipeline edge (`app_scan.go:157`) stays driven by `DetectActivity`/`HasHookData`, debounced by the existing 1.5 s gate. Tokens are a separate write the queue never reads.

**Bindings:** extend `terminal.TokenInfo` with the two cache counters; **mirror into `frontend/wailsjs/go/models.ts`** (class field + constructor) — its own step (CLAUDE.md/memory recurring silent-strip bug).

**Tests:** the **scan-loop regression guard** — replay an append-by-append stream and assert `processQueue` fires **exactly once per turn** (never 0, never 2) with tokens changing underneath; **race test** (`-race`) running the reader concurrently with a `readLoop`-style writer to `s.LastOutputAt`, asserting no lock held across I/O; fallback (corrupt complete line / missing file → keep screen-scrape).

**Deliverable:** exact deduped tokens/cost for the happy path. Does **not** need `StartedAt`, the launch-site pins, or the working/convergence signal.

---

## Sub-plan 3 — Fallback coverage + working/convergence signal

Two independent slices; ship in either order.

**3a — Low-confidence fallback + peripheral panes:**
- Add `Session.StartedAt time.Time` (set in `Start()`); enables the `ConfLow` resolver's `mtime > startedAt` gate.
- Pin `--session-id` on the three id-less sites so they become `ConfHigh`: worktree pane (`App.svelte:392`), background agents (`background-agents.ts:56`), keepalive (`keepalive.ts:46`) — each a one-liner mirroring the existing pinned path.
- Upgrade the reader to **one goroutine per session** (D3) publishing last-known-good behind a small mutex; `Latest()` becomes a non-blocking read. Cache invalidation on `(size, mtime)`; on shrink → compaction → reset offset + full recompute; the ≤5 s full recompute also catches a same-size rewrite. On repeated read-deadline miss, downgrade `Confidence` so the caller reverts to live scrape.

**3b — `working` + `convergence` display signal:**
- Derive a display-only `working` state: JSONL `TurnOpen` (D2 tail) fused with `LastOutputAt < 1.5 s`. Priority for the *display* hint: hooks > JSONL(`ConfHigh`) > screen-scrape. **This never feeds the authoritative activity state or the `done` edge** — it is display-only.
- `convergence` flag (last 5 tools identical) likewise display-only.
- Extend the `ActivityInfo` event with `working bool`, `convergence bool` (+ parsed subagents/todos for later UI); **mirror into `models.ts`**. Frontend renders badges.
- A transition table (inputs: `HasHookData`, `Confidence`, `TurnOpen`, `LastOutputAt<1.5 s` → display state) with tests asserting these fields never trigger `processQueue`.

**Deliverable:** worktree/background/keepalive panes resolve exactly; live "working" + stuck-loop badges.

---

## Risks (residual)

- **Schema drift** — Claude's JSONL is undocumented/version-dependent → isolated parser, tolerant unmarshal, soft fallback, fixtures pinned to a known CLI version.
- **Low-confidence misattribution** — mitigated by backward cwd+gitBranch match + "ambiguous → don't guess"; residual only when two same-branch+cwd id-less sessions run concurrently → screen-scrape (acceptable).
- **Cost** — API-equivalent estimate, labeled in UI.
- **Todos** — unverified locally; ship empty until a fixture exists (D7).

## Follow-ons (out of scope)

1. Subagent + todo visualization (and `Status` derivation).
2. `linesAdded`/`filesModified`/`gitCommits`/`toolErrors`.
3. fsnotify watcher (replaces the per-session goroutine's polling without changing callers).
4. Multi-provider (Codex/Gemini) resolution.

---

## Build order

**Sub-plan 1 → Sub-plan 2 (value lands here) → Sub-plan 3a / 3b (either order).** Every step keeps the branch green: any unresolved/low-confidence/non-Claude pane keeps today's screen-scrape behavior by construction (Goal: zero regression).
