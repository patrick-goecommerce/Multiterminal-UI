package terminal

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// PlainTextRow
// ---------------------------------------------------------------------------

func TestPlainTextRow_Basic(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("Hello"))

	got := s.PlainTextRow(0)
	if got != "Hello" {
		t.Errorf("PlainTextRow(0) = %q, want 'Hello'", got)
	}
}

func TestPlainTextRow_TrimsTrailingSpaces(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("Hi"))

	got := s.PlainTextRow(0)
	if got != "Hi" {
		t.Errorf("PlainTextRow(0) = %q (len %d), want 'Hi'", got, len(got))
	}
}

func TestPlainTextRow_EmptyRow(t *testing.T) {
	s := NewScreen(3, 10)
	got := s.PlainTextRow(1)
	if got != "" {
		t.Errorf("PlainTextRow empty row = %q, want empty", got)
	}
}

func TestPlainTextRow_OutOfBounds(t *testing.T) {
	s := NewScreen(3, 10)
	got := s.PlainTextRow(-1)
	if got != "" {
		t.Errorf("PlainTextRow(-1) = %q, want empty", got)
	}
	got = s.PlainTextRow(99)
	if got != "" {
		t.Errorf("PlainTextRow(99) = %q, want empty", got)
	}
}

func TestPlainTextRow_StripsANSI(t *testing.T) {
	s := NewScreen(3, 20)
	// Write colored text — PlainTextRow should return only the text
	s.Write([]byte("\x1b[31mRed\x1b[0m Normal"))

	got := s.PlainTextRow(0)
	if got != "Red Normal" {
		t.Errorf("PlainTextRow with ANSI = %q, want 'Red Normal'", got)
	}
}

// ---------------------------------------------------------------------------
// PlainText
// ---------------------------------------------------------------------------

func TestPlainText_MultipleRows(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("AAA\r\nBBB\r\nCCC"))

	got := s.PlainText()
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("PlainText has %d lines, want 3", len(lines))
	}
	// Each line is padded to column width
	if !strings.HasPrefix(lines[0], "AAA") {
		t.Errorf("Line 0 = %q, want prefix 'AAA'", lines[0])
	}
	if !strings.HasPrefix(lines[1], "BBB") {
		t.Errorf("Line 1 = %q, want prefix 'BBB'", lines[1])
	}
	if !strings.HasPrefix(lines[2], "CCC") {
		t.Errorf("Line 2 = %q, want prefix 'CCC'", lines[2])
	}
}

func TestPlainText_BlankScreen(t *testing.T) {
	s := NewScreen(2, 5)
	got := s.PlainText()
	// Should be all spaces, 2 rows of 5 chars separated by newline
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("Blank PlainText has %d lines, want 2", len(lines))
	}
	for i, line := range lines {
		if len(line) != 5 {
			t.Errorf("Blank line %d length = %d, want 5", i, len(line))
		}
		if strings.TrimSpace(line) != "" {
			t.Errorf("Blank line %d = %q, want all spaces", i, line)
		}
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestRender_ContainsText(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("Hello"))

	rendered := s.Render()
	if !strings.Contains(rendered, "Hello") {
		t.Errorf("Render output does not contain 'Hello': %q", rendered)
	}
}

func TestRender_ContainsANSI(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("\x1b[1;31mBold Red\x1b[0m"))

	rendered := s.Render()
	// Should contain ANSI escape sequences
	if !strings.Contains(rendered, "\x1b[") {
		t.Error("Render output should contain ANSI escape sequences")
	}
	if !strings.Contains(rendered, "Bold Red") {
		t.Error("Render output should contain the text")
	}
}

func TestRender_EndsWithReset(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("Test"))

	rendered := s.Render()
	if !strings.HasSuffix(rendered, "\x1b[0m") {
		t.Errorf("Render should end with reset sequence, got suffix: %q",
			rendered[len(rendered)-10:])
	}
}

// ---------------------------------------------------------------------------
// RenderRegion
// ---------------------------------------------------------------------------

func TestRenderRegion_SubRectangle(t *testing.T) {
	s := NewScreen(5, 10)
	s.Write([]byte("0123456789\r\n"))
	s.Write([]byte("ABCDEFGHIJ\r\n"))
	s.Write([]byte("abcdefghij\r\n"))

	// Render rows 1-2, cols 2-5
	rendered := s.RenderRegion(1, 2, 2, 5)

	// Should contain the sub-rectangle text
	if !strings.Contains(rendered, "CDEF") {
		t.Errorf("RenderRegion should contain 'CDEF', got %q", rendered)
	}
	if !strings.Contains(rendered, "cdef") {
		t.Errorf("RenderRegion should contain 'cdef', got %q", rendered)
	}
	// Should NOT contain content outside the region
	if strings.Contains(rendered, "0123") {
		t.Error("RenderRegion should not contain row 0 content")
	}
}

func TestRenderRegion_OutOfBounds(t *testing.T) {
	s := NewScreen(3, 5)
	s.Write([]byte("Hello"))

	// Request region beyond screen bounds — should not panic
	rendered := s.RenderRegion(0, 0, 99, 99)
	if !strings.Contains(rendered, "Hello") {
		t.Errorf("RenderRegion out-of-bounds should still contain visible content")
	}
}
