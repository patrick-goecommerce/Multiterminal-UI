package hub

import (
	"strings"
	"unicode"
)

// cleanTitle removes what an agent puts in front of its terminal title to
// show that it is busy. Claude Code prefixes the title with a spinner glyph
// ("✳ Datenbankverbindung", "⠂ Datenbankverbindung") that changes while it
// works. Left in, every frame was a new title: an event per pane per scan
// tick, and a name that jumps between glyphs.
//
// Only glyph ranges are removed, never ordinary characters, so a shell title
// like "~/src" or "/home/user" stays what it is.
func cleanTitle(title string) string {
	return strings.TrimSpace(strings.TrimLeftFunc(title, func(r rune) bool {
		return unicode.IsSpace(r) || isSpinnerGlyph(r)
	}))
}

func isSpinnerGlyph(r rune) bool {
	switch {
	case r >= 0x2800 && r <= 0x28FF: // braille patterns (⠂ ⠐ ⠋ …)
		return true
	case r >= 0x2700 && r <= 0x27BF: // dingbats (✳ ✶ ✻ ✽ ✢ …)
		return true
	case r >= 0x25A0 && r <= 0x25FF: // geometric shapes (● ◐ ◆ …)
		return true
	}
	switch r {
	case '*', '·', '•', '∙', '⏺', '✱':
		return true
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
