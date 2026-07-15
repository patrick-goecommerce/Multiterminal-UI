# Chat-Pane Display-Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Einen Chat-Darstellungsmodus pro Pane einführen, der einen persistenten `claude`-stream-json-Subprozess als Multi-Turn-Agent rendert (Bubbles + fixe Eingabezeile), statt der vermischten Terminal-Ansicht.

**Architecture:** Backend bekommt einen zweiten Session-Typ `ChatSession` (langlebiger `claude -p --output-format stream-json --input-format stream-json …`), dessen NDJSON-Output zu strukturierten Wails-Events gemappt wird. Im Frontend erhält ein Pane ein `display: 'terminal' | 'chat'`-Feld; `PaneGrid` rendert je nachdem `TerminalPane` (xterm) oder die neue `ChatPane`. Der Sprint-3-Code (Einweg-`--print`) wird auf die persistente Session umgebaut. Chat-Stil (Claude-Code/Telegram) ist eine reine CSS-Variante via Settings.

**Tech Stack:** Go 1.21+ (backend, `os/exec`, NDJSON), Wails v3 (Events), TypeScript/Svelte 4 (frontend), `lib/markdown.ts` (vorhanden).

**Spec:** `docs/superpowers/specs/2026-05-24-chat-pane-display-mode-design.md`

**Hinweis zu Tests:** Das Go-Backend hat eine Unit-Test-Suite (`go test ./internal/backend/...`) → echte TDD. Das Frontend hat **keine** Unit-Test-Infrastruktur → Verifikation per `cd frontend && npm run build` (TypeScript-Compile) + manueller E2E-Checkliste. Frontend-Tasks werden gemäß CLAUDE.md mit `needs-e2e-testing` markiert.

**Go-Path auf dieser Maschine:** `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`

---

## Phase 1 — Backend: persistente ChatSession

### Task 1: stream-json Event-Typen + Parser

Ersetzt den naiven `parseStreamDelta` durch ein Mapping der realen Claude-stream-json-Events auf Domain-Events. Voll unit-testbar.

**Files:**
- Create: `internal/backend/app_chat_events.go`
- Test: `internal/backend/app_chat_events_test.go`

- [ ] **Step 1: Failing-Test schreiben**

`internal/backend/app_chat_events_test.go`:

```go
package backend

import "testing"

func TestParseChatEvent_TextDelta(t *testing.T) {
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hallo"}}}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventText || ev.Text != "Hallo" {
		t.Fatalf("got %+v ok=%v, want text 'Hallo'", ev, ok)
	}
}

func TestParseChatEvent_Init(t *testing.T) {
	line := `{"type":"system","subtype":"init","session_id":"abc-123","model":"claude-opus-4-7"}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventInit || ev.SessionID != "abc-123" || ev.Model != "claude-opus-4-7" {
		t.Fatalf("got %+v ok=%v, want init session abc-123", ev, ok)
	}
}

func TestParseChatEvent_ToolUse(t *testing.T) {
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t1","name":"Edit","input":{}}}}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventToolUse || ev.ToolName != "Edit" || ev.ToolID != "t1" {
		t.Fatalf("got %+v ok=%v, want tool_use Edit", ev, ok)
	}
}

func TestParseChatEvent_Result(t *testing.T) {
	line := `{"type":"result","subtype":"success","total_cost_usd":0.0123,"usage":{"output_tokens":42}}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventResult || ev.CostUSD != 0.0123 || ev.OutputTokens != 42 {
		t.Fatalf("got %+v ok=%v, want result cost 0.0123", ev, ok)
	}
}

func TestParseChatEvent_Ignored(t *testing.T) {
	if _, ok := parseChatEvent([]byte(`{"type":"stream_event","event":{"type":"message_start"}}`)); ok {
		t.Fatal("message_start should be ignored")
	}
	if _, ok := parseChatEvent([]byte("not json")); ok {
		t.Fatal("non-JSON should be ignored")
	}
}
```

- [ ] **Step 2: Test ausführen, Fehlschlag bestätigen**

Run: `go test ./internal/backend/ -run TestParseChatEvent -v`
Expected: FAIL (undefined: parseChatEvent, ChatEventText, …)

- [ ] **Step 3: Minimale Implementierung**

`internal/backend/app_chat_events.go`:

```go
// Package backend — stream-json event parsing for chat sessions.
package backend

import "encoding/json"

// ChatEventKind classifies a parsed stream-json event.
type ChatEventKind string

const (
	ChatEventInit       ChatEventKind = "init"
	ChatEventText       ChatEventKind = "text"
	ChatEventToolUse    ChatEventKind = "tool_use"
	ChatEventToolResult ChatEventKind = "tool_result"
	ChatEventResult     ChatEventKind = "result"
)

// ChatEvent is the normalized domain event emitted to the frontend.
type ChatEvent struct {
	Kind         ChatEventKind `json:"kind"`
	Text         string        `json:"text,omitempty"`
	SessionID    string        `json:"sessionId,omitempty"`
	Model        string        `json:"model,omitempty"`
	ToolID       string        `json:"toolId,omitempty"`
	ToolName     string        `json:"toolName,omitempty"`
	ToolResult   string        `json:"toolResult,omitempty"`
	CostUSD      float64       `json:"costUsd,omitempty"`
	OutputTokens int           `json:"outputTokens,omitempty"`
}

// raw mirrors the relevant subset of Claude's stream-json schema.
type rawStreamLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	// result
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	// tool_result (top-level user message form)
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	// nested stream_event
	Event struct {
		Type         string `json:"type"`
		Delta        struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
		ContentBlock struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"content_block"`
	} `json:"event"`
}

// parseChatEvent normalizes one NDJSON line. ok=false means "ignore this line".
func parseChatEvent(line []byte) (ChatEvent, bool) {
	var r rawStreamLine
	if err := json.Unmarshal(line, &r); err != nil {
		return ChatEvent{}, false
	}
	switch r.Type {
	case "system":
		if r.Subtype == "init" {
			return ChatEvent{Kind: ChatEventInit, SessionID: r.SessionID, Model: r.Model}, true
		}
	case "result":
		return ChatEvent{Kind: ChatEventResult, CostUSD: r.TotalCostUSD, OutputTokens: r.Usage.OutputTokens}, true
	case "tool_result":
		return ChatEvent{Kind: ChatEventToolResult, ToolID: r.ToolUseID, ToolResult: r.Content}, true
	case "stream_event":
		switch r.Event.Type {
		case "content_block_delta":
			if r.Event.Delta.Type == "text_delta" && r.Event.Delta.Text != "" {
				return ChatEvent{Kind: ChatEventText, Text: r.Event.Delta.Text}, true
			}
		case "content_block_start":
			if r.Event.ContentBlock.Type == "tool_use" {
				return ChatEvent{Kind: ChatEventToolUse, ToolID: r.Event.ContentBlock.ID, ToolName: r.Event.ContentBlock.Name}, true
			}
		}
	}
	return ChatEvent{}, false
}
```

- [ ] **Step 4: Test ausführen, Erfolg bestätigen**

Run: `go test ./internal/backend/ -run TestParseChatEvent -v`
Expected: PASS (alle 5 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_chat_events.go internal/backend/app_chat_events_test.go
git commit -m "feat(chat): add stream-json event parser for chat sessions"
```

---

### Task 2: ChatSession-Struct + Subprozess-Lifecycle

Langlebiger `claude`-Prozess pro aktiver Konversation, mit stdin-Turns und NDJSON-Reader (robustes Buffering).

**Files:**
- Create: `internal/backend/app_chat_session.go`
- Modify: `internal/backend/app.go:34-53` (Feld `chatSessions` ergänzen), `app.go:61-66` (Map initialisieren)
- Test: `internal/backend/app_chat_session_test.go`

- [ ] **Step 1: Failing-Test schreiben** (testet Argv-Bau + NDJSON-Splitting, ohne echten Prozess)

`internal/backend/app_chat_session_test.go`:

```go
package backend

import (
	"strings"
	"testing"
)

func TestBuildChatArgs_Defaults(t *testing.T) {
	args := buildChatArgs("", "plan", "")
	joined := strings.Join(args, " ")
	for _, want := range []string{"--output-format stream-json", "--input-format stream-json", "--verbose", "--include-partial-messages", "--permission-mode plan", "-p"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q; got %q", want, joined)
		}
	}
}

func TestBuildChatArgs_ModelAndResume(t *testing.T) {
	args := buildChatArgs("claude-opus-4-7", "acceptEdits", "sess-9")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--model claude-opus-4-7") {
		t.Errorf("missing model; got %q", joined)
	}
	if !strings.Contains(joined, "--resume sess-9") {
		t.Errorf("missing resume; got %q", joined)
	}
}

func TestScanNDJSON_SplitsObjects(t *testing.T) {
	// Two complete objects, second split across writes — feed as one buffer here.
	input := `{"a":1}` + "\n" + `{"b":2}` + "\n"
	var got []string
	scanNDJSON(strings.NewReader(input), func(line []byte) {
		got = append(got, string(line))
	})
	if len(got) != 2 || got[0] != `{"a":1}` || got[1] != `{"b":2}` {
		t.Fatalf("got %v, want two objects", got)
	}
}
```

- [ ] **Step 2: Test ausführen, Fehlschlag bestätigen**

Run: `go test ./internal/backend/ -run "TestBuildChatArgs|TestScanNDJSON" -v`
Expected: FAIL (undefined: buildChatArgs, scanNDJSON)

- [ ] **Step 3: Minimale Implementierung**

`internal/backend/app_chat_session.go`:

```go
// Package backend — persistent claude stream-json chat session lifecycle.
package backend

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"
)

// ChatSession is a long-lived claude subprocess driven via stream-json.
type ChatSession struct {
	ConvID string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex
	closed bool
}

// buildChatArgs builds the claude argv (without the executable) for a chat session.
func buildChatArgs(model, permissionMode, resumeID string) []string {
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
	}
	if permissionMode == "" {
		permissionMode = "plan"
	}
	args = append(args, "--permission-mode", permissionMode)
	if model != "" {
		args = append(args, "--model", model)
	}
	if resumeID != "" {
		args = append(args, "--resume", resumeID)
	}
	return args
}

// scanNDJSON reads newline-delimited JSON from r and calls fn per complete line.
// Buffer is enlarged so large tool payloads spanning chunks are not truncated.
func scanNDJSON(r io.Reader, fn func(line []byte)) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 8*1024*1024) // up to 8 MiB per line
	for scanner.Scan() {
		b := scanner.Bytes()
		if len(b) == 0 {
			continue
		}
		// Copy: scanner reuses the underlying array on the next Scan().
		line := make([]byte, len(b))
		copy(line, b)
		fn(line)
	}
}

// startChatProcess launches the claude subprocess for a chat session.
// On Windows, claude is a .cmd shim → wrap via COMSPEC (see session.go).
func (a *AppService) startChatProcess(scope, model, permissionMode, resumeID string) (*ChatSession, error) {
	path := a.resolvedClaudePath
	if path == "" {
		path = "claude"
	}
	args := buildChatArgs(model, permissionMode, resumeID)
	cmd := wrapClaudeCmd(path, args) // helper below
	cmd.Dir = scope
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sess := &ChatSession{cmd: cmd, stdin: stdin}
	go scanNDJSON(stdout, func(line []byte) {
		if ev, ok := parseChatEvent(line); ok {
			a.dispatchChatEvent(sess.ConvID, ev) // implemented in Task 3
		}
	})
	return sess, nil
}

// SendTurn writes one user turn to the session's stdin as stream-json.
func (s *ChatSession) SendTurn(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	payload := `{"type":"user","message":{"role":"user","content":` + jsonQuote(content) + `}}` + "\n"
	_, err := s.stdin.Write([]byte(payload))
	return err
}

// Close terminates the session.
func (s *ChatSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	_ = s.stdin.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}
```

> `wrapClaudeCmd` und `jsonQuote` werden in Task 2b ergänzt (kleine Helfer, getrennt der 300-Zeilen-Regel wegen). Für diesen Test reichen `buildChatArgs` und `scanNDJSON`; die übrigen Funktionen kompilieren, sobald die Helfer existieren — erstelle sie im selben Step, damit das Paket baut:

`internal/backend/app_chat_helpers.go`:

```go
package backend

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
)

// wrapClaudeCmd wraps the claude path with COMSPEC on Windows (.cmd shim),
// otherwise runs it directly.
func wrapClaudeCmd(path string, args []string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = `C:\Windows\System32\cmd.exe`
		}
		full := append([]string{"/c", path}, args...)
		return exec.Command(comspec, full...)
	}
	return exec.Command(path, args...)
}

// jsonQuote returns a JSON-quoted string literal (with surrounding quotes).
func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
```

Dann in `internal/backend/app.go` das Feld ergänzen (nach `sessions map[int]*terminal.Session` bei Zeile 34):

```go
	chatSessions   map[string]*ChatSession // active chat sessions keyed by conversation ID
```

und in `NewAppService` (bei den anderen `make(...)`-Zeilen ~61):

```go
		chatSessions:  make(map[string]*ChatSession),
```

- [ ] **Step 4: Test + Build ausführen**

Run: `go test ./internal/backend/ -run "TestBuildChatArgs|TestScanNDJSON" -v && go build ./...`
Expected: PASS + erfolgreicher Build

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_chat_session.go internal/backend/app_chat_helpers.go internal/backend/app_chat_session_test.go internal/backend/app.go
git commit -m "feat(chat): add persistent stream-json ChatSession lifecycle"
```

---

### Task 3: Persistente Session in AddChatMessage verdrahten + Events dispatchen

Baut den Sprint-3-Einweg-Flow (`streamChatResponse` + `--print`) auf die persistente Session um, persistiert `session_id`/`permission_mode`/Tool-Events.

**Files:**
- Modify: `internal/backend/app_chat.go:107-140` (`AddChatMessage`), `:36-54` (`CreateConversation` um `permission_mode` erweitern), `:13-33` (Structs)
- Modify/Replace: `internal/backend/app_chat_stream.go` (alte `streamChatResponse`/`buildChatCommand`/`parseStreamDelta` entfernen; neue `dispatchChatEvent` + Emitter behalten)
- Modify: `internal/backend/app_chat_stream_test.go` (veraltete Tests für entfernte Funktionen löschen)

- [ ] **Step 1: Struct-Felder + neue Event-Emitter (Failing-Test)**

Test in `internal/backend/app_chat_events_test.go` ergänzen (prüft, dass Tool-Events als Message persistierbar sind):

```go
func TestChatEvent_ToolToMessage(t *testing.T) {
	ev := ChatEvent{Kind: ChatEventToolUse, ToolID: "t1", ToolName: "Bash"}
	m := chatEventToMessage(ev)
	if m == nil || m.Role != "tool" || m.ToolName != "Bash" {
		t.Fatalf("got %+v, want tool message Bash", m)
	}
	if chatEventToMessage(ChatEvent{Kind: ChatEventText, Text: "hi"}) != nil {
		t.Fatal("text events should not become standalone messages")
	}
}
```

- [ ] **Step 2: Fehlschlag bestätigen**

Run: `go test ./internal/backend/ -run TestChatEvent_ToolToMessage -v`
Expected: FAIL (undefined: chatEventToMessage)

- [ ] **Step 3: Implementierung**

3a. In `internal/backend/app_chat.go` die Structs erweitern (an `ChatMessage` und `Conversation`):

```go
// In ChatMessage ergänzen:
	ToolName   string `json:"tool_name,omitempty" yaml:"tool_name,omitempty"`
	ToolInput  string `json:"tool_input,omitempty" yaml:"tool_input,omitempty"`
	ToolResult string `json:"tool_result,omitempty" yaml:"tool_result,omitempty"`

// In Conversation ergänzen:
	SessionID      string `json:"session_id,omitempty" yaml:"session_id,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty" yaml:"permission_mode,omitempty"`
```

3b. `CreateConversation` um `permissionMode` erweitern (Signatur ändern):

```go
func (a *AppService) CreateConversation(provider, model, scope, permissionMode string) (Conversation, error) {
	if permissionMode == "" {
		permissionMode = "plan"
	}
	conv := Conversation{
		ID:             generateID(),
		Title:          "Neue Konversation",
		Provider:       provider,
		Model:          model,
		Scope:          scope,
		PermissionMode: permissionMode,
		CreatedAt:      time.Now().Format(time.RFC3339),
		UpdatedAt:      time.Now().Format(time.RFC3339),
		Messages:       []ChatMessage{},
	}
	if err := saveConversation(scope, conv); err != nil {
		return conv, fmt.Errorf("save conversation: %w", err)
	}
	return conv, nil
}
```

3c. `AddChatMessage` (ersetzt `go a.streamChatResponse(...)`):

```go
func (a *AppService) AddChatMessage(dir, convID, content string) error {
	conv, err := a.GetConversation(dir, convID)
	if err != nil {
		return fmt.Errorf("load conversation: %w", err)
	}
	msg := ChatMessage{ID: generateID(), Role: "user", Content: content, Timestamp: time.Now().Format(time.RFC3339)}
	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now().Format(time.RFC3339)
	if conv.Title == "Neue Konversation" && len(conv.Messages) == 1 {
		title := content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv.Title = title
	}
	if err := saveConversation(dir, conv); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}

	sess, err := a.ensureChatSession(dir, conv)
	if err != nil {
		a.emitChatError(convID, err.Error())
		return err
	}
	return sess.SendTurn(content)
}

// ensureChatSession returns the running session for conv, starting it if needed.
func (a *AppService) ensureChatSession(dir string, conv Conversation) (*ChatSession, error) {
	a.mu.Lock()
	if s, ok := a.chatSessions[conv.ID]; ok {
		a.mu.Unlock()
		return s, nil
	}
	a.mu.Unlock()

	sess, err := a.startChatProcess(conv.Scope, conv.Model, conv.PermissionMode, conv.SessionID)
	if err != nil {
		return nil, err
	}
	sess.ConvID = conv.ID
	a.mu.Lock()
	a.chatSessions[conv.ID] = sess
	a.mu.Unlock()
	return sess, nil
}

// CloseChatSession stops and forgets a chat session (called when a chat pane closes).
func (a *AppService) CloseChatSession(convID string) {
	a.mu.Lock()
	sess := a.chatSessions[convID]
	delete(a.chatSessions, convID)
	a.mu.Unlock()
	if sess != nil {
		sess.Close()
	}
}
```

3d. In `internal/backend/app_chat_stream.go`: alte `streamChatResponse`, `buildChatCommand`, `buildPrompt`, `parseStreamDelta`, `streamDelta` **entfernen**. `filterEnv` **behalten**. Neu hinzufügen — `dispatchChatEvent`, `chatEventToMessage` und drei Emitter (Stream/Tool/Done):

```go
// dispatchChatEvent maps a parsed ChatEvent to frontend events + persistence.
func (a *AppService) dispatchChatEvent(convID string, ev ChatEvent) {
	switch ev.Kind {
	case ChatEventInit:
		a.persistSessionID(convID, ev.SessionID)
		a.app.Event.Emit("chat:init", map[string]string{"conversationId": convID, "model": ev.Model})
	case ChatEventText:
		a.app.Event.Emit("chat:stream", map[string]string{"conversationId": convID, "delta": ev.Text})
	case ChatEventToolUse, ChatEventToolResult:
		if m := chatEventToMessage(ev); m != nil {
			a.appendMessage(convID, *m)
			a.app.Event.Emit("chat:tool", map[string]interface{}{"conversationId": convID, "message": m})
		}
	case ChatEventResult:
		a.flushAssistantMessage(convID, ev)
	}
}

// chatEventToMessage converts tool events into persistable messages; nil otherwise.
func chatEventToMessage(ev ChatEvent) *ChatMessage {
	switch ev.Kind {
	case ChatEventToolUse:
		return &ChatMessage{ID: generateID(), Role: "tool", ToolName: ev.ToolName, Timestamp: time.Now().Format(time.RFC3339)}
	case ChatEventToolResult:
		return &ChatMessage{ID: generateID(), Role: "tool", ToolResult: ev.ToolResult, Timestamp: time.Now().Format(time.RFC3339)}
	}
	return nil
}
```

> `persistSessionID`, `appendMessage`, `flushAssistantMessage` sind kleine Persistenz-Helfer. Lege sie in `internal/backend/app_chat_persist.go` an (hält Dateien < 300 Zeilen). `flushAssistantMessage` baut den gepufferten Assistant-Text (gehalten in einer `map[string]*strings.Builder` auf dem AppService, geschützt durch `a.mu`) zu einer Message zusammen, speichert sie und emittiert `chat:done`. Implementiere analog zu den vorhandenen `saveConversation`/`GetConversation`-Helfern.

3e. Veraltete Tests in `internal/backend/app_chat_stream_test.go` entfernen (`TestBuildPrompt_*`, `TestParseStreamDelta_*`); `TestFilterEnv_*` behalten.

- [ ] **Step 4: Tests + Build**

Run: `go test ./internal/backend/... && go build ./...`
Expected: PASS + Build ok. Erwartete Anpassungen: alle Aufrufer von `CreateConversation` an die neue 4-Argument-Signatur anpassen (s. Task 8).

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_chat.go internal/backend/app_chat_stream.go internal/backend/app_chat_persist.go internal/backend/app_chat_events.go internal/backend/app_chat_events_test.go internal/backend/app_chat_stream_test.go
git commit -m "feat(chat): drive conversations via persistent session; emit structured events"
```

---

## Phase 2 — Frontend: Store, Events, ChatPane

### Task 4: Chat-Store auf pro-Konversation-Streaming-State umstellen

Aktuell sind `streaming`/`streamBuffer` global → bricht bei mehreren Chat-Panes.

**Files:**
- Modify: `frontend/src/stores/chat.ts`

- [ ] **Step 1: Store-Shape ändern**

In `frontend/src/stores/chat.ts` `ChatStore` ersetzen:

```ts
export interface ConvStreamState { streaming: boolean; buffer: string; }

export interface ChatStore {
  conversations: Conversation[];
  activeConvId: string | null;
  loading: boolean;
  streams: Record<string, ConvStreamState>; // keyed by conversation id
  dir: string;
}

const initialStore: ChatStore = {
  conversations: [], activeConvId: null, loading: false, streams: {}, dir: '',
};
```

- [ ] **Step 2: Methoden auf `streams[convId]` umstellen**

`addUserMessage(convId, msg)`, `appendStream(convId, delta)`, `completeStream(convId, msg)`, `streamError(convId)` so umbauen, dass sie `s.streams[convId]` lesen/schreiben statt der globalen Felder. Beispiel `appendStream`:

```ts
appendStream(convId: string, delta: string) {
  update(s => {
    const prev = s.streams[convId] ?? { streaming: true, buffer: '' };
    return { ...s, streams: { ...s.streams, [convId]: { streaming: true, buffer: prev.buffer + delta } } };
  });
},
```

`addUserMessage` nimmt jetzt `convId` als ersten Parameter (statt implizit `activeConvId`). `completeStream` setzt `streams[convId] = { streaming: false, buffer: '' }`.

- [ ] **Step 3: Derived-Helfer ergänzen**

```ts
export const isStreaming = (convId: string | null) =>
  derived(chat, $c => (convId ? $c.streams[convId]?.streaming ?? false : false));
export const streamBuffer = (convId: string | null) =>
  derived(chat, $c => (convId ? $c.streams[convId]?.buffer ?? '' : ''));
```

- [ ] **Step 4: Build**

Run: `cd frontend && npm run build`
Expected: TypeScript compile ok (Aufrufer in ChatView/ChatPane folgen in Task 5/6).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/chat.ts
git commit -m "feat(chat): per-conversation streaming state in chat store"
```

---

### Task 5: Wails-Events abonnieren (die fehlende Verdrahtung)

**Files:**
- Create: `frontend/src/lib/chat-events.ts`

- [ ] **Step 1: Event-Bridge schreiben**

`frontend/src/lib/chat-events.ts`:

```ts
import { Events } from '@wailsio/runtime';
import { chat } from '../stores/chat';

// Subscribes to backend chat:* events. Call once on app mount; returns an unsubscribe fn.
export function subscribeChatEvents(): () => void {
  const offs: Array<() => void> = [];

  offs.push(Events.On('chat:stream', (e: any) => {
    const { conversationId, delta } = e.data ?? e;
    chat.appendStream(conversationId, delta);
  }));

  offs.push(Events.On('chat:tool', (e: any) => {
    const { conversationId, message } = e.data ?? e;
    chat.completeStream(conversationId, message); // tool card persisted as message
  }));

  offs.push(Events.On('chat:done', (e: any) => {
    const { conversationId, message } = e.data ?? e;
    chat.completeStream(conversationId, message);
  }));

  offs.push(Events.On('chat:error', (e: any) => {
    const { conversationId } = e.data ?? e;
    chat.streamError(conversationId);
  }));

  return () => offs.forEach(off => off());
}
```

> Prüfe den exakten Import (`@wailsio/runtime` Events-API) gegen vorhandene Event-Nutzung im Frontend (z.B. wie `terminal:output` abonniert wird) und passe `Events.On`-Signatur entsprechend an.

- [ ] **Step 2: In App.svelte einhängen**

In `frontend/src/App.svelte` `onMount` aufrufen und Cleanup in `onDestroy`:

```ts
import { subscribeChatEvents } from './lib/chat-events';
// in onMount:
const offChat = subscribeChatEvents();
// in onDestroy:
offChat?.();
```

- [ ] **Step 3: Build**

Run: `cd frontend && npm run build`
Expected: compile ok

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/chat-events.ts frontend/src/App.svelte
git commit -m "feat(chat): subscribe to backend chat stream events"
```

---

### Task 6: ChatPane-Komponente

Eine Pane-Variante, die genau EINE Konversation rendert (Bubbles + ChatInput), für die Grid-Einbettung. Wiederverwendung von `ChatMessage`/`ChatInput`.

**Files:**
- Create: `frontend/src/components/ChatPane.svelte`

- [ ] **Step 1: Komponente schreiben**

`frontend/src/components/ChatPane.svelte`:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import * as App from '../../wailsjs/go/backend/App';
  import ChatMessage from './ChatMessage.svelte';
  import ChatInput from './ChatInput.svelte';
  import { chat, isStreaming, streamBuffer } from '../stores/chat';
  import { config } from '../stores/config';

  export let conversationId: string;
  export let dir = '';

  let messagesEl: HTMLDivElement;
  $: streaming = isStreaming(conversationId);
  $: buffer = streamBuffer(conversationId);
  $: conv = $chat.conversations.find(c => c.id === conversationId) ?? null;
  $: chatStyle = $config?.chat_style ?? 'claude-code';

  onMount(() => {
    if (dir) App.GetConversations(dir).then(cs => chat.setConversations(cs || []));
  });

  async function handleSend(e: CustomEvent<{ content: string }>) {
    if (!conv) return;
    chat.addUserMessage(conversationId, {
      id: Date.now().toString(), role: 'user', content: e.detail.content,
      timestamp: new Date().toISOString(), cost: '', tokens: 0,
    });
    scrollToBottom();
    try { await App.AddChatMessage(dir, conversationId, e.detail.content); }
    catch (err) { console.error('[chat] send error:', err); chat.streamError(conversationId); }
  }

  function scrollToBottom() {
    setTimeout(() => { if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight; }, 50);
  }
  $: if ($buffer) scrollToBottom();
</script>

<div class="chat-pane" data-style={chatStyle}>
  <div class="chat-messages" bind:this={messagesEl}>
    {#if conv}
      {#each conv.messages as msg (msg.id)}
        <ChatMessage message={msg} />
      {/each}
      {#if $streaming && $buffer}
        <ChatMessage
          message={{ id: 'stream', role: 'assistant', content: '', timestamp: new Date().toISOString(), cost: '', tokens: 0 }}
          isStreaming={true} streamContent={$buffer} />
      {/if}
      {#if conv.messages.length === 0 && !$streaming}
        <div class="chat-welcome">Starte die Konversation mit einer Nachricht</div>
      {/if}
    {/if}
  </div>
  <ChatInput
    disabled={$streaming}
    placeholder={$streaming ? 'Antwort wird generiert...' : 'Nachricht eingeben...'}
    on:send={handleSend} />
</div>

<style>
  .chat-pane { display: flex; flex-direction: column; height: 100%; min-width: 0; background: var(--bg, #11111b); }
  .chat-messages { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 10px; }
  .chat-welcome { margin: auto; color: var(--fg-muted, #a6adc8); opacity: 0.5; font-size: 0.85rem; }

  /* Telegram-Stil: User rechts mit kräftiger Bubble, Assistant links, höhere Dichte */
  .chat-pane[data-style="telegram"] .chat-messages { gap: 6px; }
  /* Claude-Code-Stil: linksbündig, ruhiger (Default via ChatMessage-CSS) */
</style>
```

> Die feineren Stil-Unterschiede leben in `ChatMessage.svelte` über das `data-style`-Attribut am Container; die konkreten CSS-Regeln werden in Task 10 ergänzt.

- [ ] **Step 2: Build**

Run: `cd frontend && npm run build`
Expected: compile ok

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ChatPane.svelte
git commit -m "feat(chat): add ChatPane component for in-grid chat rendering"
```

---

### Task 7: Pane-Modell `display`-Feld + PaneGrid-Branch

**Files:**
- Modify: `frontend/src/stores/tabs.ts:5-23` (Interface), `:130-160` (`addPane`)
- Modify: `frontend/src/components/PaneGrid.svelte`

- [ ] **Step 1: Interface + addPane erweitern**

In `frontend/src/stores/tabs.ts` `Pane` ergänzen:

```ts
  display: 'terminal' | 'chat';
  conversationId: string;
```

`addPane`-Signatur um zwei optionale Parameter erweitern und im `panes.push({...})` setzen:

```ts
addPane(tabId: string, sessionId: number, name: string, mode: PaneMode, model: string, issueNumber?: number | null, issueTitle?: string, issueBranch?: string, worktreePath?: string, branch?: string, background?: boolean, display: 'terminal' | 'chat' = 'terminal', conversationId = ''): string {
  // ... im push():
  //   display,
  //   conversationId,
```

- [ ] **Step 2: PaneGrid branchen**

In `frontend/src/components/PaneGrid.svelte` Import + bedingtes Rendering:

```svelte
import ChatPane from './ChatPane.svelte';
```

Den `{#each visiblePanes as pane (pane.id)}`-Block so erweitern:

```svelte
{#each visiblePanes as pane (pane.id)}
  {#if pane.display === 'chat'}
    <div class="pane-chat-wrapper">
      <ChatPane conversationId={pane.conversationId} dir={tabDir} />
    </div>
  {:else}
    <TerminalPane {pane} {active} {tabId} {worktrees} {tabDir}
      paneIndex={panes.indexOf(pane) + 1}
      on:close={handleClose} on:maximize={handleMaximize} on:focus={handleFocus}
      on:rename={handleRename} on:restart={handleRestart} on:issueAction={handleIssueAction}
      on:commitPush={handleCommitPush} on:navigateFile={handleNavigateFile}
      on:splitPane={handleSplitPane} on:openWorktreePane on:worktreeListChanged />
  {/if}
{/each}
```

CSS ergänzen: `.pane-chat-wrapper { display: flex; min-width: 0; overflow: hidden; border: 1px solid var(--border); border-radius: 6px; }`

- [ ] **Step 3: Build**

Run: `cd frontend && npm run build`
Expected: compile ok

- [ ] **Step 4: Commit**

```bash
git add frontend/src/stores/tabs.ts frontend/src/components/PaneGrid.svelte
git commit -m "feat(chat): add pane display mode and render ChatPane in grid"
```

---

### Task 8: LaunchDialog-Option + App.svelte-Verdrahtung

**Files:**
- Modify: `frontend/src/components/LaunchDialog.svelte` (Anzeige-Auswahl + Permission-Mode)
- Modify: `frontend/src/App.svelte` (`handleLaunch`: bei `display==='chat'` Konversation anlegen statt PTY-Session)
- Modify: alle übrigen `CreateConversation`-Aufrufer (z.B. `ChatView.svelte`) an die 4-Argument-Signatur anpassen

- [ ] **Step 1: LaunchDialog erweitern**

In `LaunchDialog.svelte` zwei Felder ergänzen und im Dispatch mitschicken:
- Radiogruppe **Anzeige**: `Terminal` | `Chat` (nur sichtbar, wenn `type` ein Agent ist: claude/codex/gemini)
- Dropdown **Permission-Mode** (nur sichtbar bei `display==='chat'`): `plan` | `acceptEdits` | `bypassPermissions`

Dispatch-Detail erweitern: `{ type, model, issue, display, permissionMode }`.

- [ ] **Step 2: handleLaunch in App.svelte verzweigen**

In `frontend/src/App.svelte` `handleLaunch` am Anfang:

```ts
const { type, model, issue, display = 'terminal', permissionMode = 'plan' } = e.detail;
if (display === 'chat') {
  const provider = type.startsWith('codex') ? 'codex' : type.startsWith('gemini') ? 'gemini' : 'claude';
  const conv = await App.CreateConversation(provider, model, tab.dir || '', permissionMode);
  const name = getClaudeName(type, model);
  tabStore.addPane(tab.id, 0, name, type, model, null, '', '', '', '', false, 'chat', conv.id);
  workspace.setView('terminals');
  return;
}
// ... bestehender PTY-Pfad unverändert ...
```

> `CreateConversation`-Binding in `wailsjs/go/backend/App` muss die neue Signatur (4 Args) widerspiegeln — nach `go build` der Wails-Bindings prüfen/manuell angleichen.

- [ ] **Step 3: Bestehende Aufrufer fixen**

`ChatView.svelte:48` (`App.CreateConversation(newProvider, newModel, dir)`) → vierten Parameter `'plan'` ergänzen, damit es kompiliert.

- [ ] **Step 4: Build (Frontend + Backend)**

Run: `cd frontend && npm run build` und `go build ./...`
Expected: beide ok

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/LaunchDialog.svelte frontend/src/App.svelte frontend/src/components/ChatView.svelte
git commit -m "feat(chat): launch dialog chat display + permission mode wiring"
```

---

### Task 9: Titelleisten-Toggle Terminal↔Chat (Relaunch)

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte` (Toggle-Button), `frontend/src/App.svelte` (Handler)

- [ ] **Step 1: Button + Event**

In `PaneTitlebar.svelte` einen Button ergänzen, der ein `toggleDisplay`-Event mit `{ paneId }` dispatcht (nur für Agent-Panes anzeigen). Icon: `>_` ↔ `💬`.

- [ ] **Step 2: Handler in App.svelte**

`toggleDisplay` behandeln: aktuelles Pane finden; bei Wechsel auf `chat` eine Konversation anlegen (wie Task 8) und das Pane ersetzen; bei Wechsel auf `terminal` eine PTY-Session starten. Da das ein anderer Backend-Prozess ist: altes Backend sauber schließen (`App.CloseChatSession(convId)` bzw. `App.CloseSession(sessionId)`), Pane neu aufbauen. Dem Nutzer ist klar: kein verlustfreies Live-Umschalten.

- [ ] **Step 3: Build**

Run: `cd frontend && npm run build`
Expected: ok

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/PaneTitlebar.svelte frontend/src/App.svelte
git commit -m "feat(chat): pane titlebar toggle between terminal and chat"
```

---

## Phase 3 — Theme / Settings

### Task 10: `chat_style`-Config + Wails-Sync + Settings-Dropdown + CSS-Varianten

**Files:**
- Modify: `internal/config/config.go:15-49` (Feld + Default), `frontend/wailsjs/go/models.ts:706-756` (Klasse + Konstruktor — **kritischer wiederkehrender Bug**)
- Modify: `frontend/src/components/SettingsDialog.svelte` (Dropdown), `frontend/src/components/ChatMessage.svelte` (Stil-CSS)

- [ ] **Step 1: Go-Config-Feld**

In `config.go` `Config` ergänzen:

```go
	ChatStyle             string         `yaml:"chat_style" json:"chat_style"`
```

Default beim Schreiben der Default-Config: `"claude-code"`.

- [ ] **Step 2: models.ts synchronisieren (Pflicht!)**

In `frontend/wailsjs/go/models.ts` `class Config` Feld-Deklaration ergänzen:

```ts
    chat_style: string;
```

und im Konstruktor:

```ts
        this.chat_style = source["chat_style"];
```

> Ohne diesen Schritt strippt Wails das Feld stillschweigend → Frontend sieht immer `undefined` (dokumentierter Recurring-Bug, CLAUDE.md).

- [ ] **Step 3: SettingsDialog-Dropdown**

In `SettingsDialog.svelte` ein Dropdown "Chat-Stil" mit Optionen `Claude Code` (`claude-code`) / `Telegram` (`telegram`). **Achtung Svelte-Reaktivitätsfalle (CLAUDE.md):** Zuweisung NICHT in `$: { ... }` — Init via Funktion `$: if (visible) initDialog();`. Speichern über den vorhandenen Config-Save-Pfad.

- [ ] **Step 4: CSS-Varianten in ChatMessage**

In `ChatMessage.svelte` Stil-Regeln, die über das `[data-style]`-Attribut des Eltern-Containers (`ChatPane`) greifen. Beispiel:

```css
:global(.chat-pane[data-style="telegram"]) .chat-message.user {
  background: var(--accent, #39ff14); color: #000; border: none;
}
:global(.chat-pane[data-style="telegram"]) .chat-message { max-width: 75%; border-radius: 14px; }
:global(.chat-pane[data-style="claude-code"]) .chat-message { max-width: 100%; align-self: stretch; }
```

- [ ] **Step 5: Build (beide)**

Run: `go build ./... && cd frontend && npm run build`
Expected: beide ok

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go frontend/wailsjs/go/models.ts frontend/src/components/SettingsDialog.svelte frontend/src/components/ChatMessage.svelte
git commit -m "feat(chat): chat-style setting (claude-code/telegram) with wails sync"
```

---

## Phase 4 — Verifikation

### Task 11: Voll-Build + E2E-Smoke-Test

- [ ] **Step 1: Komplett-Build**

Run (gemäß MEMORY.md Build-Pfad für alpha-main/Wails v3):
```bash
cd frontend && npm run build && cd .. && go build -o build/bin/multiterminal.exe -tags desktop .
```
Expected: erfolgreicher Build, keine Fehler.

- [ ] **Step 2: Alle Go-Tests**

Run: `go test ./... && go vet ./...`
Expected: PASS

- [ ] **Step 3: Manuelle E2E-Checkliste** (echter `claude` CLI, echtes Repo)

  1. App starten → neues Pane → LaunchDialog: Agent „Claude", Anzeige „Chat", Permission „plan" → Erstellen.
  2. Nachricht senden → Text streamt als Assistant-Bubble; Eingabezeile fix unten.
  3. Folge-Nachricht senden → Kontext erhalten (Multi-Turn), keine „(Keine Antwort)".
  4. Zweites Chat-Pane parallel öffnen → beide streamen unabhängig (Store-Fix verifizieren).
  5. Titelleisten-Toggle Chat→Terminal→Chat → Relaunch funktioniert.
  6. Settings → Chat-Stil „Telegram" → Bubbles ändern Optik sofort.
  7. Chat-Pane in eigenem Tab → Tab in eigenes Fenster detachen → Chat läuft im neuen Fenster weiter.
  8. App neu starten → Konversation via `--resume` wieder ansprechbar (Kontext erhalten).

- [ ] **Step 4: Issue/Commit-Disziplin**

Falls ein Tracking-Issue existiert: Abschluss-Commit mit `Closes #N`. Andernfalls Branch mit `needs-e2e-testing` taggen, bis Schritt 3 real bestätigt ist.

---

## Self-Review (vom Plan-Autor)

**Spec-Abdeckung:** Backend persistente Session (Task 1–3) ✓; Pane-display-Modell (Task 7) ✓; Event-Mapping inkl. Tool/Result/Kosten (Task 1, 3, 5) ✓; Permission-Mode pro Konversation (Task 2, 3, 8) ✓; Theme-Platzierung Modus≠Farb-Theme (Task 8, 9) + Chat-Stil-Dropdown (Task 10) ✓; Store-Fix pro Konversation (Task 4) ✓; Detach via Tab (Task 11/E2E, nutzt bestehende Infrastruktur — kein Code) ✓; Persistenz `.mtui/chat` + `session_id`/`permission_mode` (Task 3) ✓; Gotchas: NDJSON-Chunking (Task 2 `scanNDJSON`), Windows `.cmd`/`CLAUDECODE` (Task 2 `wrapClaudeCmd`/`filterEnv`), Wails-`models.ts`-Sync (Task 10), Svelte-`$:`-Bug (Task 10) ✓.

**Offene Annahme (beim Ausführen verifizieren):** Exakte `@wailsio/runtime` Events-API (Task 5) und die generierte `CreateConversation`-Binding-Signatur (Task 8) gegen den realen Code abgleichen — Bindings werden in diesem Projekt manuell gepflegt.
