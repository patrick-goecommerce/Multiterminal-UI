# Kanban Orchestration v3 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** MTUI um eine vollstaendige AI-Agent-Orchestrierung erweitern — Git-Ref Board Storage, State Machine, Triage/Planning/Execution Pipeline mit operativen Guardrails.

**Architecture:** 3-Layer Backend (Board Layer → Orchestrator → Execution Engine) mit hartem Contract zwischen Orchestrator und Engine. Board-State in Git-Refs, volatile Daten im Filesystem, Prompts vom Orchestrator assembliert. Siehe `docs/superpowers/specs/2026-04-01-kanban-orchestration-v3-design.md`.

**Tech Stack:** Go 1.25, Svelte 4, Wails v3, xterm.js, `claude -p --output-format json`

**Branch:** Frisch von `alpha-main` — `feat/kanban-orchestration-v3`

**GitHub Milestone:** "Kanban Orchestration v3"
**Label-Schema:** `phase:0` bis `phase:3`, `type:infra`, `type:backend`, `type:frontend`, `type:test`

---

## File Structure

### New Packages/Files (Board Layer)

| File | Responsibility |
|------|---------------|
| `internal/board/refstore.go` | Low-level Git-Ref ops (hash-object, mktree, update-ref, read-ref) |
| `internal/board/refstore_test.go` | Unit tests for ref operations |
| `internal/board/board.go` | High-level CRUD (CreateTask, MoveTask, ListTasks, GetTask, DeleteTask) |
| `internal/board/board_test.go` | Unit tests for board operations |
| `internal/board/lock.go` | Atomic locking via lock-refs with timeout |
| `internal/board/lock_test.go` | Unit tests for locking |
| `internal/board/state.go` | Task State Machine (transitions, guards, events) |
| `internal/board/state_test.go` | Unit tests for every transition + guard |
| `internal/board/sync.go` | Git fetch/push of refs/mtui/* |
| `internal/board/types.go` | Shared types (TaskState, TaskCard, Transition, etc.) |

### New Packages/Files (Orchestrator)

| File | Responsibility |
|------|---------------|
| `internal/orchestrator/orchestrator.go` | Main orchestration loop (triage→plan→execute→qa) |
| `internal/orchestrator/orchestrator_test.go` | Integration tests with mocked engine |
| `internal/orchestrator/triage.go` | Complexity assessment via Haiku |
| `internal/orchestrator/triage_test.go` | Unit tests for complexity routing |
| `internal/orchestrator/planner.go` | Plan generation + JSON validation + LLM repair |
| `internal/orchestrator/planner_test.go` | Unit tests for plan validation/repair |
| `internal/orchestrator/wave.go` | Wave computation from step dependencies |
| `internal/orchestrator/wave_test.go` | Unit tests for dependency graphs |
| `internal/orchestrator/budget.go` | Budget tracking per card |
| `internal/orchestrator/budget_test.go` | Unit tests for budget guards |
| `internal/orchestrator/skills.go` | Skill registry, tech detection, policy merging |
| `internal/orchestrator/skills_test.go` | Unit tests for skill matching + conflict resolution |
| `internal/orchestrator/prompt.go` | Prompt assembly (system + skill + step context) |
| `internal/orchestrator/types.go` | ExecutionRequest, ExecutionResult, Plan types |

### New Packages/Files (Execution Engine)

| File | Responsibility |
|------|---------------|
| `internal/engine/engine.go` | ExecutionEngine interface + HeadlessEngine |
| `internal/engine/engine_test.go` | Unit tests with mocked claude -p |
| `internal/engine/slots.go` | WorktreeSlotManager (allocate/release/park/prune) |
| `internal/engine/slots_test.go` | Unit tests for slot lifecycle |
| `internal/engine/loop_step.go` | StepLoopDetector (in-memory, error normalization) |
| `internal/engine/loop_step_test.go` | Unit tests with simulated verify sequences |
| `internal/engine/loop_repo.go` | RepoLoopDetector (git log analysis) |
| `internal/engine/loop_repo_test.go` | Integration tests with prepared git histories |
| `internal/engine/checkpoint.go` | CheckpointGuard (progress-based) |
| `internal/engine/checkpoint_test.go` | Unit tests with simulated snapshots |
| `internal/engine/briefing.go` | DecisionBriefing, SecretScanner, ScopeCheck |
| `internal/engine/briefing_test.go` | Unit tests for risk scoring + secret detection |
| `internal/engine/merge.go` | Wave merge + conflict avoidance + dependency reconciliation |
| `internal/engine/merge_test.go` | Unit tests for merge decisions |

### Modified Files

| File | Changes |
|------|---------|
| `internal/backend/app.go` | Add board/orchestrator/engine fields to AppService, new Wails bindings |
| `internal/backend/app_headless.go` | Refactor into engine.HeadlessEngine (keep RunHeadless, adapt interface) |
| `internal/backend/app_worktree.go` | Refactor into engine.WorktreeSlotManager |
| `frontend/src/stores/kanban.ts` | Replace with Git-Ref-backed store, new types |
| `frontend/src/components/KanbanBoard.svelte` | Adapt to new state machine + events |
| `frontend/src/components/KanbanCard.svelte` | Show state machine status, scope warnings |
| `frontend/src/components/KanbanCardDetail.svelte` | Plan review, execution progress, guardrail status |

### New Frontend Files

| File | Responsibility |
|------|---------------|
| `frontend/src/components/SkillManager.svelte` | Skill detection display, activation toggles |
| `frontend/src/components/DecisionBriefing.svelte` | Pre-QA gate display (risks, secrets, recommendations) |
| `frontend/src/components/EscalationDialog.svelte` | Human review dialog with context package |

### Skill Files

| File | Responsibility |
|------|---------------|
| `.mtui/skills/go-backend.json` | Go skill manifest |
| `.mtui/skills/go-backend.md` | Go skill prompt bundle |
| `.mtui/skills/svelte-frontend.json` | Svelte skill manifest |
| `.mtui/skills/svelte-frontend.md` | Svelte skill prompt bundle |
| `.mtui/skills/typescript.json` | TypeScript skill manifest |
| `.mtui/skills/typescript.md` | TypeScript skill prompt bundle |
| `.mtui/skills/superpowers-core.json` | Universal skill manifest |
| `.mtui/skills/superpowers-core.md` | Universal skill prompt bundle |
| `.mtui/skills/security-basics.json` | Universal security skill manifest |
| `.mtui/skills/security-basics.md` | Universal security skill prompt bundle |

---

## Phase 0: Stabilisierung

> Label: `phase:0` | Ziel: Stabile Basis, frischer Branch, bestehenden Code verstehen

### Task 0.1: Frischen Branch erstellen

**Files:**
- No file changes, git operations only

- [ ] **Step 1: Branch von alpha-main erstellen**

```bash
git checkout alpha-main
git pull origin alpha-main
git checkout -b feat/kanban-orchestration-v3
git push -u origin feat/kanban-orchestration-v3
```

- [ ] **Step 2: Verifizieren dass Build funktioniert**

```bash
cd frontend && npm install && npm run build && cd ..
go build -tags desktop -o build/bin/mtui-test.exe .
```

Expected: Build erfolgreich, keine Fehler.

- [ ] **Step 3: Tests laufen lassen**

```bash
go test ./internal/terminal/... ./internal/config/... ./internal/backend/...
```

Expected: Alle bestehenden Tests passen.

- [ ] **Step 4: Commit**

```bash
git commit --allow-empty -m "chore: start kanban orchestration v3 from alpha-main"
```

---

### Task 0.2: Bestehenden Kanban-Code auditen und entscheiden

**Files:**
- Read: `internal/backend/app_kanban.go`, `app_kanban_types.go`, `app_headless.go`
- Read: `internal/backend/app_orchestrate_*.go`
- Read: `internal/backend/app_worktree.go`

- [ ] **Step 1: Audit-Checkliste durchgehen**

Fuer jede Datei pruefen:
1. Kompiliert der Code isoliert?
2. Gibt es Tests? Passen die Tests?
3. Welche Funktionen werden vom Frontend aufgerufen (Wails Bindings)?
4. Welcher Code ist wiederverwendbar fuer v3?

- [ ] **Step 2: Entscheidungs-Tabelle erstellen**

Dokument erstellen: `docs/analysis/2026-04-01-v2-code-audit.md`

Format:
```markdown
| Datei | Status | Entscheidung | Begruendung |
|-------|--------|-------------|-------------|
| app_headless.go | funktional | refactor in engine/ | Gute Basis, Interface anpassen |
| app_orchestrate_parallel.go | funktional | refactor wave logic | Wave-Grouping behalten |
| app_kanban.go | teilweise | neu schreiben | JSON-Storage -> Git-Refs |
| app_orchestrate_plan.go | ungetestet | referenz | Prompt-Patterns uebernehmen |
| ... | ... | ... | ... |
```

- [ ] **Step 3: Commit**

```bash
git add docs/analysis/2026-04-01-v2-code-audit.md
git commit -m "docs: audit existing v2 kanban/orchestration code for v3 rewrite"
```

---

### Task 0.3: .gitignore und Verzeichnisstruktur vorbereiten

**Files:**
- Modify: `.gitignore`
- Create: `internal/board/`, `internal/orchestrator/`, `internal/engine/`
- Create: `.mtui/skills/`

- [ ] **Step 1: .gitignore aktualisieren**

Hinzufuegen:
```
# MTUI volatile data (not board state — that's in git refs)
.mtui/qa/
.mtui/logs/
.mtui/worktrees/
```

- [ ] **Step 2: Package-Verzeichnisse erstellen**

```bash
mkdir -p internal/board internal/orchestrator internal/engine
mkdir -p .mtui/skills
```

- [ ] **Step 3: Placeholder-Dateien fuer Go-Packages**

Erstelle `internal/board/doc.go`:
```go
// Package board provides Git-ref-based kanban board storage,
// task state machine, and atomic locking.
package board
```

Erstelle `internal/orchestrator/doc.go`:
```go
// Package orchestrator provides the triage, planning, wave computation,
// budget tracking, and skill-based capability layer.
package orchestrator
```

Erstelle `internal/engine/doc.go`:
```go
// Package engine provides the execution engine interface, headless Claude
// execution, worktree slot management, and operational guardrails.
package engine
```

- [ ] **Step 4: Commit**

```bash
git add .gitignore internal/board/doc.go internal/orchestrator/doc.go internal/engine/doc.go .mtui/skills/
git commit -m "chore: scaffold v3 package structure and update .gitignore"
```

---

## Phase 1: Board Layer + State Machine

> Label: `phase:1` | Ziel: Git-Ref Storage funktional, State Machine getestet, Frontend zeigt Board

### Task 1.1: Git-Ref Store — Low-Level Operations

**Files:**
- Create: `internal/board/refstore.go`
- Create: `internal/board/refstore_test.go`

**Akzeptanzkriterien:**
- `WriteRef(repo, ref, content)` schreibt Inhalt als Blob in Git-Ref
- `ReadRef(repo, ref)` liest Inhalt aus Git-Ref
- `DeleteRef(repo, ref)` loescht Git-Ref
- `ListRefs(repo, prefix)` listet alle Refs mit Prefix
- `RefExists(repo, ref)` prueft ob Ref existiert
- Alle Ops nutzen `git hash-object -w --stdin`, `git update-ref`, `git show`
- Keine Shell-Injection moeglich (Ref-Namen validiert)
- Unit Tests fuer jede Operation + Fehlerfall (Ref existiert nicht, ungueliger Name)

**TDD-Reihenfolge:**
1. Test WriteRef + ReadRef Roundtrip
2. Implementiere WriteRef + ReadRef
3. Test DeleteRef
4. Implementiere DeleteRef
5. Test ListRefs
6. Implementiere ListRefs
7. Test RefExists
8. Implementiere RefExists
9. Test Ref-Name-Validation (Injection-Schutz)
10. Implementiere Validation

---

### Task 1.2: Board CRUD — High-Level Operations

**Files:**
- Create: `internal/board/types.go`
- Create: `internal/board/board.go`
- Create: `internal/board/board_test.go`

**Akzeptanzkriterien:**
- `TaskCard` struct mit allen Feldern aus Spec (ID, Title, Description, State, CardType, Complexity, Plan, etc.)
- Task-Serialisierung: Markdown mit YAML-Frontmatter (Karr-kompatibel)
- `CreateTask(repo, card)` → speichert in `refs/mtui/tasks/<id>/content`
- `GetTask(repo, id)` → liest und deserialisiert
- `ListTasks(repo)` → alle Tasks
- `MoveTask(repo, id, newState)` → State-Transition (via State Machine, Task 1.3)
- `DeleteTask(repo, id)` → entfernt Ref
- `SavePlan(repo, id, plan)` → speichert in `refs/mtui/tasks/<id>/plan`
- `GetPlan(repo, id)` → liest Plan-JSON

**TDD-Reihenfolge:**
1. Definiere types.go (TaskCard, CardType, Plan structs)
2. Test CreateTask + GetTask Roundtrip
3. Implementiere CreateTask + GetTask
4. Test ListTasks
5. Implementiere ListTasks
6. Test DeleteTask
7. Implementiere DeleteTask
8. Test SavePlan + GetPlan
9. Implementiere SavePlan + GetPlan
10. Test Plan-Groessen-Limit (max 50KB)

---

### Task 1.3: Task State Machine

**Files:**
- Create: `internal/board/state.go`
- Create: `internal/board/state_test.go`

**Akzeptanzkriterien:**
- Alle States aus Spec: backlog, triage, planning, review, executing, stuck, qa, merging, human_review, done
- Alle Transitions aus Spec mit Guards
- `Transition(card, event)` → (newState, error) — Fehler wenn Transition ungueltig
- Guards: Budget > 0 fuer executing, max 3x qa→executing, max 2x stuck→executing
- Jede Transition gibt ein `TransitionEvent` zurueck (fuer Wails Events)
- Metadata-Support: `execution_mode: "impl" | "qa_fix"`, `human_review_reason: string`

**TDD-Reihenfolge (ein Test pro Transition):**
1. Test backlog → triage (StartTriage)
2. Test triage → executing (trivial)
3. Test triage → planning (medium/complex)
4. Test planning → review (PlanReady)
5. Test review → executing (Approved)
6. Test executing → stuck (StepStuck)
7. Test stuck → executing (ModelEscalated)
8. Test stuck → executing (ReplanCompleted)
9. Test stuck → human_review (ScopeExpansionRequired)
10. Test stuck → human_review (MaxEscalations)
11. Test executing → qa (AllStepsDone)
12. Test qa → executing (QAFailed, zaehle 3x Guard)
13. Test qa → merging (QAPassed)
14. Test merging → done (MergeSuccess)
15. Test merging → human_review (MergeConflict)
16. Test human_review → executing/done/backlog (UserResolved)
17. Test ungueltige Transition → error
18. Implementiere State Machine

---

### Task 1.4: Atomic Locking

**Files:**
- Create: `internal/board/lock.go`
- Create: `internal/board/lock_test.go`

**Akzeptanzkriterien:**
- `AcquireLock(repo, taskID, agentName)` → schreibt `refs/mtui/tasks/<id>/lock`
- `ReleaseLock(repo, taskID, agentName)` → loescht Lock-Ref
- `IsLocked(repo, taskID)` → prueft ob Lock existiert
- Lock enthaelt: AgentName + Timestamp (JSON)
- Stale-Detection: Lock aelter als 5min → darf ueberschrieben werden
- Race-Condition-Schutz: zwei gleichzeitige AcquireLock → einer gewinnt
- Unit Tests fuer: acquire, release, stale detection, double-acquire

---

### Task 1.5: Board Sync (Git Fetch/Push)

**Files:**
- Create: `internal/board/sync.go`
- Create: `internal/board/sync_test.go`

**Akzeptanzkriterien:**
- `SyncPull(repo)` → `git fetch origin refs/mtui/*:refs/mtui/*`
- `SyncPush(repo)` → `git push origin refs/mtui/*`
- Fehlerbehandlung: Remote nicht erreichbar → Warnung, kein Crash
- Unit Tests mit lokalen bare-Repos

---

### Task 1.6: Wails Bindings fuer Board Layer

**Files:**
- Modify: `internal/backend/app.go` — Board-Felder zum AppService hinzufuegen
- Create: `internal/backend/app_board.go` — Wails-exposed Board methods
- Modify: `frontend/wailsjs/go/models.ts` — TypeScript Bindings

**Akzeptanzkriterien:**
- AppService hat `board *board.Board` Feld
- Exponierte Methods: `GetBoardTasks(dir)`, `CreateBoardTask(dir, card)`, `MoveBoardTask(dir, id, event)`, `GetBoardTask(dir, id)`, `DeleteBoardTask(dir, id)`, `SaveBoardPlan(dir, id, plan)`, `GetBoardPlan(dir, id)`, `SyncBoard(dir)`
- Events: `board:task-transition` mit CardID + OldState + NewState
- models.ts manuell synchronisiert (yaml+json Tags!)

---

### Task 1.7: Frontend Kanban Board — Git-Ref-Backend

**Files:**
- Modify: `frontend/src/stores/kanban.ts` — komplett neu, Git-Ref-backed
- Modify: `frontend/src/components/KanbanBoard.svelte` — State-Machine-aware
- Modify: `frontend/src/components/KanbanCard.svelte` — Neue States anzeigen
- Modify: `frontend/src/components/KanbanColumn.svelte` — Drag-Drop -> MoveTask Events

**Akzeptanzkriterien:**
- Board laedt Tasks via `GetBoardTasks(dir)` statt lokaler JSON-Datei
- Drag-and-Drop ruft `MoveBoardTask(dir, id, event)` auf
- State Machine States als Spalten: backlog | triage | planning | review | executing | qa | done
- stuck und human_review als besondere Markierungen (Badges)
- Echtzeit-Updates via `board:task-transition` Events
- Neuer Task erstellen via Dialog → `CreateBoardTask()`

---

## Phase 2: Orchestrator + Capability Layer

> Label: `phase:2` | Ziel: Triage, Plan-Generierung, Wave-Planung, Skills, Budget

### Task 2.1: Orchestrator Types + ExecutionEngine Interface

**Files:**
- Create: `internal/orchestrator/types.go`
- Create: `internal/engine/engine.go` (nur Interface)

**Akzeptanzkriterien:**
- `ExecutionRequest` struct exakt wie in Spec (StepID, CardID, WorktreeSlot, Prompt, SystemPrompt, Model, Verify, BudgetUSD, TimeoutSec, SkillPrompts)
- `ExecutionResult` struct exakt wie in Spec (StepID, Status, FilesChanged, FilesCreated, Verify, LoopSignals, CostUSD, DurationSec, Error)
- `VerifyStep`, `VerifyResult`, `LoopSignal`, `StepError` types
- `StepStatus` enum: success, failed, timeout, budget_exceeded, stuck
- `ExecutionEngine` interface: `Execute(ctx, req) (result, error)`, `Cancel(stepID) error`
- Plan struct: CardID, Complexity, Steps[] mit ID, Title, Wave, DependsOn, ParallelOk, Model, FilesModify, FilesCreate, MustHaves, Verify

---

### Task 2.2: Triage Agent

**Files:**
- Create: `internal/orchestrator/triage.go`
- Create: `internal/orchestrator/triage_test.go`

**Akzeptanzkriterien:**
- `AssessComplexity(ctx, engine, dir, card)` → (trivial | medium | complex)
- Nutzt Haiku via Engine.Execute mit spezifischem Triage-Prompt
- Prompt enthaelt: Card-Titel, Card-Beschreibung, Projektkontext (erkannte Skills)
- Parst JSON-Antwort: `{"complexity": "medium", "reasoning": "..."}`
- Fallback bei Parse-Fehler: medium
- Tests mit gemockter Engine

---

### Task 2.3: Plan Generator + JSON Validation

**Files:**
- Create: `internal/orchestrator/planner.go`
- Create: `internal/orchestrator/planner_test.go`

**Akzeptanzkriterien:**
- `GeneratePlan(ctx, engine, dir, card, skills)` → Plan
- Nutzt Opus via Engine.Execute
- Prompt enthaelt: Card, Akzeptanzkriterien, erkannte Skills, Dateipfade (nicht Inhalte)
- Validiert JSON-Output gegen Plan-Struct
- Bei Validation-Fehler: LLM-Repair (single Haiku call mit Fehlermeldung + Original), max 3x
- Tests: gueltiger Plan, ungueltiger Plan → Repair, 3x Repair fehlgeschlagen → Error

---

### Task 2.4: Wave Planner

**Files:**
- Create: `internal/orchestrator/wave.go`
- Create: `internal/orchestrator/wave_test.go`

**Akzeptanzkriterien:**
- `ComputeWaves(steps []PlanStep)` → []Wave (Wave = []PlanStep)
- Respektiert `depends_on` zwischen Steps
- `parallel_ok: false` Steps bekommen eigene Wave
- Conflict Avoidance: Steps mit `files_modify` Overlap nicht in gleicher Wave
- Cycle Detection: Zirkulaere Dependencies → Error
- Tests: linearer Graph, paralleler Graph, Diamond-Dependency, Cycle, File-Overlap

---

### Task 2.5: Budget Tracker

**Files:**
- Create: `internal/orchestrator/budget.go`
- Create: `internal/orchestrator/budget_test.go`

**Akzeptanzkriterien:**
- `NewBudgetTracker(defaults map[string]float64)` mit Defaults aus Spec (trivial: $0.50, medium: $2.00, complex: $10.00)
- `Allocate(cardID, complexity)` → setzt Budget
- `Spend(cardID, amount)` → reduziert Budget
- `Remaining(cardID)` → verbleibendes Budget
- `CanSpend(cardID, amount)` → bool
- Guard: Budget <= 0 → blockiert weitere Execution
- Tests: Allocation, Spending, Erschoepfung

---

### Task 2.6: Skill Registry + Tech Detection

**Files:**
- Create: `internal/orchestrator/skills.go`
- Create: `internal/orchestrator/skills_test.go`
- Create: `.mtui/skills/go-backend.json` + `.md`
- Create: `.mtui/skills/svelte-frontend.json` + `.md`
- Create: `.mtui/skills/typescript.json` + `.md`
- Create: `.mtui/skills/superpowers-core.json` + `.md`
- Create: `.mtui/skills/security-basics.json` + `.md`

**Akzeptanzkriterien:**
- `DetectStack(dir)` → []string (erkannte Manifest-Dateien: go.mod, package.json, etc.)
- `LoadSkills(skillDir)` → []Skill (parst JSON-Manifeste)
- `MatchSkills(detected []string, allSkills []Skill)` → []Skill (gefiltert nach detect-Patterns)
- `MergePolicies(skills []Skill)` → MergedPolicy (Model, Verify, ScopeLimits nach Konfliktregeln)
- Konfliktregeln aus Spec: hoechstes Model gewinnt, Verify verkettet, strengstes Limit
- Tests: Single Match, Multi Match, Model Conflict, Verify Merge, Scope Limit Min

---

### Task 2.7: Prompt Assembly

**Files:**
- Create: `internal/orchestrator/prompt.go`

**Akzeptanzkriterien:**
- `BuildSystemPrompt(basePrompt, skillPrompts []string)` → string
- `BuildStepPrompt(step PlanStep, cardContext string)` → string
- `BuildExecutionRequest(step, card, mergedPolicy, worktreeSlot)` → ExecutionRequest
- Prompt-Assembly-Verantwortung liegt beim Orchestrator, Engine konsumiert nur
- Kontextminimierung: nur Dateipfade, keine ganzen Dateien

---

### Task 2.8: Orchestrator Main Loop

**Files:**
- Create: `internal/orchestrator/orchestrator.go`
- Create: `internal/orchestrator/orchestrator_test.go`

**Akzeptanzkriterien:**
- `RunCard(ctx, board, engine, dir, cardID)` → error
- Ablauf: Triage → [Quiz] → Plan → Review-Gate → Execute Waves → Decision Briefing → QA → Merge
- Nutzt Board Layer fuer State Transitions
- Nutzt Engine fuer alle claude -p Aufrufe
- Budget-Check vor jeder Execution
- Integration Test mit gemockter Engine + echtem Board (temp Git-Repo)

---

### Task 2.9: Wails Bindings + Frontend fuer Orchestrator

**Files:**
- Create: `internal/backend/app_orchestrate_v3.go` — Wails Bindings
- Modify: `frontend/src/components/KanbanCardDetail.svelte` — Plan Review, Execution Start
- Modify: `frontend/src/stores/kanban.ts` — Orchestration Events

**Akzeptanzkriterien:**
- `StartOrchestration(dir, cardID)` → startet RunCard in Goroutine
- `CancelOrchestration(cardID)` → bricht ab
- Events: `orchestration:triage-done`, `orchestration:plan-ready`, `orchestration:wave-started`, `orchestration:step-done`, `orchestration:qa-result`
- Frontend zeigt Plan-Review mit Approve/Reject
- Frontend zeigt Wave-Progress (welcher Step laeuft, welcher fertig)

---

## Phase 3: Execution Engine + Guardrails

> Label: `phase:3` | Ziel: HeadlessEngine, Worktree Slots, QA Loop, alle Guardrails

### Task 3.1: Worktree Slot Manager

**Files:**
- Create: `internal/engine/slots.go`
- Create: `internal/engine/slots_test.go`

**Akzeptanzkriterien:**
- Pool-basiert, `maxSlots` konfigurierbar (default 4)
- `Allocate(branch)` → (slotID, workDir, error) — blockiert wenn alle belegt (WaitQueue)
- `Release(slotID)` → Worktree loeschen, Slot freigeben
- `Park(slotID)` → Worktree behalten, Slot freigeben (fuer Pause)
- `Prune(staleAfter)` → verwaiste Worktrees aufräumen
- Pfad: `.mtui/worktrees/slot-{N}/`
- Branch: `mtui/<card-id>/<step-id>`
- Tests: Allocate/Release Cycle, Pool-Exhaustion + WaitQueue, Park, Prune

---

### Task 3.2: HeadlessEngine Implementation

**Files:**
- Create: `internal/engine/engine.go` (volle Implementation)
- Create: `internal/engine/engine_test.go`

**Akzeptanzkriterien:**
- Implementiert `ExecutionEngine` Interface
- `Execute()`: Worktree allokieren → claude -p ausfuehren → Verify laufen lassen → git diff parsen → Result bauen → Worktree freigeben
- `Cancel()`: Context canceln
- `claude -p` via COMSPEC (Windows), stdin piping, `--model` flag, `--output-format json`
- `files_changed` aus `git diff --name-only` (nicht aus AI-Output)
- Verify-Commands sequentiell ausfuehren, Ergebnisse sammeln
- Tests mit gemocktem claude -p (Bash-Script das JSON zurueckgibt)

---

### Task 3.3: StepLoopDetector

**Files:**
- Create: `internal/engine/loop_step.go`
- Create: `internal/engine/loop_step_test.go`

**Akzeptanzkriterien:**
- In-Memory, lebt waehrend eines Execute()-Aufrufs
- Error-Normalisierung: Timestamps, tmp-Pfade, Hex-Adressen, volatile Zahlen strippen
- ErrorSignature: `{error_class}:{first_error_line_normalized}:{failing_symbol}`
- 4 Signale: same_error, growing_diff, error_pendulum, no_test_progress
- Tests mit simulierten VerifyResult-Sequenzen fuer jedes Signal
- Tests fuer Error-Normalisierung mit echten Go-Compiler-Outputs

---

### Task 3.4: RepoLoopDetector

**Files:**
- Create: `internal/engine/loop_repo.go`
- Create: `internal/engine/loop_repo_test.go`

**Akzeptanzkriterien:**
- Git-basiert, analysiert `git log --oneline -15`
- 4 Signale: fix_chain, revert, file_churn, pendulum
- `fix_no_test` nur bei Card-Typ bugfix + Skills die Tests erwarten
- Tests mit vorbereiteten Git-Historien (temp Repos mit spezifischen Commit-Patterns)

---

### Task 3.5: Checkpoint Guard

**Files:**
- Create: `internal/engine/checkpoint.go`
- Create: `internal/engine/checkpoint_test.go`

**Akzeptanzkriterien:**
- Progress-basiert: DiffHash, VerifyOutputs, FailingTests, ErrorClass, FilesExist
- Check alle 5min
- 2 Checks ohne Progress → Warnung
- 3 Checks ohne Progress → timeout
- Tests mit simulierten ProgressSnapshot-Sequenzen

---

### Task 3.6: QA Fix Loop

**Files:**
- Extend: `internal/engine/engine.go`
- Extend: `internal/engine/engine_test.go`

**Akzeptanzkriterien:**
- Nach fehlgeschlagenem Verify: automatischer Fix-Agent (Sonnet)
- Fix-Prompt enthaelt: Fehler-Output, betroffene Dateien, Original-Step-Prompt
- Max 3 Iterationen
- StepLoopDetector prueft nach jedem Fix-Versuch
- Bei 3x fail oder Loop-Signal → Status: stuck

---

### Task 3.7: Escalation Pipeline

**Files:**
- Extend: `internal/orchestrator/orchestrator.go`

**Akzeptanzkriterien:**
- Bei stuck: Model-Escalation (haiku→sonnet→opus), max 2x
- Bei weiterhin stuck: Re-Planning (scope-begrenzt, max 3 Sub-Steps, nur Original-Files)
- Bei scope_expansion_required: human_review mit Kontext-Package
- Bei MaxEscalations: human_review
- Kontext-Package fuer human_review: betroffene Files, Fehlerlog, was versucht wurde

---

### Task 3.8: Decision Briefing + Secrets Scanner

**Files:**
- Create: `internal/engine/briefing.go`
- Create: `internal/engine/briefing_test.go`
- Create: `frontend/src/components/DecisionBriefing.svelte`

**Akzeptanzkriterien:**
- `BuildBriefing(repo, cardID, activeCards)` → DecisionBriefing
- ScopeCheck: FilesChanged, LinesAdded/Deleted vs Limits
- SecretsScanner: Regex fuer AWS_KEY, GITHUB_TOKEN, STRIPE_SK, PRIVATE_KEY, DB_CREDENTIALS
- Secrets IMMER redacted (nur Typ + File + Line + masked Preview)
- ConflictRisk: basierend auf file-overlap mit anderen executing Cards
- DependencyRisk: go.mod/package.json Diff-Analyse
- ManifestChanges: strukturiert (File, Kind, Package, From, To)
- Reasons[]: maschinenlesbare Begruendungen
- Recommendation: proceed_to_qa | needs_human_review | revert_recommended
- Frontend-Komponente zeigt Briefing vor QA-Transition
- Tests fuer jeden Risk-Level, Secret-Pattern, False-Positive-Pruefung

---

### Task 3.9: Scope Limits + Conflict Avoidance

**Files:**
- Extend: `internal/orchestrator/wave.go` — Conflict Avoidance
- Extend: `internal/engine/engine.go` — Scope Check nach Step
- Extend: `internal/engine/briefing.go` — Scope in Briefing

**Akzeptanzkriterien:**
- Pre-Wave: files_modify Overlaps erkennen, konflikttraechtige Steps in naechste Wave
- Post-Step: git diff --stat gegen ScopeLimits pruefen
- Ausnahmen: git mv, generated files, vendor/, dist/, build/
- Bei Ueberschreitung: review_required Flag, Warnung in UI
- Tests fuer Overlap-Detection, Scope-Berechnung, Ausnahmen

---

### Task 3.10: Wave Merge + Dependency Reconciliation

**Files:**
- Create: `internal/engine/merge.go`
- Create: `internal/engine/merge_test.go`

**Akzeptanzkriterien:**
- `MergeWave(repo, worktrees []string, targetBranch)` → MergeResult
- Sequentieller Merge der Worktree-Branches in Card-Branch
- Nach Merge: Dependency Reconciliation (go mod tidy, npm install wenn relevant)
- Falls Reconciliation zu Version-Upgrades/Removals fuehrt: DependencyRisk: high
- AI-Merge nur fuer kleine Konflikte (max 3 Dateien, max 2 Hunks/Datei, max 20 Zeilen, nicht in Critical Files)
- Alles andere: human_review
- Tests mit vorbereiteten Merge-Szenarien

---

### Task 3.11: Frontend Guardrail Integration

**Files:**
- Create: `frontend/src/components/EscalationDialog.svelte`
- Modify: `frontend/src/components/KanbanCard.svelte` — Scope-Badges, Loop-Warnings
- Modify: `frontend/src/components/KanbanCardDetail.svelte` — Briefing, Escalation

**Akzeptanzkriterien:**
- EscalationDialog: zeigt Kontext-Package, User kann: Scope erweitern, Task teilen, abbrechen
- KanbanCard: Badge fuer scope_exceeded, stuck, human_review mit Reason
- KanbanCardDetail: Decision Briefing eingebettet, Execution-Timeline mit Step-Status
- Events: `orchestration:escalation`, `orchestration:briefing-ready`

---

## Phase 4: Agent Teams Integration (Future)

> Label: `phase:4` | Ziel: Optionale Live-Terminal-View via Agent Teams

*Nicht Teil der initialen Implementation. Tickets werden als Placeholders angelegt.*

### Task 4.1: Tmux-Shim capture-pane implementieren
### Task 4.2: AgentTeamsEngine implementieren
### Task 4.3: Live-Terminal-View fuer Teammates
### Task 4.4: CustomPaneBackend evaluieren (anthropics/claude-code#26572)
