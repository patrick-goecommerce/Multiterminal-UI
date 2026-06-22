# Decouple Kanban / Agent Orchestration for Release — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the Kanban board, v2 orchestrator and scheduler from a release branch so Multiterminal ships as a focused terminal multiplexer; the code stays parked on `feat/kanban-orchestration-v2` / `-v3` and in history.

**Architecture:** Work happens in the already-created git worktree `.worktrees/release-no-kanban` on branch `release/no-kanban` (cut from `alpha-main`). Delete the Kanban Go/Svelte files, surgically remove three entanglements (relocate `generateID`, drop the `notifyOrchestratorDone` call in `app_scan.go`, strip Wails bindings), and edit the nav wiring. Removal is verified by build + vet + test + frontend build + grep gates rather than new unit tests.

**Tech Stack:** Go 1.21+ (`-tags desktop`), Svelte 4 + Vite + TypeScript, Wails v3.

**Spec:** `docs/superpowers/specs/2026-05-24-decouple-kanban-for-release-design.md`

**Working directory for ALL tasks:** `D:\repos\Multiterminal\.worktrees\release-no-kanban` (paths below are relative to it). Run Go commands with `PATH` including `/c/Program Files/Go/bin`.

---

## Task 1: Backend — relocate generateID, remove Kanban/orchestrator/scheduler Go code

**Files:**
- Create: `internal/backend/app_ids.go`
- Create: `internal/backend/app_ids_test.go`
- Delete: `internal/backend/app_kanban.go`, `app_kanban_plan.go`, `app_kanban_plan_test.go`, `app_kanban_schedule.go`, `app_kanban_schedule_test.go`, `app_kanban_test.go`, `app_orchestrator.go`, `app_orchestrator_review.go`, `app_orchestrator_schedule.go`, `app_schedule_runner.go`, `app_schedule_runner_test.go`
- Modify: `internal/backend/app.go`, `internal/backend/app_scan.go`

- [ ] **Step 1: Create the relocated `generateID` in a neutral file**

`generateID()` currently lives in `app_kanban.go` but is also called by `app_chat.go` and `app_chat_stream.go`. Create `internal/backend/app_ids.go` with a uniqueness-safe version (the old `time.Now().UnixNano()` collides under Windows' coarse clock):

```go
package backend

import (
	"fmt"
	"sync/atomic"
	"time"
)

// lastGeneratedID guarantees strictly increasing IDs even when the system
// clock has coarse resolution (e.g. Windows), where time.Now().UnixNano() can
// return the same value for calls in a tight loop.
var lastGeneratedID atomic.Int64

// generateID creates a unique, strictly increasing ID seeded from the clock.
func generateID() string {
	for {
		prev := lastGeneratedID.Load()
		next := time.Now().UnixNano()
		if next <= prev {
			next = prev + 1
		}
		if lastGeneratedID.CompareAndSwap(prev, next) {
			return fmt.Sprintf("%d", next)
		}
	}
}
```

- [ ] **Step 2: Add the uniqueness test (preserves coverage lost with app_kanban_test.go)**

Create `internal/backend/app_ids_test.go`:

```go
package backend

import "testing"

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateID()
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}
```

- [ ] **Step 3: Delete the Kanban/orchestrator/scheduler Go files**

```bash
git rm internal/backend/app_kanban.go internal/backend/app_kanban_plan.go \
  internal/backend/app_kanban_plan_test.go internal/backend/app_kanban_schedule.go \
  internal/backend/app_kanban_schedule_test.go internal/backend/app_kanban_test.go \
  internal/backend/app_orchestrator.go internal/backend/app_orchestrator_review.go \
  internal/backend/app_orchestrator_schedule.go internal/backend/app_schedule_runner.go \
  internal/backend/app_schedule_runner_test.go
```

- [ ] **Step 4: Remove the scheduler wiring in `app.go`**

In `internal/backend/app.go`, delete the scheduler goroutine line inside `ServiceStartup` (it sits among the loop launches):

Remove:
```go
	go a.scheduleLoop(scanCtx)
```
Leave `go a.scanLoop(scanCtx)` and `go a.batchLoop(scanCtx)` intact.

- [ ] **Step 5: Remove the orchestrator notification in `app_scan.go`**

In `internal/backend/app_scan.go`, the fresh-"done" transition block calls the now-deleted orchestrator. Change:

```go
		// Trigger pipeline queue on fresh "done" transition
		if activityChanged && actStr == "done" && a.app != nil {
			a.processQueue(id)
			// Notify orchestrator that this agent finished
			a.notifyOrchestratorDone(id)
		}
```
to:
```go
		// Trigger pipeline queue on fresh "done" transition
		if activityChanged && actStr == "done" && a.app != nil {
			a.processQueue(id)
		}
```

- [ ] **Step 6: Verify the backend compiles, vets, and tests pass**

Run (from the worktree):
```bash
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
go build -tags desktop ./...
go vet ./...
go test ./internal/...
```
Expected: build exit 0, vet clean, all packages `ok`. If the compiler reports a remaining reference to a deleted symbol (e.g. `KanbanState`, `ScheduledTask`, `notifyOrchestratorDone`, `scheduleLoop`), remove that reference in the offending file and re-run — there should be none beyond app.go/app_scan.go.

- [ ] **Step 7: Commit**

```bash
git add internal/backend/
git commit -m "refactor(backend): remove Kanban/orchestrator/scheduler; relocate generateID"
```

---

## Task 2: Frontend — remove Kanban UI and nav wiring

**Files:**
- Delete: `frontend/src/components/KanbanBoard.svelte`, `KanbanCard.svelte`, `KanbanColumn.svelte`, `frontend/src/stores/kanban.ts`
- Modify: `frontend/src/components/LeftNav.svelte`, `frontend/src/stores/workspace.ts`, `frontend/src/App.svelte`

- [ ] **Step 1: Delete the Kanban components and store**

```bash
git rm frontend/src/components/KanbanBoard.svelte \
  frontend/src/components/KanbanCard.svelte \
  frontend/src/components/KanbanColumn.svelte \
  frontend/src/stores/kanban.ts
```

- [ ] **Step 2: Remove the `kanban` nav entry and its icon in `LeftNav.svelte`**

Remove this line from the `mainViews` array:
```js
    { id: 'kanban', label: 'Kanban', icon: 'kanban' },
```
And remove the icon CSS rule:
```css
  .icon-kanban::before { content: '\2630'; }
```

- [ ] **Step 3: Remove `'kanban'` from the `NavItem` type in `workspace.ts`**

Change:
```ts
export type NavItem =
  | 'terminals'      // Default: Terminal panes
  | 'dashboard'      // Cross-project overview
  | 'kanban';        // Kanban board with planning + automation
```
to:
```ts
export type NavItem =
  | 'terminals'      // Default: Terminal panes
  | 'dashboard';     // Cross-project overview
```

- [ ] **Step 4: Remove the Kanban imports in `App.svelte`**

Delete both import lines:
```js
  import KanbanBoard from './components/KanbanBoard.svelte';
```
```js
  import { kanban } from './stores/kanban';
```

- [ ] **Step 5: Remove the orchestrator/schedule event subscriptions in `App.svelte`**

Delete the entire `orchestrator:update` subscription block:
```js
    // Orchestrator events: reload kanban board when plan steps change
    EventsOn('orchestrator:update', (event: any) => {
      const data = event.data || event;
      const eventType = data.type || '';
      console.log('[orchestrator]', eventType, 'plan:', data.planId || data.plan_id);
      // Reload kanban state when orchestrator changes plan/step status
      const dir = get(activeTab)?.dir || '';
      if (dir) {
        import('../wailsjs/go/backend/App').then(({ GetKanbanState }) => {
          GetKanbanState(dir).then(state => {
            kanban.setState(state);
          }).catch(() => {});
        });
      }
    });
```
And delete the entire `kanban:schedules_updated` subscription block that follows it (starts at `// Schedule runner events: reload kanban schedules when a task runs` and ends at its closing `});`):
```js
    // Schedule runner events: reload kanban schedules when a task runs
    EventsOn('kanban:schedules_updated', (event: any) => {
      const data = event.data || event;
      const dir = data.dir || get(activeTab)?.dir || '';
      if (dir) {
        import('../wailsjs/go/backend/App').then(({ GetKanbanState }) => {
          GetKanbanState(dir).then(state => {
            kanban.setState(state);
          }).catch(() => {});
        });
      }
    });
```

- [ ] **Step 6: Remove the `kanban` view branch in `App.svelte` markup**

Delete:
```svelte
    {:else if $workspace.activeView === 'kanban'}
      <KanbanBoard dir={$activeTab?.dir ?? ''} />
```
Keep the surrounding `{:else if ...}` chain and the final `{/if}` intact.

- [ ] **Step 7: Verify the frontend builds**

```bash
cd frontend && npm run build && cd ..
```
Expected: `✓ built`. If the build reports an unresolved import or unknown `GetKanbanState`/`kanban`, find and remove the leftover reference, then re-run. (The chunk-size warning is pre-existing and fine.)

- [ ] **Step 8: Commit**

```bash
git add frontend/src/
git commit -m "feat(frontend): remove Kanban board view and nav wiring"
```

---

## Task 3: Strip Kanban/orchestrator/schedule Wails bindings

**Files:**
- Modify: `frontend/wailsjs/go/backend/App.d.ts`, `frontend/wailsjs/go/backend/App.js`, `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Remove the binding functions**

In `frontend/wailsjs/go/backend/App.d.ts` and `App.js`, remove the exported functions for the deleted backend methods (and the section comment headers like `// --- Sprint 2: Dashboard & Kanban ---`, `// --- Kanban Orchestration ---`):

```
GetKanbanState, SaveKanbanState, MoveKanbanCard, AddKanbanCard, RemoveKanbanCard,
SyncKanbanWithIssues, UpdateKanbanCard,
CreateSchedule, GetSchedules, UpdateSchedule, DeleteSchedule, ToggleSchedule,
StartOrchestration, StopOrchestration, GetOrchestrationStatus
```
Keep all other functions (Dashboard, terminal, git, issues, chat, ask-user, etc.).

- [ ] **Step 2: Remove the now-unused model classes**

In `frontend/wailsjs/go/models.ts`, remove the classes that only the Kanban/orchestrator/schedule methods used: `KanbanState`, `KanbanCard`, `ScheduledTask`, `OrchestrationStatus`, and any nested types referenced only by them (e.g. plan/step types). Do NOT remove `DashboardStats`/`DashboardPane` or any class still referenced by a kept binding.

- [ ] **Step 3: Verify no kept code references removed bindings**

```bash
cd frontend && npm run build && cd ..
git grep -nE "GetKanbanState|MoveKanbanCard|StartOrchestration|GetOrchestrationStatus|ScheduledTask|KanbanState" frontend/src
```
Expected: `npm run build` succeeds; the `git grep` over `frontend/src` returns nothing. If a kept feature (e.g. ask-user) still imports a binding you removed, restore just that binding.

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs/
git commit -m "chore(bindings): drop Kanban/orchestrator/schedule Wails bindings"
```

---

## Task 4: Final integration verification + dev build

**Files:** none (verification only)

- [ ] **Step 1: Full green gate**

```bash
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
go build -tags desktop ./...
go vet ./...
go test ./internal/...
cd frontend && npm run build && cd ..
```
Expected: all succeed.

- [ ] **Step 2: Confirm no Kanban remnants in source**

```bash
git grep -in "kanban\|scheduleLoop\|notifyOrchestratorDone" -- ':!docs' ':!*.md'
```
Expected: no matches (documentation/specs excluded). Investigate and remove any straggler before proceeding.

- [ ] **Step 3: Build the dev binary and sanity-check the nav**

```bash
go build -tags desktop -o build/bin/multiterminal.exe .
```
Expected: exit 0. Launching it shows a `LeftNav` with only **Dashboard** and **Terminals** (no Kanban entry), and chat still creates IDs (exercises the relocated `generateID`). Note: launching the GUI is a manual check for the user.

- [ ] **Step 4: Final commit (if Step 3 produced tracked changes; otherwise skip)**

```bash
git status --short
# build/bin is typically gitignored; only commit if something tracked changed.
```

---

## Notes for the executor

- **This is removal work:** intermediate states inside a task may not compile (e.g. after Step 3 of Task 1 before Step 6). Only the task-boundary verify steps must pass.
- **Parking is automatic:** nothing is force-deleted from history. The v2 code remains on `alpha-main`'s history and `feat/kanban-orchestration-v2`; v3 on `feat/kanban-orchestration-v3`.
- **Do not merge `release/no-kanban` anywhere** — release/tag decisions are a separate step after this builds clean.
