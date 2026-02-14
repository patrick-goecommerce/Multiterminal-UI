package terminal

// ---------------------------------------------------------------------------
// Screen manipulation helpers
// ---------------------------------------------------------------------------

// putChar writes a character at the cursor position and advances the cursor.
func (s *Screen) putChar(ch rune) {
	if s.curCol >= s.cols {
		// Line wrap
		s.curCol = 0
		s.lineFeed()
	}
	if s.curRow >= 0 && s.curRow < s.rows && s.curCol >= 0 && s.curCol < s.cols {
		s.cells[s.curRow][s.curCol] = Cell{Char: ch, Style: s.style}
	}
	s.curCol++
}

// lineFeed moves the cursor down one line, scrolling if needed.
func (s *Screen) lineFeed() {
	bottom := s.scrollRegionBottom() - 1 // convert to 0-indexed
	if s.curRow == bottom {
		s.scrollUp()
	} else if s.curRow < s.rows-1 {
		s.curRow++
	}
}

// reverseLineFeed moves the cursor up one line, scrolling if needed.
func (s *Screen) reverseLineFeed() {
	top := s.scrollRegionTop() - 1 // convert to 0-indexed
	if s.curRow == top {
		s.scrollDown()
	} else if s.curRow > 0 {
		s.curRow--
	}
}

// scrollRegionTop returns the 1-indexed top of the scroll region.
func (s *Screen) scrollRegionTop() int {
	if s.scrollTop > 0 {
		return s.scrollTop
	}
	return 1
}

// scrollRegionBottom returns the 1-indexed bottom of the scroll region.
func (s *Screen) scrollRegionBottom() int {
	if s.scrollBottom > 0 {
		return s.scrollBottom
	}
	return s.rows
}

// scrollUp scrolls the scroll region up by one line (content moves up,
// new blank line appears at the bottom of the region).
func (s *Screen) scrollUp() {
	top := s.scrollRegionTop() - 1
	bottom := s.scrollRegionBottom() - 1
	if top >= bottom || top < 0 || bottom >= s.rows {
		return
	}
	// Shift rows up
	for r := top; r < bottom; r++ {
		s.cells[r] = s.cells[r+1]
	}
	// Blank the bottom row
	s.cells[bottom] = make([]Cell, s.cols)
	for c := range s.cells[bottom] {
		s.cells[bottom][c] = Cell{Char: ' '}
	}
}

// scrollDown scrolls the scroll region down by one line (content moves down,
// new blank line appears at the top of the region).
func (s *Screen) scrollDown() {
	top := s.scrollRegionTop() - 1
	bottom := s.scrollRegionBottom() - 1
	if top >= bottom || top < 0 || bottom >= s.rows {
		return
	}
	for r := bottom; r > top; r-- {
		s.cells[r] = s.cells[r-1]
	}
	s.cells[top] = make([]Cell, s.cols)
	for c := range s.cells[top] {
		s.cells[top][c] = Cell{Char: ' '}
	}
}

// eraseDisplay clears part of the screen.
//
//	0 = cursor to end, 1 = start to cursor, 2 = entire screen, 3 = entire + scrollback
func (s *Screen) eraseDisplay(mode int) {
	blank := Cell{Char: ' ', Style: s.style}
	switch mode {
	case 0: // cursor to end
		// Clear rest of current line
		for c := s.curCol; c < s.cols; c++ {
			s.cells[s.curRow][c] = blank
		}
		// Clear all lines below
		for r := s.curRow + 1; r < s.rows; r++ {
			for c := 0; c < s.cols; c++ {
				s.cells[r][c] = blank
			}
		}
	case 1: // start to cursor
		for r := 0; r < s.curRow; r++ {
			for c := 0; c < s.cols; c++ {
				s.cells[r][c] = blank
			}
		}
		for c := 0; c <= s.curCol && c < s.cols; c++ {
			s.cells[s.curRow][c] = blank
		}
	case 2, 3: // entire screen
		for r := 0; r < s.rows; r++ {
			for c := 0; c < s.cols; c++ {
				s.cells[r][c] = blank
			}
		}
		s.curRow = 0
		s.curCol = 0
	}
}

// eraseLine clears part of the current line.
//
//	0 = cursor to end, 1 = start to cursor, 2 = entire line
func (s *Screen) eraseLine(mode int) {
	blank := Cell{Char: ' ', Style: s.style}
	switch mode {
	case 0:
		for c := s.curCol; c < s.cols; c++ {
			s.cells[s.curRow][c] = blank
		}
	case 1:
		for c := 0; c <= s.curCol && c < s.cols; c++ {
			s.cells[s.curRow][c] = blank
		}
	case 2:
		for c := 0; c < s.cols; c++ {
			s.cells[s.curRow][c] = blank
		}
	}
}

// insertLines inserts n blank lines at the cursor row, pushing content down.
func (s *Screen) insertLines(n int) {
	bottom := s.scrollRegionBottom() - 1
	for i := 0; i < n; i++ {
		if s.curRow > bottom {
			break
		}
		// Shift rows down
		for r := bottom; r > s.curRow; r-- {
			s.cells[r] = s.cells[r-1]
		}
		s.cells[s.curRow] = make([]Cell, s.cols)
		for c := range s.cells[s.curRow] {
			s.cells[s.curRow][c] = Cell{Char: ' '}
		}
	}
}

// deleteLines deletes n lines at the cursor row, pulling content up.
func (s *Screen) deleteLines(n int) {
	bottom := s.scrollRegionBottom() - 1
	for i := 0; i < n; i++ {
		if s.curRow > bottom {
			break
		}
		for r := s.curRow; r < bottom; r++ {
			s.cells[r] = s.cells[r+1]
		}
		s.cells[bottom] = make([]Cell, s.cols)
		for c := range s.cells[bottom] {
			s.cells[bottom][c] = Cell{Char: ' '}
		}
	}
}

// deleteChars deletes n characters at the cursor, shifting the rest left.
func (s *Screen) deleteChars(n int) {
	row := s.cells[s.curRow]
	for i := s.curCol; i < s.cols; i++ {
		if i+n < s.cols {
			row[i] = row[i+n]
		} else {
			row[i] = Cell{Char: ' ', Style: s.style}
		}
	}
}

// insertChars inserts n blank characters at the cursor, shifting content right.
func (s *Screen) insertChars(n int) {
	row := s.cells[s.curRow]
	for i := s.cols - 1; i >= s.curCol+n; i-- {
		row[i] = row[i-n]
	}
	for i := s.curCol; i < s.curCol+n && i < s.cols; i++ {
		row[i] = Cell{Char: ' ', Style: s.style}
	}
}

// fullReset resets the terminal to its initial state.
func (s *Screen) fullReset() {
	s.style = CellStyle{}
	s.curRow = 0
	s.curCol = 0
	s.scrollTop = 0
	s.scrollBottom = 0
	s.Title = ""
	s.cells = makeGrid(s.rows, s.cols)
}

// clampCursor ensures the cursor is within screen bounds.
func (s *Screen) clampCursor() {
	if s.curRow < 0 {
		s.curRow = 0
	}
	if s.curRow >= s.rows {
		s.curRow = s.rows - 1
	}
	if s.curCol < 0 {
		s.curCol = 0
	}
	if s.curCol >= s.cols {
		s.curCol = s.cols - 1
	}
}
