package terminal

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// CSI cursor movement
// ---------------------------------------------------------------------------

func TestCSI_CursorUp(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[5;5H"))  // move to (4,4)
	s.Write([]byte("\x1b[2A"))    // cursor up 2

	row, col := s.Cursor()
	if row != 2 || col != 4 {
		t.Errorf("After CUU 2: cursor = (%d,%d), want (2,4)", row, col)
	}
}

func TestCSI_CursorUp_ClampsToZero(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[2;1H"))  // row 1
	s.Write([]byte("\x1b[99A"))   // up 99

	row, _ := s.Cursor()
	if row != 0 {
		t.Errorf("CUU clamp: row = %d, want 0", row)
	}
}

func TestCSI_CursorDown(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[1;1H"))  // origin
	s.Write([]byte("\x1b[3B"))    // down 3

	row, col := s.Cursor()
	if row != 3 || col != 0 {
		t.Errorf("After CUD 3: cursor = (%d,%d), want (3,0)", row, col)
	}
}

func TestCSI_CursorDown_ClampsToBottom(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("\x1b[99B"))

	row, _ := s.Cursor()
	if row != 4 {
		t.Errorf("CUD clamp: row = %d, want 4", row)
	}
}

func TestCSI_CursorForward(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[5C"))

	_, col := s.Cursor()
	if col != 5 {
		t.Errorf("CUF: col = %d, want 5", col)
	}
}

func TestCSI_CursorBackward(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[1;6H"))  // col 5
	s.Write([]byte("\x1b[3D"))    // back 3

	_, col := s.Cursor()
	if col != 2 {
		t.Errorf("CUB: col = %d, want 2", col)
	}
}

func TestCSI_CursorBackward_ClampsToZero(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[99D"))

	_, col := s.Cursor()
	if col != 0 {
		t.Errorf("CUB clamp: col = %d, want 0", col)
	}
}

// ---------------------------------------------------------------------------
// CSI cursor positioning
// ---------------------------------------------------------------------------

func TestCSI_CursorPosition(t *testing.T) {
	s := NewScreen(24, 80)
	s.Write([]byte("\x1b[10;20H"))

	row, col := s.Cursor()
	if row != 9 || col != 19 {
		t.Errorf("CUP: cursor = (%d,%d), want (9,19)", row, col)
	}
}

func TestCSI_CursorPosition_Default(t *testing.T) {
	s := NewScreen(24, 80)
	s.Write([]byte("AAAA"))
	s.Write([]byte("\x1b[H")) // no params = (1,1)

	row, col := s.Cursor()
	if row != 0 || col != 0 {
		t.Errorf("CUP default: cursor = (%d,%d), want (0,0)", row, col)
	}
}

func TestCSI_CursorNextLine(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("ABCDE"))
	s.Write([]byte("\x1b[2E")) // next line x2

	row, col := s.Cursor()
	if row != 2 || col != 0 {
		t.Errorf("CNL: cursor = (%d,%d), want (2,0)", row, col)
	}
}

func TestCSI_CursorPreviousLine(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[5;5H"))
	s.Write([]byte("\x1b[2F"))

	row, col := s.Cursor()
	if row != 2 || col != 0 {
		t.Errorf("CPL: cursor = (%d,%d), want (2,0)", row, col)
	}
}

func TestCSI_CursorHorizontalAbsolute(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("ABCDE"))
	s.Write([]byte("\x1b[3G")) // col 3 (1-indexed)

	_, col := s.Cursor()
	if col != 2 {
		t.Errorf("CHA: col = %d, want 2", col)
	}
}

func TestCSI_VerticalPositionAbsolute(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[5d")) // row 5 (1-indexed)

	row, _ := s.Cursor()
	if row != 4 {
		t.Errorf("VPA: row = %d, want 4", row)
	}
}

// ---------------------------------------------------------------------------
// CSI erase
// ---------------------------------------------------------------------------

func TestCSI_EraseDisplay_CursorToEnd(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAAAAAAAAAA\r\n")) // fill
	s.Write([]byte("\x1b[1;3H"))          // row 0, col 2
	s.Write([]byte("\x1b[0J"))            // erase from cursor to end

	// Row 0, cols 0-1 should be preserved
	if ch := s.CellAt(0, 0).Char; ch != 'A' {
		t.Errorf("CellAt(0,0) = %q, want 'A'", ch)
	}
	// Row 0, col 2+ should be blank
	if ch := s.CellAt(0, 2).Char; ch != ' ' {
		t.Errorf("CellAt(0,2) = %q, want ' '", ch)
	}
	// Rows below should be blank
	r1 := s.PlainTextRow(1)
	if strings.TrimSpace(r1) != "" {
		t.Errorf("Row 1 should be blank, got %q", r1)
	}
}

func TestCSI_EraseDisplay_Full(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("Hello"))
	s.Write([]byte("\x1b[2J")) // erase entire display

	for r := 0; r < 3; r++ {
		row := s.PlainTextRow(r)
		if strings.TrimSpace(row) != "" {
			t.Errorf("After ED 2, row %d = %q, want blank", r, row)
		}
	}
}

func TestCSI_EraseLine_CursorToEnd(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))
	s.Write([]byte("\x1b[1;4H"))  // col 3
	s.Write([]byte("\x1b[0K"))    // erase from cursor to end of line

	r0 := s.PlainTextRow(0)
	if r0 != "ABC" {
		t.Errorf("After EL 0, row 0 = %q, want 'ABC'", r0)
	}
}

func TestCSI_EraseLine_StartToCursor(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))
	s.Write([]byte("\x1b[1;4H"))  // col 3
	s.Write([]byte("\x1b[1K"))    // erase from start to cursor

	r0 := s.PlainTextRow(0)
	// cols 0-3 erased, cols 4-9 preserved
	if !strings.HasPrefix(r0, "    ") {
		t.Errorf("After EL 1, row 0 = %q, expected leading spaces", r0)
	}
	if s.CellAt(0, 4).Char != 'E' {
		t.Errorf("CellAt(0,4) = %q, want 'E'", s.CellAt(0, 4).Char)
	}
}

func TestCSI_EraseLine_Entire(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))
	s.Write([]byte("\x1b[1;4H"))
	s.Write([]byte("\x1b[2K")) // erase entire line

	r0 := s.PlainTextRow(0)
	if strings.TrimSpace(r0) != "" {
		t.Errorf("After EL 2, row 0 = %q, want blank", r0)
	}
}

// ---------------------------------------------------------------------------
// CSI erase characters
// ---------------------------------------------------------------------------

func TestCSI_EraseCharacters(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))
	s.Write([]byte("\x1b[1;3H")) // col 2
	s.Write([]byte("\x1b[3X"))   // erase 3 chars

	r0 := s.PlainTextRow(0)
	if r0 != "AB   FGHIJ" {
		t.Errorf("After ECH 3, row 0 = %q, want 'AB   FGHIJ'", r0)
	}
}

// ---------------------------------------------------------------------------
// CSI delete / insert characters
// ---------------------------------------------------------------------------

func TestCSI_DeleteCharacters(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))
	s.Write([]byte("\x1b[1;3H")) // col 2
	s.Write([]byte("\x1b[2P"))   // delete 2 chars

	r0 := s.PlainTextRow(0)
	if r0 != "ABEFGHIJ" {
		t.Errorf("After DCH 2, row 0 = %q, want 'ABEFGHIJ'", r0)
	}
}

func TestCSI_InsertCharacters(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("ABCDEFGHIJ"))
	s.Write([]byte("\x1b[1;3H")) // col 2
	s.Write([]byte("\x1b[2@"))   // insert 2 blanks

	r0 := s.PlainTextRow(0)
	if r0 != "AB  CDEFGH" {
		t.Errorf("After ICH 2, row 0 = %q, want 'AB  CDEFGH'", r0)
	}
}

// ---------------------------------------------------------------------------
// CSI insert / delete lines
// ---------------------------------------------------------------------------

func TestCSI_InsertLines(t *testing.T) {
	s := NewScreen(5, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC\r\nDDDDD\r\nEEEEE"))
	s.Write([]byte("\x1b[2;1H")) // row 1
	s.Write([]byte("\x1b[1L"))   // insert 1 line

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	r2 := s.PlainTextRow(2)
	if r0 != "AAAAA" {
		t.Errorf("After IL, row 0 = %q, want 'AAAAA'", r0)
	}
	if strings.TrimSpace(r1) != "" {
		t.Errorf("After IL, row 1 = %q, want blank", r1)
	}
	if r2 != "BBBBB" {
		t.Errorf("After IL, row 2 = %q, want 'BBBBB'", r2)
	}
}

func TestCSI_DeleteLines(t *testing.T) {
	s := NewScreen(5, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC\r\nDDDDD\r\nEEEEE"))
	s.Write([]byte("\x1b[2;1H")) // row 1
	s.Write([]byte("\x1b[1M"))   // delete 1 line

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	r3 := s.PlainTextRow(3)
	if r0 != "AAAAA" {
		t.Errorf("After DL, row 0 = %q, want 'AAAAA'", r0)
	}
	if r1 != "CCCCC" {
		t.Errorf("After DL, row 1 = %q, want 'CCCCC'", r1)
	}
	// Bottom row should be blank (shifted up)
	r4 := s.PlainTextRow(4)
	if strings.TrimSpace(r4) != "" {
		t.Errorf("After DL, row 4 = %q, want blank", r4)
	}
	_ = r3
}

// ---------------------------------------------------------------------------
// CSI scroll
// ---------------------------------------------------------------------------

func TestCSI_ScrollUp(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	s.Write([]byte("\x1b[1S")) // scroll up 1

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	r2 := s.PlainTextRow(2)
	if r0 != "BBBBB" {
		t.Errorf("After SU, row 0 = %q, want 'BBBBB'", r0)
	}
	if r1 != "CCCCC" {
		t.Errorf("After SU, row 1 = %q, want 'CCCCC'", r1)
	}
	if strings.TrimSpace(r2) != "" {
		t.Errorf("After SU, row 2 = %q, want blank", r2)
	}
}

func TestCSI_ScrollDown(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	s.Write([]byte("\x1b[1T")) // scroll down 1

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	r2 := s.PlainTextRow(2)
	if strings.TrimSpace(r0) != "" {
		t.Errorf("After SD, row 0 = %q, want blank", r0)
	}
	if r1 != "AAAAA" {
		t.Errorf("After SD, row 1 = %q, want 'AAAAA'", r1)
	}
	if r2 != "BBBBB" {
		t.Errorf("After SD, row 2 = %q, want 'BBBBB'", r2)
	}
}

// ---------------------------------------------------------------------------
// CSI scroll region (DECSTBM)
// ---------------------------------------------------------------------------

func TestCSI_ScrollRegion(t *testing.T) {
	s := NewScreen(5, 5)
	s.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC\r\nDDDDD\r\nEEEEE"))

	// Set scroll region to rows 2-4 (1-indexed)
	s.Write([]byte("\x1b[2;4r"))

	// Scroll up within region
	s.Write([]byte("\x1b[1S"))

	r0 := s.PlainTextRow(0)
	r1 := s.PlainTextRow(1)
	r2 := s.PlainTextRow(2)
	r3 := s.PlainTextRow(3)
	r4 := s.PlainTextRow(4)

	// Row 0 (outside region) should be unchanged
	if r0 != "AAAAA" {
		t.Errorf("Row 0 = %q, want 'AAAAA'", r0)
	}
	// Rows 1-3 (region) should have scrolled: BBB gone, CCC->row1, DDD->row2, blank->row3
	if r1 != "CCCCC" {
		t.Errorf("Row 1 = %q, want 'CCCCC'", r1)
	}
	if r2 != "DDDDD" {
		t.Errorf("Row 2 = %q, want 'DDDDD'", r2)
	}
	if strings.TrimSpace(r3) != "" {
		t.Errorf("Row 3 = %q, want blank", r3)
	}
	// Row 4 (outside region) should be unchanged
	if r4 != "EEEEE" {
		t.Errorf("Row 4 = %q, want 'EEEEE'", r4)
	}
}

// ---------------------------------------------------------------------------
// CSI save/restore cursor
// ---------------------------------------------------------------------------

func TestCSI_SaveRestoreCursor(t *testing.T) {
	s := NewScreen(10, 10)
	s.Write([]byte("\x1b[5;5H")) // move to (4,4)
	s.Write([]byte("\x1b[s"))    // save
	s.Write([]byte("\x1b[1;1H")) // move to origin
	s.Write([]byte("\x1b[u"))    // restore

	row, col := s.Cursor()
	if row != 4 || col != 4 {
		t.Errorf("After CSI s/u, cursor = (%d,%d), want (4,4)", row, col)
	}
}

// ---------------------------------------------------------------------------
// SGR – Select Graphic Rendition
// ---------------------------------------------------------------------------

func TestSGR_Reset(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[1m"))  // bold
	s.Write([]byte("\x1b[0m"))  // reset
	s.Write([]byte("A"))

	cell := s.CellAt(0, 0)
	if cell.Style.Bold {
		t.Error("After SGR 0, Bold should be false")
	}
}

func TestSGR_Bold(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[1mA"))

	cell := s.CellAt(0, 0)
	if !cell.Style.Bold {
		t.Error("After SGR 1, Bold should be true")
	}
}

func TestSGR_Dim(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[2mA"))
	if !s.CellAt(0, 0).Style.Dim {
		t.Error("After SGR 2, Dim should be true")
	}
}

func TestSGR_Italic(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[3mA"))
	if !s.CellAt(0, 0).Style.Italic {
		t.Error("After SGR 3, Italic should be true")
	}
}

func TestSGR_Underline(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[4mA"))
	if !s.CellAt(0, 0).Style.Underline {
		t.Error("After SGR 4, Underline should be true")
	}
}

func TestSGR_Reverse(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[7mA"))
	if !s.CellAt(0, 0).Style.Reverse {
		t.Error("After SGR 7, Reverse should be true")
	}
}

func TestSGR_Strike(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[9mA"))
	if !s.CellAt(0, 0).Style.Strike {
		t.Error("After SGR 9, Strike should be true")
	}
}

func TestSGR_ResetAttributes(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[1;2;3;4;7;9m")) // all on
	s.Write([]byte("\x1b[22m"))            // bold+dim off
	s.Write([]byte("\x1b[23m"))            // italic off
	s.Write([]byte("\x1b[24m"))            // underline off
	s.Write([]byte("\x1b[27m"))            // reverse off
	s.Write([]byte("\x1b[29m"))            // strike off
	s.Write([]byte("A"))

	cell := s.CellAt(0, 0)
	if cell.Style.Bold || cell.Style.Dim || cell.Style.Italic ||
		cell.Style.Underline || cell.Style.Reverse || cell.Style.Strike {
		t.Errorf("After resetting all attributes, style = %+v", cell.Style)
	}
}

// ---------------------------------------------------------------------------
// SGR colours
// ---------------------------------------------------------------------------

func TestSGR_StandardFG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[31mA")) // red foreground

	cell := s.CellAt(0, 0)
	// Standard FG: 30+color, so 31 → FG = 31-30+1 = 2
	if cell.Style.FG != 2 {
		t.Errorf("Standard FG 31: FG = %d, want 2", cell.Style.FG)
	}
}

func TestSGR_StandardBG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[42mA")) // green background

	cell := s.CellAt(0, 0)
	// Standard BG: 40+color, so 42 → BG = 42-40+1 = 3
	if cell.Style.BG != 3 {
		t.Errorf("Standard BG 42: BG = %d, want 3", cell.Style.BG)
	}
}

func TestSGR_BrightFG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[91mA")) // bright red FG

	cell := s.CellAt(0, 0)
	// Bright FG: 90+color, so 91 → FG = 91-90+9 = 10
	if cell.Style.FG != 10 {
		t.Errorf("Bright FG 91: FG = %d, want 10", cell.Style.FG)
	}
}

func TestSGR_BrightBG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[101mA")) // bright red BG

	cell := s.CellAt(0, 0)
	// Bright BG: 100+color, so 101 → BG = 101-100+9 = 10
	if cell.Style.BG != 10 {
		t.Errorf("Bright BG 101: BG = %d, want 10", cell.Style.BG)
	}
}

func TestSGR_DefaultFG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[31m"))  // set FG
	s.Write([]byte("\x1b[39m"))  // reset to default FG
	s.Write([]byte("A"))

	cell := s.CellAt(0, 0)
	if cell.Style.FG != 0 {
		t.Errorf("After SGR 39, FG = %d, want 0", cell.Style.FG)
	}
}

func TestSGR_DefaultBG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[42m"))  // set BG
	s.Write([]byte("\x1b[49m"))  // reset to default BG
	s.Write([]byte("A"))

	cell := s.CellAt(0, 0)
	if cell.Style.BG != 0 {
		t.Errorf("After SGR 49, BG = %d, want 0", cell.Style.BG)
	}
}

func TestSGR_256Color_FG(t *testing.T) {
	s := NewScreen(3, 10)
	// ESC[38;5;196m — 256-color red foreground
	s.Write([]byte("\x1b[38;5;196mA"))

	cell := s.CellAt(0, 0)
	// 256-color: stored as palette_index + 1
	if cell.Style.FG != 197 {
		t.Errorf("256-color FG 196: FG = %d, want 197", cell.Style.FG)
	}
}

func TestSGR_256Color_BG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[48;5;50mA"))

	cell := s.CellAt(0, 0)
	if cell.Style.BG != 51 {
		t.Errorf("256-color BG 50: BG = %d, want 51", cell.Style.BG)
	}
}

func TestSGR_Truecolor_FG(t *testing.T) {
	s := NewScreen(3, 10)
	// ESC[38;2;255;128;0m — truecolor orange
	s.Write([]byte("\x1b[38;2;255;128;0mA"))

	cell := s.CellAt(0, 0)
	// Truecolor: 256 + 1 + R<<16 + G<<8 + B
	want := 256 + 1 + 255<<16 + 128<<8 + 0
	if cell.Style.FG != want {
		t.Errorf("Truecolor FG: FG = %d, want %d", cell.Style.FG, want)
	}
}

func TestSGR_Truecolor_BG(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[48;2;10;20;30mA"))

	cell := s.CellAt(0, 0)
	want := 256 + 1 + 10<<16 + 20<<8 + 30
	if cell.Style.BG != want {
		t.Errorf("Truecolor BG: BG = %d, want %d", cell.Style.BG, want)
	}
}

func TestSGR_CombinedAttributes(t *testing.T) {
	s := NewScreen(3, 10)
	// Bold + red FG in one sequence
	s.Write([]byte("\x1b[1;31mA"))

	cell := s.CellAt(0, 0)
	if !cell.Style.Bold {
		t.Error("Combined SGR: Bold should be true")
	}
	if cell.Style.FG != 2 { // red = 31-30+1 = 2
		t.Errorf("Combined SGR: FG = %d, want 2", cell.Style.FG)
	}
}

func TestSGR_EmptyParams(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[1m"))  // bold on
	s.Write([]byte("\x1b[m"))   // bare 'm' = reset
	s.Write([]byte("A"))

	cell := s.CellAt(0, 0)
	if cell.Style.Bold {
		t.Error("After bare SGR 'm', Bold should be false")
	}
}

// ---------------------------------------------------------------------------
// sgrSequence round-trip
// ---------------------------------------------------------------------------

func TestSgrSequence_Default(t *testing.T) {
	seq := sgrSequence(CellStyle{})
	if seq != "\x1b[0m" {
		t.Errorf("sgrSequence(default) = %q, want \\x1b[0m", seq)
	}
}

func TestSgrSequence_Bold(t *testing.T) {
	seq := sgrSequence(CellStyle{Bold: true})
	if seq != "\x1b[0;1m" {
		t.Errorf("sgrSequence(Bold) = %q, want \\x1b[0;1m", seq)
	}
}

func TestSgrSequence_StandardFG(t *testing.T) {
	// FG=2 means standard red (30+FG-1+1 = 30+1 = 31... wait let me check)
	// In the code: FG > 0 && FG <= 8 → fmt.Sprintf("%d", 29+FG)
	// FG=2 → 29+2 = 31 (red)
	seq := sgrSequence(CellStyle{FG: 2})
	if !strings.Contains(seq, "31") {
		t.Errorf("sgrSequence(FG=2) = %q, expected to contain '31'", seq)
	}
}

func TestSgrSequence_256Color(t *testing.T) {
	// FG=197 → 38;5;196 (palette index = FG-1)
	seq := sgrSequence(CellStyle{FG: 197})
	if !strings.Contains(seq, "38;5;196") {
		t.Errorf("sgrSequence(FG=197) = %q, expected '38;5;196'", seq)
	}
}

func TestSgrSequence_Truecolor(t *testing.T) {
	// Encode orange: 256 + 1 + 255<<16 + 128<<8 + 0
	fg := 256 + 1 + 255<<16 + 128<<8
	seq := sgrSequence(CellStyle{FG: fg})
	if !strings.Contains(seq, "38;2;255;128;0") {
		t.Errorf("sgrSequence(truecolor) = %q, expected '38;2;255;128;0'", seq)
	}
}

// ---------------------------------------------------------------------------
// parseCSIParams
// ---------------------------------------------------------------------------

func TestParseCSIParams_Normal(t *testing.T) {
	s := NewScreen(3, 3)
	s.csiBuf = []byte("5;10;20")
	params := s.parseCSIParams()
	if len(params) != 3 || params[0] != 5 || params[1] != 10 || params[2] != 20 {
		t.Errorf("parseCSIParams('5;10;20') = %v, want [5,10,20]", params)
	}
}

func TestParseCSIParams_Empty(t *testing.T) {
	s := NewScreen(3, 3)
	s.csiBuf = nil
	params := s.parseCSIParams()
	if params != nil {
		t.Errorf("parseCSIParams(nil) = %v, want nil", params)
	}
}

func TestParseCSIParams_PrivateMode(t *testing.T) {
	s := NewScreen(3, 3)
	s.csiBuf = []byte("?25")
	params := s.parseCSIParams()
	// Should strip '?' and parse "25"
	if len(params) != 1 || params[0] != 25 {
		t.Errorf("parseCSIParams('?25') = %v, want [25]", params)
	}
}

func TestParseCSIParams_MissingValues(t *testing.T) {
	s := NewScreen(3, 3)
	s.csiBuf = []byte(";5;")
	params := s.parseCSIParams()
	// ";5;" → ["", "5", ""] → [0, 5, 0]
	if len(params) != 3 || params[0] != 0 || params[1] != 5 || params[2] != 0 {
		t.Errorf("parseCSIParams(';5;') = %v, want [0,5,0]", params)
	}
}

// ---------------------------------------------------------------------------
// paramDefault
// ---------------------------------------------------------------------------

func TestParamDefault(t *testing.T) {
	params := []int{0, 5, 0}
	// idx=0, val=0 → use default
	if v := paramDefault(params, 0, 1); v != 1 {
		t.Errorf("paramDefault(0, default=1) = %d, want 1", v)
	}
	// idx=1, val=5 → use actual
	if v := paramDefault(params, 1, 1); v != 5 {
		t.Errorf("paramDefault(1, default=1) = %d, want 5", v)
	}
	// idx=99, out of range → use default
	if v := paramDefault(params, 99, 42); v != 42 {
		t.Errorf("paramDefault(99, default=42) = %d, want 42", v)
	}
	// nil params
	if v := paramDefault(nil, 0, 7); v != 7 {
		t.Errorf("paramDefault(nil, default=7) = %d, want 7", v)
	}
}

// ---------------------------------------------------------------------------
// CSI private mode (no-op but should not crash)
// ---------------------------------------------------------------------------

func TestCSI_PrivateMode_NoCrash(t *testing.T) {
	s := NewScreen(5, 10)
	// CSI ? 25 h — show cursor (ignored)
	s.Write([]byte("\x1b[?25h"))
	// CSI ? 25 l — hide cursor (ignored)
	s.Write([]byte("\x1b[?25l"))
	// CSI ? 1049 h — alternate screen buffer (ignored)
	s.Write([]byte("\x1b[?1049h"))

	// Just checking it doesn't panic
	row, col := s.Cursor()
	_ = fmt.Sprintf("cursor at (%d,%d)", row, col)
}
