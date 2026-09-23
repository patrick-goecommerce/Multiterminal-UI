package backend

import (
	"context"
	"encoding/base64"
	"log"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// outputBatcher accumulates raw PTY bytes from all sessions and emits
// them as a single batched Wails event per frame (≤16 ms).
//
// This eliminates Win32 main-thread saturation: instead of one
// ExecJS/InvokeSync call per session per coalesce window we make exactly
// one call per 16 ms frame regardless of how many sessions are active.
type outputBatcher struct {
	mu      sync.Mutex
	pending map[int][]byte // sessionID → accumulated bytes
}

func newOutputBatcher() *outputBatcher {
	return &outputBatcher{pending: make(map[int][]byte)}
}

// outputBatch returns the shared output batcher, initializing it on first use.
// The batcher's lifecycle is the AppService's, not ServiceStartup's: sessions
// (and thus the output pump) can be created before ServiceStartup — e.g. scheduled
// tasks in tests — so the batcher must never be nil.
func (a *AppService) outputBatch() *outputBatcher {
	a.batcherOnce.Do(func() {
		if a.batcher == nil {
			a.batcher = newOutputBatcher()
		}
	})
	return a.batcher
}

// add appends raw bytes for a session into the accumulation buffer.
func (b *outputBatcher) add(id int, data []byte) {
	b.mu.Lock()
	b.pending[id] = append(b.pending[id], data...)
	b.mu.Unlock()
}

// replaceWith discards the bytes still queued for a session and queues the
// result of build in their place. Used by ResyncSession: those bytes belong to a
// backlog the frontend has already thrown away, so forwarding them would only
// prepend noise to the repaint.
//
// build runs while the lock is held, on purpose. It renders the session's screen,
// and a chunk that slipped in between rendering and replacing would be dropped
// without being part of the snapshot — exactly the stream hole this whole path
// exists to prevent. Holding the lock instead means such a chunk arrives after
// the repaint and is merely applied twice. build must not call back into the
// batcher.
func (b *outputBatcher) replaceWith(id int, build func() []byte) {
	b.mu.Lock()
	b.pending[id] = build()
	b.mu.Unlock()
}

// swap atomically replaces the pending map with an empty one and
// returns the old map for emission.
func (b *outputBatcher) swap() map[int][]byte {
	b.mu.Lock()
	old := b.pending
	b.pending = make(map[int][]byte, len(old))
	b.mu.Unlock()
	return old
}

// batchLoop emits one terminal:output-batch event per 16 ms tick.
// Must be started as a goroutine in ServiceStartup.
func (a *AppService) batchLoop(ctx context.Context) {
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()
	var emitCount int64
	var totalBytes int64
	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-logTicker.C:
			log.Printf("[batchLoop] emits/5s=%d totalBytes=%d", emitCount, totalBytes)
			emitCount = 0
			totalBytes = 0
		case <-ticker.C:
			batch := a.outputBatch().swap()
			if len(batch) == 0 {
				continue
			}
			items := make([]TerminalOutputEvent, 0, len(batch))
			var batchBytes int
			hubID := a.hubID() // once per flush, not per session
			for id, raw := range batch {
				batchBytes += len(raw)
				items = append(items, TerminalOutputEvent{
					ID:   hub.Local(hubID, id),
					Data: base64.StdEncoding.EncodeToString(raw),
				})
			}
			emitCount++
			totalBytes += int64(batchBytes)
			a.app.Event.Emit("terminal:output-batch", items)
		}
	}
}
