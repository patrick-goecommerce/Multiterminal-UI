package ui

// Rect describes a rectangular region on screen (0-indexed).
type Rect struct {
	X, Y          int // top-left corner
	Width, Height int
}

// ComputeGrid calculates a grid layout for n panes within the given area.
// The algorithm aims for a roughly square arrangement:
//
//	1 pane  → 1×1
//	2 panes → 1×2 (side by side)
//	3 panes → 1×2 top + 1×1 bottom  (or 2 rows)
//	4 panes → 2×2
//	5-6     → 2×3
//	7-9     → 3×3
//	10-12   → 3×4
//
// Each pane gets a Rect. Leftover space is distributed to the last column/row.
func ComputeGrid(n, areaWidth, areaHeight int) []Rect {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []Rect{{X: 0, Y: 0, Width: areaWidth, Height: areaHeight}}
	}

	cols, rows := gridDimensions(n)

	rects := make([]Rect, n)
	baseW := areaWidth / cols
	baseH := areaHeight / rows

	idx := 0
	for r := 0; r < rows && idx < n; r++ {
		// How many panes in this row?
		rowPanes := cols
		if r == rows-1 {
			rowPanes = n - idx // last row gets the remainder
		}

		for c := 0; c < rowPanes && idx < n; c++ {
			x := c * baseW
			y := r * baseH
			w := baseW
			h := baseH

			// Give extra width to last column in this row
			if c == rowPanes-1 {
				w = areaWidth - x
			}
			// Give extra height to last row
			if r == rows-1 {
				h = areaHeight - y
			}

			rects[idx] = Rect{X: x, Y: y, Width: w, Height: h}
			idx++
		}
	}
	return rects
}

// gridDimensions returns (cols, rows) for n panes.
func gridDimensions(n int) (int, int) {
	switch {
	case n <= 1:
		return 1, 1
	case n <= 2:
		return 2, 1
	case n <= 4:
		return 2, 2
	case n <= 6:
		return 3, 2
	case n <= 9:
		return 3, 3
	default:
		return 4, 3
	}
}
