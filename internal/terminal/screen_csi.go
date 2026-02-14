package terminal

import (
	"fmt"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// CSI command dispatch
// ---------------------------------------------------------------------------

// dispatchCSI executes a CSI sequence given the final command byte.
func (s *Screen) dispatchCSI(cmd byte) {
	params := s.parseCSIParams()

	switch cmd {
	case 'A': // Cursor Up
		n := paramDefault(params, 0, 1)
		s.curRow -= n
		if s.curRow < 0 {
			s.curRow = 0
		}
	case 'B': // Cursor Down
		n := paramDefault(params, 0, 1)
		s.curRow += n
		if s.curRow >= s.rows {
			s.curRow = s.rows - 1
		}
	case 'C': // Cursor Forward
		n := paramDefault(params, 0, 1)
		s.curCol += n
		if s.curCol >= s.cols {
			s.curCol = s.cols - 1
		}
	case 'D': // Cursor Backward
		n := paramDefault(params, 0, 1)
		s.curCol -= n
		if s.curCol < 0 {
			s.curCol = 0
		}
	case 'E': // Cursor Next Line
		n := paramDefault(params, 0, 1)
		s.curRow += n
		if s.curRow >= s.rows {
			s.curRow = s.rows - 1
		}
		s.curCol = 0
	case 'F': // Cursor Previous Line
		n := paramDefault(params, 0, 1)
		s.curRow -= n
		if s.curRow < 0 {
			s.curRow = 0
		}
		s.curCol = 0
	case 'G': // Cursor Horizontal Absolute
		n := paramDefault(params, 0, 1)
		s.curCol = n - 1
		s.clampCursor()
	case 'H', 'f': // Cursor Position
		row := paramDefault(params, 0, 1)
		col := paramDefault(params, 1, 1)
		s.curRow = row - 1
		s.curCol = col - 1
		s.clampCursor()
	case 'J': // Erase in Display
		s.eraseDisplay(paramDefault(params, 0, 0))
	case 'K': // Erase in Line
		s.eraseLine(paramDefault(params, 0, 0))
	case 'L': // Insert Lines
		n := paramDefault(params, 0, 1)
		s.insertLines(n)
	case 'M': // Delete Lines
		n := paramDefault(params, 0, 1)
		s.deleteLines(n)
	case 'P': // Delete Characters
		n := paramDefault(params, 0, 1)
		s.deleteChars(n)
	case '@': // Insert Characters
		n := paramDefault(params, 0, 1)
		s.insertChars(n)
	case 'S': // Scroll Up
		n := paramDefault(params, 0, 1)
		for i := 0; i < n; i++ {
			s.scrollUp()
		}
	case 'T': // Scroll Down
		n := paramDefault(params, 0, 1)
		for i := 0; i < n; i++ {
			s.scrollDown()
		}
	case 'm': // SGR – Select Graphic Rendition
		s.handleSGR(params)
	case 'r': // DECSTBM – Set Scrolling Region
		top := paramDefault(params, 0, 1)
		bottom := paramDefault(params, 1, s.rows)
		s.scrollTop = top
		s.scrollBottom = bottom
	case 's': // Save Cursor Position
		s.savedRow = s.curRow
		s.savedCol = s.curCol
	case 'u': // Restore Cursor Position
		s.curRow = s.savedRow
		s.curCol = s.savedCol
	case 'h', 'l': // Set/Reset Mode – largely ignored
		// We handle CSI ? 25 h/l (show/hide cursor) by ignoring it
		// since our rendering always shows the cursor.
	case 'X': // Erase Characters
		n := paramDefault(params, 0, 1)
		for i := 0; i < n && s.curCol+i < s.cols; i++ {
			s.cells[s.curRow][s.curCol+i] = Cell{Char: ' ', Style: s.style}
		}
	case 'd': // Vertical Position Absolute
		n := paramDefault(params, 0, 1)
		s.curRow = n - 1
		s.clampCursor()
	}
}

// parseCSIParams splits the CSI parameter buffer into integer parameters.
// ";" separates values; missing values default to 0.
func (s *Screen) parseCSIParams() []int {
	// Strip leading '?' or '>' (private mode prefix)
	raw := string(s.csiBuf)
	raw = strings.TrimLeft(raw, "?>=!")

	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	params := make([]int, len(parts))
	for i, p := range parts {
		v, _ := strconv.Atoi(p)
		params[i] = v
	}
	return params
}

// paramDefault returns params[idx] if it exists and is > 0, otherwise def.
func paramDefault(params []int, idx, def int) int {
	if idx < len(params) && params[idx] > 0 {
		return params[idx]
	}
	return def
}

// ---------------------------------------------------------------------------
// SGR (Select Graphic Rendition) handler
// ---------------------------------------------------------------------------

// handleSGR processes a CSI m sequence to update the current drawing style.
func (s *Screen) handleSGR(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	i := 0
	for i < len(params) {
		p := params[i]
		switch {
		case p == 0: // Reset
			s.style = CellStyle{}
		case p == 1:
			s.style.Bold = true
		case p == 2:
			s.style.Dim = true
		case p == 3:
			s.style.Italic = true
		case p == 4:
			s.style.Underline = true
		case p == 7:
			s.style.Reverse = true
		case p == 9:
			s.style.Strike = true
		case p == 22:
			s.style.Bold = false
			s.style.Dim = false
		case p == 23:
			s.style.Italic = false
		case p == 24:
			s.style.Underline = false
		case p == 27:
			s.style.Reverse = false
		case p == 29:
			s.style.Strike = false
		case p >= 30 && p <= 37: // Standard FG
			s.style.FG = p - 30 + 1
		case p == 38: // Extended FG
			i = s.parseSGRColor(params, i, true)
		case p == 39: // Default FG
			s.style.FG = 0
		case p >= 40 && p <= 47: // Standard BG
			s.style.BG = p - 40 + 1
		case p == 48: // Extended BG
			i = s.parseSGRColor(params, i, false)
		case p == 49: // Default BG
			s.style.BG = 0
		case p >= 90 && p <= 97: // Bright FG
			s.style.FG = p - 90 + 9
		case p >= 100 && p <= 107: // Bright BG
			s.style.BG = p - 100 + 9
		}
		i++
	}
}

// parseSGRColor handles "38;5;N" (256-colour) and "38;2;R;G;B" (truecolour)
// sequences for foreground (fg=true) or background (fg=false).
// Returns the updated index into params.
func (s *Screen) parseSGRColor(params []int, i int, fg bool) int {
	if i+1 >= len(params) {
		return i
	}
	mode := params[i+1]
	switch mode {
	case 5: // 256-colour
		if i+2 < len(params) {
			c := params[i+2] + 1 // +1 so 0 means "default"
			if fg {
				s.style.FG = c
			} else {
				s.style.BG = c
			}
			return i + 2
		}
	case 2: // Truecolour RGB
		if i+4 < len(params) {
			r, g, b := params[i+2], params[i+3], params[i+4]
			// Encode as 0x01RRGGBB + 256 to distinguish from palette
			c := 256 + 1 + r<<16 + g<<8 + b
			if fg {
				s.style.FG = c
			} else {
				s.style.BG = c
			}
			return i + 4
		}
	}
	return i + 1
}

// sgrSequence produces the SGR escape sequence to switch to the given style.
func sgrSequence(st CellStyle) string {
	var parts []string
	parts = append(parts, "0") // always reset first

	if st.Bold {
		parts = append(parts, "1")
	}
	if st.Dim {
		parts = append(parts, "2")
	}
	if st.Italic {
		parts = append(parts, "3")
	}
	if st.Underline {
		parts = append(parts, "4")
	}
	if st.Reverse {
		parts = append(parts, "7")
	}
	if st.Strike {
		parts = append(parts, "9")
	}
	// Foreground
	if st.FG > 0 && st.FG <= 8 {
		parts = append(parts, fmt.Sprintf("%d", 29+st.FG))
	} else if st.FG >= 9 && st.FG <= 16 {
		parts = append(parts, fmt.Sprintf("%d", 81+st.FG))
	} else if st.FG > 16 && st.FG <= 256 {
		parts = append(parts, fmt.Sprintf("38;5;%d", st.FG-1))
	} else if st.FG > 256 {
		rgb := st.FG - 257
		parts = append(parts, fmt.Sprintf("38;2;%d;%d;%d", (rgb>>16)&0xFF, (rgb>>8)&0xFF, rgb&0xFF))
	}
	// Background
	if st.BG > 0 && st.BG <= 8 {
		parts = append(parts, fmt.Sprintf("%d", 39+st.BG))
	} else if st.BG >= 9 && st.BG <= 16 {
		parts = append(parts, fmt.Sprintf("%d", 91+st.BG))
	} else if st.BG > 16 && st.BG <= 256 {
		parts = append(parts, fmt.Sprintf("48;5;%d", st.BG-1))
	} else if st.BG > 256 {
		rgb := st.BG - 257
		parts = append(parts, fmt.Sprintf("48;2;%d;%d;%d", (rgb>>16)&0xFF, (rgb>>8)&0xFF, rgb&0xFF))
	}

	return "\x1b[" + strings.Join(parts, ";") + "m"
}
