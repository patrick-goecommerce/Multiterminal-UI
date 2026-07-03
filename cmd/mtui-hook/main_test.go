package main

import (
	"encoding/json"
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
