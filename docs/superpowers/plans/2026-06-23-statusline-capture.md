# Statusline Capture (Sub-plan 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Capture Claude's official cost / context% / model per pane by wrapping the statusline command with a Go forwarder shim that POSTs Claude's statusline JSON back to MTUI, attributed by `MULTITERMINAL_SESSION_ID`.

**Architecture:** A new compiled shim `cmd/statusline-forward` is registered as Claude's `statusLine.command`, wrapping the real statusline (MTUI's PS1 or the user's own). It tees Claude's stdin: pipes it to the wrapped command (display unchanged) and fire-and-forget POSTs a copy to a new localhost `/api/statusline` endpoint. The handler writes the official cost/context/model onto the `Session` and marks `costSource=statusline` so the screen-scrape can't clobber it. The pipeline `active→done` edge is untouched.

**Tech Stack:** Go 1.26 (stdlib `net/http`, `os/exec`), Wails v3 events, existing localhost tmux-API server pattern.

## Global Constraints

- **Platform:** Windows (primary). CLI/statusline invoked through `cmd.exe`/`powershell`.
- **Max 300 lines per Go file.** Split by responsibility.
- **Never block or allocate under `Session.mu`.** Read/parse outside the lock; lock only for field assignment.
- **The pipeline `active→done` edge must stay byte-identical.** It is driven by `DetectActivity`/`HasHookData` in `app_scan.go`; `processQueue`/`notifyOrchestratorDone` fire only on a fresh `done` transition and read `prevActivity`, never token state. Cost updates must not change this.
- **Forwarder is fire-and-forget and failure-silent.** A down/stale MTUI must never break or delay Claude's statusline display.
- **Structs that cross a Wails binding return need `json`+`yaml` tags and manual `models.ts` sync. Event payloads do NOT.** `terminal:activity` is an event — no `models.ts` change.

---

### Task 1: `statusline-forward` shim — passthrough + best-effort POST

**Files:**
- Create: `cmd/statusline-forward/main.go`
- Test: `cmd/statusline-forward/main_test.go`

**Interfaces:**
- Produces: a binary `statusline-forward(.exe)`. Invoked as `statusline-forward <wrapped-cmd> [args...]`; reads Claude's statusline JSON on stdin; relays the wrapped command's stdout/exit; POSTs `{"sessionId":<int>,"payload":<raw json>}` to `http://127.0.0.1:$MTUI_PORT/api/statusline`.
- Internal testable funcs: `runWrapped(args []string, stdin []byte, stdout, stderr io.Writer) int` and `forward(payload []byte, port, sid string)`.

- [ ] **Step 1: Write the failing tests**

```go
// cmd/statusline-forward/main_test.go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunWrappedPassesThroughStdoutAndExit(t *testing.T) {
	var out bytes.Buffer
	// `cmd /c echo hi` prints "hi"; exit 0.
	code := runWrapped([]string{"cmd", "/c", "echo", "hi"}, []byte("{}"), &out, io.Discard)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "hi") {
		t.Fatalf("stdout = %q, want it to contain %q", out.String(), "hi")
	}
}

func TestRunWrappedNoArgsIsNoop(t *testing.T) {
	var out bytes.Buffer
	if code := runWrapped(nil, []byte("{}"), &out, io.Discard); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestForwardPostsSessionIdAndPayload(t *testing.T) {
	type got struct {
		SessionID int             `json:"sessionId"`
		Payload   json.RawMessage `json:"payload"`
	}
	ch := make(chan got, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var g got
		_ = json.NewDecoder(r.Body).Decode(&g)
		ch <- g
	}))
	defer srv.Close()
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	forward([]byte(`{"cost":{"total_cost_usd":0.42}}`), port, "7")

	select {
	case g := <-ch:
		if g.SessionID != 7 {
			t.Fatalf("sessionId = %d, want 7", g.SessionID)
		}
		if !strings.Contains(string(g.Payload), "0.42") {
			t.Fatalf("payload = %s, want it to contain 0.42", g.Payload)
		}
	default:
		t.Fatal("server received no request")
	}
}

func TestForwardDeadPortIsSilent(t *testing.T) {
	// Port 1 is not listening; forward must return promptly without panic.
	forward([]byte(`{}`), "1", "7")
}

func TestForwardMissingEnvIsNoop(t *testing.T) {
	forward([]byte(`{}`), "", "")   // no port/sid -> no request, no panic
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/statusline-forward/...`
Expected: FAIL — `undefined: runWrapped`, `undefined: forward`.

- [ ] **Step 3: Write the implementation**

```go
// cmd/statusline-forward/main.go
// Command statusline-forward wraps a Claude Code statusLine command: it relays
// the wrapped command's output (so the displayed status line is unchanged) and
// fire-and-forget POSTs Claude's statusline JSON to MTUI for telemetry capture.
// It must never block or break the wrapped statusline.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func main() {
	data, _ := io.ReadAll(os.Stdin)
	// Display first: relay the wrapped statusline so the POST never delays it.
	code := runWrapped(os.Args[1:], data, os.Stdout, os.Stderr)
	// Then capture, best-effort.
	forward(data, os.Getenv("MTUI_PORT"), os.Getenv("MULTITERMINAL_SESSION_ID"))
	os.Exit(code)
}

// runWrapped runs args[0] with args[1:], feeding stdin and relaying stdout/stderr.
// Returns the wrapped process exit code (0 when there is no wrapped command).
func runWrapped(args []string, stdin []byte, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return 0
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// forward POSTs the raw statusline JSON to MTUI. Silent on any failure.
func forward(payload []byte, port, sid string) {
	if port == "" || sid == "" {
		return
	}
	id, err := strconv.Atoi(sid)
	if err != nil {
		return
	}
	body, err := json.Marshal(struct {
		SessionID int             `json:"sessionId"`
		Payload   json.RawMessage `json:"payload"`
	}{SessionID: id, Payload: json.RawMessage(payload)})
	if err != nil {
		return
	}
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Post("http://127.0.0.1:"+port+"/api/statusline",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/statusline-forward/...`
Expected: PASS (all 5).

- [ ] **Step 5: Commit**

```bash
git add cmd/statusline-forward/
git commit -m "feat(statusline): add statusline-forward shim (passthrough + best-effort POST)"
```

---

### Task 2: Session statusline fields, setter/getters, and ScanTokens gate

**Files:**
- Modify: `internal/terminal/session.go` (add fields to the `Session` struct, ~line 62-66)
- Create: `internal/terminal/session_statusline.go`
- Modify: `internal/terminal/activity.go` (gate the cost write in `ScanTokens`, ~line 46-54)
- Test: `internal/terminal/session_statusline_test.go`

**Interfaces:**
- Consumes: `Session.mu`, `Session.Tokens TokenInfo` (`{TotalCost, InputTokens, OutputTokens}`), `Session.Screen.Write([]byte)`.
- Produces:
  - `type CostSource int; const (CostSourceScrape CostSource = iota; CostSourceStatusline)`
  - `func (s *Session) SetStatuslineData(cost float64, contextPct int, model string)`
  - `func (s *Session) StatuslineInfo() (contextPct int, model string, src CostSource)`

- [ ] **Step 1: Add the fields to the Session struct**

In `internal/terminal/session.go`, inside `type Session struct`, after the `Tokens TokenInfo` field (line 62):

```go
	// Statusline-sourced telemetry (Claude's statusLine JSON via the forwarder shim).
	// When costSource == CostSourceStatusline, the screen scrape must not overwrite cost.
	statuslineCost float64
	contextPct     int
	model          string
	costSource     CostSource
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/terminal/session_statusline_test.go
package terminal

import "testing"

func TestSetStatuslineDataSetsFieldsAndSource(t *testing.T) {
	s := NewSession(1, 24, 80)
	s.SetStatuslineData(0.42, 35, "claude-opus-4-8")

	if got := s.GetTokens().TotalCost; got != 0.42 {
		t.Fatalf("TotalCost = %v, want 0.42", got)
	}
	pct, model, src := s.StatuslineInfo()
	if pct != 35 || model != "claude-opus-4-8" || src != CostSourceStatusline {
		t.Fatalf("StatuslineInfo() = (%d,%q,%d), want (35,claude-opus-4-8,statusline)", pct, model, src)
	}
}

func TestScanTokensDoesNotOverwriteStatuslineCost(t *testing.T) {
	s := NewSession(1, 24, 80)
	s.SetStatuslineData(0.42, 0, "")
	// Put a different cost on screen; the scrape must NOT win.
	_, _ = s.Screen.Write([]byte("context left: $0.99\r\n"))

	s.ScanTokens()

	if got := s.GetTokens().TotalCost; got != 0.42 {
		t.Fatalf("TotalCost = %v after ScanTokens, want 0.42 (statusline authoritative)", got)
	}
}

func TestScanTokensStillScrapesWhenNoStatusline(t *testing.T) {
	s := NewSession(1, 24, 80)
	_, _ = s.Screen.Write([]byte("cost: $0.99\r\n"))

	s.ScanTokens()

	if got := s.GetTokens().TotalCost; got != 0.99 {
		t.Fatalf("TotalCost = %v, want 0.99 (scrape active)", got)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/terminal/ -run 'Statusline|ScanTokens' -v`
Expected: FAIL — `undefined: SetStatuslineData`, `undefined: CostSourceStatusline`.

- [ ] **Step 4: Write the setter/getters**

```go
// internal/terminal/session_statusline.go
package terminal

// CostSource records which source last set the session's cost, so the screen
// scrape (ScanTokens) does not clobber an authoritative statusline value.
type CostSource int

const (
	CostSourceScrape     CostSource = iota // cost derived from screen scraping
	CostSourceStatusline                   // cost from Claude's statusLine JSON
)

// SetStatuslineData records official cost/context/model from Claude's statusLine
// and marks the statusline as the authoritative cost source. Called from the
// /api/statusline HTTP handler; assignment only, no I/O under the lock.
func (s *Session) SetStatuslineData(cost float64, contextPct int, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuslineCost = cost
	s.contextPct = contextPct
	s.model = model
	s.costSource = CostSourceStatusline
	s.Tokens.TotalCost = cost // single displayed cost field stays authoritative
}

// StatuslineInfo returns the statusline-sourced context%/model and the current
// cost source.
func (s *Session) StatuslineInfo() (contextPct int, model string, src CostSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contextPct, s.model, s.costSource
}
```

- [ ] **Step 5: Gate the cost scrape in ScanTokens**

In `internal/terminal/activity.go`, replace the cost-match block (currently ~lines 49-54, after `s.mu.Lock()`):

```go
	// Look for cost patterns like $0.12 or $1.50 — but never overwrite an
	// authoritative statusline-sourced cost.
	if s.costSource != CostSourceStatusline {
		if matches := costPattern.FindStringSubmatch(content); len(matches) >= 2 {
			if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
				s.Tokens.TotalCost = v
			}
		}
	}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/terminal/ -run 'Statusline|ScanTokens' -v`
Expected: PASS (3).

- [ ] **Step 7: Run the full terminal package under race**

Run: `go test -race ./internal/terminal/...`
Expected: PASS, no race warnings.

- [ ] **Step 8: Commit**

```bash
git add internal/terminal/session.go internal/terminal/session_statusline.go internal/terminal/activity.go internal/terminal/session_statusline_test.go
git commit -m "feat(statusline): Session cost source + statusline data, gate scrape"
```

---

### Task 3: `/api/statusline` endpoint and handler

**Files:**
- Modify: `internal/backend/app_tmux_api.go` (register one route in `startTmuxAPI`, line 22)
- Create: `internal/backend/app_statusline_api.go` (payload type + handler)
- Test: `internal/backend/app_statusline_api_test.go`

**Interfaces:**
- Consumes: `a.mu`, `a.sessions map[int]*terminal.Session` (lookup pattern from `app.go:242`), `Session.SetStatuslineData(cost, contextPct, model)`.
- Produces: `POST /api/statusline` accepting `{"sessionId":int,"payload":<claude statusline json>}`; calls `SetStatuslineData`.

- [ ] **Step 1: Write the failing test**

```go
// internal/backend/app_statusline_api_test.go
package backend

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func TestHandleStatuslineUpdatesSession(t *testing.T) {
	a := &AppService{sessions: map[int]*terminal.Session{}}
	sess := terminal.NewSession(5, 24, 80)
	a.sessions[5] = sess

	body := `{"sessionId":5,"payload":{"cost":{"total_cost_usd":1.23},` +
		`"context_window":{"used_percentage":40},"model":{"display_name":"Opus 4.8"}}}`
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(body))
	rec := httptest.NewRecorder()

	a.handleStatusline(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := sess.GetTokens().TotalCost; got != 1.23 {
		t.Fatalf("TotalCost = %v, want 1.23", got)
	}
	pct, model, src := sess.StatuslineInfo()
	if pct != 40 || model != "Opus 4.8" || src != terminal.CostSourceStatusline {
		t.Fatalf("StatuslineInfo = (%d,%q,%d), want (40,Opus 4.8,statusline)", pct, model, src)
	}
}

func TestHandleStatuslineUnknownSessionNoCrash(t *testing.T) {
	a := &AppService{sessions: map[int]*terminal.Session{}}
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(`{"sessionId":99,"payload":{}}`))
	rec := httptest.NewRecorder()
	a.handleStatusline(rec, req) // must not panic
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleStatuslineGarbageBodyIsBadRequest(t *testing.T) {
	a := &AppService{sessions: map[int]*terminal.Session{}}
	req := httptest.NewRequest("POST", "/api/statusline", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	a.handleStatusline(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backend/ -run Statusline -v`
Expected: FAIL — `a.handleStatusline undefined`.

- [ ] **Step 3: Write the handler**

```go
// internal/backend/app_statusline_api.go
package backend

import (
	"encoding/json"
	"net/http"
)

// statuslinePayload is MTUI's wrapper around Claude's raw statusLine JSON,
// posted by the statusline-forward shim. Only the fields MTUI consumes are typed;
// unknown fields are ignored.
type statuslinePayload struct {
	SessionID int `json:"sessionId"`
	Payload   struct {
		Cost struct {
			TotalCostUSD  float64 `json:"total_cost_usd"`
			TotalDuration int     `json:"total_duration_ms"`
		} `json:"cost"`
		ContextWindow struct {
			UsedPercentage float64 `json:"used_percentage"`
		} `json:"context_window"`
		Model struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"payload"`
}

// handleStatusline records cost/context/model from a forwarded statusLine update.
// Loopback-only, POST-only, no auth (same trust model as /api/tmux/log).
func (a *AppService) handleStatusline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var p statuslinePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	sess := a.sessions[p.SessionID]
	a.mu.Unlock()
	if sess != nil {
		sess.SetStatuslineData(
			p.Payload.Cost.TotalCostUSD,
			int(p.Payload.ContextWindow.UsedPercentage),
			p.Payload.Model.DisplayName,
		)
	}

	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: Register the route**

In `internal/backend/app_tmux_api.go`, `startTmuxAPI`, after line 22 (`mux.HandleFunc("/api/tmux/log", a.handleTmuxLog)`):

```go
	mux.HandleFunc("/api/statusline", a.handleStatusline)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/backend/ -run Statusline -v`
Expected: PASS (3).

- [ ] **Step 6: Commit**

```bash
git add internal/backend/app_statusline_api.go internal/backend/app_tmux_api.go internal/backend/app_statusline_api_test.go
git commit -m "feat(statusline): /api/statusline endpoint records cost/context/model"
```

---

### Task 4: Wrap the statusline command with the forwarder

**Files:**
- Modify: `internal/backend/app_statusline.go` (`applyStatusLine`, lines 81-83 — build the wrapping command)
- Create: `internal/backend/app_statusline_wrap.go` (forwarder-path resolution + command builder)
- Test: `internal/backend/app_statusline_wrap_test.go`

**Interfaces:**
- Consumes: `GetStatusLineStatus()` → `{HasExisting bool, IsOurs bool, ExistingCommand string}`; `statusLineScriptPath()`.
- Produces: `func statuslineForwardPath() string` (the shim next to the running exe); `func wrapStatuslineCommand(forwarder, inner string) string` (the `statusLine.command` string).

- [ ] **Step 1: Write the failing test**

```go
// internal/backend/app_statusline_wrap_test.go
package backend

import (
	"strings"
	"testing"
)

func TestWrapStatuslineCommandPrependsForwarder(t *testing.T) {
	inner := `powershell -NonInteractive -NoProfile -File "C:/Users/x/.claude/mtui-statusline.ps1"`
	got := wrapStatuslineCommand(`C:/Users/x/AppData/.../statusline-forward.exe`, inner)

	if !strings.HasPrefix(got, `"C:/Users/x/AppData/.../statusline-forward.exe" `) {
		t.Fatalf("command = %q, want it to start with the quoted forwarder path", got)
	}
	if !strings.HasSuffix(got, inner) {
		t.Fatalf("command = %q, want it to end with the wrapped inner command %q", got, inner)
	}
}

func TestWrapStatuslineCommandEmptyForwarderReturnsInner(t *testing.T) {
	// Fail-safe: no forwarder available -> register the inner command unchanged.
	inner := `powershell -File foo.ps1`
	if got := wrapStatuslineCommand("", inner); got != inner {
		t.Fatalf("command = %q, want unchanged inner %q", got, inner)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backend/ -run WrapStatusline -v`
Expected: FAIL — `wrapStatuslineCommand undefined`.

- [ ] **Step 3: Write the wrapper helpers**

```go
// internal/backend/app_statusline_wrap.go
package backend

import (
	"os"
	"path/filepath"
	"strings"
)

// statuslineForwardPath returns the path to the statusline-forward shim, which
// is built alongside the main binary in the same directory. Returns "" if it
// cannot be located, in which case the statusline is registered unwrapped.
func statuslineForwardPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "statusline-forward"
	if filepath.Ext(exe) == ".exe" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return filepath.ToSlash(p)
}

// wrapStatuslineCommand builds the statusLine.command string: the quoted
// forwarder followed by the inner (real) statusline command it wraps. If
// forwarder is "", the inner command is returned unchanged (fail-safe).
func wrapStatuslineCommand(forwarder, inner string) string {
	if forwarder == "" {
		return inner
	}
	return `"` + forwarder + `" ` + inner
}
```

- [ ] **Step 4: Use the wrapper in applyStatusLine**

In `internal/backend/app_statusline.go`, replace lines 81-83:

```go
	// Use forward slashes so PowerShell resolves the path correctly on Windows.
	fwdPath := strings.ReplaceAll(scriptPath, `\`, `/`)
	command := `powershell -NonInteractive -NoProfile -File "` + fwdPath + `"`
```

with:

```go
	// Use forward slashes so PowerShell resolves the path correctly on Windows.
	fwdPath := strings.ReplaceAll(scriptPath, `\`, `/`)
	inner := `powershell -NonInteractive -NoProfile -File "` + fwdPath + `"`
	// Wrap with the forwarder shim so MTUI captures Claude's statusline telemetry.
	// If the user already has a statusline, wrap THAT instead so capture still works.
	if st := a.GetStatusLineStatus(); st.HasExisting && !st.IsOurs && st.ExistingCommand != "" {
		inner = st.ExistingCommand
	}
	command := wrapStatuslineCommand(statuslineForwardPath(), inner)
```

- [ ] **Step 5: Run tests + build to verify**

Run: `go test ./internal/backend/ -run 'WrapStatusline|Statusline' -v && go build ./...`
Expected: PASS; build succeeds.

- [ ] **Step 6: Add the shim to the build pipeline**

In `Taskfile.yml`, in the `build` task, add a step to compile the shim into the same output dir as the main binary (next to `mtui-portable.exe` / `multiterminal.exe`):

```yaml
      - go build -o build/bin/statusline-forward.exe ./cmd/statusline-forward
```

Place it immediately after the main `go build` step so both land in `build/bin/`.

- [ ] **Step 7: Commit**

```bash
git add internal/backend/app_statusline.go internal/backend/app_statusline_wrap.go internal/backend/app_statusline_wrap_test.go Taskfile.yml
git commit -m "feat(statusline): wrap statusline command with forwarder shim"
```

---

### Task 5: Emit context% and model on the activity event

**Files:**
- Modify: `internal/backend/app_scan.go` (`ActivityInfo` struct lines 14-19; `scanAllSessions` emit ~lines 125-154)
- Test: `internal/backend/app_scan_test.go` (add a focused test; create the file if absent)

**Interfaces:**
- Consumes: `Session.StatuslineInfo() (int, string, CostSource)`, existing `GetTokens()`.
- Produces: `ActivityInfo` gains `ContextPct int` and `Model string`; emitted on the existing `terminal:activity` event (no `models.ts` change — it is an event payload, consumed via `EventsOn` in `App.svelte`).

- [ ] **Step 1: Extend the ActivityInfo struct**

In `internal/backend/app_scan.go`, lines 14-19:

```go
// ActivityInfo is sent to the frontend when a session's activity state changes.
type ActivityInfo struct {
	ID         int    `json:"id"`
	Activity   string `json:"activity"` // "idle", "active", "done", "waitingPermission", "waitingAnswer", "error"
	Cost       string `json:"cost"`
	Title      string `json:"title"`      // OSC-derived window title (fallback pane name)
	ContextPct int    `json:"contextPct"` // % of context window used (statusline); 0 if unknown
	Model      string `json:"model"`      // model display name (statusline); "" if unknown
}
```

- [ ] **Step 2: Write the failing test**

```go
// internal/backend/app_scan_test.go
package backend

import "testing"

func TestActivityInfoCarriesStatuslineFields(t *testing.T) {
	// Guards that the event payload exposes context%/model so the frontend can render them.
	info := ActivityInfo{ID: 1, Activity: "active", Cost: "$1.23", ContextPct: 40, Model: "Opus 4.8"}
	if info.ContextPct != 40 || info.Model != "Opus 4.8" {
		t.Fatalf("ActivityInfo = %+v, want ContextPct=40 Model=Opus 4.8", info)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/backend/ -run ActivityInfoCarries -v`
Expected: FAIL — `unknown field ContextPct` (until Step 1 is saved) → after Step 1, this test passes; its purpose is to lock the field names. If Step 1 is already saved, confirm it PASSES and proceed.

- [ ] **Step 4: Populate the fields in scanAllSessions**

In `internal/backend/app_scan.go`, in `scanAllSessions`, after `tokens := sess.GetTokens()` (line 125) add:

```go
		ctxPct, model, _ := sess.StatuslineInfo()
```

Then extend the emitted struct in the `a.app.Event.Emit("terminal:activity", ...)` call (lines 148-153) to include the new fields:

```go
			a.app.Event.Emit("terminal:activity", ActivityInfo{
				ID:         id,
				Activity:   actStr,
				Cost:       costStr,
				Title:      title,
				ContextPct: ctxPct,
				Model:      model,
			})
```

- [ ] **Step 5: Run the backend tests + done-edge regression guard**

Run: `go test ./internal/backend/...`
Expected: PASS. In particular, the existing queue/scan tests that assert `processQueue` fires on the `done` transition must still pass — confirm none regressed (the cost/context/model additions do not touch `activityChanged`/`actStr` logic).

- [ ] **Step 6: Commit**

```bash
git add internal/backend/app_scan.go internal/backend/app_scan_test.go
git commit -m "feat(statusline): emit context% and model on terminal:activity"
```

---

### Task 6: End-to-end verification gate (runtime, manual)

**Files:** none (verification only). This task gates the branch; it produces a short verification note, not code.

**Why:** The forwarder's value rests on two things that unit tests cannot prove: (a) the env vars `MULTITERMINAL_SESSION_ID`/`MTUI_PORT` actually reach the statusline child through the PTY → `cmd.exe /c claude` → statusline chain; (b) Claude invokes the wrapped command correctly (quoting survives) and the display is unchanged.

- [ ] **Step 1: Build and run**

Run:
```bash
cd frontend && npm run build && cd ..
go build -o build/bin/multiterminal.exe -tags desktop .
go build -o build/bin/statusline-forward.exe ./cmd/statusline-forward
build/bin/multiterminal.exe
```

- [ ] **Step 2: Verify capture end-to-end**

In the running app, open a Claude (yolo) pane in a real git repo and send one prompt so an assistant turn completes (triggers a statusline render). Then confirm in the app log that `/api/statusline` was hit and the pane's cost/model updated:

Expected: a log line from the handler (add a `log.Printf("[statusline] session %d cost=%.4f model=%q", ...)` temporarily if needed) AND the pane footer/title shows a non-`$0.00` cost and the model name. The statusline at the bottom of the Claude TUI must still render normally (wrap is transparent).

- [ ] **Step 3: Verify failure-silence**

Stop the app's API (or note that on the next app restart the port changes); confirm a still-running Claude pane's statusline keeps rendering with no hang/stall, and the pane simply stops updating cost (no crash, no error dialog).

- [ ] **Step 4: Record the result**

Append a short "E2E verified on <date>: capture works, wrap transparent, fail-silent confirmed" note to the PR description. If quoting broke the wrapped invocation (statusline blank or error), fix `wrapStatuslineCommand`/`applyStatusLine` quoting and re-verify before merge.

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix(statusline): e2e quoting/inheritance fixes from runtime verification"
```

---

## Self-Review

**Spec coverage (Rev 4 Sub-plan 1):**
- Go forwarder shim, tee + best-effort POST → Task 1. ✓
- Wrap existing statusline (D7) → Task 4. ✓
- `/api/statusline` endpoint, session lookup → Task 3. ✓
- `Session.SetStatuslineData` + new fields + `costSource` gate on `ScanTokens` → Task 2. ✓
- Event (not binding) for context%/model; no `models.ts` change → Task 5. ✓
- `done`-edge untouched (regression guard) → Task 5 Step 5. ✓
- Blocker: end-to-end env inheritance + non-blocking-under-cancellation → Task 6. ✓
- Build pipeline ships the shim next to the exe → Task 4 Step 6. ✓

**Placeholder scan:** No TBD/TODO; every code step has complete code; every command has expected output. ✓

**Type consistency:** `SetStatuslineData(cost float64, contextPct int, model string)`, `StatuslineInfo() (int, string, CostSource)`, `CostSourceStatusline`, `statuslinePayload`, `wrapStatuslineCommand(forwarder, inner string) string`, `statuslineForwardPath() string`, `ActivityInfo.ContextPct/.Model` — used identically across Tasks 2–5. ✓

**Note on Task 5 Step 3:** the `ActivityInfoCarries` test only compiles after the struct change in Step 1; treat Step 1+2 as written together (the test locks the field names). This is intentional, not a placeholder.
