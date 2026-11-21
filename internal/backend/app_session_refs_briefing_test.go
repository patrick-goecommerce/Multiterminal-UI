package backend

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// briefingRecorder is a Host that notes Briefing/SetTarget and answers on request.
type briefingRecorder struct {
	hub.Host
	briefed []int
	targets []string
	b       hub.Briefing
	ok      bool
	fail    error
}

func (r *briefingRecorder) Briefing(id int) (hub.Briefing, bool, error) {
	r.briefed = append(r.briefed, id)
	return r.b, r.ok, r.fail
}

func (r *briefingRecorder) SetTarget(id int, branch string) error {
	r.targets = append(r.targets, branch)
	r.briefed = append(r.briefed, id)
	return r.fail
}

func TestGetSessionBriefingTranslatesRef(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	rec := &briefingRecorder{Host: a.host, ok: true, b: hub.Briefing{PromptID: "p1", Prompt: "fix it"}}
	a.host = rec

	got := a.GetSessionBriefing(a.ref(4))
	if got == nil || got.PromptID != "p1" || got.Prompt != "fix it" {
		t.Fatalf("briefing = %+v, want the host's", got)
	}
	if len(rec.briefed) != 1 || rec.briefed[0] != 4 {
		t.Errorf("host asked for %v, want [4]", rec.briefed)
	}

	rec.ok = false
	if a.GetSessionBriefing(a.ref(4)) != nil {
		t.Error("no briefing yet must be nil")
	}
	// An older daemon answers 404, which is "no briefing", not a dead session.
	rec.ok, rec.fail = true, hub.ErrNoSession
	if a.GetSessionBriefing(a.ref(4)) != nil {
		t.Error("an error must be nil")
	}
}

func TestForeignRefNeverReachesHost(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	rec := &briefingRecorder{Host: a.host, ok: true}
	a.host = rec

	if a.GetSessionBriefing(hub.Local("elsewhere", 4)) != nil || a.GetSessionBriefing(hub.Ref{}) != nil {
		t.Error("a foreign ref must give nil")
	}
	a.SetSessionTarget(hub.Local("elsewhere", 4), "main")
	a.SetSessionTarget(hub.Ref{}, "main")
	if len(rec.briefed) != 0 || len(rec.targets) != 0 {
		t.Errorf("host was reached: briefed=%v targets=%v", rec.briefed, rec.targets)
	}
}

func TestSetSessionTargetLogsError(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	rec := &briefingRecorder{Host: a.host}
	a.host = rec

	a.SetSessionTarget(a.ref(6), "develop")
	if len(rec.targets) != 1 || rec.targets[0] != "develop" || rec.briefed[0] != 6 {
		t.Errorf("targets=%v ids=%v, want develop on 6", rec.targets, rec.briefed)
	}

	rec.fail = errors.New("boom")
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	a.SetSessionTarget(a.ref(6), "develop")
	if !strings.Contains(buf.String(), "[briefing] SetTarget: boom") {
		t.Errorf("log = %q, want the failure", buf.String())
	}
}
