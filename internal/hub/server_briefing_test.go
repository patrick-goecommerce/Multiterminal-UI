package hub

import (
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func storedBriefing() Briefing {
	return Briefing{
		PromptID: "p1", BuiltAt: 1700000000, Prompt: "fix it", Closing: "done",
		Checks:   []CheckResult{{Label: "go test", Result: "pass"}},
		Git:      &GitFacts{Branch: "feature/x"},
		PR:       &PRFacts{Number: 7, Checks: "green"},
		Warnings: []string{WarnDirty},
		Notes:    []string{NoteCIStale},
	}
}

func storeBriefing(h *Embedded, id int, b Briefing) {
	h.desk.mu.Lock()
	h.desk.entries[id] = deskEntry{briefing: b}
	h.desk.mu.Unlock()
}

func TestBriefingOverTheWire(t *testing.T) {
	h := newTestHost(t, nil)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	want := storedBriefing()
	storeBriefing(h, 1, want)
	addr, _ := serveHost(t, h)
	r := dialTest(t, addr, newSink())

	got, ok, err := r.Briefing(1)
	if err != nil || !ok {
		t.Fatalf("Briefing: ok=%v err=%v", ok, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("briefing = %+v, want %+v", got, want)
	}
}

func TestNoBriefingIsNull(t *testing.T) {
	h := newTestHost(t, nil)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	addr, _ := serveHost(t, h)
	r := dialTest(t, addr, newSink())

	got, ok, err := r.Briefing(1)
	if err != nil || ok {
		t.Fatalf("Briefing: ok=%v err=%v, want false and no error", ok, err)
	}
	if !reflect.DeepEqual(got, Briefing{}) {
		t.Errorf("briefing = %+v, want zero", got)
	}
}

func TestUnknownSessionBriefing(t *testing.T) {
	h := newTestHost(t, nil)
	addr, _ := serveHost(t, h)
	r := dialTest(t, addr, newSink())

	if _, _, err := r.Briefing(99); !errors.Is(err, ErrNoSession) {
		t.Errorf("Briefing(99) err = %v, want ErrNoSession", err)
	}
	if err := r.SetTarget(99, "alpha-main"); !errors.Is(err, ErrNoSession) {
		t.Errorf("SetTarget(99) err = %v, want ErrNoSession", err)
	}
}

func TestSetTargetOverTheWire(t *testing.T) {
	h := newTestHost(t, nil)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	addr, _ := serveHost(t, h)
	r := dialTest(t, addr, newSink())

	if err := r.SetTarget(1, "alpha-main"); err != nil {
		t.Fatalf("SetTarget: %v", err)
	}
	if got := h.desk.target(1); got != "alpha-main" {
		t.Errorf("target = %q, want alpha-main", got)
	}
	if err := r.SetTarget(1, ""); err != nil {
		t.Fatalf("SetTarget clear: %v", err)
	}
	if got := h.desk.target(1); got != "" {
		t.Errorf("target after clear = %q, want empty", got)
	}
}

func TestBriefingEventReachesRemote(t *testing.T) {
	sink := newBriefingSink()
	var server *Server
	host := NewEmbedded(Options{Version: "test", Sink: SinkFunc(func(name string, payload any) {
		server.Sink().Emit(name, payload)
	})})
	t.Cleanup(host.Release)
	server = NewServer(host, testToken)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	dialTest(t, strings.TrimPrefix(ts.URL, "http://"), sink)

	want := storedBriefing()
	host.emit(EventSessionBriefing, BriefingUpdate{ID: 3, Briefing: want})
	u := <-sink.ch
	if u.ID != 3 || !reflect.DeepEqual(u.Briefing, want) {
		t.Errorf("update = %+v, want id 3 and %+v", u, want)
	}
}
