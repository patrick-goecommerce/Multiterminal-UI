package backend

import (
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// ResyncSession repaints a pane from the backend's VT100 mirror.
//
// The frontend calls this when a pane's output backlog overflowed and had to be
// thrown away. It cannot simply resume with the bytes it still holds: a VT100
// stream with a hole in it truncates escape sequences and swallows whole runs of
// text, and because full-screen apps only repaint the regions they changed, the
// pane stays garbled for good (#157).
//
// The repaint is queued through the same output batcher as live PTY bytes, so it
// is strictly ordered against them: whatever the frontend still applies before
// it gets overwritten, and everything arriving after it continues from a screen
// both sides agree on.
func (a *AppService) ResyncSession(id int) {
	sess := a.session(id)
	if sess == nil {
		return
	}
	a.outputBatch().replaceWith(id, func() []byte {
		return []byte(screenRepaint(sess.Screen))
	})
}

// screenRepaint renders a screen as a self-contained repaint sequence.
// The rendering itself lives on Screen so the session daemon (internal/hub)
// can produce the identical bytes without importing the GUI backend.
func screenRepaint(scr *terminal.Screen) string {
	return scr.Repaint()
}
