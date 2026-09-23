// Package hooks reads the lifecycle events Claude Code writes through
// mtui-hook.
//
// The hook binary appends one JSON object per event to a per-session file
// (see cmd/mtui-hook); this package tails those files and hands each event to
// a callback. It lives on its own because the reader belongs wherever the
// sessions are: with the session daemon that is mtuid, not the window, and an
// agent's state has to keep being recorded while no window is open.
package hooks

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Event is the JSONL structure written by mtui-hook.
type Event struct {
	Ts             int64  `json:"ts"`
	Event          string `json:"event"`
	SessionID      string `json:"session_id"`
	MtID           int    `json:"mt_id"`
	Tool           string `json:"tool"`
	Message        string `json:"message"`
	Cwd            string `json:"cwd"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
	BlockedPath    string `json:"blocked_path"`
	BlockReason    string `json:"block_reason"`
}

// Watcher tails a hooks directory and reports what it finds.
type Watcher struct {
	dir     string
	onEvent func(Event)

	mu      sync.Mutex
	offsets map[string]int64 // filename → bytes already read
}

// NewWatcher returns a watcher over dir. onEvent is called for every event
// read, in file order, from the watcher's own goroutine.
func NewWatcher(dir string, onEvent func(Event)) *Watcher {
	return &Watcher{dir: dir, onEvent: onEvent, offsets: make(map[string]int64)}
}

// Start begins polling the hooks directory every 100ms.
// Existing files are seeked to their current end so that events from previous
// app sessions are not replayed (session IDs reset on each start, so old
// events would otherwise match new sessions and cause spurious state jumps).
func (w *Watcher) Start(ctx context.Context) {
	if err := os.MkdirAll(w.dir, 0755); err != nil {
		log.Printf("[hooks] could not create hooks dir: %v — hook integration disabled", err)
		return
	}
	// Purge before recording offsets: a file removed here must not leave an
	// entry behind in the offsets map.
	w.logPurge(w.purgeStaleFiles(FileMaxAge), "at startup")
	w.skipExistingFiles()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		purge := time.NewTicker(purgeInterval)
		defer purge.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.processDirectory()
			case <-purge.C:
				// A long-running app would otherwise accumulate for days
				// between restarts — which is exactly how this got out of hand.
				w.logPurge(w.purgeStaleFiles(FileMaxAge), "during periodic sweep")
			}
		}
	}()
}

// skipExistingFiles records the current end-of-file offset for each JSONL
// file already present so that stale events from previous sessions are ignored.
func (w *Watcher) skipExistingFiles() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		w.offsets[entry.Name()] = info.Size()
	}
}

// processDirectory scans the hooks directory for new JSONL events.
//
// Only files whose size differs from the recorded offset are opened. That guard
// is not a micro-optimisation: a file is written once per hook event and then
// stays untouched forever, so at any tick nearly every file in the directory has
// nothing new. Opening them all regardless cost 85 ms per pass against a 100 ms
// ticker — 85 % of a core, indefinitely (issue #192). os.ReadDir already carries
// the size on Windows, so the check itself is free.
func (w *Watcher) processDirectory() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// Vanished between ReadDir and Info, or unreadable — try a full read
			// rather than skipping, so a transient error cannot drop events.
			w.processFile(filepath.Join(w.dir, entry.Name()), entry.Name())
			continue
		}
		if !w.needsRead(entry.Name(), info.Size()) {
			continue
		}
		w.processFile(filepath.Join(w.dir, entry.Name()), entry.Name())
	}
}

// processFile reads new lines from a JSONL file since the last read offset.
// Events are collected while the file is open, then dispatched after closing
// so that handleEvent (which may delete the file on Windows) never races with
// an open file handle.
func (w *Watcher) processFile(path, name string) {
	w.mu.Lock()
	offset := w.offsets[name]
	w.mu.Unlock()

	events, newOffset := w.readEvents(path, offset)

	w.mu.Lock()
	w.offsets[name] = newOffset
	w.mu.Unlock()

	for _, ev := range events {
		// An event without a session id belongs to nothing we track: the hook
		// fired outside a pane MTUI launched.
		if ev.MtID == 0 {
			continue
		}
		if w.onEvent != nil {
			w.onEvent(ev)
		}
	}
}

// readEvents opens the file, seeks to offset, and collects all new events.
// Returns the parsed events and the new file offset. The file is closed before
// returning so callers can safely delete it on Windows.
func (w *Watcher) readEvents(path string, offset int64) ([]Event, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return nil, offset
		}
	}

	var events []Event
	scanner := bufio.NewScanner(f)
	newOffset := offset
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		newOffset += int64(len(scanner.Bytes())) + 1 // +1 for newline
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err == nil {
			events = append(events, ev)
		}
	}
	return events, newOffset
}

// Forget drops a session's file and the offset that went with it. The reader
// calls it when a session ends, so a name that is reused later is read from
// the start rather than seeked past the end of a file that no longer exists.
func (w *Watcher) Forget(agentSessionID string) {
	name := agentSessionID + ".jsonl"
	w.mu.Lock()
	delete(w.offsets, name)
	w.mu.Unlock()
	_ = os.Remove(filepath.Join(w.dir, name))
}
