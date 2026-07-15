package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesJSONLLine(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "7")
	os.Args = []string{"mtui-hook", "PreToolUse"}

	// stdin: a tool-use event
	r, w, _ := os.Pipe()
	w.Write([]byte(`{"session_id":"abc","tool_name":"Bash"}`))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	data, err := os.ReadFile(filepath.Join(appData, "Multiterminal", "hooks", "abc.jsonl"))
	if err != nil {
		t.Fatalf("jsonl not written: %v", err)
	}
	var line struct {
		Event     string `json:"event"`
		SessionID string `json:"session_id"`
		MtID      int    `json:"mt_id"`
		Tool      string `json:"tool"`
	}
	if err := json.Unmarshal(data[:len(data)-1], &line); err != nil {
		t.Fatalf("bad jsonl: %v (%q)", err, data)
	}
	if line.Event != "PreToolUse" || line.SessionID != "abc" || line.MtID != 7 || line.Tool != "Bash" {
		t.Fatalf("unexpected line: %+v", line)
	}
}

func TestRunCapturesCwdAndEnterWorktreeResponse(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "9")
	os.Args = []string{"mtui-hook", "PostToolUse"}

	stdin := `{"session_id":"xyz","cwd":"D:\\repos\\proj\\.claude\\worktrees\\feature-a","tool_name":"EnterWorktree","tool_input":{"name":"feature-a"},"tool_response":{"worktreePath":"D:\\repos\\proj\\.claude\\worktrees\\feature-a","worktreeBranch":"worktree-feature-a","message":"Created worktree..."}}`
	r, w, _ := os.Pipe()
	w.Write([]byte(stdin))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	data, err := os.ReadFile(filepath.Join(appData, "Multiterminal", "hooks", "xyz.jsonl"))
	if err != nil {
		t.Fatalf("jsonl not written: %v", err)
	}
	var line struct {
		Event          string `json:"event"`
		Tool           string `json:"tool"`
		Cwd            string `json:"cwd"`
		WorktreePath   string `json:"worktree_path"`
		WorktreeBranch string `json:"worktree_branch"`
	}
	if err := json.Unmarshal(data[:len(data)-1], &line); err != nil {
		t.Fatalf("bad jsonl: %v (%q)", err, data)
	}
	if line.Cwd != `D:\repos\proj\.claude\worktrees\feature-a` {
		t.Errorf("cwd = %q", line.Cwd)
	}
	if line.WorktreePath != `D:\repos\proj\.claude\worktrees\feature-a` {
		t.Errorf("worktree_path = %q", line.WorktreePath)
	}
	if line.WorktreeBranch != "worktree-feature-a" {
		t.Errorf("worktree_branch = %q", line.WorktreeBranch)
	}
}

func TestRunIgnoresToolResponseForOtherTools(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "9")
	os.Args = []string{"mtui-hook", "PostToolUse"}

	stdin := `{"session_id":"abc2","cwd":"D:\\repos\\proj","tool_name":"Bash","tool_response":{"worktreePath":"should-not-leak"}}`
	r, w, _ := os.Pipe()
	w.Write([]byte(stdin))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	data, err := os.ReadFile(filepath.Join(appData, "Multiterminal", "hooks", "abc2.jsonl"))
	if err != nil {
		t.Fatalf("jsonl not written: %v", err)
	}
	var line struct {
		WorktreePath string `json:"worktree_path"`
	}
	json.Unmarshal(data[:len(data)-1], &line)
	if line.WorktreePath != "" {
		t.Errorf("worktree_path leaked for non-EnterWorktree tool: %q", line.WorktreePath)
	}
}

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
