// Package config – pane layout settings.
package config

// Pane layout modes. LayoutGrid is the default and the only mode that existed
// before; LayoutFocus is opt-in from the appearance settings.
//
// Focus mode shows the two most recently used panes of a tab big, the rest of
// that tab small along the right edge, and lists the panes of OTHER tabs that
// wait for the user (permission or question). One of those opens as a floating
// window over the current tab, so answering it does not mean switching tabs.
const (
	LayoutGrid  = "grid"
	LayoutFocus = "focus"
)

// LayoutSettings holds the pane arrangement and where the floating window of
// focus mode was last dragged to. FloatX/FloatY are CSS pixels relative to the
// pane area; negative means "never moved", and the frontend then centres it.
// The frontend clamps them into the current area, so a smaller window after a
// restart never puts the floating window out of reach.
type LayoutSettings struct {
	Mode   string `yaml:"mode" json:"mode"`
	FloatX int    `yaml:"float_x" json:"float_x"`
	FloatY int    `yaml:"float_y" json:"float_y"`
}

func defaultLayout() LayoutSettings {
	return LayoutSettings{Mode: LayoutGrid, FloatX: -1, FloatY: -1}
}

// normalizeLayout maps anything unknown to the grid: an unrecognised value in
// a hand-edited YAML must fall back to the layout everybody already knows.
func normalizeLayout(l *LayoutSettings) {
	if l.Mode != LayoutFocus {
		l.Mode = LayoutGrid
	}
}
