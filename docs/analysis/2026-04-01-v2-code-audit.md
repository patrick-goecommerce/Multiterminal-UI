# v2 Kanban Orchestration Code Audit

**Date:** 2026-04-01
**Branch audited:** `feat/kanban-orchestration-v2`
**Purpose:** Decide per file whether to refactor, rewrite (neu), or use as reference only (referenz) for v3.

## Decision Summary

| File | Lines | Tests | Decision | Rationale |
|------|-------|-------|----------|-----------|
| `app_headless.go` | ~80 | Yes (5) | **refactor** | Clean, isolated, works standalone. Needs minor adaptation (extract from AppService, support Unix). |
| `app_orchestrate_parallel.go` | ~170 | Yes (4) | **referenz** | Wave grouping algorithm is solid, but execution is tightly coupled to AppService/worktrees. v3 Engine will own execution. |
| `app_orchestrate_plan.go` | ~200 | Yes (10) | **referenz** | Prompt templates and JSON parsers are useful reference. But v3 moves to structured Request/Response contract -- prompts will be rebuilt. |
| `app_orchestrate_exec.go` | ~210 | Yes (6) | **neu** | Core execution loop is the most changed area in v3. State machine replaces ad-hoc status updates. Too coupled to reuse. |
| `app_orchestrate_qa.go` | ~100 | Yes (6) | **referenz** | QA/drift prompts and parsers are good reference. v3 QA will be an Engine capability, not an orchestrator concern. |
| `app_orchestrate_memory.go` | ~80 | Yes (3) | **referenz** | File-based memory is replaced by git-ref storage in v3. The learning extraction pattern is useful reference. |
| `app_kanban.go` | ~250 | No | **neu** | Board state moves from JSON file to git-refs. Load/save/migrate all change fundamentally. CRUD methods are trivial to rewrite. |
| `app_kanban_types.go` | ~30 | No | **refactor** | `CardPlan`, `CardPlanStep`, `QuizQuestion` are clean value types with proper tags. Move to `internal/board/` as-is. |
| `app_worktree.go` | ~280 | No (implicit) | **refactor** | Solid worktree management. Categorization, naming, Windows path handling are all production-tested. Adapt for v3 Engine use. |

## Detailed Assessment

### app_headless.go -- REFACTOR

**What it does:** Wraps `claude -p` CLI calls via COMSPEC on Windows. Pipes prompt via stdin, captures JSON stdout.

**Quality:** Good. Clean separation of concerns. `buildHeadlessArgs` and `stripClaudeEnv` are properly extracted helpers.

**Tests:** 5 unit tests covering arg building, env stripping, JSON parsing, context cancellation. All test pure functions (no I/O mocking needed).

**v3 reuse:**
- `RunHeadless` is the foundation of all AI calls -- must survive.
- Needs extraction from `AppService` method to standalone function or `Engine` method.
- Consider adding: model selection, streaming support, retry with backoff.
- `stripClaudeEnv` duplicates logic from `session.go` (noted in comment) -- consolidate in v3.

**Action items:**
1. Move to `internal/engine/` as `RunCLI` or similar.
2. Add Unix support (no COMSPEC needed).
3. Add model parameter.
4. Keep test coverage.

---

### app_orchestrate_parallel.go -- REFERENZ

**What it does:** Groups plan steps into dependency-ordered "waves" for parallel execution. Each parallel step runs in its own git worktree. Merges results back sequentially.

**Quality:** Mixed. `groupStepsIntoWaves` is a clean, well-tested algorithm. But `executeWaveParallel` is tightly coupled:
- Direct `AppService` method (needs `a.RunHeadless`, `a.updateSubCardStatus`, `a.emitStepUpdate`).
- Creates worktrees inline with hardcoded naming convention.
- `mergeWorktreeChanges` uses raw `exec.Command` with hardcoded branch prefix `terminal/`.
- `removeNamedWorktree` and `execCmd` are utility functions that belong elsewhere.

**Tests:** 4 tests for `groupStepsIntoWaves` (including edge cases: empty, all-parallel, all-sequential). No tests for `executeWaveParallel` (would need heavy mocking).

**v3 reuse:**
- `groupStepsIntoWaves` algorithm: port to `internal/orchestrator/` as pure function.
- `executeWaveParallel` pattern: reference only. v3 Engine handles execution, Orchestrator handles scheduling.
- `mergeWorktreeChanges`: reference for git merge strategy.

---

### app_orchestrate_plan.go -- REFERENZ

**What it does:** Generates implementation plans via headless Claude calls. Three-phase flow: complexity assessment, quiz generation, plan generation. Each phase has a prompt builder and JSON response parser.

**Quality:** Good prompt engineering. Parsers are defensive (validate enum values, check for empty results). But:
- Retry logic is naive (just appends "IMPORTANT: Respond ONLY with valid JSON").
- `loadMemoryContext` reads a hardcoded path (`docs/mtui/README.md`).
- `loadCompletedCardSummaries` is imported from memory module -- cross-cutting concern.
- Pattern reuse injection (past card summaries) is bolted on to `GenerateCardPlan`.

**Tests:** 10 tests covering all three parsers plus prompt builders. Good edge cases (malformed JSON, missing fields, invalid enums). Pure function tests only.

**v3 reuse:**
- Prompt templates: reference for writing v3 prompts, but v3 uses structured Request/Response.
- JSON parsers: the `parse*Response` functions are good patterns but will be replaced by typed Engine responses.
- Three-phase flow (complexity -> quiz -> plan): good reference for v3 Orchestrator state machine transitions.

---

### app_orchestrate_exec.go -- NEU (rewrite)

**What it does:** The main execution loop. Creates sub-cards from plan steps, runs them sequentially or in parallel waves, integrates QA review after each step, checks for drift every 2 steps, persists learnings on completion.

**Quality:** Functional but problematic:
- `runPlanExecution` is a 90-line goroutine with nested control flow (wave loop > sequential/parallel branch > QA > drift check).
- State management is scattered: `loadKanbanState`/`saveCardUpdate` called repeatedly in helpers, creating race conditions under concurrent execution.
- `activeExecutions` map with `context.CancelFunc` is a simple but fragile cancellation mechanism.
- No retry on failed steps (just marks as failed and continues).
- `getDiff` called without specifying which files -- gets entire repo diff.
- Sub-cards are placed in `ColDefine` column regardless of parent card location.

**Tests:** 6 tests covering `buildSubCards`, `buildStepExecutionPrompt`, `parseStepResult`, `joinStrs`. All pure functions. No tests for the actual execution loop.

**v3 reuse:**
- `StepResult` struct: useful, port to `internal/engine/`.
- `buildStepExecutionPrompt`: reference for prompt structure.
- Execution loop: rewrite completely. v3 uses a state machine with formal transitions, not an imperative goroutine.
- Sub-card creation pattern: v3 board model handles this differently (git-refs, not JSON array manipulation).

---

### app_orchestrate_qa.go -- REFERENZ

**What it does:** QA review of step output (diff-based) and drift detection (comparing completed work against plan).

**Quality:** Clean and focused. Two independent concerns:
- Step QA: builds prompt with diff, parses approved/issues response.
- Drift detection: compares summaries against plan JSON, parses on_track/drift/recommendation.
- `getDiff` is too simple (no file filtering, no staged vs unstaged).
- Diff truncation at 8000 chars is arbitrary but pragmatic.

**Tests:** 6 tests for both QA and drift parsers. Good coverage of happy path and error cases.

**v3 reuse:**
- QA prompt pattern: good reference for v3 Engine QA capability.
- Drift detection concept: v3 Orchestrator should have similar anti-drift mechanism.
- `getDiff` utility: rewrite with better filtering.

---

### app_orchestrate_memory.go -- REFERENZ

**What it does:** Persists card execution learnings to markdown files in `docs/mtui/board/`. Optionally runs a headless Claude call to organize the memory directory.

**Quality:** Simple and effective:
- `writeCardMemory` generates structured markdown with decisions, plan summary, and learnings.
- `loadCompletedCardSummaries` reads them back for pattern reuse injection.
- The optional headless "organize" call is a nice touch but has no error handling beyond logging.
- German section headers ("Entscheidungen", "Erkenntnisse") match UI text convention.

**Tests:** 3 tests using `t.TempDir()` for filesystem operations. Clean and isolated.

**v3 reuse:**
- File-based memory: replaced by git-ref storage in v3.
- Learning extraction pattern: reference for what to persist.
- `loadCompletedCardSummaries` pattern: reference for how to feed past learnings into prompts.

---

### app_kanban.go -- NEU (rewrite)

**What it does:** Full board state management. CRUD operations (add, move, remove cards), GitHub issue sync, JSON persistence to `.mtui/kanban.json`, column migration.

**Quality:** Functional but v3-incompatible:
- State is a single JSON file with a `map[string][]KanbanCard` -- no concurrent access safety.
- `KanbanCard` struct has 25+ fields, mixing v1 fields (IssueNumber, Labels) with v2 orchestration fields (CardPlan, SubCards, Complexity).
- `findCard` does O(n) scan across all columns -- fine for small boards but not ideal.
- `SyncKanbanWithIssues` calls `a.GetIssues` twice (open + closed) -- could be one call.
- `generateID` uses `time.Now().UnixNano()` -- not UUID, collision-prone under parallel creation.
- `migrateColumns` handles old->new column renames -- won't be needed in v3.
- `Plan` and `ScheduledTask` types referenced but defined in separate files (`app_kanban_plan.go`, `app_kanban_schedule.go`) not in audit scope.

**Tests:** None for this file.

**v3 reuse:**
- Column constants (`ColDefine`, etc.): may keep similar column model.
- CRUD pattern: trivial, rewrite for git-ref backend.
- Issue sync: reference for how to bridge GitHub issues to board cards.

---

### app_kanban_types.go -- REFACTOR

**What it does:** Defines `CardPlan`, `CardPlanStep`, and `QuizQuestion` value types.

**Quality:** Excellent. Clean, minimal, proper `json`+`yaml` tags. No logic, no dependencies.

**Tests:** Indirectly tested via plan/exec tests.

**v3 reuse:**
- Move directly to `internal/board/types.go` or `internal/orchestrator/types.go`.
- May add fields (e.g., `EstimatedMinutes`, `Priority` on steps) but base structure is right.

---

### app_worktree.go -- REFACTOR

**What it does:** Git worktree lifecycle management. Creates issue-specific and named worktrees, lists/categorizes them, handles Windows path normalization.

**Quality:** Production-grade:
- `CreateWorktree` and `CreateNamedWorktree` handle existing worktrees gracefully.
- `parseWorktreePorcelain` correctly parses git's `--porcelain` output.
- `categorizeWorktree` uses case-insensitive comparison for Windows.
- `sanitizeWorktreeName` is thorough (handles spaces, slashes, consecutive hyphens).
- `hideConsole(cmd)` referenced but defined elsewhere -- Windows-specific console hide.
- `issueBranchName` and `branchExists` referenced but defined in other files.

**Tests:** No dedicated test file, but `parseWorktreePorcelain`, `categorizeWorktree`, and `sanitizeWorktreeName` are pure functions that should have tests.

**v3 reuse:**
- Move to `internal/engine/worktree.go` or keep in `internal/backend/`.
- `CreateNamedWorktree` is used by parallel execution -- keep this contract.
- Add tests for pure functions during migration.

---

## Cross-Cutting Observations

### State Management (Critical for v3)
v2 uses `loadKanbanState`/`saveKanbanState` (read-modify-write on a single JSON file) with no locking. Multiple concurrent executions will corrupt state. v3's git-ref approach solves this fundamentally.

### Coupling to AppService
Every orchestration function is an `AppService` method, accessing `a.serviceCtx`, `a.app.Event.Emit`, `a.RunHeadless`, `a.mu`, `a.activeExecutions`. v3 must separate:
- **Board** (state, types) -- `internal/board/`
- **Orchestrator** (scheduling, state machine) -- `internal/orchestrator/`
- **Engine** (execution, CLI calls, worktrees) -- `internal/engine/`

### Prompt Engineering
v2 prompts are inline string concatenation with `fmt.Sprintf`. They work but are:
- Hard to test in isolation.
- Hard to version or A/B test.
- No token counting or context window management.
v3 should consider prompt templates as first-class artifacts.

### Error Handling
Most errors are logged and swallowed (especially in `runPlanExecution` goroutine). v3 state machine should make errors explicit transitions.

### Test Quality
- Pure function tests are good (parsers, prompt builders, wave grouping).
- No integration tests for actual execution flow.
- No tests for board CRUD operations.
- No tests for worktree management (pure functions are untested).

## Migration Priority

1. **First:** `app_kanban_types.go` (refactor) + `app_worktree.go` (refactor) -- foundational, no AI dependency.
2. **Second:** `app_headless.go` (refactor) -- Engine foundation.
3. **Third:** Build v3 state machine and board (neu) using `app_kanban.go` and `app_orchestrate_exec.go` as reference.
4. **Fourth:** Port prompt patterns from `app_orchestrate_plan.go`, `app_orchestrate_qa.go`, `app_orchestrate_memory.go` into v3 Engine capabilities.
5. **Last:** Port `groupStepsIntoWaves` from `app_orchestrate_parallel.go` into Orchestrator.
