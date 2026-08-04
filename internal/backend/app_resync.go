package backend

import (
	"fmt"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// resyncPrefix drops any pending SGR state and clears the screen so the repaint
// that follows cannot inherit colours or leftover glyphs from what was there.
const resyncPrefix = "\x1b[0m\x1b[2J"

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
	a.mu.Lock()
	sess := a.sessions[id]
	a.mu.Unlock()
	if sess == nil {
		return
	}
	a.outputBatch().replaceWith(id, func() []byte {
		return []byte(screenRepaint(sess.Screen))
	})
}

// screenRepaint renders a screen as a self-contained repaint sequence.
// Rows are positioned absolutely rather than separated by newlines: a row filled
// to the right margin would otherwise wrap and push every following row down by
// one. The cursor is restored last so the app's input line lands where it was.
func screenRepaint(scr *terminal.Screen) string {
	rows, cols := scr.Rows(), scr.Cols()

	var b strings.Builder
	b.Grow(len(resyncPrefix) + rows*(cols+24))
	b.WriteString(resyncPrefix)
	for r := 0; r < rows; r++ {
		fmt.Fprintf(&b, "\x1b[%d;1H", r+1)
		b.WriteString(scr.RenderRegion(r, 0, r, cols-1))
	}
	curRow, curCol := scr.Cursor()
	fmt.Fprintf(&b, "\x1b[%d;%dH", curRow+1, curCol+1)
	return b.String()
}
