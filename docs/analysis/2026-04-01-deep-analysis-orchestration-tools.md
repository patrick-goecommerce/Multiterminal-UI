# Deep Analysis: Orchestration Tools for MTUI

> Stand: 2026-04-01 | Analysierte Tools: Just Ship, GSD, Aperant, Claude Agent Teams

## Executive Summary

Vier Tools wurden tiefgehend analysiert (Source-Code-Level), um die beste Orchestrierungs-Architektur fuer MTUI zu identifizieren. Jedes Tool loest ein anderes Teilproblem exzellent:

| Tool | Staerke | Schwaeche | Fuer MTUI relevant |
|------|---------|-----------|-------------------|
| **Just Ship** | Agent SDK Integration, Tiered Models, QA Fix Loop | Kein Merge-Conflict-Handling, keine GUI | SDK-Orchestrierung, Model-Tiering, Worktree-Slots |
| **GSD** | Context Engineering, Wave Execution, Plan-Struktur | Schwerer File-Overhead, kein Board | Context-Rot-Loesung, Wave-System, Verification |
| **Aperant** | Vollstaendige GUI-Pipeline, Memory, Multi-Account | Fragile file-basierte Agent-Kommunikation, Over-Engineering | XState Task-Machine, QA-Loop, Rate-Limit-Handling |
| **Agent Teams** | Native Claude-Primitive, Peer-to-Peer, Shared Tasks | Experimentell, kein GUI-API, Windows-Tmux-Problem | Execution-Engine via Tmux-Shim |

---

## Pattern-Extraktion: Was MTUI uebernehmen sollte

### Tier 1: Kritische Patterns (Must-Have)

#### 1. Fresh Context Per Agent (GSD)
**Problem:** Context-Rot -- AI-Qualitaet degradiert ab ~50% Fensterauslastung.
**Loesung:** Jeder Sub-Agent bekommt ein frisches Context-Fenster. Der Orchestrator bleibt schlank (~10-15%), uebergibt nur Dateipfade, keine Inhalte.
**Implementation fuer MTUI:**
- Orchestrator-Session = lean coordinator (Pfade + Aufgabenbeschreibung)
- Worker-Sessions = frische `claude -p` oder Agent-Teams-Teammates
- Niemals den gesamten Codebase-Kontext in einen Agent pumpen

#### 2. Tiered Model Selection (Just Ship)
**Problem:** Opus fuer alles ist 10-20x teurer als noetig.
**Loesung:** Task-Typ bestimmt Modell:
- **Opus:** Orchestrierung, Architektur-Entscheidungen, komplexe Planung
- **Sonnet:** Implementation, kreative UI-Arbeit, Code-Fixes
- **Haiku:** Triage, DB-Migrations, Build-Fixes, Security-Checks, QA
**Implementation fuer MTUI:** Model-Feld pro Kanban-Card-Step, konfigurierbar in Settings

#### 3. Wave-Based Parallel Execution (GSD + MTUI-existing)
**Problem:** Abhaengigkeiten zwischen Tasks verhindern naive Parallelisierung.
**Loesung:** Dependencies werden zur Plan-Zeit in Waves aufgeloest:
```
Wave 1: [Task A, Task B]     -- parallel
Wave 2: [Task C(dep:A)]      -- wartet auf Wave 1
Wave 3: [Task D(dep:B,C)]    -- wartet auf Wave 2
```
**MTUI hat das bereits** in `app_orchestrate_parallel.go`. Pattern ist validiert.

#### 4. XState-artige Task State Machine (Aperant)
**Problem:** Ad-hoc Status-Strings sind fehleranfaellig und schwer zu debuggen.
**Loesung:** Formale State Machine mit expliziten Transitions und Guards:
```
backlog -> planning -> plan_review -> coding -> qa_review -> qa_fixing -> human_review -> done
                                                                      -> error
```
**Implementation fuer MTUI:** Go-seitige State Machine mit definierten Transitions, Events und Guards. Kein direktes Status-Setzen mehr.

#### 5. QA Fix Loop (Just Ship + Aperant)
**Problem:** Build/Test-Fehler nach Implementation erfordern manuelles Eingreifen.
**Loesung:** Automatischer Zyklus: QA pruefen -> bei Fehler: Sonnet-Agent fixen lassen -> erneut pruefen -> max 3 Iterationen
**Implementation fuer MTUI:** Nach jedem Step: `go build`, `go test`, `go vet`. Bei Fehler: automatischer Fix-Agent mit Fehlermeldung als Kontext.

### Tier 2: Wichtige Patterns (Should-Have)

#### 6. Complexity-Gated Pipeline (Aperant + Just Ship)
**Problem:** Triviale Tasks durchlaufen denselben schweren Prozess wie komplexe.
**Loesung:** Triage-First:
- **Trivial:** Direkt implementieren, kein Plan noetig
- **Medium:** Kurzer Plan (2-3 Steps), kein Quiz
- **Komplex:** Voller Pipeline (Quiz -> Plan -> Review -> Execute -> QA)
**MTUI hat Ansaetze** in `AssessComplexity()`. Muss end-to-end funktionieren.

#### 7. Goal-Backward Verification (GSD)
**Problem:** "Task done" heisst nicht "Feature funktioniert".
**Loesung:** Plans deklarieren `must_haves`:
- **Truths:** "User kann sich einloggen" (testbar)
- **Artifacts:** `src/auth.ts` existiert mit mindestens 50 Zeilen
- **Key Links:** `src/auth.ts` -> `/api/auth` via fetch-Aufruf
Verification prueft diese gegen den tatsaechlichen Code.

#### 8. Worktree Slot Management (Just Ship)
**Problem:** Unkontrolliertes Worktree-Erstellen fuehrt zu Disk-Bloat und Konflikten.
**Loesung:** Pool-basierter Manager mit `allocate()` / `release()` / `park()` / `reattach()`:
- Feste Slot-Anzahl (z.B. 4)
- Wait-Queue wenn alle Slots belegt
- `park()` fuer pausierte Pipelines (Worktree auf Disk, Slot frei)
- Stale-Pruning fuer verwaiste Worktrees
**MTUI hat einfaches Worktree-Management.** Slot-Pattern ist robuster.

#### 9. Context Monitor (GSD)
**Problem:** Agents merken nicht, dass ihr Context-Fenster voll wird.
**Loesung:** PostToolUse-Hook prueft Context-Metriken:
- 35% verbleibend: WARNING (keine neue komplexe Arbeit starten)
- 25% verbleibend: CRITICAL (abschliessen und zurueckmelden)
- Debounce (5 Tool-Calls zwischen Warnungen)
**Implementation fuer MTUI:** Bei `claude -p` schwer (kein Hook-Zugriff). Bei Agent Teams: ueber Hooks konfigurierbar.

#### 10. Structured Output Validation + LLM Repair (Aperant)
**Problem:** AI-generiertes JSON ist oft malformed.
**Loesung:** Zod-Schema-Validation -> bei Fehler: leichtgewichtiger Single-Call LLM Repair (kein volles Re-Planning) -> bei erneutem Fehler: volles Re-Planning (max 3x)
**Implementation fuer MTUI:** Go-seitige JSON-Schema-Validation + Repair-Prompt als Fallback.

### Tier 3: Nice-to-Have Patterns

#### 11. Human-in-the-Loop Pause/Resume (Just Ship)
Pipeline pausiert bei kritischen Entscheidungen, wartet auf User-Input via UI, setzt exakt dort fort.
Bereits konzeptionell in MTUI's Chat-Popup vorgesehen.

#### 12. Event-Driven Board Updates (Just Ship)
SDK-Hooks (`SubagentStart`, `SubagentStop`, `PostToolUse`) pushen Live-Status an die UI.
MTUI hat bereits Event-System (`kanban:subcard-update`).

#### 13. Multi-Account Rate-Limit-Swapping (Aperant)
Bei 429/401: naechsten Account aus Priority-Queue nehmen, Session fortsetzen.
Relevant fuer Teams mit mehreren Claude-Accounts.

#### 14. Crash Recovery via Checkpoint (Just Ship)
Pipeline-Phase wird persistent gespeichert. Bei Crash: Worktree reattachen, ab letztem Checkpoint fortsetzen.
Wichtig fuer Long-Running-Pipelines.

#### 15. Deviation Rules (GSD)
Executors treffen auf unvorhergesehene Arbeit. Taxonomie:
- Rules 1-3 (Bugs, fehlende Imports, Blocker): Auto-Fix (max 3 Versuche)
- Rule 4 (Architektur-Aenderungen): User-Entscheidung erforderlich
Verhindert sowohl Stalling als auch Scope-Creep.

---

## Anti-Patterns: Was MTUI vermeiden muss

### 1. Agents schreiben in dieselbe JSON-Datei wie der Orchestrator (Aperant)
Aperants `implementation_plan.json` ist gleichzeitig Plan UND State-Store, beschrieben von Agents UND Orchestrator. Fuehrt zu Daten-Korruption und erfordert staendiges Re-Stamping.
**MTUI-Regel:** Orchestrator besitzt den State exklusiv. Agents liefern Ergebnisse zurueck, aendern nie den globalen State.

### 2. String-Matching fuer strukturierte Verdicts (Aperant)
`status: passed` aus Markdown parsen ist fragil. QA-Fixer editiert die Report-Datei und korrumpiert das Verdict.
**MTUI-Regel:** Immer JSON mit Schema-Validation fuer maschinenlesbare Kommunikation.

### 3. Over-Engineering der Memory-Schicht (Aperant)
30+ Dateien, BM25 + Dense Vector + Graph Retrieval, Cross-Encoder Reranking -- fuer eine Inject-Funktion die "ab Step 5 relevante Memories als System-Message einfuegt."
**MTUI-Regel:** Einfache File-basierte Memory (Markdown in `docs/mtui/board/`) reicht. Komplexitaet nur wenn bewiesen noetig.

### 4. 50 QA-Iterationen ohne Budget-Limit (Aperant)
Ein Task kann massiv Token verbrennen bevor der User es merkt.
**MTUI-Regel:** Harte Budget-Limits pro Card ($X max). QA-Loop max 3 Iterationen, dann Eskalation an User.

### 5. Keine Merge-Conflict-Resolution (Just Ship)
Parallele Pipelines erstellen Branches von `origin/main`, aber es gibt keine Loesung fuer Konflikte wenn zwei gleichzeitig fertig werden.
**MTUI-Regel:** Post-Wave Merge-Check. Bei Konflikten: AI-gesteuerte Resolution (wie Aperant, aber einfacher) oder User-Eskalation.

### 6. Kein Inter-Agent-Communication zur Laufzeit (Just Ship)
Agents koennen sich waehrend der Ausfuehrung keine Informationen weitergeben. Wenn der Data-Engineer eine Migration erstellt und der Backend-Agent den Tabellennamen braucht, muss der Orchestrator das vorher wissen.
**MTUI-Regel:** Agent Teams loest das nativ via SendMessage/Broadcast.

---

## Architektur-Empfehlung fuer MTUI

### Hybrides 3-Schichten-Modell

```
┌─────────────────────────────────────────────────────┐
│  MTUI Frontend (Svelte)                             │
│  ┌─────────────┐  ┌──────────────────────────────┐  │
│  │ Kanban Board │  │ Terminal Multiplexer         │  │
│  │ (Tickets,    │  │ (Live-View aller Sessions,   │  │
│  │  Status,     │  │  Agent-Output, User-Input)   │  │
│  │  QA-Reports) │  │                              │  │
│  └──────┬──────┘  └──────────────┬───────────────┘  │
│         │     Wails Events       │                  │
├─────────┼────────────────────────┼──────────────────┤
│  MTUI Backend (Go)               │                  │
│  ┌──────┴──────────────────────┐ │                  │
│  │ Orchestrator                │ │                  │
│  │ - Task State Machine        │ │                  │
│  │ - Complexity Gating         │ │                  │
│  │ - Wave Planner              │ │                  │
│  │ - Budget Tracker            │ │                  │
│  │ - QA Fix Loop               │ │                  │
│  └──────┬──────────────────────┘ │                  │
│         │                        │                  │
│  ┌──────┴──────────────────────┐ │                  │
│  │ Execution Engine            │ │                  │
│  │ Option A: claude -p         │◄┘                  │
│  │   (headless, JSON output)   │                    │
│  │ Option B: Agent Teams       │                    │
│  │   (via Tmux-Shim, live PTY) │                    │
│  │ Option C: Claude Agent SDK  │                    │
│  │   (programmatic control)    │                    │
│  └─────────────────────────────┘                    │
└─────────────────────────────────────────────────────┘
```

### Execution Engine Empfehlung

**Primaer: `claude -p --output-format json` (Option A)**
- Bereits implementiert in MTUI (`RunHeadless`)
- Volle Kontrolle ueber Prompts und Output-Parsing
- Funktioniert auf Windows ohne Tmux
- Jeder Agent = frischer Kontext (GSD-Pattern)
- Model-Selection pro Agent (Just-Ship-Pattern)

**Sekundaer: Agent Teams via Tmux-Shim (Option B)**
- Fuer komplexe Tasks die inter-Agent-Kommunikation brauchen
- Live-Terminal-View fuer User (MTUI-USP)
- Tmux-Shim fast fertig, `capture-pane` fehlt noch
- Windows-Kompatibilitaet ueber Shim geloest

**Zukunft: Claude Agent SDK (Option C)**
- Just Ship nutzt das bereits erfolgreich
- Gibt programmatische Kontrolle ueber Sessions
- Hooks fuer Live-Events (SubagentStart, etc.)
- Noch nicht evaluiert fuer Go-Integration (SDK ist TypeScript)

### Empfohlene Pipeline

```
1. TRIAGE (Haiku, ~$0.001)
   Input: Card-Beschreibung
   Output: Complexity (trivial/medium/complex), QA-Tier, betroffene Bereiche

2. ROUTING
   trivial  -> direkt zu EXECUTE (1 Agent, kein Plan)
   medium   -> PLAN (kurz) -> EXECUTE
   complex  -> QUIZ -> PLAN -> REVIEW -> EXECUTE -> QA

3. QUIZ (Opus, optional)
   3-6 Fragen an User zur Klaerung von Ambiguitaeten
   Output: Answers in Card-Context

4. PLAN (Opus)
   Input: Card + Answers + Codebase-Kontext (Dateipfade, nicht Inhalte)
   Output: Steps mit Dependencies, Wave-Zuordnung, Model pro Step
   Validation: JSON-Schema + LLM-Repair bei Fehler

5. REVIEW (User)
   Plan-Anzeige mit Correction-Feld
   User kann Steps aendern/entfernen/hinzufuegen

6. EXECUTE (Wave-basiert)
   Pro Wave: parallele Agents in isolierten Worktrees
   Pro Agent: frischer Kontext, nur relevante Dateipfade
   Model per Step (Haiku/Sonnet/Opus)
   Post-Step: Build-Check, bei Fehler: Auto-Fix (max 3x)

7. QA (Haiku)
   must_haves Verification gegen Codebase
   Bei Fehler: QA-Fix-Loop (Sonnet, max 3x)
   Bei Erfolg: Merge Worktree -> Hauptbranch

8. DONE
   Card nach "done" verschieben
   Learnings in Memory persistieren
```

---

## Naechste Schritte

### Phase 0: Stabilisierung (Empfohlen als Erstes)
1. Build testen -- kompiliert MTUI ueberhaupt noch?
2. Kanban-Board: Card-Drag-and-Drop fixen
3. RunHeadless: End-to-End-Test mit echtem `claude -p` Aufruf
4. Frontend-Events: Kommen `kanban:*` Events korrekt an?

### Phase 1: Neue Orchestrierung (Rewrite)
1. Task State Machine implementieren (Go, inspiriert von Aperants XState)
2. Triage-Agent (Haiku) fuer Complexity-Gating
3. Plan-Generierung mit JSON-Schema-Validation
4. Wave-Planner (bestehenden Code refactoren)

### Phase 2: Execution + QA
1. Fresh-Context-Execution via `claude -p` (1 Aufruf pro Step)
2. Worktree Slot Manager (Pool-basiert)
3. QA Fix Loop (Build/Test/Vet -> Fix -> Retry)
4. Budget-Tracking pro Card

### Phase 3: Agent Teams Integration
1. Tmux-Shim vervollstaendigen (`capture-pane`)
2. Agent Teams als optionale Execution Engine
3. Live-Terminal-View fuer Teammates
4. CustomPaneBackend implementieren wenn Anthropic #26572 shipped
