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

// seenRecorder is a Host that notes MarkSeen/MarkUnseen and fails on request.
type seenRecorder struct {
	hub.Host
	seen, unseen []int
	fail         error
}

func (s *seenRecorder) MarkSeen(id int) error {
	s.seen = append(s.seen, id)
	return s.fail
}

func (s *seenRecorder) MarkUnseen(id int) error {
	s.unseen = append(s.unseen, id)
	return s.fail
}

func TestMarkSessionSeen_TranslatesTheRef(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	rec := &seenRecorder{Host: a.host}
	a.host = rec

	a.MarkSessionSeen(a.ref(4))
	a.MarkSessionUnseen(a.ref(5))
	a.MarkSessionSeen(hub.Local("elsewhere", 4)) // not ours
	a.MarkSessionUnseen(hub.Ref{})

	if len(rec.seen) != 1 || rec.seen[0] != 4 || len(rec.unseen) != 1 || rec.unseen[0] != 5 {
		t.Errorf("seen=%v unseen=%v, want [4] and [5]", rec.seen, rec.unseen)
	}
}

// A failed mark used to vanish (`_ =`). It is logged now, so a pane that will
// not leave the list leaves a trace.
func TestMarkSessionSeen_LogsAFailure(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	a.host = &seenRecorder{Host: a.host, fail: errors.New("boom")}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	a.MarkSessionSeen(a.ref(7))
	a.MarkSessionUnseen(a.ref(8))
	out := buf.String()
	if !strings.Contains(out, "[attention] MarkSeen 7: boom") || !strings.Contains(out, "[attention] MarkUnseen 8: boom") {
		t.Errorf("log = %q, want both failures", out)
	}
}
