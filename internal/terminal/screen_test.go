package terminal

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewScreen
// ---------------------------------------------------------------------------

func TestNewScreen_Dimensions(t *testing.T) {
	s := NewScreen(24, 80)
	if s.Rows() != 24 {
		t.Errorf("Rows() = %d, want 24", s.Rows())
	}
	if s.Cols() != 80 {
		t.Errorf("Cols() = %d, want 80", s.Cols())
	}
}

func TestNewScreen_BlankCells(t *testing.T) {
	s := NewScreen(3, 4)
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			cell := s.CellAt(r, c)
			if cell.Char != ' ' {
				t.Errorf("CellAt(%d,%d).Char = %q, want ' '", r, c, cell.Char)
			}
		}
	}
}

func TestNewScreen_CursorAtOrigin(t *testing.T) {
	s := NewScreen(24, 80)
	row, col := s.Cursor()
	if row != 0 || col != 0 {
		t.Errorf("Cursor() = (%d,%d), want (0,0)", row, col)
	}
}

// ---------------------------------------------------------------------------
// CellAt out of bounds
// ---------------------------------------------------------------------------

func TestCellAt_OutOfBounds(t *testing.T) {
	s := NewScreen(3, 3)
	cell := s.CellAt(-1, 0)
	if cell.Char != ' ' {
		t.Errorf("CellAt(-1,0).Char = %q, want ' '", cell.Char)
	}
	cell = s.CellAt(99, 0)
	if cell.Char != ' ' {
		t.Errorf("CellAt(99,0).Char = %q, want ' '", cell.Char)
	}
	cell = s.CellAt(0, 99)
	if cell.Char != ' ' {
		t.Errorf("CellAt(0,99).Char = %q, want ' '", cell.Char)
	}
}

// ---------------------------------------------------------------------------
// Write – basic text
// ---------------------------------------------------------------------------

func TestWrite_SimpleText(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("Hello"))

	row, col := s.Cursor()
	if row != 0 || col != 5 {
		t.Errorf("Cursor after 'Hello' = (%d,%d), want (0,5)", row, col)
	}

	// Verify each character
	want := "Hello"
	for i, ch := range want {
		got := s.CellAt(0, i).Char
		if got != ch {
			t.Errorf("CellAt(0,%d) = %q, want %q", i, got, ch)
		}
	}
}

func TestWrite_LineFeed(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("Line1\nLine2"))

	row, col := s.Cursor()
	if row != 1 {
		t.Errorf("Cursor row = %d, want 1", row)
	}
	// After \n the column stays where it was (no CR),
	// so "Line2" overwrites starting at col 5
	_ = col // column depends on whether \n includes implicit CR — check content

	// Row 0 should start with "Line1"
	r0 := s.PlainTextRow(0)
	if !strings.HasPrefix(r0, "Line1") {
		t.Errorf("Row 0 = %q, want prefix 'Line1'", r0)
	}
}

func TestWrite_CarriageReturn(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("AAAA\rBB"))

	// \r moves cursor to col 0, then BB overwrites first 2 chars
	r0 := s.PlainTextRow(0)
	if !strings.HasPrefix(r0, "BBAA") {
		t.Errorf("Row 0 = %q, want prefix 'BBAA'", r0)
	}
}

func TestWrite_CRLF(t *testing.T) {
	s := NewScreen(5, 20)
	s.Write([]byte("Line1\r\nLine2"))

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	if !strings.HasPrefix(r0, "Line1") {
		t.Errorf("Row 0 = %q, want 'Line1'", r0)
	}
	if !strings.HasPrefix(r1, "Line2") {
		t.Errorf("Row 1 = %q, want 'Line2'", r1)
	}
}

// ---------------------------------------------------------------------------
// Write – control characters
// ---------------------------------------------------------------------------

func TestWrite_Backspace(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABC\bX"))

	// \b moves cursor back one, then X overwrites C
	r0 := s.PlainTextRow(0)
	if !strings.HasPrefix(r0, "ABX") {
		t.Errorf("Row 0 after backspace = %q, want prefix 'ABX'", r0)
	}
}

func TestWrite_BackspaceAtCol0(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\b"))
	row, col := s.Cursor()
	if col != 0 {
		t.Errorf("Backspace at col 0: cursor = (%d,%d), want col 0", row, col)
	}
}

func TestWrite_Tab(t *testing.T) {
	s := NewScreen(3, 80)
	s.Write([]byte("A\tB"))

	// Tab should advance to next multiple of 8
	row, col := s.Cursor()
	if row != 0 || col != 9 {
		t.Errorf("Cursor after 'A\\tB' = (%d,%d), want (0,9)", row, col)
	}
}

func TestWrite_TabClampsToEdge(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("12345678\t"))
	_, col := s.Cursor()
	// Tab from col 8 should go to 16, clamped to cols-1 = 9
	if col != 9 {
		t.Errorf("Tab at edge: col = %d, want 9", col)
	}
}

// ---------------------------------------------------------------------------
// Write – line wrapping
// ---------------------------------------------------------------------------

func TestWrite_LineWrap(t *testing.T) {
	s := NewScreen(5, 5)
	s.Write([]byte("ABCDEFGH"))

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	if r0 != "ABCDE" {
		t.Errorf("Row 0 = %q, want 'ABCDE'", r0)
	}
	if !strings.HasPrefix(r1, "FGH") {
		t.Errorf("Row 1 = %q, want prefix 'FGH'", r1)
	}
}

// ---------------------------------------------------------------------------
// Write – scrolling at bottom
// ---------------------------------------------------------------------------

func TestWrite_ScrollsAtBottom(t *testing.T) {
	s := NewScreen(3, 5)
	// Use \r\n to reset column before each new line
	s.Write([]byte("AAA\r\nBBB\r\nCCC\r\nDDD"))

	// 3-row screen: after 4 lines, first line should have scrolled off
	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	r2 := s.PlainTextRow(2)
	if !strings.HasPrefix(r0, "BBB") {
		t.Errorf("After scroll, row 0 = %q, want prefix 'BBB'", r0)
	}
	if !strings.HasPrefix(r1, "CCC") {
		t.Errorf("After scroll, row 1 = %q, want prefix 'CCC'", r1)
	}
	if !strings.HasPrefix(r2, "DDD") {
		t.Errorf("After scroll, row 2 = %q, want prefix 'DDD'", r2)
	}
}

// ---------------------------------------------------------------------------
// Resize
// ---------------------------------------------------------------------------

func TestResize_Grow(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("Hi"))
	s.Resize(5, 10)

	if s.Rows() != 5 || s.Cols() != 10 {
		t.Errorf("After Resize: %dx%d, want 5x10", s.Rows(), s.Cols())
	}
	// Old content preserved
	if ch := s.CellAt(0, 0).Char; ch != 'H' {
		t.Errorf("CellAt(0,0) = %q, want 'H'", ch)
	}
	if ch := s.CellAt(0, 1).Char; ch != 'i' {
		t.Errorf("CellAt(0,1) = %q, want 'i'", ch)
	}
}

func TestResize_Shrink(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("Hello World!"))
	s.Resize(2, 5)

	if s.Rows() != 2 || s.Cols() != 5 {
		t.Errorf("After Resize: %dx%d, want 2x5", s.Rows(), s.Cols())
	}
	// Content within new bounds is preserved
	r0 := s.PlainTextRow(0)
	if r0 != "Hello" {
		t.Errorf("Row 0 after shrink = %q, want 'Hello'", r0)
	}
}

func TestResize_ClampsCursor(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[9;9H")) // move cursor to row 9, col 9 (1-indexed)
	s.Resize(5, 5)

	row, col := s.Cursor()
	if row >= 5 || col >= 5 {
		t.Errorf("After shrink resize, cursor = (%d,%d), should be < (5,5)", row, col)
	}
}

// ---------------------------------------------------------------------------
// ESC sequences
// ---------------------------------------------------------------------------

func TestESC_SaveRestoreCursor(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("ABC"))        // cursor at (0,3)
	s.Write([]byte("\x1b7"))      // save cursor
	s.Write([]byte("\x1b[5;5H"))  // move to (4,4)
	s.Write([]byte("\x1b8"))      // restore cursor

	row, col := s.Cursor()
	if row != 0 || col != 3 {
		t.Errorf("After save/restore, cursor = (%d,%d), want (0,3)", row, col)
	}
}

func TestESC_FullReset(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("Hello"))
	s.Write([]byte("\x1bc")) // RIS – full reset

	row, col := s.Cursor()
	if row != 0 || col != 0 {
		t.Errorf("After RIS, cursor = (%d,%d), want (0,0)", row, col)
	}
	r0 := s.PlainTextRow(0)
	if r0 != "" {
		t.Errorf("After RIS, row 0 = %q, want empty", r0)
	}
}

// ---------------------------------------------------------------------------
// OSC sequences
// ---------------------------------------------------------------------------

func TestOSC_SetTitle(t *testing.T) {
	s := NewScreen(5, 80)
	// OSC 0 ; My Title BEL
	s.Write([]byte("\x1b]0;My Title\x07"))

	s.mu.Lock()
	title := s.Title
	s.mu.Unlock()
	if title != "My Title" {
		t.Errorf("Title = %q, want 'My Title'", title)
	}
}

func TestOSC_SetTitle_OSC2(t *testing.T) {
	s := NewScreen(5, 80)
	// OSC 2 ; Another Title BEL
	s.Write([]byte("\x1b]2;Another Title\x07"))

	s.mu.Lock()
	title := s.Title
	s.mu.Unlock()
	if title != "Another Title" {
		t.Errorf("Title = %q, want 'Another Title'", title)
	}
}

func TestOSC_IgnoresUnknown(t *testing.T) {
	s := NewScreen(5, 80)
	// OSC 999 ; whatever BEL — should not crash
	s.Write([]byte("\x1b]999;something\x07"))
	// Just verify it doesn't panic and title is unchanged
	s.mu.Lock()
	title := s.Title
	s.mu.Unlock()
	if title != "" {
		t.Errorf("Title after unknown OSC = %q, want empty", title)
	}
}

// ---------------------------------------------------------------------------
// Write implements io.Writer
// ---------------------------------------------------------------------------

func TestWrite_ReturnsFullLength(t *testing.T) {
	s := NewScreen(5, 80)
	data := []byte("Hello\x1b[1mWorld")
	n, err := s.Write(data)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}
}

// ---------------------------------------------------------------------------
// UTF-8 handling: process only single bytes, so multi-byte chars
// get treated byte-by-byte — test that at least ASCII works
// ---------------------------------------------------------------------------

func TestWrite_ASCIIPrintable(t *testing.T) {
	// 95 printable ASCII chars (0x20-0x7E), need screen wide enough
	s := NewScreen(3, 100)
	printable := ""
	for b := byte(0x20); b < 0x7F; b++ {
		printable += string(rune(b))
	}
	s.Write([]byte(printable))

	r0 := s.PlainTextRow(0)
	if r0 != printable {
		t.Errorf("Printable ASCII mismatch: got length %d, want %d", len(r0), len(printable))
	}
}
