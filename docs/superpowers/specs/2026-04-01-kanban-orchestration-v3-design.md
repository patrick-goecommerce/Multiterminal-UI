# MTUI Kanban Orchestration v3 — Design Specification

> Status: APPROVED | Date: 2026-04-01
> Branch: fresh from `alpha-main`, v2 branch archived as reference
> Milestone: "Kanban Orchestration v3" with phase labels

## 1. Overview

MTUI wird um eine vollstaendige AI-Agent-Orchestrierungsschicht erweitert, die Kanban-Tickets automatisiert umsetzt. Die Architektur basiert auf einer Synthese aus 7 analysierten Tools (Just Ship, GSD, Aperant, Agent Teams, everything-claude-code, App::karr, PulseFramework), wobei nur bewaehrte Patterns uebernommen und identifizierte Anti-Patterns bewusst vermieden werden.

### Pattern-Herkunft

| Pattern | Quelle |
|---------|--------|
| Git-Ref Board Storage, Atomic Locking | App::karr |
| Task State Machine | Aperant (XState-Konzept) |
| JSON Plan mit Schema-Validation | GSD (Konzept), eigene Anti-Pattern-Regel |
| Fresh Context per Agent | GSD |
| Tiered Model Selection | Just Ship |
| Wave-Based Parallel Execution | GSD + MTUI-existing |
| Worktree Slot Manager | Just Ship |
| QA Fix Loop | Just Ship + Aperant |
| Capability Layer (Skills) | everything-claude-code + Just Ship |
| Loop Detection (Step + Repo) | PulseFramework |
| Escalation Pipeline mit Model-Tiering | PulseFramework |
| Scope Limits pro Card-Typ | PulseFramework |
| Checkpoint Guard (Progress-basiert) | PulseFramework |
| Decision Briefing (Pre-QA Gate) | PulseFramework |
| Secrets Scanner | PulseFramework |
| Harter Contract Orchestrator-Engine | Eigenes Design |
| Conflict Avoidance + DependencyRisk | Eigenes Design |

### Vermiedene Anti-Patterns

| Anti-Pattern | Quelle | Warum vermieden |
|--------------|--------|-----------------|
| Agents schreiben in denselben State wie Orchestrator | Aperant | Fuehrt zu Daten-Korruption |
| String-Matching fuer Verdicts | Aperant | Fragil, nutzt JSON + Schema |
| Over-Engineering der Memory-Schicht | Aperant | 30+ Dateien fuer einfache Inject-Funktion |
| 50 QA-Iterationen ohne Budget | Aperant | Token verbrennen ohne User-Wissen |
| XML als Maschinenformat | GSD | Kollidiert mit JSON-Schema-Regel |
| Keine Merge-Conflict-Resolution | Just Ship | Parallele Branches koennen konfligieren |
| Kein Inter-Agent-Communication | Just Ship | Agents brauchen Laufzeit-Austausch |

---

## 2. Architecture

### 2.1 Layer-Modell

```
MTUI Backend (Go)

  Board Layer
    Git-Ref Storage (refs/mtui/*)
    Task State Machine
    Atomic Locking

  Orchestrator
    Triage (Complexity Gating)
    Plan Generator (JSON, Schema-validated)
    Wave Planner (Dependency Resolution)
    Budget Tracker
    Capability Layer (Skills)
      Tech Detection
      Skill Registry
      Prompt Bundles
      Execution Policies

  Execution Engine (Interface)
    Primary: HeadlessEngine (claude -p)
    Future: AgentTeamsEngine (Tmux-Shim)
    Worktree Slot Manager
    QA Fix Loop
    Loop Detection (Step + Repo)
    Checkpoint Guard
    Decision Briefing
```

### 2.2 State Ownership (Invariant)

| Layer | Besitzt | Mutiert NICHT |
|-------|---------|---------------|
| Board Layer | Task-Zustand, Plan-Ref, Locks | Laufzeit-Daten, Logs |
| Orchestrator | Routing, Planung, Waves, Budget, Policies | Task-State direkt (nur via Board Layer API) |
| Execution Engine | Laufzeit-Ausfuehrung, Verify-Ergebnisse | Board-State, Plan, Policies |
| Capability Layer | Nichts (stateless) | Beeinflusst Policies, besitzt keinen State |

**Invariante:** Kein Layer darf den State eines anderen Layers direkt mutieren. Kommunikation nur ueber definierte Interfaces und strukturierte Messages.

### 2.3 Storage-Trennung

| Daten | Storage | Begruendung |
|-------|---------|-------------|
| Card-State, Board-Zustand | `refs/mtui/tasks/*/content` | Muss mit Git reisen, Multi-Agent-safe |
| Locks, Checkpoints | `refs/mtui/tasks/*/lock` | Atomic, kurzlebig |
| Plan (JSON, kompakt) | `refs/mtui/tasks/*/plan` | Klein, versioniert, gehoert zum Task |
| QA-Reports | `.mtui/qa/<card-id>/` (gitignored) | Koennen gross werden, volatil |
| Execution-Logs | `.mtui/logs/<card-id>/` (gitignored) | Streaming-Output |
| Agent-Artefakte | Worktree (temporaer) dann Merge | Gehoeren in den Code |
| Live-Stream-Data | In-Memory + Wails Events | Fluechtig, nie persistiert |

**Ref-Plan-Regel (hart):**
- Ref speichert NUR den zuletzt gueltigen, kompakten Arbeitsplan
- Max ~50KB (typisch 2-5KB)
- Keine Reasoning-Texte, keine Review-Historie, keine QA-Dialoge
- Revisionen ueber `git reflog refs/mtui/tasks/<id>/plan`

---

## 3. Board Layer

### 3.1 Git-Ref Storage

```
refs/mtui/
  config                          Board-Konfiguration (Columns, WIP-Limits)
  tasks/<id>/content              Task als Markdown + YAML-Frontmatter
  tasks/<id>/lock                 Mutex-Ref (Agent-Name + Timestamp)
  tasks/<id>/plan                 Kompakter JSON-Plan
  log/<agent-name>                Activity Log (JSON Lines)
  skills/detected                 Tech-Stack Cache
```

Go-Package: `internal/board/`
- `refstore.go` — Low-Level Git-Ref Ops (hash-object, mktree, update-ref)
- `board.go` — High-Level CRUD (CreateTask, MoveTask, ListTasks, GetTask)
- `lock.go` — Atomic Locking mit Timeout (stale nach 5min)
- `sync.go` — fetch/push von refs/mtui/*
- `state.go` — Task State Machine

### 3.2 Task State Machine

```
backlog
  StartTriage
triage                    Haiku: Complexity Assessment
  ComplexityAssessed
  [trivial]  ---------> executing (1 Agent, kein Plan)
  [medium]   ---------> planning
  [complex]  ---------> planning

planning                  Opus: Quiz + Plan Generation
  PlanReady
review                    User prueft Plan (medium + complex, NIE trivial)
  Approved
executing                 Wave-basiert, pro Step ein Agent
                          (gilt fuer Impl UND QA-Fix — gleicher State, Substatus in Metadata)
  StepStuck
stuck                     Loop/Timeout/Budget
  ModelEscalated -------> executing (hoeheres Model)
  ReplanCompleted -------> executing (Sub-Steps, scope-begrenzt)
  ScopeExpansionRequired -> human_review
  MaxEscalations -------> human_review
  AllStepsDone
qa                        must_haves Verification + Decision Briefing
  QAPassed
  QAFailed (max 3x) ----> executing (QA Fix Loop)
merging                   Worktree in Hauptbranch
  MergeSuccess
  MergeConflict ---------> human_review
human_review              User-Intervention erforderlich
  UserResolved ----------> executing | done | backlog
done                      Abgeschlossen, Learnings persistiert
```

**Guards:**
- `executing` nur wenn Budget > 0
- `qa -> executing` max 3x (QA Fix Loop Limit)
- `stuck -> executing` max 2x (Escalation Limit)
- Jede Transition emittet Wails Event `board:task-transition`

---

## 4. Orchestrator

### 4.1 Pipeline

```
Card nach "executing"
  Orchestrator liest Plan aus refs/mtui/tasks/<id>/plan
  Berechnet Waves aus depends_on
  Fuer jede Wave:
    Conflict Avoidance Check (files_modify Overlaps)
    Parallel fuer jeden Step:
      Worktree-Slot allokieren
      Skills matchen + Policies mergen
      Context Builder: nur betroffene Dateien + Nachbar-Interfaces + Tests
      ExecutionRequest bauen (Orchestrator assembliert, Engine konsumiert)
      Engine.Execute(request)
      ExecutionResult empfangen
      Step Loop Detection pruefen
      Verify pruefen
        Bei Failure: QA Fix Loop (max 3x)
      Scope Limits pruefen
      Worktree-Slot freigeben
      Step-Status in Board Layer updaten
    Wave-Merge: Worktrees in Hauptbranch
      Dependency Reconciliation (go mod tidy etc.)
      Bei Konflikt: nur kleine lokale Textkonflikte per AI-Merge
      Kritische/strukturelle Konflikte: human_review
    Anti-Drift-Checkpoint alle 2 Waves (Haiku)
  Alle Waves done: Decision Briefing (Pre-QA Gate)
  Card nach "qa"
  QA: must_haves gegen Codebase
    Pass: Card nach "merging" -> "done"
    Fail: QA Fix Loop (max 3x), dann human_review
  Learnings persistieren
```

### 4.2 Plan Format (JSON)

```json
{
  "card_id": "feat-auth",
  "complexity": "medium",
  "steps": [
    {
      "id": "01",
      "title": "Auth endpoint erstellen",
      "wave": 1,
      "depends_on": [],
      "parallel_ok": true,
      "model": "sonnet",
      "files_modify": ["internal/backend/app_auth.go"],
      "files_create": ["internal/backend/app_auth_test.go"],
      "must_haves": {
        "truths": ["POST /api/auth returns 200 with valid credentials"],
        "artifacts": [
          {"path": "internal/backend/app_auth.go", "min_lines": 30}
        ]
      },
      "verify": [
        {"command": "go build ./...", "description": "Build succeeds"},
        {"command": "go test ./internal/backend/... -run TestAuth", "description": "Auth tests pass"}
      ]
    }
  ]
}
```

**Validation:** Go struct tags + custom validator. Bei Malformed-Output: LLM-Repair-Call (single generateText, kein Re-Planning), max 3x.

**verify als Liste** (nicht String): Besser reportbar, besser debuggbar, klar sichtbar welcher Schritt scheitert.

### 4.3 Complexity Gating

| Complexity | Pipeline | Geschaetzte Kosten |
|------------|----------|--------------------|
| trivial | Triage -> Execute (1 Agent, kein Plan) | ~$0.01-0.05 |
| medium | Triage -> Plan -> Review -> Execute -> QA | ~$0.10-0.50 |
| complex | Triage -> Quiz -> Plan -> Review -> Execute -> QA | ~$0.50-5.00 |

### 4.4 Tiered Model Selection

| Aufgabe | Model | Begruendung |
|---------|-------|-------------|
| Triage, QA Verification | Haiku | Mechanisch, regelbasiert |
| Code Implementation, QA Fixes | Sonnet | Kreativ, aber instruktionsgetrieben |
| Orchestrierung, Planung, Quiz | Opus | Architektur-Entscheidungen |
| Anti-Drift Checkpoint | Haiku | Vergleichsarbeit |

### 4.5 Budget Tracking

```go
type BudgetTracker struct {
    CardBudgets map[string]float64  // card_id -> remaining USD
    Defaults    map[string]float64  // complexity -> default budget
}
```

| Complexity | Default Budget |
|------------|---------------|
| trivial | $0.50 |
| medium | $2.00 |
| complex | $10.00 |

Bei Budget-Erschoepfung: Transition blockiert, Card nach `human_review` mit reason `budget_exceeded`.

---

## 5. Capability Layer (Skills)

### 5.1 Skill Definition

```json
{
  "name": "go-backend",
  "detect": ["go.mod"],
  "priority": 10,
  "stackable": true,
  "prompt_file": "skills/go-backend.md",
  "policies": {
    "preferred_model": "sonnet",
    "verify": [
      {"command": "go build ./...", "description": "Build"},
      {"command": "go test ./...", "description": "Tests"},
      {"command": "go vet ./...", "description": "Vet"}
    ],
    "scope_limits": {
      "bugfix": {"max_files": 10, "max_lines": 200, "max_deletes": 50},
      "feature": {"max_files": 25, "max_lines": 500, "max_deletes": 100},
      "refactor": {"max_files": 40, "max_lines": 800, "max_deletes": 400}
    },
    "qa_rules": ["no-global-state", "error-wrapping"]
  }
}
```

### 5.2 Skill-Typen

| Typ | Beispiele | Aktivierung |
|-----|-----------|-------------|
| Universal | Superpowers, TDD, Code Review, Security Basics | Immer aktiv |
| Tech-specific | go-backend, svelte-frontend, typescript-rules | Auto-detect via Manifest-Dateien |
| On-demand | Performance Audit, Accessibility, i18n | User aktiviert oder Agent empfiehlt |

### 5.3 Konfliktregeln

| Situation | Regel |
|-----------|-------|
| Mehrere Skills matchen | Alle `stackable: true` Skills geladen |
| preferred_model Konflikt | Hoechstes Model gewinnt (opus > sonnet > haiku) |
| verify Konflikt | Commands werden zusammengefuehrt (dedupliziert) |
| scope_limits Konflikt | Strengstes Limit gewinnt (min) |
| stackable: false | Exklusiv, verdraengt andere gleicher Prioritaet |

### 5.4 Prompt Assembly

**Invariante:** Der Orchestrator assembliert Prompts, die Engine konsumiert.

```
SystemPrompt = base_system_prompt
             + skill_prompts (nach Prioritaet sortiert)
             + card_context (Beschreibung, Akzeptanzkriterien)

StepPrompt   = step_description
             + files_to_read (Pfade, nicht Inhalte — Agent liest selbst)
             + must_haves fuer diesen Step
             + verify_commands

Kontext wird brutal klein gehalten:
  - Nur direkt betroffene Dateien
  - Relevante Nachbar-Interfaces
  - Relevante Tests
  - Skill-spezifische Hinweise
  - KEINE ganzen Verzeichnisse oder Codebases
```

---

## 6. Execution Engine

### 6.1 Interface

```go
type ExecutionEngine interface {
    Execute(ctx context.Context, req ExecutionRequest) (ExecutionResult, error)
    Cancel(stepID string) error
}
```

### 6.2 Contract: Request

```go
type ExecutionRequest struct {
    StepID        string         `json:"step_id"`
    CardID        string         `json:"card_id"`
    WorktreeSlot  int            `json:"worktree_slot"`
    Prompt        string         `json:"prompt"`
    SystemPrompt  string         `json:"system_prompt"`
    Model         string         `json:"model"`
    Verify        []VerifyStep   `json:"verify"`
    BudgetUSD     float64        `json:"budget_usd"`
    TimeoutSec    int            `json:"timeout_sec"`
    SkillPrompts  []string       `json:"skill_prompts"`
}

type VerifyStep struct {
    Command     string `json:"command"`
    Description string `json:"description"`
}
```

### 6.3 Contract: Response

```go
type ExecutionResult struct {
    StepID       string         `json:"step_id"`
    Status       StepStatus     `json:"status"`
    FilesChanged []string       `json:"files_changed"`
    FilesCreated []string       `json:"files_created"`
    Verify       []VerifyResult `json:"verify"`
    LoopSignals  []LoopSignal   `json:"loop_signals"`
    CostUSD      float64        `json:"cost_usd"`
    DurationSec  int            `json:"duration_sec"`
    Error        *StepError     `json:"error,omitempty"`
}

// StepStatus: "success" | "failed" | "timeout" | "budget_exceeded" | "stuck"

type VerifyResult struct {
    Command     string `json:"command"`
    Description string `json:"description"`
    ExitCode    int    `json:"exit_code"`
    Output      string `json:"output"`    // truncated, max 2KB
    Passed      bool   `json:"passed"`
}

type LoopSignal struct {
    Type    string `json:"type"`
    Detail  string `json:"detail"`
    Source  string `json:"source"`  // "step" | "repo"
}

type StepError struct {
    Class   string `json:"class"`   // "build" | "test" | "timeout" | "budget" | "crash" | "scope_exceeded"
    Message string `json:"message"`
}
```

**Regeln:**
- Engine liefert IMMER dieses Format, nie freien Text
- `files_changed` kommt aus `git diff --name-only` (Fakt, nicht AI-Behauptung)
- `verify` wird von der Engine ausgefuehrt, nicht vom Agent
- Bei `status: "stuck"` entscheidet der Orchestrator ueber Escalation

### 6.4 Worktree Slot Manager

```go
type WorktreeSlotManager struct {
    maxSlots  int                      // Konfigurierbar, default 4
    slots     map[int]*WorktreeSlot
    waitQueue chan struct{}
}

func (m *WorktreeSlotManager) Allocate(branch string) (slot int, workDir string, err error)
func (m *WorktreeSlotManager) Release(slot int) error
func (m *WorktreeSlotManager) Park(slot int) error
func (m *WorktreeSlotManager) Prune(staleAfter time.Duration) error
```

Pfad: `.mtui/worktrees/slot-{N}/`
Branch: `mtui/<card-id>/<step-id>`

### 6.5 HeadlessEngine (Primary)

```go
type HeadlessEngine struct {
    slots       *WorktreeSlotManager
    stepDetect  *StepLoopDetector
    repoDetect  *RepoLoopDetector
    checkpoint  *CheckpointGuard
}
```

Nutzt `claude -p --output-format json` mit:
- Stdin-Piping fuer Prompts (kein argv-Limit)
- Frischer Kontext pro Aufruf (GSD-Pattern)
- Model-Selection via `--model` Flag

### 6.6 AgentTeamsEngine (Future)

```go
type AgentTeamsEngine struct {
    shim     *TmuxShim
    sessions map[string]int  // stepID -> sessionID
}
```

Voraussetzungen fuer Aktivierung:
- Tmux-Shim `capture-pane` implementiert
- Agent Teams stable (nicht mehr experimental)
- Oder CustomPaneBackend (anthropics/claude-code#26572) shipped

---

## 7. Operational Guardrails

### 7.1 Loop Detection (Zwei Mechanismen)

**A. StepLoopDetector — In-Memory, waehrend Execute()**

| Signal | Erkennung | Reaktion |
|--------|-----------|----------|
| same_error | Normalisierter ErrorHash identisch ueber 2+ Versuche | Escalation |
| growing_diff | Diff waechst, Fehler bleibt gleich | Escalation |
| error_pendulum | ErrorHash alterniert zwischen 2 Werten | Step abbrechen |
| no_test_progress | Failing Tests unveraendert trotz Code-Aenderungen | Escalation |

**Error-Normalisierung vor Hashing (Invariante):**
- Timestamps strippen
- Temporaere Pfade normalisieren
- Hex-Adressen entfernen
- Absolute Pfade kuerzen
- Volatile Zahlen bereinigen
- Zusaetzlich: ErrorSignature extrahieren (Fehlerklasse + markante Tokens)

**B. RepoLoopDetector — Git-basiert, nach Step-Ende**

| Signal | Erkennung |
|--------|-----------|
| fix_chain | 3+ aufeinanderfolgende Fix-Commits |
| revert | Commit macht vorherigen rueckgaengig |
| file_churn | Gleiche Datei in 5+ von 15 Commits |
| pendulum | Aehnliche Messages alternierend |

**fix_no_test ist kontextabhaengig:** Nur bei Card-Typ `bugfix` + Skills die Tests erwarten. Nicht bei `docs`, `refactor`, `css-only`.

### 7.2 Checkpoint Guard (Progress-basiert)

```go
type ProgressSnapshot struct {
    DiffHash       string
    VerifyOutputs  []string
    FailingTests   int
    ErrorClass     string
    FilesExist     int
    Timestamp      time.Time
}
```

**Progress-Signale (mindestens eins muss sich aendern):**

| Signal | Pruefung |
|--------|----------|
| Code-Aenderung | DiffHash veraendert |
| Verify-Verbesserung | FailingTests gesunken oder ErrorClass gewechselt |
| Neues Artefakt | FilesExist gestiegen |
| Commit | Neuer Commit seit letztem Check |

Check-Intervall: 5 Minuten.
2 Checks ohne Fortschritt (10min): timeout-Warnung.
3 Checks ohne Fortschritt (15min): Step abbrechen.

### 7.3 Escalation Pipeline

```
Step failed
  QA Fix Loop (max 3x, gleiches Model)
  Model-Escalation (haiku -> sonnet -> opus)
  Re-Planning (HART BEGRENZT, siehe unten)
  human_review (mit Context-Package)
```

**Re-Planning Constraints:**

| Regel | Wert |
|-------|------|
| Darf nur den betroffenen Step zerlegen | Ja |
| Max Sub-Steps | 3 |
| Nur Dateien aus Original-Step-Scope | Ja |
| Neue Dateien ausserhalb Scope | Nein |
| Wave-Dependencies aendern | Nein |
| Budget | Nur verbleibendes Step-Budget |
| must_haves | Identisch zum Original |

**Wenn Loesung nur mit Scope-Erweiterung moeglich:**
Transition zu `human_review` mit `reason: "scope_expansion_required"`.
Kein sinnloses Budgetverbrennen in kuenstlich zu kleinem Kaefig.

### 7.4 Scope Limits

| Card-Typ | MaxFiles | MaxLines | MaxDeletes |
|----------|----------|----------|------------|
| bugfix | 10 | 200 | 50 |
| feature | 25 | 500 | 100 |
| refactor | 40 | 800 | 400 |

**Ausnahmen fuer MaxDeletes:**
- `git mv` (Rename+Move) zaehlt nicht als Delete
- Generated Files (`*_generated.go`, `vendor/`, `dist/`, `build/`) ausgenommen
- `*.min.js`, `*.min.css` ausgenommen

### 7.5 Decision Briefing (Pre-QA Gate)

```go
type DecisionBriefing struct {
    CardID           string
    FilesChanged     int
    LinesAdded       int
    LinesDeleted     int
    ScopeStatus      string          // "within_limits" | "exceeded" | "warning"

    // Security (maskiert!)
    SecretsFound     []SecretFinding
    MassDeletes      bool

    // Loop History
    LoopHistory      []LoopSignal

    // Merge-Risiko
    ConflictRisk     string          // "low" | "medium" | "high"
    CriticalFiles    []string
    SharedSurfaces   []string

    // Dependency-Risiko (separiert von Code-Risiko)
    DependencyRisk      string            // "none" | "low" | "high"
    ManifestChanges     []ManifestChange  // strukturierte Manifest-Diff-Info

    Recommendation   string
    Reasons          []string  // z.B. ["dependency_version_changed", "critical_file_overlap", "secret_detected"]
}

type ManifestChange struct {
    File    string `json:"file"`    // "go.mod"
    Kind    string `json:"kind"`    // "add" | "update" | "remove"
    Package string `json:"package"` // "github.com/foo/bar"
    From    string `json:"from"`    // "v1.2.0" (leer bei add)
    To      string `json:"to"`      // "v1.3.0" (leer bei remove)
}

type SecretFinding struct {
    Type     string  // "AWS_KEY" | "GITHUB_TOKEN" | "PRIVATE_KEY" | "DB_CREDENTIALS"
    File     string  // "config/app.env"
    Line     int
    Preview  string  // "STRIPE_SK=sk_live_****" (redacted, NIE Klartext)
}
```

**ConflictRisk:**
- high: Critical File geaendert + andere Cards in executing
- medium: Shared Surface geaendert (gleiche Package wie andere aktive Cards)
- low: Keine Ueberschneidung

**DependencyRisk:**
- none: Keine Manifest-Aenderungen
- low: Nur neue Dependencies hinzugefuegt
- high: Bestehende Versionen geaendert, Downgrades, Pakete entfernt

**DependencyRisk-Aktionen:**
- none: Normal weiter
- low: Hinweis + Reconciliation Step (go mod tidy etc.)
- high: human_review Pflicht

**Critical Files (konfigurierbar, Defaults):**
```
go.mod, go.sum, package.json, package-lock.json
**/router.go, **/routes.ts
**/schema.sql, **/migrations/**
**/auth.go, **/auth.ts, **/middleware/**
.env*, **/config.go, **/config.ts
```

### 7.6 Conflict Avoidance (vor jeder Wave)

**Invariante:** Konfliktvermeidung ist wichtiger als Konfliktaufloesung.

Vor jeder Wave:
1. `files_modify`-Overlaps zwischen parallelen Steps erkennen
2. Kritische Dateien exklusiv sperren (nur ein Step darf z.B. `router.go` aendern)
3. Konflikttraechtige Steps nicht parallel planen (in naechste Wave verschieben)

AI-Merge nur bei:
- Kleinen, lokal begrenzten Textkonflikten
- Unkritischen Dateien
- NIE als generischer Default-Pfad

Kritische/strukturelle Konflikte: Sofort `human_review`.

---

## 8. Failure Modes & Decision Tables

### 8.1 Step Failure Decision Table

| Situation | Aktion |
|-----------|--------|
| Build-Fehler, 1. Versuch | QA Fix Loop (Sonnet) |
| Build-Fehler, 3. Versuch | Model-Escalation |
| Same Error 2x (StepLoopDetector) | Model-Escalation |
| Error Pendulum erkannt | Step abbrechen, human_review |
| Kein Progress 15min | Step timeout, Escalation |
| Budget erschoepft | human_review (budget_exceeded) |
| Scope ueberschritten | Review-Flag, Warnung in UI |
| Re-Planning noetig aber Scope zu eng | human_review (scope_expansion_required) |
| Model-Escalation 2x fehlgeschlagen | human_review |
| Secret gefunden | Card blockiert bis User bestaetigt |
| Mass Delete erkannt | Review-Flag |
| Critical File geaendert + andere Cards aktiv | ConflictRisk: high, Warnung |
| Dependency downgrade/removal | human_review Pflicht |

### 8.2 Wave Merge Decision Table

| Situation | Aktion |
|-----------|--------|
| Kein Konflikt | Auto-Merge |
| Kleiner Textkonflikt, unkritische Datei | AI-Merge (Sonnet) |
| Struktureller Konflikt | human_review |
| Critical File Konflikt | human_review |
| Dependency Manifest Konflikt | Reconciliation (go mod tidy), dann Verify |
| AI-Merge fehlgeschlagen | human_review |

### 8.3 Invariants

1. **Engine liefert immer strukturiertes ExecutionResult** — nie freien Text
2. **files_changed kommt aus git diff** — nie aus AI-Behauptungen
3. **verify wird von Engine ausgefuehrt** — nie vom Agent
4. **Orchestrator assembliert Prompts** — Engine konsumiert nur
5. **Kein Layer mutiert State eines anderen** — nur via definierte Interfaces
6. **Ref-Plan bleibt kompakt** — max 50KB, keine Historie im Ref
7. **Secrets werden nie in Klartext gespeichert** — nur Typ + Ort + redacted Preview
8. **Re-Planning darf Scope nicht erweitern** — bei Bedarf: human_review
9. **AI-Merge nie als Default** — nur fuer kleine, lokale, unkritische Konflikte
10. **Fresh Context pro Agent** — kein Kontext-Bleeding zwischen Steps

---

## 9. Phased Implementation

### Phase 0: Stabilisierung
- Build verifizieren (kompiliert MTUI?)
- Bestehenden Kanban-Code auditen
- RunHeadless end-to-end testen
- Frischen Branch von alpha-main erstellen

### Phase 1: Board Layer + State Machine
- internal/board/ Package mit Git-Ref Storage
- Task State Machine mit allen Transitions
- Atomic Locking
- Wails Bindings + Frontend Kanban Board (Drag-and-Drop, State-Anzeige)

### Phase 2: Orchestrator + Capability Layer
- Triage Agent (Haiku, Complexity Gating)
- Plan Generator (JSON, Schema-Validation, LLM-Repair)
- Wave Planner (Dependency Resolution)
- Skill Registry + Tech Detection + Prompt Assembly
- Budget Tracker

### Phase 3: Execution Engine + Guardrails
- HeadlessEngine (claude -p, frischer Kontext)
- Worktree Slot Manager
- QA Fix Loop
- StepLoopDetector + RepoLoopDetector
- Checkpoint Guard
- Escalation Pipeline
- Decision Briefing + Secrets Scanner
- Scope Limits + Conflict Avoidance

### Phase 4: Agent Teams Integration (Future)
- Tmux-Shim vervollstaendigen (capture-pane)
- AgentTeamsEngine implementieren
- Live-Terminal-View fuer Teammates
- CustomPaneBackend wenn anthropics/claude-code#26572 shipped

---

## 10. Testability

Heuristische Mechanismen die dedizierte Testfaelle brauchen:

| Mechanismus | Testansatz |
|-------------|-----------|
| ErrorHash Normalisierung | Unit Tests mit echten Compiler-/Test-Outputs, volatile Teile verifizieren |
| StepLoopDetector | Unit Tests mit simulierten VerifyResult-Sequenzen |
| RepoLoopDetector | Integration Tests mit vorbereiteten Git-Historien |
| Checkpoint Guard | Unit Tests mit simulierten ProgressSnapshot-Sequenzen |
| ConflictRisk | Unit Tests mit verschiedenen File-Overlap-Szenarien |
| DependencyRisk | Unit Tests mit go.mod/package.json Diffs |
| Scope Limits | Unit Tests mit git diff --stat Outputs |
| Secrets Scanner | Unit Tests mit bekannten Secret-Patterns + False-Positive-Pruefung |
| Skill Conflict Resolution | Unit Tests mit mehreren matchenden Skills |
| State Machine Transitions | Unit Tests fuer jede Transition + Guard |
| Wave Planner | Unit Tests fuer Dependency-Graphen + Conflict Avoidance |
| AI-Merge Grenzen | Integration Tests mit verschiedenen Konflikt-Groessen + kritischen Dateien |
| ManifestChange Parser | Unit Tests mit echten go.mod/package.json Diffs |
| Reasons-Generierung | Unit Tests: gegebene Briefing-Daten -> erwartete Reasons-Liste |

---

## 11. Non-Goals

Dinge die ausdruecklich NICHT Teil dieser Spec sind:

1. **Eigene AI-Runtime** — Wir nutzen `claude -p` / Agent Teams, bauen keinen eigenen LLM-Runner
2. **Multi-Repo-Support** — Ein Board pro Repo. Cross-Repo-Orchestrierung ist out of scope
3. **Echtzeit-Collaboration** — Kein gleichzeitiges Editieren desselben Tasks durch mehrere User
4. **CI/CD-Integration** — Kein automatisches Deployment nach Done. Nur Code + PR
5. **Custom Tool Definitions** — Skills definieren Kontext und Policies, keine eigenen Tools
6. **Automatisches Merging in main** — Cards produzieren Branches/PRs, nie direkten Push
7. **Billing/Payment** — Budget-Tracking ist informativ, keine echte Kostenabrechnung
8. **Mobile UI** — Desktop-only (Wails WebView)
9. **Plugin-System fuer externe Skills** — Skills kommen aus dem Repo, nicht aus einem Marketplace (Continuous Learning + Skill Contributing ist separates Issue #102)
10. **Backwards-Kompatibilitaet mit v2** — Frischer Branch, v2-Daten werden nicht migriert

---

## 12. Open Questions

Offene Punkte die waehrend der Implementation geklaert werden muessen:

### OQ-1: AI-Merge — Was genau ist "klein"?

Aktuell: "kleine, lokal begrenzte Textkonflikte in unkritischen Dateien."
Muss operational definiert werden, z.B.:
- Max N Dateien mit Konflikten (Vorschlag: 3)
- Max N Conflict-Hunks pro Datei (Vorschlag: 2)
- Keine Konflikte in Critical Files (denylist)
- Kein Konflikt groesser als N Zeilen (Vorschlag: 20)

**Entscheidung:** Waehrend Phase 3 Implementation anhand realer Merge-Szenarien festlegen.

### OQ-2: Claude Agent SDK fuer Go?

Just Ship nutzt das TypeScript Agent SDK erfolgreich. Fuer MTUI (Go-Backend) gibt es drei Optionen:
- (a) Weiter `claude -p` via CLI (aktueller Plan)
- (b) Node.js Sidecar mit Agent SDK
- (c) Warten auf offizielles Go SDK

**Entscheidung:** Phase 3 starten mit (a). Evaluieren ob (b) oder (c) spaeter Vorteile bringt.

### OQ-3: Skill-Bibliothek Umfang zum Launch

Wie viele Skills brauchen wir fuer Phase 2 MVP?
- Minimum: 3 universelle (Superpowers-Kern, Code Review, Security Basics) + 2 tech-spezifische (Go, TypeScript)
- Nice-to-have: Svelte, React, Python, Rust

**Entscheidung:** Mit Minimum starten, erweitern basierend auf User-Feedback.

### OQ-4: executing Substatus

`executing` gilt fuer normale Implementation UND QA-Fix-Loops.
Spaeter optional aufteilen in `executing:impl` / `executing:qa_fix`?

**Entscheidung:** Vorerst ein State mit Metadata-Flag `execution_mode: "impl" | "qa_fix"`. Substatus nur einfuehren wenn UI oder Logik es erfordern.

### OQ-5: Board Sync — Automatisch oder manuell?

Git-Ref-Sync (`git fetch/push refs/mtui/*`) kann:
- (a) Automatisch bei jeder Board-Mutation
- (b) Manuell via "Sync"-Button
- (c) Periodisch (alle 60s)

**Entscheidung:** Start mit (b), dann (c) wenn stabil.

---

## 13. Operational Definitions

Praezise Definitionen fuer Begriffe die in Decision Tables und Heuristiken verwendet werden:

### Critical File

Eine Datei deren Aenderung potenziell weitreichende Auswirkungen hat.

**Default-Liste (konfigurierbar per Projekt):**
- Dependency-Manifeste: `go.mod`, `go.sum`, `package.json`, `package-lock.json`, `Cargo.toml`, `requirements.txt`
- Routing: `**/router.go`, `**/routes.ts`, `**/routes.go`
- Schema: `**/schema.sql`, `**/migrations/**`
- Auth: `**/auth.go`, `**/auth.ts`, `**/middleware/**`
- Config: `.env*`, `**/config.go`, `**/config.ts`
- Build: `Dockerfile`, `docker-compose.yml`, `.github/workflows/**`

**Konfiguration:** `.mtui/config.json` -> `critical_files: []string`

### Shared Surface

Ein logischer Bereich (Package, Modul, API-Boundary) der von mehreren aktiven Cards gleichzeitig beruehrt wird.

**Erkennung:** Zwei Cards aendern Dateien im gleichen Go-Package oder gleichen Frontend-Verzeichnis (`src/components/`, `internal/backend/`).

**Beispiele:** "auth middleware", "config registry", "DB schema", "API router"

### Kleiner Konflikt (fuer AI-Merge)

Ein Git-Merge-Konflikt der automatisch per AI aufgeloest werden darf.

**Muss ALLE Bedingungen erfuellen:**
- Datei ist NICHT in Critical Files Liste
- Max 2 Conflict-Hunks in der Datei
- Jeder Hunk ist max 20 Zeilen
- Max 3 Dateien mit Konflikten in der gesamten Wave
- Kein Konflikt in Import-/Dependency-Bloecken

**Wenn eine Bedingung verletzt:** -> `human_review`

### same_error (StepLoopDetector)

Zwei aufeinanderfolgende Fix-Versuche produzieren semantisch identische Fehler.

**Erkennung:**
1. Fehler-Output normalisieren (Timestamps, tmp-Pfade, Hex-Adressen, volatile Zahlen entfernen)
2. ErrorSignature extrahieren: `{error_class}:{first_error_line_normalized}:{failing_symbol_or_test}`
3. Hash der normalisierten ErrorSignature
4. Vergleich mit vorherigem Hash

**Beispiel:**
```
Versuch 1: "undefined: AppService.HandleAuth" in app.go:142
Versuch 2: "undefined: AppService.HandleAuth" in app.go:155
-> ErrorSignature: "build:undefined_AppService.HandleAuth:app.go"
-> same_error = true (trotz unterschiedlicher Zeilennummer)
```

### scope_exceeded

Ein Step hat mehr Dateien oder Zeilen veraendert als die Scope Limits fuer seinen Card-Typ erlauben.

**Pruefung:** `git diff --stat` im Worktree nach Step-Completion.
**Ausnahmen:** Renames, generated Files, vendor/, dist/, build/, *.min.*
**Aktion:** Review-Flag setzen, Warnung in UI. Kein automatischer Abbruch.

### stuck (State Machine)

Ein Step kann trotz mehrerer Versuche nicht erfolgreich abgeschlossen werden.

**Eintritt in stuck:**
- StepLoopDetector meldet 2+ Signale
- Checkpoint Guard meldet 3 Checks ohne Progress (15min)
- Budget fuer den Step erschoepft
- 3 QA-Fix-Versuche fehlgeschlagen

**Austritt aus stuck:**
- Model-Escalation (max 2x)
- Re-Planning (scope-begrenzt, max 1x)
- -> `human_review` wenn beides fehlschlaegt

### scope_expansion_required (human_review Reason)

Re-Planning hat ergeben, dass der Step nur mit Dateien ausserhalb des Original-Scopes loesbar ist.

**Ausloeser:** Re-Planning-Agent meldet, dass die Loesung Aenderungen an Dateien erfordert die nicht in `files_modify` / `files_create` des Original-Steps stehen.
**Aktion:** Card nach `human_review` mit Kontext: welche zusaetzlichen Dateien benoetigt werden und warum.
**User-Optionen:** Scope erweitern (Plan anpassen) | Task aufteilen | Task abbrechen
