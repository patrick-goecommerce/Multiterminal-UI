# Chat als Pane-Darstellungsmodus — Design Spec

**Datum:** 2026-05-24
**Status:** Approved (Design), bereit für Implementierungsplanung
**Branch-Kontext:** alpha-main (Wails v3 + Multi-Window)

## Problem

Multiterminal ist stark terminallastig. In der Konsolenansicht (rohes PTY/xterm) vermischen sich
Ein- und Ausgabe in einem einzigen Scrollback-Buffer. Eine klare Abtrennung zwischen Eingabeteil
und Ausgabeteil — wie sie Claude Code oder Telegram bieten — ist dort extrem schwierig.

Ziel: Eine Chat-artige Darstellung mit fixer Eingabezeile unten und strukturierten Nachrichten-Bubbles
darüber, als **alternativer Darstellungsmodus eines Panes** (nicht als Farb-Theme, nicht als separater
Top-Level-Bereich).

## Vorhandener Stand (Sprint 3, Commit 2af85b4)

Es existiert eine **halbfertige, nicht angebundene** Chat-Implementierung:

- **Backend:** `internal/backend/app_chat.go` (Konversations-CRUD, `.mtui/chat/*.json`),
  `internal/backend/app_chat_stream.go` (ein `claude --print`-Aufruf pro Nachricht).
- **Frontend:** `ChatView.svelte`, `ChatMessage.svelte`, `ChatInput.svelte`, `stores/chat.ts`,
  `lib/markdown.ts`.

**Defekte/Lücken:**
1. Nicht im `LeftNav` verlinkt → nicht erreichbar.
2. Streaming-Events (`chat:stream`/`chat:done`) werden im Frontend nie abonniert (`onMount` ist nur TODO).
3. `buildPrompt()` schickt nur die letzte User-Nachricht an `claude --print` → **kein Multi-Turn-Kontext**
   → gespeicherte Antwort ist "(Keine Antwort)".

Der Code ist kein Wegwerf-Code, aber der Kern (`streamChatResponse` + `buildPrompt`) wird von
"Einweg-`--print`" auf "persistente stream-json-Session" umgebaut.

## Recherche: Don't reinvent the wheel

Das wörtliche Parsen der interaktiven Claude-TUI (ANSI-Output) ist fragil und wird von niemandem so gemacht.
Claude Code bietet ein **offizielles strukturiertes Protokoll**, das exakt für Chat-UIs gebaut ist:

```
claude -p --output-format stream-json --input-format stream-json \
       --verbose --include-partial-messages
```

Liefert NDJSON-Events (Text-Deltas, Tool-Use, Tool-Result, Kosten/Tokens) statt ANSI. Folge-User-Turns
gehen als JSON über stdin → echtes Multi-Turn mit vollem Agent (Tools, Edits, Bash).

Referenzprojekte, die alle stream-json über Subprozess nutzen (keiner parst die TUI):
- **claude-code-webui** (sugyan) — Go/Deno-Backend + React, dem hiesigen Stack am nächsten.
- **claudecodeui / CloudCLI** (siteboon) — Session-Management, Permission-Controls, Plan-Mode.
- **Opcode** (~21k ⭐) — Tauri 2, Chat-Transcript + Diff-Viewer.

Quellen:
- https://code.claude.com/docs/en/headless
- https://code.claude.com/docs/en/cli-reference
- https://github.com/sugyan/claude-code-webui
- https://github.com/siteboon/claudecodeui

## Designentscheidungen

| Entscheidung | Wahl | Begründung |
|---|---|---|
| Chat-Kern | Persistenter `claude`-Subprozess mit bidirektionalem stream-json | Saubere I/O-Trennung **und** voller Agent, kein Kompromiss; offiziell unterstützt |
| Permissions | `--permission-mode` pro Konversation (Go-only) | Schlankes v1, kein Node-Sidecar nötig |
| Verankerung | Darstellungsmodus pro Pane (Terminal ↔ Chat) | Löst den Schmerz dort, wo er ist; baut auf vorhandenem Pane/Launch-System auf |
| Theme-Platzierung | Modus = pro Pane; Chat-*Stil* (Claude-Code/Telegram) = Settings-Dropdown | Modus ist strukturell (kein Palettentausch); Stil ist reine Optik |
| Multi-Window | Chat-Pane über vorhandenen Tab-Detach in eigenes Fenster herauslösbar | Nutzt bestehende `DetachTab`/`MergeWindowToMain`-Infrastruktur |

## Architektur

### 1. Backend — neuer Session-Typ `ChatSession`

Ein Chat-Pane ist **kein PTY**. Parallel zu `internal/terminal/session.go` (PTY+ConPTY) entsteht eine
`ChatSession`: ein langlebiger `claude`-Subprozess mit bidirektionalem stream-json.

```
claude -p --output-format stream-json --input-format stream-json \
       --verbose --include-partial-messages \
       --permission-mode <plan|acceptEdits|bypassPermissions> \
       [--model X] [--resume <session_id>]
```

- **stdin:** Folge-User-Turns als `{"type":"user","message":{"role":"user","content":"…"}}`.
- **stdout:** NDJSON → Domain-Events. **Buffer-Management:** JSON-Objekte können über Chunks splitten;
  `bufio.Scanner` mit ausreichend großem Buffer (oder eigener Reader) statt Default-Token-Limit.
- **Windows-Gotchas:** `claude` ist `.cmd`-Shim → via `COMSPEC /c` wrappen; `CLAUDECODE` aus Env strippen
  (analog `session.go:Start`).
- **Lifecycle:** Start beim Anlegen/Aktivieren eines Chat-Panes; Stop beim Schließen; Resume nach
  App-Neustart via `--resume <session_id>`.

### 2. Pane-Modell — Vereinheitlichung

`PaneMode` bleibt unverändert (`shell | claude | claude-yolo | codex | … | gemini-yolo`). Ein Pane erhält
ein zusätzliches Feld:

```ts
display: 'terminal' | 'chat'   // Default 'terminal'
conversationId?: string        // gesetzt, wenn display === 'chat'
```

- `Pane.sessionId` (PTY) bleibt für Terminal-Panes.
- `PaneGrid` rendert je nach `display`: `TerminalPane` (xterm) **oder** neue `ChatPane` (Bubbles + Eingabe).
- Umschalt-Button in `PaneTitlebar`. Hinweis: Umschalten bedeutet anderer Backend-Prozess →
  effektiv Relaunch, kein "Live-Umfärben" desselben Buffers.
- Auswahl des Modus auch im `LaunchDialog` ("Anzeige: Terminal | Chat").

### 3. Event-Mapping (stream-json → Chat-UI)

| stream-json Event | Chat-Darstellung |
|---|---|
| `system` / `subtype: init` | `session_id` merken (für `--resume`); Modell/Tools anzeigen |
| `content_block_delta` / `delta.type: text_delta` | Assistant-Text streamen (Bubble) |
| `content_block_start` `tool_use` + `input_json_delta` | Aufklappbare Tool-Karte (z.B. "Edit auth.ts") |
| `tool_result` | Ergebnis-Karte unter der Tool-Karte |
| `result` | Kosten/Tokens (`cost_usd`, `usage`) im Bubble-Footer |
| Prozess-Exit/Fehler | Fehler-Bubble |

Permissions v1: kein interaktiver Button; der pro Konversation gewählte `--permission-mode` regelt alles.

### 4. Themes / Platzierung

- **Modus Terminal↔Chat:** pro Pane (Launch-Dialog + Titelleisten-Toggle) — **nicht** unter Settings.
- **Chat-*Stil*** ("Claude-Code-Look" vs. "Telegram-Look"): neues Dropdown "Chat-Stil" unter Settings,
  getrennt vom Farb-Theme. Unterschied nur in CSS (Bubble-Ausrichtung/Dichte/Farben), nicht in der Logik.
  - Neues Config-Feld `chat_style: 'claude-code' | 'telegram'` → **Wails-Sync beachten:** Feld in
    `config.Config` mit `yaml`+`json`-Tags UND manuell in `frontend/wailsjs/go/models.ts` (Klasse +
    Konstruktor) ergänzen (bekannter wiederkehrender Bug).

### 5. Persistenz, Multi-Window & Store-Fix

- Konversationen bleiben in `.mtui/chat/*.json` (Sprint-3-Format, erweitert um `session_id` und Tool-Events
  in den Messages). Resume nach App-Neustart via `--resume`.
- **Detach:** Chat-Pane in eigenem Tab → über vorhandenen Tab-Detach in eigenes Fenster herauslösbar.
  Direkter "Pop-out"-Button optional (v2).
- **Store-Fix:** `stores/chat.ts` hat aktuell *globalen* `streaming`/`streamBuffer` → bricht bei mehreren
  Chat-Panes. Muss **pro `conversationId`** verwaltet werden (z.B. `Map<convId, {streaming, buffer}>`).

### 6. Datenmodell (erweitert)

```go
// ChatMessage erweitert um optionale Tool-Felder
type ChatMessage struct {
    ID        string `json:"id" yaml:"id"`
    Role      string `json:"role" yaml:"role"` // user | assistant | tool
    Content   string `json:"content" yaml:"content"`
    Timestamp string `json:"timestamp" yaml:"timestamp"`
    Cost      string `json:"cost" yaml:"cost"`
    Tokens    int    `json:"tokens" yaml:"tokens"`
    // neu (optional):
    ToolName   string `json:"tool_name,omitempty" yaml:"tool_name,omitempty"`
    ToolInput  string `json:"tool_input,omitempty" yaml:"tool_input,omitempty"`
    ToolResult string `json:"tool_result,omitempty" yaml:"tool_result,omitempty"`
}

// Conversation erweitert um Session-Resume + Permission-Mode
type Conversation struct {
    // … bestehende Felder …
    SessionID      string `json:"session_id,omitempty" yaml:"session_id,omitempty"`
    PermissionMode string `json:"permission_mode,omitempty" yaml:"permission_mode,omitempty"`
}
```

## Scope

**v1 (drin):**
- Persistente stream-json `ChatSession` (Subprozess-Lifecycle, stdin-Turns, NDJSON-Parsing mit Buffering)
- Multi-Turn-Kontext + `--resume` nach Neustart
- Rendering: Text-Bubbles (Markdown), Tool-Karten, Tool-Results, Kosten/Tokens, Fehler-Bubbles
- `--permission-mode` pro Konversation
- Pane-`display`-Feld + `ChatPane`-Komponente + Titelleisten-Toggle + Launch-Dialog-Option
- 2 Chat-Stile (Claude-Code / Telegram) via Settings-Dropdown
- Store-Fix (pro-Konversation-Streaming-State)
- Detach via vorhandenen Tab-Detach

**v2 (draußen, YAGNI):**
- Interaktive Approval-Buttons (Agent-SDK `canUseTool` via Node/Python-Sidecar oder MCP-Permission-Tool)
- Team-Chat (mehrere Rollen/Agenten in einer Konversation)
- Bild-/Datei-Anhänge
- Direkter "Pop-out"-Button (statt nur Tab-Detach)

## Risiken & Gotchas

- **NDJSON-Chunking:** stream-json kann JSON-Objekte über Lese-Chunks splitten → robustes Buffering nötig.
- **Windows `.cmd`-Shim & `CLAUDECODE`-Env:** wie bei PTY-Sessions behandeln.
- **Wails-Sync (`models.ts`):** neues `chat_style`-Config-Feld manuell nachziehen.
- **Svelte-Reaktivität (SettingsDialog):** Chat-Stil-Dropdown **nicht** als Variablen-Zuweisung in
  `$:`-Block — Funktion aufrufen (`$: if (visible) initDialog();`), sonst Reset-Bug.
- **Modus-Umschalten = Relaunch:** klar kommunizieren; kein verlustfreies Live-Umschalten.
- **Go-Dateigröße:** max 300 Zeilen pro Datei — `ChatSession`-Logik ggf. auf mehrere Dateien aufteilen
  (z.B. `app_chat_session.go`, `app_chat_stream.go`, `app_chat_events.go`).
