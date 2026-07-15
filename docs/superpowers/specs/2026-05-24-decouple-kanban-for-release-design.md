# Decouple Kanban / Agent Orchestration for Release — Design Specification

> Status: APPROVED | Date: 2026-05-24
> Release base: `alpha-main` → new branch `release/no-kanban`
> Parking: v2 preserved in `alpha-main` history + `feat/kanban-orchestration-v2`; v3 on `feat/kanban-orchestration-v3`

## 1. Goal

Make Multiterminal release-ready as a focused terminal multiplexer by **decoupling** the Kanban board and its agentic orchestration / scheduler from the release build. The code is **not deleted** — it is parked on existing feature branches and remains in git history for a later return.

The Kanban/agentic feature ("der agentische Kram") currently does not work reliably and should not ship in a release.

## 2. Approach

**Mechanism:** Parking branch. Cut `release/no-kanban` from `alpha-main` and remove the Kanban + scheduler code there in one logical change. The v2 implementation stays recoverable via the `alpha-main` history and `feat/kanban-orchestration-v2`; the v3 rewrite stays on `feat/kanban-orchestration-v3`. Re-integration later is a cherry-pick/merge from the relevant feature branch.

**Scope of removal:** the entire Kanban board (board/cards/columns), the v2 orchestrator, and the scheduler (auto-spawned scheduled tasks) — the scheduler is removed because, without the Kanban UI, it has no way to be managed.

**Out of scope:** the v3 packages (`internal/board`, `internal/orchestrator`, `internal/engine`) do not exist on `alpha-main`, so nothing v3-related needs removing on the release branch.

## 3. What Is Removed

### Backend — delete files
- `internal/backend/app_kanban.go`
- `internal/backend/app_kanban_plan.go`, `app_kanban_plan_test.go`
- `internal/backend/app_kanban_schedule.go`, `app_kanban_schedule_test.go`
- `internal/backend/app_kanban_test.go`
- `internal/backend/app_orchestrator.go`
- `internal/backend/app_orchestrator_review.go`
- `internal/backend/app_orchestrator_schedule.go`
- `internal/backend/app_schedule_runner.go`, `app_schedule_runner_test.go`

### Backend — edit (entanglements)
- `internal/backend/app.go`: remove the `go a.scheduleLoop(scanCtx)` startup wiring; remove any Kanban-only fields/initialization from the `AppService` struct / `ServiceStartup` if present.
- `internal/backend/app_scan.go`: remove the `notifyOrchestratorDone(id)` call (the orchestrator it notifies is being deleted).
- **Relocate `generateID()`**: it is defined in `app_kanban.go` but used by `app_chat.go` and `app_chat_stream.go`. Move `generateID()` into a new neutral file `internal/backend/app_ids.go` so chat keeps working after the Kanban files are deleted.

### Frontend — delete files
- `frontend/src/components/KanbanBoard.svelte`
- `frontend/src/components/KanbanCard.svelte`
- `frontend/src/components/KanbanColumn.svelte`
- `frontend/src/stores/kanban.ts`

### Frontend — edit
- `frontend/src/components/LeftNav.svelte`: remove the `{ id: 'kanban', ... }` nav entry and the `.icon-kanban` CSS rule.
- `frontend/src/stores/workspace.ts`: remove `'kanban'` from the `NavItem` union type.
- `frontend/src/App.svelte`: remove the `kanban` store import, the `activeView === 'kanban'` view branch + `<KanbanBoard>` usage, and the orchestrator / `kanban:schedules_updated` event subscriptions (the `GetKanbanState`/`kanban.setState` blocks).
- **Wails bindings** (`frontend/wailsjs/go/backend/App.d.ts`, `App.js`, and `frontend/wailsjs/go/models.ts`): manually remove the generated Kanban/orchestrator/schedule method signatures and model classes. Wails v3 does not regenerate these automatically.

## 4. What Is Kept

The full terminal-multiplexer core: Terminals view, **Dashboard** (cross-project overview — verified independent of Kanban: it reads git branch / session / cost data only), Sidebar (Explorer, Source Control, Issues), Git helpers, Chat, notifications, health, window management. After removal, `LeftNav` shows **Dashboard** and **Terminals**.

## 5. Verification

The change is complete only when all of the following pass on `release/no-kanban`:

- `go build -tags desktop ./...` — succeeds.
- `go vet ./...` — clean.
- `go test ./internal/...` — all packages green.
- `cd frontend && npm run build` — succeeds (no missing imports, no unresolved bindings).
- `git grep -in "kanban\|scheduleLoop\|notifyOrchestratorDone"` — no matches in source (comments/docs/history excluded).
- A dev build (`go build -tags desktop -o build/bin/multiterminal.exe .`) launches; `LeftNav` shows only Dashboard + Terminals; no dead nav entry; chat still functions (exercises the relocated `generateID`).

## 6. Non-Goals

1. **Deleting Kanban from history** — it must remain recoverable on the feature branches.
2. **Touching the v3 branch** — `feat/kanban-orchestration-v3` (this session's work) is left as-is.
3. **Reworking the v2 Kanban** — no fixes; it is simply removed from the release branch.
4. **Merging `release/no-kanban` anywhere** — branching/merge/tag decisions for the actual release are a separate step decided after this change builds clean.

## 7. Re-Integration Path (future)

When the agentic feature returns, branch from the release line and cherry-pick or merge the Kanban work from `feat/kanban-orchestration-v3` (preferred — it has the stabilized orchestration from 2026-05) or `feat/kanban-orchestration-v2`. The removal commit on `release/no-kanban` is a clean revert point if the parking ever needs to be undone wholesale.
