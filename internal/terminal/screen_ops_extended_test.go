package terminal

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// eraseLine – extended tests (complement screen_csi_test.go EraseLine tests)
// PlainTextRow trims trailing spaces, so we use strings.HasPrefix where needed
// ---------------------------------------------------------------------------

func TestEraseLine_Mode0_MidRow(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("ABCDEFGHIJ"))

	// Position cursor at col 5 (0-indexed) and erase to end
	s.Write([]byte("\x1b[1;6H")) // row 1, col 6 (1-indexed)
	s.Write([]byte("\x1b[0K"))

	row := s.PlainTextRow(0)
	if row != "ABCDE" {
		t.Fatalf("expected 'ABCDE' (trailing spaces trimmed), got %q", row)
	}
}

func TestEraseLine_Mode1_MidRow(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("ABCDEFGHIJ"))

	// Position cursor at col 5 and erase from start to cursor
	s.Write([]byte("\x1b[1;6H"))
	s.Write([]byte("\x1b[1K"))

	row := s.PlainTextRow(0)
	// Cols 0-5 erased (spaces), cols 6-9 = GHIJ → trimmed to "GHIJ" with leading spaces
	if !strings.HasSuffix(row, "GHIJ") {
		t.Fatalf("row should end with 'GHIJ', got %q", row)
	}
}

func TestEraseLine_Mode2_Clear(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("ABCDEFGHIJ"))

	s.Write([]byte("\x1b[1;3H")) // position somewhere in line
	s.Write([]byte("\x1b[2K"))   // erase entire line

	row := s.PlainTextRow(0)
	if row != "" {
		t.Fatalf("expected empty string (all spaces trimmed), got %q", row)
	}
}

// ---------------------------------------------------------------------------
// insertLines – extended tests
// ---------------------------------------------------------------------------

func TestInsertLines_Single(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("Line 0\r\nLine 1\r\nLine 2\r\nLine 3"))

	// Move cursor to row 1 and insert 1 line
	s.Write([]byte("\x1b[2;1H"))
	s.Write([]byte("\x1b[1L"))

	row0 := s.PlainTextRow(0)
	row1 := s.PlainTextRow(1) // should be blank (inserted)
	row2 := s.PlainTextRow(2) // should be old "Line 1"

	if !strings.HasPrefix(row0, "Line 0") {
		t.Fatalf("row 0 should start with 'Line 0', got %q", row0)
	}
	if row1 != "" {
		t.Fatalf("inserted row 1 should be blank, got %q", row1)
	}
	if !strings.HasPrefix(row2, "Line 1") {
		t.Fatalf("row 2 should start with 'Line 1', got %q", row2)
	}
}

func TestInsertLines_PushesContentOff(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("A\r\nB\r\nC\r\nD\r\nE"))

	// Move to row 1 and insert 2 lines
	s.Write([]byte("\x1b[2;1H"))
	s.Write([]byte("\x1b[2L"))

	// Row 0 = A, Row 1,2 = blank, Row 3 = B, Row 4 = C (D,E pushed off)
	if r := s.PlainTextRow(0); r[0] != 'A' {
		t.Fatalf("row 0 should start with 'A', got %q", r)
	}
	if r := s.PlainTextRow(1); r != "" {
		t.Fatalf("row 1 should be blank, got %q", r)
	}
	if r := s.PlainTextRow(2); r != "" {
		t.Fatalf("row 2 should be blank, got %q", r)
	}
	if r := s.PlainTextRow(3); r[0] != 'B' {
		t.Fatalf("row 3 should start with 'B', got %q", r)
	}
}

// ---------------------------------------------------------------------------
// deleteLines – extended tests
// ---------------------------------------------------------------------------

func TestDeleteLines_SinglePullsUp(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("A\r\nB\r\nC\r\nD\r\nE"))

	// Move to row 1 and delete 1 line
	s.Write([]byte("\x1b[2;1H"))
	s.Write([]byte("\x1b[1M"))

	// Row 0 = A, Row 1 = C (B deleted), Row 2 = D, Row 3 = E, Row 4 = blank
	if r := s.PlainTextRow(0); r[0] != 'A' {
		t.Fatalf("row 0 should start with 'A', got %q", r)
	}
	if r := s.PlainTextRow(1); r[0] != 'C' {
		t.Fatalf("row 1 should start with 'C' (B deleted), got %q", r)
	}
	if r := s.PlainTextRow(4); r != "" {
		t.Fatalf("row 4 should be blank, got %q", r)
	}
}

func TestDeleteLines_MultipleFromTop(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("A\r\nB\r\nC\r\nD\r\nE"))

	s.Write([]byte("\x1b[1;1H")) // row 0
	s.Write([]byte("\x1b[2M"))   // delete 2 lines

	// Row 0 = C, Row 1 = D, Row 2 = E, Row 3,4 = blank
	if r := s.PlainTextRow(0); r[0] != 'C' {
		t.Fatalf("row 0 should start with 'C', got %q", r)
	}
	if r := s.PlainTextRow(1); r[0] != 'D' {
		t.Fatalf("row 1 should start with 'D', got %q", r)
	}
}

// ---------------------------------------------------------------------------
// reverseLineFeed – extended tests
// ---------------------------------------------------------------------------

func TestReverseLineFeed_Normal_Ext(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("\x1b[3;1H")) // Move cursor to row 2

	r, _ := s.Cursor()
	if r != 2 {
		t.Fatalf("cursor should be at row 2, got %d", r)
	}

	s.Write([]byte("\x1bM")) // ESC M = reverse line feed

	r, _ = s.Cursor()
	if r != 1 {
		t.Fatalf("cursor should be at row 1 after reverse LF, got %d", r)
	}
}

func TestReverseLineFeed_AtTopScrollsDown(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("Row 0\r\nRow 1\r\nRow 2"))

	s.Write([]byte("\x1b[1;1H")) // Move to row 0 (top)
	s.Write([]byte("\x1bM"))     // Reverse LF at top should scroll down

	// Row 0 should now be blank (scrolled in from above)
	if r := s.PlainTextRow(0); r != "" {
		t.Fatalf("row 0 should be blank after reverse scroll, got %q", r)
	}
	// Old row 0 content should be at row 1
	if r := s.PlainTextRow(1); !strings.HasPrefix(r, "Row 0") {
		t.Fatalf("row 1 should have old row 0 content, got %q", r)
	}
}

// ---------------------------------------------------------------------------
// eraseDisplay mode 1 – extended
// ---------------------------------------------------------------------------

func TestEraseDisplay_Mode1_PartialClear(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))

	// Cursor at row 1, col 2
	s.Write([]byte("\x1b[2;3H"))
	s.Write([]byte("\x1b[1J")) // erase from start to cursor

	// Row 0 should be blank
	if r := s.PlainTextRow(0); r != "" {
		t.Fatalf("row 0 should be erased (blank), got %q", r)
	}
	// Row 1: cols 0-2 erased, cols 3-4 = BB
	r1 := s.PlainTextRow(1)
	if !strings.HasSuffix(r1, "BB") {
		t.Fatalf("row 1 should end with 'BB', got %q", r1)
	}
	// Row 2 should be untouched
	if r := s.PlainTextRow(2); r != "CCCCC" {
		t.Fatalf("row 2 should be untouched, got %q", r)
	}
}

// ---------------------------------------------------------------------------
// fullReset – extended
// ---------------------------------------------------------------------------

func TestFullReset_ClearsAll(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("Hello\r\nWorld"))
	s.Write([]byte("\x1b[1;2r")) // set scroll region
	s.Write([]byte("\x1bc"))     // full reset

	// All rows should be blank
	for r := 0; r < 3; r++ {
		if row := s.PlainTextRow(r); row != "" {
			t.Fatalf("row %d should be blank after reset, got %q", r, row)
		}
	}

	row, col := s.Cursor()
	if row != 0 || col != 0 {
		t.Fatalf("cursor should be at (0,0), got (%d,%d)", row, col)
	}
}

// ---------------------------------------------------------------------------
// Erase Characters (ECH / CSI X) – extended
// ---------------------------------------------------------------------------

func TestEraseCharacters_MidLine(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))

	// Position cursor at col 3 and erase 4 characters
	s.Write([]byte("\x1b[1;4H"))
	s.Write([]byte("\x1b[4X"))

	row := s.PlainTextRow(0)
	if row != "ABC    HIJ" {
		t.Fatalf("expected 'ABC    HIJ', got %q", row)
	}
}

func TestEraseCharacters_BeyondEnd(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))

	// Position cursor at col 8 and erase 10 characters (more than remaining)
	s.Write([]byte("\x1b[1;9H"))
	s.Write([]byte("\x1b[10X"))

	row := s.PlainTextRow(0)
	if row != "ABCDEFGH" {
		t.Fatalf("expected 'ABCDEFGH' (trailing spaces trimmed), got %q", row)
	}
}

// ---------------------------------------------------------------------------
// scrollRegion boundary tests
// ---------------------------------------------------------------------------

func TestScrollRegion_InsertLinesRespectsRegion(t *testing.T) {
	s := NewScreen(5, 5)
	s.Write([]byte("A\r\nB\r\nC\r\nD\r\nE"))

	// Set scroll region to rows 2-4 (1-indexed)
	s.Write([]byte("\x1b[2;4r"))
	// Move cursor to row 1 (inside region, 0-indexed)
	s.Write([]byte("\x1b[2;1H"))
	s.Write([]byte("\x1b[1L"))

	// Row 0 (A) should be unchanged (outside region)
	if r := s.PlainTextRow(0); r[0] != 'A' {
		t.Fatalf("row 0 outside region should be unchanged, got %q", r)
	}
}

func TestScrollRegion_LineFeedWithinRegion(t *testing.T) {
	s := NewScreen(5, 10)
	// Set scroll region to rows 2-4 (1-indexed)
	s.Write([]byte("\x1b[2;4r"))
	// Move to bottom of region (row 3, 0-indexed)
	s.Write([]byte("\x1b[4;1H"))
	s.Write([]byte("Bottom"))
	// Linefeed at bottom of region should scroll within region
	s.Write([]byte("\n"))

	// Row 0 should remain blank (above region, untouched)
	if r := s.PlainTextRow(0); r != "" {
		t.Fatalf("row 0 should be untouched, got %q", r)
	}
}
