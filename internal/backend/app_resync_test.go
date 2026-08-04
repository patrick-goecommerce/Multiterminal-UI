package backend

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// newResyncTestSession returns a session whose Screen holds known content.
func newResyncTestSession(t *testing.T, id int, lines ...string) *terminal.Session {
	t.Helper()
	sess := terminal.NewSession(id, 4, 20)
	sess.Screen.Write([]byte(strings.Join(lines, "\r\n")))
	return sess
}

// A resync must never leave the frontend with a partial byte stream. The stale
// bytes still queued in the batcher belong to the discarded backlog, so they
// must be dropped and replaced by a full repaint of the current screen.
func TestResyncSession_ReplacesPendingWithSnapshot(t *testing.T) {
	a := newTestApp()
	a.sessions[7] = newResyncTestSession(t, 7, "HEAD line", "second line")

	a.outputBatch().add(7, []byte("STALE-TAIL"))

	a.ResyncSession(7)

	payload := string(a.outputBatch().swap()[7])
	if payload == "" {
		t.Fatal("ResyncSession queued no payload")
	}
	if strings.Contains(payload, "STALE-TAIL") {
		t.Error("stale pending bytes survived the resync — the stream still has a hole")
	}
	if !strings.HasPrefix(payload, "\x1b[0m\x1b[2J") {
		t.Errorf("payload must start by resetting SGR and clearing the screen, got %q", payload[:min(16, len(payload))])
	}
	if !strings.Contains(payload, "HEAD line") || !strings.Contains(payload, "second line") {
		t.Errorf("payload is missing screen content: %q", payload)
	}
}

// Every row is addressed absolutely so a full-width row wrapping at the right
// margin cannot shift the rest of the repaint, and the cursor is restored last.
func TestResyncSession_AddressesRowsAndRestoresCursor(t *testing.T) {
	a := newTestApp()
	sess := terminal.NewSession(1, 3, 10)
	sess.Screen.Write([]byte("ab\r\ncd"))
	a.sessions[1] = sess

	a.ResyncSession(1)
	payload := string(a.outputBatch().swap()[1])

	for row := 1; row <= 3; row++ {
		want := "\x1b[" + itoa(row) + ";1H"
		if !strings.Contains(payload, want) {
			t.Errorf("payload does not position row %d (%q missing): %q", row, want, payload)
		}
	}

	// Screen cursor sits after "cd" on row 2 → 1-based \x1b[2;3H, emitted last.
	curRow, curCol := sess.Screen.Cursor()
	wantCursor := "\x1b[" + itoa(curRow+1) + ";" + itoa(curCol+1) + "H"
	if !strings.HasSuffix(payload, wantCursor) {
		t.Errorf("payload must end with the cursor position %q, got %q", wantCursor, payload)
	}
}

// A resync for one pane must not disturb the output queued for other panes.
func TestResyncSession_LeavesOtherSessionsUntouched(t *testing.T) {
	a := newTestApp()
	a.sessions[1] = newResyncTestSession(t, 1, "one")
	a.outputBatch().add(2, []byte("keep-me"))

	a.ResyncSession(1)

	if got := string(a.outputBatch().swap()[2]); got != "keep-me" {
		t.Errorf("session 2 pending output was disturbed: got %q, want %q", got, "keep-me")
	}
}

// Resyncing an unknown session must be a no-op, not a panic or an empty repaint
// that would clear a pane the frontend still has valid data for.
func TestResyncSession_UnknownSessionIsNoop(t *testing.T) {
	a := newTestApp()
	a.outputBatch().add(3, []byte("data"))

	a.ResyncSession(99)

	batch := a.outputBatch().swap()
	if _, ok := batch[99]; ok {
		t.Error("ResyncSession queued a payload for an unknown session")
	}
	if got := string(batch[3]); got != "data" {
		t.Errorf("unrelated pending output changed: got %q", got)
	}
}

// The snapshot must be rendered while the batcher is locked. If it were not, a
// PTY chunk arriving between the render and the replace would be discarded
// without being part of the snapshot — a fresh hole in the stream, which is the
// whole thing this path exists to prevent.
func TestReplaceWith_BuildsUnderLock(t *testing.T) {
	b := newOutputBatcher()

	lockedDuringBuild := false
	b.replaceWith(1, func() []byte {
		if b.mu.TryLock() {
			b.mu.Unlock()
		} else {
			lockedDuringBuild = true
		}
		return []byte("payload")
	})

	if !lockedDuringBuild {
		t.Error("build ran without the batcher lock held — a chunk added in that window would be dropped silently")
	}
	if got := string(b.swap()[1]); got != "payload" {
		t.Errorf("replaceWith queued %q, want %q", got, "payload")
	}
}

// replaceWith runs a callback that locks the session's screen while holding the
// batcher lock. collectOutput goroutines take those locks the other way round
// (screen first, then batcher), so this exercises both orders under contention
// to prove the added lock nesting cannot wedge the output pipeline.
func TestResyncSession_ConcurrentWithOutputDoesNotDeadlock(t *testing.T) {
	a := newTestApp()
	sess := terminal.NewSession(1, 24, 80)
	a.sessions[1] = sess

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(3)
			go func() { defer wg.Done(); sess.Screen.Write([]byte("output line\r\n")) }()
			go func() { defer wg.Done(); a.outputBatch().add(1, []byte("live bytes")) }()
			go func() { defer wg.Done(); a.ResyncSession(1) }()
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("resync deadlocked against concurrent screen writes and batcher appends")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
