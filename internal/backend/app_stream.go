package backend

import (
	"encoding/base64"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// coalesceDelay returns the output coalescing delay based on the number
// of active sessions. More sessions → longer delay to reduce event load.
func (a *App) coalesceDelay() time.Duration {
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

// streamOutput reads raw PTY bytes from the session and emits them as
// base64-encoded chunks to the frontend via Wails events.
// It coalesces rapid output over a short time window so that TUI redraws
// (which produce many small chunks) arrive as a single event, preventing
// cursor flicker in xterm.js.
func (a *App) streamOutput(id int, sess *terminal.Session) {
	for {
		select {
		case data, ok := <-sess.RawOutputCh:
			if !ok {
				return
			}
			buf := append([]byte(nil), data...)
			// Wait briefly for more chunks — TUI apps redraw in bursts
			deadline := time.After(a.coalesceDelay())
		collect:
			for {
				select {
				case more, ok := <-sess.RawOutputCh:
					if !ok {
						b64 := base64.StdEncoding.EncodeToString(buf)
						runtime.EventsEmit(a.ctx, "terminal:output", id, b64)
						return
					}
					buf = append(buf, more...)
				case <-deadline:
					break collect
				case <-a.ctx.Done():
					return
				}
			}
			b64 := base64.StdEncoding.EncodeToString(buf)
			runtime.EventsEmit(a.ctx, "terminal:output", id, b64)
		case <-a.ctx.Done():
			return
		}
	}
}

// watchExit waits for a session to exit and notifies the frontend.
func (a *App) watchExit(id int, sess *terminal.Session) {
	<-sess.Done()
	runtime.EventsEmit(a.ctx, "terminal:exit", id, sess.ExitCode)
}
