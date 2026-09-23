package hooks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newPurgeTestManager builds a Watcher over a temp dir with no callback:
// these tests are about the directory, not about what an event does.
func newPurgeTestManager(t *testing.T) *Watcher {
	t.Helper()
	return NewWatcher(t.TempDir(), nil)
}

// writeHookFile creates a hook file and back-dates it, so age-based behaviour
// can be tested without waiting.
func writeHookFile(t *testing.T, dir, name, body string, age time.Duration) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("chtimes %s: %v", name, err)
	}
	return path
}

func TestPurgeStaleFiles_RemovesOnlyOldOnes(t *testing.T) {
	w := newPurgeTestManager(t)
	writeHookFile(t, w.dir, "old.jsonl", "{}\n", 10*24*time.Hour)
	writeHookFile(t, w.dir, "fresh.jsonl", "{}\n", time.Minute)
	writeHookFile(t, w.dir, "keep.txt", "not a hook file", 30*24*time.Hour)

	if got := w.purgeStaleFiles(7 * 24 * time.Hour); got != 1 {
		t.Fatalf("purged %d files, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(w.dir, "old.jsonl")); !os.IsNotExist(err) {
		t.Error("the stale file survived")
	}
	if _, err := os.Stat(filepath.Join(w.dir, "fresh.jsonl")); err != nil {
		t.Error("a fresh file was removed")
	}
	if _, err := os.Stat(filepath.Join(w.dir, "keep.txt")); err != nil {
		t.Error("a non-hook file was removed")
	}
}

// A purged file must not leave its offset behind: if the same session id comes
// back, a stale offset would seek past the end of the new file and swallow
// every event it carries.
func TestPurgeStaleFiles_DropsTheOffsetToo(t *testing.T) {
	w := newPurgeTestManager(t)
	writeHookFile(t, w.dir, "gone.jsonl", "{}\n{}\n", 10*24*time.Hour)
	w.offsets["gone.jsonl"] = 6

	w.purgeStaleFiles(7 * 24 * time.Hour)

	w.mu.Lock()
	_, still := w.offsets["gone.jsonl"]
	w.mu.Unlock()
	if still {
		t.Error("the offset entry outlived the file it described")
	}
}

func TestPurgeStaleFiles_MissingDirIsNotAnError(t *testing.T) {
	w := NewWatcher(filepath.Join(t.TempDir(), "does-not-exist"), nil)
	if got := w.purgeStaleFiles(time.Hour); got != 0 {
		t.Errorf("purged %d from a missing directory, want 0", got)
	}
}

// The size short-circuit decides whether a file is opened at all. Getting it
// wrong either costs the whole saving or silently drops events.
func TestNeedsRead(t *testing.T) {
	tests := []struct {
		name       string
		offset     int64
		seen       bool
		size       int64
		want       bool
		wantOffset int64
	}{
		{"unseen file is read", 0, false, 120, true, 0},
		{"grown file is read", 100, true, 180, true, 100},
		{"unchanged file is skipped", 140, true, 140, false, 140},
		{"empty and unseen is read", 0, false, 0, true, 0},
		{"empty and unchanged is skipped", 0, true, 0, false, 0},
		// A shrunken file means the name was reused by a new session; the old
		// offset points past its end, so it has to go back to the start.
		{"shrunken file resets the offset", 500, true, 40, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newPurgeTestManager(t)
			if tt.seen {
				w.offsets["s.jsonl"] = tt.offset
			}

			if got := w.needsRead("s.jsonl", tt.size); got != tt.want {
				t.Errorf("needsRead(size=%d, offset=%d, seen=%v) = %v, want %v", tt.size, tt.offset, tt.seen, got, tt.want)
			}

			w.mu.Lock()
			gotOffset := w.offsets["s.jsonl"]
			w.mu.Unlock()
			if tt.seen && gotOffset != tt.wantOffset {
				t.Errorf("offset = %d after the check, want %d", gotOffset, tt.wantOffset)
			}
		})
	}
}

// The point of the whole change: a directory full of untouched files must not
// be opened on every tick.
func TestProcessDirectory_SkipsUnchangedFiles(t *testing.T) {
	w := newPurgeTestManager(t)
	for _, n := range []string{"a.jsonl", "b.jsonl", "c.jsonl"} {
		writeHookFile(t, w.dir, n, "{\"event\":\"Stop\",\"mt_id\":1}\n", time.Minute)
	}

	// First pass records every offset.
	w.processDirectory()
	w.mu.Lock()
	after := len(w.offsets)
	w.mu.Unlock()
	if after != 3 {
		t.Fatalf("recorded %d offsets after the first pass, want 3", after)
	}

	// Nothing changed on disk, so a second pass must find nothing to read.
	for _, n := range []string{"a.jsonl", "b.jsonl", "c.jsonl"} {
		info, err := os.Stat(filepath.Join(w.dir, n))
		if err != nil {
			t.Fatalf("stat %s: %v", n, err)
		}
		if w.needsRead(n, info.Size()) {
			t.Errorf("%s would be re-opened although it did not change", n)
		}
	}

	// Appending to one file brings exactly that one back.
	path := filepath.Join(w.dir, "b.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := f.WriteString("{\"event\":\"Stop\",\"mt_id\":1}\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !w.needsRead("b.jsonl", info.Size()) {
		t.Error("the appended file was not picked up")
	}
}
