package backend

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// hookFileMaxAge is how long an untouched hook file is kept before it is
// removed. Generous on purpose: a pane can sit idle for days, and the only cost
// of keeping a file too long is one directory entry.
const hookFileMaxAge = 7 * 24 * time.Hour

// hookPurgeInterval is how often stale files are swept while the app runs.
const hookPurgeInterval = time.Hour

// purgeStaleFiles removes hook files that have not been written to in
// hookFileMaxAge and returns how many it deleted.
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
func (hm *HookManager) purgeStaleFiles(maxAge time.Duration) int {
	entries, err := os.ReadDir(hm.dir)
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
		if err := os.Remove(filepath.Join(hm.dir, entry.Name())); err != nil {
			continue
		}
		hm.mu.Lock()
		delete(hm.offsets, entry.Name())
		hm.mu.Unlock()
		removed++
	}
	return removed
}

// logPurge reports a sweep, but only when it actually removed something — a
// line per hour saying "removed 0" is noise.
func (hm *HookManager) logPurge(removed int, when string) {
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
func (hm *HookManager) needsRead(name string, size int64) bool {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	offset, seen := hm.offsets[name]
	switch {
	case !seen:
		return true
	case size > offset:
		return true
	case size < offset:
		hm.offsets[name] = 0
		return true
	default:
		return false
	}
}
