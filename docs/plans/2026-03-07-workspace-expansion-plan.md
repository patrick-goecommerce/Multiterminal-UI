# Workspace Expansion Plan: Left Navigation + Kanban + Chat + Multi-Agent

**Datum:** 2026-03-07
**Status:** Draft v2 (überarbeitet)
**Scope:** Major UI/UX expansion — Left Navigation Pane, Kanban Board (mit integrierter Planung & Automation), Chat View, Multi-Agent Orchestration, Ask-User Bridging

---

## Übersicht

Multiterminal wird von einem Terminal-Multiplexer zu einer vollwertigen **Workspace-Plattform** erweitert. Die bestehende Sidebar (Explorer/SourceControl/Issues) wird durch eine **Left Navigation Pane** ersetzt, die als Hub für alle Workspace-Funktionen dient. Chat wird als **eigene Main-Content-View** integriert (nicht als rechte Sidebar), um den Terminal-Panes keinen Platz zu stehlen.

**Kernänderungen gegenüber v1:**
- Planning + Scheduler in Kanban integriert (weniger Views, weniger Komplexität)
- Chat als Main-Content-View statt rechte Sidebar
- ask_user Bridging als neues Feature aufgenommen
- "Alle Projekte" nur im Dashboard, nicht überall
- LeftNav reduziert auf 4 Workspace-Views statt 6

```
┌─ Window ──────────────────────────────────────────────────────┐
│  TabBar                                                        │
│  Toolbar                                                       │
│ ┌─ Left Nav ─┐ ┌─ Main Content ─────────────────────────────┐ │
│ │ Dashboard   │ │                                             │ │
│ │ Kanban      │ │  Terminal Panes / Dashboard / Kanban Board  │ │
│ │ Chat        │ │  / Chat View / Queue Overview               │ │
│ │ Queue       │ │                                             │ │
│ │ ─────────── │ │                                             │ │
│ │ Explorer    │ │                                             │ │
│ │ Source Ctrl │ │                                             │ │
│ │ Issues      │ │                                             │ │
│ └─────────────┘ └─────────────────────────────────────────────┘ │
│  Footer                                                        │
└────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Left Navigation Pane

### 1.1 Neue Komponente: `LeftNav.svelte`
- **Zweck:** Ersetzt die bisherige Sidebar-Icon-Leiste als primäre Navigation
- **Breite:** 48px collapsed (Icons only) / 220px expanded
- **Toggle:** Ctrl+B (wie bisher Sidebar)
- **Sections:**
  - **Workspace** (oberer Bereich): Dashboard, Kanban, Chat, Queue
  - **Projekt** (unterer Bereich): Explorer, Source Control, Issues (bestehende Views)
- **Kein globaler Projekt-Switcher.** Projekt-Kontext ergibt sich aus dem aktiven Tab-Verzeichnis. Nur das Dashboard zeigt projektübergreifende Daten.

### 1.2 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/LeftNav.svelte` | NEU | Navigationsleiste mit Icons + Labels |
| `frontend/src/components/Sidebar.svelte` | EDIT | Wird zum "Content Panel" — zeigt die vom LeftNav gewählte View |
| `frontend/src/stores/workspace.ts` | NEU | Store für activeView, collapsed-State |
| `frontend/src/App.svelte` | EDIT | Layout-Integration: LeftNav ersetzt alte Sidebar-Icons |

### 1.3 Navigation-Items

```typescript
type NavItem =
  | 'terminals'      // Standard-Ansicht: Terminal Panes (default)
  | 'dashboard'      // Übersichts-Dashboard (einzige projektübergreifende View)
  | 'kanban'         // Kanban-Board mit Planung + Automation
  | 'chat'           // Konversations-View
  | 'queue'          // Queue-Übersicht (aktives Projekt)
  | 'explorer'       // Datei-Explorer (bestehend, öffnet Sidebar)
  | 'source-control' // Git (bestehend, öffnet Sidebar)
  | 'issues';        // Issue-Liste (bestehend, öffnet Sidebar)
```

### 1.4 View-Routing

**Zwei Arten von Navigation:**
- **Main-Content-Views** (`terminals`, `dashboard`, `kanban`, `chat`, `queue`): Ersetzen den PaneGrid-Bereich komplett
- **Sidebar-Views** (`explorer`, `source-control`, `issues`): Öffnen als Sidebar neben dem PaneGrid (wie bisher)

Das bedeutet: Klick auf "Explorer" wechselt zurück zu Terminals + öffnet die Sidebar. Klick auf "Kanban" zeigt das Kanban-Board im Hauptbereich.

### 1.5 Implementierung

**`LeftNav.svelte`:**
- Vertikale Icon-Leiste (ähnlich VS Code Activity Bar)
- Jedes Item: SVG-Icon + Tooltip (collapsed) / Label (expanded)
- Active-State mit farbiger Seitenleiste
- Badge-Counts: Queue (pending), Issues (open), Chat (unread)
- Collapse/Expand-Toggle am unteren Rand

**`workspace.ts` Store:**
```typescript
interface WorkspaceState {
  activeView: NavItem;         // welche Main-Content-View ist aktiv
  leftNavCollapsed: boolean;   // Icons-only Modus
  sidebarView: string | null;  // welche Sidebar-View ist offen (null = zu)
}
```

**Layout-Logik in `App.svelte`:**
```svelte
{#if activeView === 'terminals'}
  <!-- Bestehend: Sidebar + PaneGrid -->
{:else if activeView === 'dashboard'}
  <DashboardView />
{:else if activeView === 'kanban'}
  <KanbanBoard dir={currentDir} />
{:else if activeView === 'chat'}
  <ChatView />
{:else if activeView === 'queue'}
  <QueueOverview />
{/if}
```

---

## Phase 2: Dashboard View

### 2.1 Komponente: `DashboardView.svelte` (Erweiterung)
- Bestehende `DashboardView.svelte` wird erweitert
- **Einzige projektübergreifende View** — zeigt alle Projekte
- **Projekt-Karten** mit Status-Ampel pro Projekt (aus offenen Tab-Verzeichnissen)
- **Aggregierte Metriken:** Gesamtkosten, aktive Sessions, offene Issues, Queue-Tiefe
- **Schnellaktionen:** Neues Terminal, Issue erstellen, zum Projekt wechseln
- **Agent-Fortschritt:** Laufende Orchestrator-Pläne mit Fortschrittsbalken

### 2.2 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/DashboardView.svelte` | EDIT | Erweitern um Projekt-Karten + Metriken |
| `internal/backend/app_dashboard.go` | NEU | Aggregierte Statistiken pro Projekt |
| `frontend/wailsjs/go/models.ts` | EDIT | DashboardStats-Klasse hinzufügen |

### 2.3 Dashboard-Datenstruktur

```go
// app_dashboard.go
type DashboardStats struct {
    Projects      []ProjectStats `json:"projects" yaml:"projects"`
    TotalCost     string         `json:"total_cost" yaml:"total_cost"`
    TotalSessions int            `json:"total_sessions" yaml:"total_sessions"`
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

## Phase 3: Kanban Board (mit Planung + Automation)

Das Kanban-Board ist die zentrale Planungs- und Automatisierungsansicht. Statt separater Planning- und Scheduler-Views werden diese Funktionen direkt in das Kanban integriert.

### 3.1 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/KanbanBoard.svelte` | NEU | Hauptkomponente: Spalten + Karten + Toolbar |
| `frontend/src/components/KanbanColumn.svelte` | NEU | Einzelne Spalte mit Drop-Zone |
| `frontend/src/components/KanbanCard.svelte` | NEU | Issue-Karte mit Status, Agent-Indicator, Schedule-Badge |
| `frontend/src/stores/kanban.ts` | NEU | Kanban-State, Drag&Drop, Planung, Schedules |
| `internal/backend/app_kanban.go` | NEU | Kanban-State + Issue-Sync + Planung |
| `internal/backend/app_kanban_plan.go` | NEU | Auto-Planung: Reihenfolge + Parallelisierung |
| `internal/backend/app_kanban_schedule.go` | NEU | Wiederkehrende Aufgaben (Scheduler) |
| `internal/config/kanban.go` | NEU | Persistenz (~/.multiterminal-kanban.json) |

### 3.2 Kanban-Spalten

| Spalte | Automatik | Beschreibung |
|--------|-----------|--------------|
| **Backlog** | Neue Issues landen hier | Unpriorisiert |
| **Geplant** | Manuell oder via Auto-Plan | Priorisiert, Reihenfolge festgelegt |
| **In Arbeit** | Session verknüpft → auto-move | Agent arbeitet aktiv |
| **Review** | PR erkannt → auto-move | Wartet auf Review |
| **Erledigt** | Issue geschlossen → auto-move | Abgeschlossen |

### 3.3 Karten-Datenstruktur

```go
type KanbanState struct {
    Columns   map[string][]KanbanCard `json:"columns" yaml:"columns"`
    Plans     []Plan                  `json:"plans" yaml:"plans"`
    Schedules []ScheduledTask         `json:"schedules" yaml:"schedules"`
}

type KanbanCard struct {
    IssueNumber  int      `json:"issue_number" yaml:"issue_number"`
    Title        string   `json:"title" yaml:"title"`
    Labels       []string `json:"labels" yaml:"labels"`
    Dir          string   `json:"dir" yaml:"dir"`
    SessionID    int      `json:"session_id" yaml:"session_id"`
    Priority     int      `json:"priority" yaml:"priority"`
    Dependencies []int    `json:"dependencies" yaml:"dependencies"`
    PlanID       string   `json:"plan_id" yaml:"plan_id"`       // gehört zu welchem Plan
    ScheduleID   string   `json:"schedule_id" yaml:"schedule_id"` // wiederkehrend?
}
```

### 3.4 Integrierte Planung (ersetzt separate Phase 4)

**Toolbar im Kanban-Board:**
- **"Auto-Plan" Button:** Wählt Issues aus Backlog, schlägt Reihenfolge + Parallelisierung vor
- **Plan-Ansicht:** Toggle zwischen Kanban-Ansicht und Plan-Ansicht (Gantt-artige Reihenfolge)
- **"Ausführen" Button:** Startet den Orchestrator für den aktiven Plan

```go
type Plan struct {
    ID        string     `json:"id" yaml:"id"`
    Dir       string     `json:"dir" yaml:"dir"`
    CreatedAt string     `json:"created_at" yaml:"created_at"`
    Steps     []PlanStep `json:"steps" yaml:"steps"`
    Status    string     `json:"status" yaml:"status"` // draft/approved/running/done
}

type PlanStep struct {
    IssueNumber int    `json:"issue_number" yaml:"issue_number"`
    Title       string `json:"title" yaml:"title"`
    Order       int    `json:"order" yaml:"order"`
    Parallel    bool   `json:"parallel" yaml:"parallel"`
    SessionID   int    `json:"session_id" yaml:"session_id"`
    Status      string `json:"status" yaml:"status"` // pending/running/done/skipped
    Prompt      string `json:"prompt" yaml:"prompt"`
}
```

**UI-Flow:**
1. User klickt "Auto-Plan" → Issues aus Backlog werden analysiert
2. Vorschlag erscheint als sortierte Liste mit Parallel-Markierungen
3. User kann Drag&Drop die Reihenfolge ändern, Parallel-Toggles setzen
4. "Genehmigen" → Karten wandern in "Geplant"-Spalte in der richtigen Reihenfolge
5. "Ausführen" → Orchestrator startet Sessions (→ Phase 5)

### 3.5 Integrierte Automation (ersetzt separate Phase 8)

**Schedule-Tab im Kanban-Board:**
- Zweiter Tab neben der Kanban-Ansicht: "Board" | "Zeitpläne"
- Wiederkehrende Aufgaben konfigurieren, die automatisch Karten/Sessions erzeugen

```go
type ScheduledTask struct {
    ID       string `json:"id" yaml:"id"`
    Name     string `json:"name" yaml:"name"`
    Dir      string `json:"dir" yaml:"dir"`
    Prompt   string `json:"prompt" yaml:"prompt"`
    Schedule string `json:"schedule" yaml:"schedule"` // hourly/daily/weekly/cron
    Mode     string `json:"mode" yaml:"mode"`         // claude/claude-yolo
    Model    string `json:"model" yaml:"model"`
    Enabled  bool   `json:"enabled" yaml:"enabled"`
    LastRun  string `json:"last_run" yaml:"last_run"`
    NextRun  string `json:"next_run" yaml:"next_run"`
}
```

**Anwendungsfälle:**
- Täglicher Code-Review aller offenen PRs
- Stündliche Dependency-Check
- Wöchentlicher Security-Scan

**Backend:**
```go
func (a *AppService) GetKanbanState(dir string) KanbanState
func (a *AppService) MoveKanbanCard(issueNumber int, toColumn string, position int) error
func (a *AppService) SyncKanbanWithIssues(dir string) KanbanState
func (a *AppService) GeneratePlan(dir string, issueNumbers []int) (*Plan, error)
func (a *AppService) ApprovePlan(planID string) error
func (a *AppService) ExecutePlan(planID string) error
func (a *AppService) GetPlans(dir string) []Plan
func (a *AppService) CancelPlan(planID string) error
func (a *AppService) CreateSchedule(task ScheduledTask) (*ScheduledTask, error)
func (a *AppService) GetSchedules(dir string) []ScheduledTask
func (a *AppService) UpdateSchedule(task ScheduledTask) error
func (a *AppService) DeleteSchedule(id string) error
func (a *AppService) ToggleSchedule(id string) error
```

### 3.6 Drag & Drop
- Native HTML5 Drag & Drop (kein Framework)
- Karte zwischen Spalten ziehen
- Sortierung innerhalb Spalte
- Karten-Klick → Issue-Detail-Popup mit Aktionen (Session starten, Branch erstellen)
- Visuelle Indikatoren: Agent-Status-Dot, Schedule-Badge, Dependency-Linien

---

## Phase 4: Chat View (Main Content)

Chat wird als **eigene Main-Content-View** im Hauptbereich angezeigt — nicht als rechte Sidebar. So bleibt der volle Platz für Terminal-Panes erhalten.

### 4.1 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/ChatView.svelte` | NEU | Chat-Hauptansicht (Konversationsliste + aktiver Chat) |
| `frontend/src/components/ChatMessage.svelte` | NEU | Einzelne Nachricht (User/Assistant) mit Markdown |
| `frontend/src/components/ChatInput.svelte` | NEU | Eingabefeld mit Provider-Auswahl |
| `frontend/src/stores/chat.ts` | NEU | Chat-State, Nachrichten, Konversationen |
| `internal/backend/app_chat.go` | NEU | Chat-Backend (CLI-Aufruf, Streaming) |
| `internal/backend/app_chat_stream.go` | NEU | Stream-JSON-Parsing für Chat-Responses |
| `internal/config/chat.go` | NEU | Chat-Persistenz (JSON-Dateien) |

### 4.2 Layout

```
┌─ ChatView ──────────────────────────────────────────────────┐
│ ┌─ Konversationen ─┐ ┌─ Aktiver Chat ────────────────────┐ │
│ │ [+ Neu]           │ │ Header: Titel | Provider | Scope  │ │
│ │                   │ │                                    │ │
│ │ ● Projekt-Review  │ │ ┌─ Assistant ──────────────────┐  │ │
│ │   Claude · 14:30  │ │ │ Hier ist meine Analyse...    │  │ │
│ │                   │ │ └──────────────────────────────┘  │ │
│ │ ● Architektur     │ │                                    │ │
│ │   Gemini · 13:15  │ │ ┌─ User ───────────────────────┐  │ │
│ │                   │ │ │ Kannst du das genauer...      │  │ │
│ │ ● Debug-Hilfe     │ │ └──────────────────────────────┘  │ │
│ │   Claude · 12:00  │ │                                    │ │
│ │                   │ │ ┌──────────────────────────────┐  │ │
│ │                   │ │ │ Nachricht eingeben...    [↵] │  │ │
│ └───────────────────┘ └────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

### 4.3 Chat-Architektur

**Konversationen:**
```go
type Conversation struct {
    ID        string        `json:"id" yaml:"id"`
    Title     string        `json:"title" yaml:"title"`
    Provider  string        `json:"provider" yaml:"provider"`  // claude/codex/gemini
    Model     string        `json:"model" yaml:"model"`
    Scope     string        `json:"scope" yaml:"scope"`        // Projekt-Pfad (immer ein Projekt)
    CreatedAt string        `json:"created_at" yaml:"created_at"`
    UpdatedAt string        `json:"updated_at" yaml:"updated_at"`
    Messages  []ChatMessage `json:"messages" yaml:"messages"`
}

type ChatMessage struct {
    ID        string `json:"id" yaml:"id"`
    Role      string `json:"role" yaml:"role"`       // user/assistant/ask_user
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

**Scope:** Immer ein konkretes Projekt (Verzeichnis aus den offenen Tabs). Kein "Alle Projekte"-Scope — das wäre technisch unsauber, da CLI-Tools ein Arbeitsverzeichnis brauchen.

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
chat:stream     → { conversationId, delta: string }        // progressive Anzeige
chat:done       → { conversationId, message: ChatMessage }  // fertige Nachricht
chat:error      → { conversationId, error: string }
```

### 4.4 Provider-Auswahl bei neuer Konversation

Im Header der ChatView oder über den [+ Neu] Button:

```
┌─ Neue Konversation ──────────────┐
│                                   │
│  Anbieter:                        │
│  ○ Claude  ○ Codex  ○ Gemini     │
│                                   │
│  Modell:                          │
│  [Dropdown aus config.models]     │
│                                   │
│  Projekt:                         │
│  [Dropdown aus offenen Tabs]      │
│                                   │
│  [Erstellen]  [Abbrechen]         │
└───────────────────────────────────┘
```

### 4.5 Markdown-Rendering
- Code-Blöcke mit Syntax-Highlighting + Copy-Button
- Listen, Tabellen, Links
- User-Nachrichten: rechts-ausgerichtet, themed
- Assistant-Nachrichten: links-ausgerichtet, mit Provider-Icon
- Kostenanzeige pro Nachricht (optional, aus Stream-JSON)

---

## Phase 5: Multi-Agent Orchestration + Ask-User Bridging

### 5.1 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `internal/backend/app_orchestrator.go` | NEU | Multi-Agent-Koordination |
| `internal/backend/app_ask_user.go` | NEU | Ask-User Bridging: Agent-Fragen → Chat |
| `frontend/src/components/AskUserDialog.svelte` | NEU | Strukturierte Frage-Antwort-UI |

### 5.2 Multi-Agent Orchestration

**Multi-Agent = mehrere Claude-Sessions die koordiniert an einem Kanban-Plan arbeiten.**

- Nutzt das bestehende Session-System (`CreateSession`)
- Orchestrator ist eine Schicht über dem Planungs-System (Phase 3)
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
1. Plan wird im Kanban genehmigt (Phase 3)
2. Orchestrator startet erste(n) Schritt(e)
3. Jeder Schritt = 1 Claude-Session mit generiertem Prompt
4. Bei `activity: done` → nächsten Schritt starten
5. Bei `activity: waitingPermission` oder `waitingAnswer` → Ask-User Bridging
6. Kanban-Karten zeigen Agent-Status in Echtzeit (Dot-Indicator)
7. Dashboard zeigt Gesamt-Fortschritt

### 5.3 Ask-User Bridging (neues Feature)

**Problem:** Wenn ein Agent im Hintergrund läuft und Input braucht (`waitingPermission`, `waitingAnswer`), merkt der User das nur am blinkenden Pane-Rand — leicht zu übersehen.

**Lösung:** Agent-Fragen werden strukturiert in den Chat und als Dialog-Notification weitergeleitet.

**Ablauf:**
1. `terminal:activity` meldet `waitingPermission` oder `waitingAnswer`
2. Backend analysiert den Screen-Buffer der Session (letzte Zeilen)
3. Extrahiert die Frage (z.B. "Allow tool: Read file.txt? [Y/n]")
4. Emittiert `ask_user:question` Event mit SessionID + Frage
5. Frontend zeigt:
   - **Desktop-Notification** (bestehend, erweitert)
   - **Badge im LeftNav** auf der Session-Karte
   - **AskUserDialog** — Popup mit Frage + Antwort-Buttons (Ja/Nein/Custom)
   - **Chat-Integration** — Frage erscheint als Nachricht im Chat (optional)
6. User antwortet → Antwort wird via `WriteToSession()` an die PTY gesendet

**Datenstruktur:**
```go
type AskUserQuestion struct {
    SessionID   int      `json:"session_id" yaml:"session_id"`
    SessionName string   `json:"session_name" yaml:"session_name"`
    Question    string   `json:"question" yaml:"question"`
    Options     []string `json:"options" yaml:"options"`     // ["Y", "n"] oder frei
    Timestamp   string   `json:"timestamp" yaml:"timestamp"`
}
```

**Events:**
```
ask_user:question  → AskUserQuestion   // Agent braucht Input
ask_user:answered  → { sessionId, answer: string }  // User hat geantwortet
```

**Screen-Buffer-Analyse:**
- Nutzt bestehende `PlainTextRows()` Methode
- Pattern-Matching auf letzte 3-5 Zeilen
- Erkennt: `[Y/n]`, `[y/N]`, `Allow`, `Confirm`, `?` am Zeilenende
- Fallback: zeigt die letzten 3 Zeilen als Freitext

---

## Phase 6: Queue-Übersicht

### 6.1 Dateien

| Datei | Aktion | Beschreibung |
|-------|--------|--------------|
| `frontend/src/components/QueueOverview.svelte` | NEU | Queue-View im Main Content |
| `internal/backend/app_queue.go` | EDIT | `GetAllQueues()` Methode hinzufügen |

### 6.2 Funktionalität

- Zeigt Queue-Items des aktuellen Projekts (nicht projektübergreifend)
- Gruppiert nach Session
- Status-Filter (pending/sent/done)
- Drag&Drop-Priorisierung innerhalb einer Session
- Bulk-Aktionen (alle done löschen, neuen Prompt hinzufügen)
- Direkt-Link zur Session (Klick → wechselt zu Terminals-View + fokussiert Pane)

**Neue Backend-Methode:**
```go
type QueueOverviewItem struct {
    SessionID   int         `json:"session_id" yaml:"session_id"`
    SessionName string      `json:"session_name" yaml:"session_name"`
    Dir         string      `json:"dir" yaml:"dir"`
    Activity    string      `json:"activity" yaml:"activity"`
    Items       []QueueItem `json:"items" yaml:"items"`
}

func (a *AppService) GetAllQueues() []QueueOverviewItem
```

---

## Implementierungs-Reihenfolge

```
Phase 1: Left Navigation Pane          ████░░░░░░  Basis-Infrastruktur
Phase 2: Dashboard (erweitert)          ██░░░░░░░░  Schneller Gewinn
Phase 3: Kanban + Planung + Scheduler   ██████████  Kernfeature (alles integriert)
Phase 4: Chat View                      ████████░░  Eigene Main-Content-View
Phase 5: Multi-Agent + Ask-User         ██████░░░░  Aufbauend auf Kanban
Phase 6: Queue-Übersicht                ██░░░░░░░░  Erweitert bestehend
```

**Abhängigkeiten:**
- Phase 1 → alle anderen (Layout-Basis)
- Phase 3 → Phase 5 (Kanban-Pläne werden vom Orchestrator ausgeführt)
- Phase 2, 4, 6 sind untereinander unabhängig

**Empfohlene Umsetzung:**
1. **Sprint 1:** Phase 1 (LeftNav) + Phase 2 (Dashboard)
2. **Sprint 2:** Phase 3 (Kanban inkl. Planung + Scheduler)
3. **Sprint 3:** Phase 4 (Chat) + Phase 6 (Queue) parallel
4. **Sprint 4:** Phase 5 (Multi-Agent + Ask-User)

---

## Neue Dateien (Gesamt)

### Frontend (10 neue Dateien)
```
frontend/src/components/LeftNav.svelte
frontend/src/components/KanbanBoard.svelte
frontend/src/components/KanbanColumn.svelte
frontend/src/components/KanbanCard.svelte
frontend/src/components/ChatView.svelte
frontend/src/components/ChatMessage.svelte
frontend/src/components/ChatInput.svelte
frontend/src/components/QueueOverview.svelte
frontend/src/components/AskUserDialog.svelte
frontend/src/stores/workspace.ts
frontend/src/stores/kanban.ts
frontend/src/stores/chat.ts
```

### Backend (9 neue Dateien)
```
internal/backend/app_dashboard.go
internal/backend/app_kanban.go
internal/backend/app_kanban_plan.go
internal/backend/app_kanban_schedule.go
internal/backend/app_chat.go
internal/backend/app_chat_stream.go
internal/backend/app_orchestrator.go
internal/backend/app_ask_user.go
internal/config/kanban.go
internal/config/chat.go
```

### Geänderte Dateien (5)
```
frontend/src/App.svelte                  — Layout-Umbau + View-Routing
frontend/src/components/Sidebar.svelte   — Integration mit LeftNav
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
  default_view: terminals

orchestrator:
  max_parallel_agents: 3
  auto_start_next: true

chat:
  default_provider: claude
  default_model: ""
  history_limit: 100  # max conversations to keep
```

---

## Claude Studio Feature-Abdeckung

| # | Claude Studio Feature | Unser Plan | Phase |
|---|----------------------|------------|-------|
| 1 | **Kanban Task Board** | Kanban mit Drag&Drop, Auto-Spalten, Issue-Sync | Phase 3 |
| 2 | **Scheduled Automation** | Im Kanban integriert als "Zeitpläne"-Tab | Phase 3 |
| 3 | **Multi-Agent Orchestration** | Orchestrator über Kanban-Pläne | Phase 5 |
| 4 | **Ask-User Bridging** | Screen-Buffer-Analyse → Dialog + Chat + Notification | Phase 5 |

---

## Technische Hinweise

1. **300-Zeilen-Limit:** Alle Go-Dateien unter 300 Zeilen halten (CLAUDE.md Regel)
2. **models.ts Sync:** Jede neue Go-Struct braucht manuelle models.ts-Klasse
3. **yaml+json Tags:** Alle Structs brauchen beide Tags
4. **UI-Text Deutsch:** Alle Labels, Tooltips, Dialoge auf Deutsch
5. **Kein externer Web-Zugriff:** Chat nutzt lokale CLI-Tools, kein API-Direktzugriff
6. **Svelte Reactive-Regel:** Keine Variable-Zuweisungen in `$:` Blöcken
7. **Concurrency:** Neue Mutexe für Kanban-State, Chat-State, Orchestrator-State
8. **Persistenz:** Kanban in `~/.multiterminal-kanban.json`, Chat in `~/.multiterminal-chat/` (pro Konversation eine JSON-Datei)
9. **Projekt-Kontext:** Ergibt sich immer aus dem aktiven Tab-Verzeichnis. Nur Dashboard zeigt aggregiert.
