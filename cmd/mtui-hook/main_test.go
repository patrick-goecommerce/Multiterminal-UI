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
