package terminal

import "strings"

// ---------------------------------------------------------------------------
// Render – convert the screen buffer to an ANSI string for display
// ---------------------------------------------------------------------------

// Render produces a string representation of the entire screen buffer.
// The output contains ANSI escape sequences so colours and attributes are
// preserved when displayed inside the host terminal.
func (s *Screen) Render() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var b strings.Builder
	b.Grow(s.rows * (s.cols + 16)) // rough estimate

	prev := CellStyle{}
	for r := 0; r < s.rows; r++ {
		if r > 0 {
			b.WriteByte('\n')
			// Reset style at line boundaries for cleanliness
			b.WriteString("\x1b[0m")
			prev = CellStyle{}
		}
		for c := 0; c < s.cols; c++ {
			cell := s.cells[r][c]
			if cell.Style != prev {
				b.WriteString(sgrSequence(cell.Style))
				prev = cell.Style
			}
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(ch)
		}
	}
	// Final reset
	b.WriteString("\x1b[0m")
	return b.String()
}

// RenderRegion renders a sub-rectangle of the screen (0-indexed, inclusive).
func (s *Screen) RenderRegion(startRow, startCol, endRow, endCol int) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var b strings.Builder
	prev := CellStyle{}
	for r := startRow; r <= endRow && r < s.rows; r++ {
		if r > startRow {
			b.WriteByte('\n')
			b.WriteString("\x1b[0m")
			prev = CellStyle{}
		}
		for c := startCol; c <= endCol && c < s.cols; c++ {
			cell := s.cells[r][c]
			if cell.Style != prev {
				b.WriteString(sgrSequence(cell.Style))
				prev = cell.Style
			}
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(ch)
		}
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

// PlainTextRow returns the plain text content of a single row (no ANSI),
// with trailing spaces trimmed. Useful for pattern matching.
func (s *Screen) PlainTextRow(row int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row < 0 || row >= s.rows {
		return ""
	}
	var b strings.Builder
	for _, c := range s.cells[row] {
		ch := c.Char
		if ch == 0 {
			ch = ' '
		}
		b.WriteRune(ch)
	}
	return strings.TrimRight(b.String(), " ")
}

// PlainText returns the full screen content as plain text (no ANSI).
func (s *Screen) PlainText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for r := 0; r < s.rows; r++ {
		if r > 0 {
			b.WriteByte('\n')
		}
		for _, c := range s.cells[r] {
			ch := c.Char
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(ch)
		}
	}
	return b.String()
}
