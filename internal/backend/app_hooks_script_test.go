package backend

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/backend/hooks"
)

// runHookScript deploys the embedded hook handler to a temp dir, runs it via
// powershell with the given event type and stdin JSON, and returns the parsed
// JSONL line written for the session. Windows-only (the script is PowerShell).
func runHookScript(t *testing.T, eventType, stdinJSON, sessionID string) rawHookEvent {
	t.Helper()

	appData := t.TempDir()
	scriptPath := filepath.Join(appData, "hook_handler.ps1")
	if err := os.WriteFile(scriptPath, []byte(hooks.HookHandlerScript), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("powershell", "-NonInteractive", "-File", scriptPath, eventType)
	cmd.Env = append(os.Environ(),
		"APPDATA="+appData,
		"MULTITERMINAL_SESSION_ID=5",
	)
	cmd.Stdin = strings.NewReader(stdinJSON)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook script failed: %v\n%s", err, out)
	}

	jsonlPath := filepath.Join(appData, "Multiterminal", "hooks", sessionID+".jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	var last rawHookEvent
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev rawHookEvent
		if err := json.Unmarshal([]byte(line), &ev); err == nil {
			last = ev
			found = true
		}
	}
	if !found {
		t.Fatalf("no events parsed from %q", string(data))
	}
	return last
}

func TestHookScript_UserPromptSubmitCapturesPrompt(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("hook handler is a PowerShell script (Windows-only)")
	}

	stdin := `{"session_id":"sess-prompt","prompt":"Fix the auth bug","message":""}`
	ev := runHookScript(t, "UserPromptSubmit", stdin, "sess-prompt")

	if ev.Event != "UserPromptSubmit" {
		t.Errorf("event = %q, want UserPromptSubmit", ev.Event)
	}
	if ev.Message != "Fix the auth bug" {
		t.Errorf("message = %q, want the prompt text 'Fix the auth bug'", ev.Message)
	}
	if ev.MtID != 5 {
		t.Errorf("mt_id = %d, want 5", ev.MtID)
	}
}
