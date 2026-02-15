// Package terminal provides VT100 terminal emulation and PTY session management.
//
// The Screen type maintains a virtual terminal screen buffer that processes
// raw byte output (including ANSI escape sequences) and stores the resulting
// character grid. This allows embedding real terminal content inside a
// Bubbletea TUI.
package terminal

import (
	"strings"
	"sync"
	"unicode/utf8"
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

	// UTF-8 multi-byte decoder state
	utf8Buf [4]byte // buffered UTF-8 bytes
	utf8Len int     // total bytes expected (2, 3, or 4); 0 = not in sequence
	utf8Got int     // bytes collected so far
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
	// If collecting a multi-byte UTF-8 sequence, handle continuation bytes.
	if s.utf8Len > 0 {
		if b >= 0x80 && b <= 0xBF { // valid continuation byte
			s.utf8Buf[s.utf8Got] = b
			s.utf8Got++
			if s.utf8Got == s.utf8Len {
				r, _ := utf8.DecodeRune(s.utf8Buf[:s.utf8Len])
				s.utf8Len = 0
				s.utf8Got = 0
				if r != utf8.RuneError {
					s.putChar(r)
				}
			}
			return
		}
		// Invalid continuation — discard partial sequence, process b normally.
		s.utf8Len = 0
		s.utf8Got = 0
	}

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
		if b >= 0x20 && b <= 0x7E { // printable ASCII
			s.putChar(rune(b))
		} else if b >= 0xC0 && b <= 0xF7 { // UTF-8 lead byte
			s.utf8Buf[0] = b
			s.utf8Got = 1
			switch {
			case b < 0xE0:
				s.utf8Len = 2
			case b < 0xF0:
				s.utf8Len = 3
			default:
				s.utf8Len = 4
			}
		}
		// Ignore C0 controls and invalid bytes (0x80-0xBF outside sequence)
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
