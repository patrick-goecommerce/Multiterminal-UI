package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// write appends one event to a session's file, the way mtui-hook does.
func write(t *testing.T, dir, agentSessionID string, ev Event) {
	t.Helper()
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, agentSessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// recorder collects what the watcher reports.
type recorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recorder) add(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *recorder) all() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

func TestWatcher_ReportsANewEvent(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	w := NewWatcher(dir, rec.add)

	write(t, dir, "claude-abc", Event{
		Ts: time.Now().Unix(), Event: "PermissionRequest",
		SessionID: "claude-abc", MtID: 42, Tool: "Bash",
	})
	w.processDirectory()

	got := rec.all()
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].MtID != 42 || got[0].Event != "PermissionRequest" || got[0].SessionID != "claude-abc" {
		t.Errorf("event = %+v", got[0])
	}
}

// Each pass must report only what was appended since the last one. Re-reading
// from the start would replay a session's whole history on every tick.
func TestWatcher_ReadsIncrementally(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	w := NewWatcher(dir, rec.add)

	write(t, dir, "s1", Event{Event: "PreToolUse", SessionID: "s1", MtID: 10})
	w.processDirectory()
	write(t, dir, "s1", Event{Event: "Stop", SessionID: "s1", MtID: 10})
	w.processDirectory()

	got := rec.all()
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (one per pass)", len(got))
	}
	if got[0].Event != "PreToolUse" || got[1].Event != "Stop" {
		t.Errorf("events = %q, %q", got[0].Event, got[1].Event)
	}

	// A third pass has nothing new to say.
	w.processDirectory()
	if len(rec.all()) != 2 {
		t.Errorf("a pass with no new lines reported %d events", len(rec.all())-2)
	}
}

// An event that names no MTUI session belongs to a hook that fired outside a
// pane this app launched.
func TestWatcher_IgnoresAnEventWithoutASessionID(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	w := NewWatcher(dir, rec.add)

	write(t, dir, "no-mt", Event{Event: "PreToolUse", SessionID: "no-mt", MtID: 0})
	w.processDirectory()

	if got := rec.all(); len(got) != 0 {
		t.Errorf("reported %d events for an unowned hook", len(got))
	}
}

// Events written before the watcher started belong to a previous run. Session
// ids restart, so replaying them would drive today's panes from yesterday's
// events.
func TestWatcher_SkipsWhatWasThereBeforeItStarted(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "old", Event{Event: "Stop", SessionID: "old", MtID: 1})

	rec := &recorder{}
	w := NewWatcher(dir, rec.add)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)
	time.Sleep(300 * time.Millisecond)

	if got := rec.all(); len(got) != 0 {
		t.Errorf("replayed %d events from a previous run", len(got))
	}

	// Something written after the start is reported.
	write(t, dir, "old", Event{Event: "PreToolUse", SessionID: "old", MtID: 1})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(rec.all()) == 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("a new event was never reported; saw %v", rec.all())
}

// A file that shrank was reused for a new session: the recorded offset points
// past its end, and seeking there would swallow every event until a restart.
func TestWatcher_RereadsAFileThatShrank(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	w := NewWatcher(dir, rec.add)

	write(t, dir, "s1", Event{Event: "PreToolUse", SessionID: "s1", MtID: 3})
	write(t, dir, "s1", Event{Event: "PostToolUse", SessionID: "s1", MtID: 3})
	w.processDirectory()
	if len(rec.all()) != 2 {
		t.Fatalf("setup read %d events, want 2", len(rec.all()))
	}

	// The name is reused with a shorter file.
	if err := os.Remove(filepath.Join(dir, "s1.jsonl")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	write(t, dir, "s1", Event{Event: "Stop", SessionID: "s1", MtID: 3})
	w.processDirectory()

	got := rec.all()
	if len(got) != 3 || got[2].Event != "Stop" {
		t.Errorf("after the file was replaced, events = %+v", got)
	}
}

// Forget removes the file and its offset, so a session id that comes back is
// read from the start.
func TestWatcher_ForgetRemovesTheFile(t *testing.T) {
	dir := t.TempDir()
	w := NewWatcher(dir, func(Event) {})

	write(t, dir, "gone", Event{Event: "Stop", SessionID: "gone", MtID: 1})
	w.processDirectory()
	w.Forget("gone")

	if _, err := os.Stat(filepath.Join(dir, "gone.jsonl")); !os.IsNotExist(err) {
		t.Errorf("file still there after Forget: %v", err)
	}
}
