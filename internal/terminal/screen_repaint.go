package terminal

import (
	"fmt"
	"strings"
)

// repaintPrefix drops any pending SGR state and clears the screen so the
// repaint that follows cannot inherit colours or leftover glyphs from what was
// there.
const repaintPrefix = "\x1b[0m\x1b[2J"

// Repaint renders the screen as a self-contained repaint sequence.
//
// It is what a viewer applies when its byte stream has a hole: a VT100 stream
// missing a run of bytes truncates escape sequences and swallows whole runs of
// text, and because full-screen apps only repaint the regions they changed,
// the pane stays garbled for good (#157). Sending the mirror instead gives
// both sides a screen they agree on.
//
// Rows are positioned absolutely rather than separated by newlines: a row
// filled to the right margin would otherwise wrap and push every following row
// down by one. The cursor is restored last so the app's input line lands where
// it was.
func (s *Screen) Repaint() string {
	rows, cols := s.Rows(), s.Cols()

	var b strings.Builder
	b.Grow(len(repaintPrefix) + rows*(cols+24))
	b.WriteString(repaintPrefix)
	for r := 0; r < rows; r++ {
		fmt.Fprintf(&b, "\x1b[%d;1H", r+1)
		b.WriteString(s.RenderRegion(r, 0, r, cols-1))
	}
	curRow, curCol := s.Cursor()
	fmt.Fprintf(&b, "\x1b[%d;%dH", curRow+1, curCol+1)
	return b.String()
}
