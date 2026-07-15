package terminal

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// utf8SafeChunkLen – ensures paste chunking never splits a UTF-8 rune, which
// would corrupt special characters on Windows ConPTY.
// ---------------------------------------------------------------------------

func TestUTF8SafeChunkLen_ShortInputReturnsFullLength(t *testing.T) {
	p := []byte("hello")
	if got := utf8SafeChunkLen(p, 512); got != len(p) {
		t.Fatalf("expected %d, got %d", len(p), got)
	}
}

func TestUTF8SafeChunkLen_ASCIICutAtMax(t *testing.T) {
	p := []byte(strings.Repeat("a", 1000))
	if got := utf8SafeChunkLen(p, 512); got != 512 {
		t.Fatalf("pure ASCII should cut exactly at max: got %d", got)
	}
}

func TestUTF8SafeChunkLen_BacksOffContinuationByte(t *testing.T) {
	// "─" is E2 94 80 (3 bytes). Place many so a rune straddles index 512.
	p := []byte(strings.Repeat("─", 400)) // 1200 bytes
	end := utf8SafeChunkLen(p, 512)
	if end > 512 {
		t.Fatalf("boundary must not exceed max: got %d", end)
	}
	// The byte at the returned boundary must be a lead byte (not 10xxxxxx),
	// meaning the preceding rune is complete.
	if end < len(p) && p[end]&0xC0 == 0x80 {
		t.Fatalf("boundary %d lands inside a UTF-8 sequence", end)
	}
	if !utf8.Valid(p[:end]) {
		t.Fatalf("chunk p[:%d] is not valid UTF-8", end)
	}
}

func TestUTF8SafeChunkLen_ExactRuneBoundary(t *testing.T) {
	// 512 is divisible by 2 but not 3; use 2-byte runes so a rune ends
	// exactly at 512 (256 × "ü" = 512 bytes) — no back-off needed.
	p := []byte(strings.Repeat("ü", 300)) // 600 bytes, "ü" = C3 BC
	end := utf8SafeChunkLen(p, 512)
	if end != 512 {
		t.Fatalf("expected clean cut at 512, got %d", end)
	}
}

func TestUTF8SafeChunkLen_NeverReturnsZero(t *testing.T) {
	// All continuation bytes (malformed): must fall back to max, not 0.
	p := bytes.Repeat([]byte{0x80}, 1000)
	if got := utf8SafeChunkLen(p, 512); got == 0 {
		t.Fatal("must never return 0 (would stall the write loop)")
	}
}

// Reassembling all chunks must reproduce the original bytes exactly, and no
// individual chunk may be invalid UTF-8 (proves nothing is split mid-rune).
func TestUTF8SafeChunkLen_FullReassembly(t *testing.T) {
	inputs := []string{
		strings.Repeat("─│┌┐└┘├┤┬┴┼", 200), // box-drawing table
		strings.Repeat("café ☃ 日本語 ", 100), // mixed 2/3-byte runes
		strings.Repeat("a", 2048),           // pure ASCII
		"→ ASCII table: " + strings.Repeat("═╬╣", 500),
	}
	for _, in := range inputs {
		p := []byte(in)
		var reassembled []byte
		rest := p
		for len(rest) > 0 {
			end := utf8SafeChunkLen(rest, 512)
			if end <= 0 || end > len(rest) {
				t.Fatalf("invalid chunk length %d for remaining %d bytes", end, len(rest))
			}
			chunk := rest[:end]
			if !utf8.Valid(chunk) {
				t.Fatalf("chunk is not valid UTF-8 (rune was split)")
			}
			reassembled = append(reassembled, chunk...)
			rest = rest[end:]
		}
		if !bytes.Equal(reassembled, p) {
			t.Fatalf("reassembled bytes differ from original")
		}
	}
}
