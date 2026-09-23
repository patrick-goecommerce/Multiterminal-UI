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
