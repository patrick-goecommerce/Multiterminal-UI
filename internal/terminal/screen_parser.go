package terminal

import (
	"strings"
	"unicode/utf8"
)

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
