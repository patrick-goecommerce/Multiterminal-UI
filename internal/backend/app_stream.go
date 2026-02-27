package backend

import (
	"context"
	"encoding/base64"
	"log"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// coalesceDelay returns the scan tick delay — kept for scanLoop reuse.
func (a *AppService) coalesceDelay() time.Duration {
	a.mu.Lock()
	n := len(a.sessions)
	a.mu.Unlock()
	switch {
	case n <= 2:
		return 6 * time.Millisecond
	case n <= 4:
		return 10 * time.Millisecond
	case n <= 6:
		return 14 * time.Millisecond
	default:
		return 18 * time.Millisecond
	}
}

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

// add appends raw bytes for a session into the accumulation buffer.
func (b *outputBatcher) add(id int, data []byte) {
	b.mu.Lock()
	b.pending[id] = append(b.pending[id], data...)
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
			batch := a.batcher.swap()
			if len(batch) == 0 {
				continue
			}
			items := make([]TerminalOutputEvent, 0, len(batch))
			var batchBytes int
			for id, raw := range batch {
				batchBytes += len(raw)
				items = append(items, TerminalOutputEvent{
					ID:   id,
					Data: base64.StdEncoding.EncodeToString(raw),
				})
			}
			emitCount++
			totalBytes += int64(batchBytes)
			a.app.Event.Emit("terminal:output-batch", items)
		}
	}
}

// collectOutput reads raw PTY bytes from the session's RawOutputCh and
// hands them to the shared outputBatcher. It drains all currently
// available bytes before yielding to reduce lock round-trips.
func (a *AppService) collectOutput(id int, sess *terminal.Session, ctx context.Context) {
	for {
		select {
		case data, ok := <-sess.RawOutputCh:
			if !ok {
				return
			}
			buf := append([]byte(nil), data...)
			// Non-blocking drain: collect everything already in the buffer.
		drain:
			for {
				select {
				case more, ok := <-sess.RawOutputCh:
					if !ok {
						a.batcher.add(id, buf)
						return
					}
					buf = append(buf, more...)
				case <-ctx.Done():
					a.batcher.add(id, buf)
					return
				default:
					break drain
				}
			}
			a.batcher.add(id, buf)
		case <-ctx.Done():
			return
		}
	}
}

// watchExit waits for a session to exit and notifies the frontend.
func (a *AppService) watchExit(id int, sess *terminal.Session) {
	<-sess.Done()
	a.app.Event.Emit("terminal:exit", TerminalExitEvent{ID: id, ExitCode: sess.ExitCode})
}
