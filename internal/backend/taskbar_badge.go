package backend

import "strconv"

// The number on the taskbar button: how many panes are blocked on the user
// (a permission or a question). Drawn here, without an image library, so the
// Windows half only has to hand pixels to the OS.

// badgeText is the overlay's label and its accessible description.
func badgeText(n int) string {
	switch {
	case n <= 0:
		return ""
	case n > 9:
		return "9+"
	default:
		return strconv.Itoa(n)
	}
}

// digitFont is a 3x5 bitmap font: one string per row, '#' is ink.
var digitFont = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", "###", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", "..#", "..#", "..#"},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	'+': {"...", ".#.", "###", ".#.", "..."},
}

// badgePixels draws n on an amber square with cut corners, 16x16, as top-down
// BGRA rows with straight alpha, the layout a 32-bit icon bitmap takes.
func badgePixels(n int) []byte {
	const size, scale = 16, 2
	pix := make([]byte, size*size*4)
	set := func(x, y int, r, g, b byte) {
		i := (y*size + x) * 4
		pix[i], pix[i+1], pix[i+2], pix[i+3] = b, g, r, 0xff
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if (x == 0 || x == size-1) && (y == 0 || y == size-1) {
				continue // cut corners stay transparent
			}
			set(x, y, 0xd6, 0xa8, 0x5c) // --status-waiting
		}
	}
	text := badgeText(n)
	w := (len(text)*4 - 1) * scale
	ox, oy := (size-w)/2, (size-5*scale)/2
	for ci, ch := range text {
		for gy, row := range digitFont[ch] {
			for gx, c := range row {
				if c != '#' {
					continue
				}
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						set(ox+(ci*4+gx)*scale+sx, oy+gy*scale+sy, 0x1a, 0x14, 0x05)
					}
				}
			}
		}
	}
	return pix
}

// SetAttentionBadge shows n on the taskbar button; 0 removes it. Windows only.
func (a *AppService) SetAttentionBadge(n int) {
	if a.mainWindow == nil {
		return
	}
	setTaskbarBadge(a.mainWindow, n)
}
