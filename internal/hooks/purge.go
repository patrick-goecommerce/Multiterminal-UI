package hooks

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileMaxAge is how long an untouched hook file is kept before it is
// removed. Generous on purpose: a pane can sit idle for days, and the only cost
// of keeping a file too long is one directory entry.
const FileMaxAge = 7 * 24 * time.Hour

// purgeInterval is how often stale files are swept while the app runs.
const purgeInterval = time.Hour

// purgeStaleFiles removes hook files that have not been written to in
// FileMaxAge and returns how many it deleted.
//
// Without this the directory only ever grows. A file is deleted on a SessionEnd
// event and nowhere else, so every Claude process that dies without one — crash,
// kill, PTY close, app exit, reboot — leaves its file behind for good. Measured
// on a real installation: 1038 files after ten days, accumulating at ~99 per day
// (issue #192).
//
// Deleting a file whose session later revives is safe: the offset entry goes
// with it, so the recreated file is read from the start rather than seeked past
// its end.
func (w *Watcher) purgeStaleFiles(maxAge time.Duration) int {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(w.dir, entry.Name())); err != nil {
			continue
		}
		w.mu.Lock()
		delete(w.offsets, entry.Name())
		w.mu.Unlock()
		removed++
	}
	return removed
}

// logPurge reports a sweep, but only when it actually removed something — a
// line per hour saying "removed 0" is noise.
func (w *Watcher) logPurge(removed int, when string) {
	if removed > 0 {
		log.Printf("[hooks] purged %d stale hook file(s) %s", removed, when)
	}
}

// needsRead reports whether a file has content the reader has not seen, and
// repairs the offset when a file was replaced.
//
// A shrunken file means the name was reused for a new session: the recorded
// offset points past its end, and seeking there would silently swallow every
// event until the next app start. Resetting to 0 costs one re-read of a small
// file and keeps the pane's hook events flowing.
func (w *Watcher) needsRead(name string, size int64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	offset, seen := w.offsets[name]
	switch {
	case !seen:
		return true
	case size > offset:
		return true
	case size < offset:
		w.offsets[name] = 0
		return true
	default:
		return false
	}
}
