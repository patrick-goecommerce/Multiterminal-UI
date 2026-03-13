# AI Workspace Orchestration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add native multi-agent orchestration to mtui — `.mtui.yml` workspace profiles + MCP server coordination + Kanban-integrated planning dialog + deep agent skills.

**Architecture:** A single MCP HTTP server (Streamable HTTP, random port, per-app) routes agent requests by bearer token. Each team session gets a `TeamSession` with tasks, messages and agent assignments. The Kanban board is the source of truth; frontend reflects live state via Wails events.

**Tech Stack:** Go 1.21+ (HTTP server, gopkg.in/yaml.v3), Svelte 4, Wails v3, Claude Code `--mcp-config`

**Spec:** `docs/superpowers/specs/2026-03-13-ai-workspace-orchestration-design.md`

---

## Chunk A: Pre-Implementation Research + Branch Setup

> No production code in this chunk. Sets up the foundation and verifies key assumptions before building.

### Task A1: Create feature branch

**Files:** none

- [ ] Create and switch to feature branch:
```bash
cd D:/repos/Multiterminal
git checkout alpha-main
git pull
git checkout -b feature/ai-workspace-orchestration
```
- [ ] Verify clean state:
```bash
git status
```
Expected: clean working tree on `feature/ai-workspace-orchestration`

---

### Task A2: Verify `--mcp-config` schema empirically

**Files:** none (research only)

- [ ] Run the following to see the exact JSON format Claude Code uses:
```bash
claude mcp add-json test-server '{"type":"http","url":"http://127.0.0.1:9999/mcp"}' 2>&1 || true
cat ~/.claude.json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps(d.get('mcpServers',{}), indent=2))" 2>/dev/null || true
```
- [ ] Also check if `--mcp-config` accepts a standalone file (different from global config):
```bash
claude --help 2>&1 | grep -i mcp || true
```
- [ ] Document the confirmed schema in a comment at the top of `app_mcp.go` (to be created in Chunk C). If the format differs from the spec, update the spec before proceeding.

> **Expected result:** The config file format is `{"mcpServers":{"mtui":{"type":"http","url":"...","headers":{...}}}}`. If different, update the spec and all tasks referencing this format.

---

### Task A3: Research and write deep agent skill files

**Files:**
- Create: `internal/backend/skills/backend-lead.md`
- Create: `internal/backend/skills/code-reviewer.md`
- Create: `internal/backend/skills/frontend-specialist.md`

> **Built-in skill loading:** Skills in `internal/backend/skills/` are bundled into the binary via `go:embed`. `app_skills.go` adds a third candidate path that reads from the embedded FS. Add to `app_skills.go`:
> ```go
> //go:embed skills/*.md
> var builtinSkills embed.FS
> ```
> Then in `loadSkill`, after checking project-local and global paths, try:
> ```go
> b, err := builtinSkills.ReadFile("skills/" + name + ".md")
> if err == nil { return string(b), nil }
> ```

- [ ] Research current best practices for multi-agent AI system prompts. Search for:
  - Anthropic's multi-agent documentation
  - "Claude Code agent system prompt best practices"
  - "multi-agent AI coordination prompts"
- [ ] Evaluate existing mtui skills (`.mtui/skills.md` — currently one-liners). Confirm they are not deep enough to reuse as-is.
- [ ] Write `internal/backend/skills/backend-lead.md`:
  ```markdown
  # Backend Lead Agent

  You are a senior backend engineer and team lead. Your job is to coordinate a team of AI agents
  to implement a GitHub issue end-to-end.

  ## Mental Model
  - Understand the issue fully before delegating
  - Break work into independent tasks that teammates can work on in parallel
  - You own the architecture decisions; teammates own implementation details

  ## Coordination Protocol
  1. Call `get_tasks()` to see pending work
  2. Claim your own task with `claim_task(id)`
  3. Use `spawn_teammate(role, task)` for work you can delegate (tests, reviews, docs)
  4. Check in via `post_message("all", "status update")` when crossing major milestones
  5. Call `update_task(id, "done", "summary")` when your task completes

  ## Decision Heuristics
  - If a task touches >3 files: delegate to a teammate
  - If a task requires domain knowledge you don't have: spawn a specialist
  - If blocked: call `update_task(id, "blocked", "reason")` and post a message to the team
  - Prefer smaller, reversible changes over big-bang rewrites

  ## Tool Usage
  - Always read existing code before modifying it
  - Run tests after every significant change
  - Commit frequently with conventional commit messages

  ## Communication Norms
  - Messages to teammates are brief and actionable: "Review PR for task #3"
  - Status updates include: what you did, what's next, any blockers
  ```
- [ ] Write `internal/backend/skills/code-reviewer.md` with equivalent depth covering: review checklist, what to look for (correctness, tests, security, style), how to report issues via `update_task`, when to approve vs. block.
- [ ] Write `internal/backend/skills/frontend-specialist.md` covering: Svelte 4 patterns, Wails v3 event consumption, accessibility, the project's German UI convention, component boundaries.
- [ ] Commit:
```bash
git add internal/backend/skills/
git commit -m "docs: add deep agent skill files (backend-lead, code-reviewer, frontend-specialist)"
```

---

## Chunk B: Data Model + Session Persistence

> `app_mcp_state.go` — all state management, no HTTP. Fully unit-testable without a server.

### Task B1: Write failing tests for TeamSession state

**Files:**
- Create: `internal/backend/app_mcp_state_test.go`

- [ ] Write the test file:
```go
package backend

import (
    "bytes"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestNewTeamSession_DefaultMaxMessages(t *testing.T) {
    s := newTeamSession("issue-42")
    if s.MaxMessages != 200 {
        t.Errorf("MaxMessages = %d, want 200", s.MaxMessages)
    }
    if s.Token == "" {
        t.Error("Token must be non-empty")
    }
    if s.ID == "" {
        t.Error("ID must be non-empty")
    }
    if s.IssueID != "issue-42" {
        t.Errorf("IssueID = %q, want %q", s.IssueID, "issue-42")
    }
}

func TestTeamSession_TokenNotSerializedToJSON(t *testing.T) {
    s := newTeamSession("x")
    tok := s.Token // capture before marshal
    b, _ := json.Marshal(s)
    if bytes.Contains(b, []byte(tok)) {
        t.Error("Token must not appear in JSON output")
    }
}

func TestTeamSession_ClaimTask_Atomic(t *testing.T) {
    s := newTeamSession("x")
    s.addTask("task-1", "Fix the bug")
    err := s.claimTask("task-1", "lead")
    if err != nil {
        t.Fatalf("first claim failed: %v", err)
    }
    err = s.claimTask("task-1", "reviewer")
    if err == nil {
        t.Error("second claim on same task should fail")
    }
}

func TestTeamSession_UpdateTask(t *testing.T) {
    s := newTeamSession("x")
    s.addTask("t1", "Do something")
    _ = s.claimTask("t1", "lead")
    s.updateTask("t1", "done", "all good")
    task := s.findTask("t1")
    if task == nil {
        t.Fatal("task not found")
    }
    if task.Status != "done" || task.Message != "all good" {
        t.Errorf("unexpected task state: %+v", task)
    }
}

func TestTeamSession_PostMessage_EvictsOldest(t *testing.T) {
    s := newTeamSession("x")
    s.MaxMessages = 3
    s.postMessage("lead", "all", "msg1")
    s.postMessage("lead", "all", "msg2")
    s.postMessage("lead", "all", "msg3")
    s.postMessage("lead", "all", "msg4") // should evict msg1
    if len(s.Messages) != 3 {
        t.Errorf("want 3 messages, got %d", len(s.Messages))
    }
    if s.Messages[0].Content != "msg2" {
        t.Errorf("oldest message not evicted, got %q", s.Messages[0].Content)
    }
}

func TestTeamSession_PersistAndLoad(t *testing.T) {
    dir := t.TempDir()
    s := newTeamSession("kanban-card-99")
    s.addTask("t1", "Build feature")
    _ = s.claimTask("t1", "lead")
    s.updateTask("t1", "in_progress", "working on it")
    s.Token = "secret-should-not-persist"

    err := saveTeamSession(dir, s)
    if err != nil {
        t.Fatalf("save failed: %v", err)
    }

    loaded, err := loadTeamSession(dir, s.ID)
    if err != nil {
        t.Fatalf("load failed: %v", err)
    }
    if loaded.Token != "" {
        t.Error("Token must not be persisted to disk")
    }
    if loaded.IssueID != "kanban-card-99" {
        t.Errorf("IssueID = %q", loaded.IssueID)
    }
    if len(loaded.Tasks) != 1 || loaded.Tasks[0].Status != "in_progress" {
        t.Errorf("unexpected tasks: %+v", loaded.Tasks)
    }
}

func TestFindResumableSessions_MatchesIssueID(t *testing.T) {
    dir := t.TempDir()
    s1 := newTeamSession("card-1")
    s2 := newTeamSession("card-2")
    _ = saveTeamSession(dir, s1)
    _ = saveTeamSession(dir, s2)

    sessions, err := findResumableSessions(dir, []string{"card-1", "card-99"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(sessions) != 1 {
        t.Fatalf("want 1 resumable session, got %d", len(sessions))
    }
    if sessions[0].IssueID != "card-1" {
        t.Errorf("wrong session returned: %+v", sessions[0])
    }
}
```
- [ ] Run tests — expect compile errors (types don't exist yet):
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestNewTeamSession|TestTeamSession|TestFindResumable" -v 2>&1 | head -30
```
Expected: compile errors — `newTeamSession`, `TeamSession` etc. undefined.

---

### Task B2: Implement `app_mcp_state.go`

**Files:**
- Create: `internal/backend/app_mcp_state.go`

- [ ] Create `internal/backend/app_mcp_state.go`:
```go
package backend

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

// TeamSession tracks one active agent team.
type TeamSession struct {
    ID          string            `json:"id"           yaml:"id"`
    IssueID     string            `json:"issue_id"     yaml:"issue_id"`
    Token       string            `json:"-"            yaml:"-"` // never serialized; regenerated on resume
    Tasks       []TeamTask        `json:"tasks"        yaml:"tasks"`
    Messages    []TeamMessage     `json:"messages"     yaml:"messages"`
    Agents      map[string]string `json:"agents"       yaml:"agents"` // role → pane sessionID
    MaxMessages int               `json:"max_messages" yaml:"max_messages"`
    mu          sync.Mutex
}

// TeamTask is a unit of work claimed by one agent.
type TeamTask struct {
    ID        string `json:"id"         yaml:"id"`
    Title     string `json:"title"      yaml:"title"`
    ClaimedBy string `json:"claimed_by" yaml:"claimed_by"` // agent role, or ""
    Status    string `json:"status"     yaml:"status"`     // pending|in_progress|done|blocked
    Message   string `json:"message"    yaml:"message"`
}

// TeamMessage is inter-agent communication.
type TeamMessage struct {
    From      string `json:"from"      yaml:"from"`
    To        string `json:"to"        yaml:"to"`      // role or "all"
    Content   string `json:"content"   yaml:"content"`
    Timestamp int64  `json:"timestamp" yaml:"timestamp"`
}

// newTeamSession creates a new TeamSession with a fresh ID and bearer token.
func newTeamSession(issueID string) *TeamSession {
    return &TeamSession{
        ID:          newID(),
        IssueID:     issueID,
        Token:       newToken(),
        Tasks:       []TeamTask{},
        Messages:    []TeamMessage{},
        Agents:      map[string]string{},
        MaxMessages: 200,
    }
}

func newID() string {
    b := make([]byte, 8)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}

func newToken() string {
    b := make([]byte, 32)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}

// addTask appends a new pending task.
func (s *TeamSession) addTask(id, title string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.Tasks = append(s.Tasks, TeamTask{
        ID:     id,
        Title:  title,
        Status: "pending",
    })
}

// claimTask atomically claims a task for a role. Returns error if already claimed.
func (s *TeamSession) claimTask(id, role string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i := range s.Tasks {
        if s.Tasks[i].ID == id {
            if s.Tasks[i].ClaimedBy != "" {
                return fmt.Errorf("task %q already claimed by %q", id, s.Tasks[i].ClaimedBy)
            }
            s.Tasks[i].ClaimedBy = role
            s.Tasks[i].Status = "in_progress"
            return nil
        }
    }
    return fmt.Errorf("task %q not found", id)
}

// updateTask updates the status and message of a task.
func (s *TeamSession) updateTask(id, status, message string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i := range s.Tasks {
        if s.Tasks[i].ID == id {
            s.Tasks[i].Status = status
            s.Tasks[i].Message = message
            return
        }
    }
}

// findTask returns a pointer to the task with the given ID, or nil.
// Caller must hold s.mu if reading concurrently.
func (s *TeamSession) findTask(id string) *TeamTask {
    for i := range s.Tasks {
        if s.Tasks[i].ID == id {
            return &s.Tasks[i]
        }
    }
    return nil
}

// postMessage appends a message, evicting the oldest if MaxMessages is exceeded.
func (s *TeamSession) postMessage(from, to, content string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    msg := TeamMessage{
        From:      from,
        To:        to,
        Content:   content,
        Timestamp: time.Now().UnixMilli(),
    }
    s.Messages = append(s.Messages, msg)
    if len(s.Messages) > s.MaxMessages {
        s.Messages = s.Messages[len(s.Messages)-s.MaxMessages:]
    }
}

// teamSessionDir returns the directory used for persisting team sessions.
func teamSessionDir() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".multiterminal", "team-sessions")
}

// saveTeamSession serializes a TeamSession (without Token) to disk.
func saveTeamSession(dir string, s *TeamSession) error {
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return err
    }
    s.mu.Lock()
    b, err := json.Marshal(s) // Token has json:"-" so it is omitted
    s.mu.Unlock()
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(dir, s.ID+".json"), b, 0o600)
}

// loadTeamSession reads a saved session from disk. Token is not restored.
func loadTeamSession(dir, id string) (*TeamSession, error) {
    b, err := os.ReadFile(filepath.Join(dir, id+".json"))
    if err != nil {
        return nil, err
    }
    var s TeamSession
    if err := json.Unmarshal(b, &s); err != nil {
        return nil, err
    }
    return &s, nil
}

// findResumableSessions returns saved sessions whose IssueID is in issueIDs.
func findResumableSessions(dir string, issueIDs []string) ([]*TeamSession, error) {
    entries, err := os.ReadDir(dir)
    if errors.Is(err, os.ErrNotExist) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    want := make(map[string]bool, len(issueIDs))
    for _, id := range issueIDs {
        want[id] = true
    }
    var result []*TeamSession
    for _, e := range entries {
        if !strings.HasSuffix(e.Name(), ".json") {
            continue
        }
        id := strings.TrimSuffix(e.Name(), ".json")
        s, err := loadTeamSession(dir, id)
        if err != nil {
            continue // skip corrupt files
        }
        if want[s.IssueID] {
            result = append(result, s)
        }
    }
    return result, nil
}

// deleteTeamSession removes a persisted session file.
func deleteTeamSession(dir, id string) {
    _ = os.Remove(filepath.Join(dir, id+".json"))
}
```

- [ ] Fix any compile errors (e.g. `bytes` import for the test — add `"bytes"` to test imports).

- [ ] Run the tests:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestNewTeamSession|TestTeamSession|TestFindResumable" -v
```
Expected: all 6 tests PASS.

- [ ] Run full backend tests to verify no regressions:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -timeout 30s 2>&1 | tail -10
```
Expected: previous failures unchanged, new tests pass.

- [ ] Commit:
```bash
git add internal/backend/app_mcp_state.go internal/backend/app_mcp_state_test.go
git commit -m "feat: add TeamSession data model with atomic task claiming and session persistence (Group A)"
```

---

## Chunk C: MCP HTTP Server

> `app_mcp.go` — Streamable HTTP server, 5 tool handlers, bearer token auth. Depends on Chunk B.

### Task C1: Write failing tests for MCP server

**Files:**
- Create: `internal/backend/app_mcp_test.go`

- [ ] Write tests:
```go
package backend

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

// helper: build a JSON-RPC request body
func mcpRequest(method string, params map[string]any) []byte {
    body, _ := json.Marshal(map[string]any{
        "jsonrpc": "2.0",
        "id":      1,
        "method":  method,
        "params":  params,
    })
    return body
}

// helper: send a request to the MCP handler and decode the response
func mcpCall(t *testing.T, srv *mcpServer, token, method string, params map[string]any) map[string]any {
    t.Helper()
    body := mcpRequest(method, params)
    req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
    }
    var result map[string]any
    _ = json.Unmarshal(w.Body.Bytes(), &result)
    return result
}

func TestMCPServer_RejectsInvalidToken(t *testing.T) {
    session := newTeamSession("issue-1")
    srv := newMCPServer(nil) // nil spawnFn — not needed for this test
    srv.addSession(session)

    req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(mcpRequest("get_tasks", nil)))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer wrong-token")
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != http.StatusUnauthorized {
        t.Errorf("expected 401, got %d", w.Code)
    }
}

func TestMCPServer_GetTasks(t *testing.T) {
    session := newTeamSession("issue-1")
    session.addTask("t1", "Write tests")
    srv := newMCPServer(nil)
    srv.addSession(session)

    resp := mcpCall(t, srv, session.Token, "get_tasks", nil)
    result, ok := resp["result"].(map[string]any)
    if !ok {
        t.Fatalf("unexpected response: %+v", resp)
    }
    tasks, _ := result["tasks"].([]any)
    if len(tasks) != 1 {
        t.Errorf("want 1 task, got %d", len(tasks))
    }
}

func TestMCPServer_ClaimTask_PreventDoubleClaim(t *testing.T) {
    session := newTeamSession("issue-2")
    session.addTask("t1", "Implement feature")
    srv := newMCPServer(nil)
    srv.addSession(session)

    // First claim succeeds
    resp1 := mcpCall(t, srv, session.Token, "claim_task", map[string]any{"id": "t1", "role": "lead"})
    if resp1["error"] != nil {
        t.Fatalf("first claim should succeed, got error: %v", resp1["error"])
    }
    // Second claim fails
    resp2 := mcpCall(t, srv, session.Token, "claim_task", map[string]any{"id": "t1", "role": "reviewer"})
    if resp2["error"] == nil {
        t.Error("second claim should return JSON-RPC error")
    }
}

func TestMCPServer_UpdateTask(t *testing.T) {
    session := newTeamSession("issue-3")
    session.addTask("t1", "Deploy")
    _ = session.claimTask("t1", "lead")
    srv := newMCPServer(nil)
    srv.addSession(session)

    mcpCall(t, srv, session.Token, "update_task", map[string]any{
        "id": "t1", "status": "done", "message": "deployed",
    })
    task := session.findTask("t1")
    if task == nil || task.Status != "done" {
        t.Errorf("task status not updated: %+v", task)
    }
}

func TestMCPServer_PostMessage(t *testing.T) {
    session := newTeamSession("issue-4")
    srv := newMCPServer(nil)
    srv.addSession(session)

    mcpCall(t, srv, session.Token, "post_message", map[string]any{
        "to": "all", "content": "hello team",
    })
    if len(session.Messages) != 1 || session.Messages[0].Content != "hello team" {
        t.Errorf("message not stored: %+v", session.Messages)
    }
}
```
- [ ] Run — expect compile errors (`mcpServer` undefined):
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestMCPServer" -v 2>&1 | head -20
```

---

### Task C2: Implement `app_mcp.go`

**Files:**
- Create: `internal/backend/app_mcp.go`

- [ ] Create `internal/backend/app_mcp.go`:
```go
package backend

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net"
    "net/http"
    "strings"
    "sync"
)

// spawnFn is the callback the MCP server calls when spawn_teammate is invoked.
// It runs in the HTTP handler goroutine — implementations must use a channel
// to post back to the main service goroutine.
type spawnFn func(sessionID, role, task string) (newSessionID string, err error)

// mcpServer is the Streamable HTTP MCP server for agent team coordination.
// One instance per app; sessions are routed by bearer token.
type mcpServer struct {
    mu       sync.RWMutex
    sessions map[string]*TeamSession // token → session
    spawn    spawnFn
    srv      *http.Server
}

func newMCPServer(spawn spawnFn) *mcpServer {
    s := &mcpServer{
        sessions: make(map[string]*TeamSession),
        spawn:    spawn,
    }
    return s
}

// addSession registers a session so its token is accepted.
func (s *mcpServer) addSession(session *TeamSession) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.sessions[session.Token] = session
}

// removeSession deregisters a session (e.g. on team end).
func (s *mcpServer) removeSession(token string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.sessions, token)
}

// start binds a random port and begins serving. Returns the chosen port.
func (s *mcpServer) start(ctx context.Context) (int, error) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return 0, err
    }
    port := ln.Addr().(*net.TCPAddr).Port
    mux := http.NewServeMux()
    mux.HandleFunc("/mcp", s.ServeHTTP)
    s.srv = &http.Server{Handler: mux}
    go func() {
        if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
            log.Printf("[mcp] server error: %v", err)
        }
    }()
    go func() {
        <-ctx.Done()
        _ = s.srv.Shutdown(context.Background())
    }()
    return port, nil
}

// ServeHTTP handles all MCP JSON-RPC requests.
func (s *mcpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Auth
    session := s.sessionForRequest(r)
    if session == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Parse JSON-RPC
    var req struct {
        JSONRPC string          `json:"jsonrpc"`
        ID      any             `json:"id"`
        Method  string          `json:"method"`
        Params  json.RawMessage `json:"params"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, nil, -32700, "parse error")
        return
    }

    // Dispatch
    var result any
    var rpcErr *rpcError

    switch req.Method {
    case "get_tasks":
        result = s.handleGetTasks(session)
    case "claim_task":
        result, rpcErr = s.handleClaimTask(session, req.Params)
    case "update_task":
        result, rpcErr = s.handleUpdateTask(session, req.Params)
    case "post_message":
        result, rpcErr = s.handlePostMessage(session, req.Params)
    case "spawn_teammate":
        result, rpcErr = s.handleSpawnTeammate(session, req.Params)
    default:
        rpcErr = &rpcError{Code: -32601, Message: "method not found"}
    }

    w.Header().Set("Content-Type", "application/json")
    if rpcErr != nil {
        writeError(w, req.ID, rpcErr.Code, rpcErr.Message)
        return
    }
    writeResult(w, req.ID, result)
}

// --- tool handlers ---

func (s *mcpServer) handleGetTasks(session *TeamSession) any {
    session.mu.Lock()
    tasks := make([]TeamTask, len(session.Tasks))
    copy(tasks, session.Tasks)
    session.mu.Unlock()
    return map[string]any{"tasks": tasks}
}

func (s *mcpServer) handleClaimTask(session *TeamSession, raw json.RawMessage) (any, *rpcError) {
    var p struct {
        ID   string `json:"id"`
        Role string `json:"role"`
    }
    if err := json.Unmarshal(raw, &p); err != nil || p.ID == "" || p.Role == "" {
        return nil, &rpcError{Code: -32602, Message: "invalid params: id and role required"}
    }
    if err := session.claimTask(p.ID, p.Role); err != nil {
        return nil, &rpcError{Code: 1, Message: err.Error()}
    }
    return map[string]any{"ok": true}, nil
}

func (s *mcpServer) handleUpdateTask(session *TeamSession, raw json.RawMessage) (any, *rpcError) {
    var p struct {
        ID      string `json:"id"`
        Status  string `json:"status"`
        Message string `json:"message"`
    }
    if err := json.Unmarshal(raw, &p); err != nil || p.ID == "" {
        return nil, &rpcError{Code: -32602, Message: "invalid params: id required"}
    }
    validStatuses := map[string]bool{"pending": true, "in_progress": true, "done": true, "blocked": true}
    if !validStatuses[p.Status] {
        return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid status %q", p.Status)}
    }
    session.updateTask(p.ID, p.Status, p.Message)
    return map[string]any{"ok": true}, nil
}

func (s *mcpServer) handlePostMessage(session *TeamSession, raw json.RawMessage) (any, *rpcError) {
    var p struct {
        From    string `json:"from"`    // caller must identify itself
        To      string `json:"to"`
        Content string `json:"content"`
    }
    if err := json.Unmarshal(raw, &p); err != nil || p.Content == "" || p.From == "" {
        return nil, &rpcError{Code: -32602, Message: "invalid params: from and content required"}
    }
    session.postMessage(p.From, p.To, p.Content)
    return map[string]any{"ok": true}, nil
}

func (s *mcpServer) handleSpawnTeammate(session *TeamSession, raw json.RawMessage) (any, *rpcError) {
    if s.spawn == nil {
        return nil, &rpcError{Code: -32603, Message: "spawn not available"}
    }
    var p struct {
        Role string `json:"role"`
        Task string `json:"task"`
    }
    if err := json.Unmarshal(raw, &p); err != nil || p.Role == "" {
        return nil, &rpcError{Code: -32602, Message: "invalid params: role required"}
    }
    newID, err := s.spawn(session.ID, p.Role, p.Task)
    if err != nil {
        return nil, &rpcError{Code: -32603, Message: err.Error()}
    }
    return map[string]any{"session_id": newID}, nil
}

// --- helpers ---

func (s *mcpServer) sessionForRequest(r *http.Request) *TeamSession {
    auth := r.Header.Get("Authorization")
    token := strings.TrimPrefix(auth, "Bearer ")
    if token == "" {
        return nil
    }
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.sessions[token]
}

// roleForToken is intentionally removed — all agents in a team share one token.
// Sender identity is passed explicitly via the "from" field in post_message.

type rpcError struct {
    Code    int
    Message string
}

func writeResult(w http.ResponseWriter, id any, result any) {
    _ = json.NewEncoder(w).Encode(map[string]any{
        "jsonrpc": "2.0",
        "id":      id,
        "result":  result,
    })
}

func writeError(w http.ResponseWriter, id any, code int, message string) {
    _ = json.NewEncoder(w).Encode(map[string]any{
        "jsonrpc": "2.0",
        "id":      id,
        "error":   map[string]any{"code": code, "message": message},
    })
}
```
- [ ] Run MCP tests:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestMCPServer" -v
```
Expected: all 5 tests PASS.

- [ ] Run full backend tests:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -timeout 30s 2>&1 | tail -10
```
- [ ] Commit:
```bash
git add internal/backend/app_mcp.go internal/backend/app_mcp_test.go
git commit -m "feat: implement MCP Streamable HTTP server with 5 tool handlers (Group B)"
```

---

## Chunk D: Workspace Config + Skills

> `app_workspace.go` + `app_skills.go` — independent of the MCP server. Can be tested without HTTP.

### Task D1: Write failing tests for workspace parsing

**Files:**
- Create: `internal/backend/app_workspace_test.go`

- [ ] Write tests:
```go
package backend

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadWorkspaceConfig_NoFile(t *testing.T) {
    dir := t.TempDir()
    cfg, err := loadWorkspaceConfig(dir)
    if err != nil {
        t.Fatal(err)
    }
    if cfg != nil {
        t.Error("expected nil config when no .mtui.yml exists")
    }
}

func TestLoadWorkspaceConfig_StringRoleFormat(t *testing.T) {
    dir := t.TempDir()
    yml := `
name: test-project
team:
  roles:
    lead: "You are the lead developer."
`
    _ = os.WriteFile(filepath.Join(dir, ".mtui.yml"), []byte(yml), 0o644)
    cfg, err := loadWorkspaceConfig(dir)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Team.Roles["lead"].InlinePrompt != "You are the lead developer." {
        t.Errorf("unexpected lead role: %+v", cfg.Team.Roles["lead"])
    }
}

func TestLoadWorkspaceConfig_ObjectRoleFormat(t *testing.T) {
    dir := t.TempDir()
    yml := `
name: test-project
team:
  roles:
    reviewer:
      skill: code-reviewer
      focus: "Review code carefully."
`
    _ = os.WriteFile(filepath.Join(dir, ".mtui.yml"), []byte(yml), 0o644)
    cfg, err := loadWorkspaceConfig(dir)
    if err != nil {
        t.Fatal(err)
    }
    role := cfg.Team.Roles["reviewer"]
    if role.SkillName != "code-reviewer" || role.Focus != "Review code carefully." {
        t.Errorf("unexpected reviewer role: %+v", role)
    }
}

func TestDetectStack(t *testing.T) {
    cases := []struct {
        file string
        want string
    }{
        {"go.mod", "go"},
        {"next.config.js", "nextjs"},
        {"package.json", "node"},
        {"requirements.txt", "python"},
    }
    for _, c := range cases {
        dir := t.TempDir()
        _ = os.WriteFile(filepath.Join(dir, c.file), []byte(""), 0o644)
        got := detectStack(dir)
        if got != c.want {
            t.Errorf("detectStack with %q = %q, want %q", c.file, got, c.want)
        }
    }
}

func TestDetectStack_DefaultWhenNoFiles(t *testing.T) {
    dir := t.TempDir()
    got := detectStack(dir)
    if got != "default" {
        t.Errorf("detectStack = %q, want \"default\"", got)
    }
}
```
- [ ] Run — expect compile errors:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestLoadWorkspace|TestDetectStack" -v 2>&1 | head -20
```

---

### Task D2: Implement `app_workspace.go`

**Files:**
- Create: `internal/backend/app_workspace.go`

- [ ] Create `internal/backend/app_workspace.go`:
```go
package backend

import (
    "errors"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// WorkspaceConfig is the parsed content of a project's .mtui.yml file.
type WorkspaceConfig struct {
    Name   string          `yaml:"name"`
    Stack  string          `yaml:"stack"` // auto-detected if empty
    Layout []WorkspaceRow  `yaml:"layout"`
    Team   WorkspaceTeam   `yaml:"team"`
}

// WorkspaceRow is a horizontal row in the pane layout.
// The YAML key for row height is "row" (matching .mtui.yml format: "- row: 70%").
type WorkspaceRow struct {
    Size  string          `yaml:"row"`
    Panes []WorkspacePane `yaml:"panes"`
}

// WorkspacePane is a single pane in a row.
type WorkspacePane struct {
    Title   string `yaml:"title"`
    Role    string `yaml:"role"`    // agent role name, if this is an agent pane
    Command string `yaml:"command"` // shell command for non-agent panes
    Size    string `yaml:"size"`
    Dir     string `yaml:"dir"`
}

// WorkspaceTeam holds team configuration from .mtui.yml.
type WorkspaceTeam struct {
    Model       string                    `yaml:"model"`
    MaxMessages int                       `yaml:"max_messages"`
    Roles       map[string]WorkspaceRole  `yaml:"-"` // populated by custom unmarshal
}

// WorkspaceRole represents either an inline prompt or a skill reference.
type WorkspaceRole struct {
    InlinePrompt string // set when role is a plain string
    SkillName    string // set when role is an object with "skill" key
    Focus        string // set when role is an object with "focus" key
}

// loadWorkspaceConfig reads .mtui.yml from dir. Returns nil, nil if not found.
func loadWorkspaceConfig(dir string) (*WorkspaceConfig, error) {
    path := filepath.Join(dir, ".mtui.yml")
    b, err := os.ReadFile(path)
    if errors.Is(err, os.ErrNotExist) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    // Parse into a raw map first to handle the dual role format.
    var raw struct {
        Name   string            `yaml:"name"`
        Stack  string            `yaml:"stack"`
        Layout []WorkspaceRow    `yaml:"layout"`
        Team   struct {
            Model       string                 `yaml:"model"`
            MaxMessages int                    `yaml:"max_messages"`
            Roles       map[string]yaml.Node   `yaml:"roles"`
        } `yaml:"team"`
    }
    if err := yaml.Unmarshal(b, &raw); err != nil {
        return nil, err
    }

    cfg := &WorkspaceConfig{
        Name:   raw.Name,
        Stack:  raw.Stack,
        Layout: raw.Layout,
        Team: WorkspaceTeam{
            Model:       raw.Team.Model,
            MaxMessages: raw.Team.MaxMessages,
            Roles:       make(map[string]WorkspaceRole),
        },
    }

    for name, node := range raw.Team.Roles {
        switch node.Kind {
        case yaml.ScalarNode: // plain string
            cfg.Team.Roles[name] = WorkspaceRole{InlinePrompt: node.Value}
        case yaml.MappingNode: // object with skill+focus
            var obj struct {
                Skill string `yaml:"skill"`
                Focus string `yaml:"focus"`
            }
            if err := node.Decode(&obj); err == nil {
                cfg.Team.Roles[name] = WorkspaceRole{SkillName: obj.Skill, Focus: obj.Focus}
            }
        }
    }

    if cfg.Stack == "" {
        cfg.Stack = detectStack(dir)
    }
    if cfg.Team.MaxMessages == 0 {
        cfg.Team.MaxMessages = 200
    }
    return cfg, nil
}

// detectStack identifies the project type from files in dir.
// Detection runs in priority order; first match wins.
func detectStack(dir string) string {
    checks := []struct {
        pattern string
        stack   string
    }{
        {"go.mod", "go"},
        {"next.config.js", "nextjs"},
        {"next.config.ts", "nextjs"},
        {"next.config.mjs", "nextjs"},
        {"package.json", "node"},
        {"requirements.txt", "python"},
        {"pyproject.toml", "python"},
    }
    for _, c := range checks {
        matches, _ := filepath.Glob(filepath.Join(dir, c.pattern))
        if len(matches) > 0 {
            return c.stack
        }
    }
    return "default"
}
```
- [ ] Run workspace tests:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestLoadWorkspace|TestDetectStack" -v
```
Expected: all 6 tests PASS.

---

### Task D3: Implement `app_skills.go` + write tests

**Files:**
- Create: `internal/backend/app_skills_test.go`
- Create: `internal/backend/app_skills.go`

- [ ] Write `internal/backend/app_skills_test.go`:
```go
package backend

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestLoadSkill_ProjectLocal(t *testing.T) {
    dir := t.TempDir()
    skillDir := filepath.Join(dir, ".mtui", "skills")
    _ = os.MkdirAll(skillDir, 0o755)
    _ = os.WriteFile(filepath.Join(skillDir, "my-skill.md"), []byte("# My Skill\nDo great work."), 0o644)

    content, err := loadSkill(dir, "my-skill")
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(content, "Do great work.") {
        t.Errorf("unexpected content: %q", content)
    }
}

func TestLoadSkill_MissingReturnsEmpty(t *testing.T) {
    dir := t.TempDir()
    content, err := loadSkill(dir, "nonexistent")
    if err != nil {
        t.Fatal(err)
    }
    if content != "" {
        t.Errorf("expected empty string for missing skill, got %q", content)
    }
}

func TestBuildRolePrompt_InlineString(t *testing.T) {
    role := WorkspaceRole{InlinePrompt: "You are a tester."}
    prompt, err := buildRolePrompt("/any/dir", role)
    if err != nil {
        t.Fatal(err)
    }
    if prompt != "You are a tester." {
        t.Errorf("unexpected prompt: %q", prompt)
    }
}

func TestBuildRolePrompt_SkillWithFocus(t *testing.T) {
    dir := t.TempDir()
    skillDir := filepath.Join(dir, ".mtui", "skills")
    _ = os.MkdirAll(skillDir, 0o755)
    _ = os.WriteFile(filepath.Join(skillDir, "backend-lead.md"), []byte("# Backend Lead\nCore principles."), 0o644)

    role := WorkspaceRole{SkillName: "backend-lead", Focus: "Focus on the API layer."}
    prompt, err := buildRolePrompt(dir, role)
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(prompt, "Core principles.") {
        t.Errorf("skill content missing: %q", prompt)
    }
    if !strings.Contains(prompt, "Focus on the API layer.") {
        t.Errorf("focus missing: %q", prompt)
    }
}

func TestBuildIssueAddendum(t *testing.T) {
    addendum := buildIssueAddendum("Fix login bug", 42, "go", map[string]string{
        "1": "yes",
        "2": "1.21+",
    })
    if !strings.Contains(addendum, "Fix login bug") {
        t.Error("issue title missing")
    }
    if !strings.Contains(addendum, "#42") {
        t.Error("issue number missing")
    }
    if !strings.Contains(addendum, "1.21+") {
        t.Error("planning answer missing")
    }
}
```
- [ ] Create `internal/backend/app_skills.go`:
```go
package backend

import (
    "embed"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// loadSkill loads a skill file by name.
// Search order: {projectDir}/.mtui/skills/{name}.md, then ~/.multiterminal/skills/{name}.md.
// Returns empty string (no error) if the skill doesn't exist in either location.
func loadSkill(projectDir, name string) (string, error) {
    candidates := []string{
        filepath.Join(projectDir, ".mtui", "skills", name+".md"),
    }
    if home, err := os.UserHomeDir(); err == nil {
        candidates = append(candidates, filepath.Join(home, ".multiterminal", "skills", name+".md"))
    }
    for _, path := range candidates {
        b, err := os.ReadFile(path)
        if errors.Is(err, os.ErrNotExist) {
            continue
        }
        if err != nil {
            return "", err
        }
        return string(b), nil
    }
    return "", nil
}

// buildRolePrompt resolves a WorkspaceRole into a complete system prompt string.
func buildRolePrompt(projectDir string, role WorkspaceRole) (string, error) {
    if role.InlinePrompt != "" {
        return role.InlinePrompt, nil
    }
    base, err := loadSkill(projectDir, role.SkillName)
    if err != nil {
        return "", err
    }
    if base == "" {
        return fmt.Sprintf("Role: %s\n%s", role.SkillName, role.Focus), nil
    }
    if role.Focus == "" {
        return base, nil
    }
    return base + "\n\n## Your Focus\n" + role.Focus, nil
}

// buildIssueAddendum generates the issue-specific context block appended to every agent prompt.
func buildIssueAddendum(title string, number int, stack string, planningAnswers map[string]string) string {
    var sb strings.Builder
    sb.WriteString("\n\n## Current Task Context\n")
    sb.WriteString(fmt.Sprintf("Issue: %s (#%d)\n", title, number))
    sb.WriteString(fmt.Sprintf("Stack: %s\n", stack))
    if len(planningAnswers) > 0 {
        sb.WriteString("\n### Constraints from planning\n")
        for k, v := range planningAnswers {
            sb.WriteString(fmt.Sprintf("- Q%s: %s\n", k, v))
        }
    }
    return sb.String()
}
```
- [ ] Run skills tests:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -run "TestLoadSkill|TestBuildRole|TestBuildIssue" -v
```
Expected: all 5 tests PASS.

- [ ] Commit:
```bash
git add internal/backend/app_workspace.go internal/backend/app_workspace_test.go \
        internal/backend/app_skills.go internal/backend/app_skills_test.go
git commit -m "feat: implement workspace config parser, stack detection, and skill loader (Group C)"
```

---

## Chunk E: App Integration

> Wire MCP server + workspace + skills into `app.go` and `app_kanban.go`. Adds `StartTeam` backend method and `spawnCh` channel.

### Task E1: Add `spawnCh`, MCP server, and `StartTeam` — split into `app_team.go`

> **300-line rule:** `app.go` is already at 320 lines. All new team-related methods go into a new `internal/backend/app_team.go`. Only struct fields and `ServiceStartup`/`ServiceShutdown` hooks are added to `app.go`.

**Files:**
- Modify: `internal/backend/app.go` (fields + startup/shutdown hooks only, ~+30 lines)
- Create: `internal/backend/app_team.go` (~250 lines, all team methods)

- [ ] Create `internal/backend/app_team.go` with the package header and imports:
```go
package backend

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)
```
> All `StartTeam`, `spawnTeammate`, `spawnDispatcher`, `mcpSpawn`, `buildLeadPrompt`, `injectLeadPrompt`, `cleanStaleMCPConfigs`, `checkPlanningOutput`, and `extractJSONBlock` methods go in this file. Only struct fields and startup/shutdown hooks go in `app.go`.

- [ ] Add the following fields to `AppService` struct (after `tmuxAPIPort`):
```go
mcpSrv     *mcpServer
mcpPort    int
spawnCh    chan spawnRequest // teammate spawn requests from MCP handler
```

- [ ] Add `spawnRequest` type near the top of `app.go`:
```go
// spawnRequest is posted to spawnCh by the MCP handler when spawn_teammate is called.
type spawnRequest struct {
    sessionID string
    role      string
    task      string
    result    chan spawnResult
}

type spawnResult struct {
    newSessionID string
    err          error
}
```

- [ ] In `NewAppService`, initialize fields:
```go
spawnCh: make(chan spawnRequest, 16),
```

- [ ] In `ServiceStartup`, after existing setup, start MCP server + spawn dispatcher:
```go
// Start MCP server (lazy — port reserved at startup, sessions added on demand)
a.mcpSrv = newMCPServer(a.mcpSpawn)
port, err := a.mcpSrv.start(ctx)
if err != nil {
    log.Printf("[mcp] failed to start server: %v", err)
} else {
    a.mcpPort = port
    log.Printf("[mcp] server started on port %d", port)
}
// Clean up stale MCP config files from previous crashed sessions
a.cleanStaleMCPConfigs()
// Start spawn dispatcher goroutine
go a.spawnDispatcher(ctx)
```

- [ ] Add the following methods to `app_team.go`:

```go
// mcpSpawn is the spawnFn passed to the MCP server.
// It posts a request to spawnCh and waits for the result.
func (a *AppService) mcpSpawn(sessionID, role, task string) (string, error) {
    req := spawnRequest{
        sessionID: sessionID,
        role:      role,
        task:      task,
        result:    make(chan spawnResult, 1),
    }
    select {
    case a.spawnCh <- req:
    default:
        return "", fmt.Errorf("spawn queue full")
    }
    res := <-req.result
    return res.newSessionID, res.err
}

// spawnDispatcher runs in a dedicated goroutine and processes spawn requests
// safely on the service goroutine (avoids calling CreateSession from HTTP handler).
func (a *AppService) spawnDispatcher(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case req := <-a.spawnCh:
            id, err := a.spawnTeammate(req.sessionID, req.role, req.task)
            req.result <- spawnResult{newSessionID: id, err: err}
        }
    }
}

// spawnTeammate creates a new PTY session for a teammate agent.
// It calls CreateSession which has signature: (argv []string, dir string, rows int, cols int, mode string) int
func (a *AppService) spawnTeammate(teamSessionID, role, task string) (string, error) {
    a.mu.Lock()
    ts := a.activeTeamSessions[teamSessionID]
    a.mu.Unlock()
    if ts == nil {
        return "", fmt.Errorf("team session %q not found", teamSessionID)
    }
    cfgFile := mcpConfigPath(ts.ID)
    claudePath := a.resolvedClaudePath
    if claudePath == "" {
        claudePath = "claude"
    }
    argv := []string{claudePath, "--mcp-config", cfgFile}
    sessionID := a.CreateSession(argv, "", 24, 80, "claude")
    a.mu.Lock()
    ts.Agents[role] = fmt.Sprintf("%d", sessionID)
    a.mu.Unlock()
    return fmt.Sprintf("%d", sessionID), nil
}

// cleanStaleMCPConfigs removes ~/.mtui/mcp-*.json files whose port is no longer live.
func (a *AppService) cleanStaleMCPConfigs() {
    dir := mcpConfigDir()
    entries, err := os.ReadDir(dir)
    if err != nil {
        return
    }
    for _, e := range entries {
        if !strings.HasPrefix(e.Name(), "mcp-") || !strings.HasSuffix(e.Name(), ".json") {
            continue
        }
        _ = os.Remove(filepath.Join(dir, e.Name()))
    }
}
```

- [ ] Add `activeTeamSessions` map to `AppService` struct:
```go
activeTeamSessions map[string]*TeamSession // teamSessionID → TeamSession
```
- [ ] Initialize in `NewAppService`:
```go
activeTeamSessions: make(map[string]*TeamSession),
```

- [ ] Add `StartTeam` method (new exported Wails binding):
```go
// StartTeam launches a multi-agent team for the given Kanban issue.
// dir is the project working directory. issueID is the KanbanCard.ID.
// Returns the TeamSession ID.
func (a *AppService) StartTeam(dir, issueID, issueTitle, issueBody string) (string, error) {
    // Load workspace config (nil if no .mtui.yml)
    wsCfg, err := loadWorkspaceConfig(dir)
    if err != nil {
        return "", fmt.Errorf("workspace config: %w", err)
    }

    // Create team session
    ts := newTeamSession(issueID)
    a.mcpSrv.addSession(ts)
    a.mu.Lock()
    a.activeTeamSessions[ts.ID] = ts
    a.mu.Unlock()

    // Write MCP config file for this session
    if err := writeMCPConfig(ts.ID, ts.Token, a.mcpPort); err != nil {
        return "", err
    }

    // Determine stack
    stack := "default"
    if wsCfg != nil && wsCfg.Stack != "" {
        stack = wsCfg.Stack
    }

    // Build lead system prompt
    leadPrompt := buildLeadPrompt(dir, wsCfg, stack, issueTitle, issueBody)

    // Spawn lead agent pane
    claudePath := a.resolvedClaudePath
    if claudePath == "" {
        claudePath = "claude"
    }
    cfgFile := mcpConfigPath(ts.ID)
    argv := []string{claudePath, "--mcp-config", cfgFile}
    // CreateSession signature: (argv []string, dir string, rows int, cols int, mode string) int
    sessionID := a.CreateSession(argv, dir, 24, 80, "claude")
    a.mu.Lock()
    ts.Agents["lead"] = fmt.Sprintf("%d", sessionID)
    a.mu.Unlock()

    // Inject initial prompt after Claude is ready (non-blocking)
    go a.injectLeadPrompt(sessionID, leadPrompt)

    // Emit team:start event
    a.app.Event.Emit("team:start", ts)

    return ts.ID, nil
}

// buildLeadPrompt assembles the initial system prompt + issue context for the lead agent.
func buildLeadPrompt(dir string, wsCfg *WorkspaceConfig, stack, title, body string) string {
    rolePrompt := "You are the lead developer. Coordinate the team using MCP tools."
    if wsCfg != nil {
        if r, ok := wsCfg.Team.Roles["lead"]; ok {
            if p, err := buildRolePrompt(dir, r); err == nil && p != "" {
                rolePrompt = p
            }
        }
    }
    addendum := buildIssueAddendum(title, 0, stack, nil)
    planningInstr := `
First, analyze the issue carefully. Then output your clarifying questions as JSON:
{"mtui_planning": [{"id":"1","question":"...","default":"..."}]}
Wait for the user's answers before proceeding.`
    return rolePrompt + addendum + planningInstr
}

// injectLeadPrompt waits for Claude's prompt to appear (ActivityDone) then injects the prompt.
// Uses ActivityDone — not ActivityIdle — because a fresh session starts in ActivityIdle
// before Claude has launched. ActivityDone means Claude has rendered its first prompt.
func (a *AppService) injectLeadPrompt(sessionID int, prompt string) {
    for i := 0; i < 60; i++ { // up to 30s
        a.mu.Lock()
        sess, ok := a.sessions[sessionID]
        a.mu.Unlock()
        if !ok {
            return
        }
        if sess.Activity == terminal.ActivityDone {
            break
        }
        time.Sleep(500 * time.Millisecond)
    }
    // WriteToSession expects base64-encoded input (same as frontend keyboard input)
    b64 := base64.StdEncoding.EncodeToString([]byte(prompt + "\n"))
    a.WriteToSession(sessionID, b64)
}
```

- [ ] Add MCP config file helpers (can go at bottom of `app_mcp.go`):
```go
// mcpConfigDir returns ~/.mtui/ — the directory for per-session MCP config files.
func mcpConfigDir() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".mtui")
}

// mcpConfigPath returns the path for a session's MCP config file.
func mcpConfigPath(sessionID string) string {
    return filepath.Join(mcpConfigDir(), "mcp-"+sessionID+".json")
}

// writeMCPConfig writes the Streamable HTTP config file for Claude Code.
func writeMCPConfig(sessionID, token string, port int) error {
    if err := os.MkdirAll(mcpConfigDir(), 0o700); err != nil {
        return err
    }
    cfg := map[string]any{
        "mcpServers": map[string]any{
            "mtui": map[string]any{
                "type": "http",
                "url":  fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
                "headers": map[string]string{
                    "Authorization": "Bearer " + token,
                },
            },
        },
    }
    b, err := json.Marshal(cfg)
    if err != nil {
        return err
    }
    return os.WriteFile(mcpConfigPath(sessionID), b, 0o600)
}
```

- [ ] Build to check for compile errors:
```bash
cd D:/repos/Multiterminal && go build ./internal/backend/ 2>&1
```
- [ ] Fix any compile errors.

- [ ] Run full backend tests:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -timeout 30s 2>&1 | tail -15
```

- [ ] Commit:
```bash
git add internal/backend/app.go internal/backend/app_mcp.go internal/backend/app_team.go
git commit -m "feat: integrate MCP server and StartTeam binding into AppService (Group D)"
```

---

### Task E2: Wire planning detection — add to `app_team.go`

> **300-line rule:** After Tasks C+E1 additions, `app_mcp.go` will approach 300 lines. Add `checkPlanningOutput` and `extractJSONBlock` to `app_team.go` (not `app_mcp.go`).

**Files:**
- Modify: `internal/backend/app_team.go`
- Modify: `internal/backend/app_scan.go`

> After each `ActivityDone` event, scan the lead session's screen buffer for the `{"mtui_planning":` sentinel. When found, emit `team:planning_ready`.

- [ ] Add `checkPlanningOutput` method to `AppService` (not `mcpServer` — needs access to `a.sessions` and `a.app`):
```go
// checkPlanningOutput scans the lead session's screen buffer for the planning JSON sentinel.
// Called from app_scan.go after each ActivityDone event. Emits "team:planning_ready" once per session.
func (a *AppService) checkPlanningOutput(sessionID int) {
    a.mu.Lock()
    sess, ok := a.sessions[sessionID]
    a.mu.Unlock()
    if !ok {
        return
    }
    text := sess.Screen.PlainText()
    idx := strings.Index(text, `{"mtui_planning":`)
    if idx == -1 {
        return
    }
    // Extract JSON block (find matching closing brace)
    jsonStr := extractJSONBlock(text[idx:])
    if jsonStr == "" {
        return
    }
    var envelope struct {
        Planning []struct {
            ID       string `json:"id"`
            Question string `json:"question"`
            Default  string `json:"default"`
        } `json:"mtui_planning"`
    }
    if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
        return
    }
    // Only emit once per session
    a.mu.Lock()
    if _, alreadySent := a.planningEmitted[sessionID]; alreadySent {
        a.mu.Unlock()
        return
    }
    a.planningEmitted[sessionID] = true
    a.mu.Unlock()
    a.app.Event.Emit("team:planning_ready", map[string]any{
        "session_id": sessionID,
        "questions":  envelope.Planning,
    })
}

// extractJSONBlock extracts the first complete JSON object starting at s.
func extractJSONBlock(s string) string {
    depth := 0
    for i, c := range s {
        switch c {
        case '{':
            depth++
        case '}':
            depth--
            if depth == 0 {
                return s[:i+1]
            }
        }
    }
    return ""
}
```

- [ ] Add `planningEmitted map[int]bool` to `AppService` struct, initialized in `NewAppService`.

- [ ] In `app_scan.go`'s `scanAllSessions` (or wherever `ActivityDone` is detected), call `a.checkPlanningOutput(sessionID)` when activity transitions to `ActivityDone`.

- [ ] Build and test:
```bash
cd D:/repos/Multiterminal && go build ./internal/backend/ 2>&1
```

- [ ] Commit:
```bash
git add internal/backend/app_team.go internal/backend/app_scan.go
git commit -m "feat: add planning JSON sentinel detection and team:planning_ready event"
```

---

### Task E3: Update Wails bindings

**Files:**
- Modify: `frontend/wailsjs/go/backend/App.d.ts`
- Modify: `frontend/wailsjs/go/backend/App.js`

- [ ] Add `StartTeam` binding to `App.d.ts`:
```typescript
export function StartTeam(dir: string, issueID: string, issueTitle: string, issueBody: string): Promise<string>;
```

- [ ] Add `StartTeam` to `App.js`:
```javascript
export function StartTeam(dir, issueID, issueTitle, issueBody) {
  return window['go']['backend']['AppService']['StartTeam'](dir, issueID, issueTitle, issueBody);
}
```

- [ ] Add `TeamSession`, `TeamTask`, `TeamMessage` to `frontend/wailsjs/go/models.ts`:
```typescript
export class TeamTask {
    id: string;
    title: string;
    claimed_by: string;
    status: string;
    message: string;
    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.id = source["id"];
        this.title = source["title"];
        this.claimed_by = source["claimed_by"];
        this.status = source["status"];
        this.message = source["message"];
    }
}
export class TeamMessage {
    from: string;
    to: string;
    content: string;
    timestamp: number;
    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.from = source["from"];
        this.to = source["to"];
        this.content = source["content"];
        this.timestamp = source["timestamp"];
    }
}
export class TeamSession {
    id: string;
    issue_id: string;
    tasks: TeamTask[];
    messages: TeamMessage[];
    agents: {[key: string]: string};
    max_messages: number;
    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.id = source["id"];
        this.issue_id = source["issue_id"];
        this.tasks = this.convertValues(source["tasks"], TeamTask);
        this.messages = this.convertValues(source["messages"], TeamMessage);
        this.agents = source["agents"];
        this.max_messages = source["max_messages"];
    }
    convertValues(a: any, classs: any, asMap: boolean = false): any {
        if (!a) { return a; }
        if (a.slice && a.map) { return (a as any[]).map(elem => this.convertValues(elem, classs)); }
        return new classs(a);
    }
}
```

- [ ] Commit:
```bash
git add frontend/wailsjs/
git commit -m "chore: add StartTeam binding and TeamSession models to Wails frontend bindings"
```

---

## Chunk F: Frontend Components

> Two new Svelte components + extensions to three existing ones.

### Task F1: `TeamPlanningDialog.svelte`

**Files:**
- Create: `frontend/src/components/TeamPlanningDialog.svelte`

- [ ] Create the component:
```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let questions: Array<{id: string; question: string; default: string}> = [];
  export let visible = false;

  const dispatch = createEventDispatcher<{ answered: Record<string, string> }>();

  let answers: Record<string, string> = {};

  $: if (visible) initAnswers();

  function initAnswers() {
    const init: Record<string, string> = {};
    for (const q of questions) {
      init[q.id] = q.default ?? '';
    }
    answers = init;
  }

  function submit() {
    dispatch('answered', { ...answers });
    visible = false;
  }
</script>

{#if visible}
  <div class="overlay">
    <div class="dialog">
      <h3>Kurze Rückfragen</h3>
      <p class="subtitle">Beantworte einmal, dann arbeitet das Team autonom.</p>

      <div class="questions">
        {#each questions as q}
          <label>
            <span>{q.question}</span>
            <input type="text" bind:value={answers[q.id]} placeholder={q.default} />
          </label>
        {/each}
      </div>

      <button class="primary" on:click={submit}>Los geht's</button>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.5);
    display: flex; align-items: center; justify-content: center;
    z-index: 1000;
  }
  .dialog {
    background: var(--bg-secondary, #1e1e2e);
    border: 1px solid var(--border, #45475a);
    border-radius: 8px;
    padding: 24px;
    min-width: 420px;
    max-width: 600px;
  }
  h3 { margin: 0 0 4px; color: var(--text-primary, #cdd6f4); }
  .subtitle { margin: 0 0 20px; color: var(--text-muted, #6c7086); font-size: 13px; }
  .questions { display: flex; flex-direction: column; gap: 12px; margin-bottom: 20px; }
  label { display: flex; flex-direction: column; gap: 4px; }
  label span { font-size: 13px; color: var(--text-secondary, #a6adc8); }
  input {
    background: var(--bg-primary, #181825);
    border: 1px solid var(--border, #45475a);
    border-radius: 4px;
    color: var(--text-primary, #cdd6f4);
    padding: 6px 10px;
    font-size: 13px;
  }
  .primary {
    background: var(--accent, #89b4fa);
    color: var(--bg-primary, #181825);
    border: none;
    border-radius: 4px;
    padding: 8px 20px;
    font-weight: 600;
    cursor: pointer;
    width: 100%;
  }
</style>
```

---

### Task F2: `KanbanTeamView.svelte`

**Files:**
- Create: `frontend/src/components/KanbanTeamView.svelte`

- [ ] Create the component:
```svelte
<script lang="ts">
  import type { TeamSession } from '../../wailsjs/go/models';

  export let session: TeamSession | null = null;
</script>

{#if session}
  <div class="team-view">
    <h4>Agent Team</h4>

    <div class="section">
      <div class="label">Agenten</div>
      {#each Object.entries(session.agents ?? {}) as [role, _sessID]}
        <div class="agent">
          <span class="badge">{role}</span>
        </div>
      {/each}
    </div>

    <div class="section">
      <div class="label">Tasks</div>
      {#each session.tasks ?? [] as task}
        <div class="task" class:done={task.status === 'done'}>
          <span class="status {task.status}">●</span>
          <span class="title">{task.title}</span>
          {#if task.claimed_by}
            <span class="assignee">{task.claimed_by}</span>
          {/if}
        </div>
      {/each}
    </div>

    {#if (session.messages ?? []).length > 0}
      <div class="section">
        <div class="label">Nachrichten</div>
        {#each session.messages.slice(-5) as msg}
          <div class="message">
            <span class="from">{msg.from}</span>
            <span class="content">{msg.content}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .team-view { padding: 12px; font-size: 12px; }
  h4 { margin: 0 0 12px; color: var(--text-primary, #cdd6f4); }
  .section { margin-bottom: 12px; }
  .label { font-size: 10px; text-transform: uppercase; color: var(--text-muted, #6c7086); margin-bottom: 6px; letter-spacing: 0.05em; }
  .badge { background: var(--accent, #89b4fa); color: var(--bg-primary, #181825); padding: 2px 6px; border-radius: 3px; font-size: 11px; font-weight: 600; }
  .task { display: flex; align-items: center; gap: 6px; padding: 3px 0; }
  .task.done .title { opacity: 0.5; text-decoration: line-through; }
  .status.done { color: #a6e3a1; }
  .status.in_progress { color: #f9e2af; }
  .status.blocked { color: #f38ba8; }
  .status.pending { color: #6c7086; }
  .assignee { margin-left: auto; color: var(--text-muted, #6c7086); font-size: 11px; }
  .message { display: flex; gap: 8px; padding: 3px 0; border-top: 1px solid var(--border, #313244); }
  .from { color: var(--accent, #89b4fa); min-width: 60px; }
  .content { color: var(--text-secondary, #a6adc8); }
</style>
```

---

### Task F3: Wire events in `KanbanBoard.svelte`

**Files:**
- Modify: `frontend/src/components/KanbanBoard.svelte`

- [ ] Read `KanbanBoard.svelte` to understand existing event setup, then add:

```svelte
<script>
  // Add to existing imports:
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import { StartTeam } from '../../wailsjs/go/backend/App';
  import TeamPlanningDialog from './TeamPlanningDialog.svelte';
  import KanbanTeamView from './KanbanTeamView.svelte';
  import type { TeamSession } from '../../wailsjs/go/models';

  // Add to existing state:
  let activeTeamSession: TeamSession | null = null;
  let planningQuestions: Array<{id: string; question: string; default: string}> = [];
  let showPlanningDialog = false;
  let pendingLeadSessionID: number | null = null;

  // Add in onMount / event subscription block:
  EventsOn('team:start', (session: TeamSession) => {
    activeTeamSession = session;
  });
  EventsOn('team:update', (session: TeamSession) => {
    activeTeamSession = session;
  });
  EventsOn('team:planning_ready', (data: { session_id: number; questions: any[] }) => {
    pendingLeadSessionID = data.session_id;
    planningQuestions = data.questions;
    showPlanningDialog = true;
  });

  async function startTeam(card: any) {
    await StartTeam(card.dir ?? '', card.id, card.title, card.body ?? '');
  }

  async function submitPlanningAnswers(e: CustomEvent<Record<string, string>>) {
    showPlanningDialog = false;
    if (pendingLeadSessionID == null) return;
    // Build answer string and inject into lead session
    const answers = e.detail;
    const text = Object.entries(answers)
      .map(([k, v]) => `Q${k}: ${v}`)
      .join('\n');
    // Import WriteToSession if not already imported
    await WriteToSession(pendingLeadSessionID, `Planning answers:\n${text}\n`);
    pendingLeadSessionID = null;
  }
</script>

<!-- Add inside the template, alongside existing content: -->
<TeamPlanningDialog
  bind:visible={showPlanningDialog}
  questions={planningQuestions}
  on:answered={submitPlanningAnswers}
/>

<KanbanTeamView session={activeTeamSession} />
```

- [ ] Add "Team starten" button to `KanbanColumn.svelte` on each card that has a `dir` field. Read the file first to find the right insertion point.

- [ ] Add agent badge to `KanbanCard.svelte`. Read the file first to find where the card title is rendered, then add:
```svelte
{#if agentRole}
  <span class="agent-badge">{agentRole}</span>
{/if}
```
with `agentRole` as a prop passed down from `KanbanBoard` when `activeTeamSession?.agents` has an entry matching the card's ID.

- [ ] Build frontend to check for TypeScript errors:
```bash
cd D:/repos/Multiterminal/frontend && npm run build 2>&1 | tail -20
```
- [ ] Fix any TypeScript errors.

- [ ] Commit:
```bash
git add frontend/src/components/
git commit -m "feat: add TeamPlanningDialog, KanbanTeamView, and team event wiring in KanbanBoard (Group E)"
```

---

## Chunk G: Session Persistence + Resume Banner

### Task G1: Save/load team sessions on app lifecycle

**Files:**
- Modify: `internal/backend/app.go`

- [ ] In `ServiceShutdown`, save all active team sessions:
```go
// Save active team sessions for resume
sessDir := teamSessionDir()
a.mu.Lock()
for _, ts := range a.activeTeamSessions {
    if err := saveTeamSession(sessDir, ts); err != nil {
        log.Printf("[team] failed to save session %s: %v", ts.ID, err)
    }
}
a.mu.Unlock()
```

- [ ] In `ServiceStartup`, after loading Kanban state, check for resumable sessions. Emit `team:resumable` if found. First, add a method `GetKanbanCardIDs` or use existing Kanban state access to get all current card IDs. Then:
```go
go func() {
    // Wait briefly for Kanban state to load
    time.Sleep(200 * time.Millisecond)
    cardIDs := a.allKanbanCardIDs() // implement this helper
    sessions, err := findResumableSessions(teamSessionDir(), cardIDs)
    if err != nil || len(sessions) == 0 {
        return
    }
    a.app.Event.Emit("team:resumable", sessions)
}()
```

- [ ] Add `allKanbanCardIDs(dir string) []string` helper to `app_kanban.go`.
  Kanban state is not cached in memory — it is loaded from disk on each call via `loadKanbanState(dir)`.
  Store the last-used project dir in `AppService` as `lastKanbanDir string` (set in `GetKanbanState`):
```go
// allKanbanCardIDs returns the IDs of all cards in the Kanban board for dir.
func (a *AppService) allKanbanCardIDs(dir string) []string {
    state, err := loadKanbanState(dir)
    if err != nil || state == nil {
        return nil
    }
    var ids []string
    for _, cards := range state.Columns {
        for _, c := range cards {
            ids = append(ids, c.ID)
        }
    }
    return ids
}
```
  Add `lastKanbanDir string` to `AppService` struct. Set it in `GetKanbanState`:
```go
func (a *AppService) GetKanbanState(dir string) KanbanState {
    a.mu.Lock()
    a.lastKanbanDir = dir
    a.mu.Unlock()
    // ... existing implementation ...
}
```
  Use `a.lastKanbanDir` in the resume check goroutine:
```go
go func() {
    time.Sleep(500 * time.Millisecond)
    a.mu.Lock()
    dir := a.lastKanbanDir
    a.mu.Unlock()
    if dir == "" { return }
    cardIDs := a.allKanbanCardIDs(dir)
    sessions, err := findResumableSessions(teamSessionDir(), cardIDs)
    if err != nil || len(sessions) == 0 { return }
    a.app.Event.Emit("team:resumable", sessions)
}()
```

### Task G2: Resume banner in `KanbanBoard.svelte`

**Files:**
- Modify: `frontend/src/components/KanbanBoard.svelte`

- [ ] Add to event subscriptions:
```svelte
let resumableSessions: TeamSession[] = [];

EventsOn('team:resumable', (sessions: TeamSession[]) => {
  resumableSessions = sessions;
});

function dismissResume(id: string) {
  resumableSessions = resumableSessions.filter(s => s.id !== id);
  DismissTeamResume(id); // backend call to delete the persisted file
}
```

- [ ] Add `DismissTeamResume` backend method to `app.go`:
```go
// DismissTeamResume deletes a saved team session, hiding the resume banner.
func (a *AppService) DismissTeamResume(sessionID string) {
    deleteTeamSession(teamSessionDir(), sessionID)
}
```

- [ ] Add `DismissTeamResume` to Wails bindings (`App.d.ts` + `App.js`).

- [ ] Add resume banner to KanbanBoard template:
```svelte
{#each resumableSessions as s}
  <div class="resume-banner">
    <span>Team fortsetzen? (Issue: {s.issue_id})</span>
    <button on:click={() => startTeamResume(s)}>Fortsetzen</button>
    <button class="dismiss" on:click={() => dismissResume(s.id)}>✕</button>
  </div>
{/each}
```

- [ ] Build and test frontend:
```bash
cd D:/repos/Multiterminal/frontend && npm run build 2>&1 | tail -20
```

- [ ] Run all backend tests:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ -timeout 30s 2>&1 | tail -10
```

- [ ] Commit:
```bash
git add internal/backend/app.go internal/backend/app_kanban.go \
        frontend/src/components/KanbanBoard.svelte \
        frontend/wailsjs/go/backend/App.d.ts frontend/wailsjs/go/backend/App.js
git commit -m "feat: add team session persistence and resume banner (Group F)"
```

---

## Chunk H: Final Integration + Manual Smoke Test

### Task H1: Manual end-to-end smoke test

- [ ] Build the app:
```bash
cd D:/repos/Multiterminal/frontend && npm run build && cd .. && go build -o build/bin/multiterminal.exe -tags desktop . 2>&1
```
- [ ] Start the app and verify:
  1. App starts without errors in the console
  2. MCP server starts on a random port (check logs)
  3. Open a project with a `.mtui.yml` containing Go stack config
  4. Open Kanban board — "Team starten" button appears on a card
  5. Click "Team starten" — Lead pane opens with Claude
  6. Wait for planning JSON output — dialog appears with questions
  7. Fill answers and click "Los geht's"
  8. Confirm answers are injected into Claude session
  9. Close the app — check `~/.multiterminal/team-sessions/` for saved session
  10. Reopen app — "Team fortsetzen?" banner appears on the matching card

### Task H2: Final commit + branch summary

- [ ] Run all tests one final time:
```bash
cd D:/repos/Multiterminal && go test ./internal/backend/ ./internal/terminal/ -timeout 60s 2>&1 | tail -15
```
- [ ] Final commit if any fixes were needed:
```bash
git add -p
git commit -m "fix: smoke test fixes for AI workspace orchestration"
```
- [ ] Push branch:
```bash
git push -u origin feature/ai-workspace-orchestration
```

---

## File Map Summary

| File | Status | Chunk |
|------|--------|-------|
| `internal/backend/skills/backend-lead.md` | Create | A |
| `internal/backend/skills/code-reviewer.md` | Create | A |
| `internal/backend/skills/frontend-specialist.md` | Create | A |
| `internal/backend/app_mcp_state.go` | Create | B |
| `internal/backend/app_mcp_state_test.go` | Create | B |
| `internal/backend/app_mcp.go` | Create | C |
| `internal/backend/app_mcp_test.go` | Create | C |
| `internal/backend/app_workspace.go` | Create | D |
| `internal/backend/app_workspace_test.go` | Create | D |
| `internal/backend/app_skills.go` | Create | D |
| `internal/backend/app_skills_test.go` | Create | D |
| `internal/backend/app_team.go` | Create | E (StartTeam, spawnTeammate, injectLeadPrompt, checkPlanningOutput) |
| `internal/backend/app.go` | Modify | E (fields + startup/shutdown hooks only) |
| `internal/backend/app_kanban.go` | Modify | E (lastKanbanDir), G (allKanbanCardIDs) |
| `internal/backend/app_scan.go` | Modify | E (call checkPlanningOutput on ActivityDone) |
| `frontend/src/components/TeamPlanningDialog.svelte` | Create | F |
| `frontend/src/components/KanbanTeamView.svelte` | Create | F |
| `frontend/src/components/KanbanBoard.svelte` | Modify | F, G |
| `frontend/src/components/KanbanCard.svelte` | Modify | F |
| `frontend/src/components/KanbanColumn.svelte` | Modify | F |
| `frontend/wailsjs/go/backend/App.d.ts` | Modify | E, G |
| `frontend/wailsjs/go/backend/App.js` | Modify | E, G |
| `frontend/wailsjs/go/models.ts` | Modify | E |
