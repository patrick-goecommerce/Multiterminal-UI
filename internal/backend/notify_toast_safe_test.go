package backend

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

func TestToastSafe(t *testing.T) {
	cases := []struct{ in, want string }{
		{"$(Remove-Item x)", "`$(Remove-Item x)"},
		{"a`b", "a``b"},
		{"cost $5", "cost `$5"},
		{"x]]>y", "x] ]>y"},
		{"line1\n\"@\nline2", "line1 \"@ line2"},
		{"Größe prüfen: fertig", "Größe prüfen: fertig"},
	}
	for _, c := range cases {
		if got := toastSafe(c.in); got != c.want {
			t.Errorf("toastSafe(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := toastAttr(`a"b&c`); got != "a&quot;b&amp;c" {
		t.Errorf("toastAttr = %q", got)
	}
}

func TestPaneActivation(t *testing.T) {
	if got := paneActivation(hub.Ref{}); got != "multiterminal:focus" {
		t.Errorf("zero ref: %q", got)
	}
	if got := paneActivation(hub.Local("ab12", 5)); got != "multiterminal:focus/ab12:5" {
		t.Errorf("local ref: %q", got)
	}
}
