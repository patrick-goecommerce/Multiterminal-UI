// Package terminal provides VT100 terminal emulation and PTY session management.
//
// The Screen type maintains a virtual terminal screen buffer that processes
// raw byte output (including ANSI escape sequences) and stores the resulting
// character grid. This allows embedding real terminal content inside a
// Bubbletea TUI.
package terminal

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Cell – a single character cell on the screen
// ---------------------------------------------------------------------------

// CellStyle holds the visual attributes of a single cell.
type CellStyle struct {
	FG        int  // foreground colour (0 = default, 1-8 standard, 9-16 bright, 17-256 palette, 257+ truecolor encoded)
	BG        int  // background colour (same encoding)
	Bold      bool // SGR 1
	Dim       bool // SGR 2
	Italic    bool // SGR 3
	Underline bool // SGR 4
	Reverse   bool // SGR 7
	Strike    bool // SGR 9
}

// Cell represents one character position on the terminal screen.
type Cell struct {
	Char  rune
	Style CellStyle
}

// ---------------------------------------------------------------------------
// Screen – VT100 virtual terminal
// ---------------------------------------------------------------------------

// parserState tracks the ANSI escape sequence parser automaton.
type parserState int

const (
	stateNormal parserState = iota
	stateESC                // received ESC (\x1b)
	stateCSI                // received ESC [
	stateOSC                // received ESC ]
)

// Screen is a VT100-compatible virtual terminal screen buffer.
// It accepts raw byte output via Write and maintains a grid of Cells
// that can be read back for rendering.
//
// Thread-safety: all public methods acquire an internal mutex so the
// screen can safely be written to from a PTY reader goroutine while
// the Bubbletea render loop reads cells.
type Screen struct {
	mu sync.Mutex

	rows, cols int
	cells      [][]Cell
	curRow     int
	curCol     int
	style      CellStyle // current drawing style (set by SGR)

	// Parser state
	state    parserState
	csiBuf   []byte // collects CSI parameter bytes
	oscBuf   []byte // collects OSC payload
	savedRow int    // DEC save cursor
	savedCol int

	// Scroll region (1-indexed, inclusive). Zero means "use full screen".
	scrollTop    int
	scrollBottom int

	// Title reported by OSC sequences (e.g. xterm window title).
	Title string
}

// NewScreen allocates a Screen of the given dimensions.
func NewScreen(rows, cols int) *Screen {
	s := &Screen{rows: rows, cols: cols}
	s.cells = makeGrid(rows, cols)
	return s
}

// makeGrid allocates a rows×cols grid of blank cells.
func makeGrid(rows, cols int) [][]Cell {
	g := make([][]Cell, rows)
	for r := range g {
		g[r] = make([]Cell, cols)
		for c := range g[r] {
			g[r][c] = Cell{Char: ' '}
		}
	}
	return g
}

// Resize changes the screen dimensions, preserving content where possible.
func (s *Screen) Resize(rows, cols int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ng := makeGrid(rows, cols)
	// Copy existing content
	for r := 0; r < rows && r < s.rows; r++ {
		for c := 0; c < cols && c < s.cols; c++ {
			ng[r][c] = s.cells[r][c]
		}
	}
	s.cells = ng
	s.rows = rows
	s.cols = cols
	// Clamp cursor
	if s.curRow >= rows {
		s.curRow = rows - 1
	}
	if s.curCol >= cols {
		s.curCol = cols - 1
	}
}

// Rows returns the current row count.
func (s *Screen) Rows() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rows
}

// Cols returns the current column count.
func (s *Screen) Cols() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cols
}

// CellAt returns the cell at (row, col). Out-of-bounds returns a blank cell.
func (s *Screen) CellAt(row, col int) Cell {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row < 0 || row >= s.rows || col < 0 || col >= s.cols {
		return Cell{Char: ' '}
	}
	return s.cells[row][col]
}

// Cursor returns the current cursor position (row, col).
func (s *Screen) Cursor() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.curRow, s.curCol
}

// ---------------------------------------------------------------------------
// Write – process raw terminal output bytes
// ---------------------------------------------------------------------------

// Write processes raw bytes from a PTY. It interprets control characters
// and ANSI escape sequences to update the screen buffer.
// Implements io.Writer.
func (s *Screen) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, b := range p {
		s.processByte(b)
	}
	return len(p), nil
}

// processByte feeds one byte into the parser state machine.
func (s *Screen) processByte(b byte) {
	switch s.state {
	case stateNormal:
		s.processNormal(b)
	case stateESC:
		s.processESC(b)
	case stateCSI:
		s.processCSI(b)
	case stateOSC:
		s.processOSC(b)
	}
}

// processNormal handles bytes outside any escape sequence.
func (s *Screen) processNormal(b byte) {
	switch b {
	case 0x1b: // ESC
		s.state = stateESC
	case '\n': // Line Feed
		s.lineFeed()
	case '\r': // Carriage Return
		s.curCol = 0
	case '\b': // Backspace
		if s.curCol > 0 {
			s.curCol--
		}
	case '\t': // Horizontal Tab
		s.curCol = (s.curCol/8 + 1) * 8
		if s.curCol >= s.cols {
			s.curCol = s.cols - 1
		}
	case 0x07: // BEL – ignore
	default:
		if b >= 0x20 { // printable ASCII
			s.putChar(rune(b))
		}
		// Ignore other C0 controls
	}
}

// processESC handles the byte immediately after ESC.
func (s *Screen) processESC(b byte) {
	switch b {
	case '[': // CSI introducer
		s.state = stateCSI
		s.csiBuf = s.csiBuf[:0]
	case ']': // OSC introducer
		s.state = stateOSC
		s.oscBuf = s.oscBuf[:0]
	case '7': // DEC Save Cursor
		s.savedRow = s.curRow
		s.savedCol = s.curCol
		s.state = stateNormal
	case '8': // DEC Restore Cursor
		s.curRow = s.savedRow
		s.curCol = s.savedCol
		s.state = stateNormal
	case 'D': // Index (move down, scroll if at bottom)
		s.lineFeed()
		s.state = stateNormal
	case 'M': // Reverse Index (move up, scroll if at top)
		s.reverseLineFeed()
		s.state = stateNormal
	case 'c': // Full Reset (RIS)
		s.fullReset()
		s.state = stateNormal
	default:
		// Unknown ESC sequence – return to normal
		s.state = stateNormal
	}
}

// processCSI collects CSI parameter bytes and dispatches the final byte.
func (s *Screen) processCSI(b byte) {
	if b >= 0x30 && b <= 0x3F {
		// Parameter byte (digits, semicolons, question mark, etc.)
		s.csiBuf = append(s.csiBuf, b)
		return
	}
	if b >= 0x20 && b <= 0x2F {
		// Intermediate byte – store and wait for final
		s.csiBuf = append(s.csiBuf, b)
		return
	}
	// Final byte 0x40-0x7E → dispatch
	s.dispatchCSI(b)
	s.state = stateNormal
}

// processOSC collects the OSC payload until BEL or ST.
func (s *Screen) processOSC(b byte) {
	if b == 0x07 { // BEL terminates OSC
		s.handleOSC()
		s.state = stateNormal
		return
	}
	if b == 0x1b {
		// Possible ST (ESC \)
		// For simplicity, just terminate the OSC here
		s.handleOSC()
		s.state = stateNormal
		return
	}
	s.oscBuf = append(s.oscBuf, b)
}

// handleOSC processes the completed OSC payload.
func (s *Screen) handleOSC() {
	payload := string(s.oscBuf)
	// OSC 0 ; <title> – set window title
	// OSC 2 ; <title> – set window title
	if strings.HasPrefix(payload, "0;") || strings.HasPrefix(payload, "2;") {
		s.Title = payload[2:]
	}
}

// ---------------------------------------------------------------------------
// CSI command dispatch
// ---------------------------------------------------------------------------

// dispatchCSI executes a CSI sequence given the final command byte.
func (s *Screen) dispatchCSI(cmd byte) {
	params := s.parseCSIParams()

	switch cmd {
	case 'A': // Cursor Up
		n := paramDefault(params, 0, 1)
		s.curRow -= n
		if s.curRow < 0 {
			s.curRow = 0
		}
	case 'B': // Cursor Down
		n := paramDefault(params, 0, 1)
		s.curRow += n
		if s.curRow >= s.rows {
			s.curRow = s.rows - 1
		}
	case 'C': // Cursor Forward
		n := paramDefault(params, 0, 1)
		s.curCol += n
		if s.curCol >= s.cols {
			s.curCol = s.cols - 1
		}
	case 'D': // Cursor Backward
		n := paramDefault(params, 0, 1)
		s.curCol -= n
		if s.curCol < 0 {
			s.curCol = 0
		}
	case 'E': // Cursor Next Line
		n := paramDefault(params, 0, 1)
		s.curRow += n
		if s.curRow >= s.rows {
			s.curRow = s.rows - 1
		}
		s.curCol = 0
	case 'F': // Cursor Previous Line
		n := paramDefault(params, 0, 1)
		s.curRow -= n
		if s.curRow < 0 {
			s.curRow = 0
		}
		s.curCol = 0
	case 'G': // Cursor Horizontal Absolute
		n := paramDefault(params, 0, 1)
		s.curCol = n - 1
		s.clampCursor()
	case 'H', 'f': // Cursor Position
		row := paramDefault(params, 0, 1)
		col := paramDefault(params, 1, 1)
		s.curRow = row - 1
		s.curCol = col - 1
		s.clampCursor()
	case 'J': // Erase in Display
		s.eraseDisplay(paramDefault(params, 0, 0))
	case 'K': // Erase in Line
		s.eraseLine(paramDefault(params, 0, 0))
	case 'L': // Insert Lines
		n := paramDefault(params, 0, 1)
		s.insertLines(n)
	case 'M': // Delete Lines
		n := paramDefault(params, 0, 1)
		s.deleteLines(n)
	case 'P': // Delete Characters
		n := paramDefault(params, 0, 1)
		s.deleteChars(n)
	case '@': // Insert Characters
		n := paramDefault(params, 0, 1)
		s.insertChars(n)
	case 'S': // Scroll Up
		n := paramDefault(params, 0, 1)
		for i := 0; i < n; i++ {
			s.scrollUp()
		}
	case 'T': // Scroll Down
		n := paramDefault(params, 0, 1)
		for i := 0; i < n; i++ {
			s.scrollDown()
		}
	case 'm': // SGR – Select Graphic Rendition
		s.handleSGR(params)
	case 'r': // DECSTBM – Set Scrolling Region
		top := paramDefault(params, 0, 1)
		bottom := paramDefault(params, 1, s.rows)
		s.scrollTop = top
		s.scrollBottom = bottom
	case 's': // Save Cursor Position
		s.savedRow = s.curRow
		s.savedCol = s.curCol
	case 'u': // Restore Cursor Position
		s.curRow = s.savedRow
		s.curCol = s.savedCol
	case 'h', 'l': // Set/Reset Mode – largely ignored
		// We handle CSI ? 25 h/l (show/hide cursor) by ignoring it
		// since our rendering always shows the cursor.
	case 'X': // Erase Characters
		n := paramDefault(params, 0, 1)
		for i := 0; i < n && s.curCol+i < s.cols; i++ {
			s.cells[s.curRow][s.curCol+i] = Cell{Char: ' ', Style: s.style}
		}
	case 'd': // Vertical Position Absolute
		n := paramDefault(params, 0, 1)
		s.curRow = n - 1
		s.clampCursor()
	}
}

// parseCSIParams splits the CSI parameter buffer into integer parameters.
// ";" separates values; missing values default to 0.
func (s *Screen) parseCSIParams() []int {
	// Strip leading '?' or '>' (private mode prefix)
	raw := string(s.csiBuf)
	raw = strings.TrimLeft(raw, "?>=!")

	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	params := make([]int, len(parts))
	for i, p := range parts {
		v, _ := strconv.Atoi(p)
		params[i] = v
	}
	return params
}

// paramDefault returns params[idx] if it exists and is > 0, otherwise def.
func paramDefault(params []int, idx, def int) int {
	if idx < len(params) && params[idx] > 0 {
		return params[idx]
	}
	return def
}

// ---------------------------------------------------------------------------
// SGR (Select Graphic Rendition) handler
// ---------------------------------------------------------------------------

// handleSGR processes a CSI m sequence to update the current drawing style.
func (s *Screen) handleSGR(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	i := 0
	for i < len(params) {
		p := params[i]
		switch {
		case p == 0: // Reset
			s.style = CellStyle{}
		case p == 1:
			s.style.Bold = true
		case p == 2:
			s.style.Dim = true
		case p == 3:
			s.style.Italic = true
		case p == 4:
			s.style.Underline = true
		case p == 7:
			s.style.Reverse = true
		case p == 9:
			s.style.Strike = true
		case p == 22:
			s.style.Bold = false
			s.style.Dim = false
		case p == 23:
			s.style.Italic = false
		case p == 24:
			s.style.Underline = false
		case p == 27:
			s.style.Reverse = false
		case p == 29:
			s.style.Strike = false
		case p >= 30 && p <= 37: // Standard FG
			s.style.FG = p - 30 + 1
		case p == 38: // Extended FG
			i = s.parseSGRColor(params, i, true)
		case p == 39: // Default FG
			s.style.FG = 0
		case p >= 40 && p <= 47: // Standard BG
			s.style.BG = p - 40 + 1
		case p == 48: // Extended BG
			i = s.parseSGRColor(params, i, false)
		case p == 49: // Default BG
			s.style.BG = 0
		case p >= 90 && p <= 97: // Bright FG
			s.style.FG = p - 90 + 9
		case p >= 100 && p <= 107: // Bright BG
			s.style.BG = p - 100 + 9
		}
		i++
	}
}

// parseSGRColor handles "38;5;N" (256-colour) and "38;2;R;G;B" (truecolour)
// sequences for foreground (fg=true) or background (fg=false).
// Returns the updated index into params.
func (s *Screen) parseSGRColor(params []int, i int, fg bool) int {
	if i+1 >= len(params) {
		return i
	}
	mode := params[i+1]
	switch mode {
	case 5: // 256-colour
		if i+2 < len(params) {
			c := params[i+2] + 1 // +1 so 0 means "default"
			if fg {
				s.style.FG = c
			} else {
				s.style.BG = c
			}
			return i + 2
		}
	case 2: // Truecolour RGB
		if i+4 < len(params) {
			r, g, b := params[i+2], params[i+3], params[i+4]
			// Encode as 0x01RRGGBB + 256 to distinguish from palette
			c := 256 + 1 + r<<16 + g<<8 + b
			if fg {
				s.style.FG = c
			} else {
				s.style.BG = c
			}
			return i + 4
		}
	}
	return i + 1
}

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

// sgrSequence produces the SGR escape sequence to switch to the given style.
func sgrSequence(st CellStyle) string {
	var parts []string
	parts = append(parts, "0") // always reset first

	if st.Bold {
		parts = append(parts, "1")
	}
	if st.Dim {
		parts = append(parts, "2")
	}
	if st.Italic {
		parts = append(parts, "3")
	}
	if st.Underline {
		parts = append(parts, "4")
	}
	if st.Reverse {
		parts = append(parts, "7")
	}
	if st.Strike {
		parts = append(parts, "9")
	}
	// Foreground
	if st.FG > 0 && st.FG <= 8 {
		parts = append(parts, fmt.Sprintf("%d", 29+st.FG))
	} else if st.FG >= 9 && st.FG <= 16 {
		parts = append(parts, fmt.Sprintf("%d", 81+st.FG))
	} else if st.FG > 16 && st.FG <= 256 {
		parts = append(parts, fmt.Sprintf("38;5;%d", st.FG-1))
	} else if st.FG > 256 {
		rgb := st.FG - 257
		parts = append(parts, fmt.Sprintf("38;2;%d;%d;%d", (rgb>>16)&0xFF, (rgb>>8)&0xFF, rgb&0xFF))
	}
	// Background
	if st.BG > 0 && st.BG <= 8 {
		parts = append(parts, fmt.Sprintf("%d", 39+st.BG))
	} else if st.BG >= 9 && st.BG <= 16 {
		parts = append(parts, fmt.Sprintf("%d", 91+st.BG))
	} else if st.BG > 16 && st.BG <= 256 {
		parts = append(parts, fmt.Sprintf("48;5;%d", st.BG-1))
	} else if st.BG > 256 {
		rgb := st.BG - 257
		parts = append(parts, fmt.Sprintf("48;2;%d;%d;%d", (rgb>>16)&0xFF, (rgb>>8)&0xFF, rgb&0xFF))
	}

	return "\x1b[" + strings.Join(parts, ";") + "m"
}
