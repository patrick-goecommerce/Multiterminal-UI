package terminal

import "testing"

// ---------------------------------------------------------------------------
// Extended SGR tests – complement existing screen_csi_test.go coverage
// ---------------------------------------------------------------------------

func TestSGR_Strikethrough_Ext(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("\x1b[9mTest"))

	cell := s.CellAt(0, 0)
	if !cell.Style.Strike {
		t.Fatal("expected strikethrough")
	}
}

func TestSGR_DisableAllAttributes(t *testing.T) {
	s := NewScreen(3, 30)
	// Enable everything, then disable each individually
	s.Write([]byte("\x1b[1;2;3;4;7;9m"))
	s.Write([]byte("\x1b[22;23;24;27;29mX"))

	cell := s.CellAt(0, 0)
	if cell.Style.Bold || cell.Style.Dim || cell.Style.Italic ||
		cell.Style.Underline || cell.Style.Reverse || cell.Style.Strike {
		t.Fatalf("all attributes should be off after individual disables, got: %+v", cell.Style)
	}
}

func TestSGR_StandardForegroundAllColors(t *testing.T) {
	s := NewScreen(1, 80)
	// Write chars with FG 30-37 (standard 8 colors)
	for i := 0; i < 8; i++ {
		s.Write([]byte{0x1b, '[', byte('0' + i + 30/10), byte('0' + (i+30)%10), 'm', 'X'})
	}
	// This is cumbersome with individual bytes, let's use a simpler approach
	s2 := NewScreen(1, 80)
	s2.Write([]byte("\x1b[30mA\x1b[31mB\x1b[32mC\x1b[33mD\x1b[34mE\x1b[35mF\x1b[36mG\x1b[37mH"))

	expectedFG := []int{1, 2, 3, 4, 5, 6, 7, 8}
	for i, expected := range expectedFG {
		cell := s2.CellAt(0, i)
		if cell.Style.FG != expected {
			t.Errorf("char %d: expected FG=%d, got %d", i, expected, cell.Style.FG)
		}
	}
}

func TestSGR_StandardBackgroundAllColors(t *testing.T) {
	s := NewScreen(1, 80)
	s.Write([]byte("\x1b[40mA\x1b[41mB\x1b[42mC\x1b[43mD\x1b[44mE\x1b[45mF\x1b[46mG\x1b[47mH"))

	expectedBG := []int{1, 2, 3, 4, 5, 6, 7, 8}
	for i, expected := range expectedBG {
		cell := s.CellAt(0, i)
		if cell.Style.BG != expected {
			t.Errorf("char %d: expected BG=%d, got %d", i, expected, cell.Style.BG)
		}
	}
}

func TestSGR_DefaultFGResets(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("\x1b[31m"))   // red FG
	s.Write([]byte("\x1b[39mX"))  // reset FG to default

	cell := s.CellAt(0, 0)
	if cell.Style.FG != 0 {
		t.Fatalf("FG should be default (0) after 39, got %d", cell.Style.FG)
	}
}

func TestSGR_DefaultBGResets(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("\x1b[42m"))   // green BG
	s.Write([]byte("\x1b[49mX"))  // reset BG to default

	cell := s.CellAt(0, 0)
	if cell.Style.BG != 0 {
		t.Fatalf("BG should be default (0) after 49, got %d", cell.Style.BG)
	}
}

func TestSGR_EmptyParamsResetsAll(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("\x1b[1;3;31m"))  // bold + italic + red
	s.Write([]byte("\x1b[mX"))       // empty SGR = reset

	cell := s.CellAt(0, 0)
	if cell.Style.Bold || cell.Style.Italic || cell.Style.FG != 0 {
		t.Fatalf("empty SGR should reset everything, got %+v", cell.Style)
	}
}

func TestSGR_256Color_FG_Index0(t *testing.T) {
	s := NewScreen(3, 20)
	// 256-color FG index 0 (black)
	s.Write([]byte("\x1b[38;5;0mX"))

	cell := s.CellAt(0, 0)
	// FG = 0 + 1 = 1
	if cell.Style.FG != 1 {
		t.Fatalf("expected FG=1 (256-color index 0+1), got %d", cell.Style.FG)
	}
}

func TestSGR_256Color_BG_Index255(t *testing.T) {
	s := NewScreen(3, 20)
	// 256-color BG index 255
	s.Write([]byte("\x1b[48;5;255mX"))

	cell := s.CellAt(0, 0)
	// BG = 255 + 1 = 256
	if cell.Style.BG != 256 {
		t.Fatalf("expected BG=256 (256-color index 255+1), got %d", cell.Style.BG)
	}
}

func TestSGR_TrueColor_Black(t *testing.T) {
	s := NewScreen(3, 20)
	// Truecolor FG: RGB(0, 0, 0)
	s.Write([]byte("\x1b[38;2;0;0;0mX"))

	cell := s.CellAt(0, 0)
	expected := 256 + 1 // 0x01000000 encoded
	if cell.Style.FG != expected {
		t.Fatalf("expected FG=%d for black truecolor, got %d", expected, cell.Style.FG)
	}
}

func TestSGR_TrueColor_White(t *testing.T) {
	s := NewScreen(3, 20)
	// Truecolor FG: RGB(255, 255, 255)
	s.Write([]byte("\x1b[38;2;255;255;255mX"))

	cell := s.CellAt(0, 0)
	expected := 256 + 1 + (255 << 16) + (255 << 8) + 255
	if cell.Style.FG != expected {
		t.Fatalf("expected FG=%d for white truecolor, got %d", expected, cell.Style.FG)
	}
}

// ---------------------------------------------------------------------------
// sgrSequence extended tests
// ---------------------------------------------------------------------------

func TestSgrSequence_BoldItalicUnderline(t *testing.T) {
	style := CellStyle{Bold: true, Italic: true, Underline: true}
	seq := sgrSequence(style)
	if seq != "\x1b[0;1;3;4m" {
		t.Fatalf("expected '\\x1b[0;1;3;4m', got %q", seq)
	}
}

func TestSgrSequence_AllAttributes(t *testing.T) {
	style := CellStyle{Bold: true, Dim: true, Italic: true, Underline: true, Reverse: true, Strike: true}
	seq := sgrSequence(style)
	if seq != "\x1b[0;1;2;3;4;7;9m" {
		t.Fatalf("expected all attributes sequence, got %q", seq)
	}
}

func TestSgrSequence_BrightFGBG(t *testing.T) {
	style := CellStyle{FG: 10, BG: 12} // bright green FG, bright blue BG
	seq := sgrSequence(style)
	if seq != "\x1b[0;91;103m" {
		t.Fatalf("expected bright color sequence, got %q", seq)
	}
}

func TestSgrSequence_256FG(t *testing.T) {
	style := CellStyle{FG: 197} // 256-color index 196
	seq := sgrSequence(style)
	if seq != "\x1b[0;38;5;196m" {
		t.Fatalf("expected 256-color FG sequence, got %q", seq)
	}
}

func TestSgrSequence_256BG(t *testing.T) {
	style := CellStyle{BG: 34} // 256-color index 33
	seq := sgrSequence(style)
	if seq != "\x1b[0;48;5;33m" {
		t.Fatalf("expected 256-color BG sequence, got %q", seq)
	}
}

func TestSgrSequence_TruecolorBG(t *testing.T) {
	bg := 256 + 1 + (0 << 16) + (128 << 8) + 255
	style := CellStyle{BG: bg}
	seq := sgrSequence(style)
	if seq != "\x1b[0;48;2;0;128;255m" {
		t.Fatalf("expected truecolor BG sequence, got %q", seq)
	}
}

// ---------------------------------------------------------------------------
// parseCSIParams extended tests
// ---------------------------------------------------------------------------

func TestParseCSIParams_SingleValue(t *testing.T) {
	s := NewScreen(3, 10)
	s.csiBuf = []byte("5")
	params := s.parseCSIParams()
	if len(params) != 1 || params[0] != 5 {
		t.Fatalf("expected [5], got %v", params)
	}
}

func TestParseCSIParams_MultipleValues(t *testing.T) {
	s := NewScreen(3, 10)
	s.csiBuf = []byte("1;2;3")
	params := s.parseCSIParams()
	if len(params) != 3 || params[0] != 1 || params[1] != 2 || params[2] != 3 {
		t.Fatalf("expected [1,2,3], got %v", params)
	}
}

func TestParseCSIParams_StripsGreaterThan(t *testing.T) {
	s := NewScreen(3, 10)
	s.csiBuf = []byte(">1")
	params := s.parseCSIParams()
	if len(params) != 1 || params[0] != 1 {
		t.Fatalf("expected [1] (stripping '>'), got %v", params)
	}
}

func TestParseCSIParams_StripsExclamation(t *testing.T) {
	s := NewScreen(3, 10)
	s.csiBuf = []byte("!42")
	params := s.parseCSIParams()
	if len(params) != 1 || params[0] != 42 {
		t.Fatalf("expected [42] (stripping '!'), got %v", params)
	}
}

func TestParseCSIParams_StripsEquals(t *testing.T) {
	s := NewScreen(3, 10)
	s.csiBuf = []byte("=10")
	params := s.parseCSIParams()
	if len(params) != 1 || params[0] != 10 {
		t.Fatalf("expected [10] (stripping '='), got %v", params)
	}
}

// ---------------------------------------------------------------------------
// paramDefault extended
// ---------------------------------------------------------------------------

func TestParamDefault_IndexExactlyAtLength(t *testing.T) {
	params := []int{1, 2}
	if v := paramDefault(params, 2, 99); v != 99 {
		t.Fatalf("expected default 99 for index at length, got %d", v)
	}
}

// ---------------------------------------------------------------------------
// OSC title tests
// ---------------------------------------------------------------------------

func TestOSC_SetWindowTitle(t *testing.T) {
	s := NewScreen(3, 30)
	s.Write([]byte("\x1b]0;My Terminal\x07"))

	if s.Title != "My Terminal" {
		t.Fatalf("expected title 'My Terminal', got %q", s.Title)
	}
}

func TestOSC_SetTitle_TypeTwo(t *testing.T) {
	s := NewScreen(3, 30)
	s.Write([]byte("\x1b]2;Window Title\x07"))

	if s.Title != "Window Title" {
		t.Fatalf("expected title 'Window Title', got %q", s.Title)
	}
}

func TestOSC_TitleReplacedOnMultiple(t *testing.T) {
	s := NewScreen(3, 30)
	s.Write([]byte("\x1b]0;First\x07"))
	s.Write([]byte("\x1b]0;Second\x07"))

	if s.Title != "Second" {
		t.Fatalf("expected title 'Second', got %q", s.Title)
	}
}

func TestOSC_IgnoredTypes(t *testing.T) {
	s := NewScreen(3, 30)
	s.Write([]byte("\x1b]0;Initial\x07"))
	// OSC 8 (hyperlink) should be ignored, title stays
	s.Write([]byte("\x1b]8;id=test;https://example.com\x07"))

	if s.Title != "Initial" {
		t.Fatalf("OSC 8 should not change title, got %q", s.Title)
	}
}

// ---------------------------------------------------------------------------
// DEC Save/Restore cursor (ESC 7 / ESC 8)
// ---------------------------------------------------------------------------

func TestDECSaveRestoreCursor_Ext(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("\x1b[3;7H")) // move to row 3, col 7
	s.Write([]byte("\x1b7"))     // DEC save
	s.Write([]byte("\x1b[1;1H")) // move to top-left
	s.Write([]byte("\x1b8"))     // DEC restore

	row, col := s.Cursor()
	if row != 2 || col != 6 { // 0-indexed
		t.Fatalf("expected cursor at (2,6), got (%d,%d)", row, col)
	}
}

// ---------------------------------------------------------------------------
// UTF-8 characters
// ---------------------------------------------------------------------------

func TestUTF8_TwoByte(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("H\xc3\xa4llo")) // Hällo

	cell := s.CellAt(0, 1)
	if cell.Char != 'ä' {
		t.Fatalf("expected 'ä', got %c", cell.Char)
	}
}

func TestUTF8_ThreeByte(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\xe2\x82\xac100")) // €100

	cell := s.CellAt(0, 0)
	if cell.Char != '€' {
		t.Fatalf("expected '€', got %c", cell.Char)
	}
	cell = s.CellAt(0, 1)
	if cell.Char != '1' {
		t.Fatalf("expected '1' after €, got %c", cell.Char)
	}
}

func TestUTF8_FourByte_Emoji(t *testing.T) {
	s := NewScreen(3, 10)
	// 😀 = 0xF0 0x9F 0x98 0x80
	s.Write([]byte("\xf0\x9f\x98\x80"))

	cell := s.CellAt(0, 0)
	if cell.Char != '😀' {
		t.Fatalf("expected '😀', got %c (%U)", cell.Char, cell.Char)
	}
}
