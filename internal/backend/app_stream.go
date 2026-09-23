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
	// wake has room for one signal: "something is pending". The loop sleeps
	// on it instead of ticking 62 times a second through empty frames.
	wake chan struct{}
}

func newOutputBatcher() *outputBatcher {
	return &outputBatcher{pending: make(map[int][]byte), wake: make(chan struct{}, 1)}
}

// signal wakes the loop if it is not already awake.
func (b *outputBatcher) signal() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
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
	b.signal()
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
	b.signal()
}

// swap atomically replaces the pending map with an empty one and
// returns the old map for emission.
func (b *outputBatcher) swap() map[int][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		return nil
	}
	old := b.pending
	b.pending = make(map[int][]byte, len(old))
	return old
}

// outputFrame is how long the loop gathers output before emitting it: one
// frame, so a burst from many sessions becomes one event.
const outputFrame = 16 * time.Millisecond

// batchLoop emits one terminal:output-batch event per frame with output.
// Must be started as a goroutine in ServiceStartup.
func (a *AppService) batchLoop(ctx context.Context) {
	var emits, bytes int64
	lastLog := time.Now()
	runBatchLoop(ctx, a.outputBatch(), func(batch map[int][]byte) {
		emits++
		bytes += int64(a.emitOutputBatch(batch))
		if time.Since(lastLog) >= 5*time.Second {
			log.Printf("[batchLoop] emits=%d totalBytes=%d in %s", emits, bytes, time.Since(lastLog).Round(time.Second))
			emits, bytes, lastLog = 0, 0, time.Now()
		}
	})
}

// runBatchLoop sleeps until output is pending, waits one frame so the rest of
// the burst joins it, and hands the batch to emit.
//
// It used to tick every 16 ms whether or not anything was pending: 62 wakeups
// a second, a fresh map each time and a log line every five seconds, for an
// app that mostly sits with every agent idle. The first chunk after a quiet
// stretch still goes out within one frame, and under steady output the rate
// is the same one event per frame as before.
func runBatchLoop(ctx context.Context, b *outputBatcher, emit func(map[int][]byte)) {
	frame := time.NewTimer(outputFrame)
	frame.Stop()
	defer frame.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.wake:
		}
		frame.Reset(outputFrame)
		select {
		case <-ctx.Done():
			return
		case <-frame.C:
		}
		if batch := b.swap(); len(batch) > 0 {
			emit(batch)
		}
	}
}

// emitOutputBatch sends one frame's output to the WebView and returns how
// many raw bytes it carried.
func (a *AppService) emitOutputBatch(batch map[int][]byte) int {
	items := make([]TerminalOutputEvent, 0, len(batch))
	var n int
	hubID := a.hubID() // once per flush, not per session
	for id, raw := range batch {
		n += len(raw)
		items = append(items, TerminalOutputEvent{
			ID:   hub.Local(hubID, id),
			Data: base64.StdEncoding.EncodeToString(raw),
		})
	}
	a.app.Event.Emit("terminal:output-batch", items)
	return n
}
