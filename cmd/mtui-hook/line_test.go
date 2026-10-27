package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

// runHook feeds one Claude Code hook payload through run(), exactly as Claude
// would (event name as argv[1], JSON on stdin), and returns the line it wrote
// as the host's watcher will read it.
func runHook(t *testing.T, event, payload string) hooks.Event {
	t.Helper()
	appdata := t.TempDir()
	t.Setenv("APPDATA", appdata)
	t.Setenv("MULTITERMINAL_SESSION_ID", "7")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.WriteString(payload)
	w.Close()
	oldIn, oldArgs := os.Stdin, os.Args
	os.Stdin, os.Args = r, []string{"mtui-hook", event}
	defer func() { os.Stdin, os.Args = oldIn, oldArgs }()

	run()

	f, err := os.Open(filepath.Join(appdata, "Multiterminal", "hooks", "s1.jsonl"))
	if err != nil {
		t.Fatalf("no hook line written: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("hook file is empty")
	}
	var ev hooks.Event
	if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
		t.Fatalf("line does not decode as hooks.Event: %v", err)
	}
	return ev
}

func TestHookLine_StopCarriesTheEndOfTheLastMessage(t *testing.T) {
	long := strings.Repeat("Zwischenstand. ", 100) + "\n\nSoll ich pushen und den PR gegen main öffnen?"
	payload, _ := json.Marshal(map[string]any{"session_id": "s1", "last_assistant_message": long})
	ev := runHook(t, "Stop", string(payload))

	if ev.Event != "Stop" || ev.MtID != 7 || ev.SessionID != "s1" {
		t.Errorf("header wrong: %+v", ev)
	}
	if !strings.HasSuffix(ev.Message, "Soll ich pushen und den PR gegen main öffnen?") {
		t.Errorf("the question at the end was lost: %q", ev.Message)
	}
	if n := len([]rune(ev.Message)); n > stopMessageTail {
		t.Errorf("message is %d runes, want at most %d", n, stopMessageTail)
	}
}

func TestHookLine_NotificationCarriesItsType(t *testing.T) {
	ev := runHook(t, "Notification", `{"session_id":"s1","message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`)
	if ev.NotificationType != "permission_prompt" {
		t.Errorf("notification_type = %q", ev.NotificationType)
	}
	if ev.Message != "Claude needs your permission to use Bash" {
		t.Errorf("message = %q", ev.Message)
	}
}

func TestBashFailureCarriesTurnFacts(t *testing.T) {
	ev := runHook(t, "PostToolUseFailure", `{"session_id":"s1","prompt_id":"p-1","tool_name":"Bash",
		"tool_input":{"command":"echo vorher; exit 3","description":"x"},"error":"Exit code 3\nvorher","is_interrupt":false}`)
	if ev.PromptID != "p-1" || ev.Command != "echo vorher; exit 3" || ev.Detail != "Bash: echo vorher; exit 3" ||
		ev.OutputTail != "Exit code 3\nvorher" || ev.Interrupt {
		t.Errorf("turn facts wrong: %+v", ev)
	}
}

func TestBashSuccessTailIsStdoutAndStderr(t *testing.T) {
	ev := runHook(t, "PostToolUse", `{"session_id":"s1","prompt_id":"p-1","tool_name":"Bash",
		"tool_input":{"command":"echo ok"},"tool_response":{"stdout":"ok","stderr":"warn","interrupted":false}}`)
	if ev.OutputTail != "ok\nwarn" || ev.Command != "echo ok" {
		t.Errorf("got command %q tail %q", ev.Command, ev.OutputTail)
	}
}

func TestOutputTailKeepsLast300Runes(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"session_id": "s1", "tool_name": "Bash",
		"tool_input":    map[string]any{"command": "x"},
		"tool_response": map[string]any{"stdout": strings.Repeat("x", 1000) + "END"}})
	ev := runHook(t, "PostToolUse", string(payload))
	if n := len([]rune(ev.OutputTail)); n != 300 || !strings.HasSuffix(ev.OutputTail, "END") {
		t.Errorf("tail is %d runes: %q", n, ev.OutputTail)
	}
}

func TestInterruptIsPassedOn(t *testing.T) {
	ev := runHook(t, "PostToolUseFailure", `{"session_id":"s1","tool_name":"Bash","tool_input":{"command":"sleep 9"},"error":"x","is_interrupt":true}`)
	if !ev.Interrupt {
		t.Errorf("is_interrupt lost: %+v", ev)
	}
}

func TestPromptAndStopCarryPromptID(t *testing.T) {
	for _, e := range []string{"UserPromptSubmit", "Stop"} {
		ev := runHook(t, e, `{"session_id":"s1","prompt_id":"p-2","prompt":"hi"}`)
		if ev.PromptID != "p-2" {
			t.Errorf("%s: prompt_id = %q", e, ev.PromptID)
		}
	}
}

func TestNonBashPostToolUseHasNoCommand(t *testing.T) {
	ev := runHook(t, "PostToolUse", `{"session_id":"s1","tool_name":"Edit","tool_input":{"file_path":"a.go","command":"nope"},"tool_response":{"stdout":"x"}}`)
	if ev.Command != "" || ev.OutputTail != "" {
		t.Errorf("non-Bash tool leaked facts: %+v", ev)
	}
}
