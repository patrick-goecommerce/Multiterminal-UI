package terminal

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// scrollUp / scrollDown with custom scroll region
// ---------------------------------------------------------------------------

func TestScrollUp_FullScreen(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))

	// Manually trigger scroll via Write (add a newline at the bottom)
	s.Write([]byte("\n"))

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	if !strings.HasPrefix(r0, "BBBBB") {
		t.Errorf("After scroll, row 0 = %q, want prefix 'BBBBB'", r0)
	}
	if !strings.HasPrefix(r1, "CCCCC") {
		t.Errorf("After scroll, row 1 = %q, want prefix 'CCCCC'", r1)
	}
	r2 := s.PlainTextRow(2)
	if strings.TrimSpace(r2) != "" {
		t.Errorf("After scroll, row 2 = %q, want blank", r2)
	}
}

func TestScrollDown_WithRegion(t *testing.T) {
	s := NewScreen(5, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC\r\nDDDDD\r\nEEEEE"))

	// Set scroll region to rows 2-4
	s.Write([]byte("\x1b[2;4r"))
	// Scroll down within region
	s.Write([]byte("\x1b[1T"))

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	r2 := s.PlainTextRow(2)
	r3 := s.PlainTextRow(3)
	r4 := s.PlainTextRow(4)

	if r0 != "AAAAA" {
		t.Errorf("Row 0 = %q, want 'AAAAA'", r0)
	}
	if strings.TrimSpace(r1) != "" {
		t.Errorf("Row 1 (top of region, new blank) = %q, want blank", r1)
	}
	if r2 != "BBBBB" {
		t.Errorf("Row 2 = %q, want 'BBBBB'", r2)
	}
	if r3 != "CCCCC" {
		t.Errorf("Row 3 = %q, want 'CCCCC'", r3)
	}
	if r4 != "EEEEE" {
		t.Errorf("Row 4 = %q, want 'EEEEE'", r4)
	}
}

// ---------------------------------------------------------------------------
// eraseDisplay modes
// ---------------------------------------------------------------------------

func TestEraseDisplay_StartToCursor(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	s.Write([]byte("\x1b[2;3H")) // row 1, col 2
	s.Write([]byte("\x1b[1J"))   // erase from start to cursor

	// Row 0 should be all blank
	r0 := s.PlainTextRow(0)
	if strings.TrimSpace(r0) != "" {
		t.Errorf("After ED 1, row 0 = %q, want blank", r0)
	}
	// Row 1, cols 0-2 should be blank, cols 3-4 preserved
	if s.CellAt(1, 0).Char != ' ' {
		t.Errorf("CellAt(1,0) should be blank after ED 1")
	}
	if s.CellAt(1, 2).Char != ' ' {
		t.Errorf("CellAt(1,2) should be blank after ED 1")
	}
	if s.CellAt(1, 3).Char != 'B' {
		t.Errorf("CellAt(1,3) = %q, want 'B' (preserved)", s.CellAt(1, 3).Char)
	}
	// Row 2 should be untouched
	r2 := s.PlainTextRow(2)
	if r2 != "CCCCC" {
		t.Errorf("After ED 1, row 2 = %q, want 'CCCCC'", r2)
	}
}

// ---------------------------------------------------------------------------
// deleteChars edge case: delete more chars than remaining
// ---------------------------------------------------------------------------

func TestDeleteChars_ExceedsRowLength(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("ABCDE"))
	s.Write([]byte("\x1b[1;2H")) // col 1
	s.Write([]byte("\x1b[99P"))  // delete 99 chars (more than remaining)

	r0 := s.PlainTextRow(0)
	// Col 0 ('A') preserved, rest should be blank
	if s.CellAt(0, 0).Char != 'A' {
		t.Errorf("CellAt(0,0) = %q, want 'A'", s.CellAt(0, 0).Char)
	}
	// Remaining should be spaces
	for c := 1; c < 5; c++ {
		if s.CellAt(0, c).Char != ' ' {
			t.Errorf("CellAt(0,%d) = %q, want ' '", c, s.CellAt(0, c).Char)
		}
	}
	_ = r0
}

// ---------------------------------------------------------------------------
// insertChars edge case: insert more than fits
// ---------------------------------------------------------------------------

func TestInsertChars_ExceedsRowLength(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("ABCDE"))
	s.Write([]byte("\x1b[1;2H")) // col 1
	s.Write([]byte("\x1b[99@"))  // insert 99 blanks

	// Col 0 preserved, everything else pushed off screen → blank
	if s.CellAt(0, 0).Char != 'A' {
		t.Errorf("CellAt(0,0) = %q, want 'A'", s.CellAt(0, 0).Char)
	}
	for c := 1; c < 5; c++ {
		if s.CellAt(0, c).Char != ' ' {
			t.Errorf("CellAt(0,%d) = %q, want ' '", c, s.CellAt(0, c).Char)
		}
	}
}

// ---------------------------------------------------------------------------
// fullReset
// ---------------------------------------------------------------------------

func TestFullReset(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("Hello World"))
	s.Write([]byte("\x1b]0;My Title\x07")) // set title
	s.Write([]byte("\x1b[1;31m"))           // set style
	s.Write([]byte("\x1b[3;4r"))            // set scroll region

	// Full reset
	s.Write([]byte("\x1bc"))

	// Cursor at origin
	row, col := s.Cursor()
	if row != 0 || col != 0 {
		t.Errorf("After reset, cursor = (%d,%d), want (0,0)", row, col)
	}

	// Screen blank
	for r := 0; r < 5; r++ {
		rowText := s.PlainTextRow(r)
		if strings.TrimSpace(rowText) != "" {
			t.Errorf("After reset, row %d = %q, want blank", r, rowText)
		}
	}

	// Title cleared
	s.mu.Lock()
	title := s.Title
	s.mu.Unlock()
	if title != "" {
		t.Errorf("After reset, title = %q, want empty", title)
	}
}

// ---------------------------------------------------------------------------
// clampCursor: already tested indirectly, but verify directly
// ---------------------------------------------------------------------------

func TestClampCursor_Negative(t *testing.T) {
	s := NewScreen(5, 5)
	// Force cursor to negative via a sequence that sets it directly
	s.Write([]byte("\x1b[1;1H"))  // origin (0,0)
	s.Write([]byte("\x1b[99D"))   // back 99 → clamped to 0

	_, col := s.Cursor()
	if col != 0 {
		t.Errorf("ClampCursor negative: col = %d, want 0", col)
	}
}

// ---------------------------------------------------------------------------
// Reverse line feed (ESC M)
// ---------------------------------------------------------------------------

func TestReverseLineFeed(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	// Move to top row
	s.Write([]byte("\x1b[1;1H"))
	// Reverse line feed — should scroll down
	s.Write([]byte("\x1bM"))

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	if strings.TrimSpace(r0) != "" {
		t.Errorf("After reverse LF at top, row 0 = %q, want blank", r0)
	}
	if r1 != "AAAAA" {
		t.Errorf("After reverse LF, row 1 = %q, want 'AAAAA'", r1)
	}
}

func TestReverseLineFeed_NotAtTop(t *testing.T) {
	s := NewScreen(5, 5)
	s.Write([]byte("\x1b[3;1H")) // row 2
	s.Write([]byte("\x1bM"))     // reverse line feed

	row, _ := s.Cursor()
	if row != 1 {
		t.Errorf("Reverse LF not at top: row = %d, want 1", row)
	}
}

// ---------------------------------------------------------------------------
// ESC D (index / line feed)
// ---------------------------------------------------------------------------

func TestESC_Index(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	// At row 2 (bottom), ESC D should scroll
	s.Write([]byte("\x1bD"))

	r0 := s.PlainTextRow(0)
	if !strings.HasPrefix(r0, "BBBBB") {
		t.Errorf("After ESC D at bottom, row 0 = %q, want 'BBBBB'", r0)
	}
}
