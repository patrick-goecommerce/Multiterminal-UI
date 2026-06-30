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
// a hook must never block or break Claude Code.
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
	SessionID string `json:"session_id"`
	ToolName  string `json:"tool_name"`
	Message   string `json:"message"`
	Prompt    string `json:"prompt"`
}

// hookLine mirrors internal/backend.rawHookEvent — keep the json tags in sync.
type hookLine struct {
	Ts        int64  `json:"ts"`
	Event     string `json:"event"`
	SessionID string `json:"session_id"`
	MtID      int    `json:"mt_id"`
	Tool      string `json:"tool"`
	Message   string `json:"message"`
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

	sessionID := ev.SessionID
	if sessionID == "" {
		sessionID = "unknown"
	}
	mtID := 0
	if v, err := strconv.Atoi(os.Getenv("MULTITERMINAL_SESSION_ID")); err == nil {
		mtID = v
	}
	message := ev.Message
	// UserPromptSubmit carries the user's prompt text in `prompt`, not `message`.
	if eventType == "UserPromptSubmit" && ev.Prompt != "" {
		message = ev.Prompt
	}

	line, err := json.Marshal(hookLine{
		Ts:        time.Now().Unix(),
		Event:     eventType,
		SessionID: sessionID,
		MtID:      mtID,
		Tool:      ev.ToolName,
		Message:   message,
	})
	if err != nil {
		return
	}

	hooksDir := filepath.Join(os.Getenv("APPDATA"), "Multiterminal", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(
		filepath.Join(hooksDir, sessionID+".jsonl"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}
