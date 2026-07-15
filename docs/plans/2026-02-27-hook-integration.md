# Hook Integration & Precise Status Enum Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace PTY-regex activity detection for Claude panes with Claude Code's native hook system, adding `waitingPermission`, `waitingAnswer`, and `error` states for precise UI feedback.

**Architecture:** Claude Code hooks write JSONL events to `%APPDATA%\Multiterminal\hooks\<session_id>.jsonl`. The Go backend polls this directory (100ms) and matches events to sessions via a `MULTITERMINAL_SESSION_ID` env var injected at Claude pane launch. PTY scan is retained for token/cost data and Shell panes.

**Tech Stack:** Go 1.25, PowerShell (hook script), Svelte 4 + TypeScript (frontend), `embed.FS` for script bundling.

**Design Doc:** `docs/plans/2026-02-27-hook-integration-design.md`

---

## Task 1: Extend ActivityState enum — rename NeedsInput, add new states

**Files:**
- Modify: `internal/terminal/activity.go:17-25`

### Step 1: Update the enum

Replace the `ActivityNeedsInput` constant with three new constants:

```go
const (
    ActivityIdle              ActivityState = iota // no recent output
    ActivityActive                                 // currently producing output
    ActivityDone                                   // just finished (prompt returned)
    ActivityWaitingPermission                      // tool needs user approval
    ActivityWaitingAnswer                          // waiting for text input from user
    ActivityError                                  // tool execution failed
)
```

Remove `ActivityNeedsInput` entirely. In `classifyScreenState()` (line ~119), change the return to `ActivityWaitingAnswer`:

```go
if needsInputPattern.MatchString(trimmed) {
    return ActivityWaitingAnswer
}
```

### Step 2: Fix compilation errors in Go files

Run: `go build ./...`

Expected: compile errors in these files (fix them now):
- `internal/backend/app_scan.go` — update `activityString()` case
- `internal/terminal/activity_test.go` — rename constant references
- `internal/terminal/activity_realistic_test.go` — rename constant references
- `internal/backend/app_scan_test.go` — rename constant references
- `internal/backend/app_scan_extended_test.go` — rename constant references

In `internal/backend/app_scan.go`, update `activityString()`:

```go
func activityString(a terminal.ActivityState) string {
    switch a {
    case terminal.ActivityActive:
        return "active"
    case terminal.ActivityDone:
        return "done"
    case terminal.ActivityWaitingPermission:
        return "waitingPermission"
    case terminal.ActivityWaitingAnswer:
        return "waitingAnswer"
    case terminal.ActivityError:
        return "error"
    default:
        return "idle"
    }
}
```

Also update the comment on `ActivityInfo.Activity` field:
```go
Activity string `json:"activity"` // "idle", "active", "done", "waitingPermission", "waitingAnswer", "error"
```

In all test files, rename `ActivityNeedsInput` → `ActivityWaitingAnswer`.

### Step 3: Run all Go tests

Run: `go test ./internal/terminal/... ./internal/backend/... -v 2>&1 | tail -30`

Expected: all tests PASS (the enum value `ActivityWaitingAnswer` has the same iota value as the old `ActivityNeedsInput`, so existing tests still pass).

### Step 4: Commit

```bash
git add internal/terminal/activity.go internal/backend/app_scan.go \
    internal/terminal/activity_test.go internal/terminal/activity_realistic_test.go \
    internal/backend/app_scan_test.go internal/backend/app_scan_extended_test.go
git commit -m "refactor(activity): rename NeedsInput→WaitingAnswer, add WaitingPermission+Error states"
```

---

## Task 2: Add hook fields to Session struct

**Files:**
- Modify: `internal/terminal/session.go`

### Step 1: Write failing test

Add to `internal/terminal/activity_test.go` (at the end):

```go
func TestSession_HookData(t *testing.T) {
    s := NewSession(1, 24, 80)

    // Initially no hook data
    if s.HasHookData() {
        t.Fatal("new session should not have hook data")
    }

    // Set hook activity
    s.SetHookActivity(ActivityWaitingPermission)
    if !s.HasHookData() {
        t.Fatal("session should have hook data after SetHookActivity")
    }

    s.mu.Lock()
    got := s.Activity
    s.mu.Unlock()
    if got != ActivityWaitingPermission {
        t.Errorf("Activity = %d, want ActivityWaitingPermission (%d)", got, ActivityWaitingPermission)
    }

    // ClearHookData resets flag
    s.ClearHookData()
    if s.HasHookData() {
        t.Fatal("session should not have hook data after ClearHookData")
    }
}
```

Run: `go test ./internal/terminal/... -run TestSession_HookData -v`
Expected: FAIL — `SetHookActivity`, `HasHookData`, `ClearHookData` undefined

### Step 2: Add fields and methods to session.go

In `internal/terminal/session.go`, add to the `Session` struct (after the `Tokens` field):

```go
// hookSessionID is the Claude Code session UUID from hook events.
// Empty until the first hook event is received.
hookSessionID string

// hasHookData is true once a hook event has updated Activity.
// When true, the PTY scan loop skips DetectActivity() for this session.
hasHookData bool
```

Add these methods at the end of session.go:

```go
// SetHookActivity updates the activity state from a hook event.
// Marks the session as having authoritative hook data.
func (s *Session) SetHookActivity(state ActivityState) {
    s.mu.Lock()
    s.Activity = state
    s.hasHookData = true
    s.mu.Unlock()
}

// HasHookData reports whether hook events have set the activity state.
func (s *Session) HasHookData() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.hasHookData
}

// ClearHookData resets the hook data flag (e.g. when session ends).
func (s *Session) ClearHookData() {
    s.mu.Lock()
    s.hasHookData = false
    s.mu.Unlock()
}

// SetHookSessionID stores the Claude Code session UUID for this session.
func (s *Session) SetHookSessionID(id string) {
    s.mu.Lock()
    s.hookSessionID = id
    s.mu.Unlock()
}

// HookSessionID returns the Claude Code session UUID, empty if not yet set.
func (s *Session) HookSessionID() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.hookSessionID
}
```

### Step 3: Run test

Run: `go test ./internal/terminal/... -run TestSession_HookData -v`
Expected: PASS

### Step 4: Run full test suite

Run: `go test ./internal/terminal/... ./internal/backend/... -v 2>&1 | tail -20`
Expected: all PASS

### Step 5: Commit

```bash
git add internal/terminal/session.go internal/terminal/activity_test.go
git commit -m "feat(session): add hook data fields and methods to Session"
```

---

## Task 3: PTY scan guard — skip DetectActivity when hooks active

**Files:**
- Modify: `internal/backend/app_scan.go`

### Step 1: Write failing test

Add to `internal/backend/app_scan_test.go`:

```go
func TestScanSkipsDetectWhenHookData(t *testing.T) {
    sess := terminal.NewSession(99, 24, 80)
    // Mark session as having hook-driven state
    sess.SetHookActivity(terminal.ActivityWaitingPermission)

    // Simulate the guard: if HasHookData, DetectActivity should NOT be called.
    // We verify by checking the state remains WaitingPermission after a scan.
    // (DetectActivity would reset it to Idle since there's no PTY output.)
    if !sess.HasHookData() {
        t.Fatal("precondition: session must have hook data")
    }

    // After SetHookActivity, Activity must be WaitingPermission
    sess.DetectActivity() // this WOULD change state if not guarded

    // State should still be WaitingPermission because HasHookData=true
    // NOTE: This test documents intended behavior — the guard is in scanAllSessions,
    // not in DetectActivity itself. This test verifies DetectActivity does NOT
    // reset hook-driven state when hook data is present.
    // We implement the guard in scanAllSessions() in the next step.
    _ = sess
}
```

Actually, the guard is in `scanAllSessions`, not `DetectActivity`. Replace the test above with a unit test that verifies `activityString` for the new states, and add an integration note:

```go
func TestActivityStringNewStates(t *testing.T) {
    tests := []struct {
        state terminal.ActivityState
        want  string
    }{
        {terminal.ActivityWaitingPermission, "waitingPermission"},
        {terminal.ActivityWaitingAnswer, "waitingAnswer"},
        {terminal.ActivityError, "error"},
    }
    for _, tt := range tests {
        got := activityString(tt.state)
        if got != tt.want {
            t.Errorf("activityString(%d) = %q, want %q", tt.state, got, tt.want)
        }
    }
}
```

Run: `go test ./internal/backend/... -run TestActivityStringNewStates -v`
Expected: PASS (already implemented in Task 1)

### Step 2: Add the hook guard to scanAllSessions

In `internal/backend/app_scan.go`, in `scanAllSessions()`, wrap `DetectActivity()` with the guard:

```go
// Only use PTY-based detection if no hook data is available.
// Hook events provide more accurate state for Claude panes.
var activity terminal.ActivityState
if sess.HasHookData() {
    sess.mu.Lock()
    activity = sess.Activity
    sess.mu.Unlock()
} else {
    sess.ScanTokens()
    activity = sess.DetectActivity()
}
actStr := activityString(activity)
```

Wait — `ScanTokens()` should run regardless (hooks don't provide token data). Update:

```go
sess.ScanTokens() // always scan for token/cost data

var activity terminal.ActivityState
if sess.HasHookData() {
    // Hook events drive activity state for Claude panes — skip PTY regex scan
    sess.mu.Lock()
    activity = sess.Activity
    sess.mu.Unlock()
} else {
    activity = sess.DetectActivity()
}
actStr := activityString(activity)
```

Remove the original `sess.ScanTokens()` line (which was before this block).

### Step 3: Run tests

Run: `go test ./internal/backend/... -v 2>&1 | tail -20`
Expected: all PASS

### Step 4: Commit

```bash
git add internal/backend/app_scan.go internal/backend/app_scan_test.go
git commit -m "feat(scan): skip DetectActivity when hook data present, always scan tokens"
```

---

## Task 4: Inject MULTITERMINAL_SESSION_ID into Claude pane environment

**Files:**
- Modify: `internal/backend/app.go` (the `LaunchSession` or equivalent method)

### Step 1: Find where Claude sessions are launched

In `internal/backend/app.go`, search for where `session.Start()` is called with `argv` containing `claude`. The env vars array passed to `Start()` needs `MULTITERMINAL_SESSION_ID=<id>`.

Read `app.go` fully to find the `LaunchSession` / `StartSession` method, then add:

```go
// For Claude panes, inject session ID so hook scripts can match events back.
if isClaude {
    env = append(env, fmt.Sprintf("MULTITERMINAL_SESSION_ID=%d", id))
}
```

Where `id` is the Multiterminal session integer ID and `isClaude` is true when mode is `claude` or `claude-yolo`.

### Step 2: Verify it builds

Run: `go build ./...`
Expected: no errors

### Step 3: Commit

```bash
git add internal/backend/app.go
git commit -m "feat(session): inject MULTITERMINAL_SESSION_ID env var for Claude panes"
```

---

## Task 5: PowerShell hook script (embedded)

**Files:**
- Create: `internal/backend/hooks/hook_handler.ps1`
- Create: `internal/backend/hooks/embed.go`

### Step 1: Create the embed package

Create `internal/backend/hooks/embed.go`:

```go
package hooks

import _ "embed"

//go:embed hook_handler.ps1
var HookHandlerScript string
```

### Step 2: Create the PowerShell script

Create `internal/backend/hooks/hook_handler.ps1`:

```powershell
# Multiterminal Claude Code hook handler
# Reads hook event from stdin, appends JSONL line to hooks directory.
# Called by Claude Code for: PreToolUse, PostToolUse, PostToolUseFailure,
#   PermissionRequest, Notification, Stop, SessionEnd

param([string]$EventType)

try {
    # Read stdin JSON from Claude Code
    $stdin = [Console]::In.ReadToEnd()
    $data = $stdin | ConvertFrom-Json -ErrorAction Stop

    # Determine hooks directory
    $hooksDir = Join-Path $env:APPDATA "Multiterminal\hooks"
    if (-not (Test-Path $hooksDir)) {
        New-Item -ItemType Directory -Path $hooksDir -Force | Out-Null
    }

    # Build JSONL payload
    $sessionId = if ($data.session_id) { $data.session_id } else { "unknown" }
    $mtSessionId = if ($env:MULTITERMINAL_SESSION_ID) { [int]$env:MULTITERMINAL_SESSION_ID } else { 0 }
    $toolName = if ($data.tool_name) { $data.tool_name } else { "" }
    $message = if ($data.message) { $data.message } else { "" }
    $ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()

    $payload = [ordered]@{
        ts         = $ts
        event      = $EventType
        session_id = $sessionId
        mt_id      = $mtSessionId
        tool       = $toolName
        message    = $message
    }
    $line = $payload | ConvertTo-Json -Compress

    # Append to JSONL file (one file per Claude session)
    $file = Join-Path $hooksDir "$sessionId.jsonl"
    Add-Content -Path $file -Value $line -Encoding UTF8 -NoNewline
    Add-Content -Path $file -Value "`n" -Encoding UTF8 -NoNewline
} catch {
    # Silent failure — never block Claude Code
    exit 0
}
exit 0
```

### Step 3: Verify embed builds

Run: `go build ./internal/backend/hooks/...`
Expected: no errors

### Step 4: Commit

```bash
git add internal/backend/hooks/
git commit -m "feat(hooks): add embedded PowerShell hook handler script"
```

---

## Task 6: Hook installer — register hooks in ~/.claude/settings.json

**Files:**
- Create: `internal/backend/app_hooks_installer.go`
- Create: `internal/backend/app_hooks_installer_test.go`

### Step 1: Write failing tests

Create `internal/backend/app_hooks_installer_test.go`:

```go
package backend

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestHookInstaller_Empty(t *testing.T) {
    dir := t.TempDir()
    settingsPath := filepath.Join(dir, "settings.json")
    // Write empty settings
    os.WriteFile(settingsPath, []byte(`{}`), 0644)

    hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
    if err := hi.Install(); err != nil {
        t.Fatalf("Install() error: %v", err)
    }

    data, _ := os.ReadFile(settingsPath)
    var result map[string]any
    json.Unmarshal(data, &result)
    hooks, ok := result["hooks"].(map[string]any)
    if !ok {
        t.Fatalf("hooks key missing in settings.json")
    }
    if _, exists := hooks["PreToolUse"]; !exists {
        t.Error("PreToolUse hook not installed")
    }
}

func TestHookInstaller_Idempotent(t *testing.T) {
    dir := t.TempDir()
    settingsPath := filepath.Join(dir, "settings.json")
    os.WriteFile(settingsPath, []byte(`{}`), 0644)

    hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
    hi.Install()

    data1, _ := os.ReadFile(settingsPath)
    hi.Install() // second install
    data2, _ := os.ReadFile(settingsPath)

    if string(data1) != string(data2) {
        t.Error("second Install() changed the file — not idempotent")
    }
}

func TestHookInstaller_Backup(t *testing.T) {
    dir := t.TempDir()
    settingsPath := filepath.Join(dir, "settings.json")
    os.WriteFile(settingsPath, []byte(`{"someKey": true}`), 0644)

    hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
    hi.Install()

    // A .bak file should exist
    entries, _ := os.ReadDir(dir)
    hasBak := false
    for _, e := range entries {
        if strings.HasSuffix(e.Name(), ".bak") {
            hasBak = true
        }
    }
    if !hasBak {
        t.Error("no .bak backup file created")
    }
}

func TestHookInstaller_MissingFile(t *testing.T) {
    dir := t.TempDir()
    settingsPath := filepath.Join(dir, "settings.json")
    // File does not exist — installer should create it

    hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
    if err := hi.Install(); err != nil {
        t.Fatalf("Install() on missing file should not error: %v", err)
    }

    if _, err := os.Stat(settingsPath); err != nil {
        t.Error("settings.json should have been created")
    }
}
```

Run: `go test ./internal/backend/... -run TestHookInstaller -v`
Expected: FAIL — `newHookInstaller` undefined

### Step 2: Implement hook installer

Create `internal/backend/app_hooks_installer.go`:

```go
package backend

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"
)

const hookMarker = "# multiterminal-hook"

// hookInstaller manages registration of Multiterminal hooks in ~/.claude/settings.json.
type hookInstaller struct {
    settingsPath string
    command      string // the powershell command to run for each hook event
}

func newHookInstaller(settingsPath, command string) *hookInstaller {
    return &hookInstaller{settingsPath: settingsPath, command: command}
}

// Install adds Multiterminal hook entries to settings.json if not already present.
// Idempotent: calling it multiple times produces the same result.
// Creates a .bak backup before the first modification.
func (h *hookInstaller) Install() error {
    // Read existing settings (create empty if missing)
    var settings map[string]any
    data, err := os.ReadFile(h.settingsPath)
    if err != nil {
        if !os.IsNotExist(err) {
            return fmt.Errorf("read settings: %w", err)
        }
        // Create parent directory and empty settings
        if err := os.MkdirAll(filepath.Dir(h.settingsPath), 0755); err != nil {
            return err
        }
        settings = make(map[string]any)
    } else {
        if err := json.Unmarshal(data, &settings); err != nil {
            return fmt.Errorf("parse settings.json: %w", err)
        }
    }

    // Check if already installed (idempotency check)
    if h.isInstalled(settings) {
        return nil
    }

    // Backup before modification
    if len(data) > 0 {
        backupPath := h.settingsPath + ".bak." + time.Now().Format("20060102-150405")
        if err := os.WriteFile(backupPath, data, 0644); err != nil {
            log.Printf("[hooks] warning: could not create backup: %v", err)
        }
    }

    // Merge hook entries
    h.mergeHooks(settings)

    // Write back
    out, err := json.MarshalIndent(settings, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal settings: %w", err)
    }
    return os.WriteFile(h.settingsPath, out, 0644)
}

// isInstalled checks if the Multiterminal hook marker is present.
func (h *hookInstaller) isInstalled(settings map[string]any) bool {
    hooks, ok := settings["hooks"].(map[string]any)
    if !ok {
        return false
    }
    preToolUse, ok := hooks["PreToolUse"].([]any)
    if !ok || len(preToolUse) == 0 {
        return false
    }
    // Check for our marker in the command string
    first, ok := preToolUse[0].(map[string]any)
    if !ok {
        return false
    }
    innerHooks, ok := first["hooks"].([]any)
    if !ok || len(innerHooks) == 0 {
        return false
    }
    innerFirst, ok := innerHooks[0].(map[string]any)
    if !ok {
        return false
    }
    cmd, _ := innerFirst["command"].(string)
    return len(cmd) > 0 && contains(cmd, hookMarker)
}

// mergeHooks adds Multiterminal hook entries for all relevant events.
func (h *hookInstaller) mergeHooks(settings map[string]any) {
    hooks, ok := settings["hooks"].(map[string]any)
    if !ok {
        hooks = make(map[string]any)
        settings["hooks"] = hooks
    }

    events := []string{
        "PreToolUse", "PostToolUse", "PostToolUseFailure",
        "PermissionRequest", "Notification", "Stop", "SessionEnd",
    }

    for _, event := range events {
        cmd := fmt.Sprintf("%s %s %s", h.command, event, hookMarker)
        entry := map[string]any{
            "hooks": []any{
                map[string]any{
                    "type":    "command",
                    "command": cmd,
                },
            },
        }
        // Prepend to existing array (or create new)
        existing, _ := hooks[event].([]any)
        hooks[event] = append([]any{entry}, existing...)
    }
}

func contains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr ||
        len(s) > 0 && (s[:len(substr)] == substr ||
            s[len(s)-len(substr):] == substr ||
            indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return i
        }
    }
    return -1
}
```

### Step 3: Run tests

Run: `go test ./internal/backend/... -run TestHookInstaller -v`
Expected: all PASS

### Step 4: Run full backend tests

Run: `go test ./internal/backend/... -v 2>&1 | tail -20`
Expected: all PASS

### Step 5: Commit

```bash
git add internal/backend/app_hooks_installer.go internal/backend/app_hooks_installer_test.go
git commit -m "feat(hooks): implement Claude Code hook installer for settings.json"
```

---

## Task 7: HookManager — poll hooks directory, dispatch events

**Files:**
- Create: `internal/backend/app_hooks.go`
- Create: `internal/backend/app_hooks_test.go`

### Step 1: Write failing tests

Create `internal/backend/app_hooks_test.go`:

```go
package backend

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// hookEvent mirrors the JSONL structure written by hook_handler.ps1
type hookEvent struct {
    Ts        int64  `json:"ts"`
    Event     string `json:"event"`
    SessionID string `json:"session_id"`
    MtID      int    `json:"mt_id"`
    Tool      string `json:"tool"`
    Message   string `json:"message"`
}

func writeHookEvent(t *testing.T, dir, sessionID string, ev hookEvent) {
    t.Helper()
    data, _ := json.Marshal(ev)
    line := string(data) + "\n"
    path := filepath.Join(dir, sessionID+".jsonl")
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        t.Fatal(err)
    }
    defer f.Close()
    f.WriteString(line)
}

func TestHookEventToActivity(t *testing.T) {
    tests := []struct {
        event string
        want  terminal.ActivityState
    }{
        {"PreToolUse", terminal.ActivityActive},
        {"PostToolUse", terminal.ActivityActive},
        {"PostToolUseFailure", terminal.ActivityError},
        {"PermissionRequest", terminal.ActivityWaitingPermission},
        {"Notification", terminal.ActivityWaitingAnswer},
        {"Stop", terminal.ActivityDone},
        {"UserPromptSubmit", terminal.ActivityActive},
    }
    for _, tt := range tests {
        got := hookEventToActivity(tt.event)
        if got != tt.want {
            t.Errorf("hookEventToActivity(%q) = %d, want %d", tt.event, got, tt.want)
        }
    }
}

func TestHookManager_ProcessFile(t *testing.T) {
    dir := t.TempDir()
    sess := terminal.NewSession(42, 24, 80)

    hm := newHookManager(dir, func(mtID int) *terminal.Session {
        if mtID == 42 {
            return sess
        }
        return nil
    }, nil)

    // Write a PermissionRequest event with mt_id=42
    writeHookEvent(t, dir, "claude-session-abc", hookEvent{
        Ts: time.Now().Unix(), Event: "PermissionRequest",
        SessionID: "claude-session-abc", MtID: 42, Tool: "Bash",
    })

    hm.processDirectory()

    if !sess.HasHookData() {
        t.Fatal("session should have hook data after processing")
    }
    sess.(*terminal.Session) // type assert to access Activity directly via lock
    // Check via HookSessionID
    if sess.HookSessionID() != "claude-session-abc" {
        t.Errorf("HookSessionID = %q, want %q", sess.HookSessionID(), "claude-session-abc")
    }
}
```

Run: `go test ./internal/backend/... -run TestHookEventToActivity -v`
Expected: FAIL — `hookEventToActivity` undefined

### Step 2: Implement HookManager

Create `internal/backend/app_hooks.go`:

```go
package backend

import (
    "bufio"
    "context"
    "encoding/json"
    "log"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// rawHookEvent is the JSONL structure written by hook_handler.ps1.
type rawHookEvent struct {
    Ts        int64  `json:"ts"`
    Event     string `json:"event"`
    SessionID string `json:"session_id"` // Claude Code's own session UUID
    MtID      int    `json:"mt_id"`      // MULTITERMINAL_SESSION_ID (our session int)
    Tool      string `json:"tool"`
    Message   string `json:"message"`
}

// hookEventToActivity maps a Claude Code event name to an ActivityState.
func hookEventToActivity(event string) terminal.ActivityState {
    switch event {
    case "PreToolUse", "PostToolUse", "UserPromptSubmit":
        return terminal.ActivityActive
    case "PostToolUseFailure":
        return terminal.ActivityError
    case "PermissionRequest":
        return terminal.ActivityWaitingPermission
    case "Notification":
        return terminal.ActivityWaitingAnswer
    case "Stop":
        return terminal.ActivityDone
    default:
        return terminal.ActivityIdle
    }
}

// HookManager polls the hooks directory and dispatches events to sessions.
type HookManager struct {
    dir        string
    lookupFn   func(mtID int) *terminal.Session
    onActivity func(sessionID int, activity string, cost string)

    mu       sync.Mutex
    offsets  map[string]int64 // filename → bytes already read
    disabled bool
}

func newHookManager(
    dir string,
    lookupFn func(mtID int) *terminal.Session,
    onActivity func(sessionID int, activity string, cost string),
) *HookManager {
    return &HookManager{
        dir:        dir,
        lookupFn:   lookupFn,
        onActivity: onActivity,
        offsets:    make(map[string]int64),
    }
}

// Start begins the polling loop in a goroutine.
func (hm *HookManager) Start(ctx context.Context) {
    if err := os.MkdirAll(hm.dir, 0755); err != nil {
        log.Printf("[hooks] could not create hooks dir: %v — hook integration disabled", err)
        hm.mu.Lock()
        hm.disabled = true
        hm.mu.Unlock()
        return
    }

    go func() {
        ticker := time.NewTicker(100 * time.Millisecond)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                hm.processDirectory()
            }
        }
    }()
}

// processDirectory scans the hooks directory for new JSONL lines.
func (hm *HookManager) processDirectory() {
    entries, err := os.ReadDir(hm.dir)
    if err != nil {
        return
    }
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
            continue
        }
        path := filepath.Join(hm.dir, entry.Name())
        hm.processFile(path, entry.Name())
    }
}

// processFile reads new lines from a JSONL file since the last known offset.
func (hm *HookManager) processFile(path, name string) {
    hm.mu.Lock()
    offset := hm.offsets[name]
    hm.mu.Unlock()

    f, err := os.Open(path)
    if err != nil {
        return
    }
    defer f.Close()

    if offset > 0 {
        if _, err := f.Seek(offset, 0); err != nil {
            return
        }
    }

    scanner := bufio.NewScanner(f)
    var newOffset int64 = offset
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            newOffset += int64(len(scanner.Bytes())) + 1
            continue
        }
        var ev rawHookEvent
        if err := json.Unmarshal([]byte(line), &ev); err == nil {
            hm.handleEvent(ev)
        }
        newOffset += int64(len(scanner.Bytes())) + 1
    }

    hm.mu.Lock()
    hm.offsets[name] = newOffset
    hm.mu.Unlock()
}

// handleEvent applies a hook event to the appropriate session.
func (hm *HookManager) handleEvent(ev rawHookEvent) {
    if ev.MtID == 0 {
        return // no Multiterminal session ID — can't match
    }

    sess := hm.lookupFn(ev.MtID)
    if sess == nil {
        return
    }

    // Store Claude's session UUID on our session (for cleanup)
    if ev.SessionID != "" && sess.HookSessionID() == "" {
        sess.SetHookSessionID(ev.SessionID)
    }

    newState := hookEventToActivity(ev.Event)

    // SessionEnd: clean up, reset hook data
    if ev.Event == "SessionEnd" {
        sess.ClearHookData()
        hm.cleanupFile(ev.SessionID + ".jsonl")
        return
    }

    sess.SetHookActivity(newState)

    if hm.onActivity != nil {
        hm.onActivity(ev.MtID, activityString(newState), "")
    }
}

// cleanupFile removes a finished session's JSONL file.
func (hm *HookManager) cleanupFile(name string) {
    hm.mu.Lock()
    delete(hm.offsets, name)
    hm.mu.Unlock()
    _ = os.Remove(filepath.Join(hm.dir, name))
}
```

### Step 3: Run tests

Run: `go test ./internal/backend/... -run TestHookEvent -v`
Expected: PASS

Run: `go test ./internal/backend/... -v 2>&1 | tail -20`
Expected: all PASS

### Step 4: Commit

```bash
git add internal/backend/app_hooks.go internal/backend/app_hooks_test.go
git commit -m "feat(hooks): implement HookManager with polling JSONL reader"
```

---

## Task 8: Wire HookManager into AppService

**Files:**
- Modify: `internal/backend/app.go`

### Step 1: Add hookMgr field to AppService struct

In `internal/backend/app.go`, add to the `AppService` struct:

```go
hookMgr *HookManager
```

### Step 2: Deploy hook_handler.ps1 on startup

In `ServiceStartup()`, after `a.resolveClaudeOnStartup()`, add:

```go
// Deploy hook script and register hooks
a.setupHooks(ctx)
```

### Step 3: Add setupHooks method

Add to `internal/backend/app.go` (or create `internal/backend/app_hooks_setup.go` if app.go is near 300 lines):

```go
func (a *AppService) setupHooks(ctx context.Context) {
    appDataDir := os.Getenv("APPDATA")
    if appDataDir == "" {
        log.Println("[hooks] APPDATA not set — hook integration skipped")
        return
    }

    hooksDir := filepath.Join(appDataDir, "Multiterminal", "hooks")
    scriptPath := filepath.Join(appDataDir, "Multiterminal", "hook_handler.ps1")

    // Deploy/update the hook script from embedded bytes
    if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
        log.Printf("[hooks] could not create dir: %v", err)
        return
    }
    if err := os.WriteFile(scriptPath, []byte(hooks.HookHandlerScript), 0644); err != nil {
        log.Printf("[hooks] could not write hook script: %v", err)
        return
    }

    // Register hooks in ~/.claude/settings.json
    homeDir, _ := os.UserHomeDir()
    settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
    command := fmt.Sprintf(`powershell -NonInteractive -File "%s"`, scriptPath)
    installer := newHookInstaller(settingsPath, command)
    if err := installer.Install(); err != nil {
        log.Printf("[hooks] could not install hooks: %v", err)
        // Non-fatal: hooks just won't work
    } else {
        log.Println("[hooks] hooks registered in ~/.claude/settings.json")
    }

    // Start HookManager
    a.hookMgr = newHookManager(hooksDir, func(mtID int) *terminal.Session {
        a.mu.Lock()
        defer a.mu.Unlock()
        return a.sessions[mtID]
    }, func(sessionID int, activity string, cost string) {
        log.Printf("[hooks] session %d: %s", sessionID, activity)
        a.app.Event.Emit("terminal:activity", ActivityInfo{
            ID:       sessionID,
            Activity: activity,
            Cost:     cost,
        })
        // Trigger pipeline queue on "done"
        if activity == "done" {
            a.processQueue(sessionID)
        }
        // Issue progress
        a.onActivityChangeForIssue(sessionID, activity, cost)
    })
    a.hookMgr.Start(ctx)
}
```

Add required imports: `"path/filepath"`, `"github.com/patrick-goecommerce/Multiterminal-UI/internal/backend/hooks"`.

### Step 4: Build and verify

Run: `go build ./...`
Expected: no errors

### Step 5: Commit

```bash
git add internal/backend/app.go
git commit -m "feat(hooks): wire HookManager into AppService startup"
```

---

## Task 9: Frontend — update activity type in tabs.ts

**Files:**
- Modify: `frontend/src/stores/tabs.ts`
- Modify: `frontend/src/stores/tabs.test.ts`

### Step 1: Update the Pane type

In `frontend/src/stores/tabs.ts`, line 12, change:

```typescript
activity: 'idle' | 'active' | 'done' | 'needsInput';
```

to:

```typescript
activity: 'idle' | 'active' | 'done' | 'waitingPermission' | 'waitingAnswer' | 'error';
```

Also update `Tab.unreadActivity` (line 31):

```typescript
unreadActivity: 'needsInput' | 'active' | 'done' | null;
```

to:

```typescript
unreadActivity: 'waitingPermission' | 'waitingAnswer' | 'active' | 'done' | null;
```

### Step 2: Update computeTabActivity priority

```typescript
export function computeTabActivity(panes: Pane[]): Tab['unreadActivity'] {
  let result: Tab['unreadActivity'] = null;
  for (const pane of panes) {
    if (pane.activity === 'waitingPermission') return 'waitingPermission';
    if (pane.activity === 'waitingAnswer') return 'waitingAnswer';
    if (pane.activity === 'active') result = 'active';
    else if (pane.activity === 'done' && result === null) result = 'done';
  }
  return result;
}
```

### Step 3: Update tabs.test.ts

In `frontend/src/stores/tabs.test.ts`, rename all `'needsInput'` occurrences to `'waitingAnswer'`.
Also add tests for `'waitingPermission'`:

```typescript
it('waitingPermission has higher priority than waitingAnswer', () => {
  const panes = [
    { activity: 'waitingAnswer' } as any,
    { activity: 'waitingPermission' } as any,
  ];
  expect(computeTabActivity(panes)).toBe('waitingPermission');
});
```

### Step 4: Run frontend tests

Run: `cd frontend && npx vitest run src/stores/tabs.test.ts 2>&1 | tail -20`
Expected: all PASS

### Step 5: Commit

```bash
git add frontend/src/stores/tabs.ts frontend/src/stores/tabs.test.ts
git commit -m "feat(frontend): add waitingPermission/waitingAnswer/error to Pane activity type"
```

---

## Task 10: Frontend — update PaneTitlebar.svelte

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte`

### Step 1: Update getActivityDot()

```typescript
function getActivityDot(activity: string): string {
  switch (activity) {
    case 'active': return 'dot-active';
    case 'done': return 'dot-done';
    case 'waitingPermission': return 'dot-waiting-permission';
    case 'waitingAnswer': return 'dot-waiting-answer';
    case 'error': return 'dot-error';
    default: return 'dot-idle';
  }
}
```

### Step 2: Update titlebar CSS classes

Replace:
```svelte
class:titlebar-needs-input={pane.activity === 'needsInput'}
```
with:
```svelte
class:titlebar-waiting-permission={pane.activity === 'waitingPermission'}
class:titlebar-waiting-answer={pane.activity === 'waitingAnswer'}
class:titlebar-error={pane.activity === 'error'}
```

### Step 3: Add CSS for new states

In the `<style>` block, add:

```css
/* waitingPermission — yellow, lock icon feel */
:global(.dot-waiting-permission) {
  background: var(--color-warning, #f5a623);
  animation: pulse 1.2s ease-in-out infinite;
}
:global(.titlebar-waiting-permission) {
  border-bottom-color: var(--color-warning, #f5a623);
}

/* waitingAnswer — orange, question feel */
:global(.dot-waiting-answer) {
  background: var(--color-warning-soft, #e8875a);
  animation: pulse 1.2s ease-in-out infinite;
}
:global(.titlebar-waiting-answer) {
  border-bottom-color: var(--color-warning-soft, #e8875a);
}

/* error — red */
:global(.dot-error) {
  background: var(--color-error, #e05252);
}
:global(.titlebar-error) {
  border-bottom-color: var(--color-error, #e05252);
}
```

Remove old `.dot-needs-input` and `.titlebar-needs-input` CSS rules.

### Step 4: Build check

Run: `cd frontend && npx tsc --noEmit 2>&1 | head -20`
Expected: no errors

### Step 5: Commit

```bash
git add frontend/src/components/PaneTitlebar.svelte
git commit -m "feat(ui): update PaneTitlebar for waitingPermission/waitingAnswer/error states"
```

---

## Task 11: Frontend — update TerminalPane.svelte and IssuesView.svelte

**Files:**
- Modify: `frontend/src/components/TerminalPane.svelte`
- Modify: `frontend/src/components/IssuesView.svelte`
- Modify: `frontend/src/lib/audio.ts`

### Step 1: Update TerminalPane.svelte activity-needs-input references

Search for `needsInput` in `TerminalPane.svelte`. Replace:
- `pane.activity === 'needsInput'` → `pane.activity === 'waitingPermission' || pane.activity === 'waitingAnswer'`
- `class:activity-needs-input=` → `class:activity-waiting={pane.activity === 'waitingPermission' || pane.activity === 'waitingAnswer'}`

For the audio/notification trigger, update the condition:

```typescript
} else if ((pane.activity === 'waitingPermission' || pane.activity === 'waitingAnswer') && !needsInputAlerted) {
  needsInputAlerted = true;
  if (!document.hasFocus()) {
    // notification
  }
  if (shouldPlayAudio) playBell('needsInput', audio.volume, audio.input_sound || undefined);
}
```

### Step 2: Update IssuesView.svelte

In `IssuesView.svelte`, update the activity dot condition:

```svelte
class:needs-input={['waitingPermission','waitingAnswer'].includes(paneIssues[issue.number].activity)}
```

### Step 3: Update audio.ts

In `frontend/src/lib/audio.ts`, the `playBell` function signature already uses `'needsInput'` as a string literal. No change needed since we still call it with `'needsInput'` (it's a sound type, not an activity type).

### Step 4: Check App.svelte for needsInput references

Grep `frontend/src` for any remaining `needsInput` references and update them.

Run: `grep -r "needsInput" frontend/src --include="*.svelte" --include="*.ts" -n`
Expected: 0 results (or only audio.ts which is intentional)

### Step 5: Build check

Run: `cd frontend && npx tsc --noEmit 2>&1 | head -20`
Expected: no type errors

### Step 6: Commit

```bash
git add frontend/src/components/TerminalPane.svelte \
    frontend/src/components/IssuesView.svelte \
    frontend/src/lib/audio.ts
git commit -m "feat(ui): update TerminalPane and IssuesView for new activity states"
```

---

## Task 12: Final integration test

### Step 1: Run all Go tests

Run: `go test ./... 2>&1 | tail -30`
Expected: all PASS

### Step 2: Run frontend tests

Run: `cd frontend && npx vitest run 2>&1 | tail -20`
Expected: all PASS

### Step 3: Build check

Run: `go build ./... && cd frontend && npx tsc --noEmit`
Expected: no errors

### Step 4: Verify hook script deployed correctly

Start the app in dev mode:
```bash
wails dev
```

Check that:
1. `%APPDATA%\Multiterminal\hook_handler.ps1` was created
2. `~/.claude/settings.json` has Multiterminal hook entries
3. Starting a Claude pane and running a tool shows `waitingPermission` state in the titlebar when Claude asks for permission

### Step 5: Final commit

```bash
git add -A
git commit -m "feat: Claude Code hook integration with precise activity states

- Hook events drive activity state for Claude panes (no more PTY regex guessing)
- New states: waitingPermission, waitingAnswer, error
- PTY scanning retained for token/cost data
- Hooks auto-registered in ~/.claude/settings.json on startup
- MULTITERMINAL_SESSION_ID env var enables reliable session matching"
```

---

## Quick Reference

| Event | State | UI color |
|---|---|---|
| PreToolUse / PostToolUse | active | blue pulse |
| PostToolUseFailure | error | red |
| PermissionRequest | waitingPermission | yellow pulse |
| Notification | waitingAnswer | orange pulse |
| Stop | done | green glow |
| SessionEnd | cleanup | — |

**Test commands:**
```bash
go test ./internal/terminal/... -v
go test ./internal/backend/... -v
cd frontend && npx vitest run
go build ./...
```
