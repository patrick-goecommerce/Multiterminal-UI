# Output Batcher — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace per-session `terminal:output` event spam with a central 16ms batcher that emits one `terminal:output-batch` event per frame regardless of session count, eliminating Win32 main-thread saturation.

**Architecture:** Each session's `collectOutput` goroutine reads raw bytes from `RawOutputCh` and hands them to a shared `outputBatcher`. A single `batchLoop` goroutine drains the batcher every 16ms and emits one `terminal:output-batch` Wails event containing all sessions' data. The frontend `TerminalPane` now listens to `terminal:output-batch` and filters by its own session ID.

**Root cause:** `Event.Emit` → `ExecJS` → `InvokeSync` blocks a goroutine on the Win32 main thread per emission. With 10 sessions × ~55 events/s = 550 `InvokeSync` calls/s the Win32 message queue is saturated, starving mouse/keyboard events (tab clicks, settings).

**Tech Stack:** Go (backend), TypeScript/Svelte (frontend), Wails v3 events

---

### Task 1: Backend — Add `outputBatcher` to `app_stream.go`

**Files:**
- Modify: `internal/backend/app_stream.go`

**Step 1: Replace the entire file with the new implementation**

The new file keeps `watchExit` and `coalesceDelay` (for scan interval reuse), removes `streamOutput`, and adds `outputBatcher` + `collectOutput` + `batchLoop`:

```go
package backend

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// coalesceDelay returns the scan tick delay — kept for scanLoop use.
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
// This is the key fix for Win32 main-thread saturation: instead of one
// ExecJS/InvokeSync call per session per coalesce window, we make exactly
// one call per frame regardless of how many sessions are active.
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
// It must be started as a goroutine in ServiceStartup.
func (a *AppService) batchLoop(ctx context.Context) {
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			batch := a.batcher.swap()
			if len(batch) == 0 {
				continue
			}
			items := make([]TerminalOutputEvent, 0, len(batch))
			for id, raw := range batch {
				items = append(items, TerminalOutputEvent{
					ID:   id,
					Data: base64.StdEncoding.EncodeToString(raw),
				})
			}
			a.app.Event.Emit("terminal:output-batch", items)
		}
	}
}

// collectOutput reads raw PTY bytes from the session's RawOutputCh and
// hands them to the shared outputBatcher. It drains all available bytes
// before yielding to reduce lock round-trips.
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
```

**Step 2: Verify it compiles**

```bash
go build ./internal/backend/...
```
Expected: no errors.

**Step 3: Commit**

```bash
git add internal/backend/app_stream.go
git commit -m "refactor(stream): replace per-session streamOutput with central outputBatcher"
```

---

### Task 2: Backend — Wire batcher into `AppService` struct and `app.go`

**Files:**
- Modify: `internal/backend/app.go`

**Step 1: Add `batcher` field to `AppService` struct**

Find the struct definition (around line 30-55) and add the field:

```go
batcher    *outputBatcher
```

**Step 2: Initialize batcher in `OnStartup` or constructor**

In `ServiceStartup` (after `scanCtx, cancel := context.WithCancel(ctx)`), add:

```go
a.batcher = newOutputBatcher()
go a.batchLoop(scanCtx)
```

**Step 3: Replace `streamOutput` call in `CreateSession`**

Find:
```go
go a.streamOutput(id, sess, a.serviceCtx)
```

Replace with:
```go
go a.collectOutput(id, sess, a.serviceCtx)
```

**Step 4: Verify compilation**

```bash
go build ./internal/backend/...
```
Expected: no errors.

**Step 5: Commit**

```bash
git add internal/backend/app.go
git commit -m "feat(stream): wire outputBatcher into AppService, start batchLoop"
```

---

### Task 3: Frontend — Update `TerminalPane.svelte` to handle batch event

**Files:**
- Modify: `frontend/src/components/TerminalPane.svelte`

**Step 1: Find the old event handler**

Search for the block starting with:
```ts
cleanupFn = EventsOn('terminal:output', (event: any) => {
```

**Step 2: Replace with batch handler**

The existing handler body stays almost identical — only the outer wrapper changes from receiving a single `{id, data}` to iterating an array:

Replace:
```ts
cleanupFn = EventsOn('terminal:output', (event: any) => {
  const id: number = event.data.id;
  const b64: string = event.data.data;
  if (id !== pane.sessionId || !termInstance) return;
  // ... (rest of body unchanged) ...
  pendingChunks.push(bytes);
  scheduleFlush();
});
```

With:
```ts
cleanupFn = EventsOn('terminal:output-batch', (event: any) => {
  const items: Array<{ id: number; data: string }> = event.data;
  if (!Array.isArray(items) || !termInstance) return;
  let gotData = false;
  for (const item of items) {
    if (item.id !== pane.sessionId) continue;
    const bytes = decodeBase64(item.data);

    // Scan for localhost URLs (keep existing logic)
    const mode = $config.localhost_auto_open;
    if (mode !== 'off') {
      const decoded = new TextDecoder().decode(bytes);
      LOCALHOST_REGEX.lastIndex = 0;
      let urlMatch;
      while ((urlMatch = LOCALHOST_REGEX.exec(decoded)) !== null) {
        const url = urlMatch[0];
        if (!seenLocalhostUrls.has(url)) {
          seenLocalhostUrls.add(url);
          if (mode === 'auto') {
            BrowserOpenURL(url);
            sendNotification('Dev Server', url + ' geöffnet');
          } else {
            sendNotification('Dev Server', url + ' erkannt');
          }
        }
      }
    }

    pendingChunks.push(bytes);
    gotData = true;
  }
  if (gotData) scheduleFlush();
});
```

> Note: A session only appears in a batch if it produced output in that 16ms window, so the loop body runs at most once per batch per pane in practice.

**Step 3: Build frontend**

```bash
cd frontend && npm run build
```
Expected: `✓ built in X.XXs` with no errors.

**Step 4: Commit**

```bash
git add frontend/src/components/TerminalPane.svelte
git commit -m "feat(frontend): switch terminal output to batched terminal:output-batch event"
```

---

### Task 4: Full build + smoke test

**Step 1: Build production binary**

```bash
cd /d/repos/Multiterminal
go build -tags desktop,production -ldflags "-H windowsgui" -o build/bin/mtui-portable.exe .
```
Expected: no errors.

**Step 2: Smoke test checklist**

Launch app and verify:
- [ ] Single terminal: output renders correctly (no corruption)
- [ ] Multiple terminals: all receive output
- [ ] Tab switching works while terminals are active
- [ ] Settings dialog opens while terminals are active
- [ ] 10 terminals open: UI stays responsive (tab clicks work immediately)
- [ ] Terminal with high output volume: other terminals still work

**Step 3: Commit + tag alpha**

```bash
git add .
git commit -m "fix: eliminate Win32 main-thread saturation via output batcher

Root cause: per-session Event.Emit → ExecJS → InvokeSync blocked the
Win32 main thread. With 10 sessions × ~55 events/s = 550 InvokeSync
calls/s, the Win32 message queue was saturated, starving user input.

Fix: central outputBatcher collects all sessions' PTY bytes over a 16ms
window and emits exactly one terminal:output-batch event per frame,
reducing InvokeSync calls from O(sessions × frequency) to O(1/frame).

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Performance Impact

| Metric | Before | After |
|---|---|---|
| `InvokeSync` calls/s (10 sessions) | ~550 | ~62 |
| Goroutines spawned/s by Emit | ~1100 | ~124 |
| Win32 main thread load | ~100% | ~11% |
| Latency per chunk (worst case) | 18ms | 16ms |
| User input responsiveness | ❌ frozen | ✅ instant |
