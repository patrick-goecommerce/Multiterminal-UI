# Worktree Path Firewall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `cmd/mtui-hook` (the Claude-Code hook binary) so that a `PreToolUse` `Edit`/`Write`/`NotebookEdit` call targeting a path inside the main repo checkout — while a different worktree is the expected working area for that session — is actively blocked, for every MTUI user, with no configuration.

**Architecture:** Two independent sources populate a per-session "expected worktree" context that `cmd/mtui-hook` (a short-lived subprocess with no access to the running app's memory) can resolve entirely on its own: (1) two new env vars set once at PTY launch for MTUI-created worktree panes, (2) a small JSON sidecar file `mtui-hook` writes itself when it detects Claude's native `PostToolUse:EnterWorktree`, and deletes on `PostToolUse:ExitWorktree`. On every `PreToolUse` for a write-type tool, the hook classifies the target path (inside the worktree → allow; inside the main repo but outside the worktree → deny; outside both → allow) and, only when denying, prints the officially documented block JSON to stdout. A block is also logged through the existing JSONL/HookManager pipeline so the running app can show a notification.

**Tech Stack:** Go 1.21+ (`cmd/mtui-hook`, `internal/backend`), Svelte 5 + TypeScript (`frontend/src`).

## Global Constraints

- Max 300 lines per Go file — split into logically grouped files if a task would push a file over that.
- `cmd/mtui-hook` must remain a lightweight, GUI-subsystem, console-free binary — do NOT import `internal/backend` into it (would pull in the Wails dependency tree). Duplicate the tiny `hideConsole`/git-subprocess-construction logic locally instead.
- Every non-PTY child process this hook spawns (the `git worktree list` call) MUST call `hideConsole(cmd)` before running, or it flashes a console window on Windows.
- Blocking protocol is exit code 0 + JSON on stdout (`hookSpecificOutput.permissionDecision: "deny"`) — NOT exit code 2/stderr. Verified against the current Claude Code hooks reference (see `docs/superpowers/specs/2026-07-09-worktree-path-firewall-design.md` §4.2, corrected 2026-07-10).
- `Edit`/`Write` tool_input path field is `file_path`; `NotebookEdit`'s is `notebook_path` — not the same field name.
- `Read` is never checked/blocked — only `Edit`, `Write`, `NotebookEdit`.
- All failures fail open (no context resolvable, unreadable/corrupt sidecar, git lookup error) → treat as "no restriction active", exactly like today's behavior for sessions with no worktree.
- UI text is German; code/comments are English (existing project convention).

---

### Task 1: `cmd/mtui-hook` — git root lookup + worktree sidecar lifecycle

**Files:**
- Create: `cmd/mtui-hook/hide_windows.go`
- Create: `cmd/mtui-hook/hide_other.go`
- Create: `cmd/mtui-hook/firewall.go`
- Test: `cmd/mtui-hook/firewall_test.go`

**Interfaces:**
- Produces: `gitMainRepoRoot(dir string) (string, error)`, `writeWorktreeSidecar(hooksDir, sessionID, worktreePath, mainRepoRoot string)`, `removeWorktreeSidecar(hooksDir, sessionID string)`, `resolveWorktreeContext(hooksDir, sessionID string) (worktreePath, mainRepoRoot string)`, `isUnderDir(path, dir string) bool`, `sidecarPath(hooksDir, sessionID string) string` — all consumed by Task 2.

- [ ] **Step 1: Write the failing tests**

Create `cmd/mtui-hook/firewall_test.go`:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initTestRepo creates a real git repo with one commit. Returned path is
// EvalSymlinks-resolved (t.TempDir may be a symlink on Windows/macOS).
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@test.local")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "init")
	return dir
}

func TestIsUnderDir(t *testing.T) {
	cases := []struct {
		path, dir string
		want      bool
	}{
		{`D:\repo\.claude\worktrees\a\file.go`, `D:\repo\.claude\worktrees\a`, true},
		{`D:\repo\.claude\worktrees\a`, `D:\repo\.claude\worktrees\a`, true},
		{`D:\REPO\.claude\WORKTREES\a\file.go`, `D:\repo\.claude\worktrees\a`, true},
		{`D:\repo\.claude\worktrees\b\file.go`, `D:\repo\.claude\worktrees\a`, false},
		{`D:\repo\file.go`, "", false},
		{"", `D:\repo`, false},
	}
	for _, c := range cases {
		if got := isUnderDir(c.path, c.dir); got != c.want {
			t.Errorf("isUnderDir(%q, %q) = %v, want %v", c.path, c.dir, got, c.want)
		}
	}
}

func TestGitMainRepoRoot_FromMainRepo(t *testing.T) {
	repo := initTestRepo(t)
	got, err := gitMainRepoRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	gotClean, _ := filepath.EvalSymlinks(got)
	if gotClean != repo {
		t.Errorf("gitMainRepoRoot = %q, want %q", gotClean, repo)
	}
}

func TestGitMainRepoRoot_FromLinkedWorktree(t *testing.T) {
	repo := initTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(filepath.Dir(wt), 0755)
	gitRun(t, repo, "worktree", "add", "-b", "feature-a", wt)

	got, err := gitMainRepoRoot(wt)
	if err != nil {
		t.Fatal(err)
	}
	gotClean, _ := filepath.EvalSymlinks(got)
	if gotClean != repo {
		t.Errorf("gitMainRepoRoot(worktree) = %q, want main repo %q", gotClean, repo)
	}
}

func TestGitMainRepoRoot_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := gitMainRepoRoot(dir); err == nil {
		t.Fatal("expected an error for a non-git directory")
	}
}

func TestSidecarWriteReadRemove(t *testing.T) {
	hooksDir := t.TempDir()
	writeWorktreeSidecar(hooksDir, "sess1", `D:\repo\.claude\worktrees\a`, `D:\repo`)

	wt, root := resolveWorktreeContext(hooksDir, "sess1")
	if wt != `D:\repo\.claude\worktrees\a` || root != `D:\repo` {
		t.Fatalf("got wt=%q root=%q", wt, root)
	}

	removeWorktreeSidecar(hooksDir, "sess1")
	wt, root = resolveWorktreeContext(hooksDir, "sess1")
	if wt != "" || root != "" {
		t.Fatalf("expected empty context after remove, got wt=%q root=%q", wt, root)
	}
}

func TestResolveWorktreeContext_NoSidecarNoEnv(t *testing.T) {
	hooksDir := t.TempDir()
	wt, root := resolveWorktreeContext(hooksDir, "unknown-session")
	if wt != "" || root != "" {
		t.Fatalf("expected empty context, got wt=%q root=%q", wt, root)
	}
}

func TestResolveWorktreeContext_CorruptSidecarFailsOpen(t *testing.T) {
	hooksDir := t.TempDir()
	if err := os.WriteFile(sidecarPath(hooksDir, "sess-corrupt"), []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}
	wt, root := resolveWorktreeContext(hooksDir, "sess-corrupt")
	if wt != "" || root != "" {
		t.Fatalf("expected fail-open (empty context) for corrupt sidecar JSON, got wt=%q root=%q", wt, root)
	}
}

func TestResolveWorktreeContext_EnvVarTakesPriorityOverSidecar(t *testing.T) {
	hooksDir := t.TempDir()
	writeWorktreeSidecar(hooksDir, "sess1", `D:\stale\worktree`, `D:\stale`)
	t.Setenv("MULTITERMINAL_WORKTREE_PATH", `D:\fresh\worktree`)
	t.Setenv("MULTITERMINAL_MAIN_REPO_ROOT", `D:\fresh`)

	wt, root := resolveWorktreeContext(hooksDir, "sess1")
	if wt != `D:\fresh\worktree` || root != `D:\fresh` {
		t.Fatalf("env var should win, got wt=%q root=%q", wt, root)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/mtui-hook/... -run 'TestIsUnderDir|TestGitMainRepoRoot|TestSidecar|TestResolveWorktreeContext' -v`
Expected: FAIL — `isUnderDir`, `gitMainRepoRoot`, `writeWorktreeSidecar`, `removeWorktreeSidecar`, `resolveWorktreeContext` undefined.

- [ ] **Step 3: Write the implementation**

Create `cmd/mtui-hook/hide_windows.go` (duplicated from `internal/backend/hide_windows.go` — not imported, see Global Constraints):

```go
//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideConsole sets CREATE_NO_WINDOW and HideWindow on the process so that
// neither the process itself nor any console window it allocates is visible.
// Duplicated from internal/backend/hide_windows.go: this binary must not
// import internal/backend (would pull in the Wails dependency tree into a
// binary that is meant to stay tiny and GUI-subsystem/console-free).
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
	cmd.SysProcAttr.HideWindow = true
}
```

Create `cmd/mtui-hook/hide_other.go`:

```go
//go:build !windows

package main

import "os/exec"

// hideConsole is a no-op on non-Windows platforms.
func hideConsole(_ *exec.Cmd) {}
```

Create `cmd/mtui-hook/firewall.go`:

```go
// PreToolUse path firewall: blocks Edit/Write/NotebookEdit calls that target
// a path inside the main repo checkout while a different worktree is the
// expected working area for this session. See
// docs/superpowers/specs/2026-07-09-worktree-path-firewall-design.md.
package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// worktreeSidecar is the per-session state this binary persists to disk so
// that a later, freshly-spawned PreToolUse invocation for the same session
// can recover the worktree context an earlier PostToolUse:EnterWorktree call
// discovered. mtui-hook has no long-lived process to hold this in memory.
type worktreeSidecar struct {
	WorktreePath string `json:"worktreePath"`
	MainRepoRoot string `json:"mainRepoRoot"`
}

func sidecarPath(hooksDir, sessionID string) string {
	return filepath.Join(hooksDir, sessionID+".worktree.json")
}

func writeWorktreeSidecar(hooksDir, sessionID, worktreePath, mainRepoRoot string) {
	data, err := json.Marshal(worktreeSidecar{WorktreePath: worktreePath, MainRepoRoot: mainRepoRoot})
	if err != nil {
		return
	}
	_ = os.WriteFile(sidecarPath(hooksDir, sessionID), data, 0644)
}

func removeWorktreeSidecar(hooksDir, sessionID string) {
	_ = os.Remove(sidecarPath(hooksDir, sessionID))
}

// resolveWorktreeContext returns the expected worktree path and main-repo
// root for a session, or two empty strings if no restriction is active. The
// env vars (set once at pane launch for MTUI-created worktree panes) take
// priority over the sidecar file (written by a mid-session EnterWorktree
// call) since they require no disk I/O. Any failure (missing/corrupt
// sidecar) is treated as "no context active" — fail open.
func resolveWorktreeContext(hooksDir, sessionID string) (worktreePath, mainRepoRoot string) {
	if wt := os.Getenv("MULTITERMINAL_WORKTREE_PATH"); wt != "" {
		return wt, os.Getenv("MULTITERMINAL_MAIN_REPO_ROOT")
	}
	data, err := os.ReadFile(sidecarPath(hooksDir, sessionID))
	if err != nil {
		return "", ""
	}
	var sc worktreeSidecar
	if json.Unmarshal(data, &sc) != nil {
		return "", ""
	}
	return sc.WorktreePath, sc.MainRepoRoot
}

// isUnderDir reports whether path is dir itself or nested inside it,
// comparing case-insensitively (Windows paths).
func isUnderDir(path, dir string) bool {
	if dir == "" || path == "" {
		return false
	}
	p := strings.ToLower(filepath.Clean(path))
	d := strings.ToLower(filepath.Clean(dir))
	return p == d || strings.HasPrefix(p, d+string(filepath.Separator))
}

// gitMainRepoRoot shells out to `git worktree list --porcelain` from dir and
// returns the first entry's path — git guarantees the main worktree is
// always listed first. Mirrors internal/backend.mainRepoRoot, duplicated
// here (not imported, see Global Constraints).
func gitMainRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "--no-optional-locks", "-c", "core.fsmonitor=false", "worktree", "list", "--porcelain")
	cmd.Dir = dir
	// Same suppression env as internal/backend.gitCmd (duplicated, not imported —
	// see Global Constraints): stops git from prompting or spawning helpers.
	cmd.Env = append(os.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line) // porcelain lines can carry a trailing \r
		if strings.HasPrefix(line, "worktree ") {
			return filepath.FromSlash(strings.TrimPrefix(line, "worktree ")), nil
		}
	}
	return "", errors.New("no worktree entries found")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/mtui-hook/... -run 'TestIsUnderDir|TestGitMainRepoRoot|TestSidecar|TestResolveWorktreeContext' -v`
Expected: PASS (7 tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/mtui-hook/hide_windows.go cmd/mtui-hook/hide_other.go cmd/mtui-hook/firewall.go cmd/mtui-hook/firewall_test.go
git commit -m "feat(mtui-hook): add git root lookup and worktree sidecar lifecycle"
```

---

### Task 2: `cmd/mtui-hook` — PreToolUse path classification + stdout deny JSON

**Files:**
- Modify: `cmd/mtui-hook/firewall.go`
- Modify: `cmd/mtui-hook/main.go`
- Test: `cmd/mtui-hook/firewall_test.go`
- Test: `cmd/mtui-hook/main_test.go`

**Interfaces:**
- Consumes: `resolveWorktreeContext`, `isUnderDir`, `writeWorktreeSidecar`, `removeWorktreeSidecar`, `gitMainRepoRoot` (Task 1).
- Produces: `checkPathFirewall(ev claudeEvent, hooksDir string) (blocked bool, path string, reason string)`; `hookLine.BlockedPath`/`BlockReason` JSON fields (`blocked_path`/`block_reason`) — consumed by Task 4's `rawHookEvent`, field names must match exactly.

- [ ] **Step 1: Write the failing tests**

Append to `cmd/mtui-hook/firewall_test.go`:

```go
func TestCheckPathFirewall(t *testing.T) {
	hooksDir := t.TempDir()
	writeWorktreeSidecar(hooksDir, "sess1", `D:\repo\.claude\worktrees\a`, `D:\repo`)

	// blocked: path in main repo, outside the worktree
	blocked, path, reason := checkPathFirewall(claudeEvent{
		SessionID: "sess1", ToolName: "Edit",
		ToolInput: []byte(`{"file_path":"D:\\repo\\internal\\backend\\app.go"}`),
	}, hooksDir)
	if !blocked || reason == "" || path != `D:\repo\internal\backend\app.go` {
		t.Fatalf("expected block, got blocked=%v path=%q reason=%q", blocked, path, reason)
	}

	// allowed: path inside the active worktree
	blocked, _, _ = checkPathFirewall(claudeEvent{
		SessionID: "sess1", ToolName: "Edit",
		ToolInput: []byte(`{"file_path":"D:\\repo\\.claude\\worktrees\\a\\internal\\backend\\app.go"}`),
	}, hooksDir)
	if blocked {
		t.Fatal("expected no block for path inside the active worktree")
	}

	// allowed: path outside both (e.g. scratchpad)
	blocked, _, _ = checkPathFirewall(claudeEvent{
		SessionID: "sess1", ToolName: "Edit",
		ToolInput: []byte(`{"file_path":"C:\\Temp\\scratch\\notes.md"}`),
	}, hooksDir)
	if blocked {
		t.Fatal("expected no block for a path outside both repo and worktree")
	}

	// allowed: Read is never checked
	blocked, _, _ = checkPathFirewall(claudeEvent{
		SessionID: "sess1", ToolName: "Read",
		ToolInput: []byte(`{"file_path":"D:\\repo\\internal\\backend\\app.go"}`),
	}, hooksDir)
	if blocked {
		t.Fatal("expected Read to never be blocked")
	}

	// NotebookEdit uses notebook_path, not file_path
	blocked, _, _ = checkPathFirewall(claudeEvent{
		SessionID: "sess1", ToolName: "NotebookEdit",
		ToolInput: []byte(`{"notebook_path":"D:\\repo\\.claude\\worktrees\\a\\nb.ipynb"}`),
	}, hooksDir)
	if blocked {
		t.Fatal("expected NotebookEdit inside worktree to be allowed")
	}
	blocked, _, _ = checkPathFirewall(claudeEvent{
		SessionID: "sess1", ToolName: "NotebookEdit",
		ToolInput: []byte(`{"notebook_path":"D:\\repo\\nb.ipynb"}`),
	}, hooksDir)
	if !blocked {
		t.Fatal("expected NotebookEdit in main repo to be blocked")
	}

	// allowed: no context active for this session
	blocked, _, _ = checkPathFirewall(claudeEvent{
		SessionID: "sess-none", ToolName: "Edit",
		ToolInput: []byte(`{"file_path":"D:\\repo\\internal\\backend\\app.go"}`),
	}, hooksDir)
	if blocked {
		t.Fatal("expected no block when no worktree context is active")
	}
}
```

In `cmd/mtui-hook/main_test.go`, change the import block from:

```go
import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)
```

to:

```go
import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)
```

Then append to the same file:

```go
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestRunWritesSidecarOnEnterWorktree(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "1")
	t.Setenv("MULTITERMINAL_WORKTREE_PATH", "")
	t.Setenv("MULTITERMINAL_MAIN_REPO_ROOT", "")
	os.Args = []string{"mtui-hook", "PostToolUse"}

	repo := initTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(filepath.Dir(wt), 0755)
	gitRun(t, repo, "worktree", "add", "-b", "feature-a", wt)

	stdin, _ := json.Marshal(map[string]any{
		"session_id": "sess-ew", "tool_name": "EnterWorktree",
		"tool_response": map[string]any{"worktreePath": wt, "worktreeBranch": "feature-a"},
	})
	r, w, _ := os.Pipe()
	w.Write(stdin)
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	hooksDir := filepath.Join(appData, "Multiterminal", "hooks")
	wtGot, rootGot := resolveWorktreeContext(hooksDir, "sess-ew")
	if wtGot != wt {
		t.Errorf("sidecar worktreePath = %q, want %q", wtGot, wt)
	}
	rootClean, _ := filepath.EvalSymlinks(rootGot)
	if rootClean != repo {
		t.Errorf("sidecar mainRepoRoot = %q, want %q", rootClean, repo)
	}
}

func TestRunRemovesSidecarOnExitWorktree(t *testing.T) {
	appData := t.TempDir()
	hooksDir := filepath.Join(appData, "Multiterminal", "hooks")
	os.MkdirAll(hooksDir, 0755)
	writeWorktreeSidecar(hooksDir, "sess-xw", `D:\repo\.claude\worktrees\a`, `D:\repo`)

	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "1")
	os.Args = []string{"mtui-hook", "PostToolUse"}

	stdin := `{"session_id":"sess-xw","tool_name":"ExitWorktree"}`
	r, w, _ := os.Pipe()
	w.Write([]byte(stdin))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	wtGot, _ := resolveWorktreeContext(hooksDir, "sess-xw")
	if wtGot != "" {
		t.Errorf("expected sidecar removed, still got worktreePath=%q", wtGot)
	}
}

func TestRunRemovesSidecarOnSessionEnd(t *testing.T) {
	appData := t.TempDir()
	hooksDir := filepath.Join(appData, "Multiterminal", "hooks")
	os.MkdirAll(hooksDir, 0755)
	writeWorktreeSidecar(hooksDir, "sess-crash", `D:\repo\.claude\worktrees\a`, `D:\repo`)

	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "1")
	os.Args = []string{"mtui-hook", "SessionEnd"}

	stdin := `{"session_id":"sess-crash"}`
	r, w, _ := os.Pipe()
	w.Write([]byte(stdin))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	wtGot, _ := resolveWorktreeContext(hooksDir, "sess-crash")
	if wtGot != "" {
		t.Errorf("expected sidecar removed on SessionEnd (crash/close backstop), still got worktreePath=%q", wtGot)
	}
}

func TestRunBlocksPreToolUseEditOutsideWorktree(t *testing.T) {
	appData := t.TempDir()
	hooksDir := filepath.Join(appData, "Multiterminal", "hooks")
	os.MkdirAll(hooksDir, 0755)
	writeWorktreeSidecar(hooksDir, "sess-block", `D:\repo\.claude\worktrees\a`, `D:\repo`)

	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "1")
	t.Setenv("MULTITERMINAL_WORKTREE_PATH", "")
	t.Setenv("MULTITERMINAL_MAIN_REPO_ROOT", "")
	os.Args = []string{"mtui-hook", "PreToolUse"}

	stdin, _ := json.Marshal(map[string]any{
		"session_id": "sess-block", "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": `D:\repo\internal\backend\app.go`},
	})
	r, w, _ := os.Pipe()
	w.Write(stdin)
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	out := captureStdout(t, run)

	var parsed struct {
		HookSpecificOutput struct {
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("stdout not valid JSON: %v (%q)", err, out)
	}
	if parsed.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want deny", parsed.HookSpecificOutput.PermissionDecision)
	}
	if parsed.HookSpecificOutput.PermissionDecisionReason == "" {
		t.Error("expected a non-empty permissionDecisionReason")
	}

	data, err := os.ReadFile(filepath.Join(hooksDir, "sess-block.jsonl"))
	if err != nil {
		t.Fatalf("jsonl not written: %v", err)
	}
	var line struct {
		BlockedPath string `json:"blocked_path"`
		BlockReason string `json:"block_reason"`
	}
	if err := json.Unmarshal(data[:len(data)-1], &line); err != nil {
		t.Fatalf("bad jsonl: %v (%q)", err, data)
	}
	if line.BlockedPath != `D:\repo\internal\backend\app.go` || line.BlockReason == "" {
		t.Errorf("unexpected jsonl block fields: %+v", line)
	}
}

func TestRunAllowsPreToolUseEditInsideWorktree(t *testing.T) {
	appData := t.TempDir()
	hooksDir := filepath.Join(appData, "Multiterminal", "hooks")
	os.MkdirAll(hooksDir, 0755)
	writeWorktreeSidecar(hooksDir, "sess-ok", `D:\repo\.claude\worktrees\a`, `D:\repo`)

	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "1")
	t.Setenv("MULTITERMINAL_WORKTREE_PATH", "")
	t.Setenv("MULTITERMINAL_MAIN_REPO_ROOT", "")
	os.Args = []string{"mtui-hook", "PreToolUse"}

	stdin, _ := json.Marshal(map[string]any{
		"session_id": "sess-ok", "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": `D:\repo\.claude\worktrees\a\file.go`},
	})
	r, w, _ := os.Pipe()
	w.Write(stdin)
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	out := captureStdout(t, run)
	if out != "" {
		t.Errorf("expected no stdout output for an allowed edit, got %q", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/mtui-hook/... -run 'TestCheckPathFirewall|TestRunWritesSidecarOnEnterWorktree|TestRunRemovesSidecarOnExitWorktree|TestRunRemovesSidecarOnSessionEnd|TestRunBlocksPreToolUseEditOutsideWorktree|TestRunAllowsPreToolUseEditInsideWorktree' -v`
Expected: FAIL — `checkPathFirewall` undefined; existing `run()` doesn't write sidecars/blocking output yet.

- [ ] **Step 3: Write the implementation**

Append to `cmd/mtui-hook/firewall.go`:

```go
// toolInputPath holds the path fields present in Edit/Write/NotebookEdit
// tool_input payloads. Edit and Write use file_path; NotebookEdit uses
// notebook_path — not the same field name.
type toolInputPath struct {
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
}

var writeTools = map[string]bool{"Edit": true, "Write": true, "NotebookEdit": true}

// checkPathFirewall inspects a PreToolUse event for Edit/Write/NotebookEdit
// and reports whether it should be blocked, plus the path it classified.
// hooksDir/ev.SessionID resolve the sidecar file if no env var is set.
func checkPathFirewall(ev claudeEvent, hooksDir string) (blocked bool, path string, reason string) {
	if !writeTools[ev.ToolName] || len(ev.ToolInput) == 0 {
		return false, "", ""
	}
	var input toolInputPath
	if json.Unmarshal(ev.ToolInput, &input) != nil {
		return false, "", ""
	}
	path = input.FilePath
	if path == "" {
		path = input.NotebookPath
	}
	if path == "" {
		return false, "", ""
	}

	worktreePath, mainRoot := resolveWorktreeContext(hooksDir, ev.SessionID)
	if worktreePath == "" || mainRoot == "" {
		return false, path, ""
	}
	if isUnderDir(path, worktreePath) {
		return false, path, ""
	}
	if isUnderDir(path, mainRoot) {
		return true, path, "Pfad liegt im Hauptrepo (" + mainRoot + "), nicht im aktiven Worktree (" + worktreePath + "). Bitte den Pfad korrigieren."
	}
	return false, path, ""
}
```

Replace the full contents of `cmd/mtui-hook/main.go` with:

```go
// Command mtui-hook is the Multiterminal Claude Code hook handler. It reads a
// hook event JSON on stdin and appends a JSONL line to
// %APPDATA%\Multiterminal\hooks\<session_id>.jsonl — the same format the previous
// PowerShell handler wrote (see internal/backend/app_hooks.go rawHookEvent).
//
// It replaces hook_handler.ps1 specifically to stop a console window flashing on
// every hook event: PowerShell is a console-subsystem program, so Claude spawning
// it allocates a visible conhost window per event (PreToolUse/PostToolUse fire
// many times per turn). This binary is built with `-ldflags -H windowsgui`, so it
// is GUI-subsystem and no console is ever allocated.
//
// The event type is passed as the first CLI argument. All failures are silent —
// a hook must never crash Claude Code. The one exception is the PreToolUse path
// firewall (see firewall.go): it may print a structured deny decision to stdout
// (exit code 0 + JSON, the documented Claude Code hook protocol) — this is not a
// crash/break, just the officially supported way to answer a permission check.
package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type claudeEvent struct {
	SessionID    string          `json:"session_id"`
	ToolName     string          `json:"tool_name"`
	Message      string          `json:"message"`
	Prompt       string          `json:"prompt"`
	Cwd          string          `json:"cwd"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

type enterWorktreeResponse struct {
	WorktreePath   string `json:"worktreePath"`
	WorktreeBranch string `json:"worktreeBranch"`
}

// hookLine mirrors internal/backend.rawHookEvent — keep the json tags in sync.
type hookLine struct {
	Ts             int64  `json:"ts"`
	Event          string `json:"event"`
	SessionID      string `json:"session_id"`
	MtID           int    `json:"mt_id"`
	Tool           string `json:"tool"`
	Message        string `json:"message"`
	Cwd            string `json:"cwd,omitempty"`
	WorktreePath   string `json:"worktree_path,omitempty"`
	WorktreeBranch string `json:"worktree_branch,omitempty"`
	BlockedPath    string `json:"blocked_path,omitempty"`
	BlockReason    string `json:"block_reason,omitempty"`
}

// hookSpecificOutput / preToolUseOutput implement Claude Code's documented
// PreToolUse block protocol: exit code 0 + this JSON on stdout.
type hookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

type preToolUseOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

func main() {
	defer func() { _ = recover() }() // never surface a panic to Claude
	run()
}

func run() {
	eventType := ""
	if len(os.Args) > 1 {
		eventType = os.Args[1]
	}

	data, _ := io.ReadAll(os.Stdin)
	var ev claudeEvent
	_ = json.Unmarshal(data, &ev)
	ev.SessionID = firstNonEmpty(ev.SessionID, "unknown")

	sessionID := ev.SessionID
	mtID := 0
	if v, err := strconv.Atoi(os.Getenv("MULTITERMINAL_SESSION_ID")); err == nil {
		mtID = v
	}
	message := ev.Message
	// UserPromptSubmit carries the user's prompt text in `prompt`, not `message`.
	if eventType == "UserPromptSubmit" && ev.Prompt != "" {
		message = ev.Prompt
	}

	hooksDir := filepath.Join(os.Getenv("APPDATA"), "Multiterminal", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return
	}

	var worktreePath, worktreeBranch string
	if eventType == "PostToolUse" && ev.ToolName == "EnterWorktree" && len(ev.ToolResponse) > 0 {
		var wt enterWorktreeResponse
		if json.Unmarshal(ev.ToolResponse, &wt) == nil {
			worktreePath = wt.WorktreePath
			worktreeBranch = wt.WorktreeBranch
			if worktreePath != "" {
				if root, err := gitMainRepoRoot(worktreePath); err == nil {
					writeWorktreeSidecar(hooksDir, sessionID, worktreePath, root)
				}
			}
		}
	}
	if eventType == "PostToolUse" && ev.ToolName == "ExitWorktree" {
		removeWorktreeSidecar(hooksDir, sessionID)
	}
	// A session that crashes/closes after EnterWorktree without ever calling
	// ExitWorktree would otherwise leave the sidecar orphaned forever. SessionEnd
	// already fires for every session (app_hooks.go cleans up the .jsonl file on
	// it) — remove the sidecar there too as a symmetric backstop.
	if eventType == "SessionEnd" {
		removeWorktreeSidecar(hooksDir, sessionID)
	}

	var blockedPath, blockReason string
	if eventType == "PreToolUse" {
		if blocked, path, reason := checkPathFirewall(ev, hooksDir); blocked {
			blockedPath = path
			blockReason = reason
		}
	}

	line, err := json.Marshal(hookLine{
		Ts:             time.Now().Unix(),
		Event:          eventType,
		SessionID:      sessionID,
		MtID:           mtID,
		Tool:           ev.ToolName,
		Message:        message,
		Cwd:            ev.Cwd,
		WorktreePath:   worktreePath,
		WorktreeBranch: worktreeBranch,
		BlockedPath:    blockedPath,
		BlockReason:    blockReason,
	})
	if err == nil {
		if f, ferr := os.OpenFile(
			filepath.Join(hooksDir, sessionID+".jsonl"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); ferr == nil {
			_, _ = f.Write(append(line, '\n'))
			f.Close()
		}
	}

	if blockReason != "" {
		out, err := json.Marshal(preToolUseOutput{HookSpecificOutput: hookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: blockReason,
		}})
		if err == nil {
			os.Stdout.Write(out)
		}
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/mtui-hook/... -v`
Expected: PASS (all tests in the package, including the 3 pre-existing ones from `main_test.go`)

- [ ] **Step 5: Commit**

```bash
git add cmd/mtui-hook/firewall.go cmd/mtui-hook/firewall_test.go cmd/mtui-hook/main.go cmd/mtui-hook/main_test.go
git commit -m "feat(mtui-hook): block PreToolUse writes outside the active worktree"
```

---

### Task 3: `internal/backend` — worktree env vars at pane launch

**Files:**
- Modify: `internal/backend/app_worktree_pane.go`
- Modify: `internal/backend/app.go:212-219` (CreateSession env injection)
- Test: `internal/backend/app_worktree_pane_test.go`

**Interfaces:**
- Consumes: `mainRepoRoot(dir string) (string, error)` (existing, same file).
- Produces: `worktreeEnvVars(dir string) []string` — returns `["MULTITERMINAL_WORKTREE_PATH=...", "MULTITERMINAL_MAIN_REPO_ROOT=..."]` or `nil`. Consumed by `CreateSession` in this task; the two env var NAMES are also consumed by Task 1/2's `resolveWorktreeContext` (already implemented, reads via `os.Getenv` — no code coupling, just a naming contract that must match exactly).

- [ ] **Step 1: Write the failing tests**

Add to `internal/backend/app_worktree_pane_test.go`:

```go
func TestWorktreeEnvVars_MainRepoReturnsNil(t *testing.T) {
	repo := initPaneTestRepo(t)
	if got := worktreeEnvVars(repo); got != nil {
		t.Errorf("expected nil for main repo dir, got %v", got)
	}
}

func TestWorktreeEnvVars_LinkedWorktreeReturnsEnvPairs(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	os.MkdirAll(filepath.Dir(wt), 0755)
	gitRun(t, repo, "worktree", "add", "-b", "feature-a", wt)

	got := worktreeEnvVars(wt)
	if len(got) != 2 {
		t.Fatalf("expected 2 env vars, got %v", got)
	}
	if got[0] != "MULTITERMINAL_WORKTREE_PATH="+wt {
		t.Errorf("got %q", got[0])
	}
	// mainRepoRoot's own tests (TestMainRepoRoot_FromMainRepo) compare with
	// strings.EqualFold, not exact equality, to tolerate git's own path
	// casing/symlink-resolution quirks on Windows — match that convention here.
	const rootPrefix = "MULTITERMINAL_MAIN_REPO_ROOT="
	if !strings.HasPrefix(got[1], rootPrefix) || !strings.EqualFold(strings.TrimPrefix(got[1], rootPrefix), repo) {
		t.Errorf("got %q, want root %q (case/symlink-insensitive)", got[1], repo)
	}
}

func TestWorktreeEnvVars_NonGitDirReturnsNil(t *testing.T) {
	dir := t.TempDir()
	if got := worktreeEnvVars(dir); got != nil {
		t.Errorf("expected nil for non-git dir, got %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/backend/... -run TestWorktreeEnvVars -v`
Expected: FAIL — `worktreeEnvVars` undefined.

- [ ] **Step 3: Write the implementation**

In `internal/backend/app_worktree_pane.go`, add after the `mainRepoRoot` function (imports `fmt`, `log`, `os`, `path/filepath`, `strings` already present — no new imports needed):

```go
// worktreeEnvVars returns the MULTITERMINAL_WORKTREE_PATH/MULTITERMINAL_MAIN_REPO_ROOT
// env var pairs for a Claude pane whose dir is a linked worktree (not the main
// checkout). Returns nil for the main checkout itself, non-git directories, or
// any other lookup failure — CreateSession then simply launches without the
// restriction, exactly like before this feature existed (spec 2026-07-09).
//
// Accepted cost: this runs a synchronous `git worktree list` subprocess (via
// mainRepoRoot) on every Claude-mode CreateSession call — including the
// orchestrator/schedule-runner call sites, not only interactive pane launches
// — adding roughly one git-subprocess-spawn's worth of latency (already the
// same order of magnitude as other one-time git calls CreateSession's callers
// make elsewhere). Not measured; revisit if session launch latency ever
// becomes a complaint.
func worktreeEnvVars(dir string) []string {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return nil
	}
	if strings.EqualFold(filepath.Clean(root), filepath.Clean(dir)) {
		return nil
	}
	return []string{
		"MULTITERMINAL_WORKTREE_PATH=" + dir,
		"MULTITERMINAL_MAIN_REPO_ROOT=" + root,
	}
}
```

In `internal/backend/app.go`, replace:

```go
	if mode == "claude" || mode == "claude-auto" || mode == "claude-yolo" {
		env = append(env, fmt.Sprintf("MULTITERMINAL_SESSION_ID=%d", id))
	}
```

with:

```go
	if mode == "claude" || mode == "claude-auto" || mode == "claude-yolo" {
		env = append(env, fmt.Sprintf("MULTITERMINAL_SESSION_ID=%d", id))
		env = append(env, worktreeEnvVars(dir)...)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/backend/... -run TestWorktreeEnvVars -v`
Expected: PASS (3 tests)

Then run the full backend suite to confirm nothing else broke: `go test ./internal/backend/... -v`
Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_pane.go internal/backend/app_worktree_pane_test.go internal/backend/app.go
git commit -m "feat(backend): set worktree env vars at pane launch for the path firewall"
```

---

### Task 4: `internal/backend` — HookManager `onPathBlocked` + UI event

**Files:**
- Modify: `internal/backend/app_hooks.go`
- Modify: `internal/backend/app_hooks_setup.go:80`
- Modify: `internal/backend/app_events.go`
- Modify: `internal/backend/app_worktree_detect.go`
- Test: `internal/backend/app_hooks_test.go`
- Test: `internal/backend/app_worktree_detect_test.go`

**Interfaces:**
- Consumes: `rawHookEvent.BlockedPath`/`BlockReason` JSON fields `blocked_path`/`block_reason` (Task 2, must match exactly), `emitWorktreeEventSafe(name string, payload any)` (existing).
- Produces: `HookManager.onPathBlocked func(mtID int, path, reason string)`, `WorktreePathBlockedEvent{ID, Path, Reason}`, `(a *AppService) onWorktreePathBlocked(mtID int, path, reason string)` — consumed by Task 5's frontend listener via the emitted event name `"worktree:path-blocked"` and its JSON shape `{id, path, reason}`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/backend/app_hooks_test.go`:

```go
func TestHandleEvent_CallsOnPathBlocked(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session { return sess }, nil)

	var gotPath, gotReason string
	var calls int
	hm.onPathBlocked = func(mtID int, path, reason string) {
		calls++
		gotPath, gotReason = path, reason
	}

	hm.handleEvent(rawHookEvent{
		Event: "PreToolUse", MtID: 1, SessionID: "s1", Tool: "Edit",
		BlockedPath: `D:\repo\internal\backend\app.go`,
		BlockReason: "Pfad liegt im Hauptrepo...",
	})

	if calls != 1 {
		t.Fatalf("onPathBlocked called %d times, want 1", calls)
	}
	if gotPath != `D:\repo\internal\backend\app.go` || gotReason != "Pfad liegt im Hauptrepo..." {
		t.Errorf("got path=%q reason=%q", gotPath, gotReason)
	}
}

func TestHandleEvent_DoesNotCallOnPathBlockedWhenEmpty(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	hm := newHookManager("", func(mtID int) *terminal.Session { return sess }, nil)

	calls := 0
	hm.onPathBlocked = func(int, string, string) { calls++ }

	hm.handleEvent(rawHookEvent{Event: "PreToolUse", MtID: 1, SessionID: "s1", Tool: "Edit"})

	if calls != 0 {
		t.Errorf("onPathBlocked should not fire without a BlockedPath, called %d times", calls)
	}
}
```

Add to `internal/backend/app_worktree_detect_test.go`:

```go
func TestOnWorktreePathBlocked_EmitsEvent(t *testing.T) {
	a := newDetectTestApp()
	var emitted *WorktreePathBlockedEvent
	a.emitWorktreeEvent = func(name string, payload any) {
		if ev, ok := payload.(WorktreePathBlockedEvent); ok {
			emitted = &ev
		}
	}

	a.onWorktreePathBlocked(1, `D:\repo\internal\backend\app.go`, "Pfad liegt im Hauptrepo...")

	if emitted == nil {
		t.Fatal("expected WorktreePathBlockedEvent to be emitted")
	}
	if emitted.ID != 1 || emitted.Path != `D:\repo\internal\backend\app.go` || emitted.Reason != "Pfad liegt im Hauptrepo..." {
		t.Errorf("unexpected event: %+v", emitted)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/backend/... -run 'TestHandleEvent_CallsOnPathBlocked|TestHandleEvent_DoesNotCallOnPathBlockedWhenEmpty|TestOnWorktreePathBlocked_EmitsEvent' -v`
Expected: FAIL — `onPathBlocked` field / `WorktreePathBlockedEvent` / `onWorktreePathBlocked` undefined.

- [ ] **Step 3: Write the implementation**

In `internal/backend/app_hooks.go`, extend `rawHookEvent`:

```go
type rawHookEvent struct {
	Ts             int64  `json:"ts"`
	Event          string `json:"event"`
	SessionID      string `json:"session_id"`
	MtID           int    `json:"mt_id"`
	Tool           string `json:"tool"`
	Message        string `json:"message"`
	Cwd            string `json:"cwd"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
	BlockedPath    string `json:"blocked_path"`
	BlockReason    string `json:"block_reason"`
}
```

Extend `HookManager` struct (after the `onWorktreeChange` field):

```go
	// onPathBlocked, if set, is called when mtui-hook's PreToolUse path
	// firewall blocked a write attempt outside the active worktree
	// (spec 2026-07-09-worktree-path-firewall-design.md).
	onPathBlocked func(mtID int, path, reason string)
```

In `handleEvent`, after the existing `onWorktreeChange` call:

```go
	if ev.BlockedPath != "" && hm.onPathBlocked != nil {
		hm.onPathBlocked(ev.MtID, ev.BlockedPath, ev.BlockReason)
	}
```

In `internal/backend/app_events.go`, add:

```go
// WorktreePathBlockedEvent is emitted when mtui-hook's PreToolUse path
// firewall blocked a write attempt targeting the main repo while a different
// worktree was active for the session (spec 2026-07-09).
type WorktreePathBlockedEvent struct {
	ID     int    `json:"id"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
```

In `internal/backend/app_worktree_detect.go`, add at the end of the file:

```go
// onWorktreePathBlocked is the HookManager callback for a blocked write
// attempt (spec 2026-07-09). Purely informational — no state change, MTUI
// does not intervene beyond surfacing it to the user.
func (a *AppService) onWorktreePathBlocked(mtID int, path, reason string) {
	log.Printf("[worktree-detect] session %d blocked write to %s: %s", mtID, path, reason)
	a.emitWorktreeEventSafe("worktree:path-blocked", WorktreePathBlockedEvent{ID: mtID, Path: path, Reason: reason})
}
```

In `internal/backend/app_hooks_setup.go`, after `a.hookMgr.onWorktreeChange = a.onWorktreeChange`, add:

```go
	a.hookMgr.onPathBlocked = a.onWorktreePathBlocked
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/backend/... -v`
Expected: PASS (all tests, including the 3 new ones)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_hooks.go internal/backend/app_hooks_setup.go internal/backend/app_events.go internal/backend/app_worktree_detect.go internal/backend/app_hooks_test.go internal/backend/app_worktree_detect_test.go
git commit -m "feat(backend): surface blocked path-firewall writes as a worktree event"
```

---

### Task 5: Frontend — notification on a blocked write

**Files:**
- Modify: `frontend/src/App.svelte`
- Modify: `frontend/src/i18n/de.ts`
- Modify: `frontend/src/i18n/en.ts`
- Modify: `frontend/src/i18n/es.ts`
- Modify: `frontend/src/i18n/fr.ts`
- Modify: `frontend/src/i18n/it.ts`

**Interfaces:**
- Consumes: Wails event `"worktree:path-blocked"` with payload `{id: number, path: string, reason: string}` (Task 4), existing `ownsSession(id)` helper and `sendNotification(title, body)` (`lib/notifications.ts`), existing `$t('key', params)` i18n helper.

- [ ] **Step 1: Add the i18n keys**

In `frontend/src/i18n/de.ts`, in the `app:` block, after `notifyInputBody: 'Claude wartet auf Bestätigung.',`:

```ts
    worktreePathBlocked: 'Schreibversuch blockiert',
    worktreePathBlockedBody: 'Claude wollte außerhalb des aktiven Worktrees schreiben ({path}) — blockiert.',
```

In `frontend/src/i18n/en.ts`, in the `app:` block, after `notifyInputBody: 'Claude is waiting for confirmation.',`:

```ts
    worktreePathBlocked: 'Write attempt blocked',
    worktreePathBlockedBody: 'Claude tried to write outside the active worktree ({path}) — blocked.',
```

In `frontend/src/i18n/es.ts`, in the `app:` block, after `notifyInputBody: 'Claude espera confirmación.',`:

```ts
    worktreePathBlocked: 'Intento de escritura bloqueado',
    worktreePathBlockedBody: 'Claude intentó escribir fuera del worktree activo ({path}) — bloqueado.',
```

In `frontend/src/i18n/fr.ts`, in the `app:` block, after `notifyInputBody: 'Claude attend une confirmation.',`:

```ts
    worktreePathBlocked: 'Tentative d\'écriture bloquée',
    worktreePathBlockedBody: 'Claude a tenté d\'écrire en dehors du worktree actif ({path}) — bloqué.',
```

In `frontend/src/i18n/it.ts`, in the `app:` block, after `notifyInputBody: 'Claude attende una conferma.',`:

```ts
    worktreePathBlocked: 'Tentativo di scrittura bloccato',
    worktreePathBlockedBody: 'Claude ha provato a scrivere fuori dal worktree attivo ({path}) — bloccato.',
```

- [ ] **Step 2: Wire the event listener**

In `frontend/src/App.svelte`, immediately after the existing block:

```ts
    EventsOn('worktree:cleared', (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.id)) return;
      tabStore.clearWorktree(p.id);
    });
```

add:

```ts
    EventsOn('worktree:path-blocked', (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.id)) return;
      sendNotification($t('app.worktreePathBlocked'), $t('app.worktreePathBlockedBody', { path: p.path }));
    });
```

- [ ] **Step 3: Verify the i18n dictionary loads correctly**

Run: `cd frontend && npx vitest run` (existing i18n consumption is exercised by every component test via `src/test-setup.ts`'s `initI18n('de')` — a malformed i18n file fails the whole suite at setup)
Expected: PASS (all existing tests, including component tests that already depend on `$t()` resolving)

- [ ] **Step 4: Manual/E2E verification (documented, not automated — App.svelte has no unit test harness in this codebase today)**

With a real `claude` process in an MTUI pane: call `EnterWorktree`, then attempt an `Edit` on a path inside the main repo → a desktop notification "Schreibversuch blockiert" appears and Claude receives the deny reason as tool feedback. Attempt an `Edit` inside the worktree → succeeds normally, no notification. Call `ExitWorktree` → attempt an `Edit` in the main repo again → succeeds normally (restriction lifted). Tag this task `needs-e2e-testing` per `CLAUDE.md` issue discipline until this walkthrough has been run with a real Claude CLI process.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.svelte frontend/src/i18n/de.ts frontend/src/i18n/en.ts frontend/src/i18n/es.ts frontend/src/i18n/fr.ts frontend/src/i18n/it.ts
git commit -m "feat(frontend): notify when the path firewall blocks a write"
```

---

### Task 6: Full verification pass

**Files:** none (verification only)

**Interfaces:** none — this task only runs the full build/test suite across everything Tasks 1-5 touched.

- [ ] **Step 1: Backend build and vet**

Run: `go build ./...`
Expected: builds cleanly, including `cmd/mtui-hook` as a standalone binary.

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 2: Backend full test suite**

Run: `go test ./... -v`
Expected: PASS — every package, including `cmd/mtui-hook` and `internal/backend`.

- [ ] **Step 3: Frontend full test suite and build**

Run: `cd frontend && npx vitest run`
Expected: PASS — all test files.

Run: `cd frontend && npm run build`
Expected: builds cleanly (pre-existing a11y/unused-export warnings elsewhere in the codebase are expected and unrelated — only fail on errors, not warnings).

- [ ] **Step 4: Final commit (if any verification step required a fix)**

```bash
git add -A
git commit -m "chore: fix verification issues found in the path-firewall feature"
```

(Skip this step entirely if Steps 1-3 passed without any changes.)
