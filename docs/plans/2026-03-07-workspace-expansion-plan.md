# Workspace Expansion Plan: Left Navigation + Kanban + Chat + Multi-Agent

**Datum:** 2026-03-07
**Status:** Draft
**Scope:** Major UI/UX expansion — Left Navigation Pane, Kanban Board, Auto-Planning, Cross-Project Chat, Multi-Agent Orchestration, Scheduled Automation

---

## Übersicht

Multiterminal wird von einem Terminal-Multiplexer zu einer vollwertigen **Workspace-Plattform** erweitert. Die bestehende Sidebar (Explorer/SourceControl/Issues) wird durch eine **Left Navigation Pane** ersetzt, die als Hub für alle Workspace-Funktionen dient. Zusätzlich wird eine **Right Chat Pane** eingeführt für projektübergreifende KI-Konversationen.

```
┌─ Window ──────────────────────────────────────────────────────────┐
│  TabBar                                                           │
│  Toolbar                                                          │
│ ┌─ Left Nav ─┐ ┌─ Main Content ──────────────────┐ ┌─ Chat ────┐│
│ │ Dashboard   │ │                                  │ │ Provider  ││
│ │ Kanban      │ │  Terminal Panes / Dashboard /     │ │ ▼ Claude  ││
│ │ Planung     │ │  Kanban Board / Queue Overview    │ │           ││
│ │ Queue       │ │                                  │ │ Messages  ││
│ │ Chat        │ │                                  │ │ ...       ││
│ │ ─────────── │ │                                  │ │           ││
│ │ Explorer    │ │                                  │ │ Input     ││
│ │ Source Ctrl │ │                                  │ │ [Send]    ││
│ │ Issues      │ │                                  │ └───────────┘│
│ └─────────────┘ └──────────────────────────────────┘              │
│  Footer                                                           │
└───────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Left Navigation Pane

### 1.1 Neue Komponente: `LeftNav.svelte`
- **Zweck:** Ersetzt die bisherige Sidebar-Icon-Leiste als primäre Navigation
- **Breite:** 48px collapsed (Icons only) / 220px expanded
- **Toggle:** Ctrl+B (wie bisher Sidebar)
- **Sections:**
  - **Workspace** (oberer Bereich): Dashboard, Kanban, Planung, Queue, Chat
  - **Projekt** (unterer Bereich): Explorer, Source Control, Issues (bestehende Views)
- **Projekt-Kontext-Switcher:** Dropdown oben im LeftNav
  - "Alle Projekte" — zeigt aggregierte Daten
  - Einzelnes Projekt — gefilterte Ansicht (basierend auf Tab-Verzeichnissen)
  - Projekte werden automatisch aus den offenen Tab-Verzeichnissen erkannt

### 1.2 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/LeftNav.svelte` | NEU | Navigationsleiste mit Icons + Labels |
| `frontend/src/components/Sidebar.svelte` | EDIT | Wird zum "Content Panel" — zeigt die vom LeftNav gewählte View |
| `frontend/src/stores/workspace.ts` | NEU | Store für activeView, projectFilter, collapsed-State |
| `frontend/src/App.svelte` | EDIT | Layout-Integration: LeftNav + Content Area + Chat Pane |

### 1.3 Navigation-Items

```typescript
type NavItem =
  | 'dashboard'      // Übersichts-Dashboard
  | 'kanban'         // Kanban-Board für Issues
  | 'planning'       // Auto-Planung
  | 'queue'          // Queue-Übersicht (alle Sessions)
  | 'chat'           // Konversations-Liste
  | 'explorer'       // Datei-Explorer (bestehend)
  | 'source-control' // Git (bestehend)
  | 'issues';        // Issue-Liste (bestehend)
```

### 1.4 Implementierung

**`LeftNav.svelte`:**
- Vertikale Icon-Leiste (ähnlich VS Code Activity Bar)
- Jedes Item: SVG-Icon + Tooltip (collapsed) / Label (expanded)
- Active-State mit farbiger Seitenleiste
- Badge-Counts: Queue (pending), Issues (open), Chat (unread)
- Projekt-Kontext-Dropdown ganz oben
- Collapse/Expand-Toggle am unteren Rand

**`workspace.ts` Store:**
```typescript
interface WorkspaceState {
  activeView: NavItem;
  projectFilter: string | 'all'; // directory path or 'all'
  leftNavCollapsed: boolean;
  chatPaneVisible: boolean;
}
```

**Layout-Änderung in `App.svelte`:**
```
Bisher:  [Sidebar?] [PaneGrid]
Neu:     [LeftNav] [SideContent?] [PaneGrid] [ChatPane?]
```
- LeftNav ist immer sichtbar (48px collapsed)
- SideContent öffnet sich bei Klick auf ein Nav-Item (wie bisher Sidebar)
- Einige Views (Dashboard, Kanban) ersetzen den PaneGrid-Bereich statt als Sidebar zu öffnen

---

## Phase 2: Dashboard View

### 2.1 Neue Komponente: `DashboardView.svelte` (Erweiterung)
- Bestehende `DashboardView.svelte` wird erweitert
- **Projekt-Karten** mit Status-Ampel pro Projekt
- **Aggregierte Metriken:** Gesamtkosten, aktive Sessions, offene Issues, Queue-Tiefe
- **Schnellaktionen:** Neues Terminal, Issue erstellen, Queue leeren
- **Letzte Aktivitäten:** Timeline der letzten Session-Events

### 2.2 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/DashboardView.svelte` | EDIT | Erweitern um Projekt-Karten + Metriken |
| `internal/backend/app_dashboard.go` | NEU | Aggregierte Statistiken (Kosten, Sessions, Issues pro Projekt) |
| `frontend/wailsjs/go/models.ts` | EDIT | DashboardStats-Klasse hinzufügen |

### 2.3 Dashboard-Datenstruktur

```go
// app_dashboard.go
type DashboardStats struct {
    Projects      []ProjectStats `json:"projects" yaml:"projects"`
    TotalCost     string         `json:"total_cost" yaml:"total_cost"`
    TotalSessions int            `json:"total_sessions" yaml:"total_sessions"`
    TotalIssues   int            `json:"total_issues" yaml:"total_issues"`
}

type ProjectStats struct {
    Dir            string `json:"dir" yaml:"dir"`
    Name           string `json:"name" yaml:"name"`
    ActiveSessions int    `json:"active_sessions" yaml:"active_sessions"`
    OpenIssues     int    `json:"open_issues" yaml:"open_issues"`
    TotalCost      string `json:"total_cost" yaml:"total_cost"`
    Branch         string `json:"branch" yaml:"branch"`
    QueueDepth     int    `json:"queue_depth" yaml:"queue_depth"`
}
```

---

## Phase 3: Kanban Board

### 3.1 Neue Komponenten

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/KanbanBoard.svelte` | NEU | Hauptkomponente: Spalten + Karten |
| `frontend/src/components/KanbanColumn.svelte` | NEU | Einzelne Spalte (Backlog/Todo/InProgress/Done) |
| `frontend/src/components/KanbanCard.svelte` | NEU | Issue-Karte mit Drag-Handle |
| `frontend/src/stores/kanban.ts` | NEU | Kanban-State, Drag&Drop-Logik |
| `internal/backend/app_kanban.go` | NEU | Kanban-State-Persistenz + Issue-Mapping |
| `internal/config/kanban.go` | NEU | Kanban-State in JSON (~/.multiterminal-kanban.json) |

### 3.2 Kanban-Architektur

**Spalten (fest):**
1. **Backlog** — Unpriorisierte Issues
2. **Geplant** — Priorisiert, noch nicht begonnen
3. **In Arbeit** — Aktiv bearbeitet (mit verknüpfter Session)
4. **Review** — PR erstellt, wartet auf Review
5. **Erledigt** — Geschlossen

**Karten-Datenstruktur:**
```go
// app_kanban.go
type KanbanState struct {
    Columns map[string][]KanbanCard `json:"columns" yaml:"columns"`
}

type KanbanCard struct {
    IssueNumber   int      `json:"issue_number" yaml:"issue_number"`
    Title         string   `json:"title" yaml:"title"`
    Labels        []string `json:"labels" yaml:"labels"`
    Dir           string   `json:"dir" yaml:"dir"`           // Projekt-Verzeichnis
    SessionID     int      `json:"session_id" yaml:"session_id"` // 0 = keine Session
    Priority      int      `json:"priority" yaml:"priority"`     // Sortierung innerhalb Spalte
    Dependencies  []int    `json:"dependencies" yaml:"dependencies"` // Issue-Nummern
}
```

**Automatische Spalten-Zuordnung:**
- Neue Issues → Backlog
- Issue mit verknüpfter Session → In Arbeit (automatisch)
- Session fertig + PR erkannt → Review (automatisch)
- Issue geschlossen → Erledigt (automatisch)

**Drag & Drop:**
- Native HTML5 Drag & Drop (kein Framework)
- Karte zwischen Spalten ziehen
- Sortierung innerhalb Spalte
- Karten-Klick → Issue-Detail + Aktionen (Session starten, Branch erstellen)

**Backend-Methoden:**
```go
func (a *AppService) GetKanbanState(dir string) KanbanState
func (a *AppService) MoveKanbanCard(issueNumber int, toColumn string, position int) error
func (a *AppService) SyncKanbanWithIssues(dir string) KanbanState // GitHub-Sync
func (a *AppService) GetKanbanForAllProjects() map[string]KanbanState
```

### 3.3 Integration mit bestehenden Issues
- `SyncKanbanWithIssues()` holt Issues via `gh` CLI und mappt auf Spalten
- Labels können als Spalten-Hints dienen (z.B. Label "in-progress" → In Arbeit)
- Bestehende IssuesView im LeftNav zeigt weiterhin die Listenansicht
- Kanban ist die visuelle Alternative mit Drag&Drop

---

## Phase 4: Auto-Planung

### 4.1 Neue Komponenten

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/PlanningView.svelte` | NEU | Auto-Planungs-UI |
| `frontend/src/components/PlanningCard.svelte` | NEU | Einzelner Planungsvorschlag |
| `internal/backend/app_planning.go` | NEU | Issue-Analyse + Planungsgenerierung |

### 4.2 Funktionsweise

**Auto-Planung analysiert offene Issues und erstellt Ausführungspläne:**

1. **Issue-Analyse:** Liest Issue-Body, Labels, Abhängigkeiten
2. **Vorschlag generiert:** Reihenfolge, geschätzte Komplexität, empfohlene Parallelisierung
3. **Benutzer bestätigt:** Plan kann angepasst werden
4. **Ausführung:** Issues werden als Queue-Items in die richtige Reihenfolge gebracht

**Planungsstruktur:**
```go
type Plan struct {
    ID        string      `json:"id" yaml:"id"`
    Dir       string      `json:"dir" yaml:"dir"`
    CreatedAt string      `json:"created_at" yaml:"created_at"`
    Steps     []PlanStep  `json:"steps" yaml:"steps"`
    Status    string      `json:"status" yaml:"status"` // draft/approved/running/done
}

type PlanStep struct {
    IssueNumber int    `json:"issue_number" yaml:"issue_number"`
    Title       string `json:"title" yaml:"title"`
    Order       int    `json:"order" yaml:"order"`
    Parallel    bool   `json:"parallel" yaml:"parallel"` // kann parallel laufen
    SessionID   int    `json:"session_id" yaml:"session_id"`
    Status      string `json:"status" yaml:"status"` // pending/running/done/skipped
    Prompt      string `json:"prompt" yaml:"prompt"` // generierter Prompt für Claude
}
```

**Backend-Methoden:**
```go
func (a *AppService) GeneratePlan(dir string, issueNumbers []int) (*Plan, error)
func (a *AppService) ApprovePlan(planID string) error
func (a *AppService) ExecutePlan(planID string) error // startet Sessions + Queue
func (a *AppService) GetPlans(dir string) []Plan
func (a *AppService) CancelPlan(planID string) error
```

**UI (PlanningView.svelte):**
- Liste offener Issues mit Checkboxen zur Auswahl
- "Plan erstellen" Button → ruft `GeneratePlan()` auf
- Drag&Drop-Sortierung der Schritte
- Parallel-Toggle pro Schritt
- "Ausführen" Button → startet Sessions automatisch
- Fortschrittsanzeige während Ausführung

---

## Phase 5: Queue-Übersicht (Cross-Project)

### 5.1 Neue Komponente: `QueueOverview.svelte`

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/QueueOverview.svelte` | NEU | Projektübergreifende Queue-Ansicht |
| `internal/backend/app_queue.go` | EDIT | `GetAllQueues()` Methode hinzufügen |

### 5.2 Funktionalität

- Zeigt alle Queue-Items über alle Sessions/Projekte hinweg
- Gruppiert nach Projekt oder Session
- Status-Filter (pending/sent/done)
- Drag&Drop-Priorisierung
- Bulk-Aktionen (alle pending löschen, Queue pausieren)

**Neue Backend-Methode:**
```go
type QueueOverviewItem struct {
    SessionID   int       `json:"session_id" yaml:"session_id"`
    SessionName string    `json:"session_name" yaml:"session_name"`
    Dir         string    `json:"dir" yaml:"dir"`
    Items       []QueueItem `json:"items" yaml:"items"`
}

func (a *AppService) GetAllQueues() []QueueOverviewItem
```

---

## Phase 6: Chat Pane (Right Side)

### 6.1 Neue Komponenten

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/ChatPane.svelte` | NEU | Chat-Container (rechte Seite) |
| `frontend/src/components/ChatMessage.svelte` | NEU | Einzelne Nachricht (User/Assistant) |
| `frontend/src/components/ChatInput.svelte` | NEU | Eingabefeld mit Provider-Auswahl |
| `frontend/src/components/ChatList.svelte` | NEU | Konversations-Liste (im LeftNav Content) |
| `frontend/src/stores/chat.ts` | NEU | Chat-State, Nachrichten, Konversationen |
| `internal/backend/app_chat.go` | NEU | Chat-Backend (CLI-Aufruf, Streaming) |
| `internal/backend/app_chat_stream.go` | NEU | Stream-JSON-Parsing für Chat-Responses |
| `internal/config/chat.go` | NEU | Chat-Persistenz (SQLite oder JSON) |

### 6.2 Chat-Architektur

**Konversationen:**
```go
type Conversation struct {
    ID        string    `json:"id" yaml:"id"`
    Title     string    `json:"title" yaml:"title"`
    Provider  string    `json:"provider" yaml:"provider"`  // claude/codex/gemini
    Model     string    `json:"model" yaml:"model"`
    Scope     string    `json:"scope" yaml:"scope"`        // "all" oder Projekt-Pfad
    CreatedAt string    `json:"created_at" yaml:"created_at"`
    UpdatedAt string    `json:"updated_at" yaml:"updated_at"`
    Messages  []ChatMessage `json:"messages" yaml:"messages"`
}

type ChatMessage struct {
    ID        string `json:"id" yaml:"id"`
    Role      string `json:"role" yaml:"role"`       // user/assistant
    Content   string `json:"content" yaml:"content"`
    Timestamp string `json:"timestamp" yaml:"timestamp"`
    Cost      string `json:"cost" yaml:"cost"`
    Tokens    int    `json:"tokens" yaml:"tokens"`
}
```

**Provider-Integration:**
- Nutzt die bestehende CLI-Erkennung (`app_claude_detect.go` Pattern)
- Startet `claude --output-format stream-json --print` für nicht-interaktive Antworten
- Parsed Stream-JSON für progressive Anzeige
- Unterstützt Claude, Codex, Gemini (wie bei Terminal-Sessions)

**Scope-Konzept:**
- **Projekt-Scope:** Chat kennt das Arbeitsverzeichnis, kann auf Dateien verweisen
- **All-Scope:** Kein spezifisches Verzeichnis, allgemeine Fragen
- Scope wird bei Konversations-Erstellung gewählt

**Backend-Methoden:**
```go
func (a *AppService) CreateConversation(provider, model, scope string) (*Conversation, error)
func (a *AppService) SendChatMessage(convID, content string) error // streamt via Event
func (a *AppService) GetConversations() []Conversation
func (a *AppService) GetConversation(id string) (*Conversation, error)
func (a *AppService) DeleteConversation(id string) error
```

**Events:**
```
chat:message    → { conversationId, message: ChatMessage }
chat:stream     → { conversationId, delta: string }        // progressive Anzeige
chat:done       → { conversationId, totalCost: string }
chat:error      → { conversationId, error: string }
```

### 6.3 Chat UI

**ChatPane.svelte (Right Side):**
- Breite: 350px (resizable)
- Toggle: Ctrl+Shift+C oder Klick auf "Chat" im LeftNav
- Header: Konversations-Titel + Provider-Badge + Scope-Badge
- Message-Liste: Scrollbar, Markdown-Rendering
- Input: Textarea + Send-Button (Enter = senden, Shift+Enter = Newline)
- Provider-Selector im Header (wechselbar pro Konversation)

**ChatList.svelte (LeftNav Content View):**
- Liste aller Konversationen, sortiert nach UpdatedAt
- Filter: Provider, Scope
- "Neue Konversation" Button mit Provider + Scope Auswahl
- Klick auf Konversation → öffnet ChatPane rechts + zeigt Verlauf

**ChatMessage.svelte:**
- User-Nachrichten: rechts-ausgerichtet, themed
- Assistant-Nachrichten: links-ausgerichtet, mit Provider-Icon
- Markdown-Rendering (Code-Blöcke, Listen, etc.)
- Kostenanzeige pro Nachricht
- Copy-Button für Code-Blöcke

### 6.4 Provider-Auswahl bei neuer Konversation

```
┌─ Neue Konversation ──────────────┐
│                                   │
│  Anbieter:                        │
│  ○ Claude  ○ Codex  ○ Gemini     │
│                                   │
│  Modell:                          │
│  [Dropdown aus config.models]     │
│                                   │
│  Kontext:                         │
│  ○ Alle Projekte                  │
│  ○ Projekt: /path/to/project      │
│     [Dropdown aus offenen Tabs]   │
│                                   │
│  [Erstellen]  [Abbrechen]         │
└───────────────────────────────────┘
```

---

## Phase 7: Multi-Agent Orchestration

### 7.1 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `internal/backend/app_orchestrator.go` | NEU | Multi-Agent-Koordination |
| `internal/backend/app_orchestrator_worker.go` | NEU | Worker-Management |
| `frontend/src/components/OrchestratorPanel.svelte` | NEU | Orchestrator-UI im Dashboard |

### 7.2 Konzept

**Multi-Agent = mehrere Claude-Sessions die koordiniert an einem Plan arbeiten:**

- Nutzt das bestehende Session-System (`CreateSession`)
- Orchestrator ist eine Schicht über dem Planungs-System (Phase 4)
- Startet parallele Sessions für unabhängige Plan-Schritte
- Überwacht Fortschritt via `terminal:activity` Events
- Startet nächsten Schritt wenn Vorgänger fertig

**Konfiguration:**
```go
type OrchestratorConfig struct {
    MaxParallelAgents int  `json:"max_parallel_agents" yaml:"max_parallel_agents"` // default 3
    AutoStartNext     bool `json:"auto_start_next" yaml:"auto_start_next"`         // default true
}
```

**Ablauf:**
1. Plan wird genehmigt (Phase 4)
2. Orchestrator startet erste(n) Schritt(e)
3. Jeder Schritt = 1 Claude-Session mit generiertem Prompt
4. Bei `activity: done` → nächsten Schritt starten
5. Bei `activity: waitingPermission` → Benachrichtigung an User
6. Dashboard zeigt Fortschritt als Gantt-ähnliche Ansicht

### 7.3 Abhängigkeiten zwischen Phasen

```
Phase 4 (Auto-Planung) ──────► Phase 7 (Multi-Agent)
                                  │
Phase 3 (Kanban) ────────────► Karten zeigen Agent-Status
                                  │
Phase 5 (Queue) ─────────────► Queue-Items werden automatisch erstellt
```

---

## Phase 8: Scheduled Automation

### 8.1 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `internal/backend/app_scheduler.go` | NEU | Cron-artiger Scheduler |
| `internal/config/scheduler.go` | NEU | Schedule-Persistenz |
| `frontend/src/components/SchedulerView.svelte` | NEU | Schedule-Verwaltung |

### 8.2 Konzept

**Wiederkehrende Aufgaben die automatisch als Claude-Sessions gestartet werden:**

```go
type ScheduledTask struct {
    ID        string `json:"id" yaml:"id"`
    Name      string `json:"name" yaml:"name"`
    Dir       string `json:"dir" yaml:"dir"`
    Prompt    string `json:"prompt" yaml:"prompt"`
    Schedule  string `json:"schedule" yaml:"schedule"` // "hourly", "daily", "weekly", cron
    Mode      string `json:"mode" yaml:"mode"`         // claude, claude-yolo
    Model     string `json:"model" yaml:"model"`
    Enabled   bool   `json:"enabled" yaml:"enabled"`
    LastRun   string `json:"last_run" yaml:"last_run"`
    NextRun   string `json:"next_run" yaml:"next_run"`
}
```

**Anwendungsfälle:**
- Täglicher Code-Review aller offenen PRs
- Stündliche Dependency-Check
- Wöchentlicher Security-Scan
- Nach jedem Commit: Tests laufen lassen

**Backend:**
```go
func (a *AppService) CreateSchedule(task ScheduledTask) (*ScheduledTask, error)
func (a *AppService) GetSchedules() []ScheduledTask
func (a *AppService) UpdateSchedule(task ScheduledTask) error
func (a *AppService) DeleteSchedule(id string) error
func (a *AppService) ToggleSchedule(id string) error
// Internal: scheduler goroutine checks every minute
```

---

## Implementierungs-Reihenfolge

```
Phase 1: Left Navigation Pane          ████░░░░░░  Basis-Infrastruktur
Phase 2: Dashboard (erweitert)          ██░░░░░░░░  Schneller Gewinn
Phase 3: Kanban Board                   ████████░░  Kernfeature
Phase 4: Auto-Planung                   ██████░░░░  Aufbauend auf Kanban
Phase 5: Queue-Übersicht                ██░░░░░░░░  Erweitert bestehend
Phase 6: Chat Pane                      ████████░░  Unabhängig, parallel möglich
Phase 7: Multi-Agent Orchestration      ██████░░░░  Aufbauend auf Planung
Phase 8: Scheduled Automation           ████░░░░░░  Unabhängig
```

**Abhängigkeiten:**
- Phase 1 → alle anderen (Layout-Basis)
- Phase 3 → Phase 4 → Phase 7 (aufeinander aufbauend)
- Phase 2, 5, 6, 8 sind untereinander unabhängig

**Empfohlene Umsetzung:**
1. **Sprint 1:** Phase 1 (LeftNav) + Phase 2 (Dashboard)
2. **Sprint 2:** Phase 3 (Kanban) + Phase 6 (Chat) parallel
3. **Sprint 3:** Phase 4 (Planung) + Phase 5 (Queue)
4. **Sprint 4:** Phase 7 (Multi-Agent) + Phase 8 (Scheduler)

---

## Neue Dateien (Gesamt)

### Frontend (14 neue Dateien)
```
frontend/src/components/LeftNav.svelte
frontend/src/components/KanbanBoard.svelte
frontend/src/components/KanbanColumn.svelte
frontend/src/components/KanbanCard.svelte
frontend/src/components/PlanningView.svelte
frontend/src/components/PlanningCard.svelte
frontend/src/components/QueueOverview.svelte
frontend/src/components/ChatPane.svelte
frontend/src/components/ChatMessage.svelte
frontend/src/components/ChatInput.svelte
frontend/src/components/ChatList.svelte
frontend/src/components/OrchestratorPanel.svelte
frontend/src/components/SchedulerView.svelte
frontend/src/stores/workspace.ts
frontend/src/stores/kanban.ts
frontend/src/stores/chat.ts
```

### Backend (10 neue Dateien)
```
internal/backend/app_dashboard.go
internal/backend/app_kanban.go
internal/backend/app_planning.go
internal/backend/app_chat.go
internal/backend/app_chat_stream.go
internal/backend/app_orchestrator.go
internal/backend/app_orchestrator_worker.go
internal/backend/app_scheduler.go
internal/config/kanban.go
internal/config/chat.go
internal/config/scheduler.go
```

### Geänderte Dateien (6)
```
frontend/src/App.svelte                  — Layout-Umbau
frontend/src/components/Sidebar.svelte   — Wird Content-Panel
frontend/src/stores/config.ts            — Neue Config-Felder
frontend/wailsjs/go/models.ts           — Neue Klassen
internal/config/config.go               — Neue Config-Sections
internal/backend/app_queue.go           — GetAllQueues()
```

---

## Config-Erweiterungen

```yaml
# ~/.multiterminal.yaml (neue Sections)
workspace:
  left_nav_collapsed: false
  default_view: dashboard

orchestrator:
  max_parallel_agents: 3
  auto_start_next: true

scheduler:
  enabled: true

chat:
  default_provider: claude
  default_model: ""
  history_limit: 100  # max conversations to keep
```

---

## Technische Hinweise

1. **300-Zeilen-Limit:** Alle Go-Dateien unter 300 Zeilen halten (CLAUDE.md Regel)
2. **models.ts Sync:** Jede neue Go-Struct braucht manuelle models.ts-Klasse
3. **yaml+json Tags:** Alle Structs brauchen beide Tags
4. **UI-Text Deutsch:** Alle Labels, Tooltips, Dialoge auf Deutsch
5. **Kein externer Web-Zugriff:** Chat nutzt lokale CLI-Tools, kein API-Direktzugriff
6. **Svelte Reactive-Regel:** Keine Variable-Zuweisungen in `$:` Blöcken
7. **Concurrency:** Neue Mutexe für Kanban-State, Chat-State, Scheduler-State
8. **Persistenz:** Kanban + Chat in separaten JSON-Dateien (nicht in config.yaml)
