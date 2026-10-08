package hub

import (
	"sync"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// attentionSink keeps the attention payloads and every event name.
type attentionSink struct {
	mu      sync.Mutex
	updates []AttentionUpdate
	names   chan string
}

func newAttentionSink() *attentionSink { return &attentionSink{names: make(chan string, 256)} }

func (s *attentionSink) Emit(name string, payload any) {
	if name == EventSessionAttention {
		if u, ok := DecodePayload[AttentionUpdate](payload); ok {
			s.mu.Lock()
			s.updates = append(s.updates, u)
			s.mu.Unlock()
		}
	}
	select {
	case s.names <- name:
	default:
	}
}

func (s *attentionSink) last(t *testing.T) AttentionUpdate {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.updates) == 0 {
		t.Fatal("no attention update was emitted")
	}
	return s.updates[len(s.updates)-1]
}

func (s *attentionSink) all() []AttentionUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AttentionUpdate(nil), s.updates...)
}

func (s *attentionSink) await(t *testing.T, name string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case got := <-s.names:
			if got == name {
				return
			}
		case <-deadline:
			t.Fatalf("event %q never arrived", name)
		}
	}
}

// confirmNow drives one state through the debounce without sleeping.
func confirmNow(t *testing.T, h *Embedded, id int) {
	t.Helper()
	h.ScanActivity()
	if !h.BackdateActivityForTest(id) {
		t.Fatal("no candidate armed")
	}
	h.ScanActivity()
}

func TestEmbedded_FinishedPaneIsUnseenUntilMarkSeen(t *testing.T) {
	sink := newAttentionSink()
	h := newTestHost(t, sink)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	h.applyHookEvent(nil, hooks.Event{Event: "Stop", MtID: 1, Message: "Fertig.\n\nAlle Tests grün."})
	confirmNow(t, h, 1)

	a := sink.last(t).Attention
	if a.State != ActivityDone || a.Seen || a.Rank != RankDone || a.Detail != "Alle Tests grün." || a.Source != "Hook: Stop" {
		t.Fatalf("after Stop: %+v", a)
	}
	if err := h.MarkSeen(1); err != nil {
		t.Fatal(err)
	}
	if a := sink.last(t).Attention; !a.Seen || a.Rank != RankQuiet {
		t.Errorf("after MarkSeen: %+v", a)
	}
	if got, _ := h.Get(1); got.Attention.Rank != RankQuiet {
		t.Errorf("summary rank %d, want %d", got.Attention.Rank, RankQuiet)
	}
	if err := h.MarkUnseen(1); err != nil {
		t.Fatal(err)
	}
	if a := sink.last(t).Attention; a.Seen || a.Rank != RankDone {
		t.Errorf("after MarkUnseen: %+v", a)
	}
}

func TestEmbedded_MarkSeenOnAnUnknownSessionFails(t *testing.T) {
	h := newTestHost(t, nil)
	if err := h.MarkSeen(99); err == nil {
		t.Error("want an error for an unknown session")
	}
}

func TestEmbedded_SleepKeepsTheFinishedPaneOnTheList(t *testing.T) {
	h := newTestHost(t, newAttentionSink())
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	_ = h.ForceActivity(1, ActivityDone, time.Now())
	_ = h.ForceActivity(1, ActivitySleeping, time.Now())
	if got, _ := h.Get(1); got.Attention.Rank != RankDone {
		t.Errorf("rank %d after sleep, want %d", got.Attention.Rank, RankDone)
	}
}

// A background subagent at work after the turn ended must not repaint the
// pane as running; its question would vanish (spec Teil C4).
func TestEmbedded_SubagentWorkDoesNotEndATurn(t *testing.T) {
	h := newTestHost(t, newAttentionSink())
	sess := terminal.NewSession(1, 24, 80)
	h.AdoptForTest(1, sess)
	h.applyHookEvent(nil, hooks.Event{Event: "Stop", MtID: 1, Message: "Soll ich pushen?"})
	h.applyHookEvent(nil, hooks.Event{Event: "PreToolUse", MtID: 1, Tool: "Bash", AgentID: "a-1"})
	if got := sess.GetActivity(); got != terminal.ActivityWaitingAnswer {
		t.Errorf("after subagent tool use: %v, want waitingAnswer", got)
	}
	// A subagent that needs a permission blocks the user and still counts.
	h.applyHookEvent(nil, hooks.Event{Event: "PermissionRequest", MtID: 1, Tool: "Bash", AgentID: "a-1"})
	if got := sess.GetActivity(); got != terminal.ActivityWaitingPermission {
		t.Errorf("after subagent permission: %v, want waitingPermission", got)
	}
}
