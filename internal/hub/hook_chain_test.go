package hub

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// The hook chain end to end, the way it runs in daemon mode: a line in the
// hooks directory (what mtui-hook writes), the daemon's watcher and host, the
// wire, and the client's sink. What arrives there is what the window paints.
// Then the host's own scan has to confirm the same state, which is what
// badges, queues and the waiting list settle on.

type hookChain struct {
	t       *testing.T
	dir     string
	host    *Embedded
	remote  *Remote
	reports chan HookReport
}

func newHookChain(t *testing.T) *hookChain {
	t.Helper()
	c := &hookChain{t: t, dir: t.TempDir(), reports: make(chan HookReport, 64)}
	var server *Server
	c.host = NewEmbedded(Options{Version: "test", HooksDir: c.dir, Scan: true, Sink: SinkFunc(func(name string, payload any) {
		server.Sink().Emit(name, payload)
	})})
	t.Cleanup(c.host.Release)
	server = NewServer(c.host, testToken)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	c.remote = dialTest(t, strings.TrimPrefix(ts.URL, "http://"), SinkFunc(func(name string, payload any) {
		if name != EventSessionHook {
			return
		}
		if r, ok := DecodePayload[HookReport](payload); ok {
			c.reports <- r
		}
	}))
	return c
}

// write appends one hook line, as mtui-hook would, and returns the report the
// client received for it.
func (c *hookChain) write(ev hooks.Event) HookReport {
	c.t.Helper()
	line, _ := json.Marshal(ev)
	f, err := os.OpenFile(filepath.Join(c.dir, ev.SessionID+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		c.t.Fatal(err)
	}
	_, _ = f.Write(append(line, '\n'))
	f.Close()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case r := <-c.reports:
			if r.Event == ev.Event && r.Session == ev.MtID {
				return r
			}
		case <-deadline:
			c.t.Fatalf("no report for %s reached the client", ev.Event)
		}
	}
}

func (c *hookChain) awaitConfirmed(id int, want Activity) {
	c.t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ := c.remote.ConfirmedActivity(id); got == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	got, _ := c.remote.ConfirmedActivity(id)
	c.t.Fatalf("confirmed state = %q, want %q", got, want)
}

func TestHookChain_EveryStateReachesTheClient(t *testing.T) {
	c := newHookChain(t)
	c.host.AdoptForTest(7, terminal.NewSession(7, 24, 80))
	// The watcher skips what exists when it starts; let it take its first look.
	time.Sleep(300 * time.Millisecond)

	steps := []struct {
		name string
		ev   hooks.Event
		want Activity
	}{
		{"prompt", hooks.Event{Event: "UserPromptSubmit", Message: "Bau das ein"}, ActivityActive},
		{"permission", hooks.Event{Event: "PermissionRequest", Tool: "Bash"}, ActivityWaitingPermission},
		{"typed notification", hooks.Event{Event: "Notification", NotificationType: "permission_prompt", Message: "Claude needs your permission to use Bash"}, ActivityWaitingPermission},
		{"tool ran", hooks.Event{Event: "PostToolUse", Tool: "Bash"}, ActivityActive},
		{"tool failed", hooks.Event{Event: "PostToolUseFailure", Tool: "Bash"}, ActivityError},
		{"ask user", hooks.Event{Event: "PreToolUse", Tool: "AskUserQuestion"}, ActivityWaitingAnswer},
		{"answered", hooks.Event{Event: "PostToolUse", Tool: "AskUserQuestion"}, ActivityActive},
		{"elicitation", hooks.Event{Event: "Notification", NotificationType: "elicitation_dialog"}, ActivityWaitingAnswer},
		{"stop", hooks.Event{Event: "Stop", Message: "Alles erledigt."}, ActivityDone},
		{"stop with a question", hooks.Event{Event: "Stop", Message: "Tests grün.\n\nSoll ich pushen und den PR gegen main öffnen? Danach startet das Review."}, ActivityWaitingAnswer},
	}
	for _, s := range steps {
		s.ev.MtID, s.ev.SessionID = 7, "agent-7"
		r := c.write(s.ev)
		if r.Activity != s.want {
			t.Errorf("%s: client got %q, want %q", s.name, r.Activity, s.want)
		}
	}

	// The last state holds, so the host's scan confirms it and the client
	// reads the same over the wire.
	c.awaitConfirmed(7, ActivityWaitingAnswer)
}

// A reminder carries no state: it must reach the client without one, and it
// must not undo the question the session is waiting on.
func TestHookChain_AReminderLeavesTheStateAlone(t *testing.T) {
	c := newHookChain(t)
	c.host.AdoptForTest(8, terminal.NewSession(8, 24, 80))
	time.Sleep(300 * time.Millisecond)

	c.write(hooks.Event{Event: "Stop", MtID: 8, SessionID: "agent-8", Message: "Passt das so?"})
	r := c.write(hooks.Event{Event: "Notification", MtID: 8, SessionID: "agent-8", NotificationType: "idle_prompt", Message: "Claude is waiting for your input"})
	if r.Activity != "" {
		t.Errorf("idle reminder carried a state: %q", r.Activity)
	}
	c.awaitConfirmed(8, ActivityWaitingAnswer)
}
