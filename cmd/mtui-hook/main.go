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
